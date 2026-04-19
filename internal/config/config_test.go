package config

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestLoad_LocalConfigOverridesDefaults(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql.yaml"), `
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
	writeFile(t, filepath.Join(vault, ".pql.yaml"), "frontmatter: : :")
	_, err := Load(LoadOpts{
		VaultFlag: vault, HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected YAML parse error, got nil")
	}
}

func TestLoad_UnknownFieldRejected(t *testing.T) {
	vault := t.TempDir()
	writeFile(t, filepath.Join(vault, ".pql.yaml"), "fronmtater: yaml\n") // typo
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
	writeFile(t, filepath.Join(vault, ".pql.yaml"), "frontmatter: org-mode\n")
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

func TestDBPath_FlagWins(t *testing.T) {
	vault := t.TempDir()
	cfg, err := Load(LoadOpts{
		VaultFlag: vault, DBFlag: "/tmp/explicit.sqlite",
		HomeDir: t.TempDir(), CacheDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBPath != "/tmp/explicit.sqlite" {
		t.Errorf("DBPath = %q, want explicit", cfg.DBPath)
	}
}

func TestDBPath_DefaultsUseFingerprint(t *testing.T) {
	cache := t.TempDir()
	v1 := t.TempDir()
	v2 := t.TempDir()

	c1, err := Load(LoadOpts{VaultFlag: v1, HomeDir: t.TempDir(), CacheDir: cache})
	if err != nil {
		t.Fatalf("Load v1: %v", err)
	}
	c2, err := Load(LoadOpts{VaultFlag: v2, HomeDir: t.TempDir(), CacheDir: cache})
	if err != nil {
		t.Fatalf("Load v2: %v", err)
	}
	if c1.DBPath == c2.DBPath {
		t.Errorf("different vaults shared a DB path: %s", c1.DBPath)
	}
	if !strings.HasPrefix(c1.DBPath, filepath.Join(cache, "pql")) {
		t.Errorf("DBPath %q not under %s/pql", c1.DBPath, cache)
	}
	if !strings.HasSuffix(c1.DBPath, ".sqlite") {
		t.Errorf("DBPath %q does not end in .sqlite", c1.DBPath)
	}
}

func TestDBPath_HomeDirInjectionMatchesPlatform(t *testing.T) {
	// Skip on platforms without a defined sub-path so we don't bake assumptions.
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skipf("unsupported GOOS %q", runtime.GOOS)
	}
	vault := t.TempDir()
	home := t.TempDir()
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

// --- helpers ---------------------------------------------------------------

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
