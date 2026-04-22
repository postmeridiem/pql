package signal

// TagOverlap scores candidates by the number of shared tags with
// the target path.
type TagOverlap struct{}

// Name implements Signal.
func (TagOverlap) Name() string { return "tag_overlap" }

// Score implements Signal.
func (TagOverlap) Score(ctx *Context, candidatePath string) (float64, error) {
	if ctx.TargetPath == "" {
		return 0, nil
	}
	var count int
	err := ctx.DB.QueryRowContext(ctx.Ctx, `
		SELECT COUNT(*) FROM tags a
		JOIN tags b ON a.tag = b.tag
		WHERE a.path = ? AND b.path = ?
			AND a.path != b.path
	`, ctx.TargetPath, candidatePath).Scan(&count)
	if err != nil {
		return 0, err
	}
	return float64(count), nil
}
