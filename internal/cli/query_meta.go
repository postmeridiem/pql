package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/query/primitives"
	"github.com/postmeridiem/pql/internal/store"
)

func newMetaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "meta <path>",
		Short: "Show frontmatter, tags, links, and headings for one file",
		Long: `Print the per-file aggregate as a single JSON object: file metadata
(path, name, size, mtime), frontmatter (raw values keyed by frontmatter
key), tags, outlinks, and headings.

  pql meta members/vaasa/persona.md
  pql meta sessions/2026-04-19_volt-nl-fit/outcome.md --pretty

Frontmatter values are returned as the YAML user wrote them — not wrapped
in {"type":"...", "value":...} envelopes. Type-aware introspection is
the job of pql schema, not pql meta. Exit code 66 if the file isn't
indexed; 0 otherwise.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			return runQueryOne(cmd,
				func(ctx context.Context, st *store.Store, _ *config.Config) (*primitives.Meta, error) {
					return primitives.MetaOne(ctx, st.DB(), primitives.MetaOpts{Path: path})
				},
				fmt.Sprintf("file not indexed: %s", path),
			)
		},
	}
}
