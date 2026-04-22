// Package cli is the CLI front-end. cmd/pql/main.go calls Run with os.Args[1:].
//
// Subcommand surface lives in this package as one file per command
// (query_*.go for primitive queries, intent_*.go for intent commands once
// they land). Cross-cutting concerns are in exit.go (exit-code mapping)
// and options.go (flag → opts struct extraction).
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

// Run dispatches CLI args. Returns the process exit code per
// docs/output-contract.md.
func Run(args []string) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SilenceErrors = true // we emit diagnostics via internal/diag ourselves
	cmd.SilenceUsage = true  // RunE errors don't dump usage; flag-parse errors still do

	err := cmd.Execute()
	if err == nil {
		return diag.OK
	}

	// Controlled exit via exitError carries its own code (and an optional
	// diagnostic message we surface to stderr).
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.msg != "" {
			diag.Error("cli.exit", ee.msg, ee.hint)
		}
		return ee.code
	}

	// Anything else is a cobra-layer error (unknown subcommand, unknown
	// flag, missing required arg). Subcommand RunE always wraps its own
	// failures in exitError, so a non-exitError reaching here is by
	// elimination a usage problem. Emit and return Usage (64).
	diag.Error("cli.error", err.Error(), "")
	return diag.Usage
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pql",
		Short:   "Project Query Language — SQL-derived CLI for markdown vaults",
		Long:    longDescription,
		Version: version.Version,
		// No subcommand given. Cobra's default would print help and exit 0;
		// we want exit 64 (Usage) so callers can distinguish "user invoked
		// pql with no instructions" from "pql ran something successfully".
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.SetVersionTemplate("{{.Version}}\n")

	pf := cmd.PersistentFlags()
	pf.String("vault", "", "vault root override (env: PQL_VAULT)")
	pf.String("db", "", "DB path override (env: PQL_DB)")
	pf.String("config", "", "config file override (env: PQL_CONFIG)")
	pf.Bool("pretty", false, "pretty-print JSON output")
	pf.Bool("jsonl", false, "emit JSON-per-line instead of an array")
	pf.IntP("limit", "n", 0, "cap result count (0 = no limit)")
	pf.Bool("quiet", false, "suppress stderr warnings")
	pf.Bool("verbose", false, "emit per-phase timing diagnostics on stderr")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newFilesCmd())
	cmd.AddCommand(newTagsCmd())
	cmd.AddCommand(newBacklinksCmd())
	cmd.AddCommand(newOutlinksCmd())
	cmd.AddCommand(newMetaCmd())
	cmd.AddCommand(newSchemaCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newQueryCmd())
	cmd.AddCommand(newBaseCmd())
	cmd.AddCommand(newShellCmd())
	cmd.AddCommand(newSkillCmd())
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
