package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// RowDivergence describes one row whose content did not survive a rebuild
// intact. Key is the row's primary key rendered as text (composite keys are
// joined with "|"), so the report reads the same for every table.
type RowDivergence struct {
	Table string `json:"table"`
	Key   string `json:"key"`
	// Kind is "changed" when the row came back with different content, or
	// "missing" when it did not come back at all.
	Kind string `json:"kind"`
	// HashBefore / HashAfter are the canonical row hashes either side of the
	// replay. HashAfter is empty for a missing row.
	HashBefore string `json:"hash_before"`
	HashAfter  string `json:"hash_after,omitempty"`
}

// Not reported here: whether a divergence was caused by two changelog rows
// sharing an identical updated_at, the shape where append order decides the
// winner. That tie lives in the changelog *files* — the database holds one
// upserted row per key, so it cannot be detected by querying pql.db. Answering
// it needs the per-statement staging table T-68 builds, where it becomes a
// GROUP BY; attributing causes before then would mean guessing at SQL text.

// VerifyReport is the result of a verified rebuild.
type VerifyReport struct {
	// RowsBefore / RowsAfter count every replicated row either side of the
	// replay, keyed by table.
	RowsBefore map[string]int `json:"rows_before"`
	RowsAfter  map[string]int `json:"rows_after"`
	// Divergences lists rows that changed or vanished. A rebuild of a healthy
	// vault produces none.
	Divergences []RowDivergence `json:"divergences,omitempty"`
	// RowsLost is the number of divergences of kind "missing" — the subset
	// that is data loss rather than a content change.
	RowsLost int `json:"rows_lost"`
}

// keyColumns names the primary key of each replicated table, in the order the
// key is rendered for reporting.
var keyColumns = map[string][]string{
	"tickets":       {"record_id"},
	"ticket_idmap":  {"record_id"},
	"ticket_deps":   {"blocker_record_id", "blocked_record_id"},
	"ticket_labels": {"ticket_record_id", "label"},
	// ticket_history has no natural key — it dedupes on hash and is
	// append-only, so a row cannot "change". Verified by count alone.
}

// rowState is one row's identity as far as verification cares.
type rowState struct {
	hash      string
	updatedAt string
}

// snapshot reads key → {hash, updated_at} for every verifiable table.
func snapshot(ctx context.Context, db *sql.DB) (map[string]map[string]rowState, map[string]int, error) {
	states := make(map[string]map[string]rowState, len(keyColumns))
	counts := make(map[string]int, len(replicatedTables))

	for _, table := range replicatedTables {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil { //nolint:gosec // G202: closed-set table whitelist
			return nil, nil, fmt.Errorf("changelog: count %s: %w", table, err)
		}
		counts[table] = n

		cols, ok := keyColumns[table]
		if !ok {
			continue
		}
		rows, err := db.QueryContext(ctx, //nolint:gosec // G202: closed-set table and column whitelist
			"SELECT "+joinCols(cols)+", COALESCE(hash,''), COALESCE(updated_at,'') FROM "+table)
		if err != nil {
			return nil, nil, fmt.Errorf("changelog: snapshot %s: %w", table, err)
		}
		byKey := make(map[string]rowState)
		if err := scanKeyed(rows, len(cols), func(key string, st rowState) {
			byKey[key] = st
		}); err != nil {
			return nil, nil, fmt.Errorf("changelog: snapshot %s: %w", table, err)
		}
		states[table] = byKey
	}
	return states, counts, nil
}

// VerifiedRebuild runs Rebuild with a before/after comparison around it, so a
// replay that silently changes or drops a row is reported instead of passing
// unnoticed.
//
// It exists because that failure mode went undetected for a month: the live
// database served a placeholder description while the real text survived only
// as a changelog row, and every rebuild quietly reaffirmed the placeholder
// (T-59/T-60). Counts alone would not have caught it — the row was present,
// just wrong — so the comparison is per-row on the canonical hash.
func VerifiedRebuild(ctx context.Context, db *sql.DB, vaultPath string) (*RebuildResult, *VerifyReport, error) {
	before, countsBefore, err := snapshot(ctx, db)
	if err != nil {
		return nil, nil, err
	}

	res, err := Rebuild(ctx, db, vaultPath)
	if err != nil {
		return nil, nil, err
	}

	after, countsAfter, err := snapshot(ctx, db)
	if err != nil {
		return nil, nil, err
	}

	report := &VerifyReport{RowsBefore: countsBefore, RowsAfter: countsAfter}
	for _, table := range replicatedTables {
		beforeRows, ok := before[table]
		if !ok {
			continue
		}
		afterRows := after[table]
		for key, was := range beforeRows {
			is, present := afterRows[key]
			switch {
			case !present:
				report.Divergences = append(report.Divergences, RowDivergence{
					Table: table, Key: key, Kind: "missing", HashBefore: was.hash,
				})
				report.RowsLost++
			case is.hash != was.hash:
				report.Divergences = append(report.Divergences, RowDivergence{
					Table: table, Key: key, Kind: "changed",
					HashBefore: was.hash, HashAfter: is.hash,
				})
			}
		}
	}

	// Sorted so two runs over the same vault produce the same report — map
	// iteration order would otherwise reshuffle it every time.
	sort.Slice(report.Divergences, func(i, j int) bool {
		a, b := report.Divergences[i], report.Divergences[j]
		if a.Table != b.Table {
			return a.Table < b.Table
		}
		return a.Key < b.Key
	})

	return res, report, nil
}

func joinCols(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// scanKeyed walks rows shaped as (key columns..., hash, updated_at), joining
// the key columns with "|" so composite keys report as one string.
func scanKeyed(rows *sql.Rows, nKeys int, emit func(string, rowState)) error {
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		vals := make([]string, nKeys+2)
		dest := make([]any, len(vals))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		key := ""
		for i := 0; i < nKeys; i++ {
			if i > 0 {
				key += "|"
			}
			key += vals[i]
		}
		emit(key, rowState{hash: vals[nKeys], updatedAt: vals[nKeys+1]})
	}
	return rows.Err()
}
