# Architecture Decisions

Core design constraints for pql.

---

### D-001: No vector retrieval
- **Date:** 2026-04-19
- **Decision:** Retrieval is built from cheap, inspectable, structured signals — textual match, structural centrality, path proximity, recency, co-occurrence. No vector index.
- **Rationale:** The available embedded options are not tier-one infrastructure; the tier-one options are not embeddable as a single static binary. The class of queries where vectors beat structured retrieval is narrower than the ecosystem narrative suggests on code-and-prose repos. The absence forces the ranker to do its job properly with the signals genuinely available.
- **Cost:** Some paraphrase queries will miss. Users reach for ripgrep when they need raw text recall. If a specific repeatable failure mode emerges where structured signals can't compensate, revisit.
- **Raised by:** Design philosophy founding decision.

### D-002: Intent-level surface for agents, primitives as escape hatch
- **Date:** 2026-04-19
- **Decision:** Expose intent-level commands (`pql related`, `pql search`, `pql context`) as the primary surface. Each intent does internal candidate generation, ranking, and bundle assembly. The DSL and primitive subcommands are the escape hatch for exact queries. `--flat-search` reduces any intent to its primitive layer.
- **Rationale:** Each agent tool call is a permission event (chains erode trust), a round trip (the binary's internal queries are cheaper), and a drift opportunity (composition inside the binary is deterministic). The correct shape is one tool call per intent, not N chained primitives.
- **Cost:** New capabilities require new intents, not new primitives. If agents reach for the escape hatch frequently, the intent surface is wrong.
- **Raised by:** Design philosophy founding decision.

### D-003: Two SQLite stores — index.db (cache) and pql.db (state)
- **Date:** 2026-04-22
- **Decision:** `<vault>/.pql/index.db` is a pure cache — drop and rebuild on schema mismatch. `<vault>/.pql/pql.db` is user-authored state (decisions, tickets) — forward-only migrations.
- **Rationale:** One store fails both tests: as a cache it's too durable, as user state it's too fragile. The CLI-today / MCPs-tomorrow split reinforces this: a query MCP needs only index.db, a planning MCP needs only pql.db.
- **Cost:** Two files to manage. pql.db must survive upgrades; uninstalling pql leaves it in place.
- **Raised by:** Planning subcommands feature design.
- **Cross-reference:** Full rationale in [docs/adr/0003-pql-db-for-user-state.md](../docs/adr/0003-pql-db-for-user-state.md)

### D-004: Consumer-agnostic core
- **Date:** 2026-04-22
- **Decision:** `internal/intent/`, `internal/query/`, and `internal/planning/` must not import `internal/cli/`. CLI today, MCPs (plural) tomorrow. Every consumer is an adapter.
- **Rationale:** A query-surface MCP and a planning-surface MCP are different scopes, different permissions, different audiences. Coupling them to cobra blocks future consumers.
- **Cost:** CLI wiring is separate from core logic. Two files per feature (core + CLI adapter).
- **Raised by:** Architecture review during planning subcommands.

### D-005: CLI-first, not MCP
- **Date:** 2026-04-22
- **Decision:** Claude talks to pql via `Bash(pql *)`. No MCP server. The output contract (stdout JSON, stderr diagnostics, distinguished exit codes) is what makes it safe for agents.
- **Rationale:** A static binary on PATH is the lowest-friction surface for Claude Code: one allow rule in settings.json. No daemon, no network, no permission elevation.
- **Cost:** If an MCP-only integration becomes compelling later, nothing precludes adding one that shells out to the same CLI.
- **Raised by:** Project founding decision.
