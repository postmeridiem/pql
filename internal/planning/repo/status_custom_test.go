package repo

import (
	"context"
	"testing"

	"github.com/postmeridiem/pql/internal/planning"
)

// customSet is a non-default vocabulary used to prove the status-aware
// repo functions key off the configured StatusSet, not hardcoded names.
//
//	triage  (initial, default)   groomed (initial)   doing (active)
//	checking(review)             shipped (terminal)  dropped (terminal)
func customSet() planning.StatusSet {
	return planning.NewStatusSet([]planning.StatusDef{
		{Name: "triage", Class: planning.StatusClassInitial, IsDefault: true},
		{Name: "groomed", Class: planning.StatusClassInitial},
		{Name: "doing", Class: planning.StatusClassActive},
		{Name: "checking", Class: planning.StatusClassReview},
		{Name: "shipped", Class: planning.StatusClassTerminal},
		{Name: "dropped", Class: planning.StatusClassTerminal},
	})
}

func TestCreateTicket_CustomDefaultStatus(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	ss := customSet()

	id, err := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "x", DefaultStatus: ss.Default()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	tk, _ := GetTicket(ctx, db, id)
	if tk.Status != "triage" {
		t.Errorf("status = %q, want triage (the configured default)", tk.Status)
	}
}

func TestSetStatus_CustomVocabulary(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	ss := customSet()

	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "x", DefaultStatus: ss.Default()})

	if err := SetStatus(ctx, db, id, "doing", "", ss); err != nil {
		t.Fatalf("set to configured status: %v", err)
	}
	// A status from the *default* vocabulary is invalid under this set.
	if err := SetStatus(ctx, db, id, "in_progress", "", ss); err == nil {
		t.Error("expected in_progress to be rejected under the custom set")
	}
}

func TestWhatNext_CustomVocabulary(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	ss := customSet()

	mk := func(title, status string) string {
		id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: title, DefaultStatus: ss.Default()})
		if status != ss.Default() {
			if err := SetStatus(ctx, db, id, status, "", ss); err != nil {
				t.Fatalf("set %s=%s: %v", id, status, err)
			}
		}
		return id
	}

	triage := mk("raw", "triage")     // initial, but NOT the ready lane → not actionable
	groomed := mk("ready", "groomed") // ready lane
	doing := mk("active", "doing")    // active → wins

	tk, err := WhatNext(ctx, db, ss)
	if err != nil {
		t.Fatalf("whatnext: %v", err)
	}
	if tk == nil || tk.ID != doing {
		t.Fatalf("whatnext = %v, want active ticket %s", tk, doing)
	}

	// Close the active one → the ready lane (groomed) surfaces, not triage.
	if err := SetStatus(ctx, db, doing, "shipped", "", ss); err != nil {
		t.Fatal(err)
	}
	tk, _ = WhatNext(ctx, db, ss)
	if tk == nil || tk.ID != groomed {
		t.Fatalf("whatnext = %v, want ready-lane ticket %s", tk, groomed)
	}
	_ = triage
}

func TestNextReview_CustomVocabulary(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	ss := customSet()

	if tk, _ := NextReview(ctx, db, ss); tk != nil {
		t.Fatalf("expected no review ticket, got %v", tk)
	}
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "r", DefaultStatus: ss.Default()})
	if err := SetStatus(ctx, db, id, "checking", "", ss); err != nil {
		t.Fatal(err)
	}
	tk, _ := NextReview(ctx, db, ss)
	if tk == nil || tk.ID != id {
		t.Fatalf("nextreview = %v, want %s", tk, id)
	}
}

func TestNextReview_NoReviewClass(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	// A vocabulary with no review-class status — NextReview is always empty.
	ss := planning.NewStatusSet([]planning.StatusDef{
		{Name: "todo", Class: planning.StatusClassInitial, IsDefault: true},
		{Name: "done", Class: planning.StatusClassTerminal},
	})
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "x", DefaultStatus: ss.Default()})
	_ = id
	if tk, err := NextReview(ctx, db, ss); err != nil || tk != nil {
		t.Fatalf("nextreview = (%v, %v), want (nil, nil)", tk, err)
	}
}

func TestUnblocked_CustomTerminal(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	ss := customSet()

	blocked, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "blocked", DefaultStatus: ss.Default()})
	blocker, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "blocker", DefaultStatus: ss.Default()})
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_deps (blocker_id, blocked_id, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
	`, blocker, blocked); err != nil {
		t.Fatalf("insert dep: %v", err)
	}

	// Still blocked: blocker is not terminal.
	tks, _ := ListTickets(ctx, db, TicketFilter{Unblocked: true, Statuses: ss})
	for _, tk := range tks {
		if tk.ID == blocked {
			t.Fatal("blocked ticket should be excluded while blocker is open")
		}
	}

	// Move blocker to a custom terminal status → blocked becomes unblocked.
	if err := SetStatus(ctx, db, blocker, "dropped", "", ss); err != nil {
		t.Fatal(err)
	}
	tks, _ = ListTickets(ctx, db, TicketFilter{Unblocked: true, Statuses: ss})
	var found bool
	for _, tk := range tks {
		if tk.ID == blocked {
			found = true
		}
	}
	if !found {
		t.Fatal("blocked ticket should be unblocked once blocker reaches a terminal status")
	}
}
