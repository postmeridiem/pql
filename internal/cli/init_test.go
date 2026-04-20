package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDefaultConfig_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pql", "config.yaml")
	stat, err := writeDefaultConfig(path, false)
	if err != nil {
		t.Fatalf("writeDefaultConfig: %v", err)
	}
	if !stat.Created || stat.Overwritten {
		t.Errorf("stat = %#v, want Created=true Overwritten=false", stat)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "frontmatter: yaml") {
		t.Errorf("default config missing expected key: %s", body)
	}
}

func TestWriteDefaultConfig_ExistingSkippedWithoutForce(t *testing.T) {
	// Idempotent: existing .pql/config.yaml is preserved (Skipped=true), not
	// overwritten and not errored.
	dir := t.TempDir()
	path := filepath.Join(dir, ".pql", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("frontmatter: toml\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stat, err := writeDefaultConfig(path, false)
	if err != nil {
		t.Fatalf("expected no error (idempotent), got %v", err)
	}
	if !stat.Skipped || stat.Created || stat.Overwritten {
		t.Errorf("stat = %#v, want Skipped=true (others false)", stat)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "toml") {
		t.Errorf("existing config should be untouched: %s", body)
	}
}

func TestWriteDefaultConfig_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pql", "config.yaml")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("frontmatter: toml\n"), 0o644)

	stat, err := writeDefaultConfig(path, true)
	if err != nil {
		t.Fatalf("writeDefaultConfig: %v", err)
	}
	if !stat.Overwritten || stat.Created {
		t.Errorf("stat = %#v, want Overwritten=true", stat)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "frontmatter: yaml") {
		t.Errorf("--force did not overwrite: %s", body)
	}
}

func TestEnsureGitignoreEntry_NoGitignoreNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	stat, err := ensureGitignoreEntry(path, ".pql/")
	if err != nil {
		t.Fatalf("ensureGitignoreEntry: %v", err)
	}
	if stat.Exists || stat.Appended {
		t.Errorf("stat = %#v, want Exists=false Appended=false", stat)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("gitignore should not have been created: %v", err)
	}
}

func TestEnsureGitignoreEntry_AppendsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	os.WriteFile(path, []byte("node_modules/\n*.log\n"), 0o644)

	stat, err := ensureGitignoreEntry(path, ".pql/")
	if err != nil {
		t.Fatalf("ensureGitignoreEntry: %v", err)
	}
	if !stat.Exists || !stat.Appended || stat.Entry != ".pql/" {
		t.Errorf("stat = %#v, want Exists=true Appended=true Entry=.pql/", stat)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), ".pql/") {
		t.Errorf("entry not appended: %s", body)
	}
	if !strings.HasSuffix(string(body), ".pql/\n") {
		t.Errorf("expected trailing newline after entry: %q", body)
	}
}

func TestEnsureGitignoreEntry_AlreadyPresentNoOp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	original := "node_modules/\n.pql/\n*.log\n"
	os.WriteFile(path, []byte(original), 0o644)

	stat, err := ensureGitignoreEntry(path, ".pql/")
	if err != nil {
		t.Fatalf("ensureGitignoreEntry: %v", err)
	}
	if stat.Appended {
		t.Errorf("should not have appended (already present); stat = %#v", stat)
	}
	body, _ := os.ReadFile(path)
	if string(body) != original {
		t.Errorf("file was modified despite no-op: %q", body)
	}
}

func TestEnsureGitignoreEntry_RecognisesVariantSpellings(t *testing.T) {
	cases := []string{
		".pql",
		".pql/",
		"/.pql",
		"/.pql/",
	}
	for _, existing := range cases {
		t.Run(existing, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".gitignore")
			os.WriteFile(path, []byte(existing+"\n"), 0o644)

			stat, err := ensureGitignoreEntry(path, ".pql/")
			if err != nil {
				t.Fatalf("ensureGitignoreEntry: %v", err)
			}
			if stat.Appended {
				t.Errorf("treated %q as missing — should match", existing)
			}
		})
	}
}

func TestEnsureGitignoreEntry_AddsTrailingNewlineWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	// No trailing newline on existing content.
	os.WriteFile(path, []byte("node_modules/"), 0o644)

	if _, err := ensureGitignoreEntry(path, ".pql/"); err != nil {
		t.Fatalf("ensureGitignoreEntry: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "node_modules/\n.pql/\n") {
		t.Errorf("did not insert separating newline: %q", body)
	}
}
