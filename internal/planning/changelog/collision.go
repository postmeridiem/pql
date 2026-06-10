package changelog

import (
	"context"
	"database/sql"
	"fmt"
)

// TicketCollision reports a single friendly label (ticket_id) claimed by
// more than one record (D-26). Ticket labels are allocated max+1 against
// the *local* changelog with no coordinator, so two clones/branches can
// mint the same T-NNN. Identity lives in the collision-proof record_id, so
// the graph never corrupts — but the duplicate label is surfaced here so a
// maintainer can fix it with `pql ticket relabel`. Detected against the
// rebuilt ticket_idmap (post-replay), so it reflects LWW and any prior
// relabels rather than raw changelog history.
type TicketCollision struct {
	TicketID string            `json:"ticket_id"`
	Records  []CollidingRecord `json:"records"`
}

// CollidingRecord identifies one of the records sharing a label.
type CollidingRecord struct {
	RecordID string `json:"record_id"`
	Title    string `json:"title"`
}

// detectTicketCollisions returns every ticket_id label mapped to more than
// one live record in the current ticket_idmap.
func detectTicketCollisions(ctx context.Context, db *sql.DB) ([]TicketCollision, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT m.ticket_id, m.record_id, t.title
		FROM ticket_idmap m
		JOIN tickets t ON t.record_id = m.record_id AND t.deleted_at IS NULL
		WHERE m.deleted_at IS NULL
		ORDER BY m.ticket_id, m.record_id
	`)
	if err != nil {
		return nil, fmt.Errorf("changelog: detect collisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Preserve first-seen order of labels for stable output.
	var order []string
	byLabel := map[string][]CollidingRecord{}
	for rows.Next() {
		var ticketID, recordID, title string
		if err := rows.Scan(&ticketID, &recordID, &title); err != nil {
			return nil, fmt.Errorf("changelog: scan collision row: %w", err)
		}
		if _, ok := byLabel[ticketID]; !ok {
			order = append(order, ticketID)
		}
		byLabel[ticketID] = append(byLabel[ticketID], CollidingRecord{RecordID: recordID, Title: title})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []TicketCollision
	for _, label := range order {
		if recs := byLabel[label]; len(recs) > 1 {
			out = append(out, TicketCollision{TicketID: label, Records: recs})
		}
	}
	return out, nil
}
