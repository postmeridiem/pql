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
	return &cobra.Command{
		Use:   "context <path>",
		Short: "Build a context bundle for understanding a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIntent(cmd, args[0], func(ctx gocontext.Context, st *store.Store, cfg *config.Config) ([]connect.Enriched, error) {
				limit, _ := cmd.Flags().GetInt("limit")
				if limit == 0 {
					limit = 10
				}
				return intentctx.Run(ctx, st.DB(), args[0], limit)
			})
		},
	}
}
