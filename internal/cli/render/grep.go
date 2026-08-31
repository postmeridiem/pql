package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// CompileGrep turns a --grep pattern into a matcher.
//
// Case-insensitive by design: the flag is named after grep(1) and an agent
// guessing at the casing of a title should not get a silent empty result.
// RE2 has no backtracking, so a pattern supplied by a caller we don't control
// cannot make the binary hang — the reason a regex is safe to accept here at
// all.
//
// A pattern that doesn't compile is a usage error; callers surface it at exit
// 64 with the parse error attached rather than matching nothing.
func CompileGrep(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid --grep pattern %q: %w", pattern, err)
	}
	return re, nil
}

// Matches reports whether any value in row matches re.
//
// Values only — keys are never tested. `--grep status` should search what the
// statuses are, not match every ticket because every ticket carries a status
// key. It also keeps the flag honest: whatever made a record match is visible
// in the record that gets emitted.
//
// The walk descends through objects and arrays to scalar leaves, so a value
// nested inside a join tree counts. Numbers and booleans are matched against
// their rendered text, which is what a caller piping to grep would have seen.
// JSON null has no text and never matches.
func Matches[T any](row T, re *regexp.Regexp) (bool, error) {
	b, err := json.Marshal(row)
	if err != nil {
		return false, fmt.Errorf("grep: marshal row: %w", err)
	}
	var v any
	// UseNumber keeps 1e9 and 1000000000 distinguishable, so a match tests
	// the digits the caller would have read rather than a float re-rendering.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return false, fmt.Errorf("grep: decode row: %w", err)
	}
	return matchValue(v, re), nil
}

// matchValue walks a decoded JSON value, testing scalar leaves against re.
func matchValue(v any, re *regexp.Regexp) bool {
	switch t := v.(type) {
	case map[string]any:
		for _, val := range t {
			if matchValue(val, re) {
				return true
			}
		}
		return false
	case []any:
		for _, val := range t {
			if matchValue(val, re) {
				return true
			}
		}
		return false
	case string:
		return re.MatchString(t)
	case json.Number:
		return re.MatchString(t.String())
	case bool:
		return re.MatchString(strconv.FormatBool(t))
	default:
		// nil (JSON null) and anything else carry no text to match.
		return false
	}
}

// Filter keeps the rows with at least one value matching re, preserving order.
func Filter[T any](rows []T, re *regexp.Regexp) ([]T, error) {
	out := make([]T, 0, len(rows))
	for i, row := range rows {
		ok, err := Matches(row, re)
		if err != nil {
			return nil, fmt.Errorf("grep row %d: %w", i, err)
		}
		if ok {
			out = append(out, row)
		}
	}
	return out, nil
}
