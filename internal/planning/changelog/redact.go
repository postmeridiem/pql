package changelog

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/postmeridiem/pql/internal/planning"
)

// RedactResult is the receipt: what was rewritten, and where.
type RedactResult struct {
	RecordID       string   `json:"record_id"`
	TicketRows     int      `json:"ticket_rows"`
	HistoryRows    int      `json:"history_rows"`
	StagedLines    int      `json:"changelog_lines"`
	FilesRewritten []string `json:"files_rewritten"`
	RemotesChecked []string `json:"remotes_checked"`
}

// ErrRedactPushed is wrapped by the refusal so callers can map it to the
// right exit code and hint without string matching.
var ErrRedactPushed = fmt.Errorf("value appears in pushed changelog history")

// redactTables is the subset of replicated tables that carry ticket prose —
// the only place a leaked value can live. idmap holds labels, deps and
// labels hold identifiers; none carry free text.
//
//nolint:goconst // literal table/column names keep the on-disk format legible, as in statements.go
var redactTables = []string{"tickets", "ticket_history"}

// Redact removes every occurrence of oldValue from one ticket's prose,
// everywhere it lives: the tickets row, every ticket_history row (a
// description fix appends the leaked text as old_value — fixing forward
// duplicates a leak, which is why this exists), and the committed
// changelog lines that carry those rows. Database and files move together,
// as one operation, so the divergence T-105 observed cannot arise (D-35).
//
// The push boundary is the rewrite rule: if oldValue already appears in
// any remote-tracking ref's version of the affected changelog files, the
// redaction is refused — that history is published, and rewriting it is a
// git-history operation pql documents but does not perform. A vault with
// no git or no remotes has no boundary; everything is draft.
//
// The changelog rewrite goes through the same staged-SQLite path replay
// and upgrade use (D-28): rows are parsed by executing their own INSERTs,
// rewritten as typed columns, rehashed with the same canonical projection
// the database side uses, and re-emitted through the same renderer — so
// untouched lines come back byte-identical and the rewritten file replays
// exactly like an exported one. updated_at is deliberately not bumped:
// the row is rewritten in place as if the leak never existed, so the
// write-through exporter has nothing new to re-emit.
func Redact(ctx context.Context, db *sql.DB, vaultPath, recordID, oldValue, newValue string) (*RedactResult, error) {
	if oldValue == "" {
		return nil, fmt.Errorf("changelog: redact: the value to remove must be non-empty")
	}
	if oldValue == newValue {
		return nil, fmt.Errorf("changelog: redact: replacement equals the value to remove")
	}

	remotes, err := pushedRefsHolding(vaultPath, oldValue)
	if err != nil {
		return nil, err
	}
	if len(remotes) > 0 {
		return nil, fmt.Errorf("%w: %s — redacting published history means rewriting git history (D-35)",
			ErrRedactPushed, strings.Join(remotes, ", "))
	}
	checked, err := remoteRefs(vaultPath)
	if err != nil {
		return nil, err
	}
	if checked == nil {
		checked = []string{} // receipts carry empty arrays, never null (T-81)
	}

	res := &RedactResult{RecordID: recordID, RemotesChecked: checked, FilesRewritten: []string{}}

	// Database side first, in one transaction: the current tickets row and
	// every history row that carries the value, each rehashed the moment
	// its content changes so hash never disagrees with content.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("changelog: redact: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// replace() propagates NULL, so a NULL column stays NULL rather than
	// collapsing to '' — only columns that actually carry the value change.
	tr, err := tx.ExecContext(ctx, `
		UPDATE tickets SET
			title       = replace(title, ?1, ?2),
			description = replace(description, ?1, ?2),
			assigned_to = replace(assigned_to, ?1, ?2),
			team        = replace(team, ?1, ?2)
		WHERE record_id = ?3
		  AND (instr(title, ?1) > 0 OR instr(coalesce(description,''), ?1) > 0
		    OR instr(coalesce(assigned_to,''), ?1) > 0 OR instr(coalesce(team,''), ?1) > 0)
	`, oldValue, newValue, recordID)
	if err != nil {
		return nil, fmt.Errorf("changelog: redact tickets row: %w", err)
	}
	if n, _ := tr.RowsAffected(); n > 0 {
		res.TicketRows = int(n)
		if err := planning.RehashTicket(ctx, tx, recordID); err != nil {
			return nil, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT rowid FROM ticket_history
		WHERE ticket_record_id = ?1
		  AND (instr(coalesce(old_value,''), ?2) > 0 OR instr(coalesce(new_value,''), ?2) > 0
		    OR instr(coalesce(changed_by,''), ?2) > 0)
	`, recordID, oldValue)
	if err != nil {
		return nil, fmt.Errorf("changelog: redact: find history rows: %w", err)
	}
	var histIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		histIDs = append(histIDs, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range histIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE ticket_history SET
				old_value  = replace(old_value, ?1, ?2),
				new_value  = replace(new_value, ?1, ?2),
				changed_by = replace(changed_by, ?1, ?2)
			WHERE rowid = ?3
		`, oldValue, newValue, id); err != nil {
			return nil, fmt.Errorf("changelog: redact history rowid %d: %w", id, err)
		}
		if err := planning.RehashTicketHistory(ctx, tx, id); err != nil {
			return nil, err
		}
	}
	res.HistoryRows = len(histIDs)

	// File side: stage, rewrite the staged rows of this record, rehash them
	// with the same projection, re-render. Historical lines — superseded
	// states of the tickets row, which is where a fixed-forward leak keeps
	// living — are reachable only here, because the database no longer
	// holds them.
	stage, err := newStaging(ctx)
	if err != nil {
		return nil, err
	}
	defer stage.close()

	root := filepath.Join(vaultPath, ChangelogDir)
	for _, table := range redactTables {
		spec, ok := specFor(table)
		if !ok {
			return nil, fmt.Errorf("changelog: redact: no spec for %s", table)
		}
		files, err := stage.loadTable(ctx, spec, filepath.Join(root, table))
		if err != nil {
			return nil, err
		}
		changedLines, err := stage.redactRows(ctx, spec, recordID, oldValue, newValue)
		if err != nil {
			return nil, err
		}
		if changedLines == 0 {
			continue
		}
		res.StagedLines += changedLines
		for _, name := range files {
			lines, err := stage.renderFile(ctx, spec, name)
			if err != nil {
				return nil, err
			}
			path := filepath.Join(root, table, name)
			body := ""
			if len(lines) > 0 {
				body = strings.Join(lines, "\n") + "\n"
			}
			existing, err := os.ReadFile(path) //nolint:gosec // G304: path built from a directory just listed
			if err != nil {
				return nil, fmt.Errorf("changelog: redact: read %s: %w", path, err)
			}
			if string(existing) == body {
				continue
			}
			if err := writeFileAtomic(path, body); err != nil {
				return nil, err
			}
			res.FilesRewritten = append(res.FilesRewritten, filepath.Join(table, name))
		}
	}

	if res.TicketRows == 0 && res.HistoryRows == 0 && res.StagedLines == 0 {
		return nil, fmt.Errorf("changelog: redact: value not found in ticket %s or its history", recordID)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("changelog: redact: commit: %w", err)
	}
	return res, nil
}

// redactRows rewrites the staged rows of one record that carry the value,
// recomputing each row's hash with the same canonical projection the
// database-side rehash uses, so a staged line and a database row with the
// same content always carry the same hash.
func (s *staged) redactRows(ctx context.Context, spec tableSpec, recordID, oldValue, newValue string) (int, error) {
	var idCol string
	switch spec.Name {
	case "tickets":
		idCol = "record_id" //nolint:goconst // literal names keep the on-disk format legible
	case "ticket_history":
		idCol = "ticket_record_id"
	default:
		return 0, fmt.Errorf("changelog: redact: unsupported table %s", spec.Name)
	}

	//nolint:gosec // G202: table and column names come from the closed spec list
	rows, err := s.db.QueryContext(ctx,
		"SELECT seq, "+strings.Join(spec.Columns, ", ")+" FROM "+spec.Name+
			" WHERE "+idCol+" = ? ORDER BY seq", recordID)
	if err != nil {
		return 0, fmt.Errorf("changelog: redact: read staged %s: %w", spec.Name, err)
	}
	defer func() { _ = rows.Close() }()

	type stagedRow struct {
		seq  int64
		vals map[string]sql.NullString
	}
	var touched []stagedRow
	for rows.Next() {
		var seq int64
		vals := make([]sql.NullString, len(spec.Columns))
		dest := make([]any, 0, len(vals)+1)
		dest = append(dest, &seq)
		for i := range vals {
			dest = append(dest, &vals[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return 0, err
		}
		m := make(map[string]sql.NullString, len(spec.Columns))
		hit := false
		for i, c := range spec.Columns {
			v := vals[i]
			if v.Valid && strings.Contains(v.String, oldValue) && c != "hash" { //nolint:goconst // literal column name, as in statements.go
				v.String = strings.ReplaceAll(v.String, oldValue, newValue)
				hit = true
			}
			m[c] = v
		}
		if hit {
			touched = append(touched, stagedRow{seq: seq, vals: m})
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, r := range touched {
		h, err := stagedHash(spec.Name, r.vals)
		if err != nil {
			return 0, err
		}
		r.vals["hash"] = sql.NullString{String: h, Valid: true}
		r.vals["canonical_version"] = sql.NullString{String: fmt.Sprint(planning.CanonicalVersion), Valid: true}

		sets := make([]string, len(spec.Columns))
		args := make([]any, 0, len(spec.Columns)+1)
		for i, c := range spec.Columns {
			sets[i] = c + " = ?"
			args = append(args, nullArg(r.vals[c]))
		}
		args = append(args, r.seq)
		//nolint:gosec // G202: closed-set table and column names
		if _, err := s.db.ExecContext(ctx,
			"UPDATE "+spec.Name+" SET "+strings.Join(sets, ", ")+" WHERE seq = ?", args...); err != nil {
			return 0, fmt.Errorf("changelog: redact: rewrite staged %s seq %d: %w", spec.Name, r.seq, err)
		}
	}
	return len(touched), nil
}

// stagedHash computes the canonical hash of a staged row, in exactly the
// value order the database-side Rehash* functions use for the same table.
func stagedHash(table string, v map[string]sql.NullString) (string, error) {
	np := func(c string) *string {
		val := v[c]
		if !val.Valid {
			return nil
		}
		s := val.String
		return &s
	}
	str := func(c string) string { return v[c].String }

	switch table {
	case "tickets":
		return planning.Hash([]any{
			planning.CanonicalVersion,
			str("record_id"), str("type"), np("parent_record_id"), str("title"), np("description"),
			str("status"), str("priority"),
			np("assigned_to"), np("team"), np("decision_ref"),
			str("created_at"), str("updated_at"), np("deleted_at"),
		}), nil
	case "ticket_history":
		return planning.Hash([]any{
			planning.CanonicalVersion,
			str("ticket_record_id"), str("field"),
			np("old_value"), np("new_value"), np("changed_by"),
			str("changed_at"), str("created_at"), str("updated_at"), np("deleted_at"),
		}), nil
	}
	return "", fmt.Errorf("changelog: redact: no hash recipe for table %s", table)
}

func nullArg(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}

// remoteRefs lists the remote-tracking refs of the vault's repository.
// A vault outside git, or git being absent entirely, yields none — and
// per D-35 no refs means no boundary: everything is local draft.
func remoteRefs(vaultPath string) ([]string, error) {
	out, err := exec.Command("git", "-C", vaultPath, "for-each-ref", "--format=%(refname)", "refs/remotes").Output() //nolint:gosec // G204: fixed binary, argv never shell-interpreted
	if err != nil {
		return nil, nil // not a repo, or no git: no boundary
	}
	var refs []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			refs = append(refs, l)
		}
	}
	return refs, nil
}

// pushedRefsHolding reports which remote-tracking refs carry the value in
// any affected changelog file. The question is not which lines would be
// rewritten but whether the value is already published — if it is,
// removing it locally accomplishes nothing and hides that it is out.
func pushedRefsHolding(vaultPath, value string) ([]string, error) {
	refs, err := remoteRefs(vaultPath)
	if err != nil || len(refs) == 0 {
		return nil, err
	}

	// The changelog dir may sit below the repo root; git paths are
	// root-relative, so resolve the prefix once.
	topOut, err := exec.Command("git", "-C", vaultPath, "rev-parse", "--show-toplevel").Output() //nolint:gosec // G204: fixed binary, argv never shell-interpreted
	if err != nil {
		return nil, nil
	}
	top := strings.TrimSpace(string(topOut))
	rel, err := filepath.Rel(top, filepath.Join(vaultPath, ChangelogDir))
	if err != nil {
		return nil, fmt.Errorf("changelog: redact: relate %s to %s: %w", vaultPath, top, err)
	}

	refHolds := func(ref string) bool {
		for _, table := range redactTables {
			dir := filepath.Join(vaultPath, ChangelogDir, table)
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
					continue
				}
				gitPath := filepath.ToSlash(filepath.Join(rel, table, e.Name()))
				body, err := exec.Command("git", "-C", vaultPath, "show", ref+":"+gitPath).Output() //nolint:gosec // G204: fixed binary; ref and path come from git itself and a listed dir
				if err != nil {
					continue // file absent in that ref
				}
				if bytes.Contains(body, []byte(value)) {
					return true
				}
			}
		}
		return false
	}

	var holding []string
	for _, ref := range refs {
		if refHolds(ref) {
			holding = append(holding, ref)
		}
	}
	return holding, nil
}
