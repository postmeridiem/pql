# Decisions, Questions, Rejected

This directory holds structured planning records that pql parses
into pql.db. Each record is a `### [DQR]-N: Title` heading inside
a markdown file. Files live in three per-type subdirectories:

- `decisions/<domain>.md` — confirmed design decisions
- `questions/<domain>.md` — open questions that may resolve into
  decisions or rejected proposals
- `rejected/<domain>.md` — rejected proposals (kept for the audit
  trail)

The parser infers domain from the filename stem and record type
from the parent subdirectory.

D-records that propose implementation work link to `initiative`-type
tickets via `decision_ref`. Run `pql decisions show <id>
--with-tickets` to inspect implementation status.

## Recommended domains

Start with this canonical set; create files as records land in
each domain:

- **architecture** — structural commitments (storage, layering,
  languages, libraries)
- **process** — team workflow (commits, branches, releases, reviews)
- **design** — user-facing surface (UX, UI, public APIs)
- **coding-conventions** — team-internal code shape (style, lint,
  file layout)
- **testing** — quality strategy (coverage, layers, gates)

You might also want, project-permitting:

- `accessibility` — if you ship user-facing software
- `security` — if you handle user data or network surfaces
- `licensing` — if you release open-source or commercial
- `documentation` — if user-docs are non-trivial
- `deployment` — if shipping is non-trivial
- `performance` — if you have perf budgets / SLOs

<!-- pql:records (auto-generated; do not edit manually) -->

## Decisions

- [D-1: No vector retrieval](decisions/architecture.md#d-1-no-vector-retrieval) — _architecture_
- [D-2: Intent-level surface for agents, primitives as escape hatch](decisions/architecture.md#d-2-intent-level-surface-for-agents-primitives-as-escape-hatch) — _architecture_
- [D-3: Two SQLite stores — index.db (cache) and pql.db (state)](decisions/architecture.md#d-3-two-sqlite-stores--indexdb-cache-and-pqldb-state) — _architecture_
- [D-4: Consumer-agnostic core](decisions/architecture.md#d-4-consumer-agnostic-core) — _architecture_
- [D-5: CLI-first, not MCP](decisions/architecture.md#d-5-cli-first-not-mcp) — _architecture_
- [D-6: In-house migration runner, not golang-migrate or goose](decisions/architecture.md#d-6-in-house-migration-runner-not-golang-migrate-or-goose) — _architecture_
- [D-7: Text IDs as primary keys](decisions/architecture.md#d-7-text-ids-as-primary-keys) — _architecture_
- [D-8: Ticket source of truth is SQLite, not markdown](decisions/architecture.md#d-8-ticket-source-of-truth-is-sqlite-not-markdown) — _architecture_
- [D-9: One combined skill, not separate query/plan skills](decisions/architecture.md#d-9-one-combined-skill-not-separate-queryplan-skills) — _architecture_
- [D-10: State machine for ticket status transitions](decisions/architecture.md#d-10-state-machine-for-ticket-status-transitions) — _architecture_
- [D-11: Max-normalization per signal before weighting](decisions/architecture.md#d-11-max-normalization-per-signal-before-weighting) — _architecture_
- [D-12: Clean version strings — just the number](decisions/architecture.md#d-12-clean-version-strings--just-the-number) — _architecture_
- [D-13: plan export and plan import for versioned planning snapshots](decisions/architecture.md#d-13-plan-export-and-plan-import-for-versioned-planning-snapshots) — _architecture_
- [D-14: No status transition enforcement in pql](decisions/architecture.md#d-14-no-status-transition-enforcement-in-pql) — _architecture_
- [D-15: Replication via per-table monthly SQL changelog files](decisions/architecture.md#d-15-replication-via-per-table-monthly-sql-changelog-files) — _architecture_
- [D-16: Inline LWW guards plus content hash plus canonical row columns](decisions/architecture.md#d-16-inline-lww-guards-plus-content-hash-plus-canonical-row-columns) — _architecture_
- [D-17: Soft deletes via deleted_at column](decisions/architecture.md#d-17-soft-deletes-via-deleted-at-column) — _architecture_
- [D-18: Git lifecycle hook architecture for replication](decisions/architecture.md#d-18-git-lifecycle-hook-architecture-for-replication) — _architecture_
- [D-19: No ALTER TABLE — schema lives in CREATE statements only](decisions/architecture.md#d-19-no-alter-table--schema-lives-in-create-statements-only) — _architecture_
- [D-20: Decision implementation tracked via initiative tickets, no coverage report](decisions/architecture.md#d-20-decision-implementation-tracked-via-initiative-tickets-no-coverage-report) — _architecture_
- [D-21: DQR layout — `governance/` parent with per-type subdirectories](decisions/architecture.md#d-21-dqr-layout--governance-parent-with-per-type-subdirectories) — _architecture_

## Open questions

- [Q-1: Markdown mirror for tickets](questions/architecture.md#q-1-markdown-mirror-for-tickets) — _architecture_
- [Q-2: FTS for ticket/decision search](questions/architecture.md#q-2-fts-for-ticketdecision-search) — _architecture_
- [Q-3: Multi-user planning DB](questions/architecture.md#q-3-multi-user-planning-db) — _architecture_
- [Q-4: Validator strictness](questions/architecture.md#q-4-validator-strictness) — _architecture_
- [Q-5: Body access in the DSL](questions/architecture.md#q-5-body-access-in-the-dsl) — _architecture_
- [Q-6: Outlink target normalization](questions/architecture.md#q-6-outlink-target-normalization) — _architecture_
- [Q-7: Code-aware extractor](questions/architecture.md#q-7-code-aware-extractor) — _architecture_
- [Q-8: Occasional pql.db backups into git](questions/architecture.md#q-8-occasional-pqldb-backups-into-git) — _architecture_
- [Q-9: Multi-user ticket ID distribution](questions/architecture.md#q-9-multi-user-ticket-id-distribution) — _architecture_
- [Q-10: Soft-delete stub retention and purge](questions/architecture.md#q-10-soft-delete-stub-retention-and-purge) — _architecture_

## Rejected

- _(none)_
