package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/postmeridiem/pql/internal/store/schema"
)

// ensureSchema brings the database up to schema.Version. It returns whether a
// rebuild happened (for caller diagnostics / stderr warnings).
//
// Three cases:
//  1. Empty database (no index_meta) — apply v1 fresh.
//  2. Existing database with matching schema_version — noop.
//  3. Existing database with different schema_version — drop everything and
//     reapply v1. Safe because the index is a pure cache.
func ensureSchema(ctx context.Context, db *sql.DB) (rebuilt bool, err error) {
	hasMeta, err := hasIndexMeta(ctx, db)
	if err != nil {
		return false, err
	}
	if !hasMeta {
		return false, applyFreshSchema(ctx, db)
	}

	version, err := readSchemaVersion(ctx, db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// index_meta exists but no schema_version row — corrupt; treat as
			// a mismatch and rebuild.
			return true, rebuild(ctx, db)
		}
		return false, err
	}
	if version == schema.Version {
		return false, nil
	}
	return true, rebuild(ctx, db)
}

func hasIndexMeta(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='index_meta'`,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("store: probe index_meta: %w", err)
	}
	return count > 0, nil
}

func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var raw string
	err := db.QueryRowContext(ctx,
		`SELECT value FROM index_meta WHERE key='schema_version'`,
	).Scan(&raw)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("store: schema_version not an integer (%q): %w", raw, err)
	}
	return v, nil
}

func applyFreshSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema.Current); err != nil {
		return fmt.Errorf("store: apply schema v%d: %w", schema.Version, err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO index_meta (key, value) VALUES ('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		strconv.Itoa(schema.Version),
	); err != nil {
		return fmt.Errorf("store: record schema_version: %w", err)
	}
	return nil
}

// rebuild drops every user-owned table and reapplies v1. FK enforcement is
// disabled during the drop so the order doesn't matter.
func rebuild(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("store: disable foreign_keys for rebuild: %w", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	}()

	tables, err := listUserTables(ctx, db)
	if err != nil {
		return err
	}
	for _, t := range tables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %q`, t)); err != nil {
			return fmt.Errorf("store: drop %q during rebuild: %w", t, err)
		}
	}
	return applyFreshSchema(ctx, db)
}

func listUserTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name FROM sqlite_master
		 WHERE type='table' AND name NOT LIKE 'sqlite_%'`,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list tables: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("store: scan table name: %w", err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate tables: %w", err)
	}
	return names, nil
}
