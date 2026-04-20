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
