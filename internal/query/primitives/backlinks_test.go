package primitives

import (
	"context"
	"slices"
	"testing"
)

func TestBacklinks_MatchesByFullPath(t *testing.T) {
	st := seedTestStore(t, "src.md", "members/vaasa/persona.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, NULL, 'md', 5)`,
		"src.md", "members/vaasa/persona.md",
	)
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "members/vaasa/persona.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(got) != 1 || got[0].Path != "src.md" || got[0].Via != "md" || got[0].Line != 5 {
		t.Errorf("got %#v, want one md backlink from src.md line 5", got)
	}
}

func TestBacklinks_MatchesByBasename(t *testing.T) {
	st := seedTestStore(t, "src.md", "members/vaasa/persona.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, NULL, 'wiki', 3)`,
		"src.md", "persona", // [[persona]] resolved by basename
	)
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "members/vaasa/persona.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(got) != 1 || got[0].Via != "wiki" {
		t.Errorf("got %#v, want one wiki backlink", got)
	}
}

func TestBacklinks_MatchesBasenameWithAnchor(t *testing.T) {
	st := seedTestStore(t, "src.md", "members/vaasa/persona.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, NULL, 'wiki', 7)`,
		"src.md", "persona#background",
	)
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "members/vaasa/persona.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 backlink for anchor-suffixed wikilink, got %d (%#v)", len(got), got)
	}
}

func TestBacklinks_ExcludesSelfReference(t *testing.T) {
	st := seedTestStore(t, "members/vaasa/persona.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, NULL, 'wiki', 1)`,
		"members/vaasa/persona.md", "persona",
	)
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "members/vaasa/persona.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("self-reference should be excluded, got %#v", got)
	}
}

func TestBacklinks_DerivesNameFromSource(t *testing.T) {
	st := seedTestStore(t, "members/holt/journal.md", "members/vaasa/persona.md")
	st.DB().Exec(
		`INSERT INTO links (source_path, target_path, alias, link_type, line)
		 VALUES (?, ?, NULL, 'wiki', 1)`,
		"members/holt/journal.md", "persona",
	)
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "members/vaasa/persona.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(got) != 1 || got[0].Name != "journal" {
		t.Errorf("Name = %q, want %q", got[0].Name, "journal")
	}
}

func TestBacklinks_OrderedByPathThenLine(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md", "members/vaasa/persona.md")
	for _, row := range [][]any{
		{"c.md", "persona", "wiki", 1},
		{"a.md", "persona", "wiki", 2},
		{"b.md", "persona", "wiki", 5},
		{"a.md", "persona", "wiki", 1},
	} {
		st.DB().Exec(
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES (?, ?, NULL, ?, ?)`,
			row...)
	}
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "members/vaasa/persona.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	wantPaths := []string{"a.md", "a.md", "b.md", "c.md"}
	wantLines := []int{1, 2, 5, 1}
	gotPaths := make([]string, len(got))
	gotLines := make([]int, len(got))
	for i, b := range got {
		gotPaths[i] = b.Path
		gotLines[i] = b.Line
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Errorf("paths %v, want %v", gotPaths, wantPaths)
	}
	if !slices.Equal(gotLines, wantLines) {
		t.Errorf("lines %v, want %v", gotLines, wantLines)
	}
}

func TestBacklinks_LimitClamps(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md", "members/vaasa/persona.md")
	for _, p := range []string{"a.md", "b.md", "c.md"} {
		st.DB().Exec(
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES (?, ?, NULL, 'wiki', 1)`,
			p, "persona",
		)
	}
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path:  "members/vaasa/persona.md",
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestBacklinks_EmptyPathErrors(t *testing.T) {
	st := seedTestStore(t)
	_, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{})
	if err == nil {
		t.Fatal("expected error for empty Path, got nil")
	}
}

func TestBacklinks_NoMatchesReturnsEmptySlice(t *testing.T) {
	st := seedTestStore(t, "lonely.md")
	got, err := Backlinks(context.Background(), st.DB(), BacklinksOpts{
		Path: "lonely.md",
	})
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
