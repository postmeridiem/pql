package cli

import (
	"context"
	"database/sql"
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
	cmd.AddCommand(newDecisionsReadCmd())
	cmd.AddCommand(newDecisionsRefsCmd())
	return cmd
}

// decisionsDir resolves the per-vault DQR root. Honours the cfg.DQRDir
// knob (default `governance`, configurable per-vault via .pql/config.yaml
// per D-21) and falls back to the legacy `decisions/` if the configured
// dir doesn't exist but the legacy one does. The fallback lets older
// repos keep working until they migrate.
func decisionsDir(cfg *config.Config) string {
	dqr := cfg.DQRDir
	if dqr == "" {
		dqr = "governance"
	}
	primary := filepath.Join(cfg.Vault.Path, dqr)
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	legacy := filepath.Join(cfg.Vault.Path, "decisions")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return primary
}

func openPlanningDB(ctx context.Context, cfg *config.Config) (*planning.DB, error) {
	return planning.Open(ctx, cfg.Vault.Path)
}

// --- sync ---

func newDecisionsSyncCmd() *cobra.Command {
	var noStyle bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Parse decisions/*.md and upsert into pql.db",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			dir := decisionsDir(cfg)
			if _, err := os.Stat(dir); err != nil {
				return &exitError{
					code: diag.NoInput,
					msg:  fmt.Sprintf("DQR directory not found: %s", dir),
					hint: "run `pql init` to plant the default `governance/` layout, or set dqr_dir in .pql/config.yaml",
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

			// Surface style warnings on stderr unless explicitly
			// suppressed. The drift is most noticeable at sync time, so
			// this is the natural teachable moment.
			if !noStyle {
				_, _, warnings := parser.Validate(dir, cfg.Vault.Path)
				for _, w := range warnings {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warn: %s\n", w)
				}
			}

			// Regenerate the auto-managed records section in
			// <dqr_root>/README.md so the human-readable index stays
			// in step with pql.db. Silent on no-op; failure is non-fatal
			// (sync succeeded, the README just stayed stale).
			if _, err := regenerateDQRReadme(ctx, pdb.SQL(), dir); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warn: regenerate README: %v\n", err)
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
	cmd.Flags().BoolVar(&noStyle, "no-style", false, "suppress style warnings on sync")
	return cmd
}

// --- validate ---

func newDecisionsValidateCmd() *cobra.Command {
	var noStyle bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Dry-run parse; exits non-zero on malformed records",
		Long: `Run the parser without writing to pql.db. Reports two streams:

  - errors:   structural problems — duplicate IDs, empty titles,
              broken cross-references, parse failures. Exits non-zero.
  - warnings: style-class — filename convention (lowercase / hyphenated),
              subdir-heading mismatch under the D-21 layout. Exits zero.

Use --no-style to suppress warnings (errors are always shown).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			dir := decisionsDir(cfg)
			ok, errs, warnings := parser.Validate(dir, cfg.Vault.Path)
			if noStyle {
				warnings = nil
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			type result struct {
				OK       bool     `json:"ok"`
				Errors   []string `json:"errors,omitempty"`
				Warnings []string `json:"warnings,omitempty"`
			}
			if _, err := render.One(&result{OK: ok, Errors: errs, Warnings: warnings}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if !ok {
				return &exitError{code: diag.DataErr}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noStyle, "no-style", false, "suppress style warnings (filename, subdir-mismatch); structural errors are always shown")
	return cmd
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

			dir := decisionsDir(cfg)
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

// decisionShowTree is the canonical join-tree shape for decision-
// centric surfaces. Mirrors ticketShowTree: embedded *repo.Decision
// promotes the record's fields to the top level, with optional refs
// and tickets siblings. New decision-anchored verbs should render
// through buildDecisionTree so the JSON shape stays uniform.
type decisionShowTree struct {
	*repo.Decision
	Refs    []repo.DecisionRef   `json:"refs,omitempty"`
	Tickets []repo.TicketSummary `json:"tickets,omitempty"`
}

// buildDecisionTree assembles the decision show-tree from the
// requested joins. A nil decision returns an empty tree — callers
// should treat that as "not found" upstream.
func buildDecisionTree(ctx context.Context, db *sql.DB, d *repo.Decision, withRefs, withTickets bool) (*decisionShowTree, error) {
	if d == nil {
		return &decisionShowTree{}, nil
	}
	out := &decisionShowTree{Decision: d}
	if withRefs {
		refs, err := repo.RefsOf(ctx, db, d.ID)
		if err != nil {
			return nil, err
		}
		out.Refs = refs
	}
	if withTickets {
		tks, err := repo.TicketsForDecision(ctx, db, d.ID)
		if err != nil {
			return nil, err
		}
		out.Tickets = tks
	}
	return out, nil
}

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

			out, err := buildDecisionTree(ctx, pdb.SQL(), d, withRefs, withTickets)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(out, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&withTickets, "with-tickets", false, "include linked tickets")
	cmd.Flags().BoolVar(&withRefs, "with-refs", false, "include cross-references")
	return cmd
}

// --- read ---

func newDecisionsReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "read <id>",
		Short: "Read a record with its full markdown body",
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

			d, err := repo.ReadDecision(ctx, pdb.SQL(), cfg.Vault.Path, args[0])
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if d == nil {
				return &exitError{code: diag.NoInput, msg: fmt.Sprintf("decision %s not found", args[0])}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(d, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
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
