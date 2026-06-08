package planning

import (
	"reflect"
	"testing"
)

func TestDefaultStatusSet(t *testing.T) {
	ss := DefaultStatusSet()
	if got := ss.Names(); !reflect.DeepEqual(got, []string{
		"backlog", "ready", "in_progress", "review", "done", "cancelled",
	}) {
		t.Fatalf("Names() = %v", got)
	}
	if ss.Default() != "backlog" {
		t.Errorf("Default() = %q, want backlog", ss.Default())
	}
	if got := ss.Terminal(); !reflect.DeepEqual(got, []string{"done", "cancelled"}) {
		t.Errorf("Terminal() = %v, want [done cancelled]", got)
	}
	if got := ss.Active(); !reflect.DeepEqual(got, []string{"in_progress"}) {
		t.Errorf("Active() = %v, want [in_progress]", got)
	}
	if got := ss.Review(); !reflect.DeepEqual(got, []string{"review"}) {
		t.Errorf("Review() = %v, want [review]", got)
	}
	// Ready lane is the most-advanced (last) initial status.
	if ss.ReadyLane() != "ready" {
		t.Errorf("ReadyLane() = %q, want ready", ss.ReadyLane())
	}
	if ss.OrderRank("in_progress") != 2 {
		t.Errorf("OrderRank(in_progress) = %d, want 2", ss.OrderRank("in_progress"))
	}
	if !ss.IsValid("review") || ss.IsValid("nope") {
		t.Error("IsValid wrong")
	}
}

func TestNewStatusSet_DerivesLabelAndTerminal(t *testing.T) {
	ss := NewStatusSet([]StatusDef{
		{Name: "in_progress", Class: StatusClassActive},
		{Name: "done", Class: StatusClassTerminal},
		{Name: "Custom Label", Label: "Kept", Class: StatusClassInitial, IsDefault: true},
	})
	all := ss.All()
	if all[0].Label != "In Progress" {
		t.Errorf("derived label = %q, want %q", all[0].Label, "In Progress")
	}
	if !all[1].IsTerminal {
		t.Error("done should be IsTerminal")
	}
	if all[0].IsTerminal {
		t.Error("in_progress should not be IsTerminal")
	}
	if all[2].Label != "Kept" {
		t.Errorf("explicit label should be preserved, got %q", all[2].Label)
	}
	if all[0].Order != 0 || all[2].Order != 2 {
		t.Errorf("orders = %d,%d want 0,2", all[0].Order, all[2].Order)
	}
}

func TestStatusSet_ReadyLaneAndDefaultFallbacks(t *testing.T) {
	// Multiple initial statuses: ReadyLane = last; Default = the flagged one.
	ss := NewStatusSet([]StatusDef{
		{Name: "icebox", Class: StatusClassInitial},
		{Name: "todo", Class: StatusClassInitial, IsDefault: true},
		{Name: "ready", Class: StatusClassInitial},
		{Name: "done", Class: StatusClassTerminal},
	})
	if ss.ReadyLane() != "ready" {
		t.Errorf("ReadyLane() = %q, want ready", ss.ReadyLane())
	}
	if ss.Default() != "todo" {
		t.Errorf("Default() = %q, want todo", ss.Default())
	}

	// No active and no review classes are legal.
	if got := ss.Active(); got != nil {
		t.Errorf("Active() = %v, want nil", got)
	}
	if got := ss.Review(); got != nil {
		t.Errorf("Review() = %v, want nil", got)
	}
}

func TestStatusSet_IsZero(t *testing.T) {
	if !(StatusSet{}).IsZero() {
		t.Error("zero value should be IsZero")
	}
	if DefaultStatusSet().IsZero() {
		t.Error("default set should not be IsZero")
	}
}
