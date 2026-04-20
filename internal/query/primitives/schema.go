package primitives

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// SchemaOpts configures a Schema query.
type SchemaOpts struct {
	// Sort selects the ordering. "" / "key" → alphabetical (default);
	// "count" → most-used first, alphabetical for ties.
	Sort string

	// Limit caps the result set. 0 means "no limit".
	Limit int
}

// Schema returns one entry per distinct frontmatter key across the vault.
// Each entry carries the set of types observed (sorted; >1 element = the
// same key carries different types in different files, often a typo) and
// the file count.
//
// Backs `pql schema` and is the primary motivation for the explicit type
// column on the frontmatter table — without it, this would have to scan
// value_text/value_num to guess each row's kind.
func Schema(ctx context.Context, db *sql.DB, opts SchemaOpts) ([]SchemaEntry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT key, type, count(*) AS c
		 FROM frontmatter
		 GROUP BY key, type
		 ORDER BY key, type`)
	if err != nil {
		return nil, fmt.Errorf("primitives.Schema: query: %w", err)
	}
	defer rows.Close()

	byKey := map[string]*SchemaEntry{}
	for rows.Next() {
		var key, typ string
		var n int
		if err := rows.Scan(&key, &typ, &n); err != nil {
			return nil, fmt.Errorf("primitives.Schema: scan: %w", err)
		}
		e, ok := byKey[key]
		if !ok {
			e = &SchemaEntry{Key: key}
			byKey[key] = e
		}
		e.Types = append(e.Types, typ)
		e.Count += n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primitives.Schema: iterate: %w", err)
	}

	out := make([]SchemaEntry, 0, len(byKey))
	for _, e := range byKey {
		// Types from SQL came in ORDER BY type, so they're sorted. Stable.
		out = append(out, *e)
	}

	switch opts.Sort {
	case "", "key":
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	case "count":
		sort.Slice(out, func(i, j int) bool {
			if out[i].Count != out[j].Count {
				return out[i].Count > out[j].Count
			}
			return out[i].Key < out[j].Key
		})
	default:
		return nil, fmt.Errorf("primitives.Schema: invalid sort %q (want key|count)", opts.Sort)
	}

	if opts.Limit > 0 && len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, nil
}
