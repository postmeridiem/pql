package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/parser"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

func newDecisionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decisions",
		Short: "Manage decision records from decisions/*.md",
		Long: `Parse, sync, and query decision records. Decisions live as markdown
in a decisions/ directory at the vault root and are indexed into
<vault>/.pql/pql.db by 'pql decisions sync'.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.AddCommand(newDecisionsSyncCmd())
	cmd.AddCommand(newDecisionsValidateCmd())
	cmd.AddCommand(newDecisionsClaimCmd())
	cmd.AddCommand(newDecisionsListCmd())
	cmd.AddCommand(newDecisionsShowCmd())
	cmd.AddCommand(newDecisionsCoverageCmd())
	cmd.AddCommand(newDecisionsRefsCmd())
	return cmd
}

func decisionsDir(vaultPath string) string {
	return filepath.Join(vaultPath, "decisions")
}

func openPlanningDB(ctx context.Context, cfg *config.Config) (*planning.DB, error) {
	return planning.Open(ctx, cfg.Vault.Path)
}

// --- sync ---

func newDecisionsSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Parse decisions/*.md and upsert into pql.db",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			dir := decisionsDir(cfg.Vault.Path)
			if _, err := os.Stat(dir); err != nil {
				return &exitError{
					code: diag.NoInput,
					msg:  fmt.Sprintf("decisions directory not found: %s", dir),
					hint: "create a decisions/ directory in the vault root",
				}
			}

			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			result, err := repo.SyncDecisions(ctx, pdb.SQL(), dir, cfg.Vault.Path)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(result, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

// --- validate ---

func newDecisionsValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Dry-run parse; exits non-zero on malformed records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			dir := decisionsDir(cfg.Vault.Path)
			ok, errs := parser.Validate(dir, cfg.Vault.Path)

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			type result struct {
				OK     bool     `json:"ok"`
				Errors []string `json:"errors,omitempty"`
			}
			if _, err := render.One(&result{OK: ok, Errors: errs}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if !ok {
				return &exitError{code: diag.DataErr}
			}
			return nil
		},
	}
}

// --- claim ---

func newDecisionsClaimCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claim <D|Q|R> <domain> <title>",
		Short: "Print next available ID (no side effects)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefix, domain, title := args[0], args[1], args[2]

			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			dir := decisionsDir(cfg.Vault.Path)
			records, _, err := parser.ParseAll(dir, cfg.Vault.Path)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			nextID := parser.NextID(records, prefix)

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			type claimResult struct {
				ID     string `json:"id"`
				Domain string `json:"domain"`
				Title  string `json:"title"`
			}
			if _, err := render.One(&claimResult{ID: nextID, Domain: domain, Title: title}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

// --- list ---

func newDecisionsListCmd() *cobra.Command {
	var typeFlag, domainFlag, statusFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List decisions from pql.db",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			decs, err := repo.ListDecisions(ctx, pdb.SQL(), repo.DecisionFilter{
				Type:   typeFlag,
				Domain: domainFlag,
				Status: statusFlag,
			})
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			n, err := render.Render(decs, rOpts)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if n == 0 {
				return errNoMatch
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFlag, "type", "", "filter by type (confirmed|question|rejected)")
	cmd.Flags().StringVar(&domainFlag, "domain", "", "filter by domain")
	cmd.Flags().StringVar(&statusFlag, "status", "", "filter by status (active|superseded|resolved|open)")
	return cmd
}

// --- show ---

func newDecisionsShowCmd() *cobra.Command {
	var withTickets, withRefs bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a single decision with optional joins",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			d, err := repo.GetDecision(ctx, pdb.SQL(), args[0])
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if d == nil {
				return &exitError{code: diag.NoInput, msg: fmt.Sprintf("decision %s not found", args[0])}
			}

			type showResult struct {
				repo.Decision
				Refs    []repo.DecisionRef   `json:"refs,omitempty"`
				Tickets []repo.TicketSummary  `json:"tickets,omitempty"`
			}
			out := showResult{Decision: *d}

			if withRefs {
				out.Refs, err = repo.RefsOf(ctx, pdb.SQL(), args[0])
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}
			if withTickets {
				out.Tickets, err = repo.TicketsForDecision(ctx, pdb.SQL(), args[0])
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(&out, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&withTickets, "with-tickets", false, "include linked tickets")
	cmd.Flags().BoolVar(&withRefs, "with-refs", false, "include cross-references")
	return cmd
}

// --- coverage ---

func newDecisionsCoverageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "coverage",
		Short: "List confirmed decisions without implementing tickets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			gaps, err := repo.Coverage(ctx, pdb.SQL())
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			n, err := render.Render(gaps, rOpts)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if n == 0 {
				return errNoMatch
			}
			return nil
		},
	}
}

// --- refs ---

func newDecisionsRefsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refs <id>",
		Short: "Show cross-references involving a decision",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			refs, err := repo.RefsOf(ctx, pdb.SQL(), args[0])
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			n, err := render.Render(refs, rOpts)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if n == 0 {
				return errNoMatch
			}
			return nil
		},
	}
}
