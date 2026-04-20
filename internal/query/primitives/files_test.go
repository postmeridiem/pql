package primitives

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/postmeridiem/pql/internal/store"
)

// seedTestStore opens a temp store and seeds files rows for query tests.
// We don't need the indexer here — we exercise primitives in isolation,
// inserting straight into the schema with the columns we need.
func seedTestStore(t *testing.T, paths ...string) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for i, p := range paths {
		_, err := st.DB().Exec(
			`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p, int64(1700000000+i), int64(1700000000+i), int64(100+i),
			"hash"+itoa(i), int64(1700000999),
		)
		if err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}
	return st
}

func TestFiles_AllRowsReturnedSorted(t *testing.T) {
	st := seedTestStore(t, "z.md", "a.md", "members/c.md", "members/a.md")
	got, err := Files(context.Background(), st.DB(), FilesOpts{})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	wantPaths := []string{"a.md", "members/a.md", "members/c.md", "z.md"}
	if len(got) != len(wantPaths) {
		t.Fatalf("len = %d, want %d (got %v)", len(got), len(wantPaths), pathsOf(got))
	}
	for i, want := range wantPaths {
		if got[i].Path != want {
			t.Errorf("[%d].Path = %q, want %q", i, got[i].Path, want)
		}
	}
}

func TestFiles_NameDerivedFromPath(t *testing.T) {
	st := seedTestStore(t, "members/vaasa/persona.md", "README.md")
	got, err := Files(context.Background(), st.DB(), FilesOpts{})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	wantNames := map[string]string{
		"README.md":                  "README",
		"members/vaasa/persona.md":   "persona",
	}
	for _, f := range got {
		if want, ok := wantNames[f.Path]; ok && f.Name != want {
			t.Errorf("Name(%q) = %q, want %q", f.Path, f.Name, want)
		}
	}
}

func TestFiles_GlobFilter(t *testing.T) {
	st := seedTestStore(t, "a.md", "members/x.md", "members/y.md", "sessions/z.md")
	got, err := Files(context.Background(), st.DB(), FilesOpts{Glob: "members/*"})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	want := []string{"members/x.md", "members/y.md"}
	if !sliceEq(pathsOf(got), want) {
		t.Errorf("got %v, want %v", pathsOf(got), want)
	}
}

func TestFiles_LimitClamps(t *testing.T) {
	st := seedTestStore(t, "a.md", "b.md", "c.md", "d.md", "e.md")
	got, err := Files(context.Background(), st.DB(), FilesOpts{Limit: 3})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
	if !sliceEq(pathsOf(got), []string{"a.md", "b.md", "c.md"}) {
		t.Errorf("got %v", pathsOf(got))
	}
}

func TestFiles_EmptyResultIsNonNilSlice(t *testing.T) {
	st := seedTestStore(t)
	got, err := Files(context.Background(), st.DB(), FilesOpts{})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil empty slice (renderers prefer [] over null)")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFiles_PopulatesSizeAndMtime(t *testing.T) {
	st := seedTestStore(t, "x.md")
	got, err := Files(context.Background(), st.DB(), FilesOpts{})
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Size == 0 {
		t.Error("Size should be populated from seeded value")
	}
	if got[0].Mtime == 0 {
		t.Error("Mtime should be populated from seeded value")
	}
}

// --- helpers --------------------------------------------------------------

func pathsOf(fs []File) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Path
	}
	return out
}

func sliceEq(a, b []string) bool {
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
