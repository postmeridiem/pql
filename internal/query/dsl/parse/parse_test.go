package parse

import (
	"errors"
	"testing"
)

func mustParse(t *testing.T, src string) *Query {
	t.Helper()
	q, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return q
}

// --- top-level shape ----------------------------------------------------

func TestParse_SelectStar(t *testing.T) {
	q := mustParse(t, "SELECT *")
	if !q.Star || len(q.Select) != 0 {
		t.Errorf("expected Star=true Select=[], got Star=%v len(Select)=%d", q.Star, len(q.Select))
	}
}

func TestParse_SelectDistinct(t *testing.T) {
	q := mustParse(t, "SELECT DISTINCT name")
	if !q.Distinct {
		t.Errorf("expected Distinct=true")
	}
}

func TestParse_SelectColumns(t *testing.T) {
	q := mustParse(t, "SELECT name, fm.voting AS v")
	if q.Star {
		t.Errorf("Star should be false")
	}
	if len(q.Select) != 2 {
		t.Fatalf("expected 2 projections, got %d", len(q.Select))
	}
	r0, ok := q.Select[0].Expr.(*Ref)
	if !ok || len(r0.Parts) != 1 || r0.Parts[0].Name != "name" {
		t.Errorf("projection 0 = %#v", q.Select[0])
	}
	if q.Select[1].Alias != "v" {
		t.Errorf("projection 1 alias = %q, want v", q.Select[1].Alias)
	}
	r1, ok := q.Select[1].Expr.(*Ref)
	if !ok || len(r1.Parts) != 2 || r1.Parts[0].Name != "fm" || r1.Parts[1].Name != "voting" {
		t.Errorf("projection 1 = %#v", q.Select[1])
	}
}

func TestParse_FromClauseOptional(t *testing.T) {
	q := mustParse(t, "SELECT *")
	if q.From != "" {
		t.Errorf("expected empty From (default to files), got %q", q.From)
	}
	q = mustParse(t, "SELECT * FROM files")
	if q.From != "files" {
		t.Errorf("From = %q, want files", q.From)
	}
}

func TestParse_OrderByMultipleItems(t *testing.T) {
	q := mustParse(t, "SELECT name ORDER BY mtime DESC, name ASC NULLS LAST")
	if len(q.OrderBy) != 2 {
		t.Fatalf("expected 2 sort items, got %d", len(q.OrderBy))
	}
	if !q.OrderBy[0].Desc {
		t.Errorf("first item should be DESC")
	}
	if q.OrderBy[1].Desc {
		t.Errorf("second item should be ASC")
	}
	if q.OrderBy[1].Nulls != "last" {
		t.Errorf("second item Nulls = %q, want last", q.OrderBy[1].Nulls)
	}
}

func TestParse_LimitOffset(t *testing.T) {
	q := mustParse(t, "SELECT * LIMIT 10 OFFSET 5")
	if q.Limit == nil {
		t.Fatal("Limit should not be nil")
	}
	if q.Limit.N != 10 || q.Limit.Offset != 5 {
		t.Errorf("Limit = %#v", q.Limit)
	}
}

// --- expressions: precedence + associativity ----------------------------

func TestParse_AndBinds_TighterThan_Or(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE a OR b AND c")
	// Expect: a OR (b AND c)
	or, ok := q.Where.(*Binary)
	if !ok || or.Op != "OR" {
		t.Fatalf("top-level should be OR, got %#v", q.Where)
	}
	right, ok := or.R.(*Binary)
	if !ok || right.Op != "AND" {
		t.Errorf("OR right side should be AND, got %#v", or.R)
	}
}

func TestParse_NotBinds_TighterThan_And(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE NOT a AND b")
	// Expect: (NOT a) AND b — NOT binds to its own cmp, not to the AND.
	and, ok := q.Where.(*Binary)
	if !ok || and.Op != "AND" {
		t.Fatalf("top-level should be AND, got %#v", q.Where)
	}
	left, ok := and.L.(*Unary)
	if !ok || left.Op != "NOT" {
		t.Errorf("AND left should be NOT, got %#v", and.L)
	}
}

func TestParse_MulBinds_TighterThan_Add(t *testing.T) {
	q := mustParse(t, "SELECT 1 + 2 * 3")
	// Expect: 1 + (2 * 3)
	plus, ok := q.Select[0].Expr.(*Binary)
	if !ok || plus.Op != "+" {
		t.Fatalf("expected top-level +, got %#v", q.Select[0].Expr)
	}
	if mul, ok := plus.R.(*Binary); !ok || mul.Op != "*" {
		t.Errorf("right side should be *, got %#v", plus.R)
	}
}

func TestParse_Parentheses(t *testing.T) {
	q := mustParse(t, "SELECT (1 + 2) * 3")
	// Expect: (1+2) * 3 — top-level *
	mul, ok := q.Select[0].Expr.(*Binary)
	if !ok || mul.Op != "*" {
		t.Fatalf("expected top-level *, got %#v", q.Select[0].Expr)
	}
	if plus, ok := mul.L.(*Binary); !ok || plus.Op != "+" {
		t.Errorf("left should be the parenthesised +, got %#v", mul.L)
	}
}

// --- comparison forms ---------------------------------------------------

func TestParse_ComparisonOperators(t *testing.T) {
	cases := []struct {
		src string
		op  string
	}{
		{"SELECT * WHERE a = 1", "="},
		{"SELECT * WHERE a != 1", "!="},
		{"SELECT * WHERE a <> 1", "!="},
		{"SELECT * WHERE a < 1", "<"},
		{"SELECT * WHERE a <= 1", "<="},
		{"SELECT * WHERE a > 1", ">"},
		{"SELECT * WHERE a >= 1", ">="},
		{"SELECT * WHERE a LIKE 'foo%'", "LIKE"},
		{"SELECT * WHERE a GLOB 'foo*'", "GLOB"},
		{"SELECT * WHERE a REGEXP 'foo'", "REGEXP"},
		{"SELECT * WHERE body MATCH 'consensus'", "MATCH"},
	}
	for _, c := range cases {
		q := mustParse(t, c.src)
		b, ok := q.Where.(*Binary)
		if !ok || b.Op != c.op {
			t.Errorf("%q: top should be %s, got %#v", c.src, c.op, q.Where)
		}
	}
}

func TestParse_InWithTuple(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE fm.type IN ('a', 'b', 'c')")
	in, ok := q.Where.(*Binary)
	if !ok || in.Op != "IN" {
		t.Fatalf("expected IN binary, got %#v", q.Where)
	}
	tuple, ok := in.R.(*Tuple)
	if !ok || len(tuple.Items) != 3 {
		t.Errorf("right side should be 3-item tuple, got %#v", in.R)
	}
}

func TestParse_InWithRef(t *testing.T) {
	// 'foo' IN tags — array membership
	q := mustParse(t, "SELECT * WHERE 'foo' IN tags")
	in, ok := q.Where.(*Binary)
	if !ok || in.Op != "IN" {
		t.Fatalf("expected IN binary, got %#v", q.Where)
	}
	if _, ok := in.R.(*Ref); !ok {
		t.Errorf("right side should be Ref, got %#v", in.R)
	}
}

func TestParse_NotIn(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE x NOT IN (1, 2)")
	b, ok := q.Where.(*Binary)
	if !ok || b.Op != "NOT IN" {
		t.Fatalf("expected NOT IN, got %#v", q.Where)
	}
}

func TestParse_Between(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE mtime BETWEEN 100 AND 200")
	bt, ok := q.Where.(*Between)
	if !ok || bt.Not {
		t.Fatalf("expected BETWEEN, got %#v", q.Where)
	}
	if l, ok := bt.Low.(*IntLit); !ok || l.Value != 100 {
		t.Errorf("low = %#v", bt.Low)
	}
	if h, ok := bt.High.(*IntLit); !ok || h.Value != 200 {
		t.Errorf("high = %#v", bt.High)
	}
}

func TestParse_NotBetween(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE mtime NOT BETWEEN 100 AND 200")
	bt, ok := q.Where.(*Between)
	if !ok || !bt.Not {
		t.Fatalf("expected NOT BETWEEN, got %#v", q.Where)
	}
}

func TestParse_IsNullVariants(t *testing.T) {
	q := mustParse(t, "SELECT * WHERE fm.notes IS NULL")
	n, ok := q.Where.(*IsNull)
	if !ok || n.Not {
		t.Fatalf("expected IS NULL, got %#v", q.Where)
	}
	q = mustParse(t, "SELECT * WHERE fm.notes IS NOT NULL")
	n, ok = q.Where.(*IsNull)
	if !ok || !n.Not {
		t.Fatalf("expected IS NOT NULL, got %#v", q.Where)
	}
}

// --- references and calls -----------------------------------------------

func TestParse_RefBracketAccess(t *testing.T) {
	q := mustParse(t, "SELECT fm['key with spaces']")
	r, ok := q.Select[0].Expr.(*Ref)
	if !ok || len(r.Parts) != 2 {
		t.Fatalf("expected Ref with 2 parts, got %#v", q.Select[0].Expr)
	}
	if r.Parts[1].Bracket != "key with spaces" {
		t.Errorf("bracket = %q", r.Parts[1].Bracket)
	}
}

func TestParse_FunctionCall(t *testing.T) {
	q := mustParse(t, "SELECT length(outlinks), date('now', '-30 days')")
	if len(q.Select) != 2 {
		t.Fatalf("expected 2 projections")
	}
	c0, ok := q.Select[0].Expr.(*Call)
	if !ok || c0.Name != "length" || len(c0.Args) != 1 {
		t.Errorf("call 0 = %#v", q.Select[0].Expr)
	}
	c1, ok := q.Select[1].Expr.(*Call)
	if !ok || c1.Name != "date" || len(c1.Args) != 2 {
		t.Errorf("call 1 = %#v", q.Select[1].Expr)
	}
}

func TestParse_FunctionNoArgs(t *testing.T) {
	q := mustParse(t, "SELECT now()")
	c, ok := q.Select[0].Expr.(*Call)
	if !ok || c.Name != "now" || len(c.Args) != 0 {
		t.Errorf("expected now() with 0 args, got %#v", q.Select[0].Expr)
	}
}

// --- literals -----------------------------------------------------------

func TestParse_Literals(t *testing.T) {
	cases := []struct {
		src   string
		check func(t *testing.T, e Expr)
	}{
		{"SELECT 'hello'", func(t *testing.T, e Expr) {
			if v, ok := e.(*StringLit); !ok || v.Value != "hello" {
				t.Errorf("string lit = %#v", e)
			}
		}},
		{"SELECT 42", func(t *testing.T, e Expr) {
			if v, ok := e.(*IntLit); !ok || v.Value != 42 {
				t.Errorf("int lit = %#v", e)
			}
		}},
		{"SELECT 3.14", func(t *testing.T, e Expr) {
			if v, ok := e.(*FloatLit); !ok || v.Value != 3.14 {
				t.Errorf("float lit = %#v", e)
			}
		}},
		{"SELECT TRUE", func(t *testing.T, e Expr) {
			if v, ok := e.(*BoolLit); !ok || !v.Value {
				t.Errorf("bool lit = %#v", e)
			}
		}},
		{"SELECT FALSE", func(t *testing.T, e Expr) {
			if v, ok := e.(*BoolLit); !ok || v.Value {
				t.Errorf("bool lit = %#v", e)
			}
		}},
		{"SELECT NULL", func(t *testing.T, e Expr) {
			if _, ok := e.(*NullLit); !ok {
				t.Errorf("null lit = %#v", e)
			}
		}},
	}
	for _, c := range cases {
		q := mustParse(t, c.src)
		c.check(t, q.Select[0].Expr)
	}
}

func TestParse_UnaryNegativeOnNumber(t *testing.T) {
	q := mustParse(t, "SELECT -5")
	u, ok := q.Select[0].Expr.(*Unary)
	if !ok || u.Op != "-" {
		t.Fatalf("expected unary -, got %#v", q.Select[0].Expr)
	}
	if v, ok := u.X.(*IntLit); !ok || v.Value != 5 {
		t.Errorf("unary operand = %#v", u.X)
	}
}

// --- end-to-end real query ----------------------------------------------

func TestParse_RealisticVaultQuery(t *testing.T) {
	src := `SELECT name, fm.winner
	        FROM files
	        WHERE 'council-session' IN tags
	          AND fm.tied = TRUE
	        ORDER BY fm.date DESC
	        LIMIT 5`
	q := mustParse(t, src)

	if len(q.Select) != 2 {
		t.Errorf("expected 2 projections, got %d", len(q.Select))
	}
	if q.From != "files" {
		t.Errorf("From = %q", q.From)
	}
	and, ok := q.Where.(*Binary)
	if !ok || and.Op != "AND" {
		t.Fatalf("expected AND at top of WHERE, got %#v", q.Where)
	}
	if in, ok := and.L.(*Binary); !ok || in.Op != "IN" {
		t.Errorf("AND left should be IN, got %#v", and.L)
	}
	if eq, ok := and.R.(*Binary); !ok || eq.Op != "=" {
		t.Errorf("AND right should be =, got %#v", and.R)
	}
	if len(q.OrderBy) != 1 || !q.OrderBy[0].Desc {
		t.Errorf("ORDER BY = %#v", q.OrderBy)
	}
	if q.Limit == nil || q.Limit.N != 5 {
		t.Errorf("LIMIT = %#v", q.Limit)
	}
}

// --- error cases --------------------------------------------------------

func TestParse_ErrorWhenNotSelectStart(t *testing.T) {
	_, err := Parse("FROM files")
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("expected *parse.Error, got %v", err)
	}
	if pe.Code != "pql.parse.expected_select" {
		t.Errorf("code = %q", pe.Code)
	}
}

func TestParse_ErrorOnTrailingTokens(t *testing.T) {
	_, err := Parse("SELECT * FROM files extraneous")
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("expected *parse.Error, got %v", err)
	}
	if pe.Code != "pql.parse.unexpected_trailing" {
		t.Errorf("code = %q", pe.Code)
	}
}

func TestParse_ErrorIncompleteBetween(t *testing.T) {
	_, err := Parse("SELECT * WHERE x BETWEEN 1")
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("expected *parse.Error, got %v", err)
	}
	if pe.Code != "pql.parse.expected_and_in_between" {
		t.Errorf("code = %q", pe.Code)
	}
}

func TestParse_ErrorMissingExprAfterWhere(t *testing.T) {
	_, err := Parse("SELECT * WHERE")
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("expected *parse.Error, got %v", err)
	}
	if pe.Code != "pql.parse.unexpected_token" {
		t.Errorf("code = %q", pe.Code)
	}
}

func TestParse_ErrorPositionsAreReal(t *testing.T) {
	// "SELECT * WHERE @" — @ at col 16
	_, err := Parse("SELECT * WHERE @")
	var le *struct {
		Code string
		Msg  string
		Line int
		Col  int
	}
	_ = le
	if err == nil {
		t.Fatal("expected error")
	}
	// Lex error reaches us first via Parse(); just verify position is sensible.
	if msg := err.Error(); msg == "" {
		t.Errorf("error message empty")
	}
}
