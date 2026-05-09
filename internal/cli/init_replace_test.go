package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceHookBlock_ReplacesExistingBlock(t *testing.T) {
	existing := "#!/bin/sh\n" +
		"# --- pql plan import ---\n" +
		"OLD BODY LINE 1\n" +
		"OLD BODY LINE 2\n" +
		"# --- end pql ---\n" +
		"# user customization\n"
	newBlock := "# --- pql plan import ---\n" +
		"NEW BODY LINE 1\n" +
		"# --- end pql ---\n"

	got, replaced := replaceHookBlock(existing, "# --- pql plan import ---", newBlock)
	if !replaced {
		t.Fatal("expected replaced=true")
	}
	if strings.Contains(got, "OLD BODY LINE") {
		t.Errorf("old body survived replacement:\n%s", got)
	}
	if !strings.Contains(got, "NEW BODY LINE 1") {
		t.Errorf("new body missing:\n%s", got)
	}
	if !strings.Contains(got, "# user customization") {
		t.Errorf("user customization stripped:\n%s", got)
	}
	if !strings.HasPrefix(got, "#!/bin/sh\n") {
		t.Errorf("shebang clobbered:\n%s", got)
	}
}

func TestInstallNamedHook_UpgradesExistingHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	hookPath := filepath.Join(dir, ".pql", "hooks", "post-merge")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o750); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	// Plant an old-style hook with a stale body but the current marker.
	old := "#!/bin/sh\n" +
		"# --- pql plan import ---\n" +
		"echo OLD STALE LOGIC\n" +
		"# --- end pql ---\n" +
		"# user wrote this\n"
	if err := os.WriteFile(hookPath, []byte(old), 0o750); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stat := installNamedHook(dir, "post-merge", pqlPostMergeMarker,
		renderPostMergeHook("/path/to/pql"))
	if !stat.Installed {
		t.Errorf("upgrade should set Installed=true: %#v", stat)
	}
	got, _ := os.ReadFile(hookPath)
	if strings.Contains(string(got), "OLD STALE LOGIC") {
		t.Errorf("stale body survived upgrade:\n%s", got)
	}
	if !strings.Contains(string(got), "/path/to/pql") {
		t.Errorf("new body missing:\n%s", got)
	}
	if !strings.Contains(string(got), "# user wrote this") {
		t.Errorf("user customization stripped:\n%s", got)
	}
}

// TestEnsurePqlGitignore_AppendsChangelogException regresses T-27:
// an existing .gitignore that has /.pql/* + !/.pql/pql-plan.json +
// !/.pql/hooks/ but lacks !/.pql/changelog/ should pick up the
// missing entry on pql init.
func TestEnsurePqlGitignore_AppendsChangelogException(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	pre := "# stuff\n.pql/*\n!.pql/pql-plan.json\n!.pql/hooks/\n"
	if err := os.WriteFile(path, []byte(pre), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stat, err := ensurePqlGitignore(path)
	if err != nil {
		t.Fatalf("ensurePqlGitignore: %v", err)
	}
	if !stat.Appended {
		t.Errorf("expected Appended=true, got %#v", stat)
	}

	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "!.pql/changelog/") {
		t.Errorf("missing !.pql/changelog/ entry:\n%s", body)
	}

	// Idempotent: a second pass with the entry already present must
	// not duplicate it.
	stat2, err := ensurePqlGitignore(path)
	if err != nil {
		t.Fatalf("second ensurePqlGitignore: %v", err)
	}
	if stat2.Appended {
		t.Errorf("second pass should be no-op, got %#v", stat2)
	}
	body2, _ := os.ReadFile(path)
	if strings.Count(string(body2), "!.pql/changelog/") != 1 {
		t.Errorf("entry duplicated on re-run:\n%s", body2)
	}
}

func TestReplaceHookBlock_NoMarkerReturnsUnchanged(t *testing.T) {
	existing := "#!/bin/sh\necho hello\n"
	got, replaced := replaceHookBlock(existing, "# --- pql plan import ---", "x")
	if replaced {
		t.Error("expected replaced=false")
	}
	if got != existing {
		t.Errorf("content modified despite no marker")
	}
}
