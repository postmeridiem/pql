package eval

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/postmeridiem/pql/internal/query/dsl/parse"
	"github.com/postmeridiem/pql/internal/store"
)

// runDSL parses + compiles + executes src against st, returning the rows.
// Failures fail the test.
func runDSL(t *testing.T, st *store.Store, src string) []Row {
	t.Helper()
	q, err := parse.Parse(src)
	if err != nil {
		t.Fatalf("parse(%q): %v", src, err)
	}
	c, err := Compile(q)
	if err != nil {
		t.Fatalf("compile(%q): %v", src, err)
	}
	rows, err := Exec(context.Background(), st.DB(), c)
	if err != nil {
		t.Fatalf("exec(%q):\n  SQL:    %s\n  params: %v\n  err:    %v", src, c.SQL, c.Params, err)
	}
	return rows
}

// freshStore opens an empty store at a temp path.
func freshStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	st, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// seedFile inserts one files row + matching frontmatter/tags/links rows
// described by the helper args. Keeps test setup terse.
type fileSeed struct {
	path  string
	mtime int64
	size  int64
	fm    map[string]fmVal // key → typed value
	tags  []string
	links []linkSeed
}

type fmVal struct {
	typ       string  // "string" | "number" | "bool" | "list" | "object"
	valueText string  // when typ=string
	valueNum  float64 // when typ=number or bool
	valueJSON string  // raw canonical JSON for value_json
}

type linkSeed struct {
	target string
	alias  string
	typ    string
	line   int
}

func seed(t *testing.T, st *store.Store, files ...fileSeed) {
	t.Helper()
	for _, f := range files {
		_, err := st.DB().Exec(
			`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			f.path, f.mtime, f.mtime, f.size, "h", int64(0),
		)
		if err != nil {
			t.Fatalf("seed file %s: %v", f.path, err)
		}
		for k, v := range f.fm {
			vt := any(nil)
			if v.typ == "string" {
				vt = v.valueText
			}
			vn := any(nil)
			if v.typ == "number" || v.typ == "bool" {
				vn = v.valueNum
			}
			_, err := st.DB().Exec(
				`INSERT INTO frontmatter (path, key, type, value_json, value_text, value_num)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				f.path, k, v.typ, v.valueJSON, vt, vn,
			)
			if err != nil {
				t.Fatalf("seed fm %s/%s: %v", f.path, k, err)
			}
		}
		for _, tag := range f.tags {
			_, err := st.DB().Exec(`INSERT INTO tags (path, tag) VALUES (?, ?)`, f.path, tag)
			if err != nil {
				t.Fatalf("seed tag %s/%s: %v", f.path, tag, err)
			}
		}
		for _, l := range f.links {
			alias := any(nil)
			if l.alias != "" {
				alias = l.alias
			}
			_, err := st.DB().Exec(
				`INSERT INTO links (source_path, target_path, alias, link_type, line)
				 VALUES (?, ?, ?, ?, ?)`,
				f.path, l.target, alias, l.typ, l.line,
			)
			if err != nil {
				t.Fatalf("seed link %s→%s: %v", f.path, l.target, err)
			}
		}
	}
}

// --- file-column queries ------------------------------------------------

func TestExec_SelectStarReturnsFileColumns(t *testing.T) {
	st := freshStore(t)
	seed(t, st, fileSeed{path: "a.md", mtime: 100, size: 10})
	rows := runDSL(t, st, "SELECT *")
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1: %v", len(rows), rows)
	}
	r := rows[0]
	if r["path"] != "a.md" {
		t.Errorf("path = %v, want a.md", r["path"])
	}
	if v, _ := r["mtime"].(int64); v != 100 {
		t.Errorf("mtime = %v, want 100", r["mtime"])
	}
	if v, _ := r["size"].(int64); v != 10 {
		t.Errorf("size = %v, want 10", r["size"])
	}
}

func TestExec_NameAndFolderDerived(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "members/vaasa/persona.md"},
		fileSeed{path: "README.md"},
	)
	rows := runDSL(t, st, "SELECT path, name, folder ORDER BY path")
	if len(rows) != 2 {
		t.Fatalf("len = %d", len(rows))
	}
	// Row 0: README.md → name=README, folder=""
	if rows[0]["name"] != "README" {
		t.Errorf("README name = %v, want README", rows[0]["name"])
	}
	if rows[0]["folder"] != "" {
		t.Errorf("README folder = %q, want empty", rows[0]["folder"])
	}
	// Row 1: members/vaasa/persona.md → name=persona, folder=members/vaasa
	if rows[1]["name"] != "persona" {
		t.Errorf("persona name = %v, want persona", rows[1]["name"])
	}
	if rows[1]["folder"] != "members/vaasa" {
		t.Errorf("persona folder = %q, want members/vaasa", rows[1]["folder"])
	}
}

// --- WHERE on file columns ---------------------------------------------

func TestExec_WhereStringEquality(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "members/a.md"},
		fileSeed{path: "members/b.md"},
		fileSeed{path: "sessions/x.md"},
	)
	rows := runDSL(t, st, "SELECT path WHERE folder = 'members' ORDER BY path")
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2: %v", len(rows), rows)
	}
}

func TestExec_WhereNumericComparison(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "old.md", mtime: 100},
		fileSeed{path: "new.md", mtime: 200},
	)
	rows := runDSL(t, st, "SELECT path WHERE mtime > 150")
	if len(rows) != 1 || rows[0]["path"] != "new.md" {
		t.Errorf("got %v", rows)
	}
}

func TestExec_WhereGlob(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "sessions/2026/x.md"},
		fileSeed{path: "members/y.md"},
	)
	rows := runDSL(t, st, "SELECT path WHERE path GLOB 'sessions/*'")
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1: %v", len(rows), rows)
	}
}

// --- 'x' IN tags --------------------------------------------------------

func TestExec_InTagsMembership(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "a.md", tags: []string{"shared", "rare"}},
		fileSeed{path: "b.md", tags: []string{"shared"}},
		fileSeed{path: "c.md", tags: []string{"unrelated"}},
	)
	rows := runDSL(t, st, "SELECT path WHERE 'shared' IN tags ORDER BY path")
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2: %v", len(rows), rows)
	}
	if rows[0]["path"] != "a.md" || rows[1]["path"] != "b.md" {
		t.Errorf("got %v, want a.md, b.md", rows)
	}
}

func TestExec_NotInTagsExcludes(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "tagged.md", tags: []string{"foo"}},
		fileSeed{path: "untagged.md"},
	)
	rows := runDSL(t, st, "SELECT path WHERE 'foo' NOT IN tags")
	if len(rows) != 1 || rows[0]["path"] != "untagged.md" {
		t.Errorf("got %v", rows)
	}
}

// --- fm.<key> -----------------------------------------------------------

func TestExec_FmStringEquality(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "vaasa.md", fm: map[string]fmVal{
			"type": {typ: "string", valueText: "council-member", valueJSON: `"council-member"`},
		}},
		fileSeed{path: "outcome.md", fm: map[string]fmVal{
			"type": {typ: "string", valueText: "council-session", valueJSON: `"council-session"`},
		}},
	)
	rows := runDSL(t, st, "SELECT path WHERE fm.type = 'council-member'")
	if len(rows) != 1 || rows[0]["path"] != "vaasa.md" {
		t.Errorf("got %v", rows)
	}
}

func TestExec_FmBoolEqualsTrue(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "voting.md", fm: map[string]fmVal{
			"voting": {typ: "bool", valueNum: 1, valueJSON: `true`},
		}},
		fileSeed{path: "nonvoting.md", fm: map[string]fmVal{
			"voting": {typ: "bool", valueNum: 0, valueJSON: `false`},
		}},
	)
	rows := runDSL(t, st, "SELECT path WHERE fm.voting = TRUE")
	if len(rows) != 1 || rows[0]["path"] != "voting.md" {
		t.Errorf("got %v", rows)
	}
}

func TestExec_FmNumericComparison(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "low.md", fm: map[string]fmVal{
			"score": {typ: "number", valueNum: 3, valueJSON: `3`},
		}},
		fileSeed{path: "high.md", fm: map[string]fmVal{
			"score": {typ: "number", valueNum: 8, valueJSON: `8`},
		}},
	)
	rows := runDSL(t, st, "SELECT path WHERE fm.score > 5")
	if len(rows) != 1 || rows[0]["path"] != "high.md" {
		t.Errorf("got %v", rows)
	}
}

func TestExec_FmKeyAbsentFiltersOut(t *testing.T) {
	st := freshStore(t)
	seed(t, st,
		fileSeed{path: "withkey.md", fm: map[string]fmVal{
			"type": {typ: "string", valueText: "x", valueJSON: `"x"`},
		}},
		fileSeed{path: "without.md"},
	)
	rows := runDSL(t, st, "SELECT path WHERE fm.type = 'x'")
	if len(rows) != 1 || rows[0]["path"] != "withkey.md" {
		t.Errorf("got %v", rows)
	}
}

// --- ORDER BY + LIMIT ---------------------------------------------------

func TestExec_OrderByMtimeDescLimit(t *testing.T) {
	st := freshStore(t)
	for i, p := range []string{"a.md", "b.md", "c.md", "d.md", "e.md"} {
		seed(t, st, fileSeed{path: p, mtime: int64(100 + i*10)})
	}
	rows := runDSL(t, st, "SELECT path ORDER BY mtime DESC LIMIT 2")
	if len(rows) != 2 {
		t.Fatalf("len = %d, want 2", len(rows))
	}
	if rows[0]["path"] != "e.md" || rows[1]["path"] != "d.md" {
		t.Errorf("got %v, want [e.md d.md]", []any{rows[0]["path"], rows[1]["path"]})
	}
}

// --- aliases + projections ---------------------------------------------

func TestExec_ExplicitAlias(t *testing.T) {
	st := freshStore(t)
	seed(t, st, fileSeed{path: "x.md"})
	rows := runDSL(t, st, "SELECT path AS p")
	if _, ok := rows[0]["p"]; !ok {
		t.Errorf("alias 'p' not present in row: %v", rows[0])
	}
}

func TestExec_FmAutoAlias(t *testing.T) {
	st := freshStore(t)
	seed(t, st, fileSeed{path: "x.md", fm: map[string]fmVal{
		"name": {typ: "string", valueText: "Vaasa", valueJSON: `"Vaasa"`},
	}})
	rows := runDSL(t, st, "SELECT fm.name")
	if v, ok := rows[0]["fm.name"]; !ok || v != "Vaasa" {
		t.Errorf("expected fm.name=Vaasa, got row=%v", rows[0])
	}
}

// --- normalise: JSON pass-through ---------------------------------------

func TestExec_JSONColumnRoundTrips(t *testing.T) {
	st := freshStore(t)
	seed(t, st, fileSeed{path: "x.md", fm: map[string]fmVal{
		"prior_jobs": {typ: "list", valueJSON: `["analyst","lecturer"]`},
	}})
	rows := runDSL(t, st, "SELECT fm.prior_jobs")
	v := rows[0]["fm.prior_jobs"]
	raw, ok := v.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage for list value, got %T (%v)", v, v)
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Errorf("RawMessage doesn't decode as expected list: %v (%s)", err, raw)
	}
	if len(arr) != 2 || arr[0] != "analyst" || arr[1] != "lecturer" {
		t.Errorf("decoded list = %v", arr)
	}
}

// --- empty + zero-row cases --------------------------------------------

func TestExec_EmptyVaultReturnsEmptySlice(t *testing.T) {
	st := freshStore(t)
	rows := runDSL(t, st, "SELECT path")
	if rows == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(rows) != 0 {
		t.Errorf("len = %d, want 0", len(rows))
	}
}
