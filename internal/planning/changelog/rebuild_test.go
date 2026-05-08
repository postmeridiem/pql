package changelog

import (
	"context"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

func TestRebuild_DropsThenReplays(t *testing.T) {
	ctx := context.Background()
	srcVault, srcDB := setupVault(t)
	seedTicket(t, srcDB, "T-1", "2025-05-08 11:00:00")
	seedTicket(t, srcDB, "T-2", "2025-05-08 11:01:00")
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	dstVault, dstDB := setupVault(t)
	copyTree(t,
		filepath.Join(srcVault, ".pql", "changelog"),
		filepath.Join(dstVault, ".pql", "changelog"),
	)
	res, err := Rebuild(ctx, dstDB, dstVault)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got, want := len(res.TablesCleared), len(replicatedTables); got != want {
		t.Errorf("tables cleared = %d, want %d", got, want)
	}

	var n int
	if err := dstDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("ticket count after rebuild = %d, want 2", n)
	}
}

// TestRebuild_RemovesRowsAbsentFromChangelog covers the load-bearing
// case from D-18: incremental replay can't delete rows that existed
// on a previous branch but not the new one. Rebuild's truncate-then-
// replay loop must.
func TestRebuild_RemovesRowsAbsentFromChangelog(t *testing.T) {
	ctx := context.Background()
	srcVault, srcDB := setupVault(t)
	seedTicket(t, srcDB, "T-1", "2025-05-08 11:00:00")
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Destination has its own ticket T-99 that the source's
	// changelog has never heard of — simulates a branch switch.
	dstVault, dstDB := setupVault(t)
	seedTicket(t, dstDB, "T-99", "2025-05-08 11:00:00")
	copyTree(t,
		filepath.Join(srcVault, ".pql", "changelog"),
		filepath.Join(dstVault, ".pql", "changelog"),
	)

	if _, err := Rebuild(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	// T-99 should be gone (not in the source changelog), T-1 should
	// be present (replayed from source changelog).
	var t99 int
	if err := dstDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tickets WHERE id = 'T-99'`,
	).Scan(&t99); err != nil {
		t.Fatalf("count T-99: %v", err)
	}
	if t99 != 0 {
		t.Errorf("T-99 survived rebuild — incremental replay can't remove rows")
	}
	var t1 int
	if err := dstDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tickets WHERE id = 'T-1'`,
	).Scan(&t1); err != nil {
		t.Fatalf("count T-1: %v", err)
	}
	if t1 != 1 {
		t.Errorf("T-1 missing after rebuild")
	}
}

func TestRebuild_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	srcVault, srcDB := setupVault(t)
	seedTicket(t, srcDB, "T-1", "2025-05-08 11:00:00")
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	dstVault, dstDB := setupVault(t)
	copyTree(t,
		filepath.Join(srcVault, ".pql", "changelog"),
		filepath.Join(dstVault, ".pql", "changelog"),
	)
	if _, err := Rebuild(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Rebuild 1: %v", err)
	}
	if _, err := Rebuild(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Rebuild 2: %v", err)
	}
	var n int
	if err := dstDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("ticket count after double rebuild = %d, want 1", n)
	}
}

func TestRebuild_LeavesDecisionsUntouched(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO decisions (id, type, domain, title, file_path)
		VALUES ('D-1', 'confirmed', 'arch', 't', 'a.md')
	`); err != nil {
		t.Fatalf("seed decision: %v", err)
	}
	if _, err := Rebuild(ctx, db, vault); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM decisions`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("decisions count = %d, want 1 — markdown-sourced rows must survive rebuild", n)
	}
}

func TestRebuild_ResetsImportMarker(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	// Pre-set a marker as if a previous import had run.
	if err := repo.WriteMeta(ctx, db, repo.MetaLastImportMarker, "2025-01-01 00:00:00"); err != nil {
		t.Fatalf("seed marker: %v", err)
	}
	if _, err := Rebuild(ctx, db, vault); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	// After rebuild Import will have written a fresh "now" marker —
	// what matters is that the previous value was reset before
	// replay, not that the post-rebuild marker is empty.
	got, err := repo.ReadMeta(ctx, db, repo.MetaLastImportMarker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if got == "2025-01-01 00:00:00" {
		t.Errorf("marker not reset before replay — old value survived rebuild")
	}
}
