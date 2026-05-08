package planning

import (
	"context"
	"database/sql"
	"strings"
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
	for _, want := range []string{
		"decisions", "decision_refs",
		"tickets", "ticket_deps", "ticket_history", "ticket_labels",
	} {
		if !tables[want] {
			t.Errorf("missing table %q", want)
		}
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

// TestMigrate_FreshHasReplicationColumns confirms every planning table
// carries the full replication column set (created_at, updated_at,
// deleted_at, hash, canonical_version) from the first CREATE — no
// ALTER deltas, per D-19.
func TestMigrate_FreshHasReplicationColumns(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	want := []string{"created_at", "updated_at", "deleted_at", "hash", "canonical_version"}
	for _, table := range []string{
		"decisions", "decision_refs", "tickets",
		"ticket_deps", "ticket_history", "ticket_labels",
	} {
		got := tableColumns(t, db, table)
		for _, col := range want {
			if !got[col] {
				t.Errorf("%s missing column %q", table, col)
			}
		}
	}
}

// TestMigrate_RefusesIncompleteSchema simulates a pql.db built under
// an older shape — the pre-T-17 schema, missing every replication
// column — and confirms Migrate refuses to proceed with a clear
// recovery message. Per D-19, no in-place upgrade.
func TestMigrate_RefusesIncompleteSchema(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)
	// Pre-replication schema: everything users see, but no hash,
	// canonical_version, or deleted_at columns. CREATE TABLE IF NOT
	// EXISTS in Migrate will be a no-op (tables exist), so verifySchema
	// is what trips.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE decisions (
			id TEXT PRIMARY KEY, type TEXT NOT NULL, domain TEXT NOT NULL,
			title TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
			date TEXT, file_path TEXT NOT NULL,
			synced_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE decision_refs (
			source_id TEXT NOT NULL, target_id TEXT NOT NULL,
			ref_type TEXT NOT NULL, note TEXT,
			PRIMARY KEY (source_id, target_id, ref_type)
		);
		CREATE TABLE tickets (
			id TEXT PRIMARY KEY, type TEXT NOT NULL,
			parent_id TEXT, title TEXT NOT NULL, description TEXT,
			status TEXT NOT NULL DEFAULT 'backlog',
			priority TEXT DEFAULT 'medium',
			assigned_to TEXT, team TEXT, decision_ref TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE ticket_deps (
			blocker_id TEXT NOT NULL, blocked_id TEXT NOT NULL,
			PRIMARY KEY (blocker_id, blocked_id)
		);
		CREATE TABLE ticket_history (
			ticket_id TEXT NOT NULL, field TEXT NOT NULL,
			old_value TEXT, new_value TEXT, changed_by TEXT,
			changed_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE ticket_labels (
			ticket_id TEXT NOT NULL, label TEXT NOT NULL,
			PRIMARY KEY (ticket_id, label)
		);
	`); err != nil {
		t.Fatalf("seed pre-replication schema: %v", err)
	}
	err := Migrate(ctx, db)
	if err == nil {
		t.Fatal("expected migrate to refuse incomplete schema, got nil")
	}
	if !strings.Contains(err.Error(), "earlier schema") {
		t.Errorf("error message should hint at earlier schema; got %q", err)
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
