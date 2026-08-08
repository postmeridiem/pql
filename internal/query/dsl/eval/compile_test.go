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

func TestQuoteIdent_DoublesEmbeddedQuote(t *testing.T) {
	if got := quoteIdent(`a"b`); got != `"a""b"` {
		t.Errorf("quoteIdent(`a\"b`) = %q, want %q", got, `"a""b"`)
	}
	if got := quoteIdent("plain"); got != `"plain"` {
		t.Errorf("quoteIdent(plain) = %q, want %q", got, `"plain"`)
	}
}

// TestCompile_AliasQuoteCannotBreakOut guards against SQL injection through
// a column alias: the lexer un-escapes `""`→`"`, so a user alias can carry a
// literal quote. The compiler must double it back, keeping everything inside
// one identifier rather than letting an injected projection execute.
func TestCompile_AliasQuoteCannotBreakOut(t *testing.T) {
	c := mustCompile(t, `SELECT path AS "q"" , (SELECT 1) AS leak --"`)
	// A breakout would look like `AS "q" , (SELECT 1) AS leak` — the quote
	// closing the identifier early. After escaping it is `AS "q"" , …"`.
	if strings.Contains(c.SQL, `AS "q" ,`) {
		t.Fatalf("alias quote not escaped — injection breakout in SQL: %s", c.SQL)
	}
	if !strings.Contains(c.SQL, `""`) {
		t.Fatalf("expected the embedded quote to be doubled: %s", c.SQL)
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
	// name strips a literal ".md" suffix, not a character set. rtrim's
	// second argument is a set, which ate into the stem — this assertion
	// used to require the rtrim and so held the bug in place (T-94).
	if strings.Contains(c.SQL, "rtrim(") {
		t.Errorf("name must not use rtrim — its second arg is a character set: %s", c.SQL)
	}
	if !strings.Contains(c.SQL, "LIKE '%.md'") {
		t.Errorf("name should guard on a literal .md suffix: %s", c.SQL)
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
	for _, want := range []string{"CASE type", "WHEN 'string' THEN value_text", "WHEN 'number' THEN value_num"} {
		if !strings.Contains(c.SQL, want) {
			t.Errorf("missing %q in fm subquery: %s", want, c.SQL)
		}
	}
	// The bool arm is context-dependent — see
	// TestBoolProjection_DiffersFromComparison. This test used to require
	// value_num here, in a SELECT, which is the shape that rendered true as
	// 1 (T-93).
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

// The unknown-column error is the only route a consuming agent has to the
// DSL vocabulary — the grammar doc is not shipped with the binary. So the
// list it prints has to be the list fileColumn actually accepts, or the
// error becomes a confident lie (T-86).
func TestFileColumns_MatchesErrorList(t *testing.T) {
	for _, name := range fileColumns {
		if _, ok := fileColumn(name); !ok {
			t.Errorf("error message advertises %q but fileColumn rejects it", name)
		}
	}

	// The other direction: a column added to fileColumn without a line in
	// fileColumns stays invisible to every caller who guessed wrong. There
	// is no way to enumerate a switch, so probe the identifiers a reasonable
	// contributor might add and assert each is either rejected or listed.
	candidates := []string{
		"path", "name", "folder", "size", "mtime", "ctime", "content_hash",
		"last_scanned", "ext", "depth", "basename", "dir", "modified",
		"created", "bytes", "title", "id", "hash", "scanned",
	}
	listed := make(map[string]bool, len(fileColumns))
	for _, n := range fileColumns {
		listed[n] = true
	}
	for _, n := range candidates {
		if _, ok := fileColumn(n); ok && !listed[n] {
			t.Errorf("fileColumn accepts %q but the error message does not list it", n)
		}
	}
}

// Naming an unknown column must hand back the vocabulary, not just repeat
// the mistake back.
func TestUnknownColumn_ErrorListsTheVocabulary(t *testing.T) {
	q, err := parse.Parse("SELECT bogus_column")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = Compile(q)
	if err == nil {
		t.Fatal("expected an error for an unknown column")
	}
	msg := err.Error()
	for _, want := range []string{"bogus_column", "path", "folder", "headings", "fm.<key>"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q, got: %s", want, msg)
		}
	}
}

// name derived the stem with rtrim(path, '.md'), and SQLite's two-argument
// rtrim strips trailing characters that are MEMBERS of its second argument
// rather than a literal suffix. So it ate into the stem: album.md became
// "albu", second.md became "secon". README.md and types.md were unaffected,
// which is why it survived — every example anyone tried ended in a letter
// outside the set (T-94).
func TestName_StripsOnlyTheExtension(t *testing.T) {
	cases := map[string]string{
		"album.md":           "album",   // trailing m
		"second.md":          "second",  // trailing d
		"diagram.md":         "diagram", // trailing m
		"README.md":          "README",  // unaffected, the case that hid it
		"deep/er/keyword.md": "keyword", // nested, trailing d
		"notes/m.md":         "m",       // single-char stem in the set
	}
	for path, want := range cases {
		got := stripSuffixInGo(path)
		if got != want {
			t.Errorf("%s -> %q, want %q", path, got, want)
		}
	}
}

// stripSuffixInGo mirrors what the SQL in stripMDSuffix(sqlBasename) must
// compute, so the expectations above are readable without a database. The
// SQL itself is exercised end-to-end by the integration suite.
func stripSuffixInGo(path string) string {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	return strings.TrimSuffix(base, ".md")
}

// The projected shape of a frontmatter value differs from the comparable
// shape for bools only, and only in the SELECT list. Guard both halves:
// projection casts to BLOB so normalise can tell JSON from text, comparison
// stays numeric so `= true` and `= 1` both match (T-93).
func TestBoolProjection_DiffersFromComparison(t *testing.T) {
	sel := mustCompile(t, "SELECT fm.voting")
	if !strings.Contains(sel.SQL, "CAST(value_json AS BLOB)") {
		t.Errorf("SELECT should project a bool as a JSON blob:\n%s", sel.SQL)
	}

	where := mustCompile(t, "SELECT path WHERE fm.voting = true")
	if strings.Contains(where.SQL, "CAST(value_json AS BLOB)") {
		t.Errorf("WHERE must keep the comparable numeric shape:\n%s", where.SQL)
	}

	// ORDER BY sorts, so it compares — same rule as WHERE.
	order := mustCompile(t, "SELECT path ORDER BY fm.voting")
	if strings.Contains(order.SQL, "CAST(value_json AS BLOB)") {
		t.Errorf("ORDER BY must keep the comparable numeric shape:\n%s", order.SQL)
	}
}
