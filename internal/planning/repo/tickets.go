package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var validStatuses = map[string]bool{
	"backlog": true, "ready": true, "in_progress": true,
	"review": true, "done": true, "cancelled": true,
}

var validTypes = map[string]bool{
	"initiative": true, "epic": true, "story": true,
	"task": true, "bug": true,
}

var validPriorities = map[string]bool{
	"critical": true, "high": true, "medium": true, "low": true,
}

// statusTransitions defines allowed status transitions. The key is the
// current status; the value is the set of statuses it can move to.
var statusTransitions = map[string]map[string]bool{
	"backlog":     {"ready": true, "cancelled": true},
	"ready":       {"backlog": true, "in_progress": true, "cancelled": true},
	"in_progress": {"ready": true, "review": true, "done": true, "cancelled": true},
	"review":      {"in_progress": true, "done": true, "cancelled": true},
	"done":        {"in_progress": true},
	"cancelled":   {"backlog": true},
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

	_, err = db.ExecContext(ctx, `
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
	)
	if err != nil {
		return "", fmt.Errorf("repo: create ticket: %w", err)
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
	query += " ORDER BY CAST(SUBSTR(id, 3) AS INTEGER)"

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

// SetStatus transitions a ticket to a new status, enforcing the state
// machine. Records the change in ticket_history.
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

	allowed := statusTransitions[t.Status]
	if !allowed[newStatus] {
		valid := make([]string, 0, len(allowed))
		for s := range allowed {
			valid = append(valid, s)
		}
		return fmt.Errorf("repo: cannot transition %s from %s to %s (allowed: %s)",
			id, t.Status, newStatus, strings.Join(valid, ", "))
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

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by)
		VALUES (?, 'status', ?, ?, ?)
	`, id, t.Status, newStatus, nullIfEmpty(changedBy)); err != nil {
		return fmt.Errorf("repo: record history: %w", err)
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

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by)
		VALUES (?, 'assigned_to', ?, ?, ?)
	`, id, nullIfEmpty(oldVal), agent, nullIfEmpty(changedBy)); err != nil {
		return fmt.Errorf("repo: record assign history: %w", err)
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
