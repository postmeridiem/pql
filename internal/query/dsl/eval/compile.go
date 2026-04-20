// Package eval compiles parsed PQL queries into SQLite SQL and runs them
// against the store. compile.go is the AST → (SQL, params) translator;
// exec.go (separate commit) takes those and produces typed result rows.
//
// v1 column scope is deliberately partial — see the per-column branches
// in compileRef. Unsupported references and operations error explicitly
// with pql.eval.<kind> codes so users get a clear "not yet" rather than
// silent garbage.
package eval

import (
	"fmt"
	"strings"

	"github.com/postmeridiem/pql/internal/query/dsl/parse"
)

// Compiled is the SQL + parameter list produced from an AST. Pass to a
// store DB.QueryContext call directly.
type Compiled struct {
	SQL    string
	Params []any
}

// Error is a positioned compile-time failure. Codes follow
// pql.eval.<kind> per docs/pql-grammar.md.
type Error struct {
	Code string
	Msg  string
	Line int
	Col  int
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.Code, e.Msg)
	}
	return fmt.Sprintf("%s at line %d, col %d: %s", e.Code, e.Line, e.Col, e.Msg)
}

// Compile returns the SQL + params for q. The caller is responsible for
// connecting them to a *sql.DB.
func Compile(q *parse.Query) (*Compiled, error) {
	c := &compiler{}
	if err := c.query(q); err != nil {
		return nil, err
	}
	return &Compiled{SQL: c.buf.String(), Params: c.params}, nil
}

type compiler struct {
	buf    strings.Builder
	params []any
}

func (c *compiler) emit(s string, args ...any) {
	if len(args) == 0 {
		c.buf.WriteString(s)
		return
	}
	fmt.Fprintf(&c.buf, s, args...)
}

func (c *compiler) param(v any) {
	c.params = append(c.params, v)
	c.buf.WriteString("?")
}

// errAt is a small helper to attach a node's position to an Error.
func errAt(e parse.Expr, code, format string, args ...any) *Error {
	pos := e.Position()
	return &Error{
		Code: "pql.eval." + code,
		Msg:  fmt.Sprintf(format, args...),
		Line: pos.Line,
		Col:  pos.Col,
	}
}

// --- top-level query -----------------------------------------------------

func (c *compiler) query(q *parse.Query) error {
	if q.From != "" && q.From != "files" {
		return &Error{
			Code: "pql.eval.unsupported_from",
			Msg:  fmt.Sprintf("FROM %q not supported in v1; only \"files\" (or omitted) is", q.From),
			Line: q.Pos.Line,
			Col:  q.Pos.Col,
		}
	}

	c.emit("SELECT ")
	if q.Distinct {
		c.emit("DISTINCT ")
	}
	if q.Star {
		c.emit("files.path AS path, files.mtime AS mtime, files.ctime AS ctime, files.size AS size, files.content_hash AS content_hash, files.last_scanned AS last_scanned")
	} else {
		for i, p := range q.Select {
			if i > 0 {
				c.emit(", ")
			}
			if err := c.expr(p.Expr); err != nil {
				return err
			}
			if p.Alias != "" {
				c.emit(" AS \"%s\"", p.Alias)
			} else if alias := defaultAlias(p.Expr); alias != "" {
				// SQLite's default column name is the SQL fragment itself,
				// which is unfriendly when the fragment is a long subquery.
				// Emit a stable alias derived from the AST.
				c.emit(" AS \"%s\"", alias)
			}
		}
	}

	c.emit(" FROM files")

	if q.Where != nil {
		c.emit(" WHERE ")
		if err := c.expr(q.Where); err != nil {
			return err
		}
	}

	if len(q.OrderBy) > 0 {
		c.emit(" ORDER BY ")
		for i, item := range q.OrderBy {
			if i > 0 {
				c.emit(", ")
			}
			if err := c.expr(item.Expr); err != nil {
				return err
			}
			if item.Desc {
				c.emit(" DESC")
			} else {
				c.emit(" ASC")
			}
			switch item.Nulls {
			case "first":
				c.emit(" NULLS FIRST")
			case "last":
				c.emit(" NULLS LAST")
			}
		}
	}

	if q.Limit != nil {
		c.emit(" LIMIT ")
		c.param(q.Limit.N)
		if q.Limit.Offset > 0 {
			c.emit(" OFFSET ")
			c.param(q.Limit.Offset)
		}
	}

	return nil
}

// defaultAlias returns a friendly column name for a projection that
// otherwise would inherit the raw SQL fragment as its column name.
// Returns "" when the projection is a plain column reference (which gets
// a sensible default from SQLite already).
func defaultAlias(e parse.Expr) string {
	switch x := e.(type) {
	case *parse.Ref:
		// Refs with multiple parts (fm.voting) need an alias, otherwise the
		// JSON output key becomes the full SQL subquery. Single-part refs
		// (path, mtime) inherit their column name natively.
		if len(x.Parts) >= 2 {
			var b strings.Builder
			for i, p := range x.Parts {
				if i > 0 {
					b.WriteByte('.')
				}
				if p.Name != "" {
					b.WriteString(p.Name)
				} else {
					b.WriteString(p.Bracket)
				}
			}
			return b.String()
		}
	case *parse.Call:
		return x.Name
	}
	return ""
}

// --- expressions ---------------------------------------------------------

func (c *compiler) expr(e parse.Expr) error {
	switch x := e.(type) {
	case *parse.StringLit:
		c.param(x.Value)
		return nil
	case *parse.IntLit:
		c.param(x.Value)
		return nil
	case *parse.FloatLit:
		c.param(x.Value)
		return nil
	case *parse.BoolLit:
		// SQLite represents booleans as integers; align with frontmatter
		// type=bool storing 0/1 in value_num.
		if x.Value {
			c.param(int64(1))
		} else {
			c.param(int64(0))
		}
		return nil
	case *parse.NullLit:
		c.emit("NULL")
		return nil
	case *parse.Ref:
		return c.ref(x)
	case *parse.Call:
		return c.call(x)
	case *parse.Unary:
		return c.unary(x)
	case *parse.Binary:
		return c.binary(x)
	case *parse.Between:
		return c.between(x)
	case *parse.IsNull:
		return c.isNull(x)
	case *parse.Tuple:
		return c.tuple(x)
	}
	return &Error{
		Code: "pql.eval.unknown_expr",
		Msg:  fmt.Sprintf("unsupported expression type %T", e),
	}
}

// fileColumn maps a single-part identifier to its SQL form. Returns
// the SQL fragment + true on hit; false means "not a known file-level
// column".
func fileColumn(name string) (string, bool) {
	switch name {
	case "path":
		return "files.path", true
	case "mtime":
		return "files.mtime", true
	case "ctime":
		return "files.ctime", true
	case "size":
		return "files.size", true
	case "content_hash":
		return "files.content_hash", true
	case "last_scanned":
		return "files.last_scanned", true

	// Derived via SUBSTR/INSTR — verbose but pure SQLite (no UDFs needed
	// for v1). The reverse(path) || '/' trick handles paths with no slash:
	// instr returns 0, the +1 normalises so substr from position 1 returns
	// the full path and rtrim strips '.md'.
	case "name":
		return `rtrim(substr(files.path, length(files.path) - instr(reverse(files.path) || '/', '/') + 2), '.md')`, true
	case "folder":
		return `substr(files.path, 1, length(files.path) - instr(reverse(files.path) || '/', '/') + 1 - 2)`, true
	}
	return "", false
}

func (c *compiler) ref(r *parse.Ref) error {
	// Single-part: must be a known file column. Any other root identifier
	// (tags, outlinks, body, …) is only meaningful in specific operator
	// contexts (e.g. 'x' IN tags) and is handled by the operator branch.
	if len(r.Parts) == 1 {
		name := r.Parts[0].Name
		if sql, ok := fileColumn(name); ok {
			c.buf.WriteString(sql)
			return nil
		}
		switch name {
		case "tags", "inlinks", "outlinks", "headings", "body":
			return &Error{
				Code: "pql.eval.bare_array_ref",
				Msg: fmt.Sprintf(
					"%q is an array column and only meaningful as the right side of IN (e.g. 'foo' IN %s); "+
						"projecting it as a scalar isn't supported in v1",
					name, name),
				Line: r.P.Line, Col: r.P.Col,
			}
		case "fm":
			return &Error{
				Code: "pql.eval.bare_fm",
				Msg:  "fm is an object; access a key with fm.<name> or fm['<name>']",
				Line: r.P.Line, Col: r.P.Col,
			}
		}
		return &Error{
			Code: "pql.eval.unknown_column",
			Msg:  fmt.Sprintf("unknown column %q", name),
			Line: r.P.Line, Col: r.P.Col,
		}
	}

	// Two-part: only fm.<key> or fm['<key>'] supported in v1.
	if len(r.Parts) == 2 && r.Parts[0].Name == "fm" {
		key := r.Parts[1].Name
		if key == "" {
			key = r.Parts[1].Bracket
		}
		if key == "" {
			return &Error{
				Code: "pql.eval.bad_fm_access",
				Msg:  "fm access requires a key (fm.<name> or fm['<name>'])",
				Line: r.P.Line, Col: r.P.Col,
			}
		}
		// Type-dispatching subquery: returns the value in the shape SQLite
		// can compare directly against literals, leveraging the type column
		// added in schema v2.
		c.emit(`(SELECT CASE type WHEN 'string' THEN value_text WHEN 'number' THEN value_num WHEN 'bool' THEN value_num ELSE value_json END FROM frontmatter WHERE path = files.path AND key = `)
		c.param(key)
		c.emit(")")
		return nil
	}

	return &Error{
		Code: "pql.eval.unsupported_ref",
		Msg:  fmt.Sprintf("reference path of %d parts not supported in v1", len(r.Parts)),
		Line: r.P.Line, Col: r.P.Col,
	}
}

// --- function calls -----------------------------------------------------

func (c *compiler) call(call *parse.Call) error {
	name, ok := functionMap[strings.ToLower(call.Name)]
	if !ok {
		return &Error{
			Code: "pql.eval.unknown_function",
			Msg:  fmt.Sprintf("unknown function %q (see docs/pql-grammar.md § Functions)", call.Name),
			Line: call.P.Line, Col: call.P.Col,
		}
	}
	c.emit("%s(", name)
	for i, a := range call.Args {
		if i > 0 {
			c.emit(", ")
		}
		if err := c.expr(a); err != nil {
			return err
		}
	}
	c.emit(")")
	return nil
}

// functionMap is the allowlist of PQL function names → their SQLite-side
// names. Aliases (today/now → date('now')/datetime('now')) are handled by
// matching here and emitting the rewrite. The list mirrors what's
// documented in pql-grammar.md § Functions.
var functionMap = map[string]string{
	// String
	"length":  "length",
	"upper":   "upper",
	"lower":   "lower",
	"trim":    "trim",
	"ltrim":   "ltrim",
	"rtrim":   "rtrim",
	"substr":  "substr",
	"replace": "replace",
	"instr":   "instr",
	"printf":  "printf",
	"concat":  "concat", // SQLite 3.44+
	// Date / time (modifier-style; users pass strings like '-30 days')
	"date":      "date",
	"time":      "time",
	"datetime":  "datetime",
	"julianday": "julianday",
	"strftime":  "strftime",
	// Math
	"abs":   "abs",
	"min":   "min",
	"max":   "max",
	"round": "round",
	"ceil":  "ceil",
	"floor": "floor",
	// Type / null
	"coalesce": "coalesce",
	"nullif":   "nullif",
	// JSON
	"json":              "json",
	"json_extract":      "json_extract",
	"json_array_length": "json_array_length",
}

// --- unary, binary, BETWEEN, IS NULL, tuple -----------------------------

func (c *compiler) unary(u *parse.Unary) error {
	switch u.Op {
	case "-", "+":
		c.emit("(%s", u.Op)
		if err := c.expr(u.X); err != nil {
			return err
		}
		c.emit(")")
		return nil
	case "NOT":
		c.emit("(NOT ")
		if err := c.expr(u.X); err != nil {
			return err
		}
		c.emit(")")
		return nil
	}
	return &Error{
		Code: "pql.eval.unknown_unary",
		Msg:  fmt.Sprintf("unknown unary operator %q", u.Op),
		Line: u.P.Line, Col: u.P.Col,
	}
}

func (c *compiler) binary(b *parse.Binary) error {
	// Special form: <literal-or-expr> IN <array column> → EXISTS subquery
	// against the matching child table. The plain SQL `IN <subquery>` form
	// would also work but the EXISTS shape is cheaper and friendlier to
	// SQLite's query planner with our (path, tag) and (path, value_text)
	// indexes.
	if b.Op == "IN" || b.Op == "NOT IN" {
		if r, ok := b.R.(*parse.Ref); ok && len(r.Parts) == 1 {
			if sql, ok := arrayColumnExistsSQL(r.Parts[0].Name); ok {
				negate := ""
				if b.Op == "NOT IN" {
					negate = "NOT "
				}
				c.emit("%sEXISTS (%s AND ", negate, sql)
				// Emit the comparison value column reference; for tags
				// the comparison column is "tag", for outlinks "target_path",
				// etc. — known per arrayColumnExistsSQL.
				c.buf.WriteString(arrayMatchColumn(r.Parts[0].Name))
				c.emit(" = ")
				if err := c.expr(b.L); err != nil {
					return err
				}
				c.emit(")")
				return nil
			}
			// Other single-part refs (e.g. some unknown column) fall through
			// to the standard IN path which will likely error.
		}
	}

	// Standard binary form.
	c.emit("(")
	if err := c.expr(b.L); err != nil {
		return err
	}
	c.emit(" %s ", b.Op)
	if err := c.expr(b.R); err != nil {
		return err
	}
	c.emit(")")
	return nil
}

func (c *compiler) between(b *parse.Between) error {
	negate := ""
	if b.Not {
		negate = "NOT "
	}
	c.emit("(")
	if err := c.expr(b.X); err != nil {
		return err
	}
	c.emit(" %sBETWEEN ", negate)
	if err := c.expr(b.Low); err != nil {
		return err
	}
	c.emit(" AND ")
	if err := c.expr(b.High); err != nil {
		return err
	}
	c.emit(")")
	return nil
}

func (c *compiler) isNull(n *parse.IsNull) error {
	c.emit("(")
	if err := c.expr(n.X); err != nil {
		return err
	}
	if n.Not {
		c.emit(" IS NOT NULL)")
	} else {
		c.emit(" IS NULL)")
	}
	return nil
}

func (c *compiler) tuple(t *parse.Tuple) error {
	c.emit("(")
	for i, item := range t.Items {
		if i > 0 {
			c.emit(", ")
		}
		if err := c.expr(item); err != nil {
			return err
		}
	}
	c.emit(")")
	return nil
}

// arrayColumnExistsSQL returns the inner SELECT of an EXISTS subquery
// against a child table that holds opts.array semantics for files.
// Returns the SQL prefix (without the trailing AND <col>=...) so the
// caller can append the membership predicate.
//
// v1: tags only. outlinks/inlinks/headings membership lands in v1.x with
// the resolution rules from initial-plan.md open question #6.
func arrayColumnExistsSQL(name string) (string, bool) {
	switch name {
	case "tags":
		return "SELECT 1 FROM tags WHERE tags.path = files.path", true
	}
	return "", false
}

func arrayMatchColumn(name string) string {
	switch name {
	case "tags":
		return "tags.tag"
	}
	return ""
}
