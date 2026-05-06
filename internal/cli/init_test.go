package cli

import (
	"os"
	"os/exec"
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

func TestRenderPreCommitHook_BakesAbsolutePath(t *testing.T) {
	body := renderPreCommitHook("/usr/local/bin/pql")
	if !strings.Contains(body, "'/usr/local/bin/pql' plan export --stage") {
		t.Errorf("hook missing absolute path invocation:\n%s", body)
	}
	if strings.Contains(body, "command -v pql") {
		t.Error("hook should not rely on PATH-based command -v guard")
	}
}

func TestRenderPreCommitHook_EscapesSingleQuote(t *testing.T) {
	body := renderPreCommitHook("/tmp/odd'path/pql")
	want := `'/tmp/odd'\''path/pql' plan export --stage`
	if !strings.Contains(body, want) {
		t.Errorf("expected escaped single quote %q in:\n%s", want, body)
	}
}

func TestRenderPostMergeHook_BakesAbsolutePath(t *testing.T) {
	body := renderPostMergeHook("/usr/local/bin/pql")
	if !strings.Contains(body, "'/usr/local/bin/pql' plan export --to") {
		t.Errorf("hook missing absolute path for export:\n%s", body)
	}
	if !strings.Contains(body, "'/usr/local/bin/pql' plan import") {
		t.Errorf("hook missing absolute path for import:\n%s", body)
	}
	if strings.Contains(body, "command -v pql") {
		t.Error("post-merge hook should not rely on PATH-based command -v guard")
	}
}

func TestEnsurePlanExportHook_WritesAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	stat := ensurePlanExportHook(dir)
	if !stat.Installed {
		t.Fatalf("hook not installed: %#v", stat)
	}
	body, err := os.ReadFile(stat.Path)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	resolved := resolvePqlPath()
	want := "'" + resolved + "' plan export --stage"
	if !strings.Contains(string(body), want) {
		t.Errorf("hook missing %q:\n%s", want, body)
	}
}

func TestPlanExportStage_StagesUntrackedSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	// Need an identity for git add; a config-less repo refuses some ops.
	for k, v := range map[string]string{"user.email": "t@example.com", "user.name": "t"} {
		if out, err := exec.Command("git", "-C", dir, "config", k, v).CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %v: %s", k, err, out)
		}
	}

	snap := filepath.Join(dir, "snapshot.json")
	if err := os.WriteFile(snap, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := stageSnapshot(t.Context(), "snapshot.json"); err != nil {
		t.Fatalf("stageSnapshot untracked: %v", err)
	}
	out, err := exec.Command("git", "-C", dir, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	if strings.TrimSpace(string(out)) != "snapshot.json" {
		t.Errorf("staged set = %q, want snapshot.json", out)
	}

	// Idempotent: running again on a tracked-clean file is a no-op
	// (no error).
	if err := stageSnapshot(t.Context(), "snapshot.json"); err != nil {
		t.Fatalf("stageSnapshot tracked: %v", err)
	}
}
