package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

// ChangelogDir is the per-vault directory that holds replication state
// files: <vault>/.pql/changelog/<table>/<YYYY-MM>.sql.
const ChangelogDir = ".pql/changelog"

// markerFormat is the SQLite datetime('now') format we store in the
// meta table for export/import markers. Matches updated_at on every
// row, so direct string comparison works (lexicographic == temporal
// for this format).
const markerFormat = "2006-01-02 15:04:05"

// The marker query is inclusive (updated_at >= marker), not strict.
// Write-through (D-23) calls Export after every mutation, so the marker
// is repeatedly advanced to "now" with second granularity; a mutation
// landing in the same second as the previous marker would be skipped
// under a strict `>`, silently failing to persist — the exact data-loss
// class write-through exists to close. Inclusive `>=` re-scans the
// current second instead; fileSink dedupes byte-identical lines so the
// re-scan never rewrites an already-exported row-version.

// Result summarises an Export run.
type Result struct {
	FilesWritten []string `json:"files_written"`
	RowsWritten  int      `json:"rows_written"`
}

// Export reads rows from every replicated planning table that have
// been modified since the last export marker, and appends per-table
// per-month SQL upsert files under <vault>/.pql/changelog/.
//
// The set of replicated tables is fixed (tickets, ticket_deps,
// ticket_labels, ticket_history). Decisions and decision_refs are
// not exported — they are derived from markdown per D-8 and travel
// with the markdown source files.
//
// Marker semantics: rows with updated_at >= marker get exported (see the
// note on markerFormat for why inclusive). The marker advances to "now"
// at the end of a successful run. First export against an unset marker
// treats the marker as the empty string, which sorts before every real
// timestamp — so all rows are exported. fileSink skips byte-identical
// lines already present in the target file, so re-scanning a row that was
// exported in a prior run (the cost of inclusive marker semantics) never
// produces a duplicate changelog line.
func Export(ctx context.Context, db *sql.DB, vaultPath string) (*Result, error) {
	since, err := repo.ReadMeta(ctx, db, repo.MetaLastExportMarker)
	if err != nil {
		return nil, err
	}

	sink := &fileSink{
		vaultPath: vaultPath,
		files:     make(map[string]*os.File),
		seen:      make(map[string]map[string]bool),
	}
	defer sink.close()

	res := &Result{}
	for _, exp := range tableExporters {
		n, err := exp(ctx, db, since, sink)
		if err != nil {
			return nil, err
		}
		res.RowsWritten += n
	}

	for path := range sink.files {
		res.FilesWritten = append(res.FilesWritten, path)
	}
	sort.Strings(res.FilesWritten)

	if err := repo.WriteMeta(ctx, db, repo.MetaLastExportMarker,
		time.Now().UTC().Format(markerFormat)); err != nil {
		return nil, err
	}
	return res, nil
}

// fileSink lazily opens append-mode handles for the per-table
// per-month SQL files an Export pass writes to. It dedupes byte-identical
// lines: each target file's existing lines are loaded into seen[path] on
// first touch, so a line that already exists in the file (e.g. a row
// re-scanned under the inclusive marker) is skipped rather than appended.
type fileSink struct {
	vaultPath string
	files     map[string]*os.File
	seen      map[string]map[string]bool
}

// appendLine writes line to the table's monthly file unless an identical
// line is already present. It returns whether a line was actually written
// so callers can count only real writes.
func (fs *fileSink) appendLine(table, yearMonth, line string) (bool, error) {
	path := filepath.Join(fs.vaultPath, ChangelogDir, table, yearMonth+".sql")
	f, ok := fs.files[path]
	if !ok {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: changelog dir is meant to be world-readable when committed
			return false, fmt.Errorf("changelog: mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := fs.loadSeen(path); err != nil {
			return false, err
		}
		var err error
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G302: changelog files are committed to git
		if err != nil {
			return false, fmt.Errorf("changelog: open %s: %w", path, err)
		}
		fs.files[path] = f
	}
	if fs.seen[path][line] {
		return false, nil
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		return false, fmt.Errorf("changelog: write %s: %w", path, err)
	}
	fs.seen[path][line] = true
	return true, nil
}

// loadSeen reads the existing statements of path (if any) into seen[path]
// so appendLine can skip duplicates. A missing file starts with an empty
// set. Statements are split quote-aware, not by physical line: a row whose
// value (e.g. a ticket description) contains a newline spans several
// physical lines but is one statement, and must dedupe as a whole.
func (fs *fileSink) loadSeen(path string) error {
	set := make(map[string]bool)
	fs.seen[path] = set
	b, err := os.ReadFile(path) //nolint:gosec // G304: path is a resolved changelog file under the vault
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("changelog: read %s: %w", path, err)
	}
	for _, stmt := range splitStatements(string(b)) {
		set[stmt] = true
	}
	return nil
}

// splitStatements breaks a changelog file into the individual SQL upsert
// statements the exporter emitted. It splits on a `;` that falls outside a
// single-quoted string literal (SQL escapes an embedded quote by doubling
// it), so semicolons and newlines inside a value don't break a statement.
// Each returned statement is whitespace-trimmed, matching the exact line
// appendLine writes (which ends in `;` with no surrounding whitespace).
func splitStatements(content string) []string {
	var stmts []string
	var b strings.Builder
	inStr := false
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '\'' {
			if inStr && i+1 < len(content) && content[i+1] == '\'' {
				// Escaped quote ('') — stays inside the string literal.
				b.WriteByte(c)
				b.WriteByte(content[i+1])
				i++
				continue
			}
			inStr = !inStr
			b.WriteByte(c)
			continue
		}
		b.WriteByte(c)
		if c == ';' && !inStr {
			if stmt := strings.TrimSpace(b.String()); stmt != "" {
				stmts = append(stmts, stmt)
			}
			b.Reset()
		}
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

func (fs *fileSink) close() {
	for _, f := range fs.files {
		_ = f.Close()
	}
}

// Every replicated row carries an inline last-writer-wins guard (D-16):
// `WHERE excluded.updated_at >= <table>.updated_at`. A strictly newer row wins,
// and an exact tie resolves in favour of whichever statement is applied later —
// which is the row appended later in the changelog, since replay walks tables,
// files and lines in sorted order.
//
// Position, not content hash. The guard originally broke ties on
// `excluded.hash > <table>.hash`, an ordering that is arbitrary with respect to
// causality: a ticket created and mutated inside one timestamp tick produced two
// rows whose winner depended on which content happened to sort higher, so replay
// could revert the row to its created state on every fresh clone and branch
// switch (T-59, and T-1057 in settled-reach — a description lost for a month).
// Append order within a file *is* causal order, and unlike sub-second precision
// it also repairs rows already written. The hash remains on the row for identity
// and dedupe; it no longer decides recency.
type tableExporter func(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error)

var tableExporters = []tableExporter{
	exportTickets,
	exportTicketIDMap,
	exportTicketDeps,
	exportTicketLabels,
	exportTicketHistory,
}

// exportTicketIDMap emits the record_id ↔ friendly ticket_id mapping
// (D-26). ticket_id is mutable (relabel), so the LWW UPDATE carries it.
func exportTicketIDMap(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT record_id, ticket_id,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM ticket_idmap
		WHERE updated_at >= ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query ticket_idmap: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			recordID, ticketID, createdAt, updatedAt string
			deletedAt, hash                          sql.NullString
			canonicalVersion                         sql.NullInt64
		)
		if err := rows.Scan(&recordID, &ticketID, &createdAt, &updatedAt,
			&deletedAt, &hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket_idmap: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line, err := renderRow("ticket_idmap", []string{
			sqlStr(recordID), sqlStr(ticketID),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		})
		if err != nil {
			return n, err
		}
		wrote, err := sink.appendLine("ticket_idmap", ym, line)
		if err != nil {
			return n, err
		}
		if wrote {
			n++
		}
	}
	return n, rows.Err()
}

// exportTickets emits one INSERT … ON CONFLICT(id) DO UPDATE … WHERE …
// line per row in tickets that has been touched since the marker.
// LWW guard makes the line idempotent under replay (D-16).
func exportTickets(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT record_id, type, parent_record_id, title, description, status, priority,
		       assigned_to, team, decision_ref,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM tickets
		WHERE updated_at >= ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			recordID, typ, title, status, priority, createdAt, updatedAt       string
			parentRecID, description, assignedTo, team, decisionRef, deletedAt sql.NullString
			hash                                                               sql.NullString
			canonicalVersion                                                   sql.NullInt64
		)
		if err := rows.Scan(&recordID, &typ, &parentRecID, &title, &description,
			&status, &priority, &assignedTo, &team, &decisionRef,
			&createdAt, &updatedAt, &deletedAt, &hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line, err := renderRow("tickets", []string{
			sqlStr(recordID), sqlStr(typ), sqlNullStr(parentRecID), sqlStr(title), sqlNullStr(description),
			sqlStr(status), sqlStr(priority),
			sqlNullStr(assignedTo), sqlNullStr(team), sqlNullStr(decisionRef),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		})
		if err != nil {
			return n, err
		}
		wrote, err := sink.appendLine("tickets", ym, line)
		if err != nil {
			return n, err
		}
		if wrote {
			n++
		}
	}
	return n, rows.Err()
}

func exportTicketDeps(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT blocker_record_id, blocked_record_id,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM ticket_deps
		WHERE updated_at >= ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query ticket_deps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			blocker, blocked, createdAt, updatedAt string
			deletedAt, hash                        sql.NullString
			canonicalVersion                       sql.NullInt64
		)
		if err := rows.Scan(&blocker, &blocked, &createdAt, &updatedAt,
			&deletedAt, &hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket_dep: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line, err := renderRow("ticket_deps", []string{
			sqlStr(blocker), sqlStr(blocked),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		})
		if err != nil {
			return n, err
		}
		wrote, err := sink.appendLine("ticket_deps", ym, line)
		if err != nil {
			return n, err
		}
		if wrote {
			n++
		}
	}
	return n, rows.Err()
}

func exportTicketLabels(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_record_id, label,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM ticket_labels
		WHERE updated_at >= ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query ticket_labels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			ticketRecID, label, createdAt, updatedAt string
			deletedAt, hash                          sql.NullString
			canonicalVersion                         sql.NullInt64
		)
		if err := rows.Scan(&ticketRecID, &label, &createdAt, &updatedAt,
			&deletedAt, &hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket_label: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line, err := renderRow("ticket_labels", []string{
			sqlStr(ticketRecID), sqlStr(label),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		})
		if err != nil {
			return n, err
		}
		wrote, err := sink.appendLine("ticket_labels", ym, line)
		if err != nil {
			return n, err
		}
		if wrote {
			n++
		}
	}
	return n, rows.Err()
}

// exportTicketHistory emits append-only audit rows. ticket_history
// has no natural primary key — UNIQUE(hash) on the column lets replay
// dedupe identical events idempotently via ON CONFLICT(hash) DO
// NOTHING. There is no LWW WHERE clause because audit rows don't
// mutate.
func exportTicketHistory(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_record_id, field, old_value, new_value, changed_by,
		       changed_at, created_at, updated_at, deleted_at,
		       hash, canonical_version
		FROM ticket_history
		WHERE updated_at >= ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query ticket_history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			ticketRecID, field, changedAt, createdAt, updatedAt string
			oldVal, newVal, changedBy, deletedAt, hash          sql.NullString
			canonicalVersion                                    sql.NullInt64
		)
		if err := rows.Scan(&ticketRecID, &field, &oldVal, &newVal, &changedBy,
			&changedAt, &createdAt, &updatedAt, &deletedAt,
			&hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket_history: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line, err := renderRow("ticket_history", []string{
			sqlStr(ticketRecID), sqlStr(field),
			sqlNullStr(oldVal), sqlNullStr(newVal), sqlNullStr(changedBy),
			sqlStr(changedAt), sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		})
		if err != nil {
			return n, err
		}
		wrote, err := sink.appendLine("ticket_history", ym, line)
		if err != nil {
			return n, err
		}
		if wrote {
			n++
		}
	}
	return n, rows.Err()
}
