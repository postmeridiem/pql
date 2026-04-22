# Open Questions

---

### Q-001: Markdown mirror for tickets
- **Status:** Open
- **Question:** When does `pql ticket export` become automatic? On every mutation, or on explicit command only?
- **Context:** Ticket source of truth is SQLite for now. Markdown mirror (`tickets/T-NNN.md` auto-written) is the long-term answer but the merge-conflict story isn't solved.

### Q-002: FTS for ticket/decision search
- **Status:** Open
- **Question:** SQLite FTS5 or external index for `pql ticket search` and `pql plan search`?
- **Context:** FTS5 is built-in, zero deps, good enough for N < 10k records. Currently deferred.

### Q-003: Multi-user planning DB
- **Status:** Open
- **Question:** If we want team-shared planning state, what's the locking story?
- **Context:** Currently single-user-per-DB. Network FS locking is fragile. Probably out of scope until the markdown mirror lands.

### Q-004: Validator strictness
- **Status:** Open
- **Question:** How strict should `pql decisions validate` be? Missing fields = error or warning?
- **Context:** The parser is permissive (missing Date: doesn't fail). The validate command exits non-zero only on duplicate IDs, empty titles, and broken refs.

### Q-005: Body access in the DSL
- **Status:** Open
- **Question:** `body` is reserved as an array column but requires FTS5. When does it ship, and what's the syntax?
- **Context:** The compiler returns a clear error for bare `body` refs. FTS5 is opt-in via `fts: true` in config. The natural syntax would be `'term' IN body` compiling to an FTS5 MATCH, but the virtual table isn't created unless the user opts in.

### Q-006: Outlink target normalization
- **Status:** Open
- **Question:** The links table stores raw wikilink targets (e.g. `brief`, not `sessions/.../brief.md`). Should the extractor normalize to full paths at index time?
- **Context:** Inlinks in the DSL work around this with a three-way OR (exact match, append .md, LIKE basename). Normalizing at index time would simplify the join but requires Obsidian-style shortest-unambiguous-path resolution. The pragmatic workaround works; the question is long-term correctness.

### Q-007: Code-aware extractor
- **Status:** Open
- **Question:** When does tree-sitter land? What languages first?
- **Context:** The extractor registry is designed for it — register by file pattern, produce structured output. No changes to store, connect, or query. The open question is priority and whether the tree-sitter cgo dependency is acceptable.

### Q-008: Occasional pql.db backups into git
- **Status:** Open
- **Question:** How should users snapshot planning state (pql.db) into version control? What events trigger it, and what's the artifact shape?
- **Context:** pql.db is gitignored (lives under `.pql/`), but planning state — decisions synced, tickets created, status transitions — is valuable enough to version. Options: (1) `pql plan export --to pql-snapshot.json` writes a portable dump that *is* committed; (2) `pql ticket export --to tickets/` writes markdown mirrors per [Q-001](#q-001-markdown-mirror-for-tickets); (3) a hook on `pql decisions sync` or `pql ticket status` auto-exports. The gitignore pattern should stay as-is (`.pql/` ignored, caches never committed); the backup is a separate committed artifact, not the DB file itself. The trigger question is whether this is manual (`pql plan export`), semi-automatic (pre-commit hook), or event-driven (on every write).
