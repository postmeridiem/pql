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
	sync := newDecisionsSyncCmd()
	markMutation(sync) // writes pql.db; its output is a summary of what it did
	cmd.AddCommand(sync)
	cmd.AddCommand(newDecisionsValidateCmd())
	cmd.AddCommand(newDecisionsClaimCmd())
	closeCmd := newDecisionsCloseCmd()
	markMutation(closeCmd) // writes markdown and re-syncs; the receipt names both
	cmd.AddCommand(closeCmd)
	cmd.AddCommand(newDecisionsListCmd())
	cmd.AddCommand(newDecisionsShowCmd())
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
	var proj *projection
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List decisions from pql.db",
		Long: `List decisions, optionally filtered.

Decision rows are already light (bodies stay in the DQR markdown), so
whole rows are the default. --fields id,status,title narrows the JSON
columns and --oneline emits a plain id<TAB>status<TAB>title index — the
same projection surface as ` + "`pql ticket list`" + ` (D-27).`,
		Args: cobra.NoArgs,
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

			return renderProjectedList(cmd, decs, proj, func(d repo.Decision) string {
				return d.ID + "\t" + d.Status + "\t" + d.Title
			})
		},
	}
	cmd.Flags().StringVar(&typeFlag, "type", "", "filter by type (confirmed|question|rejected)")
	cmd.Flags().StringVar(&domainFlag, "domain", "", "filter by domain")
	cmd.Flags().StringVar(&statusFlag, "status", "", "filter by status (active|superseded|resolved|open)")
	proj = addProjectionFlags(cmd, projectionFlags{
		Example: "id,status,title",
		Oneline: "id<TAB>status<TAB>title",
	})
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
	// Body and Headings come from the markdown file, not pql.db — the
	// record's prose has no column. They are part of the record rather
	// than a join: asking for a decision by id and receiving everything
	// except what it says was the defect in T-122, where `show` answered
	// with a header, said nothing about the omission, and sent callers to
	// slice line ranges out of the file by hand.
	Body     string           `json:"body,omitempty"`
	Headings []parser.Heading `json:"headings,omitempty"`
	Refs     []repo.DecisionRef   `json:"refs,omitempty"`
	Tickets  []repo.TicketSummary `json:"tickets,omitempty"`
}

// buildDecisionTree assembles the decision show-tree from the
// requested joins. A nil decision returns an empty tree — callers
// should treat that as "not found" upstream.
func buildDecisionTree(ctx context.Context, db *sql.DB, d *repo.Decision, detail *repo.DecisionDetail, withRefs, withTickets bool) (*decisionShowTree, error) {
	if d == nil {
		return &decisionShowTree{}, nil
	}
	out := &decisionShowTree{Decision: d}
	if detail != nil {
		out.Body = detail.Body
		out.Headings = detail.Headings
	}
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
	var fields string
	cmd := &cobra.Command{
		Use:     "show <id[,id,...]>",
		Aliases: []string{"read"},
		Short:   "Show one or more decisions, including the record's markdown body",
		Long: `Show decisions. Use commas to batch:

  pql decisions show D-1
  pql decisions show D-1,D-2,D-3 --with-tickets

The record's markdown body and its heading anchors are included by
default: asking for a record by id gives you the record, not a card
about it. ` + "`read`" + ` is an alias of this command and behaves
identically — the two verbs were folded together in T-122, where the
obvious verb answered incompletely and was silent about having done so.

The body is read from the source markdown rather than pql.db, which has
no column for it. That read is skipped when --fields is given and names
neither ` + "`body`" + ` nor ` + "`headings`" + `, so the compact card
costs no file access:

  pql decisions show D-1,D-2,D-3 --fields id,title,status

A single ID renders a single show-tree object; multiple IDs render an
array of show-trees in the order given. Any unknown ID fails the call.
Same batching rule as ` + "`pql ticket show`" + `.

--fields narrows each record to the named keys, same vocabulary as
` + "`decisions list`" + ` plus ` + "`body`" + ` and ` + "`headings`" + `.
It projects the top level only: the refs and tickets joins are
all-or-nothing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])

			// The body is the expensive part — one file read and parse per
			// record. A caller who projected it away has said they do not
			// want it, so a batched card query stays as cheap as it was
			// before the body became a default.
			needBody := projectionWants(fields, "body", "headings")

			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			trees := make([]*decisionShowTree, 0, len(ids))
			for _, id := range ids {
				var (
					d      *repo.Decision
					detail *repo.DecisionDetail
				)
				if needBody {
					detail, err = repo.ReadDecision(ctx, pdb.SQL(), cfg.Vault.Path, id)
					if err != nil {
						return &exitError{code: diag.Software, msg: err.Error()}
					}
					if detail != nil {
						d = &detail.Decision
					}
				} else {
					d, err = repo.GetDecision(ctx, pdb.SQL(), id)
					if err != nil {
						return &exitError{code: diag.Software, msg: err.Error()}
					}
				}
				if d == nil {
					return &exitError{code: diag.NoInput, msg: fmt.Sprintf("decision %s not found", id)}
				}
				tree, err := buildDecisionTree(ctx, pdb.SQL(), d, detail, withRefs, withTickets)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				trees = append(trees, tree)
			}

			// One id keeps the single-object shape it has always had; a
			// batch renders an array. Same rule as `ticket show`, so a
			// caller that learned one knows the other.
			return renderShowRecords(cmd, trees, fields)
		},
	}
	cmd.Flags().BoolVar(&withTickets, "with-tickets", false, "include linked tickets")
	cmd.Flags().BoolVar(&withRefs, "with-refs", false, "include cross-references")
	cmd.Flags().StringVar(&fields, "fields", "", showFieldsHelp)
	return cmd
}

// `read` is no longer a command of its own. It is an alias on `show`, which
// now returns the body by default — see newDecisionsShowCmd and T-122. The
// alias is kept rather than dropped because every skill copy installed in the
// field still documents `decisions read`, and an agent running one would get
// exit 64 on a verb that had simply moved.

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
			if _, err := render.Render(refs, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}
