package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VaultDiscovery records where the vault root resolved from. The Reason field
// is human-readable for `pql doctor`.
type VaultDiscovery struct {
	Path   string // absolute, cleaned
	Reason string // e.g. ".obsidian/ ancestor at /home/me/notes"
}

// VaultOpts feeds DiscoverVault. Caller passes whatever overrides came from
// the CLI/env; the rest defaults to runtime values (cwd) and is injectable
// for tests.
type VaultOpts struct {
	Flag     string // --vault, empty if unset
	Env      string // $PQL_VAULT, empty if unset
	StartDir string // usually os.Getwd(); test dependency injection
}

// DiscoverVault implements the documented precedence chain:
//  1. --vault flag
//  2. $PQL_VAULT env var
//  3. walk cwd up until a .obsidian/ directory is found
//  4. walk cwd up until a .git/ directory is found
//  5. cwd (with a Reason that callers should surface as a stderr warning)
//
// All returned paths are absolute and lexically cleaned.
func DiscoverVault(opts VaultOpts) (VaultDiscovery, error) {
	if opts.Flag != "" {
		abs, err := absDir(opts.Flag)
		if err != nil {
			return VaultDiscovery{}, fmt.Errorf("config: --vault %q: %w", opts.Flag, err)
		}
		return VaultDiscovery{Path: abs, Reason: "--vault flag"}, nil
	}
	if opts.Env != "" {
		abs, err := absDir(opts.Env)
		if err != nil {
			return VaultDiscovery{}, fmt.Errorf("config: PQL_VAULT %q: %w", opts.Env, err)
		}
		return VaultDiscovery{Path: abs, Reason: "PQL_VAULT env var"}, nil
	}

	start := opts.StartDir
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return VaultDiscovery{}, fmt.Errorf("config: getwd: %w", err)
		}
	}
	start, err := absDir(start)
	if err != nil {
		return VaultDiscovery{}, fmt.Errorf("config: resolve start dir %q: %w", start, err)
	}

	if dir := walkUp(start, ".obsidian"); dir != "" {
		return VaultDiscovery{
			Path:   dir,
			Reason: fmt.Sprintf(".obsidian/ ancestor at %s", dir),
		}, nil
	}
	if dir := walkUp(start, ".git"); dir != "" {
		return VaultDiscovery{
			Path:   dir,
			Reason: fmt.Sprintf(".git/ ancestor at %s", dir),
		}, nil
	}
	return VaultDiscovery{
		Path:   start,
		Reason: "cwd fallback (no .obsidian/ or .git/ ancestor found)",
	}, nil
}

// absDir resolves a path to an absolute, cleaned form and verifies it points
// at an existing directory. It does NOT follow symlinks (Clean preserves
// them); callers can EvalSymlinks if needed.
func absDir(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	if !filepath.IsAbs(p) {
		// Resolve relative to cwd.
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	info, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", p)
	}
	return p, nil
}

// walkUp searches for `marker` (a directory name) at start and each ancestor.
// Returns the directory containing the marker, or "" if none found before
// hitting the filesystem root.
func walkUp(start, marker string) string {
	dir := start
	for {
		candidate := filepath.Join(dir, marker)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// IsRootFallback reports whether the discovery resolved via the cwd-fallback
// rule (rule 5). Callers should emit a stderr warning when this is true.
func (d VaultDiscovery) IsRootFallback() bool {
	return strings.HasPrefix(d.Reason, "cwd fallback")
}
