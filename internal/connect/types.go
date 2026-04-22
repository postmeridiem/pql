// Package connect implements the optional enrichment layer that turns
// raw query rows into ranked, provenance-carrying results. The pipeline
// is: generate candidates (query/) → compute signals → rank → attach
// neighborhood connections → bundle for output.
//
// See docs/structure/design-philosophy.md for the "why" and
// docs/structure/project-structure.md for the pipeline diagram.
package connect

// Contribution records one signal's contribution to a result's score.
// Provenance travels with each result so the caller can explain rankings.
type Contribution struct {
	Name       string  `json:"name"`
	Raw        float64 `json:"raw"`
	Normalized float64 `json:"normalized"`
	Weight     float64 `json:"weight"`
	Weighted   float64 `json:"weighted"`
}

// Connection is one related item attached to a result by the
// neighborhood pass.
type Connection struct {
	Path     string `json:"path"`
	Relation string `json:"relation"`
	Via      string `json:"via,omitempty"`
}

// Enriched wraps a query result path with its ranking signals and
// neighborhood connections.
type Enriched struct {
	Path        string         `json:"path"`
	Score       float64        `json:"score"`
	Signals     []Contribution `json:"signals"`
	Connections []Connection   `json:"connections,omitempty"`
}

// WeightProfile maps signal names to weights for a specific intent.
type WeightProfile map[string]float64
