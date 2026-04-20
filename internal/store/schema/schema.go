// Package schema embeds the canonical SQL schema files for the pql index.
//
// The store package applies these on a fresh database. The current vN.sql
// is always applied; fts.sql is applied only when the user opts into FTS5
// body search via .pql/config.yaml. The Version constant must match the file
// suffix; bump both together when the schema evolves.
//
// We never write migration code — every bump triggers a drop-and-rebuild
// because the index is a pure cache. Older vN.sql files are kept in the
// repo for historical reference but are not embedded.
package schema

import _ "embed"

// Version is the current declared schema version. Stored in
// index_meta.schema_version on a fresh DB and compared on subsequent
// opens. Mismatch triggers drop-and-rebuild (the index is a pure cache).
const Version = 2

// Current is the base schema applied to every fresh database.
//
//go:embed v2.sql
var Current string

// FTS is the opt-in FTS5 virtual table; applied when fts: true in config.
//
//go:embed fts.sql
var FTS string
