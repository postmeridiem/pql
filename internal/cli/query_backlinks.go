package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newBacklinksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backlinks <path>",
		Short: "List files that link to the given vault-relative path",
		Long: `List files containing a wikilink, embed, or markdown link that resolves
to the given vault-relative path.

  pql backlinks members/vaasa/persona.md

v1 resolution is pragmatic: a link counts as a backlink when its raw
target_path matches the requested full path, the basename without .md, or
the basename followed by '#anchor'. Self-references are excluded. Output
is one row per (source, line) pair so duplicate links from the same source
are visible.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, func(ctx context.Context, st *store.Store, _ *config.Config) ([]primitives.Backlink, error) {
				opts := primitives.BacklinksOpts{Path: args[0]}
				if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
					opts.Limit = limit
				}
				return primitives.Backlinks(ctx, st.DB(), opts)
			})
		},
	}
}
