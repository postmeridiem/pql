package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newTagsCmd() *cobra.Command {
	var (
		sort     string
		minCount int
	)
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List distinct tags with file counts",
		Long: `List every tag in the indexed vault alongside the number of files it
appears on. Aggregates frontmatter tags and inline #tags per the config.

  pql tags                       # all tags, alphabetical
  pql tags --sort count          # most-used first
  pql tags --min-count 2         # drop one-off tags (typo finder)

Output is a JSON array of {tag, count} objects.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runQuery(cmd, func(ctx context.Context, st *store.Store, _ *config.Config) ([]primitives.TagCount, error) {
				opts := primitives.TagsOpts{Sort: sort, MinCount: minCount}
				if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
					opts.Limit = limit
				}
				return primitives.Tags(ctx, st.DB(), opts)
			})
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "tag", "sort order: tag (alphabetical) | count (most-used first)")
	cmd.Flags().IntVar(&minCount, "min-count", 0, "drop tags appearing on fewer than N files")
	return cmd
}
