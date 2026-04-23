# Open Questions

---

### Q-1: Markdown mirror for tickets
- **Status:** Open
- **Question:** When does `pql ticket export` become automatic? On every mutation, or on explicit command only?
- **Context:** Ticket source of truth is SQLite for now. Markdown mirror (`tickets/T-NNN.md` auto-written) is the long-term answer but the merge-conflict story isn't solved.

### Q-2: FTS for ticket/decision search
- **Status:** Open
- **Question:** SQLite FTS5 or external index for `pql ticket search` and `pql plan search`?
- **Context:** FTS5 is built-in, zero deps, good enough for N < 10k records. Currently deferred.

### Q-3: Multi-user planning DB
- **Status:** Open
- **Question:** If we want team-shared planning state, what's the locking story?
- **Context:** Currently single-user-per-DB. Network FS locking is fragile. Probably out of scope until the markdown mirror lands.

### Q-4: Validator strictness
- **Status:** Open
- **Question:** How strict should `pql decisions validate` be? Missing fields = error or warning?
- **Context:** The parser is permissive (missing Date: doesn't fail). The validate command exits non-zero only on duplicate IDs, empty titles, and broken refs.

### Q-5: Body access in the DSL
- **Status:** Open
- **Question:** `body` is reserved as an array column but requires FTS5. When does it ship, and what's the syntax?
- **Context:** The compiler returns a clear error for bare `body` refs. FTS5 is opt-in via `fts: true` in config. The natural syntax would be `'term' IN body` compiling to an FTS5 MATCH, but the virtual table isn't created unless the user opts in.

### Q-6: Outlink target normalization
- **Status:** Open
- **Question:** The links table stores raw wikilink targets (e.g. `brief`, not `sessions/.../brief.md`). Should the extractor normalize to full paths at index time?
- **Context:** Inlinks in the DSL work around this with a three-way OR (exact match, append .md, LIKE basename). Normalizing at index time would simplify the join but requires Obsidian-style shortest-unambiguous-path resolution. The pragmatic workaround works; the question is long-term correctness.

### Q-7: Code-aware extractor
- **Status:** Open
- **Question:** When does tree-sitter land? What languages first?
- **Context:** The extractor registry is designed for it — register by file pattern, produce structured output. No changes to store, connect, or query. The open question is priority and whether the tree-sitter cgo dependency is acceptable.

### Q-8: Occasional pql.db backups into git
- **Status:** Resolved → [D-13](architecture.md#d-13-plan-export-and-plan-import)
- **Question:** How should users snapshot planning state (pql.db) into version control? What events trigger it, and what's the artifact shape?
- **Context:** Resolved: `pql plan export` and `pql plan import` with a committed JSON artifact. Users wire the trigger (pre-push hook, sprint skill, manual) to their own workflow.
