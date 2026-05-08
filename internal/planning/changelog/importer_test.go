package changelog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

func TestImport_EmptyChangelogIsNoOp(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	res, err := Import(ctx, db, vault)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.StatementsRun != 0 {
		t.Errorf("statements = %d, want 0", res.StatementsRun)
	}
	if len(res.FilesReplayed) != 0 {
		t.Errorf("files = %v, want 0", res.FilesReplayed)
	}
}

func TestImport_RoundTripWithExport(t *testing.T) {
	ctx := context.Background()

	// Source replica: seed two tickets, export to changelog.
	srcVault, srcDB := setupVault(t)
	seedTicket(t, srcDB, "T-1", "2025-05-08 11:00:00")
	seedTicket(t, srcDB, "T-2", "2025-05-08 11:01:00")
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Destination replica: copy the changelog dir, import into a
	// fresh pql.db, verify both rows landed.
	dstVault, dstDB := setupVault(t)
	copyTree(t,
		filepath.Join(srcVault, ".pql", "changelog"),
		filepath.Join(dstVault, ".pql", "changelog"),
	)
	res, err := Import(ctx, dstDB, dstVault)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.StatementsRun != 2 {
		t.Errorf("statements = %d, want 2", res.StatementsRun)
	}
	for _, id := range []string{"T-1", "T-2"} {
		var got string
		if err := dstDB.QueryRowContext(ctx,
			`SELECT id FROM tickets WHERE id = ?`, id,
		).Scan(&got); err != nil {
			t.Errorf("ticket %s missing after import: %v", id, err)
		}
	}
}

func TestImport_IsIdempotent(t *testing.T) {
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

	// Replay twice; each statement executes again but the LWW guard
	// short-circuits the UPDATE branch since the row's updated_at and
	// hash haven't moved. End state is identical to a single replay.
	if _, err := Import(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Import 1: %v", err)
	}
	// Reset marker so the second pass replays the file again.
	if err := repo.WriteMeta(ctx, dstDB, repo.MetaLastImportMarker, ""); err != nil {
		t.Fatalf("reset marker: %v", err)
	}
	if _, err := Import(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Import 2: %v", err)
	}

	var n int
	if err := dstDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("ticket count = %d, want 1 (replay should not duplicate)", n)
	}
}

func TestImport_AdvancesMarker(t *testing.T) {
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
	if _, err := Import(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Import: %v", err)
	}
	marker, err := repo.ReadMeta(ctx, dstDB, repo.MetaLastImportMarker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker == "" {
		t.Error("marker not advanced after import")
	}
}

func TestImport_LWWGuardPreventsStaleOverwrite(t *testing.T) {
	ctx := context.Background()
	srcVault, srcDB := setupVault(t)
	// Seed with old updated_at, export.
	seedTicket(t, srcDB, "T-1", "2025-04-01 10:00:00")
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Destination has the same ticket but with a NEWER updated_at —
	// simulates a replica that already saw a later edit.
	dstVault, dstDB := setupVault(t)
	if _, err := dstDB.ExecContext(ctx, `
		INSERT INTO tickets (id, type, title, status, priority, created_at, updated_at)
		VALUES ('T-1', 'task', 'newer-title', 'done', 'high',
		        '2025-05-01 10:00:00', '2025-05-01 10:00:00')
	`); err != nil {
		t.Fatalf("seed dst: %v", err)
	}

	copyTree(t,
		filepath.Join(srcVault, ".pql", "changelog"),
		filepath.Join(dstVault, ".pql", "changelog"),
	)
	if _, err := Import(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Import: %v", err)
	}

	var title, status string
	if err := dstDB.QueryRowContext(ctx,
		`SELECT title, status FROM tickets WHERE id = 'T-1'`,
	).Scan(&title, &status); err != nil {
		t.Fatalf("read: %v", err)
	}
	if title != "newer-title" || status != "done" {
		t.Errorf("LWW guard failed — older import overwrote newer state: title=%q status=%q",
			title, status)
	}
}

func TestImport_RefusesMismatchedCanonicalVersion(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	// Plant a per-table dir with a schema file that declares an
	// impossible canonical_version (999) so the importer must refuse.
	tableDir := filepath.Join(vault, ".pql", "changelog", "tickets")
	if err := os.MkdirAll(tableDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	schemaBody := "-- pql:created_by: 99.99.99\n" +
		"-- pql:canonical_version: 999\n" +
		"CREATE TABLE IF NOT EXISTS tickets (id TEXT PRIMARY KEY);\n"
	schemaPath := filepath.Join(tableDir, "0000-schema.sql")
	if err := os.WriteFile(schemaPath, []byte(schemaBody), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chtimes(schemaPath, time.Now(), time.Now()); err != nil {
		t.Fatalf("touch: %v", err)
	}

	_, err := Import(ctx, db, vault)
	if err == nil {
		t.Fatal("expected schema-drift refusal, got nil")
	}
	if !strings.Contains(err.Error(), "canonical_version") {
		t.Errorf("error should reference canonical_version mismatch: %v", err)
	}
}

// copyTree mirrors a directory tree under dst. Used to simulate a
// replica pulling the source replica's changelog.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path) //nolint:gosec // G304: walking a test temp tree
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644) //nolint:gosec // G306: test fixture
	}); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	// Touch the files so their mtime is "now" — Import's marker check
	// is mtime-based, and copies on some filesystems preserve mtime.
	now := time.Now()
	if err := filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		return os.Chtimes(path, now, now)
	}); err != nil {
		t.Fatalf("touch: %v", err)
	}
}
