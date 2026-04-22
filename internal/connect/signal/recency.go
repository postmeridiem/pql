package signal

import "time"

// Recency scores candidates by how recently they were modified.
// Returns a 0–1 score where 1.0 = modified in the last hour,
// decaying toward 0 over 90 days.
type Recency struct{}

// Name implements Signal.
func (Recency) Name() string { return "recency" }

// Score implements Signal.
func (Recency) Score(ctx *Context, candidatePath string) (float64, error) {
	var mtime int64
	err := ctx.DB.QueryRowContext(ctx.Ctx,
		`SELECT mtime FROM files WHERE path = ?`, candidatePath,
	).Scan(&mtime)
	if err != nil {
		return 0, nil
	}

	age := time.Since(time.Unix(mtime, 0)).Hours()
	const decayHours = 90 * 24
	if age <= 0 {
		return 1.0, nil
	}
	if age >= decayHours {
		return 0, nil
	}
	return 1.0 - (age / decayHours), nil
}
