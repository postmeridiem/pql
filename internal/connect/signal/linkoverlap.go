package signal

// LinkOverlap scores candidates by the number of shared link targets
// with the target path. High overlap = structurally related.
type LinkOverlap struct{}

// Name implements Signal.
func (LinkOverlap) Name() string { return "link_overlap" }

// Score implements Signal.
func (LinkOverlap) Score(ctx *Context, candidatePath string) (float64, error) {
	if ctx.TargetPath == "" {
		return 0, nil
	}
	var count int
	err := ctx.DB.QueryRowContext(ctx.Ctx, `
		SELECT COUNT(*) FROM links a
		JOIN links b ON a.target_path = b.target_path
		WHERE a.source_path = ? AND b.source_path = ?
			AND a.source_path != b.source_path
	`, ctx.TargetPath, candidatePath).Scan(&count)
	if err != nil {
		return 0, err
	}
	return float64(count), nil
}
