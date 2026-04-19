-- pql FTS5 schema, v1 — opt-in
--
-- Applied on top of v1.sql when the user sets `fts: true` in .pql.yaml.
-- Held separately because FTS5 indexing is the main cost in re-indexing
-- a vault and many users will not need body search at all (frontmatter,
-- tags, and wikilinks already cover most queries).

CREATE VIRTUAL TABLE fts_bodies USING fts5(
    path UNINDEXED,
    body,
    tokenize = 'porter unicode61'
);
