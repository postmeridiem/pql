package connect

import (
	"context"
	"database/sql"
	"fmt"
)

// Neighborhood attaches one-hop structural connections to enriched
// results: outlinks, inlinks, and shared-tag siblings.
func Neighborhood(ctx context.Context, db *sql.DB, results []Enriched, maxPerResult int) ([]Enriched, error) {
	if maxPerResult <= 0 {
		maxPerResult = 5
	}
	for i := range results {
		conns, err := neighborsOf(ctx, db, results[i].Path, maxPerResult)
		if err != nil {
			return nil, err
		}
		results[i].Connections = conns
	}
	return results, nil
}

func neighborsOf(ctx context.Context, db *sql.DB, path string, limit int) ([]Connection, error) {
	var conns []Connection

	outlinks, err := db.QueryContext(ctx, `
		SELECT DISTINCT target_path FROM links
		WHERE source_path = ? AND target_path NOT LIKE 'http%'
		LIMIT ?
	`, path, limit)
	if err != nil {
		return nil, fmt.Errorf("connect: outlinks of %s: %w", path, err)
	}
	for outlinks.Next() {
		var target string
		if err := outlinks.Scan(&target); err != nil {
			_ = outlinks.Close()
			return nil, err
		}
		conns = append(conns, Connection{Path: target, Relation: "outlink"})
	}
	_ = outlinks.Close()

	inlinks, err := db.QueryContext(ctx, `
		SELECT DISTINCT source_path FROM links
		WHERE (target_path = ? OR target_path || '.md' = ?
			OR ? LIKE '%/' || target_path || '.md')
		AND source_path != ?
		LIMIT ?
	`, path, path, path, path, limit)
	if err != nil {
		return nil, fmt.Errorf("connect: inlinks of %s: %w", path, err)
	}
	for inlinks.Next() {
		var source string
		if err := inlinks.Scan(&source); err != nil {
			_ = inlinks.Close()
			return nil, err
		}
		conns = append(conns, Connection{Path: source, Relation: "inlink"})
	}
	_ = inlinks.Close()

	siblings, err := db.QueryContext(ctx, `
		SELECT DISTINCT b.path FROM tags a
		JOIN tags b ON a.tag = b.tag
		WHERE a.path = ? AND b.path != ?
		GROUP BY b.path
		ORDER BY COUNT(*) DESC
		LIMIT ?
	`, path, path, limit)
	if err != nil {
		return nil, fmt.Errorf("connect: tag siblings of %s: %w", path, err)
	}
	for siblings.Next() {
		var sib string
		if err := siblings.Scan(&sib); err != nil {
			_ = siblings.Close()
			return nil, err
		}
		conns = append(conns, Connection{Path: sib, Relation: "shared_tags"})
	}
	_ = siblings.Close()

	return conns, nil
}
