// Package store wraps the SQLite index that backs every pql query.
//
// The on-disk schema is the canonical query target — the indexer writes to it,
// the query layer reads from it. The schema is versioned (see
// internal/store/schema.Version) and the migration policy is "drop-and-rebuild
// on mismatch" because the index is a pure cache: every row is regenerable
// from the vault on disk.
//
// modernc.org/sqlite provides a pure-Go SQLite driver so the binary stays
// cgo-free and cross-compiles cleanly to all platforms in .goreleaser.yaml.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers driver name "sqlite"
)

// Store is a handle on the pql SQLite index. Open returns one ready to use;
// Close releases it. Concurrent reads are safe (WAL); writers must hold the
// connection exclusively (see BeginTx in a future revision).
type Store struct {
	db   *sql.DB
	path string
}

// Open connects to (or creates) a pql index at path. WAL is enabled, foreign
// keys are enforced, and the schema is applied or verified. If the on-disk
// schema_version doesn't match schema.Version, the index is rebuilt in place
// (no migration code — the index is a pure cache).
//
// Use ":memory:" for an ephemeral in-memory database (handy in tests).
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("store: path required (use \":memory:\" for in-memory)")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping %q: %w", path, err)
	}
	if err := setPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := ensureSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ensure schema: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

// Close releases the underlying connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB. Exposed during scaffolding so the
// indexer and query layers can run statements directly; will be replaced by
// typed repo helpers under internal/store/repo/ as those land.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Path is the filesystem path the store was opened with (or ":memory:").
func (s *Store) Path() string {
	return s.path
}

// SchemaVersion returns the schema_version recorded in index_meta.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	return readSchemaVersion(ctx, s.db)
}

// JournalMode returns the active journal_mode (typically "wal" on disk,
// "memory" for ":memory:").
func (s *Store) JournalMode(ctx context.Context) (string, error) {
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return "", fmt.Errorf("store: read journal_mode: %w", err)
	}
	return mode, nil
}

// ForeignKeysEnabled reports whether PRAGMA foreign_keys is on.
func (s *Store) ForeignKeysEnabled(ctx context.Context) (bool, error) {
	var on int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&on); err != nil {
		return false, fmt.Errorf("store: read foreign_keys: %w", err)
	}
	return on == 1, nil
}

func setPragmas(ctx context.Context, db *sql.DB) error {
	// journal_mode is a query (returns the resulting mode) — use Query, not Exec.
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&mode); err != nil {
		return fmt.Errorf("store: enable WAL: %w", err)
	}
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("store: %s: %w", p, err)
		}
	}
	return nil
}
