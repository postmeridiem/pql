package repo

import (
	"context"
	"testing"
)

func TestCreateAndGetTicket(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id, err := CreateTicket(ctx, db, NewTicketOpts{
		Type:  "task",
		Title: "probe something",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != "T-001" {
		t.Errorf("id = %q, want T-001", id)
	}

	tk, err := GetTicket(ctx, db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if tk == nil {
		t.Fatal("ticket not found")
	}
	if tk.Status != "backlog" {
		t.Errorf("status = %q, want backlog", tk.Status)
	}
	if tk.Priority != "medium" {
		t.Errorf("priority = %q, want medium", tk.Priority)
	}
}

func TestCreateTicket_WithDecisionRef(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	// Insert a decision first so FK is satisfied.
	_, err := db.ExecContext(ctx,
		`INSERT INTO decisions (id, type, domain, title, file_path) VALUES ('D-001', 'confirmed', 'test', 'test', 'test.md')`)
	if err != nil {
		t.Fatalf("insert decision: %v", err)
	}

	id, err := CreateTicket(ctx, db, NewTicketOpts{
		Type:        "task",
		Title:       "implement D-001",
		DecisionRef: "D-001",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	tk, err := GetTicket(ctx, db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if tk.DecisionRef == nil || *tk.DecisionRef != "D-001" {
		t.Errorf("decision_ref = %v, want D-001", tk.DecisionRef)
	}
}

func TestSetStatus(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "test"})

	// backlog → ready (allowed)
	if err := SetStatus(ctx, db, id, "ready", "test"); err != nil {
		t.Fatalf("backlog->ready: %v", err)
	}

	tk, _ := GetTicket(ctx, db, id)
	if tk.Status != "ready" {
		t.Errorf("status = %q, want ready", tk.Status)
	}

	// ready → done (not allowed — must go through in_progress)
	err := SetStatus(ctx, db, id, "done", "test")
	if err == nil {
		t.Fatal("expected error for ready->done")
	}

	// ready → in_progress → done (allowed)
	if err := SetStatus(ctx, db, id, "in_progress", "test"); err != nil {
		t.Fatalf("ready->in_progress: %v", err)
	}
	if err := SetStatus(ctx, db, id, "done", "test"); err != nil {
		t.Fatalf("in_progress->done: %v", err)
	}
}

func TestAssign(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "test"})

	if err := Assign(ctx, db, id, "claude", "test"); err != nil {
		t.Fatalf("assign: %v", err)
	}

	tk, _ := GetTicket(ctx, db, id)
	if tk.AssignedTo == nil || *tk.AssignedTo != "claude" {
		t.Errorf("assigned_to = %v, want claude", tk.AssignedTo)
	}
}

func TestListTickets_Filters(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "a", Team: "alpha"})
	CreateTicket(ctx, db, NewTicketOpts{Type: "bug", Title: "b", Team: "beta"})

	tks, err := ListTickets(ctx, db, TicketFilter{Team: "alpha"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tks) != 1 || tks[0].Title != "a" {
		t.Errorf("filter team=alpha: got %d tickets", len(tks))
	}
}

func TestNextTicketID_Increments(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id1, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "first"})
	id2, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "second"})

	if id1 != "T-001" || id2 != "T-002" {
		t.Errorf("ids = %q, %q — want T-001, T-002", id1, id2)
	}
}
