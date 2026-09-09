package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning/changelog"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

// --- redact ---

func newTicketRedactCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "redact <id> <value> <replacement>",
		Short: "Remove a value from a ticket's prose and its changelog history",
		Long: `Remove every occurrence of a value from one ticket's prose,
everywhere it lives: the ticket row, its history (including old_value
copies, which is where a fixed-forward leak keeps living), and the
committed changelog lines carrying those rows. Database and changelog
move together, so a scrub cannot silently come back (D-35, T-105).

This rewrites unpushed history only. If the value already appears in a
remote's copy of the changelog it is published, and the command refuses:
removing it locally would accomplish nothing and hide that it is out.
Redacting published history means rewriting git history — see D-35.

The replacement may be an empty string to delete the value outright:

  pql ticket redact T-42 "hostname.internal" "a build host"
  pql ticket redact T-42 "s3cret-token" ""

Output is a receipt: rows and changelog lines rewritten, files touched,
and which remote refs were checked for the value.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, oldValue, newValue := args[0], args[1], args[2]

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDBForMutation(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = pdb.Close() }()

			recordID, err := repo.ResolveRecordID(ctx, pdb.SQL(), id)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			res, err := changelog.Redact(ctx, pdb.SQL(), cfg.Vault.Path, recordID, oldValue, newValue)
			if err != nil {
				if errors.Is(err, changelog.ErrRedactPushed) {
					return &exitError{
						code: diag.DataErr,
						msg:  err.Error(),
						hint: "published history is immutable to pql; redacting it means rewriting git history with the changelog restored and rebuilt — see D-35 in governance/decisions/architecture.md",
					}
				}
				if strings.Contains(err.Error(), "value not found") {
					return &exitError{code: diag.DataErr, msg: err.Error()}
				}
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(res, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}
