// Package related implements the "related" intent: given a file, find
// structurally related files ranked by link overlap, tag overlap, and
// path proximity.
package related

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/postmeridiem/pql/internal/connect"
)

// Weights tuned for "related to this file" queries.
var Weights = connect.WeightProfile{
	"link_overlap":   0.35,
	"tag_overlap":    0.25,
	"path_proximity": 0.20,
	"recency":        0.05,
	"centrality":     0.15,
}

// Run finds files related to targetPath.
func Run(ctx context.Context, db *sql.DB, targetPath string, limit int) ([]connect.Enriched, error) {
	candidates, err := gatherCandidates(ctx, db, targetPath)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	return connect.Bundle(ctx, db, connect.BundleOpts{
		TargetPath: targetPath,
		Candidates: candidates,
		Weights:    Weights,
		Limit:      limit,
	})
}

func gatherCandidates(ctx context.Context, db *sql.DB, targetPath string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT path FROM (
			SELECT source_path AS path FROM links
			WHERE (target_path = ? OR target_path || '.md' = ?
				OR ? LIKE '%/' || target_path || '.md')
			AND source_path != ?
			UNION
			SELECT DISTINCT b.path FROM tags a
			JOIN tags b ON a.tag = b.tag
			WHERE a.path = ? AND b.path != ?
			UNION
			SELECT path FROM files
			WHERE path != ? AND path LIKE ? || '%'
		)
	`, targetPath, targetPath, targetPath, targetPath,
		targetPath, targetPath,
		targetPath, dirOf(targetPath))
	if err != nil {
		return nil, fmt.Errorf("related: gather candidates: %w", err)
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

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return ""
}
