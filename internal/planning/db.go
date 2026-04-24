// Package planning implements the decisions + tickets user-state store
// backed by <vault>/.pql/pql.db. This is user-authored state with
// forward-only migrations, distinct from the regenerable index.db cache.
// See decisions/architecture.md (D-3).
package planning

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers driver name "sqlite"

	"github.com/postmeridiem/pql/internal/config"
)

const dbFileName = "pql.db"

// DB is a handle on the planning store at <vault>/.pql/pql.db.
type DB struct {
	db   *sql.DB
	path string
}

// Open connects to (or creates) the planning DB for the given vault.
// The .pql/ directory is created if needed. If the vault root is
// read-only, Open returns an error — unlike the index cache, the
// planning store has no fallback location.
func Open(ctx context.Context, vaultPath string) (*DB, error) {
	dir := filepath.Join(vaultPath, config.VaultStateDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("planning: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, dbFileName)
	return OpenPath(ctx, path)
}

// OpenPath connects to a planning DB at an explicit path. Used by
// Open (after resolving the vault) and by tests with ":memory:".
func OpenPath(ctx context.Context, path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("planning: open %q: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("planning: ping %q: %w", path, err)
	}
	if err := setPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db, path: path}, nil
}

// Close releases the underlying connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// SQL returns the underlying *sql.DB for repo helpers.
func (d *DB) SQL() *sql.DB {
	return d.db
}

// Path returns the filesystem path (or ":memory:").
func (d *DB) Path() string {
	return d.path
}

func setPragmas(ctx context.Context, db *sql.DB) error {
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&mode); err != nil {
		return fmt.Errorf("planning: enable WAL: %w", err)
	}
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("planning: %s: %w", p, err)
		}
	}
	return nil
}
