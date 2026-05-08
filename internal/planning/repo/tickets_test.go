package repo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/postmeridiem/pql/internal/planning"
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

func TestListTickets_Unrefined(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	desc := "has a description"
	CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "described", Description: desc})
	CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "empty"})
	idWS, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "whitespace", Description: "   "})
	idDone, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "done-empty"})
	if err := SetStatus(ctx, db, idDone, "done", ""); err != nil {
		t.Fatalf("setstatus done: %v", err)
	}
	idCancelled, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "cancelled-empty"})
	if err := SetStatus(ctx, db, idCancelled, "cancelled", ""); err != nil {
		t.Fatalf("setstatus cancelled: %v", err)
	}

	tks, err := ListTickets(ctx, db, TicketFilter{Unrefined: true})
	if err != nil {
		t.Fatalf("list unrefined: %v", err)
	}
	titles := make([]string, len(tks))
	for i, tk := range tks {
		titles[i] = tk.Title
	}
	if len(tks) != 2 {
		t.Fatalf("unrefined count = %d (%v), want 2 (empty, whitespace)", len(tks), titles)
	}
	// Whitespace-described row should appear.
	foundWS := false
	for _, tk := range tks {
		if tk.ID == idWS {
			foundWS = true
		}
	}
	if !foundWS {
		t.Errorf("whitespace-only description not surfaced as unrefined")
	}
}

func TestListTickets_UnrefinedOrdering(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	// Create five tickets with empty descriptions in different statuses.
	idBacklog, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "a"})
	idReady, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "b"})
	if err := SetStatus(ctx, db, idReady, "ready", ""); err != nil {
		t.Fatal(err)
	}
	idInProgress, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "c"})
	if err := SetStatus(ctx, db, idInProgress, "ready", ""); err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(ctx, db, idInProgress, "in_progress", ""); err != nil {
		t.Fatal(err)
	}
	idReview, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "d"})
	if err := SetStatus(ctx, db, idReview, "ready", ""); err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(ctx, db, idReview, "in_progress", ""); err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(ctx, db, idReview, "review", ""); err != nil {
		t.Fatal(err)
	}

	tks, _ := ListTickets(ctx, db, TicketFilter{Unrefined: true})
	if len(tks) != 4 {
		t.Fatalf("got %d, want 4", len(tks))
	}
	wantOrder := []string{idInProgress, idReview, idReady, idBacklog}
	for i, want := range wantOrder {
		if tks[i].ID != want {
			gotIDs := make([]string, len(tks))
			for j, tk := range tks {
				gotIDs[j] = tk.ID
			}
			t.Fatalf("position %d: got %s, want %s (full order %v, want %v)", i, tks[i].ID, want, gotIDs, wantOrder)
		}
	}
}

func TestUpdateTicket(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "before"})

	desc := "new description"
	title := "after"
	pri := "high"
	if err := UpdateTicket(ctx, db, id, UpdateTicketFields{
		Title:       &title,
		Description: &desc,
		Priority:    &pri,
	}, "tester"); err != nil {
		t.Fatalf("update: %v", err)
	}

	tk, _ := GetTicket(ctx, db, id)
	if tk.Title != "after" || tk.Priority != "high" || tk.Description == nil || *tk.Description != desc {
		t.Errorf("post-update ticket = %+v", tk)
	}

	// History row per changed field.
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ticket_history WHERE ticket_id = ? AND changed_by = 'tester'`,
		id).Scan(&n); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if n != 3 {
		t.Errorf("history rows = %d, want 3", n)
	}
}

func TestUpdateTicket_Validation(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "x"})

	bad := "loud"
	if err := UpdateTicket(ctx, db, id, UpdateTicketFields{Priority: &bad}, ""); err == nil {
		t.Error("expected error for invalid priority")
	}
	badType := "essay"
	if err := UpdateTicket(ctx, db, id, UpdateTicketFields{Type: &badType}, ""); err == nil {
		t.Error("expected error for invalid type")
	}
	empty := "  "
	if err := UpdateTicket(ctx, db, id, UpdateTicketFields{Title: &empty}, ""); err == nil {
		t.Error("expected error for empty title")
	}
	if err := UpdateTicket(ctx, db, "T-999", UpdateTicketFields{Title: ptr("x")}, ""); err == nil {
		t.Error("expected error updating missing ticket")
	}
}

func ptr[T any](v T) *T { return &v }

func TestNextTicketID_Increments(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	id1, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "first"})
	id2, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "second"})

	if id1 != "T-1" || id2 != "T-2" {
		t.Errorf("ids = %q, %q — want T-1, T-2", id1, id2)
	}
}

// readHash returns the stored hash + canonical_version for tickets.id = id.
func readHash(t *testing.T, db *sql.DB, id string) (hash string, canonicalVersion int) {
	t.Helper()
	if err := db.QueryRow(
		`SELECT hash, canonical_version FROM tickets WHERE id = ?`, id,
	).Scan(&hash, &canonicalVersion); err != nil {
		t.Fatalf("read hash %s: %v", id, err)
	}
	return hash, canonicalVersion
}

func TestCreateTicket_PopulatesHash(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, err := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "h-test"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	h, v := readHash(t, db, id)
	if h == "" {
		t.Errorf("hash empty after create")
	}
	if v != 1 {
		t.Errorf("canonical_version = %d, want 1", v)
	}
}

func TestSetStatus_ChangesHash(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "h-test"})
	before, _ := readHash(t, db, id)

	if err := SetStatus(ctx, db, id, "ready", "claude"); err != nil {
		t.Fatalf("setstatus: %v", err)
	}
	after, _ := readHash(t, db, id)
	if before == after {
		t.Errorf("hash unchanged after status change: %s", before)
	}
}

func TestUpdateTicket_NoOpKeepsHashStable(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{
		Type: "task", Title: "h-test", Priority: "medium",
	})
	before, _ := readHash(t, db, id)

	// Same values — UpdateTicket should detect no diff and return without
	// touching the row, leaving the hash stable.
	priority := "medium"
	if err := UpdateTicket(ctx, db, id, UpdateTicketFields{
		Priority: &priority,
	}, "claude"); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, _ := readHash(t, db, id)
	if before != after {
		t.Errorf("no-op update changed hash: %s -> %s", before, after)
	}
}

func TestUpdateTicket_RealChangeChangesHash(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{
		Type: "task", Title: "h-test", Priority: "medium",
	})
	before, _ := readHash(t, db, id)

	priority := "high"
	if err := UpdateTicket(ctx, db, id, UpdateTicketFields{
		Priority: &priority,
	}, "claude"); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, _ := readHash(t, db, id)
	if before == after {
		t.Errorf("real change left hash unchanged: %s", before)
	}
}

func TestSoftDelete_TicketHiddenFromReads(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "soft-del"})

	// Sanity: ticket visible.
	if tk, _ := GetTicket(ctx, db, id); tk == nil {
		t.Fatal("ticket missing before soft delete")
	}

	// Soft-delete + rehash to keep hash current.
	if _, err := db.ExecContext(ctx,
		`UPDATE tickets SET deleted_at = datetime('now') WHERE id = ?`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if tk, _ := GetTicket(ctx, db, id); tk != nil {
		t.Errorf("GetTicket returned soft-deleted row: %+v", tk)
	}
	tickets, _ := ListTickets(ctx, db, TicketFilter{})
	for _, tk := range tickets {
		if tk.ID == id {
			t.Errorf("ListTickets returned soft-deleted %s", id)
		}
	}
}

func TestSoftDelete_ChangesHash(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "hash-on-delete"})
	before, _ := readHash(t, db, id)

	if _, err := db.ExecContext(ctx,
		`UPDATE tickets SET deleted_at = datetime('now') WHERE id = ?`, id); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := planning.RehashTicket(ctx, db, id); err != nil {
		t.Fatalf("rehash after soft delete: %v", err)
	}

	// Re-read hash directly (GetTicket would filter out the deleted row).
	var after string
	if err := db.QueryRow(`SELECT hash FROM tickets WHERE id = ?`, id).Scan(&after); err != nil {
		t.Fatalf("read post-delete hash: %v", err)
	}
	if before == after {
		t.Errorf("soft delete didn't change hash: %s", before)
	}
}
