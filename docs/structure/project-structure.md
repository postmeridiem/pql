# Project Structure

This document is the canonical reference for `pql`'s repository layout, build pipeline, test infrastructure, and growth model. Read alongside `design-philosophy.md` (the binding "why") and `initial-plan.md` (the original v1 plan, retained for grammar/schema/CLI specifics).

## Why this exists

`pql` is a Go CLI that indexes a repository and serves Claude Code (and humans) with structured repository context. The structure has to satisfy four constraints:

1. **Encode the design philosophy** — generate vs rank as separate phases, one SQLite store, provenance-as-data, intent-specific weighting.
2. **Reconcile two mental models.** Per the user's framing: *"feel simple as a query engine, but dynamically offer connections if they are available."* Primitives are the spine; ranker-driven **connections** are an optional enrichment layer on top, not a replacement. Every intent is a named combination of (query primitives + enrichment profile).
3. **Absorb growth** beyond the initial Claude-Code-skill use case — new intents, new signals, new extractors (code/Logseq), and eventually a second consumer (e.g. MCP server) without restructuring.
4. **Make ranking quality a first-class test concern** — "ranking is the product" per the philosophy, so regressions in ranking quality must be as visible as test failures.

## Guiding principles

- **Query-first surface, connections on request.** Direct subcommands (`pql files`, `pql tags`, `pql backlinks`, `pql query <DSL>`) feel like a query engine. A `--connect` flag (and intent-level commands that default it on) attaches related-context bundles when signals exist. No enrichment = plain rows. Enrichment = same rows + `connections` array with provenance.
- **Always an off-switch for enrichment.** A global `--flat-search` flag forces raw query results — disables `connect/` entirely, even on intent subcommands that would otherwise enrich by default. *"Sometimes you just need the exact result queried exactly where you want it to look."* The DSL path (`pql query <DSL>`) is already flat by default and doubles as the explicit "give me only what I asked for" entry point; `--flat-search` covers the cross-cutting case so any subcommand can be reduced to its primitive layer.
- **Generate wide, rank careful, return sparingly.** Architecturally separate packages; neither imports the other.
- **Provenance is data, not a cross-cutting concern.** Each signal returns its own `Contribution{Name, Raw, Normalized, Weight}`; the combiner aggregates. No central `explain.go`.
- **Consumer-agnostic core.** `internal/intent/` and below must not import `internal/cli/`. CLI today, MCP server tomorrow — both adapters.
- **One store, one transaction, one artifact.** SQLite with FTS5; schema versioned; drop-and-rebuild on mismatch (the index is a pure cache).

## Directory layout

```
pql/
├── cmd/
│   └── pql/main.go                   # tiny entrypoint: version stamp, calls internal/cli
├── internal/
│   ├── cli/                          # cobra root, flag parsing, subcommand wiring
│   │   ├── root.go
│   │   ├── query_*.go                # primitive query subcommands (files, tags, backlinks, outlinks, schema, meta)
│   │   ├── intent_*.go               # intent subcommands (related, search, context, base)
│   │   ├── dsl.go                    # `pql query <DSL>` escape hatch
│   │   ├── render/                   # JSON / JSONL / table / CSV; exit-code mapping
│   │   └── integration_test.go       # //go:build integration — shells the binary
│   ├── query/                        # primitive query surface (query engine feel)
│   │   ├── primitives/               # typed queries: files, tags, backlinks, frontmatter…
│   │   ├── dsl/                      # PQL DSL: lex, parse, eval, base (Obsidian .base → AST)
│   │   │   ├── lex/
│   │   │   ├── parse/
│   │   │   ├── eval/
│   │   │   └── base/
│   │   └── result/                   # typed result rows shared with connect/
│   ├── connect/                      # optional enrichment: connections + provenance
│   │   ├── signal/                   # textual, centrality, recency, proximity, identity, cooccurrence
│   │   │   └── (each signal returns Contribution{Name, Raw, Normalized, Weight})
│   │   ├── neighborhood.go           # one-hop structural context (cap at two hops)
│   │   ├── rank.go                   # weighted combination, intent-specific weights
│   │   └── bundle.go                 # attaches connections[] to query results
│   ├── intent/                       # one subpackage per intent (thin: query + connect profile)
│   │   ├── related/
│   │   ├── search/
│   │   ├── context/
│   │   └── …                         # NEW INTENT = NEW SUBPACKAGE + one cli/intent_*.go file
│   ├── index/                        # walker, parsers, incremental update
│   │   ├── walker.go
│   │   ├── extractor/                # Registry pattern — extractors register by file pattern
│   │   │   ├── markdown/             # frontmatter, wikilinks, tags, headings (v1 scope)
│   │   │   ├── code/                 # placeholder; tree-sitter later
│   │   │   └── registry.go
│   │   └── incremental.go            # change detection, mtime + content_hash
│   ├── store/                        # SQLite layer
│   │   ├── schema/                   # versioned SQL
│   │   ├── migrate.go                # drop-and-rebuild on schema_version mismatch
│   │   ├── conn.go                   # WAL, BEGIN IMMEDIATE per indexer invocation
│   │   ├── fts.go
│   │   └── repo/                     # narrow helpers per table
│   ├── config/                       # .pql/config.yaml, env vars (PQL_VAULT/DB/CONFIG), vault-root discovery
│   ├── diag/                         # stderr JSON diagnostics + exit-code constants
│   ├── telemetry/                    # per-phase timings (generate_ms, rank_ms, per-signal ms) on --verbose
│   ├── fixture/                      # synthetic vault generators for eval
│   ├── skill/                        # Claude Code skill (SKILL.md + go:embed wrapper); `pql skill install` writes it to .claude/skills/pql/
│   └── version/                      # ldflags-stamped build info; exposes schema_version for skill negotiation
├── testdata/                         # fixture vaults (Go toolchain ignores this dir specially)
│   ├── council-snapshot/             # frozen snapshot of /var/mnt/data/projects/council/
│   ├── minimal/
│   └── mixed/                        # markdown + code, for future code-aware tests
├── tools/
│   └── eval-report/                  # diff + visualize ranking-eval runs
├── docs/
│   ├── structure/
│   │   ├── design-philosophy.md      # source of truth for "why"
│   │   ├── initial-plan.md           # original v1 plan; grammar/schema/CLI specifics
│   │   └── project-structure.md      # this file
│   ├── intents.md                    # intent catalog + per-intent contract
│   ├── signals.md                    # signal catalog: what it measures, where it shines/fails
│   ├── pql-grammar.md                # DSL grammar
│   ├── output-contract.md            # stdout JSON, stderr JSON, exit codes (0/2/65/66/69)
│   ├── compatibility.md              # binary↔skill schema_version negotiation
│   ├── skill.md
│   └── adr/                          # ADRs; first two: 0001-no-vectors.md, 0002-intents-not-primitives.md
├── examples/
├── ci/                               # entry scripts shelled out to by .github/workflows/*.yaml
│   ├── lint.sh                       # golangci-lint + goreleaser check + govulncheck
│   ├── test.sh                       # unit + race + integration
│   ├── release.sh                    # invokes goreleaser; called on tag push
│   └── eval.sh                       # ranking-quality eval; for scheduled job
├── .github/workflows/                # GitHub Actions wrappers around ci/*.sh (added with first CI run)
├── .goreleaser.yaml                  # GitHub Releases publisher
├── .golangci.yaml                    # errcheck, revive, gocritic, staticcheck, gosec, …
├── .editorconfig
├── Makefile
├── go.mod                            # module github.com/postmeridiem/pql
├── go.sum
├── CLAUDE.md
├── README.md
└── LICENSE                           # MIT
```

> **Tier-2 packages.** Many of the directories above (`internal/store/`, `internal/index/`, `internal/query/…`, `internal/connect/…`, `internal/intent/…`, `tools/eval-report/`, `testdata/`) are **planned**, not yet scaffolded. They land alongside their first feature so empty package directories don't sit in the tree.

## The query → connect → bundle pipeline

```
CLI subcommand (cli/query_*.go | cli/intent_*.go | cli/dsl.go)
   │
   ▼
query/primitives  (or  query/dsl)        ← produces typed rows
   │
   ▼                                     ← if --connect or intent requests it (and not --flat-search):
connect/{signal,rank,neighborhood,bundle}
   │
   ▼
cli/render                               ← stdout JSON; provenance inline in connections[]
```

- **Primitive path only:** rows out, no connections array, no provenance. Feels like a query engine.
- **Enriched path:** same rows, each with a `connections` array (capped depth: 1 hop default, 2 max) and per-result `signals[]` recording each `Contribution`. Intent-specific weight profiles live in `internal/intent/<name>/weights.go`.
- **DSL bypasses enrichment** by default (escape hatch = raw rows); documented in `docs/intents.md`.
- **`--flat-search` global flag** forces the primitive path on any subcommand, including intent commands that would otherwise enrich. Implementation: a single short-circuit check in `cli/` before invoking `connect/` — no per-subcommand wiring needed. Listed under global flags in `docs/output-contract.md`.

## Growth — what new work looks like

| Growth | Where it lands | Files changed |
|---|---|---|
| New intent | `internal/intent/<name>/` + `internal/cli/intent_<name>.go` | 2 new files |
| New signal | `internal/connect/signal/<name>.go` + weight entries per intent | 1 new file + N-line edits |
| New extractor | `internal/index/extractor/<name>/` + registry registration | 1 new subpackage |
| New consumer (MCP) | `cmd/pql-mcp/` reusing `internal/intent/…`, `internal/query/…` | Bounded by consumer-agnostic core discipline |
| Code-aware indexing | `internal/index/extractor/code/` with tree-sitter | No changes to `store/`, `connect/`, `query/` |

## Test infrastructure

Three tiers, idiomatic Go placement:

1. **Unit tests** — `_test.go` next to source. Includes fuzz targets for the DSL (`internal/query/dsl/lex/fuzz_test.go`, `internal/query/dsl/parse/fuzz_test.go`). Run via `make test`.
2. **Integration tests** — `internal/cli/integration_test.go` gated by `//go:build integration`. Shells the built binary against fixture vaults in `testdata/`. Validates the full output contract (stdout JSON shape, stderr JSON diagnostics, exit codes). Run via `make test-integration`.
3. **Ranking-quality eval** — `internal/connect/rank/eval_test.go` gated by `//go:build eval`. Goldens at `internal/connect/rank/testdata/golden/*.json` as `{query, intent, expected_top_k, notes}`. Computes NDCG@k / MRR / P@k, **plus per-signal contribution diffs vs. the previous run** (debuggability > metric). A standalone `cmd/pql-eval/` binary exposes the same harness for bisecting weight changes. Run via `make eval`.

Fixture vaults: `testdata/council-snapshot/` (frozen copy of `/var/mnt/data/projects/council/`), plus synthetic vaults generated by `internal/fixture/` for eval corners.

## Build & release pipeline

**Makefile targets** (see `Makefile` for the full list):

| Target | Does |
|---|---|
| `make build` | `go build -ldflags='…version…' -o bin/pql ./cmd/pql` |
| `make install` | Copy `bin/pql` to `~/.local/bin/` |
| `make test` | Unit tests, fast |
| `make test-race` | Unit tests with `-race` |
| `make test-integration` | `go test -tags=integration ./internal/cli/...` |
| `make eval` | `go test -tags=eval ./internal/connect/rank/...` |
| `make eval-baseline` | Record current eval as baseline for next diff |
| `make fuzz-dsl` | Run DSL fuzz corpus (10m default) |
| `make lint` | `golangci-lint run` |
| `make vuln` | `govulncheck ./...` |
| `make fmt` | `gofmt -w` + `goimports -w` |
| `make tidy` | `go mod tidy` |
| `make snapshot` | `goreleaser release --snapshot --clean` |
| `make profile-cpu` / `make profile-mem` | pprof against largest fixture |
| `make clean` | `rm -rf bin/ dist/` |

**CI scripts in `ci/`, GitHub Actions wrappers in `.github/workflows/`:**

The substance of CI lives in `ci/*.sh` so it can run identically locally and in CI. GitHub Actions workflows are thin shell-out wrappers; they can be replaced with another provider later if needed without touching the scripts.

- `ci/lint.sh` — `golangci-lint run`, `goreleaser check`, `govulncheck ./...`. Under 1 min.
- `ci/test.sh` — unit + `-race` + integration. Under 5 min budget on PR.
- `ci/release.sh` — on tag `v*`: GoReleaser full pipeline. 5 platforms (linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64), SHA256 sums, SBOM, cosign signatures, auto-generated notes. Publishes to GitHub Releases per `.goreleaser.yaml`.
- `ci/eval.sh` — scheduled job: `make eval`, post metrics. Not blocking; regressions surface as visible drift in the metrics record.

**Distribution channels:**
- GitHub Releases — primary channel, signed binaries + SHA256SUMS + SBOM.
- `go install github.com/postmeridiem/pql/cmd/pql@latest` for developers with a Go toolchain.
- `pql self-update` once v0.1 ships — hits the GitHub Releases API, downloads + replaces atomically, verifies SHA256.

## Observability and docs discipline

- **Telemetry on `--verbose`:** `internal/telemetry/` injects per-phase timings (`generate_ms`, `rank_ms`, per-signal ms) into the stderr diagnostic stream. This is how we'll tune weights honestly.
- **Schema-version negotiation:** `pql version --build-info` emits `{version, commit, date, go_version, schema_version}`. The skill checks `schema_version` against its own and refuses on mismatch with a remediation hint. Contract documented in `docs/compatibility.md`.
- **Catalog docs** sit at the top of `docs/`:
  - `docs/intents.md` — intent catalog + per-intent contract.
  - `docs/signals.md` — every signal: what it measures, where it shines, where it fails.
  - `docs/adr/` — Architecture Decision Records. First two to land: `0001-no-vectors.md`, `0002-intents-not-primitives.md`.

## Verification

End-to-end once each milestone lands:

1. `make lint` + `make test` + `make test-race` → all green.
2. `make build` → `./bin/pql --version` prints stamped build info; `./bin/pql version --build-info` prints JSON including `schema_version`.
3. `make snapshot` → `dist/` contains 5 platform archives with checksums.
4. `make test-integration` against `testdata/council-snapshot/` → exit codes 0/2/65/66 all exercised.
5. `make eval` with a seeded 3-query golden set → produces NDCG@5/MRR/P@5 report + per-signal contribution table.
6. Push a throwaway branch → `ci/lint.sh` + `ci/test.sh` complete locally under 5 min (the host pipeline shells out to these, so local-equals-CI by construction).
7. Tag a pre-release `v0.0.1-rc1` and run `ci/release.sh` (or push the tag and let `.github/workflows/release.yaml` invoke it) → produces signed binaries published to GitHub Releases; verify cosign signature locally.
8. Install binary + skill on a clean machine; run `pql files` in the Council vault → feels like a query engine. Add `--connect` → same query, now with `connections[]` and `signals[]`. Confirms the simple-but-optionally-enriched surface.
9. Run an intent command (`pql related members/vaasa/persona.md`) with and without `--flat-search`: without the flag returns enriched bundle; with the flag returns the bare query result and zero connections/provenance. Confirms the off-switch is reachable from every entry point.
