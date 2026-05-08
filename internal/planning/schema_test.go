package planning

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openMemory(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open :memory: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrate_Fresh(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	tables := queryTables(t, db)
	for _, want := range []string{"decisions", "tickets", "ticket_deps", "ticket_history", "ticket_labels", "decision_refs", "schema_migrations"} {
		if !tables[want] {
			t.Errorf("missing table %q", want)
		}
	}

	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&v); err != nil {
		t.Fatalf("read version: %v", err)
	}
	want := len(migrations)
	if v != want {
		t.Errorf("version = %d, want %d", v, want)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	want := len(migrations)
	if count != want {
		t.Errorf("schema_migrations rows = %d, want %d", count, want)
	}
}

func TestMigrate_ForeignKeys(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable FK: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO decision_refs (source_id, target_id, ref_type)
		 VALUES ('D-999', 'D-998', 'references')`)
	if err == nil {
		t.Fatal("expected FK violation, got nil")
	}
}

// TestMigrate_V2BackfillsHashes simulates a database that's already at
// v1 with real data, then runs Migrate (which applies v2) and confirms
// the hash + canonical_version columns are populated for the
// pre-existing row.
func TestMigrate_V2BackfillsHashes(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)

	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := db.ExecContext(ctx, migrationV1); err != nil {
		t.Fatalf("apply v1: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (1)`); err != nil {
		t.Fatalf("record v1: %v", err)
	}

	// Populate every table that v2 ALTERs so the ALTER + backfill path
	// is exercised against non-empty tables — this is the case that
	// regressed against a live db (datetime('now') as ALTER ADD COLUMN
	// default isn't allowed for populated tables).
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tickets (id, type, title, status, priority, created_at, updated_at)
		VALUES ('T-99', 'task', 'pre-v2 row', 'backlog', 'medium', datetime('now'), datetime('now'))
	`); err != nil {
		t.Fatalf("insert pre-v2 ticket: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO decisions (id, type, domain, title, file_path)
		VALUES ('D-99', 'confirmed', 'arch', 'pre-v2 decision', 'd.md')
	`); err != nil {
		t.Fatalf("insert pre-v2 decision: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO decision_refs (source_id, target_id, ref_type, note)
		VALUES ('D-99', 'D-99', 'references', NULL)
	`); err != nil {
		t.Fatalf("insert pre-v2 decision_ref: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_deps (blocker_id, blocked_id) VALUES ('T-99', 'T-99')
	`); err != nil {
		t.Fatalf("insert pre-v2 ticket_dep: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_labels (ticket_id, label) VALUES ('T-99', 'pre-v2')
	`); err != nil {
		t.Fatalf("insert pre-v2 ticket_label: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by)
		VALUES ('T-99', 'status', 'backlog', 'ready', 'pre-v2')
	`); err != nil {
		t.Fatalf("insert pre-v2 ticket_history: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate v1→v2: %v", err)
	}

	var hash sql.NullString
	var canonicalVersion sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT hash, canonical_version FROM tickets WHERE id = 'T-99'`,
	).Scan(&hash, &canonicalVersion); err != nil {
		t.Fatalf("read backfilled row: %v", err)
	}
	if !hash.Valid || hash.String == "" {
		t.Errorf("hash not backfilled: %+v", hash)
	}
	if !canonicalVersion.Valid || canonicalVersion.Int64 != int64(CanonicalVersion) {
		t.Errorf("canonical_version = %+v, want %d", canonicalVersion, CanonicalVersion)
	}
}

// TestMigrate_V2AddsColumnsToAllTables verifies that v2 added the
// hash + canonical_version columns to every planning table, plus
// created_at/updated_at to tables that lacked them.
func TestMigrate_V2AddsColumnsToAllTables(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	cases := []struct {
		table   string
		columns []string
	}{
		{"decisions", []string{"created_at", "updated_at", "hash", "canonical_version"}},
		{"decision_refs", []string{"created_at", "updated_at", "hash", "canonical_version"}},
		{"tickets", []string{"hash", "canonical_version"}},
		{"ticket_deps", []string{"created_at", "updated_at", "hash", "canonical_version"}},
		{"ticket_history", []string{"created_at", "updated_at", "hash", "canonical_version"}},
		{"ticket_labels", []string{"created_at", "updated_at", "hash", "canonical_version"}},
	}
	for _, c := range cases {
		got := tableColumns(t, db, c.table)
		for _, col := range c.columns {
			if !got[col] {
				t.Errorf("%s missing column %q", c.table, col)
			}
		}
	}
}

func tableColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	m := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		m[name] = true
	}
	return m
}

func queryTables(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query(
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	m := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		m[name] = true
	}
	return m
}
