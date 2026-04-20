package eval

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Row is one query result keyed by column name (or alias). Values are
// the JSON-friendly shapes produced by normalise: float64, int64, string,
// json.RawMessage, time.Time, bool, or nil.
type Row map[string]any

// Exec runs the compiled query against db and returns the rows. Column
// names come from the SQL's SELECT list — the compiler's auto-aliasing
// keeps fm.<key> and similar derived projections human-readable.
//
// Scan targets are interface{} so the driver picks the natural Go type;
// normalise() then converts the few shapes that wouldn't JSON-encode
// usefully (notably []byte → json.RawMessage when the bytes are a JSON
// document, otherwise string).
func Exec(ctx context.Context, db *sql.DB, c *Compiled) ([]Row, error) {
	rows, err := db.QueryContext(ctx, c.SQL, c.Params...)
	if err != nil {
		return nil, fmt.Errorf("eval.Exec: query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("eval.Exec: columns: %w", err)
	}

	out := []Row{}
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("eval.Exec: scan: %w", err)
		}
		row := make(Row, len(cols))
		for i, name := range cols {
			row[name] = normalise(raw[i])
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("eval.Exec: iterate: %w", err)
	}
	return out, nil
}

// normalise converts driver-returned values into shapes the renderer
// can JSON-encode without surprises:
//
//   - []byte that holds valid JSON (BLOB cases) → json.RawMessage so it
//     round-trips as the same JSON shape rather than a base64 blob.
//   - []byte that's not valid JSON → string.
//   - string that *starts with* '[' or '{' AND parses as valid JSON →
//     json.RawMessage. modernc.org/sqlite returns TEXT columns as Go
//     strings, including value_json from frontmatter; we pattern-match
//     on the leading bracket so plain strings that happen to be valid
//     JSON literals (e.g. "true" or "42") don't get reinterpreted.
//   - Everything else passes through.
func normalise(v any) any {
	switch x := v.(type) {
	case []byte:
		if json.Valid(x) {
			buf := make([]byte, len(x))
			copy(buf, x)
			return json.RawMessage(buf)
		}
		return string(x)
	case string:
		if len(x) > 0 && (x[0] == '[' || x[0] == '{') && json.Valid([]byte(x)) {
			return json.RawMessage(x)
		}
		return x
	}
	return v
}
