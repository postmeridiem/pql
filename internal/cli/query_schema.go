package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newSchemaCmd() *cobra.Command {
	var sort string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inferred frontmatter schema across the vault",
		Long: `Print one entry per distinct frontmatter key seen in the vault: the set
of types observed for that key, and how many files use it.

  pql schema                     # alphabetical
  pql schema --sort count        # most-used first

Each row is { key, types, count }. types is sorted; usually one element —
multiple elements signal a typing inconsistency (e.g. tags: foo on one
file vs tags: [foo, bar] on another), often a typo to clean up.

This command is the primary motivation for the explicit type column on
frontmatter rows: type-aware introspection without scanning value_*.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runQuery(cmd, func(ctx context.Context, st *store.Store, _ *config.Config) ([]primitives.SchemaEntry, error) {
				opts := primitives.SchemaOpts{Sort: sort}
				if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
					opts.Limit = limit
				}
				return primitives.Schema(ctx, st.DB(), opts)
			})
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "key", "sort order: key (alphabetical) | count (most-used first)")
	return cmd
}
