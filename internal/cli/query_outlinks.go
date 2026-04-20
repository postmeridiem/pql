package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newOutlinksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "outlinks <path>",
		Short: "List links the given file makes (in document order)",
		Long: `List every wikilink, embed, and markdown link in the given vault-relative
file, in document order.

  pql outlinks members/vaasa/persona.md

Each row carries the raw target as it appears in the source — wikilink
target text, image embed path, or markdown URL — plus the alias (if any),
the 1-based line, and the link kind ("wiki", "embed", "md"). Resolution
to a real file path isn't applied; that's the indexer's job in v1.x.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, func(ctx context.Context, st *store.Store, _ *config.Config) ([]primitives.Outlink, error) {
				opts := primitives.OutlinksOpts{Path: args[0]}
				if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
					opts.Limit = limit
				}
				return primitives.Outlinks(ctx, st.DB(), opts)
			})
		},
	}
}
