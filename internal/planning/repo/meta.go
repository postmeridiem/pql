package repo

import (
	"context"
	"database/sql"
	"fmt"
)

// Meta keys live in the local-only `meta` table and don't participate
// in replication. Add a constant here when introducing a new key so
// callers can't typo it.
const (
	MetaLastExportMarker = "last_export_marker"
	MetaLastImportMarker = "last_import_marker"
)

// ReadMeta returns the value for the given key, or "" + nil if the
// key is unset. Errors are reserved for actual database failures.
func ReadMeta(ctx context.Context, db *sql.DB, key string) (string, error) {
	var v string
	err := db.QueryRowContext(ctx,
		`SELECT value FROM meta WHERE key = ?`, key,
	).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("repo: read meta %q: %w", key, err)
	}
	return v, nil
}

// WriteMeta upserts a key/value pair. updated_at is set to now.
func WriteMeta(ctx context.Context, db *sql.DB, key, value string) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO meta (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = datetime('now')
	`, key, value); err != nil {
		return fmt.Errorf("repo: write meta %q: %w", key, err)
	}
	return nil
}
