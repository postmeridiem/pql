## CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**Early scaffolding.** Build skeleton, docs structure, and Claude scaffolding are in place. `internal/store/` and `internal/index/` shipped in 0.1.0 along with the primitive query surface (`pql files|tags|backlinks|outlinks|meta|schema|query`). The Tier-2 packages described in `docs/structure/project-structure.md` (`internal/query/…` deeper layers, `internal/connect/…`, `internal/intent/…`, `internal/planning/…`) are **not yet scaffolded** — they land alongside their first feature so empty package directories don't sit in the tree.

**Read these before designing or writing code:**
- `project.yaml` — single source of truth for project metadata: name, description, current declared version, license, repo, module path, schema_version, maintainers. The Makefile sources `VERSION` from here; the skill checks `schema_version` against this. Bump fields here rather than scattering them.
- `docs/structure/design-philosophy.md` — binding "why" doc. The binary is a *ranker* with intent-level surfaces. Generate vs rank as separate phases. Provenance is data. One SQLite store. No vectors. Narrow scope.
- `docs/structure/project-structure.md` — canonical layout, build pipeline, test infrastructure, growth model.
- `docs/structure/initial-plan.md` — original v1 plan (PQL grammar, SQLite schema, CLI specifics). Some framing superseded by the philosophy + structure docs; grammar/schema/exit-codes still authoritative.
- `docs/structure/planning.md` — spec for `pql decisions` / `pql ticket` / `pql plan`. First real writer to the user-state DB (`pql.db`), distinct from the cache (`index.db`).
- `decisions/architecture.md (D-3)` — the cache (`index.db`) vs user-authored state (`pql.db`) split. Read before touching anything that persists under `.pql/`.
- `docs/vault-layout.md` — the three vault-level conventions (`.pql/config.yaml`, `.pqlignore`, `.pql/`). The index defaults to `<vault>/.pql/index.db` (in-vault, like `.git/`); falls back to user cache on read-only vaults. Planning state lives beside it at `<vault>/.pql/pql.db` — no cache fallback, writes to a read-only vault fail cleanly.
- `docs/pqlignore.md` — gitignore-compatible exclusion file spec.
- `docs/watching.md` — `pql watch` toggle command spec. Explicit user invocation only; one watcher per vault; no daemon, no cross-vault registry.

## Naming

The project is **`pql` / PQL (Project Query Language)**. The pre-rename name was `mql` / MQL — never use it in new code, docs, or commit messages. The `git-commit` skill enforces this on commits.

Hosted at https://github.com/postmeridiem/pql. Module path: `github.com/postmeridiem/pql`.

## Architecture invariants

These are load-bearing — preserve them when implementing. Full rationale in the docs above.

- **Query → connect → bundle pipeline.** Primitive query rows are the spine. Enrichment (`internal/connect/{signal,rank,neighborhood,bundle}`) is an optional pass on top, attaching `connections[]` and per-result `signals[]` provenance. The DSL bypasses enrichment by default (raw rows = escape hatch).
- **`--flat-search` global flag** forces the primitive path on any subcommand, including intents that default to enriched. Single short-circuit in `cli/` — no per-subcommand wiring.
- **Generate vs rank as separate packages.** Neither imports the other. Generation is wide and cheap; ranking is bounded and careful.
- **Provenance is data, not a cross-cutting concern.** Each signal returns `Contribution{Name, Raw, Normalized, Weight}`; the combiner aggregates. Don't centralize into an `explain.go`.
- **Consumer-agnostic core.** `internal/intent/`, `internal/query/`, and `internal/planning/` must not import `internal/cli/`. CLI today, MCPs (plural) tomorrow — a query-surface MCP and a planning-surface MCP are different scopes, different permissions, different audiences. Every consumer is an adapter.
- **Two stores, two regimes.** `<vault>/.pql/index.db` is a pure cache — anything in it must be regenerable from the vault; on `schema_version` mismatch, drop and rebuild (no migration code). `<vault>/.pql/pql.db` is user-authored state (planning, skill state later) — forward-only migrations in `internal/planning/schema.go`, because losing this data is a bug, not a cache refresh. See `decisions/architecture.md (D-3)`.
- **Output contract is load-bearing for agents.** stdout JSON, stderr JSON-per-line diagnostics, exit codes 0/2/64/65/66/69/70 — distinguished. `2` (zero matches) is success, not an error. See `internal/diag/diag.go` and `docs/output-contract.md`.

## Build & test

Go 1.26+ is installed on the dev machine; `make build`, `make test`, and the rest of the targets below run locally.

| Command | Does |
|---|---|
| `make build` | binary at `./bin/pql` with version stamped via ldflags |
| `make test` | unit tests, fast |
| `make test-race` | unit tests with `-race` |
| `make test-integration` | `go test -tags=integration ./internal/cli/...` (planned; integration suite not yet written) |
| `make eval` | ranking-quality eval (planned; harness lands with first signals) |
| `make lint` | `golangci-lint run` |
| `make vuln` | `govulncheck ./...` (pinned to v1.2.0 via `go run`, no local install) |
| `make pre-push` | lint + vuln + test + test-race. Wired by `.githooks/pre-push`; integration suite is deliberately excluded to keep the push gate fast |
| `make snapshot` | `goreleaser release --snapshot --clean` (builds 5 platforms, no publish) |

`make help` lists everything.

CI substance lives in `ci/{lint,test,release,eval}.sh`. GitHub Actions workflows in `.github/workflows/` (added with first run) are thin wrappers around these — keeps local and CI behaviour identical and lets the provider be swapped without rewriting the scripts.

**Pre-push hook.** Opt in once per clone with `git config core.hooksPath .githooks`. The hook runs `make pre-push`; a failing check aborts the push locally so nothing reaches the remote.

## Test infrastructure

Three tiers (see `docs/structure/project-structure.md` for full details):

1. **Unit** — `_test.go` next to source, including DSL fuzz targets.
2. **Integration** — `internal/cli/integration_test.go` gated by `//go:build integration`. Shells the binary against fixture vaults in `testdata/`.
3. **Ranking-quality eval** — `internal/connect/rank/eval_test.go` gated by `//go:build eval`. Goldens at `internal/connect/rank/testdata/golden/*.json`. NDCG@k / MRR / P@k + per-signal contribution diffs vs baseline.

The Council vault at `/var/mnt/data/projects/council/` is the motivating fixture and the planned `testdata/council-snapshot/`.

## Growth (where new work lands)

| Growth | Where | Files |
|---|---|---|
| New intent | `internal/intent/<name>/` + `internal/cli/intent_<name>.go` | 2 new |
| New signal | `internal/connect/signal/<name>.go` + weight entries per intent | 1 new + N edits |
| New extractor (Logseq, code) | `internal/index/extractor/<name>/` + registry registration | 1 new subpackage |
| New planning verb | `internal/planning/repo/` method + `internal/cli/{decisions,ticket,plan}_<verb>.go` | 1 new CLI file + method on repo |
| New pql.db table | `internal/planning/schema.go` forward migration + repo helpers | 1 migration step + repo additions |
| New consumer (MCPs) | `cmd/pql-mcp-query/` reusing `internal/intent/`+`internal/query/`; `cmd/pql-mcp-plan/` reusing `internal/planning/` | enabled by consumer-agnostic core discipline; query and planning ship as separate binaries |
