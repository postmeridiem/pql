package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// mkChild creates a ticket under parent and returns its id.
func mkChild(ctx context.Context, t *testing.T, db *sql.DB, typ, title, parent string) string {
	t.Helper()
	id, err := CreateTicket(ctx, db, NewTicketOpts{Type: typ, Title: title, ParentID: parent})
	if err != nil {
		t.Fatalf("create %s: %v", title, err)
	}
	return id
}

func TestSetStatus_BlocksCloseWithOpenChildren(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	parent, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "epic", Title: "epic"})
	child := mkChild(ctx, t, db, "task", "child", parent)

	// Closing the parent while the child is open is blocked.
	err := SetStatus(ctx, db, parent, "done", "")
	var ice *IncompleteChildrenError
	if !errors.As(err, &ice) {
		t.Fatalf("expected *IncompleteChildrenError, got %v", err)
	}
	if ice.TicketID != parent || len(ice.Children) != 1 || ice.Children[0].ID != child {
		t.Errorf("error payload = %+v, want parent=%s child=%s", ice, parent, child)
	}

	// Cancelling is also blocked (all terminal statuses are guarded).
	if err := SetStatus(ctx, db, parent, "cancelled", ""); !errors.As(err, &ice) {
		t.Errorf("cancel should also be blocked, got %v", err)
	}

	// A non-terminal move is never blocked.
	if err := SetStatus(ctx, db, parent, "in_progress", ""); err != nil {
		t.Errorf("non-terminal move should be allowed, got %v", err)
	}
}

func TestSetStatus_AllowsCloseWhenChildrenClosed(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	parent, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "epic", Title: "epic"})
	c1 := mkChild(ctx, t, db, "task", "a", parent)
	c2 := mkChild(ctx, t, db, "task", "b", parent)

	if err := SetStatus(ctx, db, c1, "done", ""); err != nil {
		t.Fatal(err)
	}
	// A cancelled child also counts as closed (terminal).
	if err := SetStatus(ctx, db, c2, "cancelled", ""); err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(ctx, db, parent, "done", ""); err != nil {
		t.Errorf("parent should close once all children are terminal, got %v", err)
	}
}

func TestSetStatus_LeafUnaffected(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "leaf"})
	if err := SetStatus(ctx, db, id, "done", ""); err != nil {
		t.Errorf("leaf close should be allowed, got %v", err)
	}
}

func TestSetStatus_SoftDeletedChildIgnored(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	parent, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "epic", Title: "epic"})
	child := mkChild(ctx, t, db, "task", "doomed", parent)
	if _, err := db.ExecContext(ctx,
		`UPDATE tickets SET deleted_at = datetime('now') WHERE id = ?`, child); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	// The only child is soft-deleted, so it doesn't block the close.
	if err := SetStatus(ctx, db, parent, "done", ""); err != nil {
		t.Errorf("soft-deleted child should not block close, got %v", err)
	}
}
