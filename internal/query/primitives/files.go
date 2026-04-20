package primitives

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"strings"
)

// FilesOpts configures a Files query.
type FilesOpts struct {
	// Glob is an optional path filter using SQLite's GLOB syntax (* and ?,
	// no **). Matched against the vault-relative path stored in files.path.
	// Empty means "no filter — return everything".
	Glob string

	// Limit caps the result set. 0 means "no limit" (callers can also clamp
	// at the renderer with --limit, but doing it in SQL is cheaper for
	// large vaults).
	Limit int
}

// Files returns indexed file rows ordered by path. Honours opts.Glob and
// opts.Limit. The returned slice is non-nil even when empty, so callers
// can pass it straight to the renderer.
func Files(ctx context.Context, db *sql.DB, opts FilesOpts) ([]File, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString(`SELECT path, size, mtime FROM files`)
	if opts.Glob != "" {
		query.WriteString(` WHERE path GLOB ?`)
		args = append(args, opts.Glob)
	}
	query.WriteString(` ORDER BY path`)
	if opts.Limit > 0 {
		query.WriteString(` LIMIT ?`)
		args = append(args, opts.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("primitives.Files: query: %w", err)
	}
	defer rows.Close()

	out := []File{}
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.Size, &f.Mtime); err != nil {
			return nil, fmt.Errorf("primitives.Files: scan: %w", err)
		}
		f.Name = nameFromPath(f.Path)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primitives.Files: iterate: %w", err)
	}
	return out, nil
}

// nameFromPath derives a display name from a vault-relative path:
// "members/vaasa/persona.md" → "persona". The .md extension is dropped per
// the docs/pql-grammar.md `file.name` virtual column convention.
func nameFromPath(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, ".md")
}
