package repo

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/postmeridiem/pql/internal/planning"

	_ "modernc.org/sqlite"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("FK: %v", err)
	}
	if err := planning.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func writeDecision(t *testing.T, dir, file, content string) {
	t.Helper()
	p := filepath.Join(dir, file)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

func TestSyncDecisions(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	dir := t.TempDir()
	root := dir

	writeDecision(t, dir, "architecture.md", `# Architecture

### D-001: CLI-first
- **Date:** 2026-04-20
- **Decision:** Use CLI.
- **Cross-reference:** [D-002](architecture.md#d-002)

### D-002: Feature layout
- **Date:** 2026-04-21
- **Decision:** Feature-first.
`)

	result, err := SyncDecisions(ctx, db, dir, root)
	if err != nil {
		t.Fatalf("SyncDecisions: %v", err)
	}
	if result.Synced != 2 {
		t.Errorf("synced = %d, want 2", result.Synced)
	}
	if result.Refs != 1 {
		t.Errorf("refs = %d, want 1", result.Refs)
	}

	// List
	decs, err := ListDecisions(ctx, db, DecisionFilter{})
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(decs) != 2 {
		t.Errorf("list count = %d, want 2", len(decs))
	}

	// Get
	d, err := GetDecision(ctx, db, "D-001")
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if d == nil {
		t.Fatal("D-001 not found")
	}
	if d.Title != "CLI-first" {
		t.Errorf("title = %q, want CLI-first", d.Title)
	}

	// Not found
	d, err = GetDecision(ctx, db, "D-999")
	if err != nil {
		t.Fatalf("GetDecision D-999: %v", err)
	}
	if d != nil {
		t.Error("expected nil for D-999")
	}

	// Refs
	refs, err := RefsOf(ctx, db, "D-001")
	if err != nil {
		t.Fatalf("RefsOf: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("refs count = %d, want 1", len(refs))
	}

	// Coverage (all should be gaps since no tickets)
	gaps, err := Coverage(ctx, db)
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	if len(gaps) != 2 {
		t.Errorf("coverage gaps = %d, want 2", len(gaps))
	}
}

func TestSyncDecisions_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	dir := t.TempDir()

	writeDecision(t, dir, "arch.md", `### D-001: Test
- **Date:** 2026-01-01
`)

	r1, err := SyncDecisions(ctx, db, dir, dir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}

	r2, err := SyncDecisions(ctx, db, dir, dir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}

	if r1.Synced != r2.Synced {
		t.Errorf("synced mismatch: %d vs %d", r1.Synced, r2.Synced)
	}
}

func TestListDecisions_Filters(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	dir := t.TempDir()

	writeDecision(t, dir, "arch.md", `### D-001: A
- **Date:** 2026-01-01
`)
	writeDecision(t, dir, "questions-arch.md", `### Q-001: B
- **Status:** Open
`)
	if _, err := SyncDecisions(ctx, db, dir, dir); err != nil {
		t.Fatalf("sync: %v", err)
	}

	decs, err := ListDecisions(ctx, db, DecisionFilter{Type: "confirmed"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(decs) != 1 || decs[0].ID != "D-001" {
		t.Errorf("filter type=confirmed: got %d records", len(decs))
	}

	decs, err = ListDecisions(ctx, db, DecisionFilter{Domain: "arch"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(decs) != 2 {
		t.Errorf("filter domain=arch: got %d records, want 2", len(decs))
	}
}
