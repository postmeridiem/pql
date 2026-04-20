package primitives

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// OutlinksOpts configures an Outlinks query.
type OutlinksOpts struct {
	// Path is the source file whose outgoing links we want, vault-relative.
	// Required.
	Path string

	// Limit caps the result set. 0 means "no limit".
	Limit int
}

// Outlinks returns every link originating from opts.Path, in document
// order (ORDER BY line). Each row carries the raw target as it was
// extracted — no resolution applied.
//
// Empty result and unknown path are not distinguished: if opts.Path isn't
// indexed, Outlinks returns []. Callers map zero rows to exit code 2 per
// the output contract; explicit "is this file indexed?" probes go through
// `pql meta` once that ships.
func Outlinks(ctx context.Context, db *sql.DB, opts OutlinksOpts) ([]Outlink, error) {
	if opts.Path == "" {
		return nil, errors.New("primitives.Outlinks: Path is required")
	}
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString(`
		SELECT target_path, alias, line, link_type
		FROM links
		WHERE source_path = ?
		ORDER BY line, target_path`)
	args = append(args, opts.Path)
	if opts.Limit > 0 {
		query.WriteString(` LIMIT ?`)
		args = append(args, opts.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("primitives.Outlinks: query: %w", err)
	}
	defer rows.Close()

	out := []Outlink{}
	for rows.Next() {
		var (
			o     Outlink
			alias sql.NullString
		)
		if err := rows.Scan(&o.Target, &alias, &o.Line, &o.Via); err != nil {
			return nil, fmt.Errorf("primitives.Outlinks: scan: %w", err)
		}
		if alias.Valid {
			o.Alias = alias.String
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primitives.Outlinks: iterate: %w", err)
	}
	return out, nil
}
