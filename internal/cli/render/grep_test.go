package render

import (
	"bytes"
	"strings"
	"testing"
)

type grepRow struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Count  int      `json:"count"`
	Open   bool     `json:"open"`
	Tags   []string `json:"tags,omitempty"`
	Nested *grepRow `json:"nested,omitempty"`
}

func TestCompileGrepIsCaseInsensitive(t *testing.T) {
	re, err := CompileGrep("CHANGELOG")
	if err != nil {
		t.Fatalf("CompileGrep: %v", err)
	}
	if !re.MatchString("the changelog file") {
		t.Error("expected a case-insensitive match")
	}
}

func TestCompileGrepRejectsBadPattern(t *testing.T) {
	_, err := CompileGrep("[unclosed")
	if err == nil {
		t.Fatal("expected an error for an unparseable pattern")
	}
	// The caller surfaces this at exit 64, so it has to name the pattern
	// rather than leave the agent guessing which flag was wrong.
	if !strings.Contains(err.Error(), "[unclosed") {
		t.Errorf("error should quote the pattern, got %q", err)
	}
}

func TestMatchesTestsValuesNotKeys(t *testing.T) {
	row := grepRow{ID: "T-1", Title: "converge return shapes", Count: 3}

	for _, tc := range []struct {
		pattern string
		want    bool
		why     string
	}{
		{"converge", true, "a value substring matches"},
		{"CONVERGE", true, "matching is case-insensitive"},
		{"title", false, "a key name is not a value"},
		{"count", false, "a key name is not a value"},
		{"T-1", true, "the id is a value"},
		{"3", true, "numbers match on their rendered text"},
		{"^converge", true, "^ anchors to the start of a value, which this one has"},
		{"^return", false, "^ does not match mid-value, same as grep on that line"},
		{"shapes$", true, "$ anchors to the end of a value"},
	} {
		re, err := CompileGrep(tc.pattern)
		if err != nil {
			t.Fatalf("CompileGrep(%q): %v", tc.pattern, err)
		}
		got, err := Matches(row, re)
		if err != nil {
			t.Fatalf("Matches(%q): %v", tc.pattern, err)
		}
		if got != tc.want {
			t.Errorf("Matches(%q) = %v, want %v — %s", tc.pattern, got, tc.want, tc.why)
		}
	}
}

func TestMatchesDescendsIntoNestedValues(t *testing.T) {
	// Join trees (--with-context, --tree, ticket board columns) nest records
	// inside records; a term in a child has to count.
	row := grepRow{
		ID:     "T-1",
		Tags:   []string{"urgent", "planning"},
		Nested: &grepRow{Title: "buried treasure"},
	}
	for _, pattern := range []string{"urgent", "buried treasure"} {
		re, _ := CompileGrep(pattern)
		got, err := Matches(row, re)
		if err != nil {
			t.Fatalf("Matches(%q): %v", pattern, err)
		}
		if !got {
			t.Errorf("Matches(%q) = false, want true", pattern)
		}
	}
}

func TestMatchesBooleansOnRenderedText(t *testing.T) {
	re, _ := CompileGrep("true")
	got, err := Matches(grepRow{Open: true}, re)
	if err != nil {
		t.Fatalf("Matches: %v", err)
	}
	if !got {
		t.Error("expected a bool to match its rendered text")
	}
}

func TestFilterPreservesOrder(t *testing.T) {
	rows := []grepRow{{ID: "a", Title: "keep"}, {ID: "b", Title: "drop"}, {ID: "c", Title: "keep"}}
	re, _ := CompileGrep("keep")
	got, err := Filter(rows, re)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("Filter = %+v, want rows a and c in order", got)
	}
}

func TestRenderGrepEmitsOneRecordPerLine(t *testing.T) {
	rows := []grepRow{{ID: "a", Title: "keep me"}, {ID: "b", Title: "drop me"}, {ID: "c", Title: "keep me"}}
	re, _ := CompileGrep("keep")

	var buf bytes.Buffer
	n, err := Render(rows, Opts{Out: &buf, Grep: re})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 2 {
		t.Errorf("Render wrote %d rows, want 2", n)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), buf.String())
	}
	// No enclosing array: each line has to stand alone as JSON, which is what
	// keeps a filtered result parseable instead of a text dump.
	for i, l := range lines {
		if strings.HasPrefix(l, "[") || strings.HasSuffix(l, ",") {
			t.Errorf("line %d is array-wrapped, want bare JSONL: %s", i, l)
		}
	}
}

func TestRenderGrepAppliesLimitFirst(t *testing.T) {
	// --grep filters the output buffer, so --limit picks the page and --grep
	// filters that page: `pql X --limit 2 --grep c` == `pql X --limit 2 | grep c`.
	// Filtering first would surface row c, which --limit 2 excluded.
	rows := []grepRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	re, _ := CompileGrep("c")

	var buf bytes.Buffer
	n, err := Render(rows, Opts{Out: &buf, Limit: 2, Grep: re})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 0 {
		t.Errorf("Render wrote %d rows, want 0 — row c is outside the --limit 2 page", n)
	}
	if buf.Len() != 0 {
		t.Errorf("zero matches must write zero bytes, got %q", buf.String())
	}
}

func TestRenderGrepZeroMatchesWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	re, _ := CompileGrep("nothingmatchesthis")
	n, err := Render([]grepRow{{ID: "a"}}, Opts{Out: &buf, Grep: re})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 0 || buf.Len() != 0 {
		t.Errorf("want 0 rows and 0 bytes, got %d rows and %q", n, buf.String())
	}
}

func TestOneGrepSuppressesNonMatch(t *testing.T) {
	row := grepRow{ID: "T-1", Title: "converge return shapes"}

	var hit bytes.Buffer
	re, _ := CompileGrep("converge")
	ok, err := One(&row, Opts{Out: &hit, Grep: re})
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if !ok || hit.Len() == 0 {
		t.Error("a matching object should be emitted")
	}

	var miss bytes.Buffer
	re, _ = CompileGrep("nothingmatchesthis")
	ok, err = One(&row, Opts{Out: &miss, Grep: re})
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if ok || miss.Len() != 0 {
		t.Errorf("a non-matching object should emit nothing, got %q", miss.String())
	}
}
