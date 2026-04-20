// Package primitives provides typed Go functions for the per-table queries
// the CLI subcommands need.
//
// Each primitive is a thin wrapper around a SQL query against the store —
// no extraction, no I/O, no rendering, no business logic. Their signatures
// are stable so the CLI subcommands and (eventually) intent-level commands
// can compose them without coupling to the underlying schema.
//
// PQL DSL queries take a different path (internal/query/dsl/); primitives
// are the named-shortcut surface (`pql files`, `pql tags`, `pql backlinks`,
// …) that the v0.1 milestone ships.
package primitives

import "encoding/json"

// File is one row's worth of file metadata. Name is derived from path
// (basename without .md extension) and is provided so renderers don't
// have to reconstruct it.
type File struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"` // unix seconds
}

// TagCount is one tag and the number of files it appears on.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// Backlink is one incoming reference: a file that links to the queried
// target. Path is the source file (the one doing the linking), Name is
// derived from path. Line is the 1-based line number of the link in the
// source. Via is the link kind: "wiki", "embed", or "md".
type Backlink struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Line int    `json:"line"`
	Via  string `json:"via"`
}

// Outlink is one outgoing reference: a link the queried file makes. Target
// is the raw link target as it appears in the source (resolution not
// applied — see initial-plan.md open question #6). Alias is the bracketed
// alias (`[[Note|alias]]`) or empty. Line is 1-based; Via is "wiki",
// "embed", or "md".
type Outlink struct {
	Target string `json:"target"`
	Alias  string `json:"alias,omitempty"`
	Line   int    `json:"line"`
	Via    string `json:"via"`
}

// Heading is one ATX heading from a file. LineOffset is the 0-based byte
// offset from the file start (handy for "jump to heading" without
// re-scanning).
type Heading struct {
	Depth      int    `json:"depth"`
	Text       string `json:"text"`
	LineOffset int    `json:"line_offset"`
}

// Meta is the per-file aggregate returned by `pql meta`. Frontmatter values
// are kept as raw JSON (the value_json column straight through) so the
// output reflects the YAML the user wrote — readers don't have to unwrap
// {"type":"string","value":"..."} envelopes. Type-aware introspection
// belongs to `pql schema`, not `pql meta`.
type Meta struct {
	Path        string                     `json:"path"`
	Name        string                     `json:"name"`
	Size        int64                      `json:"size"`
	Mtime       int64                      `json:"mtime"`
	Frontmatter map[string]json.RawMessage `json:"frontmatter"`
	Tags        []string                   `json:"tags"`
	Outlinks    []Outlink                  `json:"outlinks"`
	Headings    []Heading                  `json:"headings"`
}

// SchemaEntry describes one distinct frontmatter key across the vault:
// the set of types observed (sorted, usually one element — multiple
// elements signal a typing inconsistency worth investigating) and the
// number of files where the key appears.
type SchemaEntry struct {
	Key   string   `json:"key"`
	Types []string `json:"types"`
	Count int      `json:"count"`
}
