package cli

import (
	gocontext "context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/connect"
	intentctx "github.com/postmeridiem/pql/internal/intent/context"
	"github.com/postmeridiem/pql/internal/store"
)

func newContextCmd() *cobra.Command {
	var proj *projection
	cmd := &cobra.Command{
		Use:   "context <path>",
		Short: "Build a context bundle for understanding a file",
		Long: `Ranks what to read alongside a file: the files it links to, the files
that link to it, and the files sharing its tags. Every result is an indexed
path, so it can be fed straight to meta, outlinks or context again.

` + rankedProjectionHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIntent(cmd, args[0], proj, func(ctx gocontext.Context, st *store.Store, cfg *config.Config) ([]connect.Enriched, error) {
				limit, _ := cmd.Flags().GetInt("limit")
				if limit == 0 {
					limit = 10
				}
				return intentctx.Run(ctx, st.DB(), args[0], limit)
			})
		},
	}
	proj = addRankedProjectionFlags(cmd)
	return cmd
}
