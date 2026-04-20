package parse

import (
	"fmt"
	"strconv"

	"github.com/postmeridiem/pql/internal/query/dsl/lex"
)

// Error is a positioned parse failure. Codes follow the pql.parse.<kind>
// convention from docs/pql-grammar.md.
type Error struct {
	Code string
	Msg  string
	Line int
	Col  int
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s at line %d, col %d: %s", e.Code, e.Line, e.Col, e.Msg)
}

// Parse converts source text into an AST. Lex errors propagate verbatim;
// parse errors carry pql.parse.<kind> codes.
func Parse(src string) (*Query, error) {
	tokens, err := lex.All(src)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}
	q, err := p.parseQuery()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != lex.EOF {
		t := p.peek()
		return nil, p.errAt(t, "unexpected_trailing", "unexpected trailing %s; query already ended", describe(t))
	}
	return q, nil
}

// parser walks a slice of tokens by index. Single-pass, no backtracking
// (the grammar is LL(1) modulo the parenthesised-expression-vs-tuple
// disambiguation which is one-token lookahead).
type parser struct {
	tokens []lex.Token
	pos    int
}

func (p *parser) peek() lex.Token              { return p.tokens[p.pos] }
func (p *parser) peekAt(offset int) lex.Token  { return p.tokens[p.pos+offset] }
func (p *parser) advance() lex.Token {
	t := p.tokens[p.pos]
	if t.Kind != lex.EOF {
		p.pos++
	}
	return t
}

func (p *parser) accept(k lex.Kind) bool {
	if p.peek().Kind == k {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expect(k lex.Kind, code string) (lex.Token, error) {
	t := p.peek()
	if t.Kind != k {
		return lex.Token{}, p.errAt(t, code, "expected %s, got %s", k, describe(t))
	}
	p.advance()
	return t, nil
}

func (p *parser) errAt(t lex.Token, code, format string, args ...any) *Error {
	return &Error{
		Code: "pql.parse." + code,
		Msg:  fmt.Sprintf(format, args...),
		Line: t.Line,
		Col:  t.Col,
	}
}

func describe(t lex.Token) string {
	switch t.Kind {
	case lex.IDENT, lex.INT, lex.FLOAT, lex.STRING:
		return fmt.Sprintf("%s %q", t.Kind, t.Value)
	case lex.EOF:
		return "end of input"
	}
	return t.Kind.String()
}

// --- query --------------------------------------------------------------

func (p *parser) parseQuery() (*Query, error) {
	startTok, err := p.expect(lex.SELECT, "expected_select")
	if err != nil {
		return nil, err
	}
	q := &Query{Pos: Pos{Line: startTok.Line, Col: startTok.Col}}

	if p.accept(lex.DISTINCT) {
		q.Distinct = true
	}

	if p.peek().Kind == lex.STAR {
		p.advance()
		q.Star = true
	} else {
		projs, err := p.parseProjList()
		if err != nil {
			return nil, err
		}
		q.Select = projs
	}

	if p.accept(lex.FROM) {
		t, err := p.expect(lex.IDENT, "expected_from_table")
		if err != nil {
			return nil, err
		}
		q.From = t.Value
	}

	if p.accept(lex.WHERE) {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		q.Where = expr
	}

	if p.accept(lex.ORDER) {
		if _, err := p.expect(lex.BY, "expected_by_after_order"); err != nil {
			return nil, err
		}
		items, err := p.parseSortList()
		if err != nil {
			return nil, err
		}
		q.OrderBy = items
	}

	if p.accept(lex.LIMIT) {
		lim, err := p.parseLimit()
		if err != nil {
			return nil, err
		}
		q.Limit = lim
	}

	return q, nil
}

func (p *parser) parseProjList() ([]Projection, error) {
	var out []Projection
	for {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		proj := Projection{Expr: expr}
		if p.accept(lex.AS) {
			t, err := p.expect(lex.IDENT, "expected_alias")
			if err != nil {
				return nil, err
			}
			proj.Alias = t.Value
		}
		out = append(out, proj)
		if !p.accept(lex.COMMA) {
			break
		}
	}
	return out, nil
}

func (p *parser) parseSortList() ([]SortItem, error) {
	var out []SortItem
	for {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		item := SortItem{Expr: expr}
		switch {
		case p.accept(lex.ASC):
			// default; explicit
		case p.accept(lex.DESC):
			item.Desc = true
		}
		if p.accept(lex.NULLS) {
			switch {
			case p.accept(lex.FIRST):
				item.Nulls = "first"
			case p.accept(lex.LAST):
				item.Nulls = "last"
			default:
				return nil, p.errAt(p.peek(), "expected_nulls_first_or_last",
					"expected FIRST or LAST after NULLS, got %s", describe(p.peek()))
			}
		}
		out = append(out, item)
		if !p.accept(lex.COMMA) {
			break
		}
	}
	return out, nil
}

func (p *parser) parseLimit() (*Limit, error) {
	t, err := p.expect(lex.INT, "expected_limit_int")
	if err != nil {
		return nil, err
	}
	n, _ := strconv.ParseInt(t.Value, 10, 64)
	lim := &Limit{N: n}
	if p.accept(lex.OFFSET) {
		t, err := p.expect(lex.INT, "expected_offset_int")
		if err != nil {
			return nil, err
		}
		off, _ := strconv.ParseInt(t.Value, 10, 64)
		lim.Offset = off
	}
	return lim, nil
}

// --- expressions: precedence cascade -------------------------------------

func (p *parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == lex.OR {
		t := p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: "OR", L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == lex.AND {
		t := p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: "AND", L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Expr, error) {
	if p.peek().Kind == lex.NOT {
		t := p.advance()
		x, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		return &Unary{P: Pos{Line: t.Line, Col: t.Col}, Op: "NOT", X: x}, nil
	}
	return p.parseCmp()
}

func (p *parser) parseCmp() (Expr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}

	switch p.peek().Kind {
	case lex.EQ, lex.NEQ, lex.LT, lex.LTE, lex.GT, lex.GTE,
		lex.LIKE, lex.GLOB, lex.REGEXP, lex.MATCH:
		op := opName(p.peek())
		t := p.advance()
		right, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		return &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: op, L: left, R: right}, nil

	case lex.IN:
		t := p.advance()
		right, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		return &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: "IN", L: left, R: right}, nil

	case lex.NOT:
		// Could be NOT IN, NOT BETWEEN, NOT LIKE, etc. Look ahead.
		if next := p.peekAt(1); next.Kind == lex.IN {
			t := p.advance() // NOT
			p.advance()      // IN
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			return &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: "NOT IN", L: left, R: right}, nil
		}
		if next := p.peekAt(1); next.Kind == lex.BETWEEN {
			t := p.advance() // NOT
			p.advance()      // BETWEEN
			low, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lex.AND, "expected_and_in_between"); err != nil {
				return nil, err
			}
			high, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			return &Between{P: Pos{Line: t.Line, Col: t.Col}, X: left, Low: low, High: high, Not: true}, nil
		}
		// Otherwise, NOT here is unexpected (would be a logical NOT preceding
		// nothing meaningful at this position).
		return left, nil

	case lex.BETWEEN:
		t := p.advance()
		low, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lex.AND, "expected_and_in_between"); err != nil {
			return nil, err
		}
		high, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		return &Between{P: Pos{Line: t.Line, Col: t.Col}, X: left, Low: low, High: high}, nil

	case lex.IS:
		t := p.advance()
		not := p.accept(lex.NOT)
		if _, err := p.expect(lex.NULLKW, "expected_null_after_is"); err != nil {
			return nil, err
		}
		return &IsNull{P: Pos{Line: t.Line, Col: t.Col}, X: left, Not: not}, nil
	}

	return left, nil
}

func (p *parser) parseAdd() (Expr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for {
		k := p.peek().Kind
		if k != lex.PLUS && k != lex.MINUS && k != lex.CONCAT {
			return left, nil
		}
		t := p.advance()
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: opName(t), L: left, R: right}
	}
}

func (p *parser) parseMul() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		k := p.peek().Kind
		if k != lex.STAR && k != lex.SLASH && k != lex.PERCENT {
			return left, nil
		}
		t := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &Binary{P: Pos{Line: t.Line, Col: t.Col}, Op: opName(t), L: left, R: right}
	}
}

func (p *parser) parseUnary() (Expr, error) {
	switch p.peek().Kind {
	case lex.MINUS:
		t := p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Unary{P: Pos{Line: t.Line, Col: t.Col}, Op: "-", X: x}, nil
	case lex.PLUS:
		t := p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Unary{P: Pos{Line: t.Line, Col: t.Col}, Op: "+", X: x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.peek()
	switch t.Kind {
	case lex.STRING:
		p.advance()
		return &StringLit{P: Pos{t.Line, t.Col}, Value: t.Value}, nil
	case lex.INT:
		p.advance()
		n, err := strconv.ParseInt(t.Value, 10, 64)
		if err != nil {
			return nil, p.errAt(t, "invalid_int", "invalid integer literal %q: %v", t.Value, err)
		}
		return &IntLit{P: Pos{t.Line, t.Col}, Value: n}, nil
	case lex.FLOAT:
		p.advance()
		f, err := strconv.ParseFloat(t.Value, 64)
		if err != nil {
			return nil, p.errAt(t, "invalid_float", "invalid float literal %q: %v", t.Value, err)
		}
		return &FloatLit{P: Pos{t.Line, t.Col}, Value: f}, nil
	case lex.TRUE:
		p.advance()
		return &BoolLit{P: Pos{t.Line, t.Col}, Value: true}, nil
	case lex.FALSE:
		p.advance()
		return &BoolLit{P: Pos{t.Line, t.Col}, Value: false}, nil
	case lex.NULLKW:
		p.advance()
		return &NullLit{P: Pos{t.Line, t.Col}}, nil
	case lex.LPAREN:
		return p.parseParenOrTuple()
	case lex.IDENT:
		return p.parseRefOrCall()
	}
	return nil, p.errAt(t, "unexpected_token", "unexpected %s; expected expression", describe(t))
}

func (p *parser) parseParenOrTuple() (Expr, error) {
	open := p.advance() // (
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.accept(lex.RPAREN) {
		return first, nil
	}
	if !p.accept(lex.COMMA) {
		return nil, p.errAt(p.peek(), "expected_comma_or_rparen",
			"expected , or ) after expression, got %s", describe(p.peek()))
	}
	items := []Expr{first}
	for {
		next, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		items = append(items, next)
		if !p.accept(lex.COMMA) {
			break
		}
	}
	if _, err := p.expect(lex.RPAREN, "expected_rparen_in_tuple"); err != nil {
		return nil, err
	}
	return &Tuple{P: Pos{Line: open.Line, Col: open.Col}, Items: items}, nil
}

func (p *parser) parseRefOrCall() (Expr, error) {
	first := p.advance() // IDENT
	pos := Pos{Line: first.Line, Col: first.Col}

	// Function call: identifier immediately followed by ( with no DOT chain
	// in between.
	if p.peek().Kind == lex.LPAREN {
		p.advance() // (
		var args []Expr
		if p.peek().Kind != lex.RPAREN {
			for {
				a, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if !p.accept(lex.COMMA) {
					break
				}
			}
		}
		if _, err := p.expect(lex.RPAREN, "expected_rparen_in_call"); err != nil {
			return nil, err
		}
		return &Call{P: pos, Name: first.Value, Args: args}, nil
	}

	parts := []RefPart{{Name: first.Value}}
	for {
		switch p.peek().Kind {
		case lex.DOT:
			p.advance()
			t, err := p.expect(lex.IDENT, "expected_ident_after_dot")
			if err != nil {
				return nil, err
			}
			parts = append(parts, RefPart{Name: t.Value})
		case lex.LBRACKET:
			p.advance()
			t, err := p.expect(lex.STRING, "expected_string_in_bracket")
			if err != nil {
				return nil, err
			}
			parts = append(parts, RefPart{Bracket: t.Value})
			if _, err := p.expect(lex.RBRACKET, "expected_rbracket"); err != nil {
				return nil, err
			}
		default:
			return &Ref{P: pos, Parts: parts}, nil
		}
	}
}

// opName returns the canonical string for an operator token.
func opName(t lex.Token) string {
	switch t.Kind {
	case lex.EQ:
		return "="
	case lex.NEQ:
		return "!="
	case lex.LT:
		return "<"
	case lex.LTE:
		return "<="
	case lex.GT:
		return ">"
	case lex.GTE:
		return ">="
	case lex.PLUS:
		return "+"
	case lex.MINUS:
		return "-"
	case lex.STAR:
		return "*"
	case lex.SLASH:
		return "/"
	case lex.PERCENT:
		return "%"
	case lex.CONCAT:
		return "||"
	case lex.LIKE:
		return "LIKE"
	case lex.GLOB:
		return "GLOB"
	case lex.REGEXP:
		return "REGEXP"
	case lex.MATCH:
		return "MATCH"
	}
	return t.Kind.String()
}
