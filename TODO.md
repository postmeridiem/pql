# pql — TODO

Forward-looking work lives in `decisions/` now — open questions are
Q-records, confirmed decisions are D-records, and `pql decisions sync`
keeps them queryable. This file is a lightweight pointer, not the
canonical tracker.

## Open

See `decisions/questions.md` for the full list. Highlights:

- **Q-5:** Body access in DSL (requires FTS5)
- **Q-6:** Outlink target normalization
- **Q-7:** Code-aware extractor (tree-sitter)
- **Q-1:** Markdown mirror for tickets
- **Q-2:** FTS for ticket/decision search

## Shipped

Everything below landed between v0.1.0 and v1.0.0. The changelog
(`CHANGELOG.md`) has the per-version detail.

- Shell, base compiler, completions (v0.1.1-dev → v0.2.0)
- Planning subcommands, watch, DSL outlinks/inlinks/headings (v0.2.0)
- Enrichment layer, intents, self-update, telemetry, eval harness (v1.0.0)
- Documentation, CI housekeeping, decisions/ directory (post-1.0.0)
