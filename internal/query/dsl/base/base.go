// Package base compiles Obsidian .base YAML files into PQL AST queries
// that the existing eval package can compile and execute.
//
// A .base file defines filters (WHERE), properties (available columns),
// and one or more views (table layouts with column order + sort). The
// compiler resolves a named view (or the first by default) and builds a
// parse.Query that eval.Compile accepts directly.
package base

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/postmeridiem/pql/internal/query/dsl/parse"
)

// File is the top-level YAML shape of an Obsidian .base file.
type File struct {
	Filters    *FilterGroup          `yaml:"filters"`
	Properties map[string]Property   `yaml:"properties"`
	Views      []View                `yaml:"views"`
}

// Property is the metadata for one column.
type Property struct {
	DisplayName string `yaml:"displayName"`
}

// View is one named table layout inside a .base file.
type View struct {
	Type  string     `yaml:"type"`
	Name  string     `yaml:"name"`
	Order []string   `yaml:"order"`
	Sort  []SortSpec `yaml:"sort"`
}

// SortSpec is one ORDER BY item inside a view.
type SortSpec struct {
	Property  string `yaml:"property"`
	Direction string `yaml:"direction"`
}

// FilterGroup holds the and/or array of condition strings.
type FilterGroup struct {
	And []string `yaml:"and"`
	Or  []string `yaml:"or"`
}

// Compile reads a .base YAML file and returns a parse.Query.
// If viewName is empty, the first view is used.
func Compile(path, viewName string) (*parse.Query, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path supplied by caller who discovered it from the vault
	if err != nil {
		return nil, fmt.Errorf("base: read %s: %w", path, err)
	}
	return CompileBytes(data, viewName)
}

// CompileBytes parses YAML bytes and returns a parse.Query.
func CompileBytes(data []byte, viewName string) (*parse.Query, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("base: parse yaml: %w", err)
	}

	view, err := resolveView(f.Views, viewName)
	if err != nil {
		return nil, err
	}

	q := &parse.Query{Pos: parse.Pos{Line: 1, Col: 1}}

	if err := buildSelect(q, view, f.Properties); err != nil {
		return nil, err
	}
	if err := buildWhere(q, f.Filters); err != nil {
		return nil, err
	}
	buildOrderBy(q, view)
	return q, nil
}

func resolveView(views []View, name string) (*View, error) {
	if len(views) == 0 {
		return nil, nil
	}
	if name == "" {
		return &views[0], nil
	}
	for i := range views {
		if strings.EqualFold(views[i].Name, name) {
			return &views[i], nil
		}
	}
	names := make([]string, len(views))
	for i, v := range views {
		names[i] = v.Name
	}
	return nil, fmt.Errorf("base: view %q not found (available: %s)", name, strings.Join(names, ", "))
}

func buildSelect(q *parse.Query, view *View, props map[string]Property) error {
	if view != nil && len(view.Order) > 0 {
		for _, col := range view.Order {
			q.Select = append(q.Select, parse.Projection{Expr: colToRef(col, props)})
		}
		return nil
	}
	if len(props) > 0 {
		for key := range props {
			q.Select = append(q.Select, parse.Projection{Expr: propKeyToRef(key)})
		}
		return nil
	}
	q.Star = true
	return nil
}

// isFileColumn reports whether name is a file-level or array column
// that lives on files directly (as opposed to frontmatter).
func isFileColumn(name string) bool {
	switch name {
	case "path", "mtime", "ctime", "size", "name", "folder",
		"content_hash", "last_scanned", "tags":
		return true
	}
	return false
}

// fieldToRef maps a bare field name to the right AST Ref — file-level
// columns get a single-part Ref, everything else gets fm.<name>.
func fieldToRef(name string) parse.Expr {
	if isFileColumn(name) {
		return &parse.Ref{Parts: []parse.RefPart{{Name: name}}}
	}
	return &parse.Ref{Parts: []parse.RefPart{{Name: "fm"}, {Name: name}}}
}

// colToRef maps a view.order column name to a PQL AST Ref. The order
// list uses short names (like "name", "date") which may correspond to
// either a file-level column or a property with a "note." prefix.
func colToRef(col string, props map[string]Property) parse.Expr {
	if strings.HasPrefix(col, "file.") {
		return &parse.Ref{Parts: []parse.RefPart{{Name: col[len("file."):]}} }
	}
	if _, ok := props["note."+col]; ok {
		return &parse.Ref{Parts: []parse.RefPart{{Name: "fm"}, {Name: col}}}
	}
	return fieldToRef(col)
}

// propKeyToRef converts a properties map key like "note.voting" to a Ref.
func propKeyToRef(key string) parse.Expr {
	if strings.HasPrefix(key, "note.") {
		return fieldToRef(key[len("note."):])
	}
	return fieldToRef(key)
}

func buildWhere(q *parse.Query, fg *FilterGroup) error {
	if fg == nil {
		return nil
	}
	if len(fg.And) > 0 {
		exprs, err := parseConditions(fg.And)
		if err != nil {
			return err
		}
		q.Where = joinExprs(exprs, "AND")
		return nil
	}
	if len(fg.Or) > 0 {
		exprs, err := parseConditions(fg.Or)
		if err != nil {
			return err
		}
		q.Where = joinExprs(exprs, "OR")
		return nil
	}
	return nil
}

func joinExprs(exprs []parse.Expr, op string) parse.Expr {
	if len(exprs) == 1 {
		return exprs[0]
	}
	result := exprs[0]
	for _, e := range exprs[1:] {
		result = &parse.Binary{Op: op, L: result, R: e}
	}
	return result
}

func buildOrderBy(q *parse.Query, view *View) {
	if view == nil {
		return
	}
	for _, s := range view.Sort {
		q.OrderBy = append(q.OrderBy, parse.SortItem{
			Expr: fieldToRef(s.Property),
			Desc: strings.EqualFold(s.Direction, "DESC"),
		})
	}
}

// --- filter condition parsing -----------------------------------------------

// condRe matches the three-part structure of a .base filter condition.
// Groups: 1=left, 2=operator, 3=right value.
var condRe = regexp.MustCompile(`^(.+?)\s+(==|!=|>=|<=|>|<|contains)\s+(.+)$`)

func parseConditions(raw []string) ([]parse.Expr, error) {
	out := make([]parse.Expr, 0, len(raw))
	for _, s := range raw {
		e, err := parseCondition(s)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func parseCondition(s string) (parse.Expr, error) {
	m := condRe.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("base: cannot parse filter condition: %q", s)
	}
	left, op, right := strings.TrimSpace(m[1]), m[2], strings.TrimSpace(m[3])

	leftExpr := parseRef(left)
	rightExpr, err := parseValue(right)
	if err != nil {
		return nil, fmt.Errorf("base: filter %q: %w", s, err)
	}

	switch op {
	case "==":
		return &parse.Binary{Op: "=", L: leftExpr, R: rightExpr}, nil
	case "!=":
		return &parse.Binary{Op: "!=", L: leftExpr, R: rightExpr}, nil
	case ">", "<", ">=", "<=":
		return &parse.Binary{Op: op, L: leftExpr, R: rightExpr}, nil
	case "contains":
		// "file.tags contains X" → X IN tags
		return &parse.Binary{Op: "IN", L: rightExpr, R: leftExpr}, nil
	}
	return nil, fmt.Errorf("base: unknown operator %q in condition %q", op, s)
}

func parseRef(s string) parse.Expr {
	switch {
	case strings.HasPrefix(s, "note."):
		return fieldToRef(s[len("note."):])
	case strings.HasPrefix(s, "file."):
		return &parse.Ref{Parts: []parse.RefPart{{Name: s[len("file."):]}} }
	default:
		return fieldToRef(s)
	}
}

func parseValue(s string) (parse.Expr, error) {
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`)) {
		return &parse.StringLit{Value: s[1 : len(s)-1]}, nil
	}
	if s == "true" {
		return &parse.BoolLit{Value: true}, nil
	}
	if s == "false" {
		return &parse.BoolLit{Value: false}, nil
	}
	if s == "null" {
		return &parse.NullLit{}, nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &parse.IntLit{Value: i}, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return &parse.FloatLit{Value: f}, nil
	}
	return nil, fmt.Errorf("cannot parse value %q", s)
}
