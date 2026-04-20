// Package lex tokenises PQL source per docs/pql-grammar.md.
//
// The lexer is intentionally standalone: it produces a flat stream of
// typed Tokens with line/col positions, no parser dependency. Errors
// carry position so the parser (and ultimately stderr diagnostics)
// can point at the offending byte.
//
// Whitespace, line comments (--), and block comments (/* … */) are
// silently consumed. Identifiers and keywords are case-insensitive
// (the keyword table normalises to uppercase before lookup); quoted
// identifiers preserve case verbatim.
package lex

import (
	"fmt"
	"strings"
	"unicode"
)

// Kind identifies a token's syntactic category.
type Kind int

const (
	EOF Kind = iota

	// Identifiers and literals
	IDENT
	INT
	FLOAT
	STRING

	// Keywords (case-insensitive in source; canonical uppercase)
	SELECT
	DISTINCT
	FROM
	WHERE
	ORDER
	BY
	ASC
	DESC
	NULLS
	FIRST
	LAST
	LIMIT
	OFFSET
	AND
	OR
	NOT
	IS
	NULLKW // the literal NULL keyword (Kind name avoids the Go nil pun)
	IN
	BETWEEN
	LIKE
	GLOB
	REGEXP
	MATCH
	AS
	CAST
	TRUE
	FALSE

	// Operators
	EQ      // =
	NEQ     // != or <>
	LT      // <
	LTE     // <=
	GT      // >
	GTE     // >=
	PLUS    // +
	MINUS   // -
	STAR    // *  (also doubles as the SELECT * wildcard)
	SLASH   // /
	PERCENT // %
	CONCAT  // ||

	// Punctuation
	LPAREN   // (
	RPAREN   // )
	LBRACKET // [
	RBRACKET // ]
	COMMA    // ,
	DOT      // .
)

// String returns a human-readable form for diagnostics.
func (k Kind) String() string {
	switch k {
	case EOF:
		return "EOF"
	case IDENT:
		return "identifier"
	case INT:
		return "integer"
	case FLOAT:
		return "float"
	case STRING:
		return "string"
	case EQ:
		return "="
	case NEQ:
		return "!="
	case LT:
		return "<"
	case LTE:
		return "<="
	case GT:
		return ">"
	case GTE:
		return ">="
	case PLUS:
		return "+"
	case MINUS:
		return "-"
	case STAR:
		return "*"
	case SLASH:
		return "/"
	case PERCENT:
		return "%"
	case CONCAT:
		return "||"
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case LBRACKET:
		return "["
	case RBRACKET:
		return "]"
	case COMMA:
		return ","
	case DOT:
		return "."
	}
	if name, ok := kindName[k]; ok {
		return name
	}
	return fmt.Sprintf("kind(%d)", int(k))
}

var keywords = map[string]Kind{
	"SELECT":   SELECT,
	"DISTINCT": DISTINCT,
	"FROM":     FROM,
	"WHERE":    WHERE,
	"ORDER":    ORDER,
	"BY":       BY,
	"ASC":      ASC,
	"DESC":     DESC,
	"NULLS":    NULLS,
	"FIRST":    FIRST,
	"LAST":     LAST,
	"LIMIT":    LIMIT,
	"OFFSET":   OFFSET,
	"AND":      AND,
	"OR":       OR,
	"NOT":      NOT,
	"IS":       IS,
	"NULL":     NULLKW,
	"IN":       IN,
	"BETWEEN":  BETWEEN,
	"LIKE":     LIKE,
	"GLOB":     GLOB,
	"REGEXP":   REGEXP,
	"MATCH":    MATCH,
	"AS":       AS,
	"CAST":     CAST,
	"TRUE":     TRUE,
	"FALSE":    FALSE,
}

// kindName is the inverse of keywords plus a few extras, populated at
// init() time so Kind.String() can name keyword tokens.
var kindName = map[Kind]string{}

func init() {
	for name, k := range keywords {
		kindName[k] = name
	}
}

// Token is one lexed unit. Value is the raw lexeme (case preserved for
// identifiers and string contents); for STRING the surrounding quotes
// are stripped and `''` un-escaped to `'`. Line and Col are 1-based and
// point at the first character of the token.
type Token struct {
	Kind  Kind
	Value string
	Line  int
	Col   int
}

// Error is a positioned lexer failure. The error code is always
// "pql.lex.<kind>" — matches the conventions in docs/pql-grammar.md.
type Error struct {
	Code string
	Msg  string
	Line int
	Col  int
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s at line %d, col %d: %s", e.Code, e.Line, e.Col, e.Msg)
}

// Scanner walks the source one token at a time. New + repeated Next is
// the streaming API; All is the convenience for tests and one-shot
// callers.
type Scanner struct {
	src  []rune
	pos  int // index into src; points at the next-to-read rune
	line int // 1-based
	col  int // 1-based
}

// New returns a Scanner ready to read src. src is taken by string for
// convenience; we hold it as []rune internally so column counts match
// what users see in their editors regardless of UTF-8 widths.
func New(src string) *Scanner {
	return &Scanner{
		src:  []rune(src),
		pos:  0,
		line: 1,
		col:  1,
	}
}

// All consumes the full token stream and returns it as a slice. The
// trailing EOF token is included so callers can tell "stream ended
// cleanly" from "stream cut off mid-error".
func All(src string) ([]Token, error) {
	s := New(src)
	var out []Token
	for {
		tok, err := s.Next()
		if err != nil {
			return out, err
		}
		out = append(out, tok)
		if tok.Kind == EOF {
			return out, nil
		}
	}
}

// Next returns the next token. EOF is returned with Kind=EOF; subsequent
// calls keep returning EOF tokens at the same position.
func (s *Scanner) Next() (Token, error) {
	for {
		s.skipWhitespace()
		consumed, err := s.skipComment()
		if err != nil {
			return Token{}, err
		}
		if !consumed {
			break
		}
	}
	if s.pos >= len(s.src) {
		return Token{Kind: EOF, Line: s.line, Col: s.col}, nil
	}

	line, col := s.line, s.col
	c := s.src[s.pos]

	switch {
	case isLetter(c) || c == '_':
		return s.scanIdentOrKeyword(line, col), nil
	case c == '"':
		return s.scanQuotedIdent(line, col)
	case c == '\'':
		return s.scanString(line, col)
	case isDigit(c):
		return s.scanNumber(line, col)
	}

	// Operators and punctuation. Multi-char tokens are checked first.
	if tok, ok := s.scanOperator(line, col); ok {
		return tok, nil
	}

	return Token{}, &Error{
		Code: "pql.lex.unexpected_char",
		Msg:  fmt.Sprintf("unexpected character %q", string(c)),
		Line: line,
		Col:  col,
	}
}

// --- character classification --------------------------------------------

func isLetter(r rune) bool {
	return unicode.IsLetter(r)
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isIdentPart(r rune) bool {
	return isLetter(r) || isDigit(r) || r == '_'
}

// --- low-level cursor management -----------------------------------------

func (s *Scanner) advance() {
	if s.pos >= len(s.src) {
		return
	}
	if s.src[s.pos] == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
	s.pos++
}

func (s *Scanner) peek() (rune, bool) {
	if s.pos >= len(s.src) {
		return 0, false
	}
	return s.src[s.pos], true
}

func (s *Scanner) peekAt(offset int) (rune, bool) {
	if s.pos+offset >= len(s.src) {
		return 0, false
	}
	return s.src[s.pos+offset], true
}

// --- whitespace + comments -----------------------------------------------

func (s *Scanner) skipWhitespace() {
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s.advance()
			continue
		}
		break
	}
}

// skipComment consumes one comment if the cursor is positioned at one
// and returns (true, nil); on no-comment returns (false, nil); on an
// unterminated block comment returns (true, *Error).
func (s *Scanner) skipComment() (bool, error) {
	if s.pos+1 >= len(s.src) {
		return false, nil
	}
	a, b := s.src[s.pos], s.src[s.pos+1]
	if a == '-' && b == '-' {
		// Line comment: consume to end-of-line or EOF.
		for s.pos < len(s.src) && s.src[s.pos] != '\n' {
			s.advance()
		}
		return true, nil
	}
	if a == '/' && b == '*' {
		startLine, startCol := s.line, s.col
		s.advance()
		s.advance()
		for s.pos+1 < len(s.src) {
			if s.src[s.pos] == '*' && s.src[s.pos+1] == '/' {
				s.advance()
				s.advance()
				return true, nil
			}
			s.advance()
		}
		return true, &Error{
			Code: "pql.lex.unterminated_block_comment",
			Msg:  "unterminated /* … */ block comment",
			Line: startLine,
			Col:  startCol,
		}
	}
	return false, nil
}

// --- scanners for each token kind ----------------------------------------

func (s *Scanner) scanIdentOrKeyword(line, col int) Token {
	start := s.pos
	for s.pos < len(s.src) && isIdentPart(s.src[s.pos]) {
		s.advance()
	}
	lex := string(s.src[start:s.pos])
	if k, ok := keywords[strings.ToUpper(lex)]; ok {
		return Token{Kind: k, Value: lex, Line: line, Col: col}
	}
	return Token{Kind: IDENT, Value: lex, Line: line, Col: col}
}

func (s *Scanner) scanQuotedIdent(line, col int) (Token, error) {
	s.advance() // opening "
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == '"' {
			// SQL convention: "" is an escaped quote.
			if next, ok := s.peekAt(1); ok && next == '"' {
				b.WriteRune('"')
				s.advance()
				s.advance()
				continue
			}
			s.advance() // closing "
			return Token{Kind: IDENT, Value: b.String(), Line: line, Col: col}, nil
		}
		b.WriteRune(c)
		s.advance()
	}
	return Token{}, &Error{
		Code: "pql.lex.unterminated_quoted_ident",
		Msg:  "unterminated double-quoted identifier",
		Line: line,
		Col:  col,
	}
}

func (s *Scanner) scanString(line, col int) (Token, error) {
	s.advance() // opening '
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == '\'' {
			// SQL convention: '' is an escaped quote inside a string.
			if next, ok := s.peekAt(1); ok && next == '\'' {
				b.WriteRune('\'')
				s.advance()
				s.advance()
				continue
			}
			s.advance() // closing '
			return Token{Kind: STRING, Value: b.String(), Line: line, Col: col}, nil
		}
		b.WriteRune(c)
		s.advance()
	}
	return Token{}, &Error{
		Code: "pql.lex.unterminated_string",
		Msg:  "unterminated string literal",
		Line: line,
		Col:  col,
	}
}

func (s *Scanner) scanNumber(line, col int) (Token, error) {
	start := s.pos
	for s.pos < len(s.src) && isDigit(s.src[s.pos]) {
		s.advance()
	}
	kind := INT
	if s.pos < len(s.src) && s.src[s.pos] == '.' {
		// Lookahead: must be followed by a digit, otherwise leave the dot
		// for the next token (e.g. fm.voting where fm is an identifier).
		if next, ok := s.peekAt(1); ok && isDigit(next) {
			kind = FLOAT
			s.advance() // consume .
			for s.pos < len(s.src) && isDigit(s.src[s.pos]) {
				s.advance()
			}
		}
	}
	return Token{Kind: kind, Value: string(s.src[start:s.pos]), Line: line, Col: col}, nil
}

func (s *Scanner) scanOperator(line, col int) (Token, bool) {
	c := s.src[s.pos]
	// Two-character operators first.
	if next, ok := s.peekAt(1); ok {
		two := string([]rune{c, next})
		switch two {
		case "<=":
			s.advance()
			s.advance()
			return Token{Kind: LTE, Value: two, Line: line, Col: col}, true
		case ">=":
			s.advance()
			s.advance()
			return Token{Kind: GTE, Value: two, Line: line, Col: col}, true
		case "!=":
			s.advance()
			s.advance()
			return Token{Kind: NEQ, Value: two, Line: line, Col: col}, true
		case "<>":
			s.advance()
			s.advance()
			return Token{Kind: NEQ, Value: two, Line: line, Col: col}, true
		case "||":
			s.advance()
			s.advance()
			return Token{Kind: CONCAT, Value: two, Line: line, Col: col}, true
		}
	}
	// Single-char operators and punctuation.
	switch c {
	case '=':
		s.advance()
		return Token{Kind: EQ, Value: "=", Line: line, Col: col}, true
	case '<':
		s.advance()
		return Token{Kind: LT, Value: "<", Line: line, Col: col}, true
	case '>':
		s.advance()
		return Token{Kind: GT, Value: ">", Line: line, Col: col}, true
	case '+':
		s.advance()
		return Token{Kind: PLUS, Value: "+", Line: line, Col: col}, true
	case '-':
		s.advance()
		return Token{Kind: MINUS, Value: "-", Line: line, Col: col}, true
	case '*':
		s.advance()
		return Token{Kind: STAR, Value: "*", Line: line, Col: col}, true
	case '/':
		s.advance()
		return Token{Kind: SLASH, Value: "/", Line: line, Col: col}, true
	case '%':
		s.advance()
		return Token{Kind: PERCENT, Value: "%", Line: line, Col: col}, true
	case '(':
		s.advance()
		return Token{Kind: LPAREN, Value: "(", Line: line, Col: col}, true
	case ')':
		s.advance()
		return Token{Kind: RPAREN, Value: ")", Line: line, Col: col}, true
	case '[':
		s.advance()
		return Token{Kind: LBRACKET, Value: "[", Line: line, Col: col}, true
	case ']':
		s.advance()
		return Token{Kind: RBRACKET, Value: "]", Line: line, Col: col}, true
	case ',':
		s.advance()
		return Token{Kind: COMMA, Value: ",", Line: line, Col: col}, true
	case '.':
		s.advance()
		return Token{Kind: DOT, Value: ".", Line: line, Col: col}, true
	}
	return Token{}, false
}
