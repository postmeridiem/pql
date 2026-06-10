package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

// ticketShowTree is the canonical join-tree shape used by every
// ticket-centric surface — `ticket show`, `ticket refine next/write`,
// `plan whatsnext`, `plan review`. Embedding *repo.Ticket promotes its
// fields to the top level; when the pointer is nil only Message
// renders, which lets the planning verbs report "no actionable ticket"
// without inventing a parallel shape.
type ticketShowTree struct {
	*repo.Ticket
	Ancestors []repo.Ticket        `json:"ancestors,omitempty"`
	Decisions []repo.Decision      `json:"decisions,omitempty"`
	Blockers  []repo.BlockerInfo   `json:"blockers,omitempty"`
	Children  []repo.TicketSummary `json:"children,omitempty"`
	Subtree   []repo.TreeNode      `json:"subtree,omitempty"`
	Message   string               `json:"message,omitempty"`
}

// showOpts selects which joins buildShowTree assembles.
//
// withContext pulls the full ancestor spine to the root plus every
// linked decision; tree pulls the recursive descendant subtree plus the
// direct parent only (the cheap "why does this subtree exist" context,
// without the full spine). withContext is a superset of tree's parent,
// so when both are set the full spine wins.
type showOpts struct {
	withContext  bool
	withBlockers bool
	withChildren bool
	tree         bool
	treeDepth    int
}

// fullTree is the join set used by the planning verbs and refinement
// flows: ancestors, blockers, and direct children, but not the
// recursive subtree (those surfaces zoom in on one ticket).
var fullTree = showOpts{withContext: true, withBlockers: true, withChildren: true}

// buildShowTree assembles the ticket show-tree from the requested
// joins. A nil ticket returns an empty tree — callers that want a
// "nothing to do" response set Message on the result.
func buildShowTree(ctx context.Context, db *sql.DB, t *repo.Ticket, o showOpts) (*ticketShowTree, error) {
	if t == nil {
		return &ticketShowTree{}, nil
	}
	out := &ticketShowTree{Ticket: t}

	switch {
	case o.withContext:
		ancestors, err := repo.Ancestors(ctx, db, t)
		if err != nil {
			return nil, err
		}
		if len(ancestors) > 0 {
			out.Ancestors = ancestors
		}
		for _, ref := range collectDecisionRefs(t, ancestors) {
			d, err := repo.GetDecision(ctx, db, ref)
			if err != nil {
				return nil, err
			}
			if d != nil {
				out.Decisions = append(out.Decisions, *d)
			}
		}
	case o.tree && t.ParentID != nil:
		// Direct parent only: paints why the subtree exists without the
		// cost of the full ancestor spine + decision joins.
		p, err := repo.GetTicket(ctx, db, *t.ParentID)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out.Ancestors = []repo.Ticket{*p}
		}
	}
	if o.withChildren || o.withContext {
		children, err := repo.ChildrenOf(ctx, db, t.ID)
		if err != nil {
			return nil, err
		}
		if len(children) > 0 {
			out.Children = children
		}
	}
	if o.tree {
		sub, err := repo.Subtree(ctx, db, t.ID, o.treeDepth)
		if err != nil {
			return nil, err
		}
		out.Subtree = sub
	}
	if o.withBlockers {
		blockers, err := repo.BlockersOf(ctx, db, t.ID)
		if err != nil {
			return nil, err
		}
		out.Blockers = blockers
	}
	return out, nil
}

// collectDecisionRefs walks a ticket and its ancestors, returning the
// distinct decision IDs referenced. Order is ticket-first then root-up.
func collectDecisionRefs(t *repo.Ticket, ancestors []repo.Ticket) []string {
	seen := map[string]bool{}
	var refs []string
	add := func(ref *string) {
		if ref != nil && !seen[*ref] {
			seen[*ref] = true
			refs = append(refs, *ref)
		}
	}
	add(t.DecisionRef)
	for i := range ancestors {
		add(ancestors[i].DecisionRef)
	}
	return refs
}

// parseIDs splits a comma-separated ID argument into one or more IDs.
// A single ID passes through unchanged; commas signal a batch:
//
//	"T-001"             → ["T-001"]
//	"T-001,T-002,T-003" → ["T-001", "T-002", "T-003"]
func parseIDs(arg string) []string {
	parts := strings.Split(arg, ",")
	ids := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

func newTicketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Manage tickets in pql.db",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.AddCommand(newTicketNewCmd())
	cmd.AddCommand(newTicketListCmd())
	cmd.AddCommand(newTicketShowCmd())
	cmd.AddCommand(newTicketStatusCmd())
	cmd.AddCommand(newTicketStatusListCmd())
	cmd.AddCommand(newTicketRelabelCmd())
	cmd.AddCommand(newTicketAssignCmd())
	cmd.AddCommand(newTicketSetParentCmd())
	cmd.AddCommand(newTicketBlockCmd())
	cmd.AddCommand(newTicketUnblockCmd())
	cmd.AddCommand(newTicketTeamCmd())
	cmd.AddCommand(newTicketLabelCmd())
	cmd.AddCommand(newTicketBoardCmd())
	cmd.AddCommand(newTicketAppendCmd())
	cmd.AddCommand(newTicketRefineCmd())
	return cmd
}

// --- append ---

func newTicketAppendCmd() *cobra.Command {
	var fromFile string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "append <id> [text]",
		Short: "Append text to a ticket's description",
		Long: `Append text to a ticket's description, separated from existing
content by a blank line. When the description is empty the text becomes
the whole description. Unlike ` + "`refine write`" + ` (which replaces the
description from a JSON patch), this never round-trips the existing
text, so accumulating notes is one call.

Three input modes (mutually exclusive), mirroring ` + "`refine write`" + `:

  pql ticket append T-5 "refined the cache TTL to 5m after benchmarking"
  pql ticket append T-5 --file note.md
  pql ticket append T-5 --stdin

Output is the updated ticket row, so the caller can verify the append.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

			content, err := readContentArg(args[1:], fromFile, fromStdin, cmd.InOrStdin(), "pql ticket append")
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			if err := repo.AppendDescription(ctx, pdb.SQL(), id, string(content), ""); err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}
			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}

			tk, err := repo.GetTicket(ctx, pdb.SQL(), id)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if tk == nil {
				return &exitError{code: diag.NoInput, msg: fmt.Sprintf("ticket %s not found", id)}
			}
			return renderTicketResults(cmd, []repo.Ticket{*tk})
		},
	}
	cmd.Flags().StringVar(&fromFile, "file", "", "read the text from this file path")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read the text from stdin")
	return cmd
}

// --- new ---

func newTicketNewCmd() *cobra.Command {
	var parentID, priority, decisionRef, team, assignedTo, description string
	var idOnly bool
	cmd := &cobra.Command{
		Use:   "new <type> <title>",
		Short: "Create a new ticket",
		Long:  `Type must be one of: initiative, epic, story, task, bug.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			id, err := repo.CreateTicket(ctx, pdb.SQL(), repo.NewTicketOpts{
				Type:          args[0],
				Title:         args[1],
				Description:   description,
				ParentID:      parentID,
				Priority:      priority,
				DecisionRef:   decisionRef,
				Team:          team,
				AssignedTo:    assignedTo,
				DefaultStatus: statusSetFromConfig(cfg).Default(),
			})
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}

			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}

			// --id-only keeps tree-creation scripts simple: the bare
			// T-NNN feeds straight into the next call's --parent/--by
			// without a JSON parse.
			if idOnly {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), id); err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				return nil
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			type newResult struct {
				ID string `json:"id"`
			}
			if _, err := render.One(&newResult{ID: id}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&parentID, "parent", "", "parent ticket ID (e.g. T-1)")
	cmd.Flags().StringVar(&priority, "priority", "medium", "priority (critical|high|medium|low)")
	cmd.Flags().StringVar(&decisionRef, "decision", "", "linked decision ID (e.g. D-1)")
	cmd.Flags().StringVar(&team, "team", "", "team name")
	cmd.Flags().StringVar(&assignedTo, "assign", "", "assignee")
	cmd.Flags().StringVar(&description, "description", "", "ticket description")
	cmd.Flags().BoolVar(&idOnly, "id-only", false, "print only the new ticket id (T-NNN), no JSON")
	return cmd
}

// --- list ---

func newTicketListCmd() *cobra.Command {
	var statusFlag, teamFlag, assignedFlag, decisionFlag, labelFlag, underFlag string
	var leafFlag, unblockedFlag bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tickets from pql.db",
		Long: `List tickets, optionally filtered. Filters compose (AND):

  pql ticket list --status ready
  pql ticket list --under T-2 --leaf --unblocked

--under restricts to the recursive descendants of a ticket; --leaf to
tickets with no children; --unblocked to tickets whose blockers have all
reached a terminal status. Composed, they answer "what leaf work under this
epic is ready to pick up" — the batch complement to ` + "`pql plan whatsnext`" + `.`,
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

			tks, err := repo.ListTickets(ctx, pdb.SQL(), repo.TicketFilter{
				Status:      statusFlag,
				Team:        teamFlag,
				AssignedTo:  assignedFlag,
				DecisionRef: decisionFlag,
				Label:       labelFlag,
				Under:       underFlag,
				Leaf:        leafFlag,
				Unblocked:   unblockedFlag,
				Statuses:    statusSetFromConfig(cfg),
			})
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.Render(tks, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&statusFlag, "status", "", "filter by status")
	cmd.Flags().StringVar(&teamFlag, "team", "", "filter by team")
	cmd.Flags().StringVar(&assignedFlag, "assigned", "", "filter by assignee")
	cmd.Flags().StringVar(&decisionFlag, "decision", "", "filter by linked decision")
	cmd.Flags().StringVar(&labelFlag, "label", "", "filter by label")
	cmd.Flags().StringVar(&underFlag, "under", "", "restrict to recursive descendants of this ticket")
	cmd.Flags().BoolVar(&leafFlag, "leaf", false, "restrict to tickets with no children")
	cmd.Flags().BoolVar(&unblockedFlag, "unblocked", false, "restrict to tickets whose blockers have all reached a terminal status")
	return cmd
}

// --- show ---

func newTicketShowCmd() *cobra.Command {
	var withContext, withBlockers, withChildren, tree bool
	var depth int
	cmd := &cobra.Command{
		Use:   "show <id[,id,...]>",
		Short: "Show one or more tickets with optional joins",
		Long: `Show tickets with optional joins. Use commas to batch:

  pql ticket show T-001
  pql ticket show T-001,T-002,T-003 --with-context

A single ID renders a single show-tree object; multiple IDs render an
array of show-trees in the order given. Any unknown ID fails the call.

--with-children lists direct children only. --tree instead nests the
full descendant subtree (cap depth with --depth N) and includes the
direct parent for context. --with-context pulls the full ancestor spine
to the root plus linked decisions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			trees := make([]*ticketShowTree, 0, len(ids))
			for _, id := range ids {
				tk, err := repo.GetTicket(ctx, pdb.SQL(), id)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				if tk == nil {
					return &exitError{code: diag.NoInput, msg: fmt.Sprintf("ticket %s not found", id)}
				}
				showTree, err := buildShowTree(ctx, pdb.SQL(), tk, showOpts{
					withContext:  withContext,
					withBlockers: withBlockers,
					withChildren: withChildren,
					tree:         tree,
					treeDepth:    depth,
				})
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				trees = append(trees, showTree)
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if len(trees) == 1 {
				if _, err := render.One(trees[0], rOpts); err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			} else {
				if _, err := render.Render(trees, rOpts); err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&withContext, "with-context", false, "include ancestor tree and linked decisions")
	cmd.Flags().BoolVar(&withBlockers, "with-blockers", false, "include blocking tickets")
	cmd.Flags().BoolVar(&withChildren, "with-children", false, "include direct child tickets")
	cmd.Flags().BoolVar(&tree, "tree", false, "include the nested descendant subtree plus the direct parent")
	cmd.Flags().IntVar(&depth, "depth", 0, "limit --tree to this many levels (0 = unlimited)")
	return cmd
}

// --- status ---

func newTicketStatusCmd() *cobra.Command {
	var forceFlag bool
	cmd := &cobra.Command{
		Use:   "status <id[,id,...]> <new-status>",
		Short: "Transition one or more tickets to a new status",
		Long: `Transition tickets to a new status. Use commas to batch:

  pql ticket status T-001 done
  pql ticket status T-001,T-002,T-003 done

The status vocabulary is per-vault (see ticket_statuses in .pql/config.yaml);
run "pql ticket statuslist" to see it. Transitions are otherwise unrestricted
(D-14) — any configured status may follow any other.

A ticket cannot be moved to a terminal (closed) status while it still has
open child tickets (D-25) — close the children first, or pass --force.
--force cascades the SAME terminal status down the subtree: every not-yet-
closed descendant is set to <new-status> bottom-up, then the ticket itself.
Already-closed descendants are left untouched. The command lists every
ticket it closed.

  pql ticket status E-1 cancelled --force   # closes E-1 and its open subtree`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])
			newStatus := args[1]

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
			ss := statusSetFromConfig(cfg)
			var results []repo.Ticket
			seen := map[string]bool{}
			for _, id := range ids {
				closedIDs, err := applyTicketStatus(ctx, db, id, newStatus, ss, forceFlag)
				if err != nil {
					var ice *repo.IncompleteChildrenError
					if errors.As(err, &ice) {
						return &exitError{code: diag.DataErr, msg: fmt.Sprintf(
							"%v — close them first, or re-run with --force to cascade %q to the whole open subtree",
							err, newStatus)}
					}
					return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, err)}
				}
				for _, cid := range closedIDs {
					if seen[cid] {
						continue
					}
					seen[cid] = true
					tk, err := repo.GetTicket(ctx, db, cid)
					if err != nil {
						return &exitError{code: diag.Software, msg: err.Error()}
					}
					if tk != nil {
						results = append(results, *tk)
					}
				}
			}

			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}
			return renderTicketResults(cmd, results)
		},
	}
	cmd.Flags().BoolVar(&forceFlag, "force", false,
		"when closing a ticket with open children, cascade the status to every not-yet-closed descendant")
	return cmd
}

// applyTicketStatus sets newStatus on id and returns every ticket id it
// closed. Without --force it is a single transition (the repo guards a
// terminal move against open children, returning *IncompleteChildrenError).
// With --force on a terminal status it cascades: every not-yet-closed
// descendant is closed bottom-up (deepest first) so each close satisfies the
// child-completeness guard, then the ticket itself.
func applyTicketStatus(ctx context.Context, db *sql.DB, id, newStatus string, ss planning.StatusSet, force bool) ([]string, error) {
	if !force || !ss.IsTerminal(newStatus) {
		if err := repo.SetStatus(ctx, db, id, newStatus, "", ss); err != nil {
			return nil, err
		}
		return []string{id}, nil
	}

	// DescendantsOf is ordered parent-before-child (depth ascending); walk it
	// in reverse so children are closed before their parents.
	descs, err := repo.DescendantsOf(ctx, db, id, 0)
	if err != nil {
		return nil, err
	}
	var closed []string
	for i := len(descs) - 1; i >= 0; i-- {
		d := descs[i]
		if ss.IsTerminal(d.Status) {
			continue // already closed — leave it as-is
		}
		if err := repo.SetStatus(ctx, db, d.ID, newStatus, "", ss); err != nil {
			return nil, err
		}
		closed = append(closed, d.ID)
	}
	if err := repo.SetStatus(ctx, db, id, newStatus, "", ss); err != nil {
		return nil, err
	}
	return append(closed, id), nil
}

// --- statuslist ---

func newTicketStatusListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "statuslist",
		Short: "List the configured ticket status vocabulary",
		Long: `Emit the per-vault ticket status vocabulary (ticket_statuses in
.pql/config.yaml, or the built-in default when unset), in board order. Each
entry carries name, label, class (initial|active|review|terminal), order,
is_default and is_terminal — enough for a consumer (e.g. clide) to drive its
UI without hardcoding the statuses.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.Render(statusSetFromConfig(cfg).All(), rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

// --- relabel ---

// proseMatch reports a DQR markdown file that mentions a relabeled ticket id.
type proseMatch struct {
	File  string `json:"file"`
	Count int    `json:"count"`
	Fixed bool   `json:"fixed"`
}

type relabelResult struct {
	RecordID     string       `json:"record_id"`
	OldLabel     string       `json:"old_label"`
	NewLabel     string       `json:"new_label"`
	ProseMatches []proseMatch `json:"prose_matches,omitempty"`
}

func newTicketRelabelCmd() *cobra.Command {
	var newLabel string
	var fixProse bool
	cmd := &cobra.Command{
		Use:   "relabel <id|record_id>",
		Short: "Reassign a ticket's friendly id (reconcile a duplicate label)",
		Long: `Change a ticket's friendly T-NNN label without touching its stable
record_id (D-26). The structural graph (parent, blockers, labels, history) keys
on record_id, so it is unaffected — only this mapping moves. Use this to
reconcile a duplicate-label collision surfaced at replay: pass the record_id
(the collision warning prints both) so the right ticket is picked.

  pql ticket relabel T-52 --new-label T-60      # rename a unique label
  pql ticket relabel <record_id> --new-label T-60   # disambiguate a collision

Without --new-label a fresh T-NNN is allocated. The old label may still be
referenced as prose in the DQR tree; those files are reported, and --fix-prose
rewrites the whole-word mentions in place.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if newLabel == "" {
				newLabel, err = repo.NextTicketID(ctx, pdb.SQL())
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}

			oldLabel, recordID, err := repo.Relabel(ctx, pdb.SQL(), args[0], newLabel)
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}
			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}

			matches, err := scanDQRForLabel(decisionsDir(cfg), oldLabel, newLabel, fixProse)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			res := &relabelResult{RecordID: recordID, OldLabel: oldLabel, NewLabel: newLabel, ProseMatches: matches}
			if _, err := render.One(res, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if len(matches) > 0 && !fixProse {
				diag.Warn("ticket.relabel.prose", fmt.Sprintf(
					"%s is still referenced as prose in %d DQR file(s); re-run with --fix-prose to rewrite them",
					oldLabel, len(matches)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&newLabel, "new-label", "", "new T-NNN label (default: allocate the next free one)")
	cmd.Flags().BoolVar(&fixProse, "fix-prose", false, "rewrite whole-word mentions of the old label in DQR markdown")
	return cmd
}

// scanDQRForLabel finds (and optionally rewrites) whole-word mentions of
// oldLabel in the markdown files under the DQR root. Structural references are
// untouched (they key on record_id); this is purely the human-readable prose.
func scanDQRForLabel(dqrRoot, oldLabel, newLabel string, fix bool) ([]proseMatch, error) {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(oldLabel) + `\b`)
	var matches []proseMatch
	err := filepath.WalkDir(dqrRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // no DQR tree → nothing to scan
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := os.ReadFile(path) //nolint:gosec // G304: path comes from walking the resolved DQR root
		if err != nil {
			return err
		}
		locs := re.FindAllIndex(b, -1)
		if len(locs) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(dqrRoot, path)
		m := proseMatch{File: rel, Count: len(locs)}
		if fix {
			if err := os.WriteFile(path, re.ReplaceAll(b, []byte(newLabel)), 0o644); err != nil { //nolint:gosec // G306: markdown is meant to be world-readable
				return err
			}
			m.Fixed = true
		}
		matches = append(matches, m)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan DQR for %s: %w", oldLabel, err)
	}
	return matches, nil
}

// --- assign ---

func newTicketAssignCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "assign <id[,id,...]> <agent>",
		Short: "Assign one or more tickets",
		Long: `Assign tickets to an agent. Use commas to batch:

  pql ticket assign T-001 claude
  pql ticket assign T-001,T-002,T-003 claude`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])
			agent := args[1]

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			var results []repo.Ticket
			for _, id := range ids {
				if err := repo.Assign(ctx, pdb.SQL(), id, agent, ""); err != nil {
					return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, err)}
				}
				tk, err := repo.GetTicket(ctx, pdb.SQL(), id)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				if tk != nil {
					results = append(results, *tk)
				}
			}

			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}
			return renderTicketResults(cmd, results)
		},
	}
}

// --- setparent ---

func newTicketSetParentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setparent <id[,id,...]> <parent-id | none>",
		Short: "Set or clear the parent of one or more tickets",
		Long: `Set or clear the parent of one or more tickets. Use commas to batch:

  pql ticket setparent T-9 T-2
  pql ticket setparent T-9,T-10,T-12 T-3
  pql ticket setparent T-9 none`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])
			parentID := args[1]
			if parentID == "none" {
				parentID = ""
			}

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			var results []repo.Ticket
			for _, id := range ids {
				if err := repo.SetParent(ctx, pdb.SQL(), id, parentID, ""); err != nil {
					return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, err)}
				}
				tk, err := repo.GetTicket(ctx, pdb.SQL(), id)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				if tk != nil {
					results = append(results, *tk)
				}
			}

			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}
			return renderTicketResults(cmd, results)
		},
	}
}

// --- block / unblock ---

func newTicketBlockCmd() *cobra.Command {
	var byID string
	cmd := &cobra.Command{
		Use:   "block <id> --by <blocker-id>",
		Short: "Mark a ticket as blocked by another",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			blockerRec, blockedRec, err := resolveDepRecords(ctx, pdb.SQL(), byID, args[0])
			if err != nil {
				return err
			}
			// Insert-or-resurrect: if a previous unblock soft-deleted
			// the (blocker, blocked) row, this clears deleted_at and
			// bumps updated_at so the row is live again.
			if _, err := pdb.SQL().ExecContext(ctx, `
				INSERT INTO ticket_deps (blocker_record_id, blocked_record_id, created_at, updated_at)
				VALUES (?, ?, datetime('now'), datetime('now'))
				ON CONFLICT(blocker_record_id, blocked_record_id) DO UPDATE SET
					deleted_at = NULL, updated_at = datetime('now')
			`, blockerRec, blockedRec); err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}
			if err := planning.RehashTicketDep(ctx, pdb.SQL(), blockerRec, blockedRec); err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}
			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			type blockResult struct {
				BlockerID string `json:"blocker_id"`
				BlockedID string `json:"blocked_id"`
			}
			if _, err := render.One(&blockResult{BlockerID: byID, BlockedID: args[0]}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&byID, "by", "", "blocker ticket ID (required)")
	_ = cmd.MarkFlagRequired("by")
	return cmd
}

func newTicketUnblockCmd() *cobra.Command {
	var fromID string
	cmd := &cobra.Command{
		Use:   "unblock <id> --from <blocker-id>",
		Short: "Remove a blocking relationship",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			blockerRec, blockedRec, err := resolveDepRecords(ctx, pdb.SQL(), fromID, args[0])
			if err != nil {
				return err
			}
			res, err := pdb.SQL().ExecContext(ctx, `
				UPDATE ticket_deps
				SET deleted_at = datetime('now'), updated_at = datetime('now')
				WHERE blocker_record_id = ? AND blocked_record_id = ? AND deleted_at IS NULL
			`, blockerRec, blockedRec)
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}
			if n, _ := res.RowsAffected(); n > 0 {
				if err := planning.RehashTicketDep(ctx, pdb.SQL(), blockerRec, blockedRec); err != nil {
					return &exitError{code: diag.DataErr, msg: err.Error()}
				}
			}
			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			type unblockResult struct {
				Removed bool `json:"removed"`
			}
			if _, err := render.One(&unblockResult{Removed: true}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromID, "from", "", "blocker ticket ID to remove (required)")
	_ = cmd.MarkFlagRequired("from")
	return cmd
}

// --- team ---

func newTicketTeamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "team <id[,id,...]> <team>",
		Short: "Set team for one or more tickets",
		Long: `Set a ticket's team. Use commas to batch:

  pql ticket team T-001 backend
  pql ticket team T-001,T-002 backend`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])
			team := args[1]

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			var results []repo.Ticket
			for _, id := range ids {
				rec, err := resolveTicketRecord(ctx, pdb.SQL(), id)
				if err != nil {
					return err
				}
				if _, err := pdb.SQL().ExecContext(ctx, `
					UPDATE tickets SET team = ?, updated_at = datetime('now') WHERE record_id = ?
				`, team, rec); err != nil {
					return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, err)}
				}
				if err := planning.RehashTicket(ctx, pdb.SQL(), rec); err != nil {
					return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, err)}
				}
				tk, err := repo.GetTicket(ctx, pdb.SQL(), id)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				if tk == nil {
					return &exitError{code: diag.NoInput, msg: fmt.Sprintf("ticket %s not found", id)}
				}
				results = append(results, *tk)
			}

			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}
			return renderTicketResults(cmd, results)
		},
	}
}

// --- label ---

func newTicketLabelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "label <id[,id,...]> <add|rm> <label>",
		Short: "Add or remove a label on one or more tickets",
		Long: `Manage labels. Use commas to batch:

  pql ticket label T-001 add urgent
  pql ticket label T-001,T-002,T-003 add blocked
  pql ticket label T-001,T-002 rm urgent`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := parseIDs(args[0])
			action, label := args[1], args[2]

			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			for _, id := range ids {
				if action != "add" && action != "rm" {
					return &exitError{code: diag.Usage, msg: fmt.Sprintf("unknown label action %q (use add or rm)", action)}
				}
				rec, err := resolveTicketRecord(ctx, pdb.SQL(), id)
				if err != nil {
					return err
				}
				switch action {
				case "add":
					// Insert-or-resurrect: a previously rm'd label gets
					// its deleted_at cleared so the changelog records
					// the re-attachment as a state change.
					if _, addErr := pdb.SQL().ExecContext(ctx, `
						INSERT INTO ticket_labels (ticket_record_id, label, created_at, updated_at)
						VALUES (?, ?, datetime('now'), datetime('now'))
						ON CONFLICT(ticket_record_id, label) DO UPDATE SET
							deleted_at = NULL, updated_at = datetime('now')
					`, rec, label); addErr != nil {
						return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, addErr)}
					}
					if rehashErr := planning.RehashTicketLabel(ctx, pdb.SQL(), rec, label); rehashErr != nil {
						return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, rehashErr)}
					}
				case "rm":
					res, rmErr := pdb.SQL().ExecContext(ctx, `
						UPDATE ticket_labels
						SET deleted_at = datetime('now'), updated_at = datetime('now')
						WHERE ticket_record_id = ? AND label = ? AND deleted_at IS NULL
					`, rec, label)
					if rmErr != nil {
						return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, rmErr)}
					}
					if n, _ := res.RowsAffected(); n > 0 {
						if rehashErr := planning.RehashTicketLabel(ctx, pdb.SQL(), rec, label); rehashErr != nil {
							return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, rehashErr)}
						}
					}
				}
			}

			if err := exportThrough(ctx, pdb, cfg.Vault.Path); err != nil {
				return err
			}

			type labelResult struct {
				TicketIDs []string `json:"ticket_ids"`
				Action    string   `json:"action"`
				Label     string   `json:"label"`
			}
			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(&labelResult{TicketIDs: ids, Action: action, Label: label}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

// --- board ---

func newTicketBoardCmd() *cobra.Command {
	var teamFlag string
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Kanban board view of tickets",
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

			ss := statusSetFromConfig(cfg)
			tks, err := repo.ListTickets(ctx, pdb.SQL(), repo.TicketFilter{Team: teamFlag, Statuses: ss})
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			type column struct {
				Status  string               `json:"status"`
				Tickets []repo.TicketSummary `json:"tickets"`
			}
			byStatus := make(map[string][]repo.TicketSummary)
			for _, tk := range tks {
				byStatus[tk.Status] = append(byStatus[tk.Status], repo.TicketSummary{
					ID: tk.ID, Type: tk.Type, Title: tk.Title,
					Status: tk.Status, Priority: tk.Priority,
				})
			}

			var board []column
			for _, s := range ss.Names() {
				if len(byStatus[s]) > 0 {
					board = append(board, column{Status: s, Tickets: byStatus[s]})
				}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.Render(board, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&teamFlag, "team", "", "filter by team")
	return cmd
}

func renderTicketResults(cmd *cobra.Command, results []repo.Ticket) error {
	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()
	if len(results) == 1 {
		if _, err := render.One(&results[0], rOpts); err != nil {
			return &exitError{code: diag.Software, msg: err.Error()}
		}
	} else {
		if _, err := render.Render(results, rOpts); err != nil {
			return &exitError{code: diag.Software, msg: err.Error()}
		}
	}
	return nil
}

// loadConfig is a short helper shared by ticket subcommands.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.Load(loadOptsFromFlags(cmd))
	if err != nil {
		return nil, &exitError{code: diag.NoInput, msg: err.Error()}
	}
	return cfg, nil
}

// statusSetFromConfig adapts the resolved config's ticket status
// vocabulary into a planning.StatusSet. This is the one bridge between
// the two packages: config cannot import planning (planning imports
// config), so the CLI translates here. Config has already validated the
// statuses (config.validate), so NewStatusSet just stamps order/labels.
// resolveTicketRecord maps a friendly label to its record_id, returning a
// clean not-found exit error when the label is unknown.
func resolveTicketRecord(ctx context.Context, db *sql.DB, label string) (string, error) {
	rec, err := repo.ResolveRecordID(ctx, db, label)
	if err != nil {
		return "", &exitError{code: diag.DataErr, msg: err.Error()}
	}
	if rec == "" {
		return "", &exitError{code: diag.NoInput, msg: fmt.Sprintf("ticket %s not found", label)}
	}
	return rec, nil
}

// resolveDepRecords resolves both ends of a dependency to record_ids.
func resolveDepRecords(ctx context.Context, db *sql.DB, blockerLabel, blockedLabel string) (blockerRec, blockedRec string, err error) {
	if blockerRec, err = resolveTicketRecord(ctx, db, blockerLabel); err != nil {
		return "", "", err
	}
	if blockedRec, err = resolveTicketRecord(ctx, db, blockedLabel); err != nil {
		return "", "", err
	}
	return blockerRec, blockedRec, nil
}

func statusSetFromConfig(cfg *config.Config) planning.StatusSet {
	defs := make([]planning.StatusDef, len(cfg.TicketStatuses))
	for i, s := range cfg.TicketStatuses {
		defs[i] = planning.StatusDef{
			Name:      s.Name,
			Label:     s.Label,
			Class:     s.Class,
			IsDefault: s.IsDefault,
		}
	}
	return planning.NewStatusSet(defs)
}
