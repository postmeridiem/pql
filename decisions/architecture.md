# Architecture Decisions

Core design constraints for pql.

---

### D-1: No vector retrieval
- **Date:** 2026-04-19
- **Decision:** Retrieval is built from cheap, inspectable, structured signals — textual match, structural centrality, path proximity, recency, co-occurrence. No vector index.
- **Rationale:** The available embedded options are not tier-one infrastructure; the tier-one options are not embeddable as a single static binary. The class of queries where vectors beat structured retrieval is narrower than the ecosystem narrative suggests on code-and-prose repos. The absence forces the ranker to do its job properly with the signals genuinely available.
- **Cost:** Some paraphrase queries will miss. Users reach for ripgrep when they need raw text recall. If a specific repeatable failure mode emerges where structured signals can't compensate, revisit.
- **Raised by:** Design philosophy founding decision.

### D-2: Intent-level surface for agents, primitives as escape hatch
- **Date:** 2026-04-19
- **Decision:** Expose intent-level commands (`pql related`, `pql search`, `pql context`) as the primary surface. Each intent does internal candidate generation, ranking, and bundle assembly. The DSL and primitive subcommands are the escape hatch for exact queries. `--flat-search` reduces any intent to its primitive layer.
- **Rationale:** Each agent tool call is a permission event (chains erode trust), a round trip (the binary's internal queries are cheaper), and a drift opportunity (composition inside the binary is deterministic). The correct shape is one tool call per intent, not N chained primitives.
- **Cost:** New capabilities require new intents, not new primitives. If agents reach for the escape hatch frequently, the intent surface is wrong.
- **Raised by:** Design philosophy founding decision.

### D-3: Two SQLite stores — index.db (cache) and pql.db (state)
- **Date:** 2026-04-22
- **Decision:** `<vault>/.pql/index.db` is a pure cache — drop and rebuild on schema mismatch. `<vault>/.pql/pql.db` is user-authored state (decisions, tickets) — forward-only migrations.
- **Rationale:** One store fails both tests: as a cache it's too durable, as user state it's too fragile. The CLI-today / MCPs-tomorrow split reinforces this: a query MCP needs only index.db, a planning MCP needs only pql.db.
- **Cost:** Two files to manage. pql.db must survive upgrades; uninstalling pql leaves it in place.
- **Raised by:** Planning subcommands feature design.
- **Cross-reference:** Full rationale in [docs/adr/0003-pql-db-for-user-state.md](../docs/adr/0003-pql-db-for-user-state.md)

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
- **Raised by:** Planning locked decision. See [Q-1](#q-1-markdown-mirror-for-tickets).

### D-9: One combined skill, not separate query/plan skills
- **Date:** 2026-04-22
- **Decision:** One `pql` skill with two sections (query + planning), not separate `pql-query` and `pql-plan` skills.
- **Rationale:** Every skill's metadata sits in context permanently (~100 words each). Two skills double the metadata cost for one binary. A router skill to dispatch between them is a signal the split was wrong. The skill-create guidance says "multi-action wrappers can use the domain name alone" — pql is the domain.
- **Cost:** The skill body is longer (~170 lines). Still well under the 500-line limit.
- **Raised by:** Skill design discussion.

### D-10: State machine for ticket status transitions
- **Date:** 2026-04-22
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
- **Decision:** `pql plan export` writes all planning state (decisions + tickets + refs + deps + labels + history) to a single JSON file at the vault root (default `pql-plan.json`). `pql plan import` restores from that file into pql.db. The artifact is committed to git; pql.db stays gitignored.
- **Rationale:** Planning state is valuable enough to version but SQLite files don't belong in git. A JSON export is diffable, portable, and merge-friendly. The trigger is the user's choice — pre-push hook, sprint skill, manual — pql provides the verbs, not the policy.
- **Cost:** Two representations of the same data (pql.db + export file) can drift. The export is a snapshot, not a live mirror. Users who want the export current must run `pql plan export` before committing.
- **Raised by:** Resolved [Q-8](questions.md#q-8-occasional-pqldb-backups-into-git).
