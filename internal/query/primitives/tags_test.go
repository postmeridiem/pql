package primitives

import (
	"context"
	"testing"
)

func TestTags_GroupedByTag(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md")
	exec := st.DB().Exec
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'foo')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'bar')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('b.md', 'foo')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('c.md', 'bar')`)

	got, err := Tags(context.Background(), st.DB(), TagsOpts{})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	want := []TagCount{
		{Tag: "bar", Count: 2},
		{Tag: "foo", Count: 2},
	}
	if !tagCountsEq(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTags_SortByCount(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md")
	exec := st.DB().Exec
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'rare')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'common')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('b.md', 'common')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('c.md', 'common')`)

	got, err := Tags(context.Background(), st.DB(), TagsOpts{Sort: "count"})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(got) != 2 || got[0].Tag != "common" || got[1].Tag != "rare" {
		t.Errorf("sort by count failed: %v", got)
	}
}

func TestTags_MinCountFilter(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md")
	exec := st.DB().Exec
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'oneoff')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'shared')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('b.md', 'shared')`)

	got, err := Tags(context.Background(), st.DB(), TagsOpts{MinCount: 2})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(got) != 1 || got[0].Tag != "shared" {
		t.Errorf("min-count filter failed: %v", got)
	}
}

func TestTags_LimitClamps(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md")
	exec := st.DB().Exec
	exec(`INSERT INTO tags (path, tag) VALUES ('a.md', 'a')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('b.md', 'b')`)
	exec(`INSERT INTO tags (path, tag) VALUES ('c.md', 'c')`)

	got, err := Tags(context.Background(), st.DB(), TagsOpts{Limit: 2})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestTags_EmptyResultIsNonNilSlice(t *testing.T) {
	st := seedTestStore(t)
	got, err := Tags(context.Background(), st.DB(), TagsOpts{})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil slice")
	}
}

func TestTags_InvalidSortErrors(t *testing.T) {
	st := seedTestStore(t)
	_, err := Tags(context.Background(), st.DB(), TagsOpts{Sort: "size"})
	if err == nil {
		t.Fatal("expected error for invalid sort, got nil")
	}
}

func tagCountsEq(a, b []TagCount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
