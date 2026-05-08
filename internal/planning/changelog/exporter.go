package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
// Marker semantics: rows with updated_at > marker get exported. The
// marker advances to "now" at the end of a successful run. First
// export against an unset marker treats the marker as the empty
// string, which sorts before every real timestamp — so all rows are
// exported.
func Export(ctx context.Context, db *sql.DB, vaultPath string) (*Result, error) {
	since, err := repo.ReadMeta(ctx, db, repo.MetaLastExportMarker)
	if err != nil {
		return nil, err
	}

	sink := &fileSink{vaultPath: vaultPath, files: make(map[string]*os.File)}
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
// per-month SQL files an Export pass writes to.
type fileSink struct {
	vaultPath string
	files     map[string]*os.File
}

func (fs *fileSink) appendLine(table, yearMonth, line string) error {
	path := filepath.Join(fs.vaultPath, ChangelogDir, table, yearMonth+".sql")
	f, ok := fs.files[path]
	if !ok {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: changelog dir is meant to be world-readable when committed
			return fmt.Errorf("changelog: mkdir %s: %w", filepath.Dir(path), err)
		}
		var err error
		f, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G302: changelog files are committed to git
		if err != nil {
			return fmt.Errorf("changelog: open %s: %w", path, err)
		}
		fs.files[path] = f
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("changelog: write %s: %w", path, err)
	}
	return nil
}

func (fs *fileSink) close() {
	for _, f := range fs.files {
		_ = f.Close()
	}
}

type tableExporter func(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error)

var tableExporters = []tableExporter{
	exportTickets,
	exportTicketDeps,
	exportTicketLabels,
	exportTicketHistory,
}

// exportTickets emits one INSERT … ON CONFLICT(id) DO UPDATE … WHERE …
// line per row in tickets that has been touched since the marker.
// LWW guard makes the line idempotent under replay (D-16).
func exportTickets(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, parent_id, title, description, status, priority,
		       assigned_to, team, decision_ref,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM tickets
		WHERE updated_at > ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			id, typ, title, status, priority, createdAt, updatedAt           string
			parentID, description, assignedTo, team, decisionRef, deletedAt  sql.NullString
			hash                                                             sql.NullString
			canonicalVersion                                                 sql.NullInt64
		)
		if err := rows.Scan(&id, &typ, &parentID, &title, &description,
			&status, &priority, &assignedTo, &team, &decisionRef,
			&createdAt, &updatedAt, &deletedAt, &hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line := fmt.Sprintf(
			`INSERT INTO tickets (id, type, parent_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) ON CONFLICT(id) DO UPDATE SET type=excluded.type, parent_id=excluded.parent_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at > tickets.updated_at OR (excluded.updated_at = tickets.updated_at AND excluded.hash > tickets.hash);`,
			sqlStr(id), sqlStr(typ), sqlNullStr(parentID), sqlStr(title), sqlNullStr(description),
			sqlStr(status), sqlStr(priority),
			sqlNullStr(assignedTo), sqlNullStr(team), sqlNullStr(decisionRef),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		)
		if err := sink.appendLine("tickets", ym, line); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func exportTicketDeps(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT blocker_id, blocked_id,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM ticket_deps
		WHERE updated_at > ?
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
		line := fmt.Sprintf(
			`INSERT INTO ticket_deps (blocker_id, blocked_id, created_at, updated_at, deleted_at, hash, canonical_version) VALUES (%s, %s, %s, %s, %s, %s, %s) ON CONFLICT(blocker_id, blocked_id) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at > ticket_deps.updated_at OR (excluded.updated_at = ticket_deps.updated_at AND excluded.hash > ticket_deps.hash);`,
			sqlStr(blocker), sqlStr(blocked),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		)
		if err := sink.appendLine("ticket_deps", ym, line); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func exportTicketLabels(ctx context.Context, db *sql.DB, since string, sink *fileSink) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_id, label,
		       created_at, updated_at, deleted_at, hash, canonical_version
		FROM ticket_labels
		WHERE updated_at > ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query ticket_labels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			ticketID, label, createdAt, updatedAt string
			deletedAt, hash                       sql.NullString
			canonicalVersion                      sql.NullInt64
		)
		if err := rows.Scan(&ticketID, &label, &createdAt, &updatedAt,
			&deletedAt, &hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket_label: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line := fmt.Sprintf(
			`INSERT INTO ticket_labels (ticket_id, label, created_at, updated_at, deleted_at, hash, canonical_version) VALUES (%s, %s, %s, %s, %s, %s, %s) ON CONFLICT(ticket_id, label) DO UPDATE SET updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at > ticket_labels.updated_at OR (excluded.updated_at = ticket_labels.updated_at AND excluded.hash > ticket_labels.hash);`,
			sqlStr(ticketID), sqlStr(label),
			sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		)
		if err := sink.appendLine("ticket_labels", ym, line); err != nil {
			return n, err
		}
		n++
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
		SELECT ticket_id, field, old_value, new_value, changed_by,
		       changed_at, created_at, updated_at, deleted_at,
		       hash, canonical_version
		FROM ticket_history
		WHERE updated_at > ?
		ORDER BY updated_at, hash
	`, since)
	if err != nil {
		return 0, fmt.Errorf("changelog: query ticket_history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := 0
	for rows.Next() {
		var (
			ticketID, field, changedAt, createdAt, updatedAt string
			oldVal, newVal, changedBy, deletedAt, hash       sql.NullString
			canonicalVersion                                 sql.NullInt64
		)
		if err := rows.Scan(&ticketID, &field, &oldVal, &newVal, &changedBy,
			&changedAt, &createdAt, &updatedAt, &deletedAt,
			&hash, &canonicalVersion); err != nil {
			return n, fmt.Errorf("changelog: scan ticket_history: %w", err)
		}
		ym, err := monthOf(updatedAt)
		if err != nil {
			return n, err
		}
		line := fmt.Sprintf(
			`INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) ON CONFLICT(hash) DO NOTHING;`,
			sqlStr(ticketID), sqlStr(field),
			sqlNullStr(oldVal), sqlNullStr(newVal), sqlNullStr(changedBy),
			sqlStr(changedAt), sqlStr(createdAt), sqlStr(updatedAt), sqlNullStr(deletedAt),
			sqlNullStr(hash), sqlNullInt(canonicalVersion),
		)
		if err := sink.appendLine("ticket_history", ym, line); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}
