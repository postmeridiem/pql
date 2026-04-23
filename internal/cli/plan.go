package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

const defaultSnapshotFile = ".pql/pql-plan.json"

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Cross-cutting planning dashboard and search",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.AddCommand(newPlanStatusCmd())
	cmd.AddCommand(newPlanExportCmd())
	cmd.AddCommand(newPlanImportCmd())
	return cmd
}

// --- status ---

func newPlanStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Planning dashboard: decision counts, open Qs, ticket summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			db := pdb.SQL()
			dash, err := buildDashboard(ctx, db)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(dash, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

type dashboard struct {
	Decisions    decisionSummary `json:"decisions"`
	Tickets      ticketSummary   `json:"tickets"`
	CoverageGaps int            `json:"coverage_gaps"`
}

type decisionSummary struct {
	Total      int `json:"total"`
	Confirmed  int `json:"confirmed"`
	Questions  int `json:"questions"`
	Rejected   int `json:"rejected"`
	OpenQs     int `json:"open_questions"`
}

type ticketSummary struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"by_status"`
}

func buildDashboard(ctx context.Context, db *sql.DB) (*dashboard, error) {
	var d dashboard

	rows, err := db.QueryContext(ctx, `
		SELECT type, status, COUNT(*) FROM decisions GROUP BY type, status
	`)
	if err != nil {
		return nil, err
	}
	d.Tickets.ByStatus = make(map[string]int)

	for rows.Next() {
		var typ, status string
		var count int
		if err := rows.Scan(&typ, &status, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		d.Decisions.Total += count
		switch typ {
		case "confirmed":
			d.Decisions.Confirmed += count
		case "question":
			d.Decisions.Questions += count
			if status == "open" {
				d.Decisions.OpenQs += count
			}
		case "rejected":
			d.Decisions.Rejected += count
		}
	}
	_ = rows.Close()

	trows, err := db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM tickets GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var status string
		var count int
		if err := trows.Scan(&status, &count); err != nil {
			_ = trows.Close()
			return nil, err
		}
		d.Tickets.Total += count
		d.Tickets.ByStatus[status] = count
	}
	_ = trows.Close()

	gaps, err := repo.Coverage(ctx, db)
	if err != nil {
		return nil, err
	}
	d.CoverageGaps = len(gaps)

	return &d, nil
}

// --- export ---

func newPlanExportCmd() *cobra.Command {
	var outFile string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export planning state to a JSON file for version control",
		Long: `Dump all planning state (decisions, tickets, refs, deps, labels,
history) to a single JSON file. The file is meant to be committed
to git — pql.db itself stays gitignored.

  pql plan export                         # writes .pql/pql-plan.json
  pql plan export --to planning.json      # custom filename

Automatically wired by pql init: a pre-commit hook at .pql/hooks/pre-commit
runs plan export and stages the file if it changed. Manual export is also
fine — run it before committing when planning state changed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			snap, err := repo.Export(ctx, pdb.SQL())
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			data, err := json.MarshalIndent(snap, "", "  ")
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			if err := os.WriteFile(outFile, append(data, '\n'), 0o644); err != nil { //nolint:gosec // G306: export file is meant to be committed
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "exported to %s\n", outFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&outFile, "to", defaultSnapshotFile, "output file path")
	return cmd
}

// --- import ---

func newPlanImportCmd() *cobra.Command {
	var inFile string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Restore planning state from a JSON export",
		Long: `Import planning state from a pql plan export file. Existing data
is upserted (decisions, tickets) or replaced (refs, deps, labels,
history).

  pql plan import                         # reads .pql/pql-plan.json
  pql plan import --from planning.json    # custom filename`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			data, err := os.ReadFile(inFile) //nolint:gosec // G304: user-specified import file
			if err != nil {
				return &exitError{code: diag.NoInput, msg: fmt.Sprintf("read %s: %v", inFile, err)}
			}

			var snap repo.Snapshot
			if err := json.Unmarshal(data, &snap); err != nil {
				return &exitError{code: diag.DataErr, msg: fmt.Sprintf("parse %s: %v", inFile, err)}
			}

			if err := repo.Import(ctx, pdb.SQL(), &snap); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "imported from %s (%d decisions, %d tickets)\n",
				inFile, len(snap.Decisions), len(snap.Tickets))
			return nil
		},
	}
	cmd.Flags().StringVar(&inFile, "from", defaultSnapshotFile, "input file path")
	return cmd
}
