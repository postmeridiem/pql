package cli

import (
	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/index"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
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
			ctx := cmd.Context()

			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			st, err := store.Open(ctx, cfg.DBPath)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer st.Close()

			// Refresh the index. Cheap when nothing changed (mtime + content
			// hash short-circuiting); the indexer is the contract that
			// queries see up-to-date data.
			if _, err := index.New(st, cfg).Run(ctx); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			opts := primitives.FilesOpts{}
			if len(args) > 0 {
				opts.Glob = args[0]
			}
			limit, _ := cmd.Flags().GetInt("limit")
			opts.Limit = limit // SQL-side LIMIT is cheaper than render-side truncation

			files, err := primitives.Files(ctx, st.DB(), opts)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			n, err := render.Render(files, rOpts)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if n == 0 {
				return errNoMatch
			}
			return nil
		},
	}
	return cmd
}
