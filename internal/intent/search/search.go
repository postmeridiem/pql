// Package search implements the "search" intent: find files matching
// a text query, ranked by relevance signals.
package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/postmeridiem/pql/internal/connect"
)

// Weights tuned for text search queries.
var Weights = connect.WeightProfile{
	"link_overlap":   0.10,
	"tag_overlap":    0.15,
	"path_proximity": 0.10,
	"recency":        0.25,
	"centrality":     0.40,
}

// Run finds files matching the query text by searching paths, tags,
// frontmatter values, and headings.
func Run(ctx context.Context, db *sql.DB, query string, limit int) ([]connect.Enriched, error) {
	candidates, err := gatherCandidates(ctx, db, query)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	return connect.Bundle(ctx, db, connect.BundleOpts{
		Query:      query,
		Candidates: candidates,
		Weights:    Weights,
		Limit:      limit,
	})
}

func gatherCandidates(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT path FROM (
			SELECT path FROM files WHERE LOWER(path) LIKE ?
			UNION
			SELECT path FROM tags WHERE LOWER(tag) LIKE ?
			UNION
			SELECT path FROM frontmatter WHERE LOWER(value_text) LIKE ?
			UNION
			SELECT path FROM headings WHERE LOWER(text) LIKE ?
		)
	`, pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("search: gather candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
