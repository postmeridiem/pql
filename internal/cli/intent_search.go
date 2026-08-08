package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/connect"
	"github.com/postmeridiem/pql/internal/intent/search"
	"github.com/postmeridiem/pql/internal/store"
)

func newSearchCmd() *cobra.Command {
	var proj *projection
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the vault with ranked results",
		Long: `Ranks vault files against a query on structural signals. There is no
text-match signal, so an exact phrase sitting in a file can still rank nowhere
— use grep for literal strings.

` + rankedProjectionHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIntent(cmd, "", proj, func(ctx context.Context, st *store.Store, cfg *config.Config) ([]connect.Enriched, error) {
				limit, _ := cmd.Flags().GetInt("limit")
				if limit == 0 {
					limit = 10
				}
				return search.Run(ctx, st.DB(), args[0], limit)
			})
		},
	}
	proj = addRankedProjectionFlags(cmd)
	return cmd
}
