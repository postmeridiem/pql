// Package context implements the "context" intent: given a file, build
// a rich context bundle of the file itself plus its most important
// neighbors — the "what should I read to understand this file" answer.
package context

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/postmeridiem/pql/internal/connect"
)

// Weights tuned for "understand this file" queries — centrality and
// link overlap matter most.
var Weights = connect.WeightProfile{
	"link_overlap":   0.30,
	"tag_overlap":    0.15,
	"path_proximity": 0.25,
	"recency":        0.05,
	"centrality":     0.25,
}

// Run builds a context bundle for the given file.
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
			SELECT target_path AS path FROM links
			WHERE source_path = ? AND target_path NOT LIKE 'http%'
			UNION
			SELECT DISTINCT b.path FROM tags a
			JOIN tags b ON a.tag = b.tag
			WHERE a.path = ? AND b.path != ?
		)
	`, targetPath, targetPath, targetPath, targetPath,
		targetPath,
		targetPath, targetPath)
	if err != nil {
		return nil, fmt.Errorf("context: gather candidates: %w", err)
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
