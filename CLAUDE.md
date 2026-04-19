## CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**Early scaffolding.** The repo has the build skeleton (`Makefile`, `.goreleaser.yaml`, `.golangci.yaml`, `ci/*.sh`, `go.mod`, `cmd/pql/main.go`, `internal/{cli,version,diag}`), the docs structure, and the Claude scaffolding. The Tier-2 packages described in `docs/structure/project-structure.md` (`internal/store/`, `internal/index/`, `internal/query/…`, `internal/connect/…`, `internal/intent/…`) are **not yet scaffolded** — they land alongside their first feature so empty package directories don't sit in the tree.

**Read these before designing or writing code:**
- `project.yaml` — single source of truth for project metadata: name, description, current declared version, license, repo, module path, schema_version, maintainers. The Makefile sources `VERSION` from here; the skill checks `schema_version` against this. Bump fields here rather than scattering them.
- `docs/structure/design-philosophy.md` — binding "why" doc. The binary is a *ranker* with intent-level surfaces. Generate vs rank as separate phases. Provenance is data. One SQLite store. No vectors. Narrow scope.
- `docs/structure/project-structure.md` — canonical layout, build pipeline, test infrastructure, growth model.
- `docs/structure/initial-plan.md` — original v1 plan (PQL grammar, SQLite schema, CLI specifics). Some framing superseded by the philosophy + structure docs; grammar/schema/exit-codes still authoritative.

## Naming

The project is **`pql` / PQL (Project Query Language)**. The pre-rename name was `mql` / MQL — never use it in new code, docs, or commit messages. The `git-commit` skill enforces this on commits.

Hosted at https://github.com/postmeridiem/pql. Module path: `github.com/postmeridiem/pql`.

## Architecture invariants

These are load-bearing — preserve them when implementing. Full rationale in the docs above.

- **Query → connect → bundle pipeline.** Primitive query rows are the spine. Enrichment (`internal/connect/{signal,rank,neighborhood,bundle}`) is an optional pass on top, attaching `connections[]` and per-result `signals[]` provenance. The DSL bypasses enrichment by default (raw rows = escape hatch).
- **`--flat-search` global flag** forces the primitive path on any subcommand, including intents that default to enriched. Single short-circuit in `cli/` — no per-subcommand wiring.
- **Generate vs rank as separate packages.** Neither imports the other. Generation is wide and cheap; ranking is bounded and careful.
- **Provenance is data, not a cross-cutting concern.** Each signal returns `Contribution{Name, Raw, Normalized, Weight}`; the combiner aggregates. Don't centralize into an `explain.go`.
- **Consumer-agnostic core.** `internal/intent/` and below must not import `internal/cli/`. CLI today, possibly MCP server later — both adapters.
- **Index is a pure cache.** Anything in SQLite must be regenerable. Schema versioned via `internal/version.SchemaVersion`; on mismatch, drop the DB and rebuild — no migration code.
- **Output contract is load-bearing for agents.** stdout JSON, stderr JSON-per-line diagnostics, exit codes 0/2/64/65/66/69/70 — distinguished. `2` (zero matches) is success, not an error. See `internal/diag/diag.go` and `docs/output-contract.md`.

## Build & test

Go is currently **not installed** on the dev machine. `make build` will fail until `brew install go` (or equivalent). The skeleton is hand-written and syntactically intended to compile once Go is present.

| Command | Does |
|---|---|
| `make build` | binary at `./bin/pql` with version stamped via ldflags |
| `make test` | unit tests, fast |
| `make test-race` | unit tests with `-race` |
| `make test-integration` | `go test -tags=integration ./internal/cli/...` (planned; integration suite not yet written) |
| `make eval` | ranking-quality eval (planned; harness lands with first signals) |
| `make lint` | `golangci-lint run` |
| `make vuln` | `govulncheck ./...` |
| `make snapshot` | `goreleaser release --snapshot --clean` (builds 5 platforms, no publish) |

`make help` lists everything.

CI substance lives in `ci/{lint,test,release,eval}.sh`. GitHub Actions workflows in `.github/workflows/` (added with first run) are thin wrappers around these — keeps local and CI behaviour identical and lets the provider be swapped without rewriting the scripts.

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
| New consumer (e.g. MCP server) | `cmd/pql-mcp/` reusing `internal/intent/`, `internal/query/` | enabled by consumer-agnostic core discipline |
