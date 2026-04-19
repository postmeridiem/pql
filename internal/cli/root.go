// Package cli is the CLI front-end. cmd/pql/main.go calls Run with os.Args[1:].
//
// The root command is wired with cobra so subcommands can be added incrementally
// as the v0.1 milestone ships query primitives (files, tags, meta, backlinks,
// outlinks, schema, init, doctor) and v0.2 adds the SQL DSL.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/version"
)

// Run dispatches CLI args. Returns the process exit code.
func Run(args []string) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		// cobra prints the usage error to stderr already; we just translate
		// to the exit-code contract documented in docs/output-contract.md.
		return diag.Usage
	}
	return diag.OK
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pql",
		Short:         "Project Query Language — SQL-derived CLI for markdown vaults",
		Long:          longDescription,
		Version:       version.Version,
		SilenceErrors: false,
		SilenceUsage:  false,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("unknown command: %q", args[0])
		},
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.AddCommand(newVersionCmd())
	return cmd
}

func newVersionCmd() *cobra.Command {
	var buildInfo bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version (or full build info with --build-info)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !buildInfo {
				fmt.Println(version.Version)
				return nil
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(version.Info()); err != nil {
				return errors.New("failed to encode build info")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&buildInfo, "build-info", false, "emit JSON with full build info")
	return cmd
}

const longDescription = `pql indexes a markdown vault into SQLite and exposes intent-level tools so
agents (Claude Code, primarily) can ask structural questions instead of falling
back to grep+read. See docs/structure/{design-philosophy,project-structure}.md
for the architecture and docs/pql-grammar.md for the query language.`
