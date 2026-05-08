package planning

import (
	"context"
	"database/sql"
	"fmt"
)

// Execer is satisfied by both *sql.DB and *sql.Tx, so rehash helpers
// can run in or out of a transaction without duplicate definitions.
type Execer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Per-table Rehash helpers re-read the row, compute its canonical
// hash, and write hash + canonical_version back. Called both during
// migration backfill (schema_v2.go) and after every mutation in the
// repo write paths so the column is always current.
//
// Projection order MUST match the migration backfill projection.
// Bumping CanonicalVersion or changing the column list requires a
// re-hash migration in schema.go.

// RehashTicket recomputes the canonical hash for tickets.id = id and
// writes it to the hash + canonical_version columns.
func RehashTicket(ctx context.Context, e Execer, id string) error {
	var (
		ttype, title, status, priority, createdAt, updatedAt string
		parentID, description, assignedTo, team, decisionRef sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT type, parent_id, title, description, status, priority,
		       assigned_to, team, decision_ref, created_at, updated_at
		FROM tickets WHERE id = ?
	`, id).Scan(&ttype, &parentID, &title, &description, &status, &priority,
		&assignedTo, &team, &decisionRef, &createdAt, &updatedAt); err != nil {
		return fmt.Errorf("planning: rehash ticket %s: select: %w", id, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		id, ttype, nullStrPtr(parentID), title, nullStrPtr(description),
		status, priority,
		nullStrPtr(assignedTo), nullStrPtr(team), nullStrPtr(decisionRef),
		createdAt, updatedAt,
	})
	if _, err := e.ExecContext(ctx,
		`UPDATE tickets SET hash = ?, canonical_version = ? WHERE id = ?`,
		h, CanonicalVersion, id,
	); err != nil {
		return fmt.Errorf("planning: rehash ticket %s: update: %w", id, err)
	}
	return nil
}

// RehashDecision recomputes the canonical hash for decisions.id = id.
func RehashDecision(ctx context.Context, e Execer, id string) error {
	var (
		dtype, domain, title, status, filePath, syncedAt, createdAt, updatedAt string
		date                                                                   sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT type, domain, title, status, date, file_path,
		       synced_at, created_at, updated_at
		FROM decisions WHERE id = ?
	`, id).Scan(&dtype, &domain, &title, &status, &date, &filePath,
		&syncedAt, &createdAt, &updatedAt); err != nil {
		return fmt.Errorf("planning: rehash decision %s: select: %w", id, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		id, dtype, domain, title, status,
		nullStrPtr(date), filePath, syncedAt,
		createdAt, updatedAt,
	})
	if _, err := e.ExecContext(ctx,
		`UPDATE decisions SET hash = ?, canonical_version = ? WHERE id = ?`,
		h, CanonicalVersion, id,
	); err != nil {
		return fmt.Errorf("planning: rehash decision %s: update: %w", id, err)
	}
	return nil
}

// RehashDecisionRef recomputes the canonical hash for the
// (source_id, target_id, ref_type) decision_refs row.
func RehashDecisionRef(ctx context.Context, e Execer, sourceID, targetID, refType string) error {
	var (
		createdAt, updatedAt string
		note                 sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT note, created_at, updated_at
		FROM decision_refs
		WHERE source_id = ? AND target_id = ? AND ref_type = ?
	`, sourceID, targetID, refType).Scan(&note, &createdAt, &updatedAt); err != nil {
		return fmt.Errorf("planning: rehash decision_ref %s→%s/%s: select: %w",
			sourceID, targetID, refType, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		sourceID, targetID, refType, nullStrPtr(note),
		createdAt, updatedAt,
	})
	if _, err := e.ExecContext(ctx, `
		UPDATE decision_refs SET hash = ?, canonical_version = ?
		WHERE source_id = ? AND target_id = ? AND ref_type = ?
	`, h, CanonicalVersion, sourceID, targetID, refType); err != nil {
		return fmt.Errorf("planning: rehash decision_ref %s→%s/%s: update: %w",
			sourceID, targetID, refType, err)
	}
	return nil
}

// RehashTicketDep recomputes the canonical hash for the
// (blocker_id, blocked_id) ticket_deps row.
func RehashTicketDep(ctx context.Context, e Execer, blockerID, blockedID string) error {
	var createdAt, updatedAt string
	if err := e.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM ticket_deps
		WHERE blocker_id = ? AND blocked_id = ?
	`, blockerID, blockedID).Scan(&createdAt, &updatedAt); err != nil {
		return fmt.Errorf("planning: rehash ticket_dep %s→%s: select: %w",
			blockerID, blockedID, err)
	}
	h := Hash([]any{CanonicalVersion, blockerID, blockedID, createdAt, updatedAt})
	if _, err := e.ExecContext(ctx, `
		UPDATE ticket_deps SET hash = ?, canonical_version = ?
		WHERE blocker_id = ? AND blocked_id = ?
	`, h, CanonicalVersion, blockerID, blockedID); err != nil {
		return fmt.Errorf("planning: rehash ticket_dep %s→%s: update: %w",
			blockerID, blockedID, err)
	}
	return nil
}

// RehashTicketLabel recomputes the canonical hash for the
// (ticket_id, label) ticket_labels row.
func RehashTicketLabel(ctx context.Context, e Execer, ticketID, label string) error {
	var createdAt, updatedAt string
	if err := e.QueryRowContext(ctx, `
		SELECT created_at, updated_at FROM ticket_labels
		WHERE ticket_id = ? AND label = ?
	`, ticketID, label).Scan(&createdAt, &updatedAt); err != nil {
		return fmt.Errorf("planning: rehash ticket_label %s/%s: select: %w",
			ticketID, label, err)
	}
	h := Hash([]any{CanonicalVersion, ticketID, label, createdAt, updatedAt})
	if _, err := e.ExecContext(ctx, `
		UPDATE ticket_labels SET hash = ?, canonical_version = ?
		WHERE ticket_id = ? AND label = ?
	`, h, CanonicalVersion, ticketID, label); err != nil {
		return fmt.Errorf("planning: rehash ticket_label %s/%s: update: %w",
			ticketID, label, err)
	}
	return nil
}

// RehashTicketHistory keys on rowid because ticket_history has no
// natural primary key (multiple changes to the same field on the same
// timestamp are allowed).
func RehashTicketHistory(ctx context.Context, e Execer, rowid int64) error {
	var (
		ticketID, field, changedAt, createdAt, updatedAt string
		oldVal, newVal, changedBy                        sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT ticket_id, field, old_value, new_value, changed_by,
		       changed_at, created_at, updated_at
		FROM ticket_history WHERE rowid = ?
	`, rowid).Scan(&ticketID, &field, &oldVal, &newVal, &changedBy,
		&changedAt, &createdAt, &updatedAt); err != nil {
		return fmt.Errorf("planning: rehash ticket_history rowid %d: select: %w", rowid, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		ticketID, field,
		nullStrPtr(oldVal), nullStrPtr(newVal), nullStrPtr(changedBy),
		changedAt, createdAt, updatedAt,
	})
	if _, err := e.ExecContext(ctx,
		`UPDATE ticket_history SET hash = ?, canonical_version = ? WHERE rowid = ?`,
		h, CanonicalVersion, rowid,
	); err != nil {
		return fmt.Errorf("planning: rehash ticket_history rowid %d: update: %w", rowid, err)
	}
	return nil
}
