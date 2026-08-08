package config

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// --- DiscoverVault ----------------------------------------------------------

func TestDiscoverVault_FlagWins(t *testing.T) {
	dir := t.TempDir()
	d, err := DiscoverVault(VaultOpts{Flag: dir})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != dir {
		t.Errorf("Path = %q, want %q", d.Path, dir)
	}
	if d.Reason != "--vault flag" {
		t.Errorf("Reason = %q, want %q", d.Reason, "--vault flag")
	}
}

func TestDiscoverVault_EnvWinsOverWalkUp(t *testing.T) {
	envDir := t.TempDir()
	startDir := t.TempDir()
	mkdir(t, filepath.Join(startDir, ".obsidian"))

	d, err := DiscoverVault(VaultOpts{Env: envDir, StartDir: startDir})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != envDir {
		t.Errorf("Path = %q, want env dir %q", d.Path, envDir)
	}
	if d.Reason != "PQL_VAULT env var" {
		t.Errorf("Reason = %q, want PQL_VAULT env var", d.Reason)
	}
}

func TestDiscoverVault_ObsidianAncestorPreferred(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, ".obsidian"))
	mkdir(t, filepath.Join(root, ".git"))           // both markers at root
	deep := filepath.Join(root, "members", "vaasa") // start two levels deep
	mkdir(t, deep)

	d, err := DiscoverVault(VaultOpts{StartDir: deep})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != root {
		t.Errorf("Path = %q, want root %q", d.Path, root)
	}
	if !strings.HasPrefix(d.Reason, ".obsidian/") {
		t.Errorf("Reason = %q, want .obsidian/ prefix", d.Reason)
	}
}

func TestDiscoverVault_GitAncestorUsedWhenNoObsidian(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, ".git"))
	deep := filepath.Join(root, "src")
	mkdir(t, deep)

	d, err := DiscoverVault(VaultOpts{StartDir: deep})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != root {
		t.Errorf("Path = %q, want root %q", d.Path, root)
	}
	if !strings.HasPrefix(d.Reason, ".git/") {
		t.Errorf("Reason = %q, want .git/ prefix", d.Reason)
	}
	if d.IsRootFallback() {
		t.Error("IsRootFallback unexpectedly true")
	}
}

// A linked git worktree carries a .git *file* (a `gitdir:` pointer), not a
// directory. Requiring a directory made discovery skip the worktree and keep
// ascending — silently resolving to the main checkout (T-58).
func TestDiscoverVault_GitFileMarksWorktreeRoot(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "wt")
	mkdir(t, wt)
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: /somewhere/.git/worktrees/wt\n")

	d, err := DiscoverVault(VaultOpts{StartDir: wt})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != wt {
		t.Errorf("Path = %q, want worktree %q", d.Path, wt)
	}
	if !strings.Contains(d.Reason, "linked worktree") {
		t.Errorf("Reason = %q, want it to name the linked worktree", d.Reason)
	}
}

// The .git-file fix alone is not enough: with one full ascent per marker, a
// worktree nested under an Obsidian vault still resolves to the vault, because
// the .obsidian pass runs to completion before .git is ever considered. Every
// marker must be checked at each level (T-58).
func TestDiscoverVault_WorktreeNestedUnderObsidianVault(t *testing.T) {
	vault := t.TempDir()
	mkdir(t, filepath.Join(vault, ".obsidian"))
	mkdir(t, filepath.Join(vault, ".git"))
	wt := filepath.Join(vault, ".worktrees", "feature")
	mkdir(t, wt)
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: "+vault+"/.git/worktrees/feature\n")

	d, err := DiscoverVault(VaultOpts{StartDir: wt})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != wt {
		t.Errorf("Path = %q, want worktree %q, not the enclosing vault", d.Path, wt)
	}
}

// A worktree placed outside its main checkout used to match no marker at all
// and land on the cwd fallback — a second, quieter wrong answer (T-58).
func TestDiscoverVault_WorktreeOutsideMainCheckout(t *testing.T) {
	wt := t.TempDir()
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: /elsewhere/.git/worktrees/detached\n")

	d, err := DiscoverVault(VaultOpts{StartDir: wt})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != wt {
		t.Errorf("Path = %q, want worktree %q", d.Path, wt)
	}
	if d.IsRootFallback() {
		t.Errorf("IsRootFallback = true, expected a marker match (Reason=%q)", d.Reason)
	}
}

// Deeper inside a worktree the same rule has to hold, one level up at a time.
func TestDiscoverVault_StartsDeepInsideWorktree(t *testing.T) {
	vault := t.TempDir()
	mkdir(t, filepath.Join(vault, ".obsidian"))
	wt := filepath.Join(vault, ".worktrees", "feature")
	deep := filepath.Join(wt, "governance", "decisions")
	mkdir(t, deep)
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: "+vault+"/.git/worktrees/feature\n")

	d, err := DiscoverVault(VaultOpts{StartDir: deep})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != wt {
		t.Errorf("Path = %q, want worktree root %q", d.Path, wt)
	}
}

func TestDiscoverVault_CWDFallbackWhenNoMarkers(t *testing.T) {
	dir := t.TempDir()
	d, err := DiscoverVault(VaultOpts{StartDir: dir})
	if err != nil {
		t.Fatalf("DiscoverVault: %v", err)
	}
	if d.Path != dir {
		t.Errorf("Path = %q, want %q", d.Path, dir)
	}
	if !d.IsRootFallback() {
		t.Errorf("IsRootFallback = false, expected true (Reason=%q)", d.Reason)
	}
}

func TestDiscoverVault_FlagToNonexistentErrors(t *testing.T) {
	_, err := DiscoverVault(VaultOpts{Flag: "/this/path/does/not/exist/anywhere"})
	if err == nil {
		t.Fatal("expected error for nonexistent --vault, got nil")
	}
}

// --- Load: defaults and YAML -----------------------------------------------

func TestLoad_NoFile_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(LoadOpts{
		VaultFlag: dir,
		HomeDir:   t.TempDir(),
		CacheDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty", cfg.ConfigPath)
	}
	if cfg.Frontmatter != FrontmatterYAML {
		t.Errorf("Frontmatter default = %q, want %q", cfg.Frontmatter, FrontmatterYAML)
	}
	if cfg.Wikilinks != WikilinksObsidian {
		t.Errorf("Wikilinks default = %q, want %q", cfg.Wikilinks, WikilinksObsidian)
	}
	if len(cfg.Exclude) == 0 {
		t.Error("Exclude defaults missing")
	}
}

func TestLoad_TicketStatusesDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(LoadOpts{VaultFlag: dir, HomeDir: t.TempDir(), CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"backlog", "ready", "in_progress", "review", "done", "cancelled"}
	if len(cfg.TicketStatuses) != len(want) {
		t.Fatalf("default ticket_statuses = %d entries, want %d", len(cfg.TicketStatuses), len(want))
	}
	for i, name := range want {
		if cfg.TicketStatuses[i].Name != name {
			t.Errorf("ticket_statuses[%d] = %q, want %q", i, cfg.TicketStatuses[i].Name, name)
		}
	}
	if !cfg.TicketStatuses[0].IsDefault || cfg.TicketStatuses[0].Class != StatusClassInitial {
		t.Errorf("backlog should be the default initial status, got %+v", cfg.TicketStatuses[0])
	}
}

func TestLoad_TicketStatusesCustom(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), `
ticket_statuses:
  - { name: triage,  class: initial, is_default: true }
  - { name: doing,   class: active }
  - { name: shipped, class: terminal }
`)
	cfg, err := Load(LoadOpts{VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TicketStatuses) != 3 {
		t.Fatalf("custom ticket_statuses = %d entries, want 3 (slice should replace, not merge)", len(cfg.TicketStatuses))
	}
	if cfg.TicketStatuses[0].Name != "triage" || !cfg.TicketStatuses[0].IsDefault {
		t.Errorf("first status = %+v, want triage/default", cfg.TicketStatuses[0])
	}
}

func TestLoad_TicketStatusesValidation(t *testing.T) {
	cases := map[string]string{
		"no default": `
ticket_statuses:
  - { name: a, class: initial }
  - { name: b, class: terminal }`,
		"two defaults": `
ticket_statuses:
  - { name: a, class: initial, is_default: true }
  - { name: b, class: initial, is_default: true }
  - { name: c, class: terminal }`,
		"default not initial": `
ticket_statuses:
  - { name: a, class: active, is_default: true }
  - { name: b, class: terminal }`,
		"no terminal": `
ticket_statuses:
  - { name: a, class: initial, is_default: true }`,
		"bad class": `
ticket_statuses:
  - { name: a, class: bogus, is_default: true }
  - { name: b, class: terminal }`,
		"duplicate name": `
ticket_statuses:
  - { name: a, class: initial, is_default: true }
  - { name: a, class: terminal }`,
		"empty list": `ticket_statuses: []`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			vault := t.TempDir()
			writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), body)
			_, err := Load(LoadOpts{VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir()})
			if err == nil {
				t.Fatalf("expected validation error for %q, got nil", name)
			}
		})
	}
}

func TestLoad_LocalConfigOverridesDefaults(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), `
frontmatter: yaml
wikilinks: pandoc
tags:
  sources: [frontmatter]
exclude:
  - "**/draft/**"
git_metadata: true
fts: true
aliases:
  members: "type = 'council-member'"
`)
	cfg, err := Load(LoadOpts{
		VaultFlag: vault,
		HomeDir:   t.TempDir(),
		CacheDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Wikilinks != WikilinksPandoc {
		t.Errorf("Wikilinks = %q, want %q", cfg.Wikilinks, WikilinksPandoc)
	}
	if !cfg.GitMetadata {
		t.Error("GitMetadata not loaded")
	}
	if !cfg.FTS {
		t.Error("FTS not loaded")
	}
	if got := cfg.Aliases["members"]; got != "type = 'council-member'" {
		t.Errorf("alias members = %q", got)
	}
	if cfg.ConfigPath == "" {
		t.Error("ConfigPath should be populated")
	}
}

func TestLoad_BadYAMLErrors(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), "frontmatter: : :")
	_, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected YAML parse error, got nil")
	}
}

func TestLoad_UnknownFieldRejected(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), "fronmtater: yaml\n") // typo
	_, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected unknown-field error, got nil")
	}
	if !strings.Contains(err.Error(), "fronmtater") {
		t.Errorf("error should mention the typo'd field, got: %v", err)
	}
}

func TestLoad_ValidationRejectsBadFrontmatter(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), "frontmatter: org-mode\n")
	_, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// --- Hash ------------------------------------------------------------------

func TestHash_StableForIdenticalConfigs(t *testing.T) {
	a := defaults()
	b := defaults()
	ah, err := a.Hash()
	if err != nil {
		t.Fatalf("Hash a: %v", err)
	}
	bh, err := b.Hash()
	if err != nil {
		t.Fatalf("Hash b: %v", err)
	}
	if ah != bh {
		t.Errorf("identical configs hashed differently: %s vs %s", ah, bh)
	}
}

func TestHash_ChangesWhenConfigChanges(t *testing.T) {
	a := defaults()
	b := defaults()
	b.FTS = !b.FTS
	ah, _ := a.Hash()
	bh, _ := b.Hash()
	if ah == bh {
		t.Errorf("FTS toggle did not change hash (%s)", ah)
	}
}

// --- DB path ---------------------------------------------------------------

// skipOnWindows skips tests that simulate a read-only vault via chmod:
// stripping write bits from a directory does not prevent file creation on
// Windows, so the cache-fallback path never triggers there.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod cannot make a directory read-only on Windows; fallback untestable here")
	}
}

func TestDBPath_FlagWins(t *testing.T) {
	vault := t.TempDir()
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, DBFlag: "/tmp/explicit.sqlite",
		HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath != filepath.Clean("/tmp/explicit.sqlite") {
		t.Errorf("DBPath = %q, want explicit", cfg.DBPath)
	}
}

func TestDBPath_EnvOverridesDefault(t *testing.T) {
	vault := t.TempDir()
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, DBEnv: "/tmp/from-env.sqlite",
		HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath != filepath.Clean("/tmp/from-env.sqlite") {
		t.Errorf("DBPath = %q, want env value", cfg.DBPath)
	}
}

func TestDBPath_YAMLDBFieldOverridesDefault(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), "db: custom/path.sqlite\n")
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(vault, "custom", "path.sqlite")
	if cfg.DBPath != want {
		t.Errorf("DBPath = %q, want %q (vault-relative)", cfg.DBPath, want)
	}
}

func TestDBPath_DefaultsToInVaultPqlDir(t *testing.T) {
	vault := t.TempDir()
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(vault, ".pql", "index.db")
	if cfg.DBPath != want {
		t.Errorf("DBPath = %q, want in-vault %q", cfg.DBPath, want)
	}
	// .pql/ should have been created so the path is actually usable.
	if info, err := os.Stat(filepath.Join(vault, ".pql")); err != nil || !info.IsDir() {
		t.Errorf(".pql/ not created: %v", err)
	}
}

func TestDBPath_FallsBackToCacheOnReadOnlyVault(t *testing.T) {
	skipOnWindows(t)
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks; cannot test fallback")
	}
	vault := t.TempDir()
	cache := t.TempDir()
	// Strip write permission so MkdirAll(<vault>/.pql) returns EACCES.
	if err := os.Chmod(vault, 0o555); err != nil {
		t.Fatalf("chmod readonly: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(vault, 0o755) }) // so t.TempDir cleanup works

	cfg, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: cache,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.HasPrefix(cfg.DBPath, filepath.Join(cache, "pql")) {
		t.Errorf("DBPath = %q, want fallback under %s/pql", cfg.DBPath, cache)
	}
	if !strings.HasSuffix(cfg.DBPath, "index.db") {
		t.Errorf("DBPath = %q, want suffix index.db", cfg.DBPath)
	}
}

func TestDBPath_FallbackUsesPerVaultSubdir(t *testing.T) {
	skipOnWindows(t)
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks; cannot test fallback")
	}
	cache := t.TempDir()
	v1 := t.TempDir()
	v2 := t.TempDir()
	for _, v := range []string{v1, v2} {
		if err := os.Chmod(v, 0o555); err != nil {
			t.Fatalf("chmod %s: %v", v, err)
		}
		t.Cleanup(func() { _ = os.Chmod(v, 0o755) })
	}
	c1, err := Load(LoadOpts{VaultFlag: v1, HomeDir: t.TempDir(), CacheDir: cache})
	if err != nil {
		t.Fatalf("Load v1: %v", err)
	}
	c2, err := Load(LoadOpts{VaultFlag: v2, HomeDir: t.TempDir(), CacheDir: cache})
	if err != nil {
		t.Fatalf("Load v2: %v", err)
	}
	if c1.DBPath == c2.DBPath {
		t.Errorf("different vaults shared fallback DB path: %s", c1.DBPath)
	}
}

func TestDBPath_HomeDirFallbackMatchesPlatform(t *testing.T) {
	skipOnWindows(t)
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks; cannot test fallback")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skipf("unsupported GOOS %q", runtime.GOOS)
	}
	vault := t.TempDir()
	home := t.TempDir()
	if err := os.Chmod(vault, 0o555); err != nil {
		t.Fatalf("chmod readonly: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(vault, 0o755) })

	cfg, err := Load(LoadOpts{VaultFlag: vault, HomeDir: home})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var wantPrefix string
	switch runtime.GOOS {
	case "darwin":
		wantPrefix = filepath.Join(home, "Library", "Caches", "pql")
	case "windows":
		wantPrefix = filepath.Join(home, "AppData", "Local", "pql")
	default:
		wantPrefix = filepath.Join(home, ".cache", "pql")
	}
	if !strings.HasPrefix(cfg.DBPath, wantPrefix) {
		t.Errorf("DBPath = %q, want prefix %q", cfg.DBPath, wantPrefix)
	}
}

func TestLoad_IgnoreFilesDefault(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(LoadOpts{
		VaultFlag: dir, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// .pqlignore is in the default set, not just documented as an option to
	// add: a vault-root .pqlignore has to work without a config edit first,
	// or the file pql names after itself is inert (T-78). Order is
	// load-bearing — later files win on per-pattern conflicts, so
	// .pqlignore must come after .gitignore to be able to re-include.
	want := []string{".gitignore", ".pqlignore"}
	if !slices.Equal(cfg.IgnoreFiles, want) {
		t.Errorf("default IgnoreFiles = %v, want %v", cfg.IgnoreFiles, want)
	}
}

func TestLoad_IgnoreFilesOverride(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"),
		"ignore_files: [.gitignore, .pqlignore]\n")
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.IgnoreFiles) != 2 ||
		cfg.IgnoreFiles[0] != ".gitignore" ||
		cfg.IgnoreFiles[1] != ".pqlignore" {
		t.Errorf("IgnoreFiles = %v, want [.gitignore .pqlignore]", cfg.IgnoreFiles)
	}
}

func TestLoad_IgnoreFilesEmpty(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), "ignore_files: []\n")
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.IgnoreFiles) != 0 {
		t.Errorf("explicit empty list should disable file-based ignores; got %v", cfg.IgnoreFiles)
	}
}

func TestHash_IgnoreFilesAffectsHash(t *testing.T) {
	a := defaults()
	b := defaults()
	b.IgnoreFiles = append([]string(nil), b.IgnoreFiles...)
	b.IgnoreFiles = append(b.IgnoreFiles, ".pqlignore")
	ah, _ := a.Hash()
	bh, _ := b.Hash()
	if ah == bh {
		t.Errorf("IgnoreFiles change did not affect hash (%s)", ah)
	}
}

func TestHash_DBFieldDoesNotAffectHash(t *testing.T) {
	a := defaults()
	b := defaults()
	b.DB = "/somewhere/else.sqlite"
	ah, _ := a.Hash()
	bh, _ := b.Hash()
	if ah != bh {
		t.Errorf("DB field changed hash but shouldn't (only WHERE not WHAT): %s vs %s", ah, bh)
	}
}

// --- helpers ---------------------------------------------------------------

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func TestLoad_DQRDirDefaultAndEnvOverride(t *testing.T) {
	// Default when unset.
	cfg, err := Load(LoadOpts{
		VaultFlag: t.TempDir(),
		HomeDir:   t.TempDir(),
		CacheDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DQRDir != "governance" {
		t.Errorf("default DQRDir = %q, want governance", cfg.DQRDir)
	}

	// Config-file value overrides the default.
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql", "config.yaml"), "dqr_dir: decisions\n")
	cfg, err = Load(LoadOpts{VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Load with file: %v", err)
	}
	if cfg.DQRDir != "decisions" {
		t.Errorf("file DQRDir = %q, want decisions", cfg.DQRDir)
	}

	// $PQL_DQR_DIR wins over the config file (env > file > default).
	cfg, err = Load(LoadOpts{VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(), DQRDirEnv: "rules"})
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}
	if cfg.DQRDir != "rules" {
		t.Errorf("env DQRDir = %q, want rules (env should beat file)", cfg.DQRDir)
	}
}
