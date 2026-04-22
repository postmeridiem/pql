package cli

import (
	"context"
	"database/sql"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

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
