package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/connect"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/index"
	"github.com/postmeridiem/pql/internal/intent/related"
	"github.com/postmeridiem/pql/internal/store"
	"github.com/postmeridiem/pql/internal/telemetry"
)

func newRelatedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "related <path>",
		Short: "Find files structurally related to a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIntent(cmd, args[0], func(ctx context.Context, db *store.Store, cfg *config.Config) ([]connect.Enriched, error) {
				limit, _ := cmd.Flags().GetInt("limit")
				if limit == 0 {
					limit = 10
				}
				return related.Run(ctx, db.DB(), args[0], limit)
			})
		},
	}
}

func runIntent(
	cmd *cobra.Command,
	targetPath string,
	fn func(ctx context.Context, st *store.Store, cfg *config.Config) ([]connect.Enriched, error),
) error {
	ctx := cmd.Context()
	verbose, _ := cmd.Flags().GetBool("verbose")
	tm := telemetry.New(verbose)
	defer tm.Emit()

	stopCfg := tm.Start("config")
	cfg, err := config.Load(loadOptsFromFlags(cmd))
	stopCfg()
	if err != nil {
		return &exitError{code: diag.NoInput, msg: err.Error()}
	}

	stopStore := tm.Start("store_open")
	st, err := store.Open(ctx, cfg.DBPath)
	stopStore()
	if err != nil {
		return &exitError{code: diag.Unavail, msg: err.Error()}
	}
	defer func() { _ = st.Close() }()

	stopIndex := tm.Start("index")
	if _, err := index.New(st, cfg).Run(ctx); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	stopIndex()

	flatSearch, _ := cmd.Flags().GetBool("flat-search")
	if flatSearch {
		return runFlatFallback(cmd, st, cfg, targetPath)
	}

	stopEnrich := tm.Start("enrich")
	results, err := fn(ctx, st, cfg)
	stopEnrich()
	if err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}

	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()
	n, err := render.Render(results, rOpts)
	if err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	if n == 0 {
		return errNoMatch
	}
	return nil
}

func runFlatFallback(cmd *cobra.Command, st *store.Store, cfg *config.Config, path string) error {
	ctx := cmd.Context()
	rows, err := st.DB().QueryContext(ctx,
		`SELECT path FROM files WHERE path != ? ORDER BY path`, path)
	if err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("flat fallback: %v", err)}
	}
	defer func() { _ = rows.Close() }()

	type row struct {
		Path string `json:"path"`
	}
	var results []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Path); err != nil {
			return &exitError{code: diag.Software, msg: err.Error()}
		}
		results = append(results, r)
	}

	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()
	n, err := render.Render(results, rOpts)
	if err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	if n == 0 {
		return errNoMatch
	}
	return nil
}
