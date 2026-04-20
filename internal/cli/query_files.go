package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newFilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "files [glob]",
		Short: "List indexed markdown files (optionally filtered by GLOB)",
		Long: `List markdown files in the indexed vault, sorted by path.

The optional [glob] argument is a SQLite GLOB pattern matched against the
vault-relative path (* and ? wildcards; no doublestar **). Examples:

  pql files                       # everything
  pql files 'members/*'           # one level under members/
  pql files 'sessions/*/outcome.md'

Output is JSON by default; --jsonl streams one object per line and --pretty
indents. Exit code 2 means "ran successfully, zero matches" — distinguish
from real errors (65/66/69/70) per docs/output-contract.md.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuery(cmd, func(ctx context.Context, st *store.Store, _ *config.Config) ([]primitives.File, error) {
				opts := primitives.FilesOpts{}
				if len(args) > 0 {
					opts.Glob = args[0]
				}
				if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
					opts.Limit = limit
				}
				return primitives.Files(ctx, st.DB(), opts)
			})
		},
	}
}
