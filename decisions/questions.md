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
