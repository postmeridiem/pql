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
