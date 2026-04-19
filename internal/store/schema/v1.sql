-- pql index schema, v1
--
-- Authoritative reference: docs/structure/initial-plan.md § "The SQLite index".
-- This file is embedded into the binary via schema.go and applied on a fresh
-- database. The index is a pure cache — every row here is regenerable from the
-- vault on disk, which is why the migration policy is drop-and-rebuild on
-- schema_version mismatch (no migration code to maintain).

CREATE TABLE index_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Conventional keys: schema_version, pql_version, config_hash, last_full_scan.
-- schema_version is set by the migrate code; the rest by indexer/config flows.

CREATE TABLE files (
    path          TEXT PRIMARY KEY,
    mtime         INTEGER NOT NULL,           -- unix seconds
    ctime         INTEGER NOT NULL,
    size          INTEGER NOT NULL,
    content_hash  TEXT NOT NULL,
    last_scanned  INTEGER NOT NULL
);

CREATE TABLE frontmatter (
    path        TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value_json  TEXT NOT NULL,                -- canonical typed value
    value_text  TEXT,                         -- for text ops / LIKE / REGEXP
    value_num   REAL,                         -- for numeric comparisons
    PRIMARY KEY (path, key)
);
CREATE INDEX idx_frontmatter_key_text ON frontmatter(key, value_text);
CREATE INDEX idx_frontmatter_key_num  ON frontmatter(key, value_num);

CREATE TABLE tags (
    path TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    tag  TEXT NOT NULL,
    PRIMARY KEY (path, tag)
);
CREATE INDEX idx_tags_tag ON tags(tag);

CREATE TABLE links (
    source_path TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    target_path TEXT NOT NULL,                -- not FK; dangling links are allowed
    alias       TEXT,
    link_type   TEXT NOT NULL,                -- wiki | md | embed
    line        INTEGER NOT NULL
);
CREATE INDEX idx_links_source ON links(source_path);
CREATE INDEX idx_links_target ON links(target_path);

CREATE TABLE headings (
    path         TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    depth        INTEGER NOT NULL,
    text         TEXT NOT NULL,
    line_offset  INTEGER NOT NULL
);

CREATE TABLE bases (
    name      TEXT PRIMARY KEY,                -- .base file name without extension
    path      TEXT NOT NULL,
    ast_json  TEXT NOT NULL,                   -- parsed Base → PQL AST
    mtime     INTEGER NOT NULL
);
