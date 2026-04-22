package base

import (
	"path/filepath"
	"testing"

	"github.com/postmeridiem/pql/internal/query/dsl/eval"
	"github.com/postmeridiem/pql/internal/query/dsl/parse"
)

func TestCompile_CouncilMembers(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "council-snapshot", "council-members.base")
	q, err := Compile(path, "")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertCompiles(t, q)

	if len(q.Select) != 5 {
		t.Errorf("SELECT has %d projections, want 5 (name, prior_job, lens, voting, model)", len(q.Select))
	}
	if q.Where == nil {
		t.Fatal("WHERE is nil, want fm.type = 'council-member'")
	}
	if len(q.OrderBy) != 2 {
		t.Errorf("ORDER BY has %d items, want 2", len(q.OrderBy))
	}
	if len(q.OrderBy) >= 1 && !q.OrderBy[0].Desc {
		t.Error("first sort (voting) should be DESC")
	}
	if len(q.OrderBy) >= 2 && q.OrderBy[1].Desc {
		t.Error("second sort (name) should be ASC")
	}
}

func TestCompile_CouncilSessions(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "council-snapshot", "council-sessions.base")
	q, err := Compile(path, "")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	assertCompiles(t, q)

	if len(q.Select) != 6 {
		t.Errorf("SELECT has %d projections, want 6", len(q.Select))
	}
	if q.Where == nil {
		t.Fatal("WHERE is nil")
	}
	if len(q.OrderBy) != 1 {
		t.Errorf("ORDER BY has %d items, want 1", len(q.OrderBy))
	}
}

func TestCompile_NamedView(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "council-snapshot", "council-members.base")
	q, err := Compile(path, "The Council")
	if err != nil {
		t.Fatalf("Compile with named view: %v", err)
	}
	if len(q.Select) != 5 {
		t.Errorf("SELECT has %d projections, want 5", len(q.Select))
	}
}

func TestCompile_UnknownView(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "council-snapshot", "council-members.base")
	_, err := Compile(path, "Nonexistent View")
	if err == nil {
		t.Fatal("expected error for unknown view name")
	}
}

func TestCompileBytes_MinimalBase(t *testing.T) {
	yaml := []byte(`
filters:
  and:
    - note.type == "test"
properties:
  note.name:
    displayName: Name
views:
  - type: table
    name: default
    order: [name]
    sort:
      - property: name
        direction: ASC
`)
	q, err := CompileBytes(yaml, "")
	if err != nil {
		t.Fatalf("CompileBytes: %v", err)
	}
	assertCompiles(t, q)

	if len(q.Select) != 1 {
		t.Errorf("SELECT has %d projections, want 1", len(q.Select))
	}
	if q.Where == nil {
		t.Fatal("WHERE is nil")
	}
}

func TestParseCondition_Operators(t *testing.T) {
	tests := []struct {
		cond string
		ok   bool
	}{
		{`note.type == "member"`, true},
		{`note.count > 5`, true},
		{`note.count >= 10`, true},
		{`note.voting != false`, true},
		{`file.tags contains "foo"`, true},
		{`garbage`, false},
	}
	for _, tt := range tests {
		_, err := parseCondition(tt.cond)
		if (err == nil) != tt.ok {
			t.Errorf("parseCondition(%q): err=%v, wantOk=%v", tt.cond, err, tt.ok)
		}
	}
}

// assertCompiles verifies the AST compiles to SQL without error.
func assertCompiles(t *testing.T, q *parse.Query) {
	t.Helper()
	compiled, err := eval.Compile(q)
	if err != nil {
		t.Fatalf("eval.Compile failed on base-generated AST: %v", err)
	}
	if compiled.SQL == "" {
		t.Fatal("compiled SQL is empty")
	}
	t.Logf("SQL: %s", compiled.SQL)
	t.Logf("Params: %v", compiled.Params)
}
