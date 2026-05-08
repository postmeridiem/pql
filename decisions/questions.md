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

### Q-9: Multi-user ticket ID distribution
- **Status:** Open
- **Question:** How do multiple contributors create tickets without ID collisions? Current `T-<n>` sequential IDs assume a single writer.
- **Context:** Two agents or contributors independently creating T-5 is a data integrity problem no merge strategy can fix — orthogonal to replication (D-15 through D-18) which solves how state is *synchronised* but not how IDs are *minted*. The replication design intentionally left this open. Surveyed approaches:
  - **Earliest-wins drift with aliases.** Two replicas both mint T-91; consolidate keeps the earliest by `(commit-time, uid)` and renumbers the loser, recording a redirect in a `ref_aliases` table so old markdown references still resolve. Correct at any N but accumulates churn — drifts of drifts get noisy by N>3.
  - **Replica-namespaced bare-unless-ambiguous (git-branch-style).** Each replica freely mints `T-N` from its own counter; rows carry a `replica` column. Display logic shows bare `T-91` when unique, `alice/T-91` / `bob/T-91` when ambiguous. No drift, no fake authority — collisions coexist visibly until a human renames. Closest to how Lotus Notes solved this in 1989: the user-visible name is a label, not a key. UX cost: prefixed forms appear during collisions.
  - **Reservation + push-time canonicalisation.** `pql ticket new` mints `<replica>-r<n>` reservations; pre-push hook redeems against the existing canonical state, allocating the next free `T-N`. Whoever pushes first wins canonical numbering; later pushers rebase onto the new baseline. Cost: tentative refs in daily UX until push, plus markdown-rewrite churn for canonicalised names.
  - **Local master marker + atomic counter primitive.** One clone holds a gitignored `.pql/master` file designating it as canonicaliser. Generic mechanism: any caller asks for `+1` on a named counter, master returns and persists. Tentative IDs on non-master clones stored as pointers in a "bounce-link" table for later reconciliation. Atomic-counter primitive isn't ticket-specific — could serve any future sequential namespace.
  - **CI-hosted master.** Same atomic-counter + bounce-link pattern, but the master lives in GitHub Actions / Bitbucket Pipelines. Pipeline runs on push, redeems pending requests, commits canonical state back. No clone is anointed; the CI workflow is the authority.
  
  The trade-off is *where* friction lives: daily UX (reservations), display ambiguity (replica-namespaced), occasional renames (drift), infrastructure (CI master), or per-clone config (local master). All approaches compose with D-15's changelog mechanism unchanged. Related to [Q-3](#q-3-multi-user-planning-db).

### Q-10: Soft-delete stub retention and purge
- **Status:** Open
- **Question:** When and how do we purge soft-delete stubs (rows with `deleted_at` set) from the planning store and the changelog?
- **Context:** Surfaced during the changelog-replication design (`pql-plan-replication.md`). Replication requires soft deletes via a `deleted_at` column — without stubs, replicas would re-create deleted rows on replay, mirroring Lotus Notes' "deletion stub" mechanism. Stubs accumulate indefinitely without a purge step. Candidate mechanism: a separate set of changelog files (e.g. `.pql/changelog/<table>/purge-<window>.sql`) carrying explicit `DELETE` statements for stubs older than ~90 days, applied as a periodic GC pass. Open: retention threshold (90 days vs longer/shorter), mechanism (separate purge files vs DELETE lines inside the regular changelog vs consolidate-time GC), and how to ensure all replicas converge on the same purge decisions without re-resurrecting recently-deleted rows.
