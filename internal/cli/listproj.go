package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
)

// listProjection carries the projection flags shared by the planning
// list verbs (D-27): --fields for an exact column set, --oneline for a
// plain-text index, and --full on verbs whose default projection drops
// a heavy column.
type listProjection struct {
	fields  string
	oneline bool
	full    bool
}

// addListProjectionFlags registers the shared projection flags.
// withFull is set only by verbs that trim their default projection
// (today: ticket list, which drops description) — verbs whose rows are
// already light get --fields/--oneline without a meaningless --full.
func addListProjectionFlags(cmd *cobra.Command, withFull bool) *listProjection {
	p := &listProjection{}
	cmd.Flags().StringVar(&p.fields, "fields", "", "project rows to these comma-separated fields (e.g. id,status,title); '*' selects all")
	cmd.Flags().BoolVar(&p.oneline, "oneline", false, "plain-text mode: one id<TAB>status<TAB>title line per row, no JSON")
	if withFull {
		cmd.Flags().BoolVar(&p.full, "full", false, "emit whole rows, including the description the default projection omits")
	}
	return p
}

// wantsFullRows reports whether the caller opted back into whole rows
// (--full, or its --fields '*' spelling).
func (p *listProjection) wantsFullRows() bool {
	return p.full || p.fields == "*"
}

// renderProjectedList renders list rows honouring the projection flags.
// --oneline emits plain lines via line() — explicitly a non-JSON human
// mode, like `git log --oneline` — and composes only with --limit.
// --fields narrows the JSON rows through render.Project, preserving the
// requested key order. Contradictory flag mixes exit Usage.
func renderProjectedList[T any](cmd *cobra.Command, rows []T, p *listProjection, line func(T) string) error {
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
