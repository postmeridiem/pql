// Package signal defines individual scoring signals. Each signal
// computes a raw score for a candidate path given a query context.
// The ranker normalizes and combines them.
package signal

import (
	"context"
	"database/sql"
)

// Context carries the query parameters signals need to compute scores.
type Context struct {
	Query      string   // the user's query text or intent input
	TargetPath string   // for path-centric intents (e.g. "related <path>")
	DB         *sql.DB  // the index.db connection
	Ctx        context.Context
}

// Signal computes a raw score for a candidate file.
type Signal interface {
	Name() string
	Score(ctx *Context, candidatePath string) (float64, error)
}
