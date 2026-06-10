package changelog

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// mkTicketLabel inserts a ticket (by record_id) and its idmap label
// directly, so tests can stage a duplicate-label collision.
func mkTicketLabel(ctx context.Context, t *testing.T, db *sql.DB, recordID, label string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tickets (record_id, type, title, status, priority, created_at, updated_at)
		VALUES (?, 'task', ?, 'backlog', 'medium', datetime('now'), datetime('now'))
	`, recordID, "title-"+recordID); err != nil {
		t.Fatalf("insert ticket %s: %v", recordID, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_idmap (record_id, ticket_id, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
	`, recordID, label); err != nil {
		t.Fatalf("insert idmap %s: %v", recordID, err)
	}
}

func TestDetectTicketCollisions_Clean(t *testing.T) {
	ctx := context.Background()
	_, db := setupVault(t)
	mkTicketLabel(ctx, t, db, "R1", "T-1")
	mkTicketLabel(ctx, t, db, "R2", "T-2")

	cols, err := detectTicketCollisions(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 0 {
		t.Fatalf("clean idmap: got %d collisions, want 0: %+v", len(cols), cols)
	}
}

func TestDetectTicketCollisions_DuplicateLabel(t *testing.T) {
	ctx := context.Background()
	_, db := setupVault(t)
	// Two distinct records claiming the same friendly label T-5 — the
	// cross-clone collision (D-26).
	mkTicketLabel(ctx, t, db, "R1", "T-5")
	mkTicketLabel(ctx, t, db, "R2", "T-5")
	mkTicketLabel(ctx, t, db, "R3", "T-6") // a clean one alongside

	cols, err := detectTicketCollisions(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("got %d collisions, want 1: %+v", len(cols), cols)
	}
	c := cols[0]
	if c.TicketID != "T-5" {
		t.Errorf("collision label = %q, want T-5", c.TicketID)
	}
	if len(c.Records) != 2 {
		t.Fatalf("records = %d, want 2: %+v", len(c.Records), c.Records)
	}
	seen := map[string]bool{}
	for _, r := range c.Records {
		seen[r.RecordID] = true
	}
	if !seen["R1"] || !seen["R2"] {
		t.Errorf("records = %+v, want R1 and R2", c.Records)
	}
}

func TestDetectTicketCollisions_IgnoresDeleted(t *testing.T) {
	ctx := context.Background()
	_, db := setupVault(t)
	mkTicketLabel(ctx, t, db, "R1", "T-7")
	mkTicketLabel(ctx, t, db, "R2", "T-7")
	// Soft-delete one idmap row → no live collision.
	if _, err := db.ExecContext(ctx,
		`UPDATE ticket_idmap SET deleted_at = datetime('now') WHERE record_id = 'R2'`); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	cols, err := detectTicketCollisions(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(cols) != 0 {
		t.Fatalf("soft-deleted dup should not collide: %+v", cols)
	}
}
