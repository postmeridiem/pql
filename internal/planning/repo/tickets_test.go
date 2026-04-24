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
	if id != "T-1" {
		t.Errorf("id = %q, want T-1", id)
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

	for _, next := range []string{"ready", "done", "backlog", "in_progress", "cancelled"} {
		if err := SetStatus(ctx, db, id, next, "test"); err != nil {
			t.Fatalf("%s->%s: %v", "prev", next, err)
		}
		tk, _ := GetTicket(ctx, db, id)
		if tk.Status != next {
			t.Errorf("status = %q, want %q", tk.Status, next)
		}
	}

	if err := SetStatus(ctx, db, id, "bogus", "test"); err == nil {
		t.Fatal("expected error for invalid status")
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

func TestWhatNext(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	// Nothing actionable → nil ticket, no error.
	tk, err := WhatNext(ctx, db)
	if err != nil {
		t.Fatalf("whatnext empty: %v", err)
	}
	if tk != nil {
		t.Fatal("expected nil ticket when nothing is actionable")
	}

	// Create two tickets: low-priority ready and high-priority ready.
	id1, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "low ready", Priority: "low"})
	_ = SetStatus(ctx, db, id1, "ready", "test")

	id2, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "high ready", Priority: "high"})
	_ = SetStatus(ctx, db, id2, "ready", "test")

	tk, err = WhatNext(ctx, db)
	if err != nil {
		t.Fatalf("whatnext ready: %v", err)
	}
	if tk == nil || tk.ID != id2 {
		t.Fatalf("expected %s (high priority), got %v", id2, tk)
	}

	// In-progress beats ready regardless of priority.
	_ = SetStatus(ctx, db, id1, "in_progress", "test")
	tk, err = WhatNext(ctx, db)
	if err != nil {
		t.Fatalf("whatnext in_progress: %v", err)
	}
	if tk == nil || tk.ID != id1 {
		t.Fatalf("expected %s (in_progress), got %v", id1, tk)
	}

	// Review tickets are excluded.
	_ = SetStatus(ctx, db, id1, "review", "test")
	tk, err = WhatNext(ctx, db)
	if err != nil {
		t.Fatalf("whatnext skip review: %v", err)
	}
	if tk == nil || tk.ID != id2 {
		t.Fatalf("expected %s (ready, skipping review), got %v", id2, tk)
	}
}

func TestNextReview(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	tk, err := NextReview(ctx, db)
	if err != nil {
		t.Fatalf("next review empty: %v", err)
	}
	if tk != nil {
		t.Fatal("expected nil when no review tickets")
	}

	id1, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "review me", Priority: "medium"})
	_ = SetStatus(ctx, db, id1, "review", "test")

	tk, err = NextReview(ctx, db)
	if err != nil {
		t.Fatalf("next review: %v", err)
	}
	if tk == nil || tk.ID != id1 {
		t.Fatalf("expected %s, got %v", id1, tk)
	}
}

func TestAncestors(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id1, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "initiative", Title: "top"})
	id2, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "epic", Title: "mid", ParentID: id1})
	id3, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "leaf", ParentID: id2})

	leaf, _ := GetTicket(ctx, db, id3)
	ancestors, err := Ancestors(ctx, db, leaf)
	if err != nil {
		t.Fatalf("ancestors: %v", err)
	}
	if len(ancestors) != 2 {
		t.Fatalf("expected 2 ancestors, got %d", len(ancestors))
	}
	if ancestors[0].ID != id2 {
		t.Errorf("ancestors[0] = %s, want %s", ancestors[0].ID, id2)
	}
	if ancestors[1].ID != id1 {
		t.Errorf("ancestors[1] = %s, want %s", ancestors[1].ID, id1)
	}

	// No parent → empty ancestors.
	top, _ := GetTicket(ctx, db, id1)
	ancestors, err = Ancestors(ctx, db, top)
	if err != nil {
		t.Fatalf("ancestors root: %v", err)
	}
	if len(ancestors) != 0 {
		t.Errorf("expected 0 ancestors for root, got %d", len(ancestors))
	}
}

func TestNextTicketID_Increments(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id1, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "first"})
	id2, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "second"})

	if id1 != "T-1" || id2 != "T-2" {
		t.Errorf("ids = %q, %q — want T-1, T-2", id1, id2)
	}
}
