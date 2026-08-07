package repo

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/postmeridiem/pql/internal/planning"
)

// Validator maps. Strings are intentionally inline — these are the
// canonical schema enum values, repeated across tests and SQL CHECK
// constraints. Hiding them behind constants would obscure the
// schema's surface area more than it would dedupe.
//
// Ticket *status* is the exception: it is no longer a fixed enum but a
// per-vault vocabulary (planning.StatusSet, from .pql/config.yaml). The
// status-aware functions below take an optional StatusSet and fall back
// to planning.DefaultStatusSet() (the historical six) when none is given.

//nolint:goconst // schema enum values
var validTypes = map[string]bool{
	"initiative": true, "epic": true, "story": true,
	"task": true, "bug": true,
}

//nolint:goconst // schema enum values
var validPriorities = map[string]bool{
	"critical": true, "high": true, "medium": true, "low": true,
}

// priorityRank is the shared ORDER BY fragment ranking tickets by
// priority (critical first). Literal — no bound args.
const priorityRank = `CASE priority
		WHEN 'critical' THEN 0
		WHEN 'high' THEN 1
		WHEN 'medium' THEN 2
		WHEN 'low' THEN 3
	END`

// resolveStatusSet returns the caller-supplied status vocabulary, or the
// built-in default when none (or the zero value) was passed.
func resolveStatusSet(ss []planning.StatusSet) planning.StatusSet {
	if len(ss) > 0 && !ss[0].IsZero() {
		return ss[0]
	}
	return planning.DefaultStatusSet()
}

// statusInClause builds a "?, ?, …" placeholder list plus the matching
// bind args for an IN (…) over status names. Returns ("", nil) for an
// empty name list so callers can omit the clause.
func statusInClause(names []string) (clause string, args []any) {
	if len(names) == 0 {
		return "", nil
	}
	ph := make([]string, len(names))
	args = make([]any, len(names))
	for i, n := range names {
		ph[i] = "?"
		args[i] = n
	}
	return strings.Join(ph, ", "), args
}

// refineOrderCase builds the ORDER BY fragment that ranks unrefined
// tickets by how in-flight they are: active statuses first, then review,
// then initial statuses with the most-advanced (the "ready" lane) first.
// Terminal statuses are excluded from the unrefined list and sort last if
// they appear. Returns the CASE expression plus its bound status args.
func refineOrderCase(ss planning.StatusSet) (clause string, args []any) {
	var active, review, initial []string
	for _, d := range ss.All() {
		switch d.Class {
		case planning.StatusClassActive:
			active = append(active, d.Name)
		case planning.StatusClassReview:
			review = append(review, d.Name)
		case planning.StatusClassInitial:
			initial = append(initial, d.Name)
		}
	}
	// Initial statuses: most-advanced first (ready before backlog).
	for i, j := 0, len(initial)-1; i < j; i, j = i+1, j-1 {
		initial[i], initial[j] = initial[j], initial[i]
	}
	ranked := append(append(active, review...), initial...)
	if len(ranked) == 0 {
		return "0", nil
	}
	var b strings.Builder
	b.WriteString("CASE status")
	args = make([]any, 0, len(ranked))
	for i, name := range ranked {
		fmt.Fprintf(&b, " WHEN ? THEN %d", i)
		args = append(args, name)
	}
	fmt.Fprintf(&b, " ELSE %d END", len(ranked))
	return b.String(), args
}

// Ticket is a row from the tickets table joined to its friendly label.
// ID is the public T-NNN label (ticket_idmap.ticket_id); RecordID is the
// stable underwater identity (tickets.record_id). ParentID is the parent's
// label. See D-26.
type Ticket struct {
	ID          string  `json:"id"`
	RecordID    string  `json:"record_id"`
	Type        string  `json:"type"`
	ParentID    *string `json:"parent_id,omitempty"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	AssignedTo  *string `json:"assigned_to,omitempty"`
	Team        *string `json:"team,omitempty"`
	DecisionRef *string `json:"decision_ref,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ticketReadSQL is the shared SELECT that presents a ticket in its public
// shape: m.ticket_id as the id, the parent's label via a self-join on the
// idmap, and the underlying record_id. Callers append WHERE/ORDER clauses
// (the tickets table is aliased t, its idmap m, the parent's idmap pm).
const ticketReadSQL = `
	SELECT m.ticket_id, t.record_id, t.type, pm.ticket_id, t.title, t.description,
	       t.status, t.priority, t.assigned_to, t.team, t.decision_ref,
	       t.created_at, t.updated_at
	FROM tickets t
	JOIN ticket_idmap m ON m.record_id = t.record_id AND m.deleted_at IS NULL
	LEFT JOIN ticket_idmap pm ON pm.record_id = t.parent_record_id AND pm.deleted_at IS NULL`

// ResolveRecordID maps a friendly ticket_id label to its record_id for
// consumers outside the repo package (e.g. the CLI's dep/label commands).
// Returns ("", nil) for an unknown label.
func ResolveRecordID(ctx context.Context, db *sql.DB, ticketID string) (string, error) {
	return resolveRecordID(ctx, db, ticketID)
}

// Relabel changes the friendly ticket_id of a record (D-26 reconcile). The
// target is identified by record_id (unambiguous — preferred for resolving a
// duplicate label) or by a unique current label. newLabel must be non-empty
// and not already in use by a *different* record (which would just create a
// new collision). Returns the previous label and the resolved record_id.
// The structural graph (parent/deps/labels/history) is untouched — it keys on
// record_id — so only this one mapping row and any prose references move.
func Relabel(ctx context.Context, db *sql.DB, target, newLabel string) (oldLabel, recordID string, err error) {
	if strings.TrimSpace(newLabel) == "" {
		return "", "", fmt.Errorf("repo: new label cannot be empty")
	}
	// Resolve target: a record_id if it names a live ticket, else a label.
	var exists int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tickets WHERE record_id = ? AND deleted_at IS NULL`, target,
	).Scan(&exists); err != nil {
		return "", "", fmt.Errorf("repo: relabel resolve: %w", err)
	}
	if exists > 0 {
		recordID = target
	} else {
		recordID, err = resolveRecordID(ctx, db, target)
		if err != nil {
			return "", "", err
		}
		if recordID == "" {
			return "", "", fmt.Errorf("repo: ticket %s not found", target)
		}
	}

	if err := db.QueryRowContext(ctx,
		`SELECT ticket_id FROM ticket_idmap WHERE record_id = ? AND deleted_at IS NULL`, recordID,
	).Scan(&oldLabel); err != nil {
		return "", "", fmt.Errorf("repo: relabel read current label: %w", err)
	}
	if oldLabel == newLabel {
		return oldLabel, recordID, nil // no-op
	}
	// Refuse to relabel onto a label already held by a different record.
	var clash int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ticket_idmap WHERE ticket_id = ? AND record_id != ? AND deleted_at IS NULL`,
		newLabel, recordID,
	).Scan(&clash); err != nil {
		return "", "", fmt.Errorf("repo: relabel clash check: %w", err)
	}
	if clash > 0 {
		return "", "", fmt.Errorf("repo: label %s is already in use by another record", newLabel)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("repo: begin relabel: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`UPDATE ticket_idmap SET ticket_id = ?, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE record_id = ?`,
		newLabel, recordID,
	); err != nil {
		return "", "", fmt.Errorf("repo: relabel update: %w", err)
	}
	if err := planning.RehashTicketIDMap(ctx, tx, recordID); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("repo: commit relabel: %w", err)
	}
	return oldLabel, recordID, nil
}

// resolveRecordID maps a friendly ticket_id (T-NNN) to its record_id.
// Returns ("", nil) when the label is unknown. When a label is claimed by
// more than one record (an unreconciled collision), it returns an error so
// callers don't silently act on the wrong ticket.
func resolveRecordID(ctx context.Context, db *sql.DB, ticketID string) (string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.record_id FROM ticket_idmap m
		JOIN tickets t ON t.record_id = m.record_id AND t.deleted_at IS NULL
		WHERE m.ticket_id = ? AND m.deleted_at IS NULL
	`, ticketID)
	if err != nil {
		return "", fmt.Errorf("repo: resolve %s: %w", ticketID, err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("repo: resolve %s: scan: %w", ticketID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("repo: ticket id %s is claimed by %d records; run `pql ticket relabel` to reconcile", ticketID, len(ids))
	}
}

// NewTicketOpts are the parameters for creating a ticket.
type NewTicketOpts struct {
	Type        string
	Title       string
	Description string
	ParentID    string
	Priority    string
	DecisionRef string
	Team        string
	AssignedTo  string
	// DefaultStatus is the status a new ticket starts in. Empty falls
	// back to the built-in default (planning.DefaultStatusSet().Default(),
	// i.e. "backlog"); callers with a configured vocabulary set it to
	// StatusSet.Default().
	DefaultStatus string
}

// CreateTicket inserts a new ticket and returns its ID.
func CreateTicket(ctx context.Context, db *sql.DB, opts NewTicketOpts) (string, error) {
	if !validTypes[opts.Type] {
		return "", fmt.Errorf("repo: invalid ticket type %q", opts.Type)
	}
	if opts.Priority == "" {
		opts.Priority = "medium"
	}
	if !validPriorities[opts.Priority] {
		return "", fmt.Errorf("repo: invalid priority %q", opts.Priority)
	}
	status := opts.DefaultStatus
	if status == "" {
		status = planning.DefaultStatusSet().Default()
	}

	// Resolve an optional parent label to its record_id before we open the
	// write transaction.
	var parentRecID string
	if opts.ParentID != "" {
		rec, err := resolveRecordID(ctx, db, opts.ParentID)
		if err != nil {
			return "", err
		}
		if rec == "" {
			return "", fmt.Errorf("repo: parent ticket %s not found", opts.ParentID)
		}
		parentRecID = rec
	}

	label, err := nextTicketID(ctx, db)
	if err != nil {
		return "", err
	}
	recordID, err := planning.NewRecordID()
	if err != nil {
		return "", err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("repo: begin create ticket: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority,
			assigned_to, team, decision_ref, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
	`, recordID, opts.Type,
		nullIfEmpty(parentRecID),
		opts.Title,
		nullIfEmpty(opts.Description),
		status,
		opts.Priority,
		nullIfEmpty(opts.AssignedTo),
		nullIfEmpty(opts.Team),
		nullIfEmpty(opts.DecisionRef),
	); err != nil {
		return "", fmt.Errorf("repo: create ticket: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_idmap (record_id, ticket_id, created_at, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
	`, recordID, label); err != nil {
		return "", fmt.Errorf("repo: create ticket idmap: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, recordID); err != nil {
		return "", err
	}
	if err := planning.RehashTicketIDMap(ctx, tx, recordID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("repo: commit create ticket: %w", err)
	}
	return label, nil
}

// NextTicketID exposes the next free friendly label for consumers (e.g. the
// CLI relabel command allocating a fresh id).
func NextTicketID(ctx context.Context, db *sql.DB) (string, error) {
	return nextTicketID(ctx, db)
}

// nextTicketID allocates the next friendly T-NNN label as max+1 over the
// labels in ticket_idmap (local-only; cross-clone duplicates are reconciled
// via relabel, D-26).
func nextTicketID(ctx context.Context, db *sql.DB) (string, error) {
	var maxNum int
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(ticket_id, 3) AS INTEGER)), 0)
		FROM ticket_idmap WHERE ticket_id LIKE 'T-%'
	`).Scan(&maxNum)
	if err != nil {
		return "", fmt.Errorf("repo: next ticket id: %w", err)
	}
	return fmt.Sprintf("T-%d", maxNum+1), nil
}

// TicketFilter constrains ListTickets results.
type TicketFilter struct {
	Status      string
	Team        string
	AssignedTo  string
	DecisionRef string
	Label       string
	ParentID    string
	// Under, when set, restricts to the recursive descendants of the
	// given ticket (any depth). The root itself is excluded. Shares the
	// descendants primitive with `ticket show --tree`.
	Under string
	// Leaf, when true, restricts to tickets with no live children.
	Leaf bool
	// Unblocked, when true, restricts to tickets with no live blocker
	// still open (a blocker counts as cleared once done or cancelled).
	Unblocked bool
	// Unrefined, when true, restricts to tickets with an empty
	// description (NULL or whitespace) and excludes terminal statuses,
	// ordering by how in-flight the work is (active, review, then the
	// most-advanced initial status first).
	Unrefined bool
	// Statuses supplies the configured status vocabulary used by the
	// Unblocked and Unrefined logic (terminal set + refine ordering). The
	// zero value falls back to planning.DefaultStatusSet().
	Statuses planning.StatusSet
}

// unblockedClause restricts a ticket query to rows with no live blocker
// still open — a blocker counts as cleared once it reaches a terminal
// status (class terminal in the configured StatusSet; done/cancelled by
// default). Correlates on tickets.id, so the surrounding query must select
// FROM tickets unaliased. Shared by ListTickets (--unblocked) and WhatNext
// so "what can I pick up" and "what's next" agree on what blocked means.
// Returns the SQL fragment plus the terminal-status bind args.
func unblockedClause(ss planning.StatusSet) (clause string, args []any) {
	notTerminal := ""
	var inClause string
	inClause, args = statusInClause(ss.Terminal())
	if inClause != "" {
		notTerminal = " AND b.status NOT IN (" + inClause + ")"
	}
	return ` AND NOT EXISTS (
		SELECT 1 FROM ticket_deps d
		JOIN tickets b ON b.record_id = d.blocker_record_id
		WHERE d.blocked_record_id = t.record_id
		  AND d.deleted_at IS NULL
		  AND b.deleted_at IS NULL` + notTerminal + `
	)`, args
}

// ListTickets returns tickets matching the filter. Soft-deleted rows
// (deleted_at IS NOT NULL) are excluded by default per D-17.
func ListTickets(ctx context.Context, db *sql.DB, f TicketFilter) ([]Ticket, error) {
	query := ticketReadSQL + ` WHERE t.deleted_at IS NULL`
	var params []any
	if f.Status != "" {
		query += " AND t.status = ?"
		params = append(params, f.Status)
	}
	if f.Team != "" {
		query += " AND t.team = ?"
		params = append(params, f.Team)
	}
	if f.AssignedTo != "" {
		query += " AND t.assigned_to = ?"
		params = append(params, f.AssignedTo)
	}
	if f.DecisionRef != "" {
		query += " AND t.decision_ref = ?"
		params = append(params, f.DecisionRef)
	}
	if f.ParentID != "" {
		parentRecID, err := resolveRecordID(ctx, db, f.ParentID)
		if err != nil {
			return nil, err
		}
		if parentRecID == "" {
			return nil, nil // unknown parent → nothing matches
		}
		query += " AND t.parent_record_id = ?"
		params = append(params, parentRecID)
	}
	if f.Label != "" {
		query += ` AND t.record_id IN (
			SELECT ticket_record_id FROM ticket_labels
			WHERE label = ? AND deleted_at IS NULL
		)`
		params = append(params, f.Label)
	}
	if f.Under != "" {
		ds, err := DescendantsOf(ctx, db, f.Under, 0)
		if err != nil {
			return nil, err
		}
		if len(ds) == 0 {
			// No descendants → nothing can match. Short-circuit rather
			// than emit an invalid empty `IN ()`.
			return nil, nil
		}
		placeholders := make([]string, len(ds))
		for i, d := range ds {
			placeholders[i] = "?"
			params = append(params, d.RecordID)
		}
		//nolint:gosec // G202: the joined fragment is only "?" placeholders; the descendant record_ids bind via params
		query += " AND t.record_id IN (" + strings.Join(placeholders, ", ") + ")"
	}
	if f.Leaf {
		query += ` AND NOT EXISTS (
			SELECT 1 FROM tickets c
			WHERE c.parent_record_id = t.record_id AND c.deleted_at IS NULL
		)`
	}
	ss := resolveStatusSet([]planning.StatusSet{f.Statuses})
	if f.Unblocked {
		clause, cargs := unblockedClause(ss)
		query += clause
		params = append(params, cargs...)
	}
	if f.Unrefined {
		query += " AND (t.description IS NULL OR TRIM(t.description) = '')"
		if termIn, termArgs := statusInClause(ss.Terminal()); termIn != "" {
			//nolint:gosec // G202: termIn is a "?"-placeholder fragment; statuses bind via params.
			query += " AND t.status NOT IN (" + termIn + ")"
			params = append(params, termArgs...)
		}
		rankCase, rankArgs := refineOrderCase(ss)
		//nolint:gosec // G202: rankCase is a CASE over "?"-placeholders; statuses bind via params.
		query += " ORDER BY " + rankCase + ", CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)"
		params = append(params, rankArgs...)
	} else {
		query += " ORDER BY CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)"
	}

	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("repo: list tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanTickets(rows)
}

// GetTicket returns a single ticket by its friendly label, or nil if not
// found. Soft-deleted rows (deleted_at IS NOT NULL) are treated as absent.
func GetTicket(ctx context.Context, db *sql.DB, id string) (*Ticket, error) {
	rows, err := db.QueryContext(ctx, ticketReadSQL+` WHERE m.ticket_id = ? AND t.deleted_at IS NULL`, id)
	if err != nil {
		return nil, fmt.Errorf("repo: get ticket %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()
	ts, err := scanTickets(rows)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return nil, nil
	}
	return &ts[0], nil
}

// GetTicketByRecordID returns a single ticket by its stable record_id.
func GetTicketByRecordID(ctx context.Context, db *sql.DB, recordID string) (*Ticket, error) {
	rows, err := db.QueryContext(ctx, ticketReadSQL+` WHERE t.record_id = ? AND t.deleted_at IS NULL`, recordID)
	if err != nil {
		return nil, fmt.Errorf("repo: get ticket record %s: %w", recordID, err)
	}
	defer func() { _ = rows.Close() }()
	ts, err := scanTickets(rows)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return nil, nil
	}
	return &ts[0], nil
}

// IncompleteChildrenError is returned by SetStatus when a ticket is moved
// to a terminal status while it still has live, non-terminal children
// (D-25). It is a tree-integrity guard, not a workflow state machine —
// D-14 (any status → any status) still holds for non-terminal targets and
// for tickets without open children. Callers that genuinely want to close
// an unfinished subtree (e.g. abandon an epic) resolve the children first;
// the CLI's `--force` does this by cascading the status down the subtree.
type IncompleteChildrenError struct {
	TicketID  string
	NewStatus string
	Children  []TicketSummary // the live, non-terminal direct children
}

func (e *IncompleteChildrenError) Error() string {
	ids := make([]string, len(e.Children))
	for i, c := range e.Children {
		ids[i] = c.ID
	}
	return fmt.Sprintf(
		"repo: cannot set %s to %q while %d child ticket(s) are not yet closed: %s",
		e.TicketID, e.NewStatus, len(e.Children), strings.Join(ids, ", "))
}

// SetStatus changes a ticket's status. Records the change in ticket_history.
// The optional StatusSet supplies the valid vocabulary; when omitted it
// falls back to planning.DefaultStatusSet(). Transitions are unrestricted
// (D-14) — only the target status must be a known status — with one
// exception: a ticket cannot move to a terminal status while it has live,
// non-terminal direct children (D-25), which returns *IncompleteChildrenError.
func SetStatus(ctx context.Context, db *sql.DB, id, newStatus, changedBy string, ss ...planning.StatusSet) error {
	set := resolveStatusSet(ss)
	if !set.IsValid(newStatus) {
		return fmt.Errorf("repo: invalid status %q", newStatus)
	}

	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}

	if set.IsTerminal(newStatus) {
		children, err := ChildrenOf(ctx, db, t.RecordID)
		if err != nil {
			return err
		}
		var open []TicketSummary
		for _, c := range children {
			if !set.IsTerminal(c.Status) {
				open = append(open, c)
			}
		}
		if len(open) > 0 {
			return &IncompleteChildrenError{TicketID: id, NewStatus: newStatus, Children: open}
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin status change: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET status = ?, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE record_id = ?
	`, newStatus, t.RecordID); err != nil {
		return fmt.Errorf("repo: update status: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, t.RecordID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'status', ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
	`, t.RecordID, t.Status, newStatus, nullIfEmpty(changedBy))
	if err != nil {
		return fmt.Errorf("repo: record history: %w", err)
	}
	if err := rehashHistoryRow(ctx, tx, res); err != nil {
		return err
	}

	return tx.Commit()
}

// Assign sets the assigned_to field and records history.
func Assign(ctx context.Context, db *sql.DB, id, agent, changedBy string) error {
	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin assign: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	oldVal := ""
	if t.AssignedTo != nil {
		oldVal = *t.AssignedTo
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET assigned_to = ?, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE record_id = ?
	`, agent, t.RecordID); err != nil {
		return fmt.Errorf("repo: update assigned_to: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, t.RecordID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'assigned_to', ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
	`, t.RecordID, nullIfEmpty(oldVal), agent, nullIfEmpty(changedBy))
	if err != nil {
		return fmt.Errorf("repo: record assign history: %w", err)
	}
	if err := rehashHistoryRow(ctx, tx, res); err != nil {
		return err
	}

	return tx.Commit()
}

// SetParent sets a ticket's parent (or clears it with ""). Idempotent:
// if the parent is already the requested value, returns nil without
// writing history.
func SetParent(ctx context.Context, db *sql.DB, id, parentID, changedBy string) error {
	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}

	oldVal := ""
	if t.ParentID != nil {
		oldVal = *t.ParentID
	}
	if oldVal == parentID {
		return nil
	}

	var parentRecID string
	if parentID != "" {
		if parentID == id {
			return fmt.Errorf("repo: ticket %s cannot be its own parent", id)
		}
		p, err := GetTicket(ctx, db, parentID)
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("repo: parent ticket %s not found", parentID)
		}
		parentRecID = p.RecordID
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin setparent: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET parent_record_id = ?, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE record_id = ?
	`, nullIfEmpty(parentRecID), t.RecordID); err != nil {
		return fmt.Errorf("repo: update parent_record_id: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, t.RecordID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'parent_id', ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
	`, t.RecordID, nullIfEmpty(oldVal), nullIfEmpty(parentID), nullIfEmpty(changedBy))
	if err != nil {
		return fmt.Errorf("repo: record setparent history: %w", err)
	}
	if err := rehashHistoryRow(ctx, tx, res); err != nil {
		return err
	}

	return tx.Commit()
}

// BlockerInfo is a ticket that blocks another.
type BlockerInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// BlockersOf returns tickets that block the given ticket (by its label).
// Soft-deleted blockers and soft-deleted dep rows are excluded.
func BlockersOf(ctx context.Context, db *sql.DB, id string) ([]BlockerInfo, error) {
	recID, err := resolveRecordID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if recID == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT m.ticket_id, t.title, t.status
		FROM ticket_deps d
		JOIN tickets t ON t.record_id = d.blocker_record_id
		JOIN ticket_idmap m ON m.record_id = t.record_id AND m.deleted_at IS NULL
		WHERE d.blocked_record_id = ?
		  AND d.deleted_at IS NULL
		  AND t.deleted_at IS NULL
		ORDER BY CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)
	`, recID)
	if err != nil {
		return nil, fmt.Errorf("repo: blockers of %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var result []BlockerInfo
	for rows.Next() {
		var b BlockerInfo
		if err := rows.Scan(&b.ID, &b.Title, &b.Status); err != nil {
			return nil, fmt.Errorf("repo: scan blocker: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// ChildrenOf returns the direct children of a ticket, keyed by the
// parent's record_id. Soft-deleted children are excluded. Summaries carry
// the child's friendly label as ID.
func ChildrenOf(ctx context.Context, db *sql.DB, parentRecordID string) ([]TicketSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.ticket_id, t.type, t.title, t.status, t.priority
		FROM tickets t
		JOIN ticket_idmap m ON m.record_id = t.record_id AND m.deleted_at IS NULL
		WHERE t.parent_record_id = ? AND t.deleted_at IS NULL
		ORDER BY CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)
	`, parentRecordID)
	if err != nil {
		return nil, fmt.Errorf("repo: children of %s: %w", parentRecordID, err)
	}
	defer func() { _ = rows.Close() }()

	var result []TicketSummary
	for rows.Next() {
		var t TicketSummary
		if err := rows.Scan(&t.ID, &t.Type, &t.Title, &t.Status, &t.Priority); err != nil {
			return nil, fmt.Errorf("repo: scan child: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// maxTreeDepth is the sentinel used when a descendants walk is
// unbounded. It also caps a pathological parent cycle (SetParent only
// rejects direct self-parent, not deeper loops) so the recursive CTE
// terminates instead of spinning.
const maxTreeDepth = 1 << 20

// Descendant is a ticket in a subtree, carrying the parent link and
// depth (1 = direct child of the root) needed to reassemble nesting.
// ID/ParentID are friendly labels; RecordID is the stable identity.
type Descendant struct {
	TicketSummary
	RecordID string `json:"record_id"`
	ParentID string `json:"parent_id"`
	Depth    int    `json:"depth"`
}

// descendantsCTE walks the parent_record_id chain from a root record_id,
// then joins the idmap to present friendly labels. It is the shared
// primitive behind `ticket show --tree` (nested) and `ticket list --under`
// (flat filter). Rows are ordered parent-before-child (by depth).
const descendantsCTE = `
WITH RECURSIVE subtree(record_id, depth) AS (
	SELECT record_id, 1
	FROM tickets
	WHERE parent_record_id = ? AND deleted_at IS NULL
	UNION ALL
	SELECT t.record_id, s.depth + 1
	FROM tickets t
	JOIN subtree s ON t.parent_record_id = s.record_id
	WHERE t.deleted_at IS NULL AND s.depth < ?
)
SELECT m.ticket_id, t.record_id, t.type, t.title, t.status, t.priority, pm.ticket_id, s.depth
FROM subtree s
JOIN tickets t ON t.record_id = s.record_id
JOIN ticket_idmap m ON m.record_id = t.record_id AND m.deleted_at IS NULL
LEFT JOIN ticket_idmap pm ON pm.record_id = t.parent_record_id AND pm.deleted_at IS NULL
ORDER BY s.depth, CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)`

// DescendantsOf returns the flat list of descendants of rootID (a label),
// ordered parent-before-child. maxDepth <= 0 means unbounded (1 = direct
// children only, 2 = children + grandchildren, …). The root ticket is
// not included. An unknown or childless root yields an empty slice.
func DescendantsOf(ctx context.Context, db *sql.DB, rootID string, maxDepth int) ([]Descendant, error) {
	if maxDepth <= 0 {
		maxDepth = maxTreeDepth
	}
	rootRecID, err := resolveRecordID(ctx, db, rootID)
	if err != nil {
		return nil, err
	}
	if rootRecID == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, descendantsCTE, rootRecID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("repo: descendants of %s: %w", rootID, err)
	}
	defer func() { _ = rows.Close() }()

	var result []Descendant
	for rows.Next() {
		var d Descendant
		var parent sql.NullString
		if err := rows.Scan(&d.ID, &d.RecordID, &d.Type, &d.Title, &d.Status, &d.Priority, &parent, &d.Depth); err != nil {
			return nil, fmt.Errorf("repo: scan descendant: %w", err)
		}
		d.ParentID = parent.String
		result = append(result, d)
	}
	return result, rows.Err()
}

// TreeNode is a ticket summary plus its nested descendant subtree.
// Children is omitted when empty so leaves render flat.
type TreeNode struct {
	TicketSummary
	Children []TreeNode `json:"children,omitempty"`
}

// Subtree returns the descendants of rootID nested into a tree, ordered
// parent-before-child within each level. maxDepth follows DescendantsOf
// semantics. The returned slice is the root's direct children, each
// carrying its own subtree.
func Subtree(ctx context.Context, db *sql.DB, rootID string, maxDepth int) ([]TreeNode, error) {
	ds, err := DescendantsOf(ctx, db, rootID, maxDepth)
	if err != nil {
		return nil, err
	}
	return nestDescendants(rootID, ds), nil
}

// nestDescendants assembles flat descendant rows (ordered
// parent-before-child) into a tree hanging off rootID. Insertion order
// per parent is preserved, so the nested output keeps the flat query's
// ordering.
func nestDescendants(rootID string, ds []Descendant) []TreeNode {
	index := make(map[string]*TreeNode, len(ds))
	childIDs := make(map[string][]string)
	for i := range ds {
		index[ds[i].ID] = &TreeNode{TicketSummary: ds[i].TicketSummary}
		childIDs[ds[i].ParentID] = append(childIDs[ds[i].ParentID], ds[i].ID)
	}
	var build func(parent string) []TreeNode
	build = func(parent string) []TreeNode {
		ids := childIDs[parent]
		if len(ids) == 0 {
			return nil
		}
		out := make([]TreeNode, 0, len(ids))
		for _, cid := range ids {
			n := index[cid]
			n.Children = build(cid)
			out = append(out, *n)
		}
		return out
	}
	return build(rootID)
}

// WhatNext returns the single best ticket to work on, or nil when
// nothing is actionable. Selection order: active statuses (finish current
// work), then the "ready" lane — the most-advanced initial status — to
// pick up new work, each bucket sorted by priority. Earlier initial
// statuses (e.g. backlog) are not actionable. Tickets with an open blocker
// are excluded — recommending work you can't start isn't actionable
// (shares unblockedClause with the ListTickets --unblocked filter). Review
// statuses are deliberately excluded too — the author context should not
// review its own work. The optional StatusSet supplies the vocabulary;
// when omitted it falls back to planning.DefaultStatusSet().
func WhatNext(ctx context.Context, db *sql.DB, ss ...planning.StatusSet) (*Ticket, error) {
	set := resolveStatusSet(ss)
	active := set.Active()
	candidates := append([]string{}, active...)
	if ready := set.ReadyLane(); ready != "" {
		candidates = append(candidates, ready)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	candIn, args := statusInClause(candidates)
	unblocked, unblockedArgs := unblockedClause(set)
	args = append(args, unblockedArgs...)

	// Rank active statuses ahead of the ready lane. With no active
	// statuses every candidate is the ready lane → constant rank.
	rankExpr := "0"
	if activeIn, activeArgs := statusInClause(active); activeIn != "" {
		rankExpr = "CASE WHEN t.status IN (" + activeIn + ") THEN 0 ELSE 1 END"
		args = append(args, activeArgs...)
	}

	//nolint:gosec // G202: candIn/unblocked/rankExpr are "?"-placeholder fragments and
	// the trusted priorityRank const; every status value binds via args.
	query := ticketReadSQL + `
		WHERE t.status IN (` + candIn + `) AND t.deleted_at IS NULL` +
		unblocked + `
		ORDER BY ` + rankExpr + `, ` + priorityRank + `,
			CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)
		LIMIT 1
	`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: whatnext query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets, err := scanTickets(rows)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return nil, nil
	}
	return &tickets[0], nil
}

// NextReview returns the highest-priority ticket in a review status,
// or nil when nothing needs review (incl. when the vocabulary has no
// review-class status). The optional StatusSet supplies the vocabulary;
// when omitted it falls back to planning.DefaultStatusSet().
func NextReview(ctx context.Context, db *sql.DB, ss ...planning.StatusSet) (*Ticket, error) {
	reviewIn, args := statusInClause(resolveStatusSet(ss).Review())
	if reviewIn == "" {
		return nil, nil
	}
	//nolint:gosec // G202: reviewIn is a "?"-placeholder fragment and priorityRank is a
	// trusted const; every status value binds via args.
	query := ticketReadSQL + `
		WHERE t.status IN (` + reviewIn + `) AND t.deleted_at IS NULL
		ORDER BY ` + priorityRank + `,
			CAST(SUBSTR(m.ticket_id, 3) AS INTEGER)
		LIMIT 1
	`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: next review query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets, err := scanTickets(rows)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return nil, nil
	}
	return &tickets[0], nil
}

// Ancestors walks the parent chain from a ticket up to the root,
// returning the path from immediate parent to top-level ancestor.
func Ancestors(ctx context.Context, db *sql.DB, t *Ticket) ([]Ticket, error) {
	var result []Ticket
	seen := map[string]bool{t.ID: true}
	current := t
	for current.ParentID != nil {
		if seen[*current.ParentID] {
			break
		}
		parent, err := GetTicket(ctx, db, *current.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			break
		}
		seen[parent.ID] = true
		result = append(result, *parent)
		current = parent
	}
	return result, nil
}

// SetDecision links a ticket to a decision record, or clears the link when
// decisionID is empty. Every other structural attribute of a ticket is
// repairable after the fact — parent, blockers, assignee, team, labels, status,
// even a colliding label — and this one used to be settable only at `ticket
// new`, which made a delegation mistake permanent and quietly degraded the
// implementation-status view decisions carry (D-20).
//
// The decision is resolved before the write so an unknown id fails as a plain
// "not found" rather than as a foreign-key violation from SQLite.
func SetDecision(ctx context.Context, db *sql.DB, id, decisionID, changedBy string) error {
	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}

	oldVal := ""
	if t.DecisionRef != nil {
		oldVal = *t.DecisionRef
	}
	if oldVal == decisionID {
		return nil
	}
	if decisionID != "" {
		d, err := GetDecision(ctx, db, decisionID)
		if err != nil {
			return err
		}
		if d == nil {
			return fmt.Errorf("repo: decision %s not found (run `pql decisions sync` if it was just written)", decisionID)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin set decision: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET decision_ref = ?, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE record_id = ?
	`, nullIfEmpty(decisionID), t.RecordID); err != nil {
		return fmt.Errorf("repo: update decision_ref: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, t.RecordID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'decision_ref', ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
	`, t.RecordID, nullIfEmpty(oldVal), nullIfEmpty(decisionID), nullIfEmpty(changedBy))
	if err != nil {
		return fmt.Errorf("repo: record decision history: %w", err)
	}
	if err := rehashHistoryRow(ctx, tx, res); err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateTicketFields names the optional fields a refine-style multi-field
// update can touch. Status, parent, assignee, team, and labels have
// dedicated verbs and are intentionally excluded.
type UpdateTicketFields struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Type        *string `json:"type,omitempty"`
}

// UpdateTicket applies the non-nil fields to the ticket and records one
// history row per changed field. Empty fields map is a no-op.
func UpdateTicket(ctx context.Context, db *sql.DB, id string, f UpdateTicketFields, changedBy string) error {
	if f.Title != nil && strings.TrimSpace(*f.Title) == "" {
		return fmt.Errorf("repo: title cannot be empty")
	}
	if f.Type != nil && !validTypes[*f.Type] {
		return fmt.Errorf("repo: invalid type %q", *f.Type)
	}
	if f.Priority != nil && !validPriorities[*f.Priority] {
		return fmt.Errorf("repo: invalid priority %q", *f.Priority)
	}

	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}

	type change struct {
		field   string
		old     string
		new     string
		colExpr string
		colArg  any
	}
	var changes []change
	if f.Title != nil && *f.Title != t.Title {
		changes = append(changes, change{
			field: "title", old: t.Title, new: *f.Title,
			colExpr: "title = ?", colArg: *f.Title,
		})
	}
	if f.Description != nil {
		oldDesc := ""
		if t.Description != nil {
			oldDesc = *t.Description
		}
		if *f.Description != oldDesc {
			changes = append(changes, change{
				field: "description", old: oldDesc, new: *f.Description,
				colExpr: "description = ?", colArg: nullIfEmpty(*f.Description),
			})
		}
	}
	if f.Priority != nil && *f.Priority != t.Priority {
		changes = append(changes, change{
			field: "priority", old: t.Priority, new: *f.Priority,
			colExpr: "priority = ?", colArg: *f.Priority,
		})
	}
	if f.Type != nil && *f.Type != t.Type {
		changes = append(changes, change{
			field: "type", old: t.Type, new: *f.Type,
			colExpr: "type = ?", colArg: *f.Type,
		})
	}
	if len(changes) == 0 {
		return nil
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].field < changes[j].field })

	setParts := make([]string, 0, len(changes)+1)
	args := make([]any, 0, len(changes)+1)
	for _, c := range changes {
		setParts = append(setParts, c.colExpr)
		args = append(args, c.colArg)
	}
	setParts = append(setParts, "updated_at = strftime('%Y-%m-%d %H:%M:%f','now')")
	args = append(args, t.RecordID)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt := fmt.Sprintf("UPDATE tickets SET %s WHERE record_id = ?", strings.Join(setParts, ", ")) //nolint:gosec // G201: setParts items are constructed from a closed-set whitelist of column names + literal "= ?", values bind via ExecContext
	if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
		return fmt.Errorf("repo: update ticket: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, t.RecordID); err != nil {
		return err
	}

	for _, c := range changes {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by,
				created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%d %H:%M:%f','now'), strftime('%Y-%m-%d %H:%M:%f','now'))
		`, t.RecordID, c.field, nullIfEmpty(c.old), nullIfEmpty(c.new), nullIfEmpty(changedBy))
		if err != nil {
			return fmt.Errorf("repo: record update history: %w", err)
		}
		if err := rehashHistoryRow(ctx, tx, res); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AppendDescription appends text to a ticket's description, separated
// from any existing content by a blank line (the markdown paragraph
// break, so accumulated notes stay readable). When the description is
// empty the text becomes the whole description. Delegates the write to
// UpdateTicket so the history row, rehash, and transaction are shared.
// Empty or whitespace-only text is rejected.
func AppendDescription(ctx context.Context, db *sql.DB, id, text, changedBy string) error {
	// Trim surrounding whitespace so piped/file input doesn't leave a
	// dangling newline before the next block; internal structure is kept.
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("repo: append text cannot be empty")
	}
	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}
	combined := text
	if t.Description != nil && strings.TrimSpace(*t.Description) != "" {
		combined = *t.Description + "\n\n" + text
	}
	return UpdateTicket(ctx, db, id, UpdateTicketFields{Description: &combined}, changedBy)
}

// rehashHistoryRow looks up the rowid of a just-inserted ticket_history
// row and rehashes it. ticket_history has no natural primary key.
func rehashHistoryRow(ctx context.Context, tx *sql.Tx, res sql.Result) error {
	rowid, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repo: history rowid: %w", err)
	}
	return planning.RehashTicketHistory(ctx, tx, rowid)
}

func scanTickets(rows *sql.Rows) ([]Ticket, error) {
	var result []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.RecordID, &t.Type, &t.ParentID, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.AssignedTo, &t.Team, &t.DecisionRef,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo: scan ticket: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
