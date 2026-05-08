package changelog

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

// setupVault creates a temp dir, opens an :memory: pql.db, runs
// Migrate, and returns (vaultPath, db). The vaultPath is the dir that
// will hold .pql/changelog/ after Export runs.
func setupVault(t *testing.T) (string, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := planning.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return dir, db
}

func TestSQLStr_EscapesSingleQuote(t *testing.T) {
	got := sqlStr("it's")
	want := "'it''s'"
	if got != want {
		t.Errorf("sqlStr(%q) = %q, want %q", "it's", got, want)
	}
}

func TestSQLNullStr_NilRendersNULL(t *testing.T) {
	if got := sqlNullStr(sql.NullString{}); got != "NULL" {
		t.Errorf("nil NullString = %q, want NULL", got)
	}
	if got := sqlNullStr(sql.NullString{String: "x", Valid: true}); got != "'x'" {
		t.Errorf("valid NullString = %q, want 'x'", got)
	}
}

func TestSQLNullInt_NilRendersNULL(t *testing.T) {
	if got := sqlNullInt(sql.NullInt64{}); got != "NULL" {
		t.Errorf("nil NullInt = %q, want NULL", got)
	}
	if got := sqlNullInt(sql.NullInt64{Int64: 42, Valid: true}); got != "42" {
		t.Errorf("valid NullInt = %q, want 42", got)
	}
}

func TestMonthOf(t *testing.T) {
	got, err := monthOf("2026-05-08 09:54:33")
	if err != nil {
		t.Fatalf("monthOf: %v", err)
	}
	if got != "2026-05" {
		t.Errorf("monthOf = %q, want 2026-05", got)
	}
}

func TestMonthOf_RejectsShort(t *testing.T) {
	if _, err := monthOf("2026"); err == nil {
		t.Error("expected error for short timestamp, got nil")
	}
}

// seedTicket inserts a ticket with explicit timestamps and computes
// the hash so the row matches what the exporter will read.
func seedTicket(t *testing.T, db *sql.DB, id, updatedAt string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tickets (id, type, title, status, priority, created_at, updated_at)
		VALUES (?, 'task', ?, 'backlog', 'medium', ?, ?)
	`, id, "title-"+id, updatedAt, updatedAt); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
	if err := planning.RehashTicket(ctx, db, id); err != nil {
		t.Fatalf("rehash %s: %v", id, err)
	}
}

func TestExport_WritesPerTablePerMonthFiles(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	seedTicket(t, db, "T-1", "2025-04-15 10:00:00")
	seedTicket(t, db, "T-2", "2025-05-08 11:00:00")
	seedTicket(t, db, "T-3", "2025-05-08 11:00:01")

	res, err := Export(ctx, db, vault)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.RowsWritten != 3 {
		t.Errorf("rows = %d, want 3", res.RowsWritten)
	}
	want := []string{
		filepath.Join(vault, ".pql", "changelog", "tickets", "2025-04.sql"),
		filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"),
	}
	if len(res.FilesWritten) != len(want) {
		t.Fatalf("files = %v, want %v", res.FilesWritten, want)
	}
	for i, p := range want {
		if res.FilesWritten[i] != p {
			t.Errorf("file[%d] = %q, want %q", i, res.FilesWritten[i], p)
		}
	}
}

func TestExport_LinesAreDeterministic(t *testing.T) {
	ctx := context.Background()
	vault1, db1 := setupVault(t)
	vault2, db2 := setupVault(t)

	for _, db := range []*sql.DB{db1, db2} {
		seedTicket(t, db, "T-1", "2025-05-08 11:00:00")
	}

	if _, err := Export(ctx, db1, vault1); err != nil {
		t.Fatalf("Export 1: %v", err)
	}
	if _, err := Export(ctx, db2, vault2); err != nil {
		t.Fatalf("Export 2: %v", err)
	}

	read := func(p string) []byte {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		return b
	}
	a := read(filepath.Join(vault1, ".pql", "changelog", "tickets", "2025-05.sql"))
	b := read(filepath.Join(vault2, ".pql", "changelog", "tickets", "2025-05.sql"))
	if !bytes.Equal(a, b) {
		t.Errorf("file contents differ across replicas:\nA:\n%s\nB:\n%s", a, b)
	}
}

func TestExport_OrdersByUpdatedAtThenHash(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	// Same updated_at; hash provides deterministic tiebreaker. Different
	// titles yield different hashes; we just need stability across runs.
	seedTicket(t, db, "T-2", "2025-05-08 11:00:00")
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00")
	seedTicket(t, db, "T-3", "2025-05-08 11:00:00")

	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), body)
	}
	// Re-running export against an empty marker would re-emit the
	// same lines in the same order — verify by reset-marker + re-export.
	if err := repo.WriteMeta(ctx, db, repo.MetaLastExportMarker, ""); err != nil {
		t.Fatalf("reset marker: %v", err)
	}
	// Truncate and re-export.
	if err := os.Truncate(filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"), 0); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("re-export: %v", err)
	}
	body2, err := os.ReadFile(filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"))
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if !bytes.Equal(body, body2) {
		t.Errorf("non-deterministic ordering across runs")
	}
}

func TestExport_SkipsDecisionsTable(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	// Seed a decision (not a ticket) — should not appear in changelog.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO decisions (id, type, domain, title, file_path, created_at, updated_at)
		VALUES ('D-1', 'confirmed', 'arch', 't', 'a.md',
		        '2025-05-08 11:00:00', '2025-05-08 11:00:00')
	`); err != nil {
		t.Fatalf("seed decision: %v", err)
	}
	if err := planning.RehashDecision(ctx, db, "D-1"); err != nil {
		t.Fatalf("rehash decision: %v", err)
	}

	res, err := Export(ctx, db, vault)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.RowsWritten != 0 {
		t.Errorf("rows = %d, want 0 (decisions not replicated)", res.RowsWritten)
	}
	if _, err := os.Stat(filepath.Join(vault, ".pql", "changelog", "decisions")); !os.IsNotExist(err) {
		t.Errorf("decisions changelog dir should not exist: err=%v", err)
	}
}

func TestExport_AdvancesMarker(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	seedTicket(t, db, "T-1", "2025-05-08 11:00:00")

	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export 1: %v", err)
	}
	marker, err := repo.ReadMeta(ctx, db, repo.MetaLastExportMarker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker == "" {
		t.Error("marker not advanced after export")
	}

	// A second export with no new rows produces zero output.
	res, err := Export(ctx, db, vault)
	if err != nil {
		t.Fatalf("Export 2: %v", err)
	}
	if res.RowsWritten != 0 {
		t.Errorf("rows = %d, want 0 on idle re-export", res.RowsWritten)
	}
}

func TestExport_IncludesSoftDeletedRows(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	seedTicket(t, db, "T-1", "2025-05-08 11:00:00")
	// Soft-delete (sets deleted_at), bumps updated_at.
	if _, err := db.ExecContext(ctx, `
		UPDATE tickets SET deleted_at = '2025-05-08 12:00:00',
		                   updated_at = '2025-05-08 12:00:00'
		WHERE id = 'T-1'
	`); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := planning.RehashTicket(ctx, db, "T-1"); err != nil {
		t.Fatalf("rehash: %v", err)
	}

	res, err := Export(ctx, db, vault)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if res.RowsWritten == 0 {
		t.Error("soft-deleted row not exported")
	}
	body, err := os.ReadFile(filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "deleted_at") {
		t.Errorf("export line missing deleted_at column reference:\n%s", body)
	}
}
