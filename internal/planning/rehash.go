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
// hash, and write hash + canonical_version back. Called after every
// mutation in the repo write paths so the column is always current.
//
// Projection order is part of CanonicalVersion's contract — changing
// the column list or order requires bumping CanonicalVersion and
// re-hashing existing rows (in practice: a fresh import).

// nullStrPtr converts a sql.NullString to *string for canonical
// projection — Valid=false renders as the canonical NULL sentinel.
func nullStrPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

// RehashTicket recomputes the canonical hash for tickets.record_id =
// recordID and writes it to the hash + canonical_version columns.
func RehashTicket(ctx context.Context, e Execer, recordID string) error {
	var (
		ttype, title, status, priority, createdAt, updatedAt             string
		parentRecID, description, assignedTo, team, decisionRef, deleted sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT type, parent_record_id, title, description, status, priority,
		       assigned_to, team, decision_ref, created_at, updated_at, deleted_at
		FROM tickets WHERE record_id = ?
	`, recordID).Scan(&ttype, &parentRecID, &title, &description, &status, &priority,
		&assignedTo, &team, &decisionRef, &createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash ticket %s: select: %w", recordID, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		recordID, ttype, nullStrPtr(parentRecID), title, nullStrPtr(description),
		status, priority,
		nullStrPtr(assignedTo), nullStrPtr(team), nullStrPtr(decisionRef),
		createdAt, updatedAt, nullStrPtr(deleted),
	})
	if _, err := e.ExecContext(ctx,
		`UPDATE tickets SET hash = ?, canonical_version = ? WHERE record_id = ?`,
		h, CanonicalVersion, recordID,
	); err != nil {
		return fmt.Errorf("planning: rehash ticket %s: update: %w", recordID, err)
	}
	return nil
}

// RehashTicketIDMap recomputes the canonical hash for the ticket_idmap row
// keyed by record_id (the record_id ↔ friendly ticket_id mapping).
func RehashTicketIDMap(ctx context.Context, e Execer, recordID string) error {
	var (
		ticketID, createdAt, updatedAt string
		deleted                        sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT ticket_id, created_at, updated_at, deleted_at
		FROM ticket_idmap WHERE record_id = ?
	`, recordID).Scan(&ticketID, &createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash ticket_idmap %s: select: %w", recordID, err)
	}
	h := Hash([]any{
		CanonicalVersion, recordID, ticketID,
		createdAt, updatedAt, nullStrPtr(deleted),
	})
	if _, err := e.ExecContext(ctx,
		`UPDATE ticket_idmap SET hash = ?, canonical_version = ? WHERE record_id = ?`,
		h, CanonicalVersion, recordID,
	); err != nil {
		return fmt.Errorf("planning: rehash ticket_idmap %s: update: %w", recordID, err)
	}
	return nil
}

// RehashDecision recomputes the canonical hash for decisions.id = id.
func RehashDecision(ctx context.Context, e Execer, id string) error {
	var (
		dtype, domain, title, status, filePath, syncedAt, createdAt, updatedAt string
		date, deleted                                                          sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT type, domain, title, status, date, file_path,
		       synced_at, created_at, updated_at, deleted_at
		FROM decisions WHERE id = ?
	`, id).Scan(&dtype, &domain, &title, &status, &date, &filePath,
		&syncedAt, &createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash decision %s: select: %w", id, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		id, dtype, domain, title, status,
		nullStrPtr(date), filePath, syncedAt,
		createdAt, updatedAt, nullStrPtr(deleted),
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
		note, deleted        sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT note, created_at, updated_at, deleted_at
		FROM decision_refs
		WHERE source_id = ? AND target_id = ? AND ref_type = ?
	`, sourceID, targetID, refType).Scan(&note, &createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash decision_ref %s→%s/%s: select: %w",
			sourceID, targetID, refType, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		sourceID, targetID, refType, nullStrPtr(note),
		createdAt, updatedAt, nullStrPtr(deleted),
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
func RehashTicketDep(ctx context.Context, e Execer, blockerRecID, blockedRecID string) error {
	var (
		createdAt, updatedAt string
		deleted              sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT created_at, updated_at, deleted_at FROM ticket_deps
		WHERE blocker_record_id = ? AND blocked_record_id = ?
	`, blockerRecID, blockedRecID).Scan(&createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash ticket_dep %s→%s: select: %w",
			blockerRecID, blockedRecID, err)
	}
	h := Hash([]any{
		CanonicalVersion, blockerRecID, blockedRecID,
		createdAt, updatedAt, nullStrPtr(deleted),
	})
	if _, err := e.ExecContext(ctx, `
		UPDATE ticket_deps SET hash = ?, canonical_version = ?
		WHERE blocker_record_id = ? AND blocked_record_id = ?
	`, h, CanonicalVersion, blockerRecID, blockedRecID); err != nil {
		return fmt.Errorf("planning: rehash ticket_dep %s→%s: update: %w",
			blockerRecID, blockedRecID, err)
	}
	return nil
}

// RehashTicketLabel recomputes the canonical hash for the
// (ticket_record_id, label) ticket_labels row.
func RehashTicketLabel(ctx context.Context, e Execer, ticketRecID, label string) error {
	var (
		createdAt, updatedAt string
		deleted              sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT created_at, updated_at, deleted_at FROM ticket_labels
		WHERE ticket_record_id = ? AND label = ?
	`, ticketRecID, label).Scan(&createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash ticket_label %s/%s: select: %w",
			ticketRecID, label, err)
	}
	h := Hash([]any{
		CanonicalVersion, ticketRecID, label,
		createdAt, updatedAt, nullStrPtr(deleted),
	})
	if _, err := e.ExecContext(ctx, `
		UPDATE ticket_labels SET hash = ?, canonical_version = ?
		WHERE ticket_record_id = ? AND label = ?
	`, h, CanonicalVersion, ticketRecID, label); err != nil {
		return fmt.Errorf("planning: rehash ticket_label %s/%s: update: %w",
			ticketRecID, label, err)
	}
	return nil
}

// RehashTicketHistory keys on rowid because ticket_history has no
// natural primary key (multiple changes to the same field on the same
// timestamp are allowed).
func RehashTicketHistory(ctx context.Context, e Execer, rowid int64) error {
	var (
		ticketID, field, changedAt, createdAt, updatedAt string
		oldVal, newVal, changedBy, deleted               sql.NullString
	)
	if err := e.QueryRowContext(ctx, `
		SELECT ticket_record_id, field, old_value, new_value, changed_by,
		       changed_at, created_at, updated_at, deleted_at
		FROM ticket_history WHERE rowid = ?
	`, rowid).Scan(&ticketID, &field, &oldVal, &newVal, &changedBy,
		&changedAt, &createdAt, &updatedAt, &deleted); err != nil {
		return fmt.Errorf("planning: rehash ticket_history rowid %d: select: %w", rowid, err)
	}
	h := Hash([]any{
		CanonicalVersion,
		ticketID, field,
		nullStrPtr(oldVal), nullStrPtr(newVal), nullStrPtr(changedBy),
		changedAt, createdAt, updatedAt, nullStrPtr(deleted),
	})
	if _, err := e.ExecContext(ctx,
		`UPDATE ticket_history SET hash = ?, canonical_version = ? WHERE rowid = ?`,
		h, CanonicalVersion, rowid,
	); err != nil {
		return fmt.Errorf("planning: rehash ticket_history rowid %d: update: %w", rowid, err)
	}
	return nil
}
