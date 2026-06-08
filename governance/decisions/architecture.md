# Architecture Decisions

Core design constraints for pql.

---

### D-1: No vector retrieval
- **Date:** 2026-04-19
- **Decision:** Retrieval is built from cheap, inspectable, structured signals — textual match, structural centrality, path proximity, recency, co-occurrence. No vector index.
- **Rationale:** The available embedded options are not tier-one infrastructure; the tier-one options are not embeddable as a single static binary. Shipping vectors means shipping a second store — new consistency problem, new lifecycle, new failure mode. The class of queries where vectors beat structured retrieval is narrower than the ecosystem narrative suggests on code-and-prose repos where identifiers, paths, and structure carry most of the signal. The absence is not a gap — it is a constraint that forces the ranker to do its job properly with the signals genuinely available.
- **Cost:** Some paraphrase queries will miss. Users reach for ripgrep when they need raw text recall. The ranker is the product; weight tuning and signal coverage are first-class engineering.
- **Raised by:** Design philosophy founding decision.
- **Revisit when:** (a) a specific class of agent task is shown, with examples, to fail on structured retrieval and succeed on semantic; AND (b) a pure-Go embedded vector store reaches tier-one quality. Either alone is insufficient.

### D-2: Intent-level surface for agents, primitives as escape hatch
- **Date:** 2026-04-19
- **Decision:** Expose intent-level commands (`pql related`, `pql search`, `pql context`) as the primary surface. Each intent does internal candidate generation, ranking, and bundle assembly. The DSL and primitive subcommands are the escape hatch for exact queries. A global `--flat-search` flag reduces any intent to its primitive layer for one invocation.
- **Rationale:** Each agent tool call is a permission event (chains erode trust), a round trip (the binary's internal queries are cheaper), and a drift opportunity (composition inside the binary is deterministic). The correct shape is one tool call per intent, not N chained primitives. The intent layer must not import the CLI layer (`internal/intent/` consumer-agnostic) to keep future MCP consumers cheap.
- **Cost:** New capabilities require new intents, not new primitives. Two files per feature (core + CLI adapter). If agents reach for the escape hatch frequently, the intent surface is wrong and should be revisited — not supplemented with more primitives.
- **Raised by:** Design philosophy founding decision.

### D-3: Two SQLite stores — index.db (cache) and pql.db (state)
- **Date:** 2026-04-22
- **Decision:** `<vault>/.pql/index.db` is a pure cache — drop and rebuild on schema mismatch. `<vault>/.pql/pql.db` is user-authored state (decisions, tickets) — forward-only migrations. Both live in `<vault>/.pql/` alongside `config.yaml`. The read-only-vault fallback applies to index.db; pql.db writes to a read-only vault fail cleanly.
- **Rationale:** One store fails both tests: as a cache it's too durable, as user state it's too fragile. The CLI-today / MCPs-tomorrow split reinforces this: a query MCP needs only index.db, a planning MCP needs only pql.db. The skill install lock sits outside SQLite so the cache invariant survived; planning cannot — status transitions, ticket history, and the D-record ↔ ticket join graph are user-authored state that must survive index rebuilds.
- **Cost:** Two files to manage. pql.db must survive upgrades; uninstalling pql leaves it in place.
- **Raised by:** Planning subcommands feature design.
- **Amendment (2026-05-31):** The "forward-only migrations" phrase predates [D-19](#d-19-no-alter-table--schema-lives-in-create-statements-only), which removed the migration runner — pql.db is currently schema-create-if-missing and regenerated from the changelog, with no in-place migrations. Forward migrations return as a planned step at third-party distribution (see D-19's amendment + T-44). Treat D-19 as the current authority on how pql.db evolves; the index.db-vs-pql.db split this record establishes is unchanged.

### D-4: Consumer-agnostic core
- **Date:** 2026-04-22
- **Decision:** `internal/intent/`, `internal/query/`, and `internal/planning/` must not import `internal/cli/`. CLI today, MCPs (plural) tomorrow. Every consumer is an adapter.
- **Rationale:** A query-surface MCP and a planning-surface MCP are different scopes, different permissions, different audiences. Coupling them to cobra blocks future consumers.
- **Cost:** CLI wiring is separate from core logic. Two files per feature (core + CLI adapter).
- **Raised by:** Architecture review during planning subcommands.

### D-5: CLI-first, not MCP
- **Date:** 2026-04-22
- **Decision:** Claude talks to pql via `Bash(pql *)`. No MCP server. The output contract (stdout JSON, stderr diagnostics, distinguished exit codes) is what makes it safe for agents.
- **Rationale:** A static binary on PATH is the lowest-friction surface for Claude Code: one allow rule in settings.json. No daemon, no network, no permission elevation.
- **Cost:** If an MCP-only integration becomes compelling later, nothing precludes adding one that shells out to the same CLI.
- **Raised by:** Project founding decision.

### D-6: In-house migration runner, not golang-migrate or goose
- **Date:** 2026-04-22
- **Status:** Superseded by [D-19](#d-19-no-alter-table--schema-lives-in-create-statements-only)
- **Decision:** pql.db migrations use a ~50-line in-house runner in `internal/planning/schema.go`. A `schema_migrations` table tracks applied versions; each migration runs in a transaction.
- **Rationale:** The migration surface is tiny (one DB, forward-only, single-user). An external dependency adds a build dep, a CLI tool, and a migration-file convention for a problem that's ~50 lines of Go. The runner uses `CREATE TABLE IF NOT EXISTS` so it's compatible with databases created by the Python stopgap.
- **Cost:** No rollback support. If a migration is wrong, the fix is a new forward migration. Acceptable for the scale.
- **Raised by:** Resolved P-Q-3 during planning implementation.

### D-7: Text IDs as primary keys
- **Date:** 2026-04-22
- **Decision:** Decision, question, and ticket IDs are TEXT primary keys (e.g. `D-<n>`, `Q-<n>`, `T-<n>`), not auto-incrementing integers.
- **Rationale:** Easier to preserve identity across imports. No translation layer between markdown and SQLite. IDs match what humans type and what the parser extracts verbatim.
- **Cost:** Sequential ID generation requires a MAX query. Acceptable at the expected scale.
- **Raised by:** Planning schema design.

### D-8: Ticket source of truth is SQLite, not markdown
- **Date:** 2026-04-22
- **Decision:** Tickets live in pql.db only. No automatic markdown mirror. `pql ticket export` is the half-step toward a mirror when the merge-conflict story is solved.
- **Rationale:** Shipping tickets without the mirror lets us deliver the feature now. The markdown mirror (auto-writing `tickets/T-NNN.md` on mutation) requires solving concurrent-edit conflicts, which is a separate problem.
- **Cost:** Tickets are invisible to git until explicitly exported. Acceptable for single-user workflows.
- **Raised by:** Planning locked decision. See [Q-1](../questions/architecture.md#q-1-markdown-mirror-for-tickets).
- **Amendment (2026-05-31):** `pql ticket export` was never built, and the merge-conflict problem this record was waiting on was since solved by the changelog replication design ([D-15](#d-15-replication-via-per-table-monthly-sql-changelog-files)/[D-16](#d-16-inline-lww-guards-plus-content-hash-plus-canonical-row-columns)) — tickets now travel in git via `.pql/changelog/`, not a markdown mirror. The load-bearing decision (tickets live in pql.db, no automatic `tickets/T-NNN.md` mirror) still holds; whether a markdown mirror is wanted at all is the live question (Q-1).

### D-9: One combined skill, not separate query/plan skills
- **Date:** 2026-04-22
- **Decision:** One `pql` skill with two sections (query + planning), not separate `pql-query` and `pql-plan` skills.
- **Rationale:** Every skill's metadata sits in context permanently (~100 words each). Two skills double the metadata cost for one binary. A router skill to dispatch between them is a signal the split was wrong. The skill-create guidance says "multi-action wrappers can use the domain name alone" — pql is the domain.
- **Cost:** The skill body is longer (~170 lines). Still well under the 500-line limit.
- **Raised by:** Skill design discussion.

### D-10: State machine for ticket status transitions
- **Date:** 2026-04-22
- **Status:** Superseded by [D-14](#d-14-no-status-transition-enforcement-in-pql)
- **Decision:** Ticket status follows a directed graph: backlog→ready→in_progress→review→done, with shortcuts (in_progress→done, any→cancelled) and reverse paths (done→in_progress, cancelled→backlog). Transitions outside the graph are rejected.
- **Rationale:** Prevents nonsensical jumps (backlog→done) that make ticket history meaningless. The allowed transitions match common kanban workflows.
- **Cost:** Users can't force arbitrary transitions. If a new workflow needs a different graph, the transitions map in `repo/tickets.go` is the single edit point.
- **Raised by:** Ticket subcommands implementation.

### D-11: Max-normalization per signal before weighting
- **Date:** 2026-04-22
- **Decision:** Each signal's raw scores are divided by the maximum absolute value in the candidate batch before applying weights. This puts all signals on [0, 1].
- **Rationale:** Without normalization, signals with larger raw ranges (e.g. centrality counts of 10+ vs path_proximity of 0–1) dominate the weighted sum regardless of weight tuning. Max-norm is the simplest approach that handles heterogeneous scales.
- **Cost:** Normalization is relative to the candidate batch — the same file can score differently in different queries. Acceptable because rankings are always relative.
- **Raised by:** Enrichment layer implementation.

### D-12: Clean version strings — just the number
- **Date:** 2026-04-22
- **Decision:** `pql version` outputs only the version number from `project.yaml` (e.g. `1.0.0`). No git SHA, dirty markers, or `-dev` suffixes.
- **Rationale:** Build metadata in the version string is confusing to users and creates mismatches between the installed binary and the project manifest.
- **Cost:** No way to tell which exact commit a binary was built from via `pql version` alone. `pql version --build-info` still includes the commit hash for debugging.
- **Raised by:** User feedback after clide integration.

### D-13: plan export and plan import for versioned planning snapshots
- **Date:** 2026-04-22
- **Status:** Superseded by [D-15](#d-15-replication-via-per-table-monthly-sql-changelog-files)
- **Decision:** `pql plan export` writes all planning state (decisions + tickets + refs + deps + labels + history) to a single JSON file at the vault root (default `pql-plan.json`). `pql plan import` restores from that file into pql.db. The artifact is committed to git; pql.db stays gitignored.
- **Rationale:** Planning state is valuable enough to version but SQLite files don't belong in git. A JSON export is diffable, portable, and merge-friendly. The trigger is the user's choice — pre-push hook, sprint skill, manual — pql provides the verbs, not the policy.
- **Cost:** Two representations of the same data (pql.db + export file) can drift. The export is a snapshot, not a live mirror. Users who want the export current must run `pql plan export` before committing.
- **Raised by:** Resolved [Q-8](../questions/architecture.md#q-8-occasional-pqldb-backups-into-git).

### D-14: No status transition enforcement in pql
- **Date:** 2026-04-24
- **Supersedes:** [D-10](#d-10-state-machine-for-ticket-status-transitions)
- **Decision:** pql validates that the target status is a known value but does not enforce a transition graph. Any valid status can move to any other valid status. Callers (scripts, IDE plugins, agents) layer their own workflow rules on top.
- **Rationale:** pql is a local tool — the caller is better positioned to enforce workflow-specific transitions. Hardcoding a state machine in the core blocks legitimate use cases (e.g. an agent that moves backlog→done after batch processing). The audit log records every transition regardless.
- **Cost:** Nothing prevents nonsensical transitions at the pql level. Callers that need guardrails must implement them.
- **Raised by:** User feedback during planning usage.

### D-15: Replication via per-table monthly SQL changelog files
- **Date:** 2026-05-08
- **Supersedes:** [D-13](#d-13-plan-export-and-plan-import-for-versioned-planning-snapshots)
- **Decision:** Planning state is replicated through a committed directory `.pql/changelog/<table>/`, containing two file shapes that share a naming convention so lexicographic sort gives the correct replay order:
  - `0000-schema.sql`, `0001-schema-<slug>.sql`, … — schema files. The initial `0000-schema.sql` (CREATE TABLE IF NOT EXISTS) is installed by `pql init`. Later migrations land as additional numbered schema files committed when they're authored, so schema evolution travels in the changelog alongside data.
  - `<YYYY-MM>.sql` — append-only data files, one per month of activity, containing SQL upserts sorted within the file by `(updated_at, hash)`.
  Replay reads files in lexicographic order: zero-prefixed schema files first, then year-prefixed data files chronologically. `pql.db` is a derived cache, rebuilt from the changelog.
- **Rationale:** A single mutable JSON snapshot (D-13) produces guaranteed merge conflicts because git's text merger can't distinguish rows when the file changes wholesale. Per-table monthly SQL files cooperate with git's line-based merger: distinct `updated_at` values land at distinct line positions, so concurrent commits auto-merge as line additions. Per-table partitioning keeps each file strictly append-only — cross-table sort would re-segment files mid-update. Monthly buckets keep individual files bounded. Schema files in the same directory make the changelog self-describing (any SQLite tool can replay it without pql installed) and let schema migrations propagate through the same git-merge mechanism as data — the in-house migration runner (D-6) generates and tracks these files instead of carrying SQL inline in Go.
- **Cost:** On-disk format is SQLite-flavored; cross-store portability is reduced. Decisions still source from markdown, not changelog (preserves D-8). Long-lived projects accumulate monthly files; a future rollup pass may be needed at scale (deferred). Existing `pql-plan.json` artifacts must be migrated on first export against an upgraded repo.
- **Raised by:** Multi-machine merge conflict on `pql-plan.json` during clide T-91/T-92/T-93 work — see archived design at `pql-plan-replication.md` (retired in favour of these records).
- **Amendment (2026-06-03):** "`pql.db` is a derived cache, rebuilt from the changelog" is now true at every instant, not just after an export: ticket mutations write through to `.pql/changelog/` synchronously ([D-23](#d-23-write-through-ticket-mutations-to-the-changelog)). The old `pql-plan.json` snapshot is fully retired — `pql plan export` only ever writes the changelog.

### D-16: Inline LWW guards plus content hash plus canonical row columns
- **Date:** 2026-05-08
- **Decision:** Every planning row carries `created_at`, `updated_at`, and `hash` columns. `hash = H(canonical(row except hash))` — deterministic across replicas, set at write-finalisation, immutable per write. Every SQL upsert line emitted to the changelog includes an inline last-write-wins guard: `WHERE excluded.updated_at > target.updated_at OR (excluded.updated_at = target.updated_at AND excluded.hash > target.hash)`. Canonicalisation rules — column order, value formatting, NULL representation — are version-stamped (`canonical_version`) so changes are detectable.
- **Rationale:** Inline guards make every emitted line idempotent and order-free under replay — a line can be replayed any number of times against any starting state and converge to the same result. This collapses the conflict surface to git itself: any divergence between replicas resolves at the row level via the SQL guard, regardless of how the changelog files got merged. Hash earns its keep three ways: content integrity (corruption detection), sort tiebreaker for same-`updated_at` rows (deterministic across replicas), and LWW tiebreaker at the millisecond-collision boundary (unbiased — no replica systematically wins).
- **Cost:** Every table grows three columns. Canonicalisation discipline becomes load-bearing — drift across replicas (different value formatting, schema mismatches, library upgrades) breaks hash equality and replay convergence. Wall-clock skew between machines can flip LWW outcomes within the skew window; accepted as a known limitation since sub-millisecond collisions are inherently human-ambiguous and require communication, not replication, to resolve.
- **Raised by:** Replication design — the property that "every line is self-protecting" is what makes the changelog robust under concurrent edits.

### D-17: Soft deletes via deleted_at column
- **Date:** 2026-05-08
- **Decision:** Deletions in planning tables are soft. Every table has a `deleted_at` column; deleting a row sets it to a timestamp. Read queries filter `WHERE deleted_at IS NULL` by default. The "stub" row remains in the database and the changelog so other replicas know the deletion is intentional rather than missing.
- **Rationale:** Without stubs, a replica that hasn't seen the deletion would recreate the row on replay (no row → INSERT). Lotus Notes solved this in 1989 with deletion stubs; the same primitive applies to git-backed replication. Hard deletes can't be expressed in an upsert-only changelog without breaking the "every line is idempotent" invariant, which D-16 depends on.
- **Cost:** Every table grows a `deleted_at` column; every read query needs the filter. Stubs accumulate indefinitely without a purge mechanism — long-running projects need a periodic GC pass. Purge mechanism, retention threshold, and how to ensure replicas converge on purge decisions are deferred to [Q-10](../questions/architecture.md#q-10-soft-delete-stub-retention-and-purge).
- **Raised by:** Replication design — emerged from the need to express deletion in the changelog without breaking idempotent replay.

### D-18: Git lifecycle hook architecture for replication
- **Date:** 2026-05-08
- **Decision:** `pql init` installs four git hooks plus a `.gitattributes` rule covering the replication lifecycle:
  - `pre-commit` → `pql plan export` (silent, fast). Materialises dirty `pql.db` rows into changelog files and stages them.
  - `post-merge` → `pql plan import` (silent if quick). Replays incoming changelog lines into `pql.db` via inline-guarded upserts.
  - `post-checkout` (branch switch only, `$3 == 1`) → `pql plan rebuild` with a visible `echo "rebuilding pql database..."`. Drops planning tables and replays the changelog from scratch.
  - `post-rewrite` (rebase, amend) → `pql plan rebuild`, also visible.
  - `.gitattributes` adds `merge=union` for `.pql/changelog/*.sql` as belt-and-braces for same-line conflicts.
- **Rationale:** Hooks below the client layer catch all git operations — IDE pulls, raw CLI, GUI tools — without requiring a `pql sync` wrapper command (rejected for scope drift into git porcelain). post-checkout and post-rewrite need full rebuild because LWW-guarded replay can only INSERT/UPDATE; a row that exists on the previous branch but not the new one would linger in `pql.db` after a checkout if only an incremental import ran. Synchronous visible rebuild matches user mental model: people expect cleanup after these operations and prefer a clear cause-and-effect message over hidden lazy execution that surfaces as random slowness on a later command.
- **Cost:** Four hooks instead of one in the install footprint. Users who disable hooks (`--no-verify`, unset `core.hooksPath`) lose the guarantees and operate on stale or divergent state — accepted, same posture as the existing pre-push lint gate. Branch switches incur a synchronous rebuild (bounded by changelog size, but visible in `git checkout` latency).
- **Raised by:** Replication design — settled after iteration through `pql sync` (rejected for scope drift), lazy-on-read (rejected for hidden slowness on hot paths like clide's once-per-minute polling), and flag-then-defer rebuild (rejected for hidden process / unexpected slowness).
- **Amendment (2026-06-03):** The `pre-commit` clause changes under [D-23](#d-23-write-through-ticket-mutations-to-the-changelog): mutations materialise into `.pql/changelog/` at mutation time (write-through), so `pre-commit` no longer relies on a commit-time `pql plan export` to materialise dirty rows — it runs a catch-up export and then `git add`s the whole `.pql/changelog/` tree so the already-written files land in the commit. This also makes the `post-checkout`/`post-rewrite` rebuild non-destructive of un-committed local work, since the changelog on disk is always current.

### D-19: No ALTER TABLE — schema lives in CREATE statements only
- **Date:** 2026-05-08
- **Supersedes:** [D-6](#d-6-in-house-migration-runner-not-golang-migrate-or-goose)
- **Decision:** pql.db schema is defined entirely by `CREATE TABLE IF NOT EXISTS` statements in one place (`internal/planning/schema.go`). When the schema needs to change, edit the CREATE statements and bump `CanonicalVersion`. Existing pql.db files are never altered in place — they are regenerated from the changelog (D-15) or from a `pql-plan.json` import. No `schema_migrations` table, no migration runner, no ALTER TABLE statements ever.
- **Rationale:** pql.db is a derived cache of authoritative state living in the changelog (D-3 + D-15). Carrying a migration framework — version tracking, ALTER TABLE deltas, backfill goroutines, transient-state tests — to evolve a regenerable cache is unwarranted complexity. The codebase should optimise for clean installs, not for in-place upgrades. Schema changes propagate the same way data changes propagate: through the changelog, with consumers rebuilding their local cache. Open path becomes "create schema if missing, verify columns match expectations, refuse to operate if not."
- **Cost:** Any pql.db built under an earlier schema is non-recoverable in place. Recovery is `rm .pql/pql.db && pql plan import` (or `pql plan rebuild` once T-21 lands). Migration-style ALTER TABLE deltas would have allowed in-place upgrades; this decision trades that affordance for a cleaner codebase.
- **Raised by:** T-18 implementation review. Initial plan layered an ALTER TABLE migration v2 on top of v1 to add `deleted_at`; user pushback flagged this as unnecessary clutter for a regenerable cache.
- **Amendment (2026-05-31):** "No migration runner *ever*" is too strong — it holds only while pql is single-author and pre-distribution, where pql.db is a regenerable cache (rm + rebuild from the changelog is fine recovery). Once pql is distributed to third parties or used beyond the author, an in-place forward-migration path becomes necessary: external users can't be told to `rm .pql/pql.db` on every schema change, and their pql.db may hold state not yet in any shared changelog. The no-migration stance stands **for now**; introducing forward migrations (the in-house runner [D-6](#d-6-in-house-migration-runner-not-golang-migrate-or-goose) sketched, or equivalent) is gated on that distribution boundary and tracked by T-44.

### D-20: Decision implementation tracked via initiative tickets, no coverage report
- **Date:** 2026-05-11
- **Decision:** Confirmed D-records that propose implementation work record that work as `initiative`-type tickets with `decision_ref` pointing back to the D-record. Reverse linkage is surfaced via `pql decisions show <id> --with-tickets`. There is no separate "coverage" report or class column on the decisions table — `pql decisions coverage` is removed.
- **Rationale:** The coverage report was answering an ambiguous question ("confirmed decisions without a linked ticket"). Founding policies, structural standards, and informational records legitimately don't need tickets — flagging them as gaps produced more noise than signal. A "coverage class" column to distinguish them duplicates the ticket system: a proposal-in-ready-state is exactly what an initiative ticket already expresses, with richer state (priority, blockers, children, history, assignees) than any label could carry. Filing an `initiative` for D-records that propose change and leaving founding records ticket-free is the cleaner separation. Status checks against a decision flow through the existing ticket queries — `pql decisions show D-N --with-tickets` is the canonical "what's the implementation state of D-N" lookup.
- **Cost:** Authors must remember to file an initiative for D-records that propose work. The system can't distinguish "founding policy with no tickets" from "proposal with no tickets filed yet" — both look identical in the database. Convention carries the distinction; tooling doesn't enforce it. A future `pql ticket new initiative` invoked from inside a D-record context could automate the linkage if the friction proves real.
- **Raised by:** T-15 implementation review. Initial work added a `Coverage:` / `Class:` markdown field to filter founding decisions from the report; user pushback flagged this as duplicating the ticket-type system without earning its place.

### D-21: DQR layout — `governance/` parent with per-type subdirectories
- **Date:** 2026-05-11
- **Decision:** Planning records live under a configurable parent directory (default `governance/`), organised into three per-type subdirectories:
  - `governance/decisions/<domain>.md` — D-records
  - `governance/questions/<domain>.md` — Q-records
  - `governance/rejected/<domain>.md` — R-records

  The parent name is configurable via `dqr_dir` in `.pql/config.yaml` so existing repos can keep `decisions/` (or any other name) by overriding. Domain is inferred from the filename stem; record type is inferred from the immediate parent subdirectory.

  `pql init` plants the parent + three type subdirectories empty, plus a `governance/README.md` whose prologue describes the system (mechanism, record types, the link from D-records to initiative tickets per [D-20](#d-20-decision-implementation-tracked-via-initiative-tickets-no-coverage-report)) and recommends a starter set of domains:

  - **architecture** — structural commitments
  - **process** — team workflow
  - **design** — user-facing surface
  - **coding-conventions** — team-internal code shape
  - **testing** — quality strategy

  Plus a "you might also want" list (accessibility, security, licensing, documentation, deployment, performance) as project-specific add-ons. `pql init` does NOT pre-create domain files; authors create them as records land.

  The README's records section is delimited by a `<!-- pql:records -->` marker and regenerated by `pql decisions sync` from current pql.db state. The prologue is human-edited and never overwritten.

  `pql decisions validate` and `pql decisions sync` enforce convention by default: filename style (lowercase, hyphenated), one record type per file, file-vs-subdirectory consistency, stale-README detection. The flag `--no-style` opts out of warnings; structural errors (broken refs, malformed records) remain unconditional.

  **Amendment (2026-05-31):** The domain-stem prefix-collision check (two stems in one subdirectory where one is a prefix of the other, e.g. `arch.md` vs `architecture.md`) and the questions↔decisions domain-pairing check ship as style **warnings**, not errors. The original sketch (via T-36) framed the collision as a hard error; in practice a fuzzy prefix heuristic must not fail `sync`/`validate` (exit 65) on a legitimate pair like `test`/`testing`. Both are suppressible via `--no-style`; only structural problems (broken refs, malformed records) remain unconditional errors. (T-43)

- **Rationale:** The previous flat layout had asymmetric prefixes (`questions-X.md`, `rejected.md`, plain `X.md` for decisions) and no enforcement, creating a "filename wilderness" risk across multiple consumers. Subdirectories are symmetric — each record type owns its namespace, `ls governance/questions/` is the natural way to see all Q-records, and per-domain files inside each type scale to many records without one giant file.

  `governance/` as the parent reads as "rules of the road" with at least the LLM-priority of `decisions/` (the previous default), avoids the `decisions/decisions/` repetition that a simpler rename would have caused, and creates room for future record types (`governance/runbooks/`, `governance/postmortems/`) without re-rooting.

  Default-on style warnings serve as a teachable moment for new contributors and LLM agents — drift is visible on every sync rather than only when someone explicitly validates. The recommended-but-not-created starter domains in the README give authors a pattern to follow without pre-creating directories that risk cargo-cult filling.

- **Cost:** Existing repos using `decisions/` must migrate (move files into subdirectories, rename parent, update `.gitignore`). One-time helper migrates this repo as part of the rollout; downstream consumers (clide, settled-reach) either migrate similarly or override `dqr_dir` to point at their existing layout. Style warnings are visible by default, which some authors may find noisy until they conform — the `--no-style` knob exists for that case.

- **Raised by:** DQR layout discussion following T-15's pivot. Multiple consumers (this repo, clide, settled-reach) each invented their own filing convention; capturing the shared shape prevents further divergence.

### D-22: Zero results returns exit 0, no distinct no-match code
- **Date:** 2026-05-25
- **Decision:** A query that matches nothing exits `0` and emits an empty array `[]` on stdout (JSONL emits nothing). There is no dedicated "no match" exit code. The exit-code set is `0` (success, including zero results) plus the error range `64`/`65`/`66`/`69`/`70`. The former exit `2` (`NoMatch`) is retired — `internal/diag` no longer defines it and no subcommand returns it.
- **Rationale:** pql is a JSON tool whose emptiness is already fully represented in the data channel (`[]`). A separate exit code for "zero rows" is redundant with that data and actively hostile to the near-universal "non-zero means failure" convention that callers assume — `set -e`, `subprocess.check_call`, CI steps, and agent harnesses all read exit `2` as a failure. The output contract's own goal ([D-5](#d-5-cli-first-not-mcp): "safe for agents", a zero-row query "must never look like a bug") is better served by collapsing into `0`: success is success, the empty result lives in the data, and errors stay cleanly in the `64`–`70` range with a stderr diagnostic. This matches the prevailing JSON-CLI norm (`gh`, `aws`, `jq` all exit `0` with an empty array). The grep-style `0/1/2` convention pql originally borrowed fits line tools where the exit code *is* the interface, not a structured-output tool where the body carries the answer.
- **Cost:** Breaking change to the documented contract — callers that branched on exit `2` for a cheap shell-level "did it match?" must now inspect the data (empty `[]` / length). A compatibility note ships in CHANGELOG. No `--fail-on-empty` escape hatch is provided yet; it can be added if a genuine shell-branching need surfaces. `ticket refine next` on an exhausted queue now emits an envelope `{"refinement":{"remaining":0,…}}` at exit `0` instead of bailing with exit `2` and no body.
- **Raised by:** Downstream users reporting that empty results were being flagged as errors because exit `2` is non-zero.

### D-23: Write-through ticket mutations to the changelog
- **Date:** 2026-06-03
- **Refines:** [D-15](#d-15-replication-via-per-table-monthly-sql-changelog-files), [D-18](#d-18-git-lifecycle-hook-architecture-for-replication)
- **Decision:** Every ticket mutation (`pql ticket new|append|status|assign|setparent|block|unblock|team|label|refine write`) writes through to `.pql/changelog/` synchronously. After the write to `pql.db` succeeds, the command appends the affected rows to the changelog via the existing incremental exporter (`changelog.Export`, which selects rows with `updated_at > last_export_marker` and advances the marker), so exactly the just-touched rows are emitted. The changelog is the synchronous log of record; `pql plan export` becomes a manual catch-up/reconcile that is normally a no-op, and the `pre-commit` hook stages the whole `.pql/changelog/` tree rather than relying on a commit-time export. Decisions stay markdown-sourced ([D-8](#d-8-ticket-source-of-truth-is-sqlite-not-markdown)) and are not written through. The write-through call lives in the CLI adapter (it composes `changelog`, which imports `repo`, so it cannot live in `repo` without a cycle); any future writing consumer (a planning-surface MCP) must route mutations through the same helper.
- **Rationale:** Before this, mutations landed only in the gitignored `pql.db`; the changelog was written by a separate batch step that in practice ran only at commit time. A routine `git checkout` fires `post-checkout` → `pql plan rebuild`, which truncates the replicated tables and replays the changelog **from disk** (D-18) — silently destroying any ticket created-but-not-committed. Write-through closes the hole at the source: because the changelog file exists on disk the moment a mutation completes, and git carries uncommitted/untracked `.pql/changelog/*.sql` across a checkout, rebuild replays the row back. Rebuild is thus non-destructive of local work without a divergence guard, a `--force` flag, or a merge-rebuild — the changelog simply is always current, making D-15's "pql.db is a derived cache" literally true at every instant rather than only after an export.
- **Cost:** Every ticket mutation now performs a small extra changelog scan-and-append (bounded by rows-since-marker, normally one row). A mutation that succeeds but whose write-through export fails leaves `pql.db` ahead of the changelog until the next mutation or a manual `pql plan export` catches up — surfaced as an error so it is not silent. The CLI is the only writer today, so the call is wired per-mutation-command; a regression test asserting each verb grows the changelog guards against a future consumer forgetting it. This supersedes the relevant clause of D-18 (`pre-commit` "materialises dirty rows"): materialisation happens at mutation time, and `pre-commit` only stages.
- **Raised by:** Live-session feedback (pql-improvements.md #1): an 18-ticket tree created via `pql ticket new` was silently erased by a `git checkout` that triggered `pql plan rebuild`. Write-through was chosen over a divergence guard as the principled fix.

### D-24: Ticket status set is configurable, modelled as four classes
- **Date:** 2026-06-08
- **Refines:** [D-14](#d-14-no-status-transition-enforcement-in-pql); references [D-10](#d-10-state-machine-for-ticket-status-transitions)
- **Decision:** The ticket status set is a per-vault vocabulary configured under `ticket_statuses` in `.pql/config.yaml`, not a fixed enum baked into the binary. Each status declares a `name`, optional `label`, and a `class` — one of `initial | active | review | terminal` — and exactly one status sets `is_default` (which must be class `initial`). Order is the list position. The planning engine reasons about *classes*, never literal names: the default/initial status, the terminal set (clears blockers; excluded from refine/unblocked), `next-review` (class `review`), and `what-next` (class `active`, then the most-advanced `initial` status as the "ready" lane; `review` excluded). The built-in default (`backlog, ready, in_progress, review, done, cancelled`) reproduces pql's historical behaviour exactly, so omitting the key changes nothing. `pql ticket statuslist` surfaces the resolved set with full metadata (`name, label, class, order, is_default, is_terminal`) so consumers (e.g. clide) adapt their UI without hardcoding. The SQL `CHECK(status IN (...))` enumeration is dropped (validation moves to Go, `planning.StatusSet`); the column stays `TEXT NOT NULL`. This does **not** reintroduce transition enforcement — D-14 holds: it adds a *vocabulary*, not a state machine; any configured status may follow any other.
- **Rationale:** Hardcoding both the status names and their meanings blocked projects with different workflows and forced clide to mirror pql's enum. Four classes is the smallest model that captures every role the engine actually keys off (which is default, which closes work, which is reviewable, which is actionable), so a custom vocabulary stays correct without per-name special-casing. It echoes the Jira status-category model (to-do / in-progress / done) with `review` split out because pql treats it distinctly. Validation in Go rather than a SQL `CHECK` is what makes the set editable with no schema migration, consistent with [D-19](#d-19-no-alter-table--schema-lives-in-create-statements-only).
- **Cost:** Dropping the `CHECK` is a tickets-table schema change. Per D-19 there is no in-place migration: an existing `pql.db` created under the old `CHECK` keeps that constraint and would reject a custom status until rebuilt (`rm .pql/pql.db && pql plan rebuild`). `CanonicalVersion` is deliberately **not** bumped — it governs hash projection (column order/values), which is unchanged; bumping it would needlessly invalidate every row hash. Config↔planning stay decoupled (config cannot import planning, which imports config), so the CLI carries a small adapter (`statusSetFromConfig`) and the class constants exist in both packages, kept in sync by docs + the adapter.
- **Raised by:** User request to make the status flow configurable in the repo and discoverable for clide UI.
