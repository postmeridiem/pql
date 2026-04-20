package primitives

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMetaOne_UnknownFileReturnsNil(t *testing.T) {
	st := seedTestStore(t)
	got, err := MetaOne(context.Background(), st.DB(), MetaOpts{Path: "ghost.md"})
	if err != nil {
		t.Fatalf("MetaOne: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unknown file, got %#v", got)
	}
}

func TestMetaOne_PopulatesAllFields(t *testing.T) {
	st := seedTestStore(t, "members/vaasa/persona.md")
	exec := st.DB().Exec
	// Frontmatter
	exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text, value_num)
	      VALUES ('members/vaasa/persona.md', 'name', 'string', '"Vaasa"', 'Vaasa', NULL)`)
	exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text, value_num)
	      VALUES ('members/vaasa/persona.md', 'voting', 'bool', 'true', NULL, 1)`)
	// Tags
	exec(`INSERT INTO tags (path, tag) VALUES ('members/vaasa/persona.md', 'council-member')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('members/vaasa/persona.md', 'voting')`)
	// Outlinks
	exec(`INSERT INTO links (source_path, target_path, alias, link_type, line)
	      VALUES ('members/vaasa/persona.md', 'Holt', 'the analyst', 'wiki', 5)`)
	// Headings
	exec(`INSERT INTO headings (path, depth, text, line_offset)
	      VALUES ('members/vaasa/persona.md', 1, 'Vaasa', 0)`)
	exec(`INSERT INTO headings (path, depth, text, line_offset)
	      VALUES ('members/vaasa/persona.md', 2, 'Background', 100)`)

	got, err := MetaOne(context.Background(), st.DB(), MetaOpts{Path: "members/vaasa/persona.md"})
	if err != nil {
		t.Fatalf("MetaOne: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil Meta")
	}
	if got.Path != "members/vaasa/persona.md" {
		t.Errorf("Path = %q", got.Path)
	}
	if got.Name != "persona" {
		t.Errorf("Name = %q, want persona", got.Name)
	}
	if len(got.Frontmatter) != 2 {
		t.Errorf("Frontmatter len = %d, want 2", len(got.Frontmatter))
	}
	// Verify raw JSON pass-through.
	var name string
	if err := json.Unmarshal(got.Frontmatter["name"], &name); err != nil {
		t.Errorf("frontmatter[name] not valid JSON: %v", err)
	}
	if name != "Vaasa" {
		t.Errorf("frontmatter[name] = %q, want Vaasa", name)
	}
	if len(got.Tags) != 2 {
		t.Errorf("Tags = %v, want 2", got.Tags)
	}
	if len(got.Outlinks) != 1 || got.Outlinks[0].Target != "Holt" || got.Outlinks[0].Alias != "the analyst" {
		t.Errorf("Outlinks = %#v", got.Outlinks)
	}
	if len(got.Headings) != 2 || got.Headings[0].Depth != 1 || got.Headings[1].Depth != 2 {
		t.Errorf("Headings = %#v", got.Headings)
	}
}

func TestMetaOne_EmptyChildrenAreNonNilSlices(t *testing.T) {
	st := seedTestStore(t, "lonely.md")
	got, err := MetaOne(context.Background(), st.DB(), MetaOpts{Path: "lonely.md"})
	if err != nil {
		t.Fatalf("MetaOne: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil Meta for indexed file")
	}
	if got.Tags == nil || got.Outlinks == nil || got.Headings == nil {
		t.Errorf("empty children should be non-nil slices, got %#v", got)
	}
	if got.Frontmatter == nil {
		t.Errorf("empty Frontmatter should be non-nil map")
	}
}

func TestMetaOne_HeadingsOrderedByLineOffset(t *testing.T) {
	st := seedTestStore(t, "x.md")
	exec := st.DB().Exec
	exec(`INSERT INTO headings (path, depth, text, line_offset) VALUES ('x.md', 1, 'third', 200)`)
	exec(`INSERT INTO headings (path, depth, text, line_offset) VALUES ('x.md', 1, 'first', 0)`)
	exec(`INSERT INTO headings (path, depth, text, line_offset) VALUES ('x.md', 1, 'second', 50)`)

	got, _ := MetaOne(context.Background(), st.DB(), MetaOpts{Path: "x.md"})
	if got == nil || len(got.Headings) != 3 {
		t.Fatalf("got %#v", got)
	}
	wantOrder := []string{"first", "second", "third"}
	for i, h := range got.Headings {
		if h.Text != wantOrder[i] {
			t.Errorf("heading[%d] = %q, want %q", i, h.Text, wantOrder[i])
		}
	}
}

func TestMetaOne_EmptyPathErrors(t *testing.T) {
	st := seedTestStore(t)
	_, err := MetaOne(context.Background(), st.DB(), MetaOpts{})
	if err == nil {
		t.Fatal("expected error for empty Path")
	}
}
