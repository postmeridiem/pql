package primitives

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// TagsOpts configures a Tags query.
type TagsOpts struct {
	// MinCount filters out tags appearing on fewer than N files. 0 keeps
	// every tag; useful for spotting one-off / typo tags by setting MinCount=2.
	MinCount int

	// Sort selects the ordering. Valid values:
	//   ""        — same as "tag"
	//   "tag"     — alphabetical by tag name (default)
	//   "count"   — descending by count, then alphabetical for ties
	Sort string

	// Limit caps the result set. 0 means "no limit".
	Limit int
}

// Tags returns the deduplicated tag → file-count distribution. Backs
// `pql tags`. Aggregates the tags table; one row per distinct tag.
func Tags(ctx context.Context, db *sql.DB, opts TagsOpts) ([]TagCount, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString(`SELECT tag, count(*) AS c FROM tags GROUP BY tag`)
	if opts.MinCount > 0 {
		query.WriteString(` HAVING c >= ?`)
		args = append(args, opts.MinCount)
	}
	switch opts.Sort {
	case "", "tag":
		query.WriteString(` ORDER BY tag`)
	case "count":
		query.WriteString(` ORDER BY c DESC, tag`)
	default:
		return nil, fmt.Errorf("primitives.Tags: invalid sort %q (want tag|count)", opts.Sort)
	}
	if opts.Limit > 0 {
		query.WriteString(` LIMIT ?`)
		args = append(args, opts.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("primitives.Tags: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []TagCount{}
	for rows.Next() {
		var t TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, fmt.Errorf("primitives.Tags: scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primitives.Tags: iterate: %w", err)
	}
	return out, nil
}
