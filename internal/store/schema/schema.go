// Package schema embeds the canonical SQL schema files for the pql index.
//
// The store package applies these on a fresh database. v1.sql is always
// applied; fts.sql is applied only when the user opts into FTS5 body
// search via .pql.yaml. The Version constant must match
// internal/version.SchemaVersion — bump both together.
package schema

import _ "embed"

// Version is the current declared schema version. Stored in
// index_meta.schema_version on a fresh DB and compared on subsequent
// opens. Mismatch triggers drop-and-rebuild (the index is a pure cache).
const Version = 1

// V1 is the base schema applied to every fresh database.
//
//go:embed v1.sql
var V1 string

// FTS is the opt-in FTS5 virtual table; applied when fts: true in config.
//
//go:embed fts.sql
var FTS string
