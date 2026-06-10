// Command migrate-ids is a one-off, local-only migrator for the D-26 ticket
// identity split. It rewrites a vault's committed .pql/changelog/ from the
// old format (T-NNN as the tickets primary key / FK target) to the new one
// (a ULID record_id is the identity; T-NNN lives in ticket_idmap).
//
// It is intentionally NOT part of the pql binary (only ~3 known vaults need
// it; per the no-defensive-migration policy a shipped command would be dead
// weight). Run it once per vault, then commit the changelog and rebuild:
//
//	go run ./cmd/migrate-ids <vault-path>
//	cd <vault> && git add .pql/changelog && git commit && rm .pql/pql.db && pql plan rebuild
//
// It replays the old changelog into an old-schema temp DB, mints a record_id
// per ticket, rebuilds a new-schema temp DB (remapping every reference and
// recomputing hashes), then wipes and re-exports the changelog fresh.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/changelog"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate-ids <vault-path>")
		os.Exit(2)
	}
	if err := migrate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate-ids:", err)
		os.Exit(1)
	}
}

type oldTicket struct {
	id, typ, title, status, priority, createdAt, updatedAt          string
	parentID, description, assignedTo, team, decisionRef, deletedAt sql.NullString
}
type oldDep struct {
	blocker, blocked, createdAt, updatedAt string
	deletedAt                              sql.NullString
}
type oldLabel struct {
	ticketID, label, createdAt, updatedAt string
	deletedAt                             sql.NullString
}
type oldHistory struct {
	ticketID, field, changedAt, createdAt, updatedAt string
	oldVal, newVal, changedBy, deletedAt             sql.NullString
}

func migrate(vault string) error {
	ctx := context.Background()
	clRoot := filepath.Join(vault, ".pql", "changelog")
	if _, err := os.Stat(clRoot); err != nil { //nolint:gosec // G703: clRoot is the user-supplied local vault path; this is a one-off migration tool
		return fmt.Errorf("no changelog at %s: %w", clRoot, err)
	}

	oldDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}
	defer func() { _ = oldDB.Close() }()
	if _, err := oldDB.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	if err := replayOldChangelog(ctx, oldDB, clRoot); err != nil {
		return fmt.Errorf("replay old changelog: %w", err)
	}

	tickets, deps, labels, history, err := readOld(ctx, oldDB)
	if err != nil {
		return err
	}

	// Mint a stable record_id per old ticket id.
	recOf := make(map[string]string, len(tickets))
	for _, t := range tickets {
		rid, err := planning.NewRecordID()
		if err != nil {
			return err
		}
		recOf[t.id] = rid
	}

	newDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}
	defer func() { _ = newDB.Close() }()
	if err := planning.Migrate(ctx, newDB); err != nil {
		return err
	}
	if _, err := newDB.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	if err := writeNew(ctx, newDB, recOf, tickets, deps, labels, history); err != nil {
		return fmt.Errorf("write new-schema rows: %w", err)
	}

	// Wipe and re-export the changelog in the new format.
	if err := os.RemoveAll(clRoot); err != nil { //nolint:gosec // G703: clRoot is the user-supplied local vault path
		return err
	}
	if err := writeSchemaFiles(clRoot); err != nil {
		return err
	}
	if _, err := changelog.Export(ctx, newDB, vault); err != nil {
		return fmt.Errorf("export new changelog: %w", err)
	}
	fmt.Printf("migrated %s: %d tickets, %d deps, %d labels, %d history rows\n",
		vault, len(tickets), len(deps), len(labels), len(history))
	return nil
}

func replayOldChangelog(ctx context.Context, db *sql.DB, root string) error {
	tableDirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var dirs []string
	for _, d := range tableDirs {
		if d.IsDir() {
			dirs = append(dirs, d.Name())
		}
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		files, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			return err
		}
		var names []string
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".sql") {
				names = append(names, f.Name())
			}
		}
		sort.Strings(names) // 0000-schema.sql first, then YYYY-MM
		for _, n := range names {
			b, err := os.ReadFile(filepath.Join(root, dir, n)) //nolint:gosec // G304: path is from walking the vault's own changelog dir
			if err != nil {
				return err
			}
			//nolint:gosec // G701: replaying the vault's own committed changelog SQL is the tool's purpose
			if _, err := db.ExecContext(ctx, string(b)); err != nil {
				return fmt.Errorf("%s/%s: %w", dir, n, err)
			}
		}
	}
	return nil
}

func readOld(ctx context.Context, db *sql.DB) (ts []oldTicket, ds []oldDep, ls []oldLabel, hs []oldHistory, err error) {
	rows, err := db.QueryContext(ctx, `SELECT id, type, parent_id, title, description, status, priority,
		assigned_to, team, decision_ref, created_at, updated_at, deleted_at FROM tickets`)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for rows.Next() {
		var t oldTicket
		if err = rows.Scan(&t.id, &t.typ, &t.parentID, &t.title, &t.description, &t.status, &t.priority,
			&t.assignedTo, &t.team, &t.decisionRef, &t.createdAt, &t.updatedAt, &t.deletedAt); err != nil {
			_ = rows.Close()
			return
		}
		ts = append(ts, t)
	}
	_ = rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT blocker_id, blocked_id, created_at, updated_at, deleted_at FROM ticket_deps`)
	if err != nil {
		return
	}
	for rows.Next() {
		var d oldDep
		if err = rows.Scan(&d.blocker, &d.blocked, &d.createdAt, &d.updatedAt, &d.deletedAt); err != nil {
			_ = rows.Close()
			return
		}
		ds = append(ds, d)
	}
	_ = rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT ticket_id, label, created_at, updated_at, deleted_at FROM ticket_labels`)
	if err != nil {
		return
	}
	for rows.Next() {
		var l oldLabel
		if err = rows.Scan(&l.ticketID, &l.label, &l.createdAt, &l.updatedAt, &l.deletedAt); err != nil {
			_ = rows.Close()
			return
		}
		ls = append(ls, l)
	}
	_ = rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT ticket_id, field, old_value, new_value, changed_by,
		changed_at, created_at, updated_at, deleted_at FROM ticket_history`)
	if err != nil {
		return
	}
	for rows.Next() {
		var h oldHistory
		if err = rows.Scan(&h.ticketID, &h.field, &h.oldVal, &h.newVal, &h.changedBy,
			&h.changedAt, &h.createdAt, &h.updatedAt, &h.deletedAt); err != nil {
			_ = rows.Close()
			return
		}
		hs = append(hs, h)
	}
	_ = rows.Close()
	return ts, ds, ls, hs, rows.Err()
}

// remap returns the record_id for an old id, erroring if a referenced id has
// no ticket (a corrupt changelog) so the migration fails loudly.
func remap(recOf map[string]string, oldID string) (string, error) {
	if r, ok := recOf[oldID]; ok {
		return r, nil
	}
	return "", fmt.Errorf("dangling reference to unknown ticket %q", oldID)
}

func writeNew(ctx context.Context, db *sql.DB, recOf map[string]string,
	ts []oldTicket, ds []oldDep, ls []oldLabel, hs []oldHistory) error {
	for _, t := range ts {
		rec := recOf[t.id]
		var parentRec sql.NullString
		if t.parentID.Valid {
			pr, err := remap(recOf, t.parentID.String)
			if err != nil {
				return err
			}
			parentRec = sql.NullString{String: pr, Valid: true}
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO tickets
			(record_id, type, parent_record_id, title, description, status, priority,
			 assigned_to, team, decision_ref, created_at, updated_at, deleted_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			rec, t.typ, parentRec, t.title, t.description, t.status, t.priority,
			t.assignedTo, t.team, t.decisionRef, t.createdAt, t.updatedAt, t.deletedAt); err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO ticket_idmap
			(record_id, ticket_id, created_at, updated_at) VALUES (?,?,?,?)`,
			rec, t.id, t.createdAt, t.updatedAt); err != nil {
			return err
		}
		if err := planning.RehashTicket(ctx, db, rec); err != nil {
			return err
		}
		if err := planning.RehashTicketIDMap(ctx, db, rec); err != nil {
			return err
		}
	}
	for _, d := range ds {
		br, err := remap(recOf, d.blocker)
		if err != nil {
			return err
		}
		bd, err := remap(recOf, d.blocked)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO ticket_deps
			(blocker_record_id, blocked_record_id, created_at, updated_at, deleted_at)
			VALUES (?,?,?,?,?)`, br, bd, d.createdAt, d.updatedAt, d.deletedAt); err != nil {
			return err
		}
		if err := planning.RehashTicketDep(ctx, db, br, bd); err != nil {
			return err
		}
	}
	for _, l := range ls {
		rec, err := remap(recOf, l.ticketID)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO ticket_labels
			(ticket_record_id, label, created_at, updated_at, deleted_at) VALUES (?,?,?,?,?)`,
			rec, l.label, l.createdAt, l.updatedAt, l.deletedAt); err != nil {
			return err
		}
		if err := planning.RehashTicketLabel(ctx, db, rec, l.label); err != nil {
			return err
		}
	}
	for _, h := range hs {
		rec, err := remap(recOf, h.ticketID)
		if err != nil {
			return err
		}
		res, err := db.ExecContext(ctx, `INSERT INTO ticket_history
			(ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			rec, h.field, h.oldVal, h.newVal, h.changedBy, h.changedAt, h.createdAt, h.updatedAt, h.deletedAt)
		if err != nil {
			return err
		}
		rowid, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if err := planning.RehashTicketHistory(ctx, db, rowid); err != nil {
			return err
		}
	}
	return nil
}

// writeSchemaFiles plants the new-format 0000-schema.sql in each replicated
// table dir, matching what `pql init` writes (markers + planning.Schema()).
func writeSchemaFiles(clRoot string) error {
	header := "-- Auto-generated by migrate-ids (D-26). CREATE TABLE statements\n" +
		"-- for the planning schema; per-table dir keeps the changelog\n" +
		"-- self-describing per D-15.\n" +
		"-- pql:created_by: migrate-ids\n" +
		"-- pql:canonical_version: " + strconv.Itoa(planning.CanonicalVersion) + "\n\n" +
		planning.Schema()
	for _, table := range []string{"tickets", "ticket_idmap", "ticket_deps", "ticket_labels", "ticket_history"} {
		dir := filepath.Join(clRoot, table)
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: changelog dirs are committed and world-readable
			return err
		}
		//nolint:gosec // G306: schema file is committed to git and world-readable
		if err := os.WriteFile(filepath.Join(dir, "0000-schema.sql"), []byte(header), 0o644); err != nil {
			return err
		}
	}
	return nil
}
