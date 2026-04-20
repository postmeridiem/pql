package lex

import (
	"errors"
	"testing"
)

// kinds returns the kind sequence (no positions, no values) of toks.
// Used when only the shape matters.
func kinds(toks []Token) []Kind {
	out := make([]Kind, len(toks))
	for i, t := range toks {
		out[i] = t.Kind
	}
	return out
}

func mustAll(t *testing.T, src string) []Token {
	t.Helper()
	toks, err := All(src)
	if err != nil {
		t.Fatalf("All(%q): %v", src, err)
	}
	return toks
}

// --- happy-path cases ----------------------------------------------------

func TestAll_EmptyInput(t *testing.T) {
	toks := mustAll(t, "")
	if len(toks) != 1 || toks[0].Kind != EOF {
		t.Errorf("empty input should produce only EOF, got %v", toks)
	}
}

func TestAll_KeywordsAreCaseInsensitive(t *testing.T) {
	cases := []struct {
		src  string
		kind Kind
	}{
		{"SELECT", SELECT},
		{"select", SELECT},
		{"Select", SELECT},
		{"sElEcT", SELECT},
		{"FROM", FROM},
		{"where", WHERE},
		{"ORDER", ORDER},
		{"BY", BY},
		{"AND", AND},
		{"or", OR},
		{"not", NOT},
		{"NULL", NULLKW},
		{"TRUE", TRUE},
		{"false", FALSE},
		{"DISTINCT", DISTINCT},
	}
	for _, c := range cases {
		toks := mustAll(t, c.src)
		if len(toks) < 1 || toks[0].Kind != c.kind {
			t.Errorf("source %q → kind %s, want %s", c.src, toks[0].Kind, c.kind)
		}
	}
}

func TestAll_UnquotedIdentifier(t *testing.T) {
	toks := mustAll(t, "fm voting fm_voting f1 _x")
	wantValues := []string{"fm", "voting", "fm_voting", "f1", "_x"}
	for i, want := range wantValues {
		if toks[i].Kind != IDENT {
			t.Errorf("token %d: kind = %s, want IDENT (token=%v)", i, toks[i].Kind, toks[i])
		}
		if toks[i].Value != want {
			t.Errorf("token %d: Value = %q, want %q", i, toks[i].Value, want)
		}
	}
}

func TestAll_QuotedIdentifier(t *testing.T) {
	toks := mustAll(t, `"order" "key with spaces" "embedded ""quote"""`)
	wantValues := []string{"order", "key with spaces", `embedded "quote"`}
	for i, want := range wantValues {
		if toks[i].Kind != IDENT {
			t.Errorf("token %d: kind = %s, want IDENT", i, toks[i].Kind)
		}
		if toks[i].Value != want {
			t.Errorf("token %d: Value = %q, want %q", i, toks[i].Value, want)
		}
	}
}

func TestAll_StringLiteral(t *testing.T) {
	toks := mustAll(t, `'hello' 'it''s' '' 'multi
line'`)
	want := []string{"hello", "it's", "", "multi\nline"}
	for i, w := range want {
		if toks[i].Kind != STRING {
			t.Errorf("token %d: kind = %s, want STRING", i, toks[i].Kind)
		}
		if toks[i].Value != w {
			t.Errorf("token %d: Value = %q, want %q", i, toks[i].Value, w)
		}
	}
}

func TestAll_IntAndFloat(t *testing.T) {
	cases := []struct {
		src  string
		kind Kind
		val  string
	}{
		{"0", INT, "0"},
		{"42", INT, "42"},
		{"100000", INT, "100000"},
		{"3.14", FLOAT, "3.14"},
		{"0.5", FLOAT, "0.5"},
	}
	for _, c := range cases {
		toks := mustAll(t, c.src)
		if toks[0].Kind != c.kind || toks[0].Value != c.val {
			t.Errorf("source %q → (%s, %q), want (%s, %q)",
				c.src, toks[0].Kind, toks[0].Value, c.kind, c.val)
		}
	}
}

func TestAll_DotIsTokenWhenFollowedByLetter(t *testing.T) {
	// `fm.voting` is three tokens (IDENT, DOT, IDENT), not "fm" + "0.voting".
	// `mtime > 0.5` is INT comparison vs a float.
	toks := mustAll(t, "fm.voting")
	if len(toks) != 4 {
		t.Fatalf("expected 4 tokens (incl EOF), got %d: %v", len(toks), toks)
	}
	wantKinds := []Kind{IDENT, DOT, IDENT, EOF}
	for i, k := range wantKinds {
		if toks[i].Kind != k {
			t.Errorf("token %d: %s, want %s", i, toks[i].Kind, k)
		}
	}
}

func TestAll_TrailingDotOnNumberStops(t *testing.T) {
	// `42.` (no digit after) should be INT followed by DOT, not a malformed
	// FLOAT. This is what makes `fm.x` and number tokens unambiguous.
	toks := mustAll(t, "42.foo")
	if toks[0].Kind != INT || toks[0].Value != "42" {
		t.Errorf("token 0 = (%s, %q), want (INT, 42)", toks[0].Kind, toks[0].Value)
	}
	if toks[1].Kind != DOT {
		t.Errorf("token 1 = %s, want DOT", toks[1].Kind)
	}
	if toks[2].Kind != IDENT || toks[2].Value != "foo" {
		t.Errorf("token 2 = (%s, %q), want (IDENT, foo)", toks[2].Kind, toks[2].Value)
	}
}

func TestAll_OperatorsAndPunctuation(t *testing.T) {
	src := `= != <> < <= > >= + - * / % || ( ) [ ] , .`
	want := []Kind{
		EQ, NEQ, NEQ, LT, LTE, GT, GTE,
		PLUS, MINUS, STAR, SLASH, PERCENT, CONCAT,
		LPAREN, RPAREN, LBRACKET, RBRACKET, COMMA, DOT,
		EOF,
	}
	got := kinds(mustAll(t, src))
	if len(got) != len(want) {
		t.Fatalf("got %d kinds, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

func TestAll_LineComment(t *testing.T) {
	toks := mustAll(t, "SELECT -- comment to EOL\nname")
	if !equalKinds(kinds(toks), []Kind{SELECT, IDENT, EOF}) {
		t.Errorf("got %v", kinds(toks))
	}
	if toks[1].Value != "name" {
		t.Errorf("identifier value = %q, want name", toks[1].Value)
	}
}

func TestAll_BlockComment(t *testing.T) {
	toks := mustAll(t, "SELECT /* one\n  two\n  three */ name")
	if !equalKinds(kinds(toks), []Kind{SELECT, IDENT, EOF}) {
		t.Errorf("got %v", kinds(toks))
	}
	if toks[1].Line != 3 {
		t.Errorf("name should be on line 3 (after multi-line comment), got line %d", toks[1].Line)
	}
}

func TestAll_FullSelect(t *testing.T) {
	src := "SELECT name, fm.voting FROM files WHERE folder = 'members' ORDER BY name LIMIT 10"
	want := []Kind{
		SELECT, IDENT, COMMA, IDENT, DOT, IDENT,
		FROM, IDENT,
		WHERE, IDENT, EQ, STRING,
		ORDER, BY, IDENT,
		LIMIT, INT,
		EOF,
	}
	got := kinds(mustAll(t, src))
	if !equalKinds(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

// --- positions -----------------------------------------------------------

func TestAll_PositionsTrackedAcrossLines(t *testing.T) {
	src := "SELECT\n  name\nFROM"
	toks := mustAll(t, src)
	cases := []struct {
		idx       int
		wantLine  int
		wantCol   int
		wantKind  Kind
		wantValue string
	}{
		{0, 1, 1, SELECT, "SELECT"},
		{1, 2, 3, IDENT, "name"},
		{2, 3, 1, FROM, "FROM"},
	}
	for _, c := range cases {
		got := toks[c.idx]
		if got.Kind != c.wantKind || got.Line != c.wantLine || got.Col != c.wantCol {
			t.Errorf("token %d: got (%s, line=%d, col=%d), want (%s, line=%d, col=%d)",
				c.idx, got.Kind, got.Line, got.Col, c.wantKind, c.wantLine, c.wantCol)
		}
	}
}

// --- error cases ---------------------------------------------------------

func TestAll_UnterminatedString(t *testing.T) {
	_, err := All("'hello")
	var le *Error
	if !errors.As(err, &le) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if le.Code != "pql.lex.unterminated_string" {
		t.Errorf("code = %q, want pql.lex.unterminated_string", le.Code)
	}
	if le.Line != 1 || le.Col != 1 {
		t.Errorf("position = (line %d, col %d), want (1, 1)", le.Line, le.Col)
	}
}

func TestAll_UnterminatedQuotedIdent(t *testing.T) {
	_, err := All(`"never_closed`)
	var le *Error
	if !errors.As(err, &le) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if le.Code != "pql.lex.unterminated_quoted_ident" {
		t.Errorf("code = %q", le.Code)
	}
}

func TestAll_UnterminatedBlockComment(t *testing.T) {
	_, err := All("SELECT /* never closes")
	var le *Error
	if !errors.As(err, &le) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if le.Code != "pql.lex.unterminated_block_comment" {
		t.Errorf("code = %q", le.Code)
	}
}

func TestAll_UnexpectedCharacter(t *testing.T) {
	_, err := All("SELECT @")
	var le *Error
	if !errors.As(err, &le) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if le.Code != "pql.lex.unexpected_char" {
		t.Errorf("code = %q", le.Code)
	}
	// '@' is at col 8 (1-based), line 1.
	if le.Line != 1 || le.Col != 8 {
		t.Errorf("position = (line %d, col %d), want (1, 8)", le.Line, le.Col)
	}
}

// --- helpers --------------------------------------------------------------

func equalKinds(a, b []Kind) bool {
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
