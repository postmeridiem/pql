package repo

import (
	"context"
	"testing"
)

// TestImport_ForwardParentReferences regresses the legacy-import FK
// failure clideclaude surfaced during T-25 scenario 1 (2026-05-09).
// Snapshot tickets[] is JSON-array-ordered; parent_id can point
// forward in the array (e.g. T-1.parent = T-6 with T-6 later in the
// list). Without deferred FK enforcement the per-row check aborts
// the import on the first forward reference.
func TestImport_ForwardParentReferences(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	parentID := "T-6"
	snap := &Snapshot{
		// T-1 references T-6, but T-6 appears later in the array.
		Tickets: []Ticket{
			{
				ID: "T-1", Type: "task", ParentID: &parentID,
				Title: "child first", Status: "backlog", Priority: "medium",
				CreatedAt: "2025-01-01 00:00:00", UpdatedAt: "2025-01-01 00:00:00",
			},
			{
				ID: "T-6", Type: "task",
				Title: "parent later", Status: "backlog", Priority: "medium",
				CreatedAt: "2025-01-01 00:00:00", UpdatedAt: "2025-01-01 00:00:00",
			},
		},
	}

	if err := Import(ctx, db, snap); err != nil {
		t.Fatalf("Import: %v", err)
	}

	for _, id := range []string{"T-1", "T-6"} {
		tk, err := GetTicket(ctx, db, id)
		if err != nil || tk == nil {
			t.Errorf("ticket %s missing after import: tk=%v err=%v", id, tk, err)
		}
	}

	// FK enforcement should be back to normal after the import — a
	// fresh insert pointing at a non-existent parent must still fail.
	_, err := db.ExecContext(ctx, `
		INSERT INTO tickets (id, type, parent_id, title, status, priority,
			created_at, updated_at)
		VALUES ('T-99', 'task', 'T-NONEXISTENT', 'x', 'backlog', 'medium',
			datetime('now'), datetime('now'))
	`)
	if err == nil {
		t.Error("FK enforcement leaked off the connection after import")
	}
}
