package connect

import (
	"math"
	"sort"

	"github.com/postmeridiem/pql/internal/connect/signal"
)

type scored struct {
	path     string
	contribs []Contribution
}

// Rank scores candidates using the given signals and weight profile,
// returning Enriched results sorted by score descending.
func Rank(ctx *signal.Context, candidates []string, signals []signal.Signal, weights WeightProfile) ([]Enriched, error) {
	results := make([]scored, 0, len(candidates))
	for _, path := range candidates {
		var contribs []Contribution
		for _, sig := range signals {
			raw, err := sig.Score(ctx, path)
			if err != nil {
				return nil, err
			}
			contribs = append(contribs, Contribution{
				Name: sig.Name(),
				Raw:  raw,
			})
		}
		results = append(results, scored{path: path, contribs: contribs})
	}

	normalize(results)

	enriched := make([]Enriched, len(results))
	for i, r := range results {
		var total float64
		for j := range r.contribs {
			w := weights[r.contribs[j].Name]
			r.contribs[j].Weight = w
			r.contribs[j].Weighted = r.contribs[j].Normalized * w
			total += r.contribs[j].Weighted
		}
		enriched[i] = Enriched{
			Path:    r.path,
			Score:   total,
			Signals: r.contribs,
		}
	}

	// Total order: score first, path as tie-break. Ties are common, not
	// exotic — a link-sparse vault zeroes several signals at once and
	// files past the recency clamp tie at exactly 0 — and without the
	// tie-break the arrangement of tied candidates is whatever the sort
	// leaves, i.e. a function of input order (T-127).
	sort.SliceStable(enriched, func(i, j int) bool {
		if enriched[i].Score != enriched[j].Score {
			return enriched[i].Score > enriched[j].Score
		}
		return enriched[i].Path < enriched[j].Path
	})
	return enriched, nil
}

func normalize(results []scored) {
	if len(results) == 0 {
		return
	}
	sigCount := len(results[0].contribs)
	for s := range sigCount {
		var maxVal float64
		for _, r := range results {
			if math.Abs(r.contribs[s].Raw) > maxVal {
				maxVal = math.Abs(r.contribs[s].Raw)
			}
		}
		for i := range results {
			if maxVal > 0 {
				results[i].contribs[s].Normalized = results[i].contribs[s].Raw / maxVal
			}
		}
	}
}
