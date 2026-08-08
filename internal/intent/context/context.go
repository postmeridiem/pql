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

// gatherCandidates collects the files worth ranking against targetPath:
// what links to it, what it links to, and what shares its tags.
//
// The outbound branch resolves link targets against the files table rather
// than emitting them directly. A target is link text as written — extensionless
// (`members/x/persona`), anchored (`guide.md#install`), or a bare same-document
// anchor (`#some-heading`) — and emitting those unresolved put values in the
// result's `path` field that no other command would accept: `pql meta` rejected
// them as "file not indexed", and a bare anchor named no file at all (T-73).
// Joining to files means every candidate is a real indexed path, which is what
// callers need to follow a result anywhere.
//
// Relative-prefix targets (`../questions/architecture.md`) still fail to
// resolve and are dropped rather than emitted broken. Resolving those properly
// is link normalisation, Q-6.
func gatherCandidates(ctx context.Context, db *sql.DB, targetPath string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT path FROM (
			SELECT source_path AS path FROM links
			WHERE (target_path = ? OR target_path || '.md' = ?
				OR ? LIKE '%/' || target_path || '.md')
			AND source_path != ?
			UNION
			SELECT f.path FROM (
				SELECT CASE
					WHEN instr(target_path, '#') > 0
						THEN substr(target_path, 1, instr(target_path, '#') - 1)
					ELSE target_path
				END AS target
				FROM links
				WHERE source_path = ? AND target_path NOT LIKE 'http%'
			) o
			JOIN files f
			  ON f.path = o.target
			  OR f.path = o.target || '.md'
			  OR f.path LIKE '%/' || o.target || '.md'
			WHERE o.target != '' AND f.path != ?
			UNION
			SELECT DISTINCT b.path FROM tags a
			JOIN tags b ON a.tag = b.tag
			WHERE a.path = ? AND b.path != ?
		)
	`, targetPath, targetPath, targetPath, targetPath,
		targetPath, targetPath,
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
