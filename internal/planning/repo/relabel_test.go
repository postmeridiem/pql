package repo

import (
	"context"
	"testing"
)

func TestRelabel_ByLabelPreservesRecordID(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "x"})
	before, _ := GetTicket(ctx, db, id)

	old, rec, err := Relabel(ctx, db, id, "T-100")
	if err != nil {
		t.Fatalf("relabel: %v", err)
	}
	if old != id {
		t.Errorf("old label = %q, want %q", old, id)
	}
	if rec != before.RecordID {
		t.Errorf("record_id changed: %q -> %q (relabel must preserve identity)", before.RecordID, rec)
	}
	// New label resolves; old label no longer does.
	if tk, _ := GetTicket(ctx, db, "T-100"); tk == nil || tk.RecordID != before.RecordID {
		t.Fatalf("new label T-100 should resolve to the same record")
	}
	if tk, _ := GetTicket(ctx, db, id); tk != nil {
		t.Errorf("old label %s should no longer resolve, got %+v", id, tk)
	}
}

func TestRelabel_RefusesClash(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	a, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "a"}) // T-1
	b, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "b"}) // T-2
	if _, _, err := Relabel(ctx, db, a, b); err == nil {
		t.Errorf("relabel %s -> %s should be refused (label in use)", a, b)
	}
}

func TestRelabel_ByRecordIDDisambiguates(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)
	id, _ := CreateTicket(ctx, db, NewTicketOpts{Type: "task", Title: "x"})
	rec := recordOf(t, db, id)
	old, got, err := Relabel(ctx, db, rec, "T-77")
	if err != nil {
		t.Fatalf("relabel by record_id: %v", err)
	}
	if got != rec || old != id {
		t.Errorf("relabel by record_id = (old %q, rec %q), want (%q, %q)", old, got, id, rec)
	}
}
