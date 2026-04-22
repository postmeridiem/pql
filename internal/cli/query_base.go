package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/query/dsl/base"
	"github.com/postmeridiem/pql/internal/query/dsl/eval"
	"github.com/postmeridiem/pql/internal/store"
)

func newBaseCmd() *cobra.Command {
	var viewName string
	cmd := &cobra.Command{
		Use:   "base [name]",
		Short: "Execute an Obsidian .base file as a PQL query",
		Long: `Compile an Obsidian .base YAML file into PQL, then run it against the
indexed vault. The base is resolved by name (without the .base extension)
from the vault root.

  pql base council-sessions
  pql base council-members --view "The Council"
  pql base                               # list discovered .base files

The --view flag selects a named view from the .base file; without it the
first view is used.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			if len(args) == 0 {
				return listBases(cmd, cfg.Vault.Path)
			}

			path := filepath.Join(cfg.Vault.Path, args[0]+".base")
			if _, err := os.Stat(path); err != nil {
				return &exitError{
					code: diag.NoInput,
					msg:  fmt.Sprintf("base file not found: %s", path),
					hint: "run pql base (no args) to list available bases",
				}
			}

			q, err := base.Compile(path, viewName)
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
	cmd.Flags().StringVar(&viewName, "view", "", "select a named view from the .base file")
	return cmd
}

func listBases(cmd *cobra.Command, vault string) error {
	entries, err := os.ReadDir(vault)
	if err != nil {
		return &exitError{code: diag.NoInput, msg: fmt.Sprintf("read vault dir: %v", err)}
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".base") {
			names = append(names, strings.TrimSuffix(e.Name(), ".base"))
		}
	}
	if len(names) == 0 {
		return &exitError{code: diag.NoMatch}
	}

	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()

	type baseEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	rows := make([]baseEntry, len(names))
	for i, n := range names {
		rows[i] = baseEntry{Name: n, Path: filepath.Join(vault, n+".base")}
	}
	if _, err := render.Render(rows, rOpts); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	return nil
}
