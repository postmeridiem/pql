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
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search the vault with ranked results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIntent(cmd, "", func(ctx context.Context, st *store.Store, cfg *config.Config) ([]connect.Enriched, error) {
				limit, _ := cmd.Flags().GetInt("limit")
				if limit == 0 {
					limit = 10
				}
				return search.Run(ctx, st.DB(), args[0], limit)
			})
		},
	}
}
