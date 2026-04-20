package primitives

import (
	"context"
	"slices"
	"testing"
)

func TestSchema_GroupsByKey(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md")
	exec := st.DB().Exec
	for _, p := range []string{"a.md", "b.md", "c.md"} {
		exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text)
		      VALUES (?, 'name', 'string', '"x"', 'x')`, p)
		exec(`INSERT INTO frontmatter (path, key, type, value_json, value_num)
		      VALUES (?, 'voting', 'bool', 'true', 1)`, p)
	}
	got, err := Schema(context.Background(), st.DB(), SchemaOpts{})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 distinct keys: %v", len(got), got)
	}
	for _, e := range got {
		if e.Count != 3 {
			t.Errorf("%s: Count = %d, want 3", e.Key, e.Count)
		}
		if len(e.Types) != 1 {
			t.Errorf("%s: Types = %v, want one element", e.Key, e.Types)
		}
	}
}

func TestSchema_DetectsMixedTypes(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md")
	exec := st.DB().Exec
	exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text)
	      VALUES ('a.md', 'tags', 'string', '"foo"', 'foo')`)
	exec(`INSERT INTO frontmatter (path, key, type, value_json)
	      VALUES ('b.md', 'tags', 'list', '["foo","bar"]')`)

	got, err := Schema(context.Background(), st.DB(), SchemaOpts{})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (one key)", len(got))
	}
	want := []string{"list", "string"}
	if !slices.Equal(got[0].Types, want) {
		t.Errorf("Types = %v, want %v (sorted)", got[0].Types, want)
	}
	if got[0].Count != 2 {
		t.Errorf("Count = %d, want 2", got[0].Count)
	}
}

func TestSchema_SortByCount(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md")
	exec := st.DB().Exec
	// "common" present in all three; "rare" only in a.md
	for _, p := range []string{"a.md", "b.md", "c.md"} {
		exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text)
		      VALUES (?, 'common', 'string', '"x"', 'x')`, p)
	}
	exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text)
	      VALUES ('a.md', 'rare', 'string', '"y"', 'y')`)

	got, err := Schema(context.Background(), st.DB(), SchemaOpts{Sort: "count"})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(got) != 2 || got[0].Key != "common" || got[1].Key != "rare" {
		t.Errorf("sort by count failed: %v", got)
	}
}

func TestSchema_LimitClamps(t *testing.T) {
	st := seedTestStore(t, "a.md")
	exec := st.DB().Exec
	for _, k := range []string{"a", "b", "c", "d"} {
		exec(`INSERT INTO frontmatter (path, key, type, value_json, value_text)
		      VALUES ('a.md', ?, 'string', '"x"', 'x')`, k)
	}
	got, err := Schema(context.Background(), st.DB(), SchemaOpts{Limit: 2})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestSchema_EmptyVaultIsNonNilSlice(t *testing.T) {
	st := seedTestStore(t)
	got, err := Schema(context.Background(), st.DB(), SchemaOpts{})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil empty slice")
	}
}

func TestSchema_InvalidSortErrors(t *testing.T) {
	st := seedTestStore(t)
	_, err := Schema(context.Background(), st.DB(), SchemaOpts{Sort: "size"})
	if err == nil {
		t.Fatal("expected error for invalid sort")
	}
}
