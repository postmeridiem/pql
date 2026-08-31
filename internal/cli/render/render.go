// Package render formats query results for stdout per docs/output-contract.md.
//
// v1 ships JSON (default), pretty JSON, and JSONL, plus field projection
// (Project, backing the planning list verbs' --fields flag — the call
// site that finally forced the issue, per D-27). Table / CSV remain
// deferred until a real consumer needs them.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
)

// Format selects the output encoding. Empty / FormatJSON is the default.
type Format string

// The supported Format values. Selection happens via the global --format
// flag; see docs/output-contract.md for shape guarantees.
const (
	FormatJSON   Format = "json"   // canonical: single JSON array on stdout
	FormatPretty Format = "pretty" // same as JSON but indented
	FormatJSONL  Format = "jsonl"  // one JSON object per line; no enclosing array
)

// Opts controls a single Render call. Out defaults to os.Stdout.
type Opts struct {
	Format Format
	Limit  int       // 0 means "no limit"
	Out    io.Writer // defaults to os.Stdout
	// Grep filters rows to those with a matching value and switches output to
	// one record per line — see Render. nil means no filtering.
	Grep *regexp.Regexp
}

// One writes a single value to opts.Out and returns true when a
// non-nil value was written. Used by single-record subcommands like
// `pql meta` and `pql doctor` where a list shape would be misleading.
//
// JSONL is treated as JSON for this path — the streaming-line shape
// doesn't add value for one object. Pretty/JSON behave as expected.
// A nil pointer renders as JSON null and returns false; callers map
// that to the appropriate exit code (typically NoInput).
// Under --grep the object is a single record, so it is a single line: emitted
// when a value matches, and omitted entirely when none does — grep's own
// behaviour on one-line input. A non-match returns false with nothing written,
// which callers must not confuse with the nil-pointer case above; both are
// still exit 0 per D-22.
func One[T any](v *T, opts Opts) (bool, error) {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	if opts.Format == FormatPretty {
		enc.SetIndent("", "  ")
	}
	if v == nil {
		if err := enc.Encode(nil); err != nil {
			return false, fmt.Errorf("render: encode null: %w", err)
		}
		return false, nil
	}
	if opts.Grep != nil {
		ok, err := Matches(*v, opts.Grep)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	if err := enc.Encode(v); err != nil {
		return false, fmt.Errorf("render: encode object: %w", err)
	}
	return true, nil
}

// Render writes rows to opts.Out in the requested format and returns the
// number of rows actually written. Callers map that count to the
// 0-vs-2 exit-code distinction in docs/output-contract.md.
//
// Generic over T so callers don't have to box typed slices into []any.
// JSON encoding goes through encoding/json's reflection path, which works
// for any T whose fields use standard `json:` tags.
//
// Grep filters the output buffer: it runs last, over exactly the rows this
// call was otherwise going to emit, so `pql X --limit 5 --grep p` means what
// `pql X --limit 5 | grep p` means. That ordering matters because several
// verbs push --limit down into the query, and filtering before it would make
// --grep search a page the caller never asked to see.
//
// It also forces the line-oriented shape: matching rows go out one per line
// with no enclosing array, because that is what makes the flag a drop-in for
// a grep pipe. Each line stays individually valid JSON, so the result is
// still parseable as JSONL. Zero survivors write zero bytes — the precedent
// --oneline already sets — and remain exit 0.
func Render[T any](rows []T, opts Opts) (int, error) {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	if opts.Limit > 0 && len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}
	format := opts.Format
	if opts.Grep != nil {
		filtered, err := Filter(rows, opts.Grep)
		if err != nil {
			return 0, err
		}
		rows = filtered
		format = FormatJSONL
	}

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false) // wikilink targets contain & and < in the wild

	switch format {
	case "", FormatJSON:
		// Emit a single JSON array. nil and empty slices both render as "[]".
		if rows == nil {
			rows = []T{}
		}
		if err := enc.Encode(rows); err != nil {
			return 0, fmt.Errorf("render: encode json: %w", err)
		}
	case FormatPretty:
		if rows == nil {
			rows = []T{}
		}
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return 0, fmt.Errorf("render: encode pretty json: %w", err)
		}
	case FormatJSONL:
		// One object per line; no enclosing array. Empty input emits nothing
		// (callers can detect zero matches via the returned count).
		for i, r := range rows {
			if err := enc.Encode(r); err != nil {
				return 0, fmt.Errorf("render: encode jsonl row %d: %w", i, err)
			}
		}
	default:
		return 0, fmt.Errorf("render: unknown format %q (want json|pretty|jsonl)", opts.Format)
	}
	return len(rows), nil
}
