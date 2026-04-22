package connect

import (
	"context"
	"database/sql"

	"github.com/postmeridiem/pql/internal/connect/signal"
)

// DefaultSignals returns the standard set of signals.
func DefaultSignals() []signal.Signal {
	return []signal.Signal{
		signal.LinkOverlap{},
		signal.TagOverlap{},
		signal.PathProximity{},
		signal.Recency{},
		signal.Centrality{},
	}
}

// BundleOpts configures an enrichment pass.
type BundleOpts struct {
	Query          string
	TargetPath     string
	Candidates     []string
	Weights        WeightProfile
	Limit          int
	NeighborhoodN  int
}

// Bundle runs the full enrichment pipeline: score → rank → neighborhood.
func Bundle(ctx context.Context, db *sql.DB, opts BundleOpts) ([]Enriched, error) {
	signals := DefaultSignals()
	if opts.Weights == nil {
		opts.Weights = defaultWeights()
	}

	sigCtx := &signal.Context{
		Query:      opts.Query,
		TargetPath: opts.TargetPath,
		DB:         db,
		Ctx:        ctx,
	}

	ranked, err := Rank(sigCtx, opts.Candidates, signals, opts.Weights)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(ranked) > opts.Limit {
		ranked = ranked[:opts.Limit]
	}

	return Neighborhood(ctx, db, ranked, opts.NeighborhoodN)
}

func defaultWeights() WeightProfile {
	return WeightProfile{
		"link_overlap":   0.30,
		"tag_overlap":    0.25,
		"path_proximity": 0.20,
		"recency":        0.10,
		"centrality":     0.15,
	}
}
