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
//nolint:goconst // schema enum values
var validStatuses = map[string]bool{
	"backlog": true, "ready": true, "in_progress": true,
	"review": true, "done": true, "cancelled": true,
}

//nolint:goconst // schema enum values
var validTypes = map[string]bool{
	"initiative": true, "epic": true, "story": true,
	"task": true, "bug": true,
}

//nolint:goconst // schema enum values
var validPriorities = map[string]bool{
	"critical": true, "high": true, "medium": true, "low": true,
}


// Ticket is a row from the tickets table.
type Ticket struct {
	ID          string  `json:"id"`
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

	id, err := nextTicketID(ctx, db)
	if err != nil {
		return "", err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("repo: begin create ticket: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tickets (id, type, parent_id, title, description, status, priority,
			assigned_to, team, decision_ref, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'backlog', ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, opts.Type,
		nullIfEmpty(opts.ParentID),
		opts.Title,
		nullIfEmpty(opts.Description),
		opts.Priority,
		nullIfEmpty(opts.AssignedTo),
		nullIfEmpty(opts.Team),
		nullIfEmpty(opts.DecisionRef),
	); err != nil {
		return "", fmt.Errorf("repo: create ticket: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, id); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("repo: commit create ticket: %w", err)
	}
	return id, nil
}

func nextTicketID(ctx context.Context, db *sql.DB) (string, error) {
	var maxNum int
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(id, 3) AS INTEGER)), 0)
		FROM tickets WHERE id LIKE 'T-%'
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
	// Unrefined, when true, restricts to tickets with an empty
	// description (NULL or whitespace) and excludes done/cancelled.
	Unrefined bool
}

// ListTickets returns tickets matching the filter.
func ListTickets(ctx context.Context, db *sql.DB, f TicketFilter) ([]Ticket, error) {
	query := `SELECT id, type, parent_id, title, description, status, priority,
		assigned_to, team, decision_ref, created_at, updated_at
		FROM tickets WHERE 1=1`
	var params []any
	if f.Status != "" {
		query += " AND status = ?"
		params = append(params, f.Status)
	}
	if f.Team != "" {
		query += " AND team = ?"
		params = append(params, f.Team)
	}
	if f.AssignedTo != "" {
		query += " AND assigned_to = ?"
		params = append(params, f.AssignedTo)
	}
	if f.DecisionRef != "" {
		query += " AND decision_ref = ?"
		params = append(params, f.DecisionRef)
	}
	if f.ParentID != "" {
		query += " AND parent_id = ?"
		params = append(params, f.ParentID)
	}
	if f.Label != "" {
		query += " AND id IN (SELECT ticket_id FROM ticket_labels WHERE label = ?)"
		params = append(params, f.Label)
	}
	if f.Unrefined {
		query += " AND (description IS NULL OR TRIM(description) = '')"
		query += " AND status NOT IN ('done', 'cancelled')"
		query += ` ORDER BY
			CASE status
				WHEN 'in_progress' THEN 0
				WHEN 'review' THEN 1
				WHEN 'ready' THEN 2
				WHEN 'backlog' THEN 3
				ELSE 4
			END,
			CAST(SUBSTR(id, 3) AS INTEGER)`
	} else {
		query += " ORDER BY CAST(SUBSTR(id, 3) AS INTEGER)"
	}

	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("repo: list tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanTickets(rows)
}

// GetTicket returns a single ticket by ID, or nil if not found.
func GetTicket(ctx context.Context, db *sql.DB, id string) (*Ticket, error) {
	var t Ticket
	err := db.QueryRowContext(ctx, `
		SELECT id, type, parent_id, title, description, status, priority,
			assigned_to, team, decision_ref, created_at, updated_at
		FROM tickets WHERE id = ?
	`, id).Scan(&t.ID, &t.Type, &t.ParentID, &t.Title, &t.Description,
		&t.Status, &t.Priority, &t.AssignedTo, &t.Team, &t.DecisionRef,
		&t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("repo: get ticket %s: %w", id, err)
	}
	return &t, nil
}

// SetStatus changes a ticket's status. Records the change in ticket_history.
func SetStatus(ctx context.Context, db *sql.DB, id, newStatus, changedBy string) error {
	if !validStatuses[newStatus] {
		return fmt.Errorf("repo: invalid status %q", newStatus)
	}

	t, err := GetTicket(ctx, db, id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("repo: ticket %s not found", id)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin status change: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET status = ?, updated_at = datetime('now') WHERE id = ?
	`, newStatus, id); err != nil {
		return fmt.Errorf("repo: update status: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, id); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'status', ?, ?, ?, datetime('now'), datetime('now'))
	`, id, t.Status, newStatus, nullIfEmpty(changedBy))
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
		UPDATE tickets SET assigned_to = ?, updated_at = datetime('now') WHERE id = ?
	`, agent, id); err != nil {
		return fmt.Errorf("repo: update assigned_to: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, id); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'assigned_to', ?, ?, ?, datetime('now'), datetime('now'))
	`, id, nullIfEmpty(oldVal), agent, nullIfEmpty(changedBy))
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
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin setparent: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE tickets SET parent_id = ?, updated_at = datetime('now') WHERE id = ?
	`, nullIfEmpty(parentID), id); err != nil {
		return fmt.Errorf("repo: update parent_id: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, id); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by,
			created_at, updated_at)
		VALUES (?, 'parent_id', ?, ?, ?, datetime('now'), datetime('now'))
	`, id, nullIfEmpty(oldVal), nullIfEmpty(parentID), nullIfEmpty(changedBy))
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

// BlockersOf returns tickets that block the given ticket.
func BlockersOf(ctx context.Context, db *sql.DB, id string) ([]BlockerInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.title, t.status
		FROM ticket_deps d
		JOIN tickets t ON t.id = d.blocker_id
		WHERE d.blocked_id = ?
		ORDER BY CAST(SUBSTR(t.id, 3) AS INTEGER)
	`, id)
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

// ChildrenOf returns direct children of a ticket.
func ChildrenOf(ctx context.Context, db *sql.DB, parentID string) ([]TicketSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, title, status, priority
		FROM tickets WHERE parent_id = ? ORDER BY CAST(SUBSTR(id, 3) AS INTEGER)
	`, parentID)
	if err != nil {
		return nil, fmt.Errorf("repo: children of %s: %w", parentID, err)
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

// WhatNext returns the single best ticket to work on, or nil when
// nothing is actionable. Selection order: in_progress (finish current
// work), then ready (pick up new work), each bucket sorted by priority.
// Review tickets are deliberately excluded — the author context should
// not review its own work.
func WhatNext(ctx context.Context, db *sql.DB) (*Ticket, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, parent_id, title, description, status, priority,
			assigned_to, team, decision_ref, created_at, updated_at
		FROM tickets
		WHERE status IN ('in_progress', 'ready')
		ORDER BY
			CASE status WHEN 'in_progress' THEN 0 ELSE 1 END,
			CASE priority
				WHEN 'critical' THEN 0
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			CAST(SUBSTR(id, 3) AS INTEGER)
		LIMIT 1
	`)
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

// NextReview returns the highest-priority ticket in review status,
// or nil when nothing needs review.
func NextReview(ctx context.Context, db *sql.DB) (*Ticket, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, parent_id, title, description, status, priority,
			assigned_to, team, decision_ref, created_at, updated_at
		FROM tickets
		WHERE status = 'review'
		ORDER BY
			CASE priority
				WHEN 'critical' THEN 0
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			CAST(SUBSTR(id, 3) AS INTEGER)
		LIMIT 1
	`)
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
	setParts = append(setParts, "updated_at = datetime('now')")
	args = append(args, id)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repo: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt := fmt.Sprintf("UPDATE tickets SET %s WHERE id = ?", strings.Join(setParts, ", ")) //nolint:gosec // G201: setParts items are constructed from a closed-set whitelist of column names + literal "= ?", values bind via ExecContext
	if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
		return fmt.Errorf("repo: update ticket: %w", err)
	}
	if err := planning.RehashTicket(ctx, tx, id); err != nil {
		return err
	}

	for _, c := range changes {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by,
				created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, id, c.field, nullIfEmpty(c.old), nullIfEmpty(c.new), nullIfEmpty(changedBy))
		if err != nil {
			return fmt.Errorf("repo: record update history: %w", err)
		}
		if err := rehashHistoryRow(ctx, tx, res); err != nil {
			return err
		}
	}

	return tx.Commit()
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
		if err := rows.Scan(&t.ID, &t.Type, &t.ParentID, &t.Title, &t.Description,
			&t.Status, &t.Priority, &t.AssignedTo, &t.Team, &t.DecisionRef,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("repo: scan ticket: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
