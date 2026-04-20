package primitives

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// BacklinksOpts configures a Backlinks query.
type BacklinksOpts struct {
	// Path is the file we want backlinks for, vault-relative
	// (e.g. "members/vaasa/persona.md"). Required.
	Path string

	// Limit caps the result set. 0 means "no limit".
	Limit int
}

// Backlinks returns every link whose target plausibly resolves to opts.Path.
//
// The links table stores wikilink targets verbatim — `[[Vaasa]]` gives
// target_path = "Vaasa", not the real file path. v1 resolution is
// pragmatic: a link is treated as a backlink to opts.Path if any of:
//
//   - target_path == opts.Path                      (markdown links)
//   - target_path == basename(opts.Path) sans .md   (wikilink/embed convention)
//   - target_path starts with the same basename followed by '#'
//     (wikilink with a heading anchor)
//
// Self-references (a file linking to itself) are excluded.
//
// Full Obsidian-style "shortest unambiguous path" resolution is more
// involved and lands in v1.x — see initial-plan.md open question #6.
func Backlinks(ctx context.Context, db *sql.DB, opts BacklinksOpts) ([]Backlink, error) {
	if opts.Path == "" {
		return nil, errors.New("primitives.Backlinks: Path is required")
	}
	base := nameFromPath(opts.Path)

	var (
		query strings.Builder
		args  []any
	)
	query.WriteString(`
		SELECT source_path, line, link_type
		FROM links
		WHERE source_path != ?
		  AND (
		        target_path = ?
		     OR target_path = ?
		     OR target_path LIKE ?
		      )
		ORDER BY source_path, line`)
	args = append(args, opts.Path, opts.Path, base, base+"#%")
	if opts.Limit > 0 {
		query.WriteString(` LIMIT ?`)
		args = append(args, opts.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("primitives.Backlinks: query: %w", err)
	}
	defer rows.Close()

	out := []Backlink{}
	for rows.Next() {
		var b Backlink
		if err := rows.Scan(&b.Path, &b.Line, &b.Via); err != nil {
			return nil, fmt.Errorf("primitives.Backlinks: scan: %w", err)
		}
		b.Name = nameFromPath(b.Path)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primitives.Backlinks: iterate: %w", err)
	}
	return out, nil
}

