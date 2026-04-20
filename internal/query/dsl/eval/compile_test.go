package eval

import (
	"errors"
	"strings"
	"testing"

	"github.com/postmeridiem/pql/internal/query/dsl/parse"
)

func mustCompile(t *testing.T, src string) *Compiled {
	t.Helper()
	q, err := parse.Parse(src)
	if err != nil {
		t.Fatalf("parse(%q): %v", src, err)
	}
	c, err := Compile(q)
	if err != nil {
		t.Fatalf("compile(%q): %v", src, err)
	}
	return c
}

func compileErr(t *testing.T, src string) error {
	t.Helper()
	q, err := parse.Parse(src)
	if err != nil {
		t.Fatalf("parse(%q): %v", src, err)
	}
	_, err = Compile(q)
	if err == nil {
		t.Fatalf("expected compile error for %q, got nil", src)
	}
	return err
}

// --- top-level shape ----------------------------------------------------

func TestCompile_SelectStarExpandsToFileColumns(t *testing.T) {
	c := mustCompile(t, "SELECT *")
	if !strings.Contains(c.SQL, "files.path AS path") {
		t.Errorf("SELECT * missing path expansion: %s", c.SQL)
	}
	if !strings.Contains(c.SQL, "files.mtime AS mtime") {
		t.Errorf("SELECT * missing mtime expansion: %s", c.SQL)
	}
	if !strings.Contains(c.SQL, " FROM files") {
		t.Errorf("missing FROM files: %s", c.SQL)
	}
}

func TestCompile_DefaultFromIsFiles(t *testing.T) {
	c := mustCompile(t, "SELECT path")
	if !strings.Contains(c.SQL, "FROM files") {
		t.Errorf("default FROM not files: %s", c.SQL)
	}
}

func TestCompile_ExplicitFromFilesAccepted(t *testing.T) {
	c := mustCompile(t, "SELECT path FROM files")
	if !strings.Contains(c.SQL, "FROM files") {
		t.Errorf("explicit FROM files lost: %s", c.SQL)
	}
}

func TestCompile_FromOtherTableErrors(t *testing.T) {
	err := compileErr(t, "SELECT * FROM tags")
	var ee *Error
	if !errors.As(err, &ee) || ee.Code != "pql.eval.unsupported_from" {
		t.Errorf("got %v", err)
	}
}

func TestCompile_DistinctEmittedOncePerSelect(t *testing.T) {
	c := mustCompile(t, "SELECT DISTINCT folder")
	if !strings.HasPrefix(c.SQL, "SELECT DISTINCT ") {
		t.Errorf("DISTINCT not emitted: %s", c.SQL)
	}
}

// --- file-column projections + aliases ---------------------------------

func TestCompile_FileColumnsDirect(t *testing.T) {
	c := mustCompile(t, "SELECT path, mtime, size")
	want := []string{"files.path", "files.mtime", "files.size"}
	for _, w := range want {
		if !strings.Contains(c.SQL, w) {
			t.Errorf("missing %s in: %s", w, c.SQL)
		}
	}
}

func TestCompile_NameAndFolderUseSubstr(t *testing.T) {
	c := mustCompile(t, "SELECT name, folder")
	if !strings.Contains(c.SQL, "rtrim(substr(files.path") {
		t.Errorf("name should use rtrim(substr(...)): %s", c.SQL)
	}
	if !strings.Contains(c.SQL, "substr(files.path, 1") {
		t.Errorf("folder should use substr(files.path, 1, ...): %s", c.SQL)
	}
}

func TestCompile_AliasIsHonoured(t *testing.T) {
	c := mustCompile(t, "SELECT path AS p")
	if !strings.Contains(c.SQL, ` AS "p"`) {
		t.Errorf("alias not emitted: %s", c.SQL)
	}
}

func TestCompile_FmRefGetsAutoAlias(t *testing.T) {
	c := mustCompile(t, "SELECT fm.voting")
	// Without an alias the SQLite column name would be the long subquery;
	// we synthesise "fm.voting" so the JSON column key stays readable.
	if !strings.Contains(c.SQL, ` AS "fm.voting"`) {
		t.Errorf("fm.voting auto-alias missing: %s", c.SQL)
	}
}

// --- fm.<key> --------------------------------------------------------

func TestCompile_FmDottedAccess(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE fm.voting = TRUE")
	if !strings.Contains(c.SQL, "FROM frontmatter WHERE path = files.path AND key = ?") {
		t.Errorf("fm subquery shape wrong: %s", c.SQL)
	}
	// Param order: 'voting' for the fm key, then 1 for TRUE.
	if len(c.Params) != 2 {
		t.Fatalf("params len = %d (%v), want 2", len(c.Params), c.Params)
	}
	if c.Params[0] != "voting" {
		t.Errorf("first param = %v, want 'voting'", c.Params[0])
	}
	if c.Params[1] != int64(1) {
		t.Errorf("TRUE param = %v, want int64(1)", c.Params[1])
	}
}

func TestCompile_FmBracketAccess(t *testing.T) {
	c := mustCompile(t, "SELECT fm['key with spaces']")
	if !strings.Contains(c.SQL, "FROM frontmatter") {
		t.Errorf("bracket access didn't emit fm subquery: %s", c.SQL)
	}
	if c.Params[0] != "key with spaces" {
		t.Errorf("bracket key param = %v", c.Params[0])
	}
}

func TestCompile_FmTypeDispatchInSubquery(t *testing.T) {
	c := mustCompile(t, "SELECT fm.x")
	// Type-dispatching SELECT inside the subquery — leverages the v2 type
	// column so SQLite gets the right native type back.
	for _, want := range []string{"CASE type", "WHEN 'string' THEN value_text", "WHEN 'number' THEN value_num", "WHEN 'bool' THEN value_num"} {
		if !strings.Contains(c.SQL, want) {
			t.Errorf("missing %q in fm subquery: %s", want, c.SQL)
		}
	}
}

// --- WHERE + operators --------------------------------------------------

func TestCompile_StringEquality(t *testing.T) {
	c := mustCompile(t, "SELECT name WHERE folder = 'members'")
	if !strings.Contains(c.SQL, " = ?") {
		t.Errorf("expected '= ?': %s", c.SQL)
	}
	if c.Params[len(c.Params)-1] != "members" {
		t.Errorf("last param = %v, want 'members'", c.Params[len(c.Params)-1])
	}
}

func TestCompile_NumericComparison(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE mtime > 1000")
	if !strings.Contains(c.SQL, "files.mtime > ?") {
		t.Errorf("expected mtime > ?: %s", c.SQL)
	}
}

func TestCompile_LikeAndGlobAndRegexp(t *testing.T) {
	cases := []struct {
		src string
		op  string
	}{
		{"SELECT * WHERE path LIKE 'foo%'", "LIKE"},
		{"SELECT * WHERE path GLOB 'sessions/**/*.md'", "GLOB"},
		{"SELECT * WHERE name REGEXP '^Dr\\.'", "REGEXP"},
	}
	for _, c := range cases {
		got := mustCompile(t, c.src)
		if !strings.Contains(got.SQL, " "+c.op+" ?") {
			t.Errorf("%q → SQL missing %s: %s", c.src, c.op, got.SQL)
		}
	}
}

func TestCompile_AndOrParens(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE folder = 'a' OR folder = 'b' AND folder = 'c'")
	// We always parenthesise binary ops so precedence is explicit in SQL.
	if !strings.Contains(c.SQL, "(") || !strings.Contains(c.SQL, ")") {
		t.Errorf("expected parens around binary ops: %s", c.SQL)
	}
	// AND should be grouped tighter than OR — both end up parenthesised.
	if strings.Count(c.SQL, "(") < 3 {
		t.Errorf("expected at least three opening parens (one per binary), got: %s", c.SQL)
	}
}

func TestCompile_NotPrefix(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE NOT path = 'x'")
	if !strings.Contains(c.SQL, "(NOT ") {
		t.Errorf("NOT prefix missing: %s", c.SQL)
	}
}

func TestCompile_BetweenAndIsNull(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE mtime BETWEEN 100 AND 200")
	if !strings.Contains(c.SQL, "BETWEEN") {
		t.Errorf("BETWEEN missing: %s", c.SQL)
	}
	c = mustCompile(t, "SELECT * WHERE fm.notes IS NULL")
	if !strings.Contains(c.SQL, "IS NULL") {
		t.Errorf("IS NULL missing: %s", c.SQL)
	}
	c = mustCompile(t, "SELECT * WHERE fm.notes IS NOT NULL")
	if !strings.Contains(c.SQL, "IS NOT NULL") {
		t.Errorf("IS NOT NULL missing: %s", c.SQL)
	}
}

// --- IN tags membership -------------------------------------------------

func TestCompile_InTagsBecomesExists(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE 'council-member' IN tags")
	if !strings.Contains(c.SQL, "EXISTS (SELECT 1 FROM tags WHERE tags.path = files.path AND tags.tag = ?)") {
		t.Errorf("EXISTS shape wrong: %s", c.SQL)
	}
	if c.Params[len(c.Params)-1] != "council-member" {
		t.Errorf("last param = %v", c.Params[len(c.Params)-1])
	}
}

func TestCompile_NotInTagsBecomesNotExists(t *testing.T) {
	c := mustCompile(t, "SELECT * WHERE 'foo' NOT IN tags")
	if !strings.Contains(c.SQL, "NOT EXISTS (SELECT 1 FROM tags") {
		t.Errorf("NOT EXISTS shape wrong: %s", c.SQL)
	}
}

func TestCompile_InTupleStaysAsRegularIn(t *testing.T) {
	// fm.type IN ('a', 'b') uses the standard IN form against a literal tuple.
	c := mustCompile(t, "SELECT * WHERE fm.type IN ('a', 'b')")
	if strings.Contains(c.SQL, "EXISTS") {
		t.Errorf("tuple IN should not become EXISTS: %s", c.SQL)
	}
	if !strings.Contains(c.SQL, " IN (?, ?)") {
		t.Errorf("expected ' IN (?, ?)': %s", c.SQL)
	}
}

// --- bare array column references error clearly ------------------------

func TestCompile_BareTagsRefErrors(t *testing.T) {
	err := compileErr(t, "SELECT tags")
	var ee *Error
	if !errors.As(err, &ee) || ee.Code != "pql.eval.bare_array_ref" {
		t.Errorf("got %v", err)
	}
}

func TestCompile_BareFmRefErrors(t *testing.T) {
	err := compileErr(t, "SELECT fm")
	var ee *Error
	if !errors.As(err, &ee) || ee.Code != "pql.eval.bare_fm" {
		t.Errorf("got %v", err)
	}
}

func TestCompile_UnknownColumnErrors(t *testing.T) {
	err := compileErr(t, "SELECT typo_column")
	var ee *Error
	if !errors.As(err, &ee) || ee.Code != "pql.eval.unknown_column" {
		t.Errorf("got %v", err)
	}
}

// --- functions ----------------------------------------------------------

func TestCompile_KnownFunctions(t *testing.T) {
	for _, src := range []string{
		"SELECT length(path)",
		"SELECT upper(name)",
		"SELECT date('now', '-30 days')",
		"SELECT coalesce(fm.x, 'missing')",
	} {
		_ = mustCompile(t, src) // just verify no error
	}
}

func TestCompile_UnknownFunctionErrors(t *testing.T) {
	err := compileErr(t, "SELECT mystery(path)")
	var ee *Error
	if !errors.As(err, &ee) || ee.Code != "pql.eval.unknown_function" {
		t.Errorf("got %v", err)
	}
}

// --- ORDER BY + LIMIT ---------------------------------------------------

func TestCompile_OrderBy(t *testing.T) {
	c := mustCompile(t, "SELECT path ORDER BY mtime DESC, path ASC NULLS LAST")
	if !strings.Contains(c.SQL, "ORDER BY files.mtime DESC, files.path ASC NULLS LAST") {
		t.Errorf("ORDER BY wrong: %s", c.SQL)
	}
}

func TestCompile_LimitOffset(t *testing.T) {
	c := mustCompile(t, "SELECT path LIMIT 5 OFFSET 10")
	if !strings.Contains(c.SQL, "LIMIT ? OFFSET ?") {
		t.Errorf("LIMIT/OFFSET wrong: %s", c.SQL)
	}
	// last two params should be 5 then 10
	n := len(c.Params)
	if c.Params[n-2] != int64(5) || c.Params[n-1] != int64(10) {
		t.Errorf("limit/offset params = %v, want [5 10] at end", c.Params)
	}
}

// --- end-to-end realistic compile --------------------------------------

func TestCompile_RealisticVaultQuery(t *testing.T) {
	c := mustCompile(t, `
		SELECT name, fm.winner
		WHERE 'council-session' IN tags
		  AND fm.tied = TRUE
		ORDER BY fm.date DESC
		LIMIT 5
	`)
	// Sanity checks rather than exact-string match (whitespace is fragile):
	for _, want := range []string{
		"FROM files",
		"EXISTS (SELECT 1 FROM tags",
		"FROM frontmatter WHERE path = files.path",
		"ORDER BY",
		"LIMIT ?",
	} {
		if !strings.Contains(c.SQL, want) {
			t.Errorf("missing %q in compiled SQL:\n%s", want, c.SQL)
		}
	}
}
