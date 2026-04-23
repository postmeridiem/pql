package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

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
	cmd.AddCommand(newTicketAssignCmd())
	cmd.AddCommand(newTicketSetParentCmd())
	cmd.AddCommand(newTicketBlockCmd())
	cmd.AddCommand(newTicketUnblockCmd())
	cmd.AddCommand(newTicketTeamCmd())
	cmd.AddCommand(newTicketLabelCmd())
	cmd.AddCommand(newTicketBoardCmd())
	return cmd
}

// --- new ---

func newTicketNewCmd() *cobra.Command {
	var parentID, priority, decisionRef, team, assignedTo, description string
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
				Type:        args[0],
				Title:       args[1],
				Description: description,
				ParentID:    parentID,
				Priority:    priority,
				DecisionRef: decisionRef,
				Team:        team,
				AssignedTo:  assignedTo,
			})
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
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
	return cmd
}

// --- list ---

func newTicketListCmd() *cobra.Command {
	var statusFlag, teamFlag, assignedFlag, decisionFlag, labelFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tickets from pql.db",
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

			tks, err := repo.ListTickets(ctx, pdb.SQL(), repo.TicketFilter{
				Status:      statusFlag,
				Team:        teamFlag,
				AssignedTo:  assignedFlag,
				DecisionRef: decisionFlag,
				Label:       labelFlag,
			})
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			n, err := render.Render(tks, rOpts)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if n == 0 {
				return errNoMatch
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&statusFlag, "status", "", "filter by status")
	cmd.Flags().StringVar(&teamFlag, "team", "", "filter by team")
	cmd.Flags().StringVar(&assignedFlag, "assigned", "", "filter by assignee")
	cmd.Flags().StringVar(&decisionFlag, "decision", "", "filter by linked decision")
	cmd.Flags().StringVar(&labelFlag, "label", "", "filter by label")
	return cmd
}

// --- show ---

func newTicketShowCmd() *cobra.Command {
	var withDecision, withBlockers, withChildren bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a ticket with optional joins",
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

			tk, err := repo.GetTicket(ctx, pdb.SQL(), args[0])
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if tk == nil {
				return &exitError{code: diag.NoInput, msg: fmt.Sprintf("ticket %s not found", args[0])}
			}

			type showResult struct {
				repo.Ticket
				Decision *repo.Decision     `json:"decision,omitempty"`
				Blockers []repo.BlockerInfo  `json:"blockers,omitempty"`
				Children []repo.TicketSummary `json:"children,omitempty"`
			}
			out := showResult{Ticket: *tk}

			if withDecision && tk.DecisionRef != nil {
				out.Decision, err = repo.GetDecision(ctx, pdb.SQL(), *tk.DecisionRef)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}
			if withBlockers {
				out.Blockers, err = repo.BlockersOf(ctx, pdb.SQL(), args[0])
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}
			if withChildren {
				out.Children, err = repo.ChildrenOf(ctx, pdb.SQL(), args[0])
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
	cmd.Flags().BoolVar(&withDecision, "with-decision", false, "include linked decision")
	cmd.Flags().BoolVar(&withBlockers, "with-blockers", false, "include blocking tickets")
	cmd.Flags().BoolVar(&withChildren, "with-children", false, "include child tickets")
	return cmd
}

// --- status ---

func newTicketStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id[,id,...]> <new-status>",
		Short: "Transition one or more tickets to a new status",
		Long: `Transition tickets to a new status. Use commas to batch:

  pql ticket status T-001 done
  pql ticket status T-001,T-002,T-003 done

Status flow: backlog → ready → in_progress → review → done (also cancelled).
Invalid transitions are rejected per ticket.`,
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

			var results []repo.Ticket
			for _, id := range ids {
				if err := repo.SetStatus(ctx, pdb.SQL(), id, newStatus, ""); err != nil {
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

			return renderTicketResults(cmd, results)
		},
	}
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

			if _, err := pdb.SQL().ExecContext(ctx, `
				INSERT OR IGNORE INTO ticket_deps (blocker_id, blocked_id) VALUES (?, ?)
			`, byID, args[0]); err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
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

			if _, err := pdb.SQL().ExecContext(ctx, `
				DELETE FROM ticket_deps WHERE blocker_id = ? AND blocked_id = ?
			`, fromID, args[0]); err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
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
				if _, err := pdb.SQL().ExecContext(ctx, `
					UPDATE tickets SET team = ?, updated_at = datetime('now') WHERE id = ?
				`, team, id); err != nil {
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
				switch action {
				case "add":
					_, err = pdb.SQL().ExecContext(ctx, `
						INSERT OR IGNORE INTO ticket_labels (ticket_id, label) VALUES (?, ?)
					`, id, label)
				case "rm":
					_, err = pdb.SQL().ExecContext(ctx, `
						DELETE FROM ticket_labels WHERE ticket_id = ? AND label = ?
					`, id, label)
				}
				if err != nil {
					return &exitError{code: diag.DataErr, msg: fmt.Sprintf("%s: %v", id, err)}
				}
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

			tks, err := repo.ListTickets(ctx, pdb.SQL(), repo.TicketFilter{Team: teamFlag})
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if len(tks) == 0 {
				return errNoMatch
			}

			type column struct {
				Status  string             `json:"status"`
				Tickets []repo.TicketSummary `json:"tickets"`
			}
			statuses := []string{"backlog", "ready", "in_progress", "review", "done", "cancelled"}
			byStatus := make(map[string][]repo.TicketSummary)
			for _, tk := range tks {
				byStatus[tk.Status] = append(byStatus[tk.Status], repo.TicketSummary{
					ID: tk.ID, Type: tk.Type, Title: tk.Title,
					Status: tk.Status, Priority: tk.Priority,
				})
			}

			var board []column
			for _, s := range statuses {
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
