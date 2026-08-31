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
		DQRDirEnv:  os.Getenv("PQL_DQR_DIR"),
	}
}

// mutationAnnotation tags a command whose stdout is a receipt confirming a
// change landed, rather than an answer to a question. D-30 keeps projection
// off those verbs for that reason — a receipt trimmed to `id` confirms
// nothing — and --grep is the same kind of filter: one that suppressed a
// non-matching receipt would report a successful mutation as silence. Read
// verbs carry no annotation and accept --grep.
const mutationAnnotation = "pql.surface.mutation"

// markMutation tags commands as mutation verbs. Registered where the verbs are
// assembled so the list stays next to the AddCommand calls it mirrors.
func markMutation(cmds ...*cobra.Command) {
	for _, c := range cmds {
		if c.Annotations == nil {
			c.Annotations = map[string]string{}
		}
		c.Annotations[mutationAnnotation] = "true"
	}
}

// renderOptsFromFlags maps --pretty / --jsonl / --limit / --grep to
// render.Opts. Errors when both --pretty and --jsonl are passed (mutually
// exclusive — pretty is a single multi-line array; jsonl is one object per
// line), and when --grep is combined with --pretty for the same reason:
// --grep is line-oriented like grep(1), and an indented multi-line array
// cannot also be one record per line. --grep with --jsonl is accepted and
// redundant, since --grep already emits that shape.
func renderOptsFromFlags(cmd *cobra.Command) (render.Opts, error) {
	pretty, _ := cmd.Flags().GetBool("pretty")
	jsonl, _ := cmd.Flags().GetBool("jsonl")
	limit, _ := cmd.Flags().GetInt("limit")
	grep, _ := cmd.Flags().GetString("grep")

	if pretty && jsonl {
		return render.Opts{}, errors.New("--pretty and --jsonl are mutually exclusive")
	}
	if pretty && grep != "" {
		return render.Opts{}, errors.New("--grep is line-oriented; it cannot be combined with --pretty")
	}
	if grep != "" && cmd.Annotations[mutationAnnotation] == "true" {
		return render.Opts{}, errors.New("--grep filters the read surface; it cannot be used on a mutation verb, whose output is a receipt confirming the change landed")
	}

	format := render.FormatJSON
	switch {
	case pretty:
		format = render.FormatPretty
	case jsonl:
		format = render.FormatJSONL
	}

	opts := render.Opts{Format: format, Limit: limit}
	if grep != "" {
		re, err := render.CompileGrep(grep)
		if err != nil {
			return render.Opts{}, err
		}
		opts.Grep = re
	}
	return opts, nil
}
