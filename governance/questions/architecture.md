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
- **Context:** The parser is permissive (missing Date: doesn't fail). The validate command exits non-zero only on duplicate IDs, empty titles, and broken refs. A separate **warnings** tier has since shipped (filename convention, subdir/heading mismatch, domain pairing/conflict — suppressible via `--no-style`, D-21/T-43), but the core question stands: should *missing fields* be an error or a warning? Still open.

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
- **Status:** Resolved → [D-13](../decisions/architecture.md#d-13-plan-export-and-plan-import-for-versioned-planning-snapshots)
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

  The trade-off is *where* friction lives: daily UX (reservations), display ambiguity (replica-namespaced), occasional renames (drift), infrastructure (CI master), or per-clone config (local master). All approaches compose with D-15's changelog mechanism unchanged. Related to [Q-3](#q-3-multi-user-planning-db). **Converged direction:** the issue-tracker-as-referee variant is developed in [Q-13](#q-13-githubgitea-issue-backend-for-id-claiming-and-one-way-mirror).

### Q-10: Soft-delete stub retention and purge
- **Status:** Open
- **Question:** When and how do we purge soft-delete stubs (rows with `deleted_at` set) from the planning store and the changelog?
- **Context:** Surfaced during the changelog-replication design (`pql-plan-replication.md`). Replication requires soft deletes via a `deleted_at` column — without stubs, replicas would re-create deleted rows on replay, mirroring Lotus Notes' "deletion stub" mechanism. Stubs accumulate indefinitely without a purge step. Candidate mechanism: a separate set of changelog files (e.g. `.pql/changelog/<table>/purge-<window>.sql`) carrying explicit `DELETE` statements for stubs older than ~90 days, applied as a periodic GC pass. Open: retention threshold (90 days vs longer/shorter), mechanism (separate purge files vs DELETE lines inside the regular changelog vs consolidate-time GC), and how to ensure all replicas converge on the same purge decisions without re-resurrecting recently-deleted rows.

### Q-11: Comment / discussion threads on tickets (and decisions?)
- **Status:** Open
- **Question:** Do tickets need an authored, free-form comment stream beyond `ticket append` + `ticket_history`, and what shape should it take? Do decisions get the same, or stay markdown-only?
- **Context:** Tickets are SQLite-sourced, so comments would be a new changelog-replicated table (`ticket_comments`: record_id, ticket_record_id, author, body, created_at, …) rehashed and LWW-guarded like the other replicated tables (D-15/16) — distinct from `ticket_history` (the field-change audit log) and richer than `ticket append` (which mutates the description in place). The value shows up mainly in multi-contributor repos (relates to [Q-3](#q-3-multi-user-planning-db) and [Q-9](#q-9-multi-user-ticket-id-distribution)). Decisions are markdown-sourced (D-8): a "comment" there is just a `## Discussion` section in the `.md`, so a separate DB store for decision comments would fight the source-of-truth model and is likely out of scope. Open: flat vs threaded; where author identity comes from (git ident vs config); how `ticket show` renders the stream; whether authored comments earn their keep over `append` + history for small teams; and the interaction with the eventual ticket markdown mirror ([Q-1](#q-1-markdown-mirror-for-tickets)).

### Q-12: External tracker (JIRA) as a pql ticket backend — proxy or separate tool?
- **Status:** Open (parked)
- **Question:** When work mandates an external tracker (JIRA), should pql *proxy* it — keep the pql interface, swap an external backend behind it — or should that live in a separate tool that merely shares pql's output contract?
- **Context:** Distinct from [Q-9](#q-9-multi-user-ticket-id-distribution) and the GitHub/Gitea idea, where the tracker is only an **ID allocator** and pql stays leading. Here the external system is the **source of truth** (work mandates JIRA), which inverts pql's local-first, changelog-replicated model (D-3, D-15/16) and "tickets' source of truth is SQLite" (D-8). Options: **(a) proxy inside pql** — ticket verbs map to JIRA REST, local DB becomes an optional cache; bloats scope against the narrow-scope invariant, breaks offline/changelog/two-store guarantees, and leaks through lossy feature-mapping. **(b) separate `jira` adapter/CLI** that emits pql's JSON output contract and verbs, with the consumer (clide) routing to `pql` for local repos and `jira` for work repos — keeps pql narrow and applies the consumer-agnostic principle one level up. The observation that pql's ticket verbs have clean JIRA equivalents argues for a shared *interface contract*, not a shared binary. Candidate governing boundary worth deciding alongside this: *integrations that keep pql local-first + changelog-leading may live in pql (an ID allocator does); integrations where an external system is the source of truth must be separate tools sharing the output contract.* A disposable `spike/jira-proxy` branch to pressure-test option (a) was proposed and deferred. Relates to the consumer-agnostic core invariant and D-8.

### Q-13: GitHub/Gitea issue backend for ID claiming and one-way mirror
- **Status:** Open (converged direction; not yet ticketed)
- **Question:** For multi-contributor repos, can an optional external issue tracker (GitHub/Gitea) coordinate friendly `T-NNN` labels and mirror tickets outward, while pql + the changelog stay fully leading on everything but the claim? A concrete instantiation of [Q-9](#q-9-multi-user-ticket-id-distribution).
- **Why now:** D-26's `record_id` made *structural* identity collision-proof, but left *friendly-label* coordination across contributors as a manual `pql ticket relabel` step. The clide repo (two contributors) keeps hitting exactly that residual friction — safe, but manual. This automates the label coordination D-26 deliberately left manual; it is not re-solving identity.
- **Activation:** strictly optional, per-repo (`.pql/config.yaml`), enabled only where multiple contributors warrant it. Online-only at create (first cut).
- **Anchor & label:**
  - `record_id` (ULID) stays the local anchor and additionally **stores the remote issue id** as a backlink (a small `external_ref` keyed by `record_id`, sibling to `ticket_idmap`). The local system consolidates on `record_id`.
  - `T-NNN` stays **pql-owned**: contiguous, clean, `relabel`-able. It is **never** the issue number — GitHub and Gitea share one number sequence across issues *and* PRs (and unrelated issues), so an issue-number-derived label would be polluted and non-contiguous.
- **Mirror:** one-way push only (no read-back of edits, no two-way sync — editing in the tracker UI is explicitly out of scope). Title `T-NNN: <title>`; body = description plus a readable block for fields the tracker can't model natively (parent, blockers, `decision_ref`); labels for `type:`/`priority:`/`status:`; open/closed driven by the terminal status class.
- **Mint flow (issue-first):**
  1. Mint `record_id` locally (free, ULID) so the issue carries it as a backlink from birth.
  2. **Create the remote issue** → obtain the atomic, globally-unique issue number. (Online requirement bites here; fail-fast if unreachable.)
  3. **T-NNN dance:** claim `max+1`, write it into the issue, verify against other pql issues; on contention the **lowest issue number keeps the contested label** and losers re-mint to the next free label. The issue number is never the label but acts as the deterministic **referee** — a globally-observable total order that makes label resolution race-free and convergent with no coordination, while `record_id` keeps the tickets distinct throughout. This is D-26's manual relabel-on-collision, made automatic.
  4. **Continue local creation:** write the `tickets` row, the `ticket_idmap` label, and store the issue number against `record_id`.
- **Caveat:** issue-first means a failure after step 2 can orphan a remote issue; GitHub/Gitea allow *close* but not clean *delete* via API → the rule is close-on-rollback, tolerate the rare stray.
- **Open points:** mutation-push timing (create must be online; later `status`/`assign`/`append` edits could be best-effort/queued vs holding the whole feature online-only); exact status→state/label mapping against the configurable status classes; body-metadata format (readable section vs invisible `<!-- pql: … -->` block vs none); auth/token handling (per-repo config, token via env, never stored); GitHub-vs-Gitea sequencing.
- **Relation:** concrete instantiation of [Q-9](#q-9-multi-user-ticket-id-distribution); builds on D-26 (`record_id`); distinct from [Q-12](#q-12-external-tracker-jira-as-a-pql-ticket-backend--proxy-or-separate-tool) (which is external-as-source-of-truth — this keeps pql leading). A writing consumer (e.g. a planning-surface MCP) must route mint/claim through the same helper, per the consumer-agnostic core.
