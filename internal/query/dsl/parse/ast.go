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

type StringLit struct {
	P     Pos
	Value string
}

type IntLit struct {
	P     Pos
	Value int64
}

type FloatLit struct {
	P     Pos
	Value float64
}

type BoolLit struct {
	P     Pos
	Value bool
}

type NullLit struct {
	P Pos
}

func (n *StringLit) Position() Pos { return n.P }
func (n *IntLit) Position() Pos    { return n.P }
func (n *FloatLit) Position() Pos  { return n.P }
func (n *BoolLit) Position() Pos   { return n.P }
func (n *NullLit) Position() Pos   { return n.P }

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

type RefPart struct {
	Name    string
	Bracket string // mutually exclusive with Name; non-empty means bracket form
}

func (n *Ref) Position() Pos { return n.P }
func (*Ref) expr()           {}

// Call is a function invocation: length(outlinks), date('now', '-30 days').
type Call struct {
	P    Pos
	Name string
	Args []Expr
}

func (n *Call) Position() Pos { return n.P }
func (*Call) expr()           {}

// --- composite expressions -----------------------------------------------

// Unary covers prefix -, +, NOT.
type Unary struct {
	P  Pos
	Op string // "-", "+", "NOT"
	X  Expr
}

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

func (n *Between) Position() Pos { return n.P }
func (*Between) expr()           {}

// IsNull is x IS NULL or x IS NOT NULL.
type IsNull struct {
	P   Pos
	X   Expr
	Not bool
}

func (n *IsNull) Position() Pos { return n.P }
func (*IsNull) expr()           {}

// Tuple is a parenthesised list of two or more expressions. Used as the
// right side of IN: fm.type IN ('a', 'b'). A single ( expr ) is unwrapped
// to its inner expression by the parser, so Tuples always have len(Items)>=2.
type Tuple struct {
	P     Pos
	Items []Expr
}

func (n *Tuple) Position() Pos { return n.P }
func (*Tuple) expr()           {}
