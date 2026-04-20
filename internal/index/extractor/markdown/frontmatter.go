package markdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// frontmatterDelim is the YAML frontmatter marker. TOML (`+++`) is on the
// roadmap; when it lands, SplitFrontmatter dispatches on the leading bytes.
var frontmatterDelim = []byte("---")

// SplitFrontmatter separates the YAML frontmatter (if any) from the body.
// The frontmatter must begin at byte 0 with `---\n` (or `---\r\n`) and is
// terminated by another `---` on its own line.
//
// If no frontmatter is present (or the opening delimiter has no closer),
// head is nil and body is the original raw bytes — the file is treated as
// "plain markdown with no metadata" rather than an error.
func SplitFrontmatter(raw []byte) (head, body []byte, err error) {
	if !startsWithFenceLine(raw) {
		return nil, raw, nil
	}
	// Skip the opening delimiter line (handles both LF and CRLF).
	_, rest := nextLine(raw)
	headBuf := bytes.Buffer{}
	for len(rest) > 0 {
		line, after := nextLine(rest)
		if isFenceLine(line) {
			return headBuf.Bytes(), after, nil
		}
		headBuf.Write(line)
		headBuf.WriteByte('\n')
		rest = after
	}
	// Opening delimiter without a closer — treat as no frontmatter so the
	// whole file is still indexed as body. Surface a soft warning later via
	// the indexer's diagnostic stream.
	return nil, raw, nil
}

// ParseFrontmatter decodes head as YAML into typed Values keyed by
// frontmatter key. Empty / nil head returns an empty map; a malformed YAML
// document returns an error so the indexer can surface a diagnostic.
//
// Per the schema, null values are skipped (no row in the frontmatter table)
// because they carry no useful query signal.
func ParseFrontmatter(head []byte) (map[string]Value, error) {
	if len(bytes.TrimSpace(head)) == 0 {
		return map[string]Value{}, nil
	}
	var raw map[string]any
	if err := yaml.Unmarshal(head, &raw); err != nil {
		return nil, fmt.Errorf("markdown: parse frontmatter: %w", err)
	}
	out := make(map[string]Value, len(raw))
	for k, v := range raw {
		if v == nil {
			continue
		}
		val, err := typeValue(v)
		if err != nil {
			return nil, fmt.Errorf("markdown: type frontmatter key %q: %w", k, err)
		}
		out[k] = val
	}
	return out, nil
}

// typeValue produces the typed Value for one decoded YAML scalar/list/map.
// The returned Value.Type tag is one of the markdown.Type* constants and
// always set; a value that doesn't match any known shape falls back to
// TypeString with text-form preserved in JSON only.
func typeValue(v any) (Value, error) {
	jb, err := json.Marshal(v)
	if err != nil {
		return Value{}, err
	}
	val := Value{JSON: string(jb)}
	switch x := v.(type) {
	case string:
		val.Type = TypeString
		val.Text = x
		val.HasText = true
	case bool:
		val.Type = TypeBool
		if x {
			val.Num = 1
		}
		val.HasNum = true
	case int:
		val.Type = TypeNumber
		val.Num = float64(x)
		val.HasNum = true
	case int64:
		val.Type = TypeNumber
		val.Num = float64(x)
		val.HasNum = true
	case uint64:
		val.Type = TypeNumber
		val.Num = float64(x)
		val.HasNum = true
	case float64:
		val.Type = TypeNumber
		val.Num = x
		val.HasNum = true
	case float32:
		val.Type = TypeNumber
		val.Num = float64(x)
		val.HasNum = true
	case time.Time:
		// Dates / datetimes ride as strings in v1; the JSON form is the ISO
		// representation, which sorts and compares correctly. A native date
		// type can land later if querying ergonomics demand it.
		val.Type = TypeString
		val.Text = x.Format(time.RFC3339)
		val.HasText = true
	case []any:
		val.Type = TypeList
	case map[string]any, map[any]any:
		val.Type = TypeObject
	default:
		// Unknown YAML decoder output — fall back to TypeString so the NOT
		// NULL constraint on frontmatter.type is satisfied. JSON form is
		// still preserved for the escape hatch.
		val.Type = TypeString
	}
	return val, nil
}

// startsWithFenceLine reports whether raw begins with `---` followed by an
// EOL (or EOF). Any leading whitespace disqualifies — frontmatter must live
// at byte 0, no BOM, no blank lines.
func startsWithFenceLine(raw []byte) bool {
	if !bytes.HasPrefix(raw, frontmatterDelim) {
		return false
	}
	tail := raw[len(frontmatterDelim):]
	if len(tail) == 0 {
		return true
	}
	switch tail[0] {
	case '\n', '\r':
		return true
	}
	return false
}

// isFenceLine reports whether a single (newline-stripped) line is the
// frontmatter terminator `---` (with optional trailing whitespace).
func isFenceLine(line []byte) bool {
	return bytes.Equal(bytes.TrimRight(line, " \t\r"), frontmatterDelim)
}

// nextLine returns one line (without the trailing newline) and the
// remaining bytes after that newline. Handles LF and CRLF transparently.
// If raw has no newline, the whole slice is returned and after is nil.
func nextLine(raw []byte) (line, after []byte) {
	idx := bytes.IndexByte(raw, '\n')
	if idx == -1 {
		return raw, nil
	}
	line = raw[:idx]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line, raw[idx+1:]
}
