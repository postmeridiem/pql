package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
)

// projection carries the flags that let a caller trim what a multi-row verb
// emits (D-27): --fields for an exact key set, --oneline for a plain-text
// index, and --full on verbs whose default projection drops a heavy key.
//
// Two families use it. The planning list verbs drop `description`; the ranked
// verbs (search, related, context) drop `signals` and `connections`. Same
// principle either way — the default is the answer, and the bulk is the extra.
type projection struct {
	fields  string
	oneline bool
	full    bool
}

// projectionFlags is the per-verb wording of the shared flags. Only the help
// text varies; the behaviour is identical everywhere.
type projectionFlags struct {
	// Example is a plausible --fields value for this verb, e.g. "id,status,title".
	Example string
	// Oneline is the row shape --oneline emits, e.g. "id<TAB>status<TAB>title".
	Oneline string
	// Full is the help for --full. Empty omits the flag: a verb whose default
	// projection drops nothing has nothing to opt back into, and a --full that
	// means "same as without it" is a flag that lies.
	Full string
}

// addProjectionFlags registers the shared projection flags on cmd.
func addProjectionFlags(cmd *cobra.Command, f projectionFlags) *projection {
	p := &projection{}
	cmd.Flags().StringVar(&p.fields, "fields", "",
		"project rows to these comma-separated fields (e.g. "+f.Example+"); '*' selects all")
	cmd.Flags().BoolVar(&p.oneline, "oneline", false,
		"plain-text mode: one "+f.Oneline+" line per row, no JSON")
	if f.Full != "" {
		cmd.Flags().BoolVar(&p.full, "full", false, f.Full)
	}
	return p
}

// wantsFullRows reports whether the caller opted back into whole rows
// (--full, or its --fields '*' spelling).
func (p *projection) wantsFullRows() bool {
	return p.full || p.fields == "*"
}

// wantsDefaultProjection reports whether the verb should apply its default
// trim. Naming a dropped key in --fields projects from whole rows, so an
// explicit --fields always overrides the default.
func (p *projection) wantsDefaultProjection() bool {
	return !p.wantsFullRows() && p.fields == ""
}

// renderProjectedList renders list rows honouring the projection flags.
// --oneline emits plain lines via line() — explicitly a non-JSON human
// mode, like `git log --oneline` — and composes only with --limit.
// --fields narrows the JSON rows through render.Project, preserving the
// requested key order. Contradictory flag mixes exit Usage.
func renderProjectedList[T any](cmd *cobra.Command, rows []T, p *projection, line func(T) string) error {
	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()

	if p.oneline {
		if p.fields != "" || p.full {
			return &exitError{code: diag.Usage, msg: "--oneline cannot be combined with --fields or --full"}
		}
		if rOpts.Format != render.FormatJSON {
			return &exitError{code: diag.Usage, msg: "--oneline is plain text; it cannot be combined with --pretty or --jsonl"}
		}
		if rOpts.Limit > 0 && len(rows) > rOpts.Limit {
			rows = rows[:rOpts.Limit]
		}
		for _, r := range rows {
			if _, err := fmt.Fprintln(rOpts.Out, line(r)); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
		}
		return nil
	}

	if p.fields != "" && p.fields != "*" {
		if p.full {
			return &exitError{code: diag.Usage, msg: "--fields and --full are mutually exclusive"}
		}
		projected, err := render.Project(rows, splitFieldList(p.fields))
		if err != nil {
			return &exitError{code: diag.Usage, msg: err.Error()}
		}
		if _, err := render.Render(projected, rOpts); err != nil {
			return &exitError{code: diag.Software, msg: err.Error()}
		}
		return nil
	}

	if _, err := render.Render(rows, rOpts); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	return nil
}

// splitFieldList parses the --fields value: comma-separated, whitespace
// tolerated, duplicates collapsed to the first occurrence (a repeated
// key would otherwise emit twice).
func splitFieldList(s string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
