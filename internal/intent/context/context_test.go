package context

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/postmeridiem/pql/internal/store"

	_ "modernc.org/sqlite"
)

// seedVault builds an indexed store with the given files, then the given links
// as (source, target) pairs written exactly as they would appear in markdown.
func seedVault(t *testing.T, files []string, links [][2]string) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "pql.sqlite"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	for i, p := range files {
		if _, err := st.DB().Exec(
			`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p, int64(1700000000+i), int64(1700000000+i), int64(100+i), "h", int64(1700000999),
		); err != nil {
			t.Fatalf("seed file %s: %v", p, err)
		}
	}
	for i, l := range links {
		if _, err := st.DB().Exec(
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES (?, ?, NULL, 'md', ?)`,
			l[0], l[1], i+1,
		); err != nil {
			t.Fatalf("seed link %v: %v", l, err)
		}
	}
	return st
}

// Every candidate must be a path another command will accept. Link targets are
// written text — extensionless, anchored, or a bare same-document anchor — and
// emitting them unresolved produced results that `pql meta` rejected as "file
// not indexed" (T-73).
func TestGatherCandidates_ResolvesTargetsToIndexedPaths(t *testing.T) {
	indexed := []string{
		"members/vaasa/persona.md",
		"members/koskela/persona.md",
		"docs/guide.md",
	}
	st := seedVault(t, indexed, [][2]string{
		{"members/vaasa/persona.md", "members/koskela/persona"},  // extensionless full path
		{"members/vaasa/persona.md", "docs/guide.md#installing"}, // anchored file link
	})

	got, err := gatherCandidates(context.Background(), st.DB(), "members/vaasa/persona.md")
	if err != nil {
		t.Fatalf("gatherCandidates: %v", err)
	}

	slices.Sort(got)
	want := []string{"docs/guide.md", "members/koskela/persona.md"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v — targets must resolve to indexed paths", got, want)
	}
	for _, p := range got {
		if !slices.Contains(indexed, p) {
			t.Errorf("%q is not an indexed file; a caller could not open it", p)
		}
	}
}

// A bare `#anchor` addresses a heading in the same document, so it names no
// other file. Emitting it made same-document headings look like related files —
// six of them scored and ranked against one another in a real vault.
func TestGatherCandidates_DropsSameDocumentAnchors(t *testing.T) {
	st := seedVault(t,
		[]string{"governance/decisions/architecture.md"},
		[][2]string{
			{"governance/decisions/architecture.md", "#d-19-no-alter-table"},
			{"governance/decisions/architecture.md", "#d-15-replication"},
		},
	)

	got, err := gatherCandidates(context.Background(), st.DB(), "governance/decisions/architecture.md")
	if err != nil {
		t.Fatalf("gatherCandidates: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want none — a same-document anchor is not a related file", got)
	}
}

// The target file itself must never be its own candidate, whichever branch
// reaches it.
func TestGatherCandidates_ExcludesTheTargetItself(t *testing.T) {
	st := seedVault(t,
		[]string{"a.md", "b.md"},
		[][2]string{
			{"b.md", "a"},    // inbound, extensionless
			{"a.md", "a.md"}, // a link to itself
		},
	)

	got, err := gatherCandidates(context.Background(), st.DB(), "a.md")
	if err != nil {
		t.Fatalf("gatherCandidates: %v", err)
	}
	if slices.Contains(got, "a.md") {
		t.Errorf("got %v, want the target excluded", got)
	}
	if !slices.Contains(got, "b.md") {
		t.Errorf("got %v, want the inbound link from b.md", got)
	}
}
