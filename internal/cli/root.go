// Package cli is the CLI front-end. cmd/pql/main.go calls Run with os.Args[1:].
//
// This is intentionally a stdlib-only stub. Cobra and full subcommand wiring
// land alongside the indexer milestone (v0.1) where they actually pay off;
// adding them now would be scaffolding without code to support.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/version"
)

const usage = `pql — Project Query Language

Usage:
  pql <subcommand> [flags] [args]
  pql <QUERY>                     run PQL DSL (positional)
  pql --version                   print short version
  pql version --build-info        print full build info as JSON
  pql --help                      this message

Status: scaffolding. Subcommands land per the v0.1 milestone in docs/structure/initial-plan.md.
See also docs/structure/{design-philosophy,project-structure}.md.
`

// Run dispatches CLI args. Returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return diag.Usage
	}
	switch args[0] {
	case "--help", "-h", "help":
		fmt.Fprint(os.Stdout, usage)
		return diag.OK
	case "--version", "-v":
		fmt.Println(version.Version)
		return diag.OK
	case "version":
		return runVersion(args[1:])
	}
	diag.Error("cli.unknown_subcommand", fmt.Sprintf("unknown subcommand: %q", args[0]), "see `pql --help`")
	return diag.Usage
}

func runVersion(args []string) int {
	buildInfo := false
	for _, a := range args {
		if a == "--build-info" {
			buildInfo = true
		}
	}
	if !buildInfo {
		fmt.Println(version.Version)
		return diag.OK
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(version.Info()); err != nil {
		diag.Error("cli.version.encode", err.Error(), "")
		return diag.Software
	}
	return diag.OK
}
