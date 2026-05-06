package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

// newTicketRefineCmd groups the unrefined-ticket triage flow.
//
// Refinement surfaces tickets whose description is empty so a human or
// agent can write a proper one. The flow is read-then-write: `list`
// shows the queue, `next` zooms in on the head with a full show-tree,
// `write` applies a JSON patch and re-renders the show-tree.
func newTicketRefineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refine",
		Short: "Surface and write descriptions for unrefined tickets",
		Long: `Tickets without a description are "unrefined". This subcommand
groups the triage flow: list them, zoom in on the next one with full
context, and write descriptions back via a JSON payload.

  pql ticket refine list
  pql ticket refine next [--skip N]
  pql ticket refine write T-5 '{"description":"..."}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.AddCommand(newTicketRefineListCmd())
	cmd.AddCommand(newTicketRefineNextCmd())
	cmd.AddCommand(newTicketRefineWriteCmd())
	return cmd
}

// --- refine list ---

func newTicketRefineListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tickets with an empty description",
		Long: `Lists tickets whose description is NULL or whitespace-only,
excluding done and cancelled. Sorted by status priority
(in_progress > review > ready > backlog) then numeric ID.`,
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

			tks, err := repo.ListTickets(ctx, pdb.SQL(), repo.TicketFilter{Unrefined: true})
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
}

// --- refine next ---

// refineMeta annotates a refine-next result with queue position so the
// caller knows how many tickets remain after the current one.
type refineMeta struct {
	Remaining int `json:"remaining"`
	Skipped   int `json:"skipped"`
}

// refineNextResult wraps the standard ticket show-tree with refinement
// metadata. Embedding *ticketShowTree promotes its fields to the top
// level so the JSON shape matches `ticket show` plus a `refinement`
// envelope.
type refineNextResult struct {
	Refinement refineMeta `json:"refinement"`
	*ticketShowTree
}

func newTicketRefineNextCmd() *cobra.Command {
	var skip int
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show the next unrefined ticket with full context",
		Long: `Zooms in on the head of the unrefined queue with the same
join-tree as ` + "`ticket show --with-context --with-blockers --with-children`" + `,
plus a refinement envelope reporting how many unrefined tickets remain.

Use --skip N to step past N tickets you cannot refine right now without
mutating any state.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if skip < 0 {
				return &exitError{code: diag.Usage, msg: "--skip must be non-negative"}
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

			tks, err := repo.ListTickets(ctx, pdb.SQL(), repo.TicketFilter{Unrefined: true})
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if skip >= len(tks) {
				return errNoMatch
			}

			head := tks[skip]
			tree, err := buildShowTree(ctx, pdb.SQL(), &head, true, true, true)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			out := &refineNextResult{
				Refinement: refineMeta{
					Remaining: len(tks) - skip - 1,
					Skipped:   skip,
				},
				ticketShowTree: tree,
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
	cmd.Flags().IntVar(&skip, "skip", 0, "step past this many leading tickets")
	return cmd
}

// --- refine write ---

func newTicketRefineWriteCmd() *cobra.Command {
	var fromFile string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "write <id> [json]",
		Short: "Apply a JSON patch to a ticket's writable fields",
		Long: `Updates writable fields on a ticket from a JSON payload.
Editable fields: title, description, priority, type. Status, parent,
assignee, team, and labels have dedicated subcommands and cannot be
written here. Unknown fields are rejected.

Three input modes (mutually exclusive), mirroring ` + "`pql query`" + `:

  pql ticket refine write T-5 '{"description":"..."}'   # positional
  pql ticket refine write T-5 --file patch.json
  pql ticket refine write T-5 --stdin

Output is the same show-tree as ` + "`ticket show --with-context --with-blockers --with-children`" + `,
so the caller can verify the write took.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]

			payloadArgs := args[1:]
			payload, err := readRefinePayload(payloadArgs, fromFile, fromStdin, cmd.InOrStdin())
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}

			var fields repo.UpdateTicketFields
			dec := json.NewDecoder(bytes.NewReader(payload))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&fields); err != nil {
				return &exitError{code: diag.DataErr, msg: fmt.Sprintf("parse payload: %v", err)}
			}
			if dec.More() {
				return &exitError{code: diag.DataErr, msg: "parse payload: trailing data after JSON object"}
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

			if err := repo.UpdateTicket(ctx, pdb.SQL(), id, fields, ""); err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}

			tk, err := repo.GetTicket(ctx, pdb.SQL(), id)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if tk == nil {
				return &exitError{code: diag.NoInput, msg: fmt.Sprintf("ticket %s not found", id)}
			}
			tree, err := buildShowTree(ctx, pdb.SQL(), tk, true, true, true)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(tree, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "file", "", "read the JSON payload from this file path")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read the JSON payload from stdin")
	return cmd
}

// readRefinePayload resolves the JSON payload from one of three
// mutually exclusive inputs. Mirrors readDSLSource in query_dsl.go.
func readRefinePayload(args []string, fromFile string, fromStdin bool, stdin io.Reader) ([]byte, error) {
	chosen := 0
	if len(args) > 0 {
		chosen++
	}
	if fromFile != "" {
		chosen++
	}
	if fromStdin {
		chosen++
	}
	switch chosen {
	case 0:
		return nil, errors.New("pql ticket refine write: provide a JSON payload via positional arg, --file, or --stdin")
	case 1:
	default:
		return nil, errors.New("pql ticket refine write: positional, --file, and --stdin are mutually exclusive")
	}

	switch {
	case len(args) > 0:
		return []byte(args[0]), nil
	case fromFile != "":
		b, err := os.ReadFile(fromFile) //nolint:gosec // G304: --file path is user-supplied; reading from it is the feature
		if err != nil {
			return nil, fmt.Errorf("read --file %q: %w", fromFile, err)
		}
		return b, nil
	case fromStdin:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return b, nil
	}
	return nil, errors.New("unreachable")
}
