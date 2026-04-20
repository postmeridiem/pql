package index

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/store"
)

// --- helpers --------------------------------------------------------------

// newTestEnv spins up a vault directory with a paired open store. Caller
// gets back the vault path, the live store, and the loaded config.
func newTestEnv(t *testing.T) (vault string, st *store.Store, cfg *config.Config) {
	t.Helper()
	vault = t.TempDir()
	cfg, err := config.Load(config.LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	st, err = store.Open(context.Background(), cfg.DBPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return vault, st, cfg
}

func writeNote(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// --- tests ----------------------------------------------------------------

func TestIndexer_Run_PopulatesAllTables(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	writeNote(t, vault, "members/vaasa.md", `---
type: council-member
name: Vaasa
voting: true
tags: [council-member, voting]
---
# Vaasa

She links to [[Holt]] and embeds ![[chart.png]].

## Background

#strategic
`)
	idx := New(st, cfg)
	stats, err := idx.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Walked != 1 || stats.Indexed != 1 || stats.Unchanged != 0 || stats.Removed != 0 {
		t.Errorf("stats = %+v, want Walked=1 Indexed=1", stats)
	}

	db := st.DB()
	if n := countRows(t, db, `SELECT count(*) FROM files`); n != 1 {
		t.Errorf("files count = %d", n)
	}
	if n := countRows(t, db, `SELECT count(*) FROM frontmatter WHERE path = ?`, "members/vaasa.md"); n != 4 {
		t.Errorf("frontmatter row count = %d, want 4 (type/name/voting/tags)", n)
	}
	if n := countRows(t, db, `SELECT count(*) FROM tags WHERE path = ?`, "members/vaasa.md"); n != 3 {
		t.Errorf("tags count = %d, want 3 (council-member, voting, strategic)", n)
	}
	if n := countRows(t, db, `SELECT count(*) FROM links WHERE source_path = ?`, "members/vaasa.md"); n != 2 {
		t.Errorf("links count = %d, want 2 (Holt + chart.png embed)", n)
	}
	if n := countRows(t, db, `SELECT count(*) FROM headings WHERE path = ?`, "members/vaasa.md"); n != 2 {
		t.Errorf("headings count = %d, want 2", n)
	}
}

func TestIndexer_Run_TypesFrontmatterCorrectly(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	writeNote(t, vault, "x.md", `---
name: Vaasa
voting: true
seat: 4
score: 3.14
prior_jobs: [analyst, lecturer]
---
body
`)
	idx := New(st, cfg)
	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	cases := []struct {
		key      string
		wantType string
	}{
		{"name", "string"},
		{"voting", "bool"},
		{"seat", "number"},
		{"score", "number"},
		{"prior_jobs", "list"},
	}
	for _, c := range cases {
		var got string
		err := st.DB().QueryRow(
			`SELECT type FROM frontmatter WHERE path = ? AND key = ?`,
			"x.md", c.key,
		).Scan(&got)
		if err != nil {
			t.Errorf("%s: %v", c.key, err)
			continue
		}
		if got != c.wantType {
			t.Errorf("%s: type = %q, want %q", c.key, got, c.wantType)
		}
	}
}

func TestIndexer_Run_IsIncrementalOnUnchangedFile(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	writeNote(t, vault, "stable.md", "# stable\n")
	idx := New(st, cfg)

	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	stats, err := idx.Run(context.Background())
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if stats.Unchanged != 1 || stats.Indexed != 0 {
		t.Errorf("second run stats = %+v, want Unchanged=1 Indexed=0", stats)
	}
}

func TestIndexer_Run_DetectsContentChange(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	writeNote(t, vault, "a.md", "# original\n")
	idx := New(st, cfg)
	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Bump mtime forward AND change content so the indexer picks it up.
	// Use os.Chtimes to set mtime explicitly — Unix mtime resolution is
	// 1 second, so a sub-second sleep would let the second write fall
	// inside the same mtime tick and the indexer would (correctly) skip.
	writeNote(t, vault, "a.md", "# modified\n\n[[NewLink]]\n")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(vault, "a.md"), future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	stats, err := idx.Run(context.Background())
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if stats.Indexed != 1 {
		t.Errorf("expected 1 indexed file after content change, got %+v", stats)
	}
	if n := countRows(t, st.DB(), `SELECT count(*) FROM links WHERE source_path = ? AND target_path = ?`, "a.md", "NewLink"); n != 1 {
		t.Errorf("expected NewLink in links table after re-extract, got %d rows", n)
	}
}

func TestIndexer_Run_PrunesDeletedFiles(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	writeNote(t, vault, "keep.md", "")
	writeNote(t, vault, "ephemeral.md", "")
	idx := New(st, cfg)
	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if n := countRows(t, st.DB(), `SELECT count(*) FROM files`); n != 2 {
		t.Fatalf("expected 2 files after first run, got %d", n)
	}

	// Delete one file from disk.
	if err := os.Remove(filepath.Join(vault, "ephemeral.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	stats, err := idx.Run(context.Background())
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if stats.Removed != 1 {
		t.Errorf("expected Removed=1, got %+v", stats)
	}
	if n := countRows(t, st.DB(), `SELECT count(*) FROM files`); n != 1 {
		t.Errorf("files count after prune = %d, want 1", n)
	}
}

func TestIndexer_Run_ExcludesHonoured(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	cfg.Exclude = append(cfg.Exclude, "drafts/**")
	writeNote(t, vault, "keep.md", "")
	writeNote(t, vault, "drafts/wip.md", "")

	idx := New(st, cfg)
	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := countRows(t, st.DB(), `SELECT count(*) FROM files`); n != 1 {
		t.Errorf("files count = %d, want 1 (drafts excluded)", n)
	}
}

func TestIndexer_Run_RecordsConfigHashAndScanTime(t *testing.T) {
	vault, st, cfg := newTestEnv(t)
	writeNote(t, vault, "x.md", "")
	idx := New(st, cfg)
	if _, err := idx.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	wantHash, _ := cfg.Hash()
	var gotHash string
	if err := st.DB().QueryRow(`SELECT value FROM index_meta WHERE key='config_hash'`).Scan(&gotHash); err != nil {
		t.Fatalf("read config_hash: %v", err)
	}
	if gotHash != wantHash {
		t.Errorf("config_hash = %q, want %q", gotHash, wantHash)
	}
	var lastScan string
	if err := st.DB().QueryRow(`SELECT value FROM index_meta WHERE key='last_full_scan'`).Scan(&lastScan); err != nil {
		t.Fatalf("read last_full_scan: %v", err)
	}
	if lastScan == "" {
		t.Error("last_full_scan should be set")
	}
}

func TestIndexer_Run_CouncilSnapshot(t *testing.T) {
	vault := repoTestdataPath(t, "council-snapshot")
	cfg, err := config.Load(config.LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
		// In-vault .pql/ creation is the default; our snapshot fixture is
		// writable so it'll land at testdata/council-snapshot/.pql/. We
		// override DB to a temp file to keep the fixture clean.
		DBFlag: filepath.Join(t.TempDir(), "council.sqlite"),
	})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	st, err := store.Open(context.Background(), cfg.DBPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	idx := New(st, cfg)
	stats, err := idx.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Indexed < 30 {
		t.Errorf("expected at least 30 files indexed from Council, got %+v", stats)
	}

	// Spot-check: vaasa's persona has the expected frontmatter type tags.
	var typeName, typeFM string
	if err := st.DB().QueryRow(
		`SELECT type FROM frontmatter WHERE path = ? AND key = 'type'`,
		"members/vaasa/persona.md",
	).Scan(&typeFM); err != nil {
		t.Errorf("vaasa type lookup: %v", err)
	}
	if typeFM != "string" {
		t.Errorf("vaasa frontmatter.type column for key 'type' = %q, want string", typeFM)
	}
	if err := st.DB().QueryRow(
		`SELECT type FROM frontmatter WHERE path = ? AND key = 'name'`,
		"members/vaasa/persona.md",
	).Scan(&typeName); err != nil {
		t.Errorf("vaasa name lookup: %v", err)
	}
	if typeName != "string" {
		t.Errorf("vaasa frontmatter.type column for key 'name' = %q, want string", typeName)
	}

	// Spot-check tags + links counts are non-zero.
	if n := countRows(t, st.DB(), `SELECT count(*) FROM tags`); n == 0 {
		t.Error("expected some tags from Council snapshot, got 0")
	}
	if n := countRows(t, st.DB(), `SELECT count(*) FROM links`); n == 0 {
		t.Error("expected some links from Council snapshot, got 0")
	}
	if n := countRows(t, st.DB(), `SELECT count(*) FROM headings`); n == 0 {
		t.Error("expected some headings from Council snapshot, got 0")
	}
}
