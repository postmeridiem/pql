// Package signal defines individual scoring signals. Each signal
// computes a raw score for a candidate path given a query context.
// The ranker normalizes and combines them.
package signal

import (
	"context"
	"database/sql"
	"time"
)

// Context carries the query parameters signals need to compute scores.
type Context struct {
	Query      string  // the user's query text or intent input
	TargetPath string  // for path-centric intents (e.g. "related <path>")
	DB         *sql.DB // the index.db connection
	Ctx        context.Context

	// Now is the reference instant for time-derived signals, captured
	// once per enrichment pass so every candidate in a batch scores
	// against the same moment — and so tests can pin it (T-126). Zero
	// means "current time", so an omitted field stays correct.
	Now time.Time
}

// Signal computes a raw score for a candidate file.
type Signal interface {
	Name() string
	Score(ctx *Context, candidatePath string) (float64, error)
}
