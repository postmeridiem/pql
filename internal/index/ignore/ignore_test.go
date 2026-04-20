package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func writeIgnore(t *testing.T, vault, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(vault, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// --- Load -----------------------------------------------------------------

func TestLoad_NoFilesReturnsNoop(t *testing.T) {
	m, err := Load(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Matches("anything.md") {
		t.Errorf("noop matcher should never match")
	}
}

func TestLoad_AllNamedFilesMissingReturnsNoop(t *testing.T) {
	vault := t.TempDir()
	m, err := Load(vault, []string{".gitignore", ".pqlignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Matches("anything.md") {
		t.Errorf("matcher with no loaded rules should not match")
	}
}

func TestLoad_SingleFileMatchesPattern(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "node_modules/\n*.log\n")
	m, err := Load(vault, []string{".gitignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := []struct {
		path string
		want bool
	}{
		{"node_modules/foo.js", true},
		{"members/vaasa/persona.md", false},
		{"build.log", true},
		{"members/notes.md", false},
	}
	for _, c := range cases {
		if got := m.Matches(c.path); got != c.want {
			t.Errorf("Matches(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestLoad_MultipleFilesAreConcatenated(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "node_modules/\n")
	writeIgnore(t, vault, ".pqlignore", "drafts/\n")
	m, err := Load(vault, []string{".gitignore", ".pqlignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, path := range []string{"node_modules/x", "drafts/wip.md"} {
		if !m.Matches(path) {
			t.Errorf("Matches(%q) = false, want true", path)
		}
	}
	if m.Matches("members/persona.md") {
		t.Errorf("Matches(members/persona.md) = true, want false")
	}
}

func TestLoad_LaterFileCanReIncludeViaNegation(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "drafts/\n")
	writeIgnore(t, vault, ".pqlignore", "!drafts/published/\n")
	m, err := Load(vault, []string{".gitignore", ".pqlignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !m.Matches("drafts/wip.md") {
		t.Errorf("drafts/wip.md should still match")
	}
	if m.Matches("drafts/published/release-notes.md") {
		t.Errorf("drafts/published/* should be re-included by .pqlignore")
	}
}

func TestLoad_OrderMatters_FirstFileLoses(t *testing.T) {
	// Reverse the previous test's file order — now .pqlignore comes first
	// and its !-rule is overridden by .gitignore's later exclude.
	vault := t.TempDir()
	writeIgnore(t, vault, ".pqlignore", "!drafts/published/\n")
	writeIgnore(t, vault, ".gitignore", "drafts/\n")
	m, err := Load(vault, []string{".pqlignore", ".gitignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// .gitignore's drafts/ excludes the directory entirely, and gitignore's
	// directory short-circuit means the earlier !drafts/published/ can't
	// re-include files inside it.
	if !m.Matches("drafts/published/release-notes.md") {
		t.Errorf("expected drafts/published/* to be excluded when .gitignore comes last")
	}
}

func TestLoad_MissingFileSkippedAmongPresent(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "node_modules/\n")
	// .pqlignore intentionally not created.
	m, err := Load(vault, []string{".gitignore", ".pqlignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !m.Matches("node_modules/x") {
		t.Errorf(".gitignore rule should still apply when .pqlignore is missing")
	}
}

func TestLoad_CRLFLineEndings(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "node_modules/\r\n*.log\r\n")
	m, err := Load(vault, []string{".gitignore"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !m.Matches("node_modules/x") || !m.Matches("foo.log") {
		t.Errorf("CRLF-terminated rules not honoured")
	}
}

// --- pattern shape coverage ----------------------------------------------

func TestLoad_BasenamePattern(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "node_modules\n")
	m, _ := Load(vault, []string{".gitignore"})
	for _, p := range []string{"node_modules", "deep/path/node_modules"} {
		if !m.Matches(p) {
			t.Errorf("basename pattern should match %q", p)
		}
	}
}

func TestLoad_AnchoredPattern(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "/build\n")
	m, _ := Load(vault, []string{".gitignore"})
	if !m.Matches("build/x") {
		t.Errorf("anchored /build should match top-level build/")
	}
	if m.Matches("members/build/x") {
		t.Errorf("anchored /build should NOT match nested build/")
	}
}

func TestLoad_DoubleStar(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "**/scratch/\n")
	m, _ := Load(vault, []string{".gitignore"})
	for _, p := range []string{"scratch/x", "members/foo/scratch/x", "a/b/c/scratch/x"} {
		if !m.Matches(p) {
			t.Errorf("**/scratch/ should match %q", p)
		}
	}
}

func TestLoad_CommentLinesAndBlanks(t *testing.T) {
	vault := t.TempDir()
	writeIgnore(t, vault, ".gitignore", "# comment\n\nnode_modules/\n# another\n")
	m, _ := Load(vault, []string{".gitignore"})
	if !m.Matches("node_modules/x") {
		t.Errorf("expected node_modules/ to match alongside comments + blanks")
	}
}
