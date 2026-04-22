package signal

// Centrality scores candidates by inbound link count — files that
// are linked to by many others are structurally central.
type Centrality struct{}

// Name implements Signal.
func (Centrality) Name() string { return "centrality" }

// Score implements Signal.
func (Centrality) Score(ctx *Context, candidatePath string) (float64, error) {
	var count int
	err := ctx.DB.QueryRowContext(ctx.Ctx, `
		SELECT COUNT(DISTINCT source_path) FROM links
		WHERE target_path = ? OR target_path || '.md' = ?
	`, candidatePath, candidatePath).Scan(&count)
	if err != nil {
		return 0, nil
	}
	return float64(count), nil
}
