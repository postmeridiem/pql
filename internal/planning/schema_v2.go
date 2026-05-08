package planning

import (
	"context"
	"database/sql"
	"fmt"
)

// migrationV2SQL adds the replication columns needed by the
// changelog-based replication design (D-15..D-18):
//
//   - hash TEXT and canonical_version INTEGER on every planning table.
//   - created_at and updated_at on tables that lacked them
//     (decisions, decision_refs, ticket_deps, ticket_history,
//     ticket_labels). tickets already had these from migration v1.
//
// The SQL block also backfills the timestamps from the row's
// most-relevant existing column (synced_at for decisions, changed_at
// for ticket_history) so the migration leaves the database in a
// consistent state. Hashes are backfilled in backfillHashesV2 because
// SQLite has no built-in MD5 — they need a Go pass.
const migrationV2SQL = `
ALTER TABLE decisions       ADD COLUMN created_at        TEXT;
ALTER TABLE decisions       ADD COLUMN updated_at        TEXT;
ALTER TABLE decisions       ADD COLUMN hash              TEXT;
ALTER TABLE decisions       ADD COLUMN canonical_version INTEGER;

ALTER TABLE decision_refs   ADD COLUMN created_at        TEXT;
ALTER TABLE decision_refs   ADD COLUMN updated_at        TEXT;
ALTER TABLE decision_refs   ADD COLUMN hash              TEXT;
ALTER TABLE decision_refs   ADD COLUMN canonical_version INTEGER;

ALTER TABLE tickets         ADD COLUMN hash              TEXT;
ALTER TABLE tickets         ADD COLUMN canonical_version INTEGER;

ALTER TABLE ticket_deps     ADD COLUMN created_at        TEXT;
ALTER TABLE ticket_deps     ADD COLUMN updated_at        TEXT;
ALTER TABLE ticket_deps     ADD COLUMN hash              TEXT;
ALTER TABLE ticket_deps     ADD COLUMN canonical_version INTEGER;

ALTER TABLE ticket_history  ADD COLUMN created_at        TEXT;
ALTER TABLE ticket_history  ADD COLUMN updated_at        TEXT;
ALTER TABLE ticket_history  ADD COLUMN hash              TEXT;
ALTER TABLE ticket_history  ADD COLUMN canonical_version INTEGER;

ALTER TABLE ticket_labels   ADD COLUMN created_at        TEXT;
ALTER TABLE ticket_labels   ADD COLUMN updated_at        TEXT;
ALTER TABLE ticket_labels   ADD COLUMN hash              TEXT;
ALTER TABLE ticket_labels   ADD COLUMN canonical_version INTEGER;

UPDATE decisions      SET created_at = COALESCE(synced_at, datetime('now')),
                          updated_at = COALESCE(synced_at, datetime('now'));
UPDATE decision_refs  SET created_at = datetime('now'),
                          updated_at = datetime('now');
UPDATE ticket_deps    SET created_at = datetime('now'),
                          updated_at = datetime('now');
UPDATE ticket_history SET created_at = changed_at,
                          updated_at = changed_at;
UPDATE ticket_labels  SET created_at = datetime('now'),
                          updated_at = datetime('now');
`

// backfillHashesV2 walks every row in every planning table and calls
// the per-table Rehash helper, which is the same code the runtime
// write paths use. Single source of truth for the canonical
// projection.
func backfillHashesV2(ctx context.Context, tx *sql.Tx) error {
	if err := backfillTable(ctx, tx, "tickets", "id"); err != nil {
		return err
	}
	if err := backfillTable(ctx, tx, "decisions", "id"); err != nil {
		return err
	}
	if err := backfillDecisionRefs(ctx, tx); err != nil {
		return err
	}
	if err := backfillTicketDeps(ctx, tx); err != nil {
		return err
	}
	if err := backfillTicketLabels(ctx, tx); err != nil {
		return err
	}
	if err := backfillTicketHistory(ctx, tx); err != nil {
		return err
	}
	return nil
}

// backfillTable handles tables with a single TEXT primary key (tickets,
// decisions). Reads ids, then defers to the per-row Rehash helper.
func backfillTable(ctx context.Context, tx *sql.Tx, table, pk string) error {
	ids, err := scanIDs(ctx, tx, fmt.Sprintf("SELECT %s FROM %s", pk, table)) //nolint:gosec // table & pk are compile-time literals
	if err != nil {
		return fmt.Errorf("backfill %s: %w", table, err)
	}
	for _, id := range ids {
		switch table {
		case "tickets":
			if err := RehashTicket(ctx, tx, id); err != nil {
				return err
			}
		case "decisions":
			if err := RehashDecision(ctx, tx, id); err != nil {
				return err
			}
		default:
			return fmt.Errorf("backfill %s: no rehash handler", table)
		}
	}
	return nil
}

func backfillDecisionRefs(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT source_id, target_id, ref_type FROM decision_refs`)
	if err != nil {
		return fmt.Errorf("backfill decision_refs: %w", err)
	}
	type triple struct{ source, target, refType string }
	var keys []triple
	for rows.Next() {
		var k triple
		if err := rows.Scan(&k.source, &k.target, &k.refType); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill decision_refs: scan: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("backfill decision_refs: rows: %w", err)
	}
	_ = rows.Close()
	for _, k := range keys {
		if err := RehashDecisionRef(ctx, tx, k.source, k.target, k.refType); err != nil {
			return err
		}
	}
	return nil
}

func backfillTicketDeps(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT blocker_id, blocked_id FROM ticket_deps`)
	if err != nil {
		return fmt.Errorf("backfill ticket_deps: %w", err)
	}
	type pair struct{ blocker, blocked string }
	var keys []pair
	for rows.Next() {
		var k pair
		if err := rows.Scan(&k.blocker, &k.blocked); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill ticket_deps: scan: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("backfill ticket_deps: rows: %w", err)
	}
	_ = rows.Close()
	for _, k := range keys {
		if err := RehashTicketDep(ctx, tx, k.blocker, k.blocked); err != nil {
			return err
		}
	}
	return nil
}

func backfillTicketLabels(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT ticket_id, label FROM ticket_labels`)
	if err != nil {
		return fmt.Errorf("backfill ticket_labels: %w", err)
	}
	type pair struct{ ticketID, label string }
	var keys []pair
	for rows.Next() {
		var k pair
		if err := rows.Scan(&k.ticketID, &k.label); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill ticket_labels: scan: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("backfill ticket_labels: rows: %w", err)
	}
	_ = rows.Close()
	for _, k := range keys {
		if err := RehashTicketLabel(ctx, tx, k.ticketID, k.label); err != nil {
			return err
		}
	}
	return nil
}

func backfillTicketHistory(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT rowid FROM ticket_history`)
	if err != nil {
		return fmt.Errorf("backfill ticket_history: %w", err)
	}
	var rowids []int64
	for rows.Next() {
		var r int64
		if err := rows.Scan(&r); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill ticket_history: scan: %w", err)
		}
		rowids = append(rowids, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("backfill ticket_history: rows: %w", err)
	}
	_ = rows.Close()
	for _, r := range rowids {
		if err := RehashTicketHistory(ctx, tx, r); err != nil {
			return err
		}
	}
	return nil
}

func scanIDs(ctx context.Context, tx *sql.Tx, query string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// nullStrPtr converts a sql.NullString to *string for canonical
// projection — Valid=false renders as the canonical NULL sentinel.
func nullStrPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}
