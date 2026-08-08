package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/query/dsl/eval"
	"github.com/postmeridiem/pql/internal/query/dsl/parse"
	"github.com/postmeridiem/pql/internal/store"
)

func newQueryCmd() *cobra.Command {
	var (
		fromFile  string
		fromStdin bool
	)
	cmd := &cobra.Command{
		Use:   "query [DSL]",
		Short: "Run a PQL DSL query against the indexed vault",
		Long: `Run a PQL query against the indexed vault.

Three input modes (mutually exclusive):

  pql query "SELECT name WHERE folder = 'members'"     # positional
  pql query --file q.sql                               # from a file
  echo "SELECT name" | pql query --stdin               # from stdin

Columns: path, name, folder, size, mtime, tags, headings, and fm.<key>
for any frontmatter key. Naming one that does not exist exits 65 listing
the built-ins.

SELECT DISTINCT works and is the way to find which values a frontmatter
key actually takes — pql schema reports keys and types, never values.
WHERE supports =, !=, <, >, <=, >=, LIKE, AND, OR, NOT, and IN against
tags and headings. Aggregates (COUNT, GROUP BY) are not supported and
fail at exit 65.

  pql query "SELECT DISTINCT fm.status"
  pql query "SELECT path WHERE folder = 'members/vaasa'"
  pql query "SELECT path WHERE 'Design notes' IN headings"
  pql query "SELECT path, size WHERE path LIKE 'notes/%' AND size > 5000"

The DSL bypasses ranking and provenance — rows out are exactly what the
query selects, in the order ORDER BY (if any) requests. For ranked
results with attached connections, use an intent subcommand instead.

Exit codes follow docs/output-contract.md: 0 on success — including zero
matches, which returns an empty array, not a distinct code (D-22) — 65 on
parse, unknown-column and evaluation errors, 64 on bad flags, 66/69 on the
usual config/store failures, 70 on runtime SQL errors.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := readDSLSource(args, fromFile, fromStdin, cmd.InOrStdin())
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}

			q, err := parse.Parse(src)
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}
			compiled, err := eval.Compile(q)
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}

			return runQuery(cmd, func(ctx context.Context, st *store.Store, _ *config.Config) ([]eval.Row, error) {
				return eval.Exec(ctx, st.DB(), compiled)
			})
		},
	}
	cmd.Flags().StringVar(&fromFile, "file", "", "read the DSL from this file path")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read the DSL from stdin")
	return cmd
}

// readDSLSource resolves the query source per the three mutually
// exclusive input modes. Returns a clear error when none / multiple
// modes are specified or the chosen one fails.
func readDSLSource(args []string, fromFile string, fromStdin bool, stdin io.Reader) (string, error) {
	chosen := 0
	if len(args) > 0 {
		chosen++
	}
	if fromFile != "" {
		chosen++
	}
	if fromStdin {
		chosen++
	}
	switch chosen {
	case 0:
		return "", errors.New("pql query: provide a DSL via positional arg, --file, or --stdin")
	case 1:
		// good
	default:
		return "", errors.New("pql query: positional, --file, and --stdin are mutually exclusive")
	}

	switch {
	case len(args) > 0:
		return args[0], nil
	case fromFile != "":
		b, err := os.ReadFile(fromFile) //nolint:gosec // G304: --file path is supplied by the user invoking the CLI; reading from a user-named path is the feature
		if err != nil {
			return "", fmt.Errorf("read --file %q: %w", fromFile, err)
		}
		return string(b), nil
	case fromStdin:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	}
	return "", errors.New("unreachable")
}
