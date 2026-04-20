package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/index"
	"github.com/postmeridiem/pql/internal/store"
)

// runQuery encapsulates the boilerplate every read-only primitive
// subcommand shares: load config, open the store, refresh the index, run
// the query, render, map the result count to an exit code.
//
// Subcommands provide only the "query" closure — what they actually want
// from the store given a fresh index. The closure receives the live store
// and config so subcommands can use config-driven data (e.g. tag sources)
// without re-loading. Returning a non-nil error from the closure surfaces
// as exit code 70 (Software); zero rows returns errNoMatch (exit 2).
//
// Generic over T so each subcommand keeps its typed primitive return value
// — render.Render works on any T via JSON reflection.
func runQuery[T any](
	cmd *cobra.Command,
	q func(ctx context.Context, st *store.Store, cfg *config.Config) ([]T, error),
) error {
	ctx := cmd.Context()

	cfg, err := config.Load(loadOptsFromFlags(cmd))
	if err != nil {
		return &exitError{code: diag.NoInput, msg: err.Error()}
	}

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return &exitError{code: diag.Unavail, msg: err.Error()}
	}
	defer st.Close()

	if _, err := index.New(st, cfg).Run(ctx); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}

	rows, err := q(ctx, st, cfg)
	if err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}

	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	n, err := render.Render(rows, rOpts)
	if err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	if n == 0 {
		return errNoMatch
	}
	return nil
}
