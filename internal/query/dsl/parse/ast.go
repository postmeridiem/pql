// Package parse turns lexer tokens into a PQL AST per docs/pql-grammar.md.
//
// The AST mirrors the EBNF: one Query at the root, then Projection /
// SortItem / Limit collections, then a single Expr hierarchy underneath
// for everything WHERE-clause-shaped. Position-bearing nodes carry the
// line/col of their first token so the evaluator and CLI diagnostics can
// point at the source.
package parse

// Pos is the 1-based line+column of a node's first token. Mirrors
// lex.Token's Line/Col so error spans in stderr line up with what users
// see in their editor.
type Pos struct {
	Line int
	Col  int
}

// Query is the root of every parsed PQL query.
type Query struct {
	Pos      Pos
	Distinct bool
	// Star=true means SELECT *; Select is empty in that case.
	Star    bool
	Select  []Projection
	From    string // empty means default ("files")
	Where   Expr   // nil if no WHERE clause
	OrderBy []SortItem
	Limit   *Limit // nil if no LIMIT
}

// Projection is one entry in the SELECT list. Alias is empty when the
// user didn't write AS.
type Projection struct {
	Expr  Expr
	Alias string
}

// SortItem is one entry in ORDER BY. Desc=false is ASC (the default).
// Nulls is "first", "last", or "" for unspecified.
type SortItem struct {
	Expr  Expr
	Desc  bool
	Nulls string
}

// Limit holds the LIMIT (and optional OFFSET) clause.
type Limit struct {
	N      int64
	Offset int64 // 0 if no OFFSET
}

// Expr is any expression node. The marker method keeps the interface
// closed within this package.
type Expr interface {
	Position() Pos
	expr()
}

// --- literals ------------------------------------------------------------
//
// One Lit type per scalar shape. The compiler dispatches on the concrete
// type; keeping them as separate types (rather than a tagged union) means
// the type system enforces "string literals can't go where ints are
// expected" at compile time.

// StringLit is a quoted string literal.
type StringLit struct {
	P     Pos
	Value string
}

// IntLit is an integer literal.
type IntLit struct {
	P     Pos
	Value int64
}

// FloatLit is a floating-point literal.
type FloatLit struct {
	P     Pos
	Value float64
}

// BoolLit is TRUE or FALSE.
type BoolLit struct {
	P     Pos
	Value bool
}

// NullLit is the NULL keyword.
type NullLit struct {
	P Pos
}

// Position returns the source location of the literal's first token.
func (n *StringLit) Position() Pos { return n.P }

// Position returns the source location of the literal's first token.
func (n *IntLit) Position() Pos { return n.P }

// Position returns the source location of the literal's first token.
func (n *FloatLit) Position() Pos { return n.P }

// Position returns the source location of the literal's first token.
func (n *BoolLit) Position() Pos { return n.P }

// Position returns the source location of the literal's first token.
func (n *NullLit) Position() Pos { return n.P }

func (*StringLit) expr() {}
func (*IntLit) expr()    {}
func (*FloatLit) expr()  {}
func (*BoolLit) expr()   {}
func (*NullLit) expr()   {}

// --- references and calls ------------------------------------------------

// Ref is an identifier path: name, fm.voting, fm['key with spaces'].
// Each Part is either a dotted name (RefPart.Name) or a bracket-string
// access (RefPart.Bracket). The first Part always has a Name (the root
// identifier); subsequent Parts may use either form.
type Ref struct {
	P     Pos
	Parts []RefPart
}

// RefPart is one segment of a Ref. Exactly one of Name / Bracket is set.
type RefPart struct {
	Name    string
	Bracket string // mutually exclusive with Name; non-empty means bracket form
}

// Position returns the source location of the ref's first identifier.
func (n *Ref) Position() Pos { return n.P }
func (*Ref) expr()           {}

// Call is a function invocation: length(outlinks), date('now', '-30 days').
type Call struct {
	P    Pos
	Name string
	Args []Expr
}

// Position returns the source location of the call's name token.
func (n *Call) Position() Pos { return n.P }
func (*Call) expr()           {}

// --- composite expressions -----------------------------------------------

// Unary covers prefix -, +, NOT.
type Unary struct {
	P  Pos
	Op string // "-", "+", "NOT"
	X  Expr
}

// Position returns the source location of the unary operator token.
func (n *Unary) Position() Pos { return n.P }
func (*Unary) expr()           {}

// Binary covers everything two-operand: arithmetic, logical, equality,
// LIKE/GLOB/REGEXP/MATCH, IN/NOT IN.
type Binary struct {
	P  Pos
	Op string
	L  Expr
	R  Expr
}

// Position returns the source location of the binary expression's left operand.
func (n *Binary) Position() Pos { return n.P }
func (*Binary) expr()           {}

// Between is x BETWEEN low AND high (or NOT BETWEEN). Distinct from
// Binary because the right side splits into two operands joined by AND.
type Between struct {
	P    Pos
	X    Expr
	Low  Expr
	High Expr
	Not  bool
}

// Position returns the source location of the BETWEEN expression's value operand.
func (n *Between) Position() Pos { return n.P }
func (*Between) expr()           {}

// IsNull is x IS NULL or x IS NOT NULL.
type IsNull struct {
	P   Pos
	X   Expr
	Not bool
}

// Position returns the source location of the IS NULL expression's value operand.
func (n *IsNull) Position() Pos { return n.P }
func (*IsNull) expr()           {}

// Tuple is a parenthesised list of two or more expressions. Used as the
// right side of IN: fm.type IN ('a', 'b'). A single ( expr ) is unwrapped
// to its inner expression by the parser, so Tuples always have len(Items)>=2.
type Tuple struct {
	P     Pos
	Items []Expr
}

// Position returns the source location of the tuple's opening parenthesis.
func (n *Tuple) Position() Pos { return n.P }
func (*Tuple) expr()           {}
