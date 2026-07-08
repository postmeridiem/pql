package render

import (
	"encoding/json"
	"strings"
	"testing"
)

type projRow struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	Ignored     string  `json:"-"`
	unexported  string
}

func TestFieldsOf_DeclarationOrderSkipsDashAndUnexported(t *testing.T) {
	got := FieldsOf[projRow]()
	want := []string{"id", "title", "description", "status"}
	if len(got) != len(want) {
		t.Fatalf("FieldsOf = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FieldsOf = %v, want %v", got, want)
		}
	}
	_ = projRow{unexported: ""} // silence unused-field vet
}

func TestProject_RequestedOrderPreserved(t *testing.T) {
	desc := "body"
	rows := []projRow{{ID: "T-1", Title: "a", Description: &desc, Status: "ready"}}
	out, err := Project(rows, []string{"status", "id"})
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1", len(out))
	}
	want := `{"status":"ready","id":"T-1"}`
	if string(out[0]) != want {
		t.Fatalf("row = %s, want %s", out[0], want)
	}
}

func TestProject_OmitemptyFieldSkippedWhenAbsent(t *testing.T) {
	rows := []projRow{{ID: "T-1", Title: "a", Status: "ready"}} // nil Description
	out, err := Project(rows, []string{"id", "description"})
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	want := `{"id":"T-1"}`
	if string(out[0]) != want {
		t.Fatalf("row = %s, want %s", out[0], want)
	}
}

func TestProject_UnknownFieldErrorsWithValidList(t *testing.T) {
	_, err := Project([]projRow{{}}, []string{"id", "titel"})
	if err == nil {
		t.Fatal("want error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), `"titel"`) || !strings.Contains(err.Error(), "title") {
		t.Fatalf("error should name the bad field and list valid ones, got: %v", err)
	}
}

func TestProject_OutputSurvivesRenderRoundTrip(t *testing.T) {
	rows := []projRow{{ID: "T-1", Title: "x", Status: "done"}, {ID: "T-2", Title: "y", Status: "ready"}}
	out, err := Project(rows, []string{"id", "status"})
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	var sb strings.Builder
	n, err := Render(out, Opts{Format: FormatPretty, Out: &sb})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 2 {
		t.Fatalf("rendered %d rows, want 2", n)
	}
	var back []map[string]string
	if err := json.Unmarshal([]byte(sb.String()), &back); err != nil {
		t.Fatalf("pretty output is not valid JSON: %v\n%s", err, sb.String())
	}
	if back[1]["id"] != "T-2" || len(back[0]) != 2 {
		t.Fatalf("round trip mismatch: %v", back)
	}
}
