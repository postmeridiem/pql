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

type resolveResult struct {
	QuestionID      string `json:"question_id"`
	DecisionID      string `json:"decision_id"`
	Status          string `json:"status"`
	QuestionFile    string `json:"question_file"`
	DecisionFile    string `json:"decision_file"`
	StatusLine      string `json:"status_line"`
	BackLink        string `json:"back_link,omitempty"`
	BackLinkSkipped string `json:"back_link_skipped,omitempty"`
	Synced          int    `json:"synced"`
}

// newDecisionsResolveCmd closes a question into the decision that answers it.
//
// This is the only verb that writes the user's own markdown, and it has to be:
// decisions are markdown-sourced (D-8), so a status written to pql.db alone
// would be reverted by the next `decisions sync`. `ticket relabel --fix-prose`
// set the precedent for editing DQR prose in place.
func newDecisionsResolveCmd() *cobra.Command {
	var into string
	cmd := &cobra.Command{
		Use:   "resolve <question-id> --into <decision-id>",
		Short: "Close a question into the decision that answers it",
		Long: `Mark a question resolved and record which decision resolved it.

  pql decisions resolve Q-2 --into D-23

pql could already mint a record id with ` + "`decisions claim`" + ` but had no
counterpart for closing one, so every vault invented its own convention for the
Q -> D transition and none of them were enforced or queryable.

The edit lands in the markdown, not just pql.db, because the DQR tree is the
source of truth for decisions (D-8) — a status written only to the database is
undone by the next sync. The question's **Status:** line becomes:

  - **Status:** Resolved → [D-23](../decisions/<domain>.md#d-23-<slug>)

That single line is enough for both records: refs are looked up on either end,
so ` + "`decisions refs Q-2`" + ` and ` + "`decisions show D-23 --with-refs`" + `
both surface the link from it. The decision additionally gets a readable
**Raised by:** line when it has none; when it already has one that field is left
exactly as written, because it carries free prose about where a record came
from and overwriting it would destroy something to duplicate something.

pql.db is re-synced from the markdown before the command returns, so the change
is queryable immediately rather than pending a step the caller has to remember.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			qID := args[0]

			if into == "" {
				return &exitError{
					code: diag.Usage,
					msg:  "--into <decision-id> is required",
					hint: "a question is resolved BY something; `pql decisions resolve Q-2 --into D-23`",
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
			q, err := findRecord(records, qID, "question")
			if err != nil {
				return err
			}
			d, err := findRecord(records, into, "confirmed")
			if err != nil {
				return err
			}

			if q.Status == "resolved" {
				return &exitError{
					code: diag.Usage,
					msg:  fmt.Sprintf("%s is already resolved", q.ID),
					hint: "edit the **Status:** line in " + q.FilePath + " to point somewhere else",
				}
			}

			outcome, err := parser.ResolveQuestion(cfg.Vault.Path, *q, *d)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			// Re-sync so the answer is queryable now. The markdown is already
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

			// Refresh the human-readable index too. Its records section splits
			// questions into open and resolved buckets, so skipping this would
			// leave the README still listing the question as open — the exact
			// staleness this verb exists to remove, just one file over. Same
			// non-fatal treatment as `decisions sync`: the resolution has
			// landed either way, the index merely lags.
			if _, err := regenerateDQRReadme(ctx, pdb.SQL(), dir); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warn: regenerate README: %v\n", err)
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			result := &resolveResult{
				QuestionID:      q.ID,
				DecisionID:      d.ID,
				Status:          "resolved",
				QuestionFile:    outcome.QuestionFile,
				DecisionFile:    outcome.DecisionFile,
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
	cmd.Flags().StringVar(&into, "into", "", "id of the decision that resolves the question (required)")
	return cmd
}

// findRecord looks up one parsed record by id and checks its type. Both the
// missing case and the wrong-type case name what was expected: an id that
// exists but is the wrong kind is the likelier mistake of the two, and
// "Q-2 not found" would be a misleading way to say it.
func findRecord(records []parser.Record, id, wantType string) (*parser.Record, error) {
	for i := range records {
		if records[i].ID != id {
			continue
		}
		if records[i].Type != wantType {
			return nil, &exitError{
				code: diag.Usage,
				msg: fmt.Sprintf("%s is a %s record, not a %s",
					id, records[i].Type, wantType),
			}
		}
		return &records[i], nil
	}
	return nil, &exitError{
		code: diag.NoInput,
		msg:  fmt.Sprintf("%s not found in the DQR tree", id),
		hint: "`pql decisions list --oneline` lists every record id",
	}
}
