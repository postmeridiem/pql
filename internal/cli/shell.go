package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/index"
	"github.com/postmeridiem/pql/internal/query/dsl/eval"
	"github.com/postmeridiem/pql/internal/query/dsl/parse"
	"github.com/postmeridiem/pql/internal/store"
)

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Interactive PQL REPL",
		Long: `Start an interactive read-eval-print loop for PQL DSL queries.

The vault is indexed once at startup; each line is parsed, compiled, and
executed against the same store. Type "exit" or "quit" (or Ctrl-D) to
leave. Blank lines and lines starting with -- are skipped.

Output format respects --pretty / --jsonl / --limit, same as pql query.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShell(cmd)
		},
	}
}

func runShell(cmd *cobra.Command) error {
	ctx := cmd.Context()

	cfg, err := config.Load(loadOptsFromFlags(cmd))
	if err != nil {
		return &exitError{code: diag.NoInput, msg: err.Error()}
	}

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return &exitError{code: diag.Unavail, msg: err.Error()}
	}
	defer func() { _ = st.Close() }()

	if _, err := index.New(st, cfg).Run(ctx); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}

	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()

	interactive := isTerminal(cmd.InOrStdin())
	prompt := func() {
		if interactive {
			fmt.Fprint(os.Stderr, "pql> ")
		}
	}

	scanner := bufio.NewScanner(cmd.InOrStdin())
	prompt()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			prompt()
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		execOne(ctx, st, rOpts, line)
		prompt()
	}
	if err := scanner.Err(); err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("shell: read stdin: %v", err)}
	}
	return nil
}

func execOne(ctx context.Context, st *store.Store, rOpts render.Opts, src string) {
	q, err := parse.Parse(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return
	}
	compiled, err := eval.Compile(q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compile error: %v\n", err)
		return
	}
	rows, err := eval.Exec(ctx, st.DB(), compiled)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exec error: %v\n", err)
		return
	}
	if _, err := render.Render(rows, rOpts); err != nil {
		fmt.Fprintf(os.Stderr, "render error: %v\n", err)
	}
}
