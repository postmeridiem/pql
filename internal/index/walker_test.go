package index

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWalk_EmptyVault(t *testing.T) {
	got, err := Walk(WalkOpts{VaultPath: t.TempDir()})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestWalk_RequiresVaultPath(t *testing.T) {
	if _, err := Walk(WalkOpts{}); err == nil {
		t.Fatal("expected error for empty VaultPath, got nil")
	}
}

func TestWalk_OnlyMarkdownFiles(t *testing.T) {
	vault := t.TempDir()
	mkfile(t, vault, "a.md", "# a")
	mkfile(t, vault, "b.txt", "ignored")
	mkfile(t, vault, "c.md", "# c")
	mkfile(t, vault, "image.png", "binary")

	got, err := Walk(WalkOpts{VaultPath: vault})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := []string{"a.md", "c.md"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalk_BuiltinExcludesSkipped(t *testing.T) {
	vault := t.TempDir()
	mkfile(t, vault, "keep.md", "# keep")
	mkfile(t, vault, ".git/HEAD", "ref: refs/heads/main")
	mkfile(t, vault, ".git/notes.md", "should not be indexed")
	mkfile(t, vault, ".pql/index.sqlite", "binary")
	mkfile(t, vault, ".pql/notes.md", "should not be indexed")
	mkfile(t, vault, "data.sqlite", "binary")
	mkfile(t, vault, "data.sqlite-wal", "binary")

	got, err := Walk(WalkOpts{VaultPath: vault})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !slices.Equal(got, []string{"keep.md"}) {
		t.Errorf("got %v, want only keep.md", got)
	}
}

func TestWalk_CustomExcludePatterns(t *testing.T) {
	vault := t.TempDir()
	mkfile(t, vault, "keep.md", "# keep")
	mkfile(t, vault, "drafts/wip.md", "draft")
	mkfile(t, vault, "members/vaasa/persona.md", "person")
	mkfile(t, vault, "members/vaasa/scratch/notes.md", "scratch")

	got, err := Walk(WalkOpts{
		VaultPath: vault,
		Exclude:   []string{"drafts/**", "**/scratch/**"},
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := []string{"keep.md", "members/vaasa/persona.md"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalk_ExcludedDirsArePruned(t *testing.T) {
	// Sanity check that pruning works: putting many files inside an excluded
	// directory shouldn't change the result, and (more importantly) shouldn't
	// hit any kind of recursion guard.
	vault := t.TempDir()
	mkfile(t, vault, "keep.md", "# keep")
	for i := range 50 {
		mkfile(t, vault, filepath.Join(".git", "objects", "deep", "noisy", "f"+itoa(i)+".md"), "x")
	}
	got, err := Walk(WalkOpts{VaultPath: vault})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !slices.Equal(got, []string{"keep.md"}) {
		t.Errorf("got %v, want only keep.md (pruning of .git/ failed)", got)
	}
}

func TestWalk_SortedOutput(t *testing.T) {
	vault := t.TempDir()
	for _, name := range []string{"z.md", "a.md", "m.md", "members/c.md", "members/a.md"} {
		mkfile(t, vault, name, "")
	}
	got, err := Walk(WalkOpts{VaultPath: vault})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := []string{"a.md", "m.md", "members/a.md", "members/c.md", "z.md"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalk_ForwardSlashesOnAllPlatforms(t *testing.T) {
	vault := t.TempDir()
	mkfile(t, vault, filepath.Join("a", "b", "c.md"), "")
	got, err := Walk(WalkOpts{VaultPath: vault})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(got) != 1 || got[0] != "a/b/c.md" {
		t.Errorf("got %v, want [a/b/c.md] with forward slashes", got)
	}
}

func TestWalk_CouncilSnapshot(t *testing.T) {
	// End-to-end against the real fixture vault. This is the "would v0.1's
	// `pql files` find every Council member's persona?" smoke test.
	vault := repoTestdataPath(t, "council-snapshot")
	got, err := Walk(WalkOpts{
		VaultPath: vault,
		Exclude:   []string{"**/.obsidian/**", "**/node_modules/**"},
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(got) < 30 {
		t.Errorf("expected at least 30 .md files in Council snapshot, got %d", len(got))
	}
	// Spot-check: vaasa/persona.md must be there (used as a fixture in
	// docs/pql-grammar.md examples).
	if !slices.Contains(got, "members/vaasa/persona.md") {
		t.Errorf("members/vaasa/persona.md missing from walk output (sample: %v)", got[:min(5, len(got))])
	}
	// Sanity: no .base files leaked through (we only index .md).
	for _, p := range got {
		if filepath.Ext(p) != ".md" {
			t.Errorf("non-.md file in walk output: %s", p)
		}
	}
}

// --- helpers ---------------------------------------------------------------

func mkfile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// repoTestdataPath finds <repo>/testdata/<name> by walking up from the test
// file's working dir until it finds the project root.
func repoTestdataPath(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		candidate := filepath.Join(dir, "testdata", name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate testdata/%s starting from %s", name, wd)
		}
		dir = parent
	}
}
