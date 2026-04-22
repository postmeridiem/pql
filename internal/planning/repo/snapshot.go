package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Snapshot is the portable JSON shape for plan export/import.
type Snapshot struct {
	ExportedAt   string           `json:"exported_at"`
	Decisions    []Decision       `json:"decisions"`
	DecisionRefs []DecisionRef    `json:"decision_refs"`
	Tickets      []Ticket         `json:"tickets"`
	TicketDeps   []TicketDep      `json:"ticket_deps"`
	TicketLabels []TicketLabel    `json:"ticket_labels"`
	History      []HistoryEntry   `json:"history"`
}

// TicketDep is a row from ticket_deps.
type TicketDep struct {
	BlockerID string `json:"blocker_id"`
	BlockedID string `json:"blocked_id"`
}

// TicketLabel is a row from ticket_labels.
type TicketLabel struct {
	TicketID string `json:"ticket_id"`
	Label    string `json:"label"`
}

// HistoryEntry is a row from ticket_history.
type HistoryEntry struct {
	TicketID  string  `json:"ticket_id"`
	Field     string  `json:"field"`
	OldValue  *string `json:"old_value,omitempty"`
	NewValue  *string `json:"new_value,omitempty"`
	ChangedBy *string `json:"changed_by,omitempty"`
	ChangedAt string  `json:"changed_at"`
}

// Export reads all planning state from the database.
func Export(ctx context.Context, db *sql.DB) (*Snapshot, error) {
	snap := &Snapshot{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}

	decs, err := ListDecisions(ctx, db, DecisionFilter{})
	if err != nil {
		return nil, fmt.Errorf("export decisions: %w", err)
	}
	snap.Decisions = decs

	refs, err := allDecisionRefs(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("export decision_refs: %w", err)
	}
	snap.DecisionRefs = refs

	tickets, err := ListTickets(ctx, db, TicketFilter{})
	if err != nil {
		return nil, fmt.Errorf("export tickets: %w", err)
	}
	snap.Tickets = tickets

	deps, err := allTicketDeps(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("export ticket_deps: %w", err)
	}
	snap.TicketDeps = deps

	labels, err := allTicketLabels(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("export ticket_labels: %w", err)
	}
	snap.TicketLabels = labels

	history, err := allHistory(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("export ticket_history: %w", err)
	}
	snap.History = history

	return snap, nil
}

// Import restores planning state from a snapshot. Existing data is
// replaced (upsert for decisions/tickets, delete+reinsert for the
// rest). Runs in a single transaction.
func Import(ctx context.Context, db *sql.DB, snap *Snapshot) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("import: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, d := range snap.Decisions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO decisions (id, type, domain, title, status, date, file_path, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
			ON CONFLICT(id) DO UPDATE SET
				type=excluded.type, domain=excluded.domain, title=excluded.title,
				status=excluded.status, date=excluded.date, file_path=excluded.file_path,
				synced_at=datetime('now')
		`, d.ID, d.Type, d.Domain, d.Title, d.Status, d.Date, d.FilePath); err != nil {
			return fmt.Errorf("import decision %s: %w", d.ID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM decision_refs"); err != nil {
		return fmt.Errorf("import: clear refs: %w", err)
	}
	for _, r := range snap.DecisionRefs {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO decision_refs (source_id, target_id, ref_type, note)
			VALUES (?, ?, ?, ?)
		`, r.SourceID, r.TargetID, r.RefType, r.Note); err != nil {
			return fmt.Errorf("import ref %s->%s: %w", r.SourceID, r.TargetID, err)
		}
	}

	for _, t := range snap.Tickets {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tickets (id, type, parent_id, title, description, status, priority,
				assigned_to, team, decision_ref, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				type=excluded.type, parent_id=excluded.parent_id, title=excluded.title,
				description=excluded.description, status=excluded.status, priority=excluded.priority,
				assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref,
				updated_at=datetime('now')
		`, t.ID, t.Type, t.ParentID, t.Title, t.Description, t.Status, t.Priority,
			t.AssignedTo, t.Team, t.DecisionRef, t.CreatedAt, t.UpdatedAt); err != nil {
			return fmt.Errorf("import ticket %s: %w", t.ID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM ticket_deps"); err != nil {
		return fmt.Errorf("import: clear deps: %w", err)
	}
	for _, d := range snap.TicketDeps {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO ticket_deps (blocker_id, blocked_id) VALUES (?, ?)
		`, d.BlockerID, d.BlockedID); err != nil {
			return fmt.Errorf("import dep %s->%s: %w", d.BlockerID, d.BlockedID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM ticket_labels"); err != nil {
		return fmt.Errorf("import: clear labels: %w", err)
	}
	for _, l := range snap.TicketLabels {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO ticket_labels (ticket_id, label) VALUES (?, ?)
		`, l.TicketID, l.Label); err != nil {
			return fmt.Errorf("import label %s/%s: %w", l.TicketID, l.Label, err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM ticket_history"); err != nil {
		return fmt.Errorf("import: clear history: %w", err)
	}
	for _, h := range snap.History {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ticket_history (ticket_id, field, old_value, new_value, changed_by, changed_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, h.TicketID, h.Field, h.OldValue, h.NewValue, h.ChangedBy, h.ChangedAt); err != nil {
			return fmt.Errorf("import history: %w", err)
		}
	}

	return tx.Commit()
}

func allDecisionRefs(ctx context.Context, db *sql.DB) ([]DecisionRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT source_id, target_id, ref_type, COALESCE(note, '')
		FROM decision_refs ORDER BY source_id, target_id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []DecisionRef
	for rows.Next() {
		var r DecisionRef
		if err := rows.Scan(&r.SourceID, &r.TargetID, &r.RefType, &r.Note); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func allTicketDeps(ctx context.Context, db *sql.DB) ([]TicketDep, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT blocker_id, blocked_id FROM ticket_deps ORDER BY blocker_id, blocked_id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []TicketDep
	for rows.Next() {
		var d TicketDep
		if err := rows.Scan(&d.BlockerID, &d.BlockedID); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func allTicketLabels(ctx context.Context, db *sql.DB) ([]TicketLabel, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_id, label FROM ticket_labels ORDER BY ticket_id, label
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []TicketLabel
	for rows.Next() {
		var l TicketLabel
		if err := rows.Scan(&l.TicketID, &l.Label); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func allHistory(ctx context.Context, db *sql.DB) ([]HistoryEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ticket_id, field, old_value, new_value, changed_by, changed_at
		FROM ticket_history ORDER BY changed_at, ticket_id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var result []HistoryEntry
	for rows.Next() {
		var h HistoryEntry
		if err := rows.Scan(&h.TicketID, &h.Field, &h.OldValue, &h.NewValue, &h.ChangedBy, &h.ChangedAt); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}
