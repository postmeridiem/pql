package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// VaultStateDir is the per-vault directory pql owns at the vault root,
// analogous to .git/. See docs/vault-layout.md.
const VaultStateDir = ".pql"

// IndexFileName is the SQLite file pql writes inside VaultStateDir (or the
// per-vault cache subdirectory when falling back).
const IndexFileName = "index.sqlite"

// resolveDBPath determines where the SQLite index lives for this vault.
//
// Precedence:
//  1. --db flag (LoadOpts.DBFlag)
//  2. PQL_DB env var (LoadOpts.DBEnv)
//  3. db: field in .pql/config.yaml (cfgDB)
//  4. <vault>/.pql/index.sqlite — created in place if writeable
//  5. <cache>/pql/<sha256(vault)[:16]>/index.sqlite — fallback when the vault
//     is read-only (EROFS / EACCES / EPERM)
//
// Override paths from rules 1–3 are returned verbatim; the store layer's
// Open() handles parent-directory creation and surfaces a clear error if the
// path is unusable. The default + fallback paths (4 and 5) ARE created here
// because deciding "we'll put it there" requires proving we can.
func resolveDBPath(opts LoadOpts, vaultPath, cfgDB string) (string, error) {
	if opts.DBFlag != "" {
		return filepath.Clean(opts.DBFlag), nil
	}
	if opts.DBEnv != "" {
		return filepath.Clean(opts.DBEnv), nil
	}
	if cfgDB != "" {
		// db: in .pql/config.yaml is interpreted relative to the vault root if not absolute.
		if !filepath.IsAbs(cfgDB) {
			cfgDB = filepath.Join(vaultPath, cfgDB)
		}
		return filepath.Clean(cfgDB), nil
	}

	inVault := filepath.Join(vaultPath, VaultStateDir)
	if err := os.MkdirAll(inVault, 0o750); err == nil {
		return filepath.Join(inVault, IndexFileName), nil
	} else if !isReadOnlyError(err) {
		return "", fmt.Errorf("config: create %q: %w", inVault, err)
	}

	cacheDir, err := cacheDirFor(opts)
	if err != nil {
		return "", fmt.Errorf("config: locate cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "pql", vaultFingerprint(vaultPath))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("config: create cache dir %q: %w", dir, err)
	}
	return filepath.Join(dir, IndexFileName), nil
}

// vaultFingerprint hashes the absolute vault path so each vault gets its own
// stable on-disk subdirectory in the user cache without exposing the raw path.
func vaultFingerprint(vaultPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(vaultPath)))
	return hex.EncodeToString(sum[:8]) // 16 hex chars; collision risk negligible
}

// cacheDirFor returns the platform-conventional cache directory:
//   - Linux:   $XDG_CACHE_HOME or ~/.cache
//   - macOS:   ~/Library/Caches
//   - Windows: %LocalAppData%
//
// Routed through os.UserCacheDir, with HomeDir/CacheDir overrides for tests.
func cacheDirFor(opts LoadOpts) (string, error) {
	if opts.CacheDir != "" {
		return opts.CacheDir, nil
	}
	if opts.HomeDir != "" {
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

// isReadOnlyError reports whether err signals "the filesystem won't let us
// write here" — used to trigger the cache fallback. Covers EACCES/EPERM
// portably via fs.ErrPermission. EROFS-specific detection is intentionally
// skipped: on the platforms where it matters (Unix), read-only mounts also
// surface as fs.ErrPermission for mkdir; on Windows the syscall constant
// doesn't exist.
func isReadOnlyError(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}
