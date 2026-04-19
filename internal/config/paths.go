package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// resolveDBPath determines where the SQLite index lives for this vault.
//
// Precedence:
//  1. --db flag
//  2. PQL_DB env var
//  3. <cache>/pql/<sha256(vault_path)[:16]>.sqlite (default)
//
// The fingerprint scheme means a single user with multiple vaults gets one
// index per vault automatically; switching between them doesn't churn the DB.
func resolveDBPath(opts LoadOpts, vaultPath string) (string, error) {
	if opts.DBFlag != "" {
		return filepath.Clean(opts.DBFlag), nil
	}
	if opts.DBEnv != "" {
		return filepath.Clean(opts.DBEnv), nil
	}
	cacheDir, err := cacheDirFor(opts)
	if err != nil {
		return "", fmt.Errorf("config: locate cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "pql")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("config: create cache dir %q: %w", dir, err)
	}
	return filepath.Join(dir, vaultFingerprint(vaultPath)+".sqlite"), nil
}

// vaultFingerprint hashes the absolute vault path so each vault gets its own
// stable on-disk filename without exposing the raw path.
func vaultFingerprint(vaultPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(vaultPath)))
	return hex.EncodeToString(sum[:8]) // 16 hex chars; collision risk negligible
}

// cacheDirFor returns the platform-conventional cache directory:
//   - Linux:   $XDG_CACHE_HOME or ~/.cache
//   - macOS:   ~/Library/Caches
//   - Windows: %LocalAppData%
//
// All routed through os.UserCacheDir, with a HomeDir override for tests.
func cacheDirFor(opts LoadOpts) (string, error) {
	if opts.CacheDir != "" {
		return opts.CacheDir, nil
	}
	if opts.HomeDir != "" {
		// Match os.UserCacheDir's per-OS convention manually so tests can
		// inject HomeDir without depending on real env vars.
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(opts.HomeDir, "Library", "Caches"), nil
		case "windows":
			return filepath.Join(opts.HomeDir, "AppData", "Local"), nil
		default:
			return filepath.Join(opts.HomeDir, ".cache"), nil
		}
	}
	return os.UserCacheDir()
}
