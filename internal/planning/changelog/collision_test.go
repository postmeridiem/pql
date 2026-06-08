package changelog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/postmeridiem/pql/internal/planning"
)

func TestParseTicketLineage_RoundTripsNastyTitle(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	// A title and description loaded with the characters that would break a
	// naive comma/quote split: single quotes, commas, parentheses, newline.
	title := "fix (a,b) — it's broken"
	desc := "line one, with comma\nline two: it's (really) nested, see ON CONFLICT(id) DO UPDATE"
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tickets (id, type, title, description, status, priority, created_at, updated_at)
		VALUES ('T-7','task',?,?,'backlog','medium','2025-05-08 11:00:00','2025-05-08 11:00:00')
	`, title, desc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := planning.RehashTicket(ctx, db, "T-7"); err != nil {
		t.Fatalf("rehash: %v", err)
	}
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	content := readTicketChangelog(t, vault)
	var parsed bool
	for _, stmt := range splitStatements(content) {
		lin, id, ok := parseTicketLineage(stmt)
		if !ok {
			continue
		}
		parsed = true
		if id != "T-7" {
			t.Errorf("id = %q, want T-7", id)
		}
		if lin.Title != title {
			t.Errorf("title = %q, want %q", lin.Title, title)
		}
		if lin.CreatedAt != "2025-05-08 11:00:00" {
			t.Errorf("created_at = %q, want 2025-05-08 11:00:00", lin.CreatedAt)
		}
		if len(lin.Hash) != 32 {
			t.Errorf("hash = %q, want a 32-char md5 hex", lin.Hash)
		}
	}
	if !parsed {
		t.Fatal("parseTicketLineage never matched the exported ticket statement — prefix drift?")
	}
}

func TestDetectTicketCollisions_Clean(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00")
	seedTicket(t, db, "T-2", "2025-05-08 11:01:00")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	cols, err := detectTicketCollisions(filepath.Join(vault, ".pql", "changelog"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 0 {
		t.Fatalf("clean changelog: got %d collisions, want 0: %+v", len(cols), cols)
	}
}

func TestDetectTicketCollisions_FlagsTwoLineages(t *testing.T) {
	ctx := context.Background()

	// Lineage A: T-3 created in May.
	vA, dbA := setupVault(t)
	seedTicket(t, dbA, "T-3", "2025-05-08 11:00:00")
	if _, err := Export(ctx, dbA, vA); err != nil {
		t.Fatalf("export A: %v", err)
	}

	// Lineage B: a *different* T-3, created in June (different created_at +
	// title) — the classic cross-clone collision from the T-54 context.
	vB, dbB := setupVault(t)
	if _, err := dbB.ExecContext(ctx, `
		INSERT INTO tickets (id, type, title, status, priority, created_at, updated_at)
		VALUES ('T-3','task','UNRELATED other ticket','backlog','medium','2025-06-09 09:00:00','2025-06-09 09:00:00')
	`); err != nil {
		t.Fatalf("seed B: %v", err)
	}
	if err := planning.RehashTicket(ctx, dbB, "T-3"); err != nil {
		t.Fatalf("rehash B: %v", err)
	}
	if _, err := Export(ctx, dbB, vB); err != nil {
		t.Fatalf("export B: %v", err)
	}

	// Merge B's tickets files into A's changelog. updated_at differs by
	// month (May vs June) so the files don't share a name and coexist.
	mergeTicketChangelog(t, vB, vA)

	cols, err := detectTicketCollisions(filepath.Join(vA, ".pql", "changelog"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("got %d collisions, want 1: %+v", len(cols), cols)
	}
	c := cols[0]
	if c.ID != "T-3" {
		t.Errorf("collision id = %q, want T-3", c.ID)
	}
	if len(c.Lineages) != 2 {
		t.Fatalf("lineages = %d, want 2: %+v", len(c.Lineages), c.Lineages)
	}
	// Two distinct created_at values, ordered first-seen (May before June).
	if c.Lineages[0].CreatedAt == c.Lineages[1].CreatedAt {
		t.Errorf("lineages share created_at %q — not a real collision", c.Lineages[0].CreatedAt)
	}
}

func TestSplitValuesTuple_RespectsQuotesAndParens(t *testing.T) {
	// Two top-level fields: a string literal containing a comma, a paren and
	// an escaped quote, then NULL.
	fields, ok := splitValuesTuple("'a, (b) it''s', NULL)")
	if !ok {
		t.Fatal("tuple not terminated")
	}
	if len(fields) != 2 {
		t.Fatalf("fields = %v, want 2", fields)
	}
	if got := unquoteSQL(fields[0]); got != "a, (b) it's" {
		t.Errorf("field 0 = %q, want %q", got, "a, (b) it's")
	}
	if got := unquoteSQL(fields[1]); got != "" {
		t.Errorf("field 1 (NULL) = %q, want empty", got)
	}
}

// --- helpers ---

func readTicketChangelog(t *testing.T, vault string) string {
	t.Helper()
	dir := filepath.Join(vault, ".pql", "changelog", "tickets")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read tickets dir: %v", err)
	}
	var all string
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		all += string(b)
	}
	return all
}

// mergeTicketChangelog copies every tickets/*.sql file from src's
// changelog into dst's, leaving dst's existing files in place.
func mergeTicketChangelog(t *testing.T, srcVault, dstVault string) {
	t.Helper()
	srcDir := filepath.Join(srcVault, ".pql", "changelog", "tickets")
	dstDir := filepath.Join(dstVault, ".pql", "changelog", "tickets")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read src tickets: %v", err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, e.Name()), b, 0o644); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}
