package primitives

import (
	"context"
	"slices"
	"testing"
)

func TestOutlinks_ReturnsAllLinksFromSource(t *testing.T) {
	st := seedTestStore(t, "src.md", "other.md")
	for _, row := range [][]any{
		{"src.md", "Holt", "the analyst", "wiki", 5},
		{"src.md", "Tintori", nil, "wiki", 3},
		{"src.md", "image.png", nil, "embed", 8},
		{"other.md", "should-not-appear", nil, "wiki", 1},
	} {
		st.DB().Exec(
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES (?, ?, ?, ?, ?)`,
			row...)
	}
	got, err := Outlinks(context.Background(), st.DB(), OutlinksOpts{Path: "src.md"})
	if err != nil {
		t.Fatalf("Outlinks: %v", err)
	}
	wantTargets := []string{"Tintori", "Holt", "image.png"} // ordered by line: 3, 5, 8
	gotTargets := make([]string, len(got))
	for i, o := range got {
		gotTargets[i] = o.Target
	}
	if !slices.Equal(gotTargets, wantTargets) {
		t.Errorf("targets %v, want %v", gotTargets, wantTargets)
	}
}

func TestOutlinks_PreservesAliasWhenSet(t *testing.T) {
	st := seedTestStore(t, "src.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, ?, ?, ?)`,
		"src.md", "Tintori", "the analyst", "wiki", 1,
	)
	got, _ := Outlinks(context.Background(), st.DB(), OutlinksOpts{Path: "src.md"})
	if len(got) != 1 || got[0].Alias != "the analyst" {
		t.Errorf("alias missing or wrong: %#v", got)
	}
}

func TestOutlinks_NullAliasIsEmptyString(t *testing.T) {
	st := seedTestStore(t, "src.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, NULL, 'wiki', 1)`,
		"src.md", "Tintori",
	)
	got, _ := Outlinks(context.Background(), st.DB(), OutlinksOpts{Path: "src.md"})
	if got[0].Alias != "" {
		t.Errorf("alias for NULL = %q, want empty", got[0].Alias)
	}
}

func TestOutlinks_OrderedByLine(t *testing.T) {
	st := seedTestStore(t, "src.md")
	for _, line := range []int{10, 1, 5} {
		st.DB().Exec(
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES ('src.md', 'a', NULL, 'wiki', ?)`,
			line)
	}
	got, _ := Outlinks(context.Background(), st.DB(), OutlinksOpts{Path: "src.md"})
	wantLines := []int{1, 5, 10}
	for i, o := range got {
		if o.Line != wantLines[i] {
			t.Errorf("got line %d at index %d, want %d", o.Line, i, wantLines[i])
		}
	}
}

func TestOutlinks_LimitClamps(t *testing.T) {
	st := seedTestStore(t, "src.md")
	for i := range 5 {
		st.DB().Exec(
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES ('src.md', ?, NULL, 'wiki', ?)`,
			"t"+itoa(i), i)
	}
	got, _ := Outlinks(context.Background(), st.DB(), OutlinksOpts{Path: "src.md", Limit: 3})
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
}

func TestOutlinks_EmptyPathErrors(t *testing.T) {
	st := seedTestStore(t)
	_, err := Outlinks(context.Background(), st.DB(), OutlinksOpts{})
	if err == nil {
		t.Fatal("expected error for empty Path, got nil")
	}
}

func TestOutlinks_UnknownPathReturnsEmpty(t *testing.T) {
	st := seedTestStore(t)
	got, err := Outlinks(context.Background(), st.DB(), OutlinksOpts{Path: "nope.md"})
	if err != nil {
		t.Fatalf("Outlinks: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
