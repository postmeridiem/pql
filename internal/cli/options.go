package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
)

// loadOptsFromFlags collects the global flags + matching env vars into a
// config.LoadOpts. Subcommands call this and pass the result to
// config.Load so the precedence chain is consistent across the CLI.
func loadOptsFromFlags(cmd *cobra.Command) config.LoadOpts {
	flag := func(name string) string {
		v, _ := cmd.Flags().GetString(name)
		return v
	}
	return config.LoadOpts{
		VaultFlag:  flag("vault"),
		VaultEnv:   os.Getenv("PQL_VAULT"),
		DBFlag:     flag("db"),
		DBEnv:      os.Getenv("PQL_DB"),
		ConfigFlag: flag("config"),
		ConfigEnv:  os.Getenv("PQL_CONFIG"),
	}
}

// renderOptsFromFlags maps --pretty / --jsonl / --limit to render.Opts.
// Errors when both --pretty and --jsonl are passed (mutually exclusive
// — pretty is a single multi-line array; jsonl is one object per line).
func renderOptsFromFlags(cmd *cobra.Command) (render.Opts, error) {
	pretty, _ := cmd.Flags().GetBool("pretty")
	jsonl, _ := cmd.Flags().GetBool("jsonl")
	limit, _ := cmd.Flags().GetInt("limit")

	if pretty && jsonl {
		return render.Opts{}, errors.New("--pretty and --jsonl are mutually exclusive")
	}

	format := render.FormatJSON
	switch {
	case pretty:
		format = render.FormatPretty
	case jsonl:
		format = render.FormatJSONL
	}
	return render.Opts{Format: format, Limit: limit}, nil
}
