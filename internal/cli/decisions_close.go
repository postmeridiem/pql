package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning/parser"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

type closeResult struct {
	ID              string `json:"id"`
	Relation        string `json:"relation"`
	Into            string `json:"into,omitempty"`
	SubjectFile     string `json:"subject_file"`
	TargetFile      string `json:"target_file,omitempty"`
	StatusLine      string `json:"status_line"`
	BackLink        string `json:"back_link,omitempty"`
	BackLinkSkipped string `json:"back_link_skipped,omitempty"`
	Synced          int    `json:"synced"`
}

// newDecisionsCloseCmd records a record's terminal disposition.
//
// This is the only verb that writes the user's own markdown, and it has to be:
// decisions are markdown-sourced (D-8), so a status written to pql.db alone
// would be reverted by the next `decisions sync`. `ticket relabel --fix-prose`
// set the precedent for editing DQR prose in place.
func newDecisionsCloseCmd() *cobra.Command {
	var into string
	var obsolete bool
	cmd := &cobra.Command{
		Use:   "close <id> --into <id> | --obsolete",
		Short: "Close a record into the one that settles it, or as obsolete",
		Long: `Record that a question was answered, or that a decision was replaced.

  pql decisions close Q-2  --into D-23    question answered by a decision
  pql decisions close Q-4  --into R-1     question answered "no"
  pql decisions close D-13 --into D-15    decision replaced by a later one
  pql decisions close Q-7  --obsolete     stopped mattering; no longer relevant

The relation follows from the pair of record types, so it is never named at the
call site: a question closed by a decision or a rejection is *resolved*, a
decision closed by a decision is *superseded*. A pair that is not a closure is
refused with the supported routes listed — notably a question into a question,
which is a cross-reference rather than a closure and leaves both records open.

The edit lands in the markdown, not just pql.db, because the DQR tree is the
source of truth for decisions (D-8) — a status written only to the database is
undone by the next sync. pql.db and the DQR README are both refreshed before
the command returns, so neither the queryable copy nor the human-readable index
lags behind the files.

For a resolution one rewritten **Status:** line is enough for both records to
show the link, because refs resolve from either end. Supersession additionally
writes **Supersedes:** on the newer record. Either way an existing field is
left exactly as written and the receipt says so, rather than overwriting prose
to insert a link the database already holds.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

			switch {
			case into == "" && !obsolete:
				return &exitError{
					code: diag.Usage,
					msg:  "one of --into <id> or --obsolete is required",
					hint: "a record is closed by what settles it, or by nothing: `--into D-23`, or `--obsolete`",
				}
			case into != "" && obsolete:
				return &exitError{
					code: diag.Usage,
					msg:  "--into and --obsolete are mutually exclusive",
					hint: "--obsolete means the record stopped mattering; naming a target contradicts that",
				}
			}

			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			dir := decisionsDir(cfg)
			records, _, err := parser.ParseAll(dir, cfg.Vault.Path)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			// Both arguments NAME a record rather than filtering a set, so
			// both are validated and an unknown one is an error rather than
			// an empty result (D-29).
			subject, err := findRecord(records, id)
			if err != nil {
				return err
			}
			if isTerminalStatus(subject.Status) {
				return &exitError{
					code: diag.Usage,
					msg:  fmt.Sprintf("%s is already %s", subject.ID, subject.Status),
					hint: "edit the **Status:** line in " + subject.FilePath + " to point somewhere else",
				}
			}

			var target *parser.Record
			if into != "" {
				target, err = findRecord(records, into)
				if err != nil {
					return err
				}
				// Surfaces the unsupported-pair message, which lists every
				// route out — the program knows the accepted set at the
				// moment it rejects the input, so it prints it.
				if _, relErr := parser.RelationFor(*subject, *target); relErr != nil {
					return &exitError{code: diag.Usage, msg: relErr.Error()}
				}
			}

			outcome, err := parser.Close(cfg.Vault.Path, *subject, target)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			// Re-sync so the change is queryable now. The markdown is already
			// written at this point, so a sync failure is reported without
			// discarding the edit — re-running `decisions sync` recovers it.
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			synced, err := repo.SyncDecisions(ctx, pdb.SQL(), dir, cfg.Vault.Path)
			if err != nil {
				return &exitError{
					code: diag.Software,
					msg:  fmt.Sprintf("markdown written but sync failed: %v", err),
					hint: "run `pql decisions sync` to pick the edit up",
				}
			}

			// The README's records section splits questions into open and
			// resolved buckets, so skipping this would leave it contradicting
			// the database. Non-fatal, as in `decisions sync`.
			if _, err := regenerateDQRReadme(ctx, pdb.SQL(), dir); err != nil {
				diag.Warn("decisions.readme_regen", fmt.Sprintf("regenerate README: %v", err))
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			result := &closeResult{
				ID:              subject.ID,
				Relation:        string(outcome.Relation),
				Into:            into,
				SubjectFile:     outcome.SubjectFile,
				TargetFile:      outcome.TargetFile,
				StatusLine:      outcome.StatusLine,
				BackLink:        outcome.BackLink,
				BackLinkSkipped: outcome.BackLinkSkipped,
				Synced:          synced.Synced,
			}
			if _, err := render.One(result, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&into, "into", "", "id of the record that settles this one")
	cmd.Flags().BoolVar(&obsolete, "obsolete", false, "close with no target: the record stopped mattering and is no longer relevant")
	return cmd
}

// isTerminalStatus reports whether a record has already been closed. Both
// terminal values are refused rather than silently re-pointed, so a second
// close has to be a deliberate edit.
func isTerminalStatus(s string) bool {
	return s == "resolved" || s == "superseded"
}

// findRecord looks up one parsed record by id. The type is not checked here —
// which pairs are legal is RelationFor's job, and it can say far more about a
// wrong pair than a lookup can about a wrong type.
func findRecord(records []parser.Record, id string) (*parser.Record, error) {
	for i := range records {
		if records[i].ID == id {
			return &records[i], nil
		}
	}
	return nil, &exitError{
		code: diag.NoInput,
		msg:  fmt.Sprintf("%s not found in the DQR tree", id),
		hint: "`pql decisions list --oneline` lists every record id",
	}
}
