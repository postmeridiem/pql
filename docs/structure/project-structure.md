# Project Structure

This document is the canonical reference for `pql`'s repository layout, build pipeline, test infrastructure, and growth model. Read alongside `design-philosophy.md` (the binding "why") and `initial-plan.md` (the original v1 plan, retained for grammar/schema/CLI specifics).

## Why this exists

`pql` is a Go CLI that indexes a repository and serves Claude Code (and humans) with structured repository context. The structure has to satisfy four constraints:

1. **Encode the design philosophy** — generate vs rank as separate phases, one SQLite store, provenance-as-data, intent-specific weighting.
2. **Reconcile two mental models.** Per the user's framing: *"feel simple as a query engine, but dynamically offer connections if they are available."* Primitives are the spine; ranker-driven **connections** are an optional enrichment layer on top, not a replacement. Every intent is a named combination of (query primitives + enrichment profile).
3. **Absorb growth** beyond the initial Claude-Code-skill use case — new intents, new signals, new extractors (code/Logseq), and eventually a second consumer (e.g. MCP server) without restructuring.
4. **Make ranking quality a first-class test concern** — "ranking is the product" per the philosophy, so regressions in ranking quality must be as visible as test failures.

## Guiding principles

- **Query-first surface, connections on request.** Direct subcommands (`pql files`, `pql tags`, `pql backlinks`, `pql query <DSL>`) feel like a query engine and return plain rows. Intent-level commands (`pql related`, `pql search`, `pql context`) attach related-context bundles by default; `--flat-search` forces the plain path on any command. No enrichment = plain rows. Enrichment = same rows + `connections` array with provenance.
- **Always an off-switch for enrichment.** A global `--flat-search` flag forces raw query results — disables `connect/` entirely, even on intent subcommands that would otherwise enrich by default. *"Sometimes you just need the exact result queried exactly where you want it to look."* The DSL path (`pql query <DSL>`) is already flat by default and doubles as the explicit "give me only what I asked for" entry point; `--flat-search` covers the cross-cutting case so any subcommand can be reduced to its primitive layer.
- **Generate wide, rank careful, return sparingly.** Architecturally separate packages; neither imports the other.
- **Provenance is data, not a cross-cutting concern.** Each signal returns its own `Contribution{Name, Raw, Normalized, Weight}`; the combiner aggregates. No central `explain.go`.
- **Consumer-agnostic core.** `internal/intent/`, `internal/query/`, and `internal/planning/` must not import `internal/cli/`. CLI today, MCPs (plural) tomorrow — a query-surface MCP and a planning-surface MCP are different scopes, different permissions, different audiences; no reason to assume one fused server. Every consumer is an adapter.
- **Two stores, two regimes.** `<vault>/.pql/index.db` is the regenerable cache — SQLite with FTS5; schema versioned; drop-and-rebuild on mismatch. `<vault>/.pql/pql.db` is user-authored state (planning, possibly other features later), lazily created by the first writer. The split is codified in `governance/decisions/architecture.md` (D-3). Note that D-3's own "forward-only migrations" phrase is superseded by **D-19**: there is no migration runner today, the schema lives in `CREATE TABLE IF NOT EXISTS` statements, and pql.db is regenerated from the committed changelog rather than altered in place. D-19 is the current authority on how pql.db evolves.

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
│   │   ├── decisions_*.go            # planning: `pql decisions …` (see planning.md)
│   │   ├── ticket_*.go               # planning: `pql ticket …`
│   │   ├── plan_*.go                 # planning: `pql plan …` (cross-cutting)
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
│   ├── planning/                     # decisions + tickets; writes to pql.db (see planning.md, D-3)
│   │   ├── db.go                     # opens <vault>/.pql/pql.db; creates schema if missing
│   │   ├── schema.go                 # CREATE TABLE IF NOT EXISTS + CanonicalVersion (no migration runner, D-19)
│   │   ├── parser/                   # DQR markdown → []Record (decisions.go, headings.go)
│   │   ├── repo/                     # decisions.go, tickets.go, meta.go, snapshot.go
│   │   └── changelog/                # exporter/importer/rebuild: git-committed replication (D-15/D-16)
│   ├── index/                        # walker, parsers, incremental update
│   │   ├── walker.go
│   │   ├── extractor/                # Registry pattern — extractors register by file pattern
│   │   │   ├── markdown/             # frontmatter, wikilinks, tags, headings (v1 scope)
│   │   │   ├── code/                 # placeholder; tree-sitter later
│   │   │   └── registry.go
│   │   └── incremental.go            # change detection, mtime + content_hash
│   ├── store/                        # SQLite layer for index.db (the cache)
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
│   ├── council-snapshot/             # frozen snapshot of the Council vault (a sibling checkout)
│   ├── minimal/
│   └── mixed/                        # markdown + code, for future code-aware tests
├── tools/
│   └── eval-report/                  # diff + visualize ranking-eval runs
├── docs/
│   ├── structure/
│   │   ├── design-philosophy.md      # source of truth for "why"
│   │   ├── initial-plan.md           # original v1 plan; grammar/schema/CLI specifics
│   │   ├── project-structure.md      # this file
│   │   └── planning.md               # decisions + tickets spec (pql.db, the state store)
│   ├── intents.md                    # intent catalog + per-intent contract
│   ├── signals.md                    # signal catalog: what it measures, where it shines/fails
│   ├── pql-grammar.md                # DSL grammar
│   ├── output-contract.md            # stdout JSON, stderr JSON, exit codes (0/64/65/66/69/70)
│   ├── compatibility.md              # binary↔skill schema_version negotiation
│   ├── skill.md
│   └── adr/                          # ADRs: 0001-no-vectors, 0002-intents-not-primitives, 0003-pql-db-for-user-state
├── examples/
├── ci/                               # entry scripts; lint.sh + test.sh are what .github/workflows/*.yaml shells out to
│   ├── lint.sh                       # golangci-lint + goreleaser check + govulncheck  (CI + `make lint`)
│   ├── test.sh                       # unit + race + integration                        (CI + `make ci-test`)
│   ├── secrets.sh                    # gitleaks over the outgoing range                 (local only: `make pre-push`)
│   ├── secrets-selftest.sh           # proves .gitleaks.toml still matches what it claims
│   ├── eval.sh                       # ranking-quality eval                             (local only: `make eval`; not a gate)
│   ├── release.sh                    # goreleaser release --clean                       (release.yaml's release job)
│   └── workflows_test.go             # guards this directory's wiring — see below
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

> **Layout status (as of v1.6).** The tree above is the *intended* canonical layout; most of it now exists — `internal/query/`, `internal/connect/`, `internal/intent/`, `internal/planning/`, `internal/store/`, `internal/index/` are all shipped and populated, and `internal/watch/` (the `pql watch` toggle) is live but not drawn above. Still **not built / aspirational**, despite appearing in the tree: `internal/query/result/`, `internal/planning/format/` (rendering lives in `internal/cli/render` + `planning/repo`), `internal/index/incremental.go` (change detection is in `indexer.go`/`walker.go`), `internal/fixture/`, `tools/eval-report/`, `cmd/pql-eval/`, and `docs/adr/` (ADRs became decision records under `governance/decisions/`). New packages still land alongside their first feature.

## The query → connect → bundle pipeline

```
CLI subcommand (cli/query_*.go | cli/intent_*.go | cli/dsl.go)
   │
   ▼
query/primitives  (or  query/dsl)        ← produces typed rows
   │
   ▼                                     ← if an intent requests enrichment (and not --flat-search):
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
| New planning verb | `internal/planning/repo/` method + `internal/cli/{decisions,ticket,plan}_<verb>.go` | 1 new CLI file + method on repo |
| New pql.db table | `CREATE TABLE IF NOT EXISTS` in `internal/planning/schema.go`, bump `CanonicalVersion`, + repo helpers | schema edit + repo additions (no migration runner — D-19) |
| New consumer (MCPs) | `cmd/pql-mcp-query/` reusing `internal/intent/`+`internal/query/`; `cmd/pql-mcp-plan/` reusing `internal/planning/` | Bounded by consumer-agnostic core discipline; query surface and planning surface can ship as separate binaries |
| Code-aware indexing | `internal/index/extractor/code/` with tree-sitter | No changes to `store/`, `connect/`, `query/`, `planning/` |

## Test infrastructure

Three tiers, idiomatic Go placement:

1. **Unit tests** — `_test.go` next to source. Includes fuzz targets for the DSL (`internal/query/dsl/lex/fuzz_test.go`, `internal/query/dsl/parse/fuzz_test.go`). Run via `make test`.
2. **Integration tests** — `internal/cli/integration_test.go` gated by `//go:build integration`. Shells the built binary against fixture vaults in `testdata/`. Validates the full output contract (stdout JSON shape, stderr JSON diagnostics, exit codes). Run via `make test-integration`.
3. **Ranking-quality eval** — `internal/connect/rank/eval_test.go` gated by `//go:build eval`. Goldens at `internal/connect/rank/testdata/golden/*.json` as `{query, intent, expected_top_k, notes}`. Computes NDCG@k / MRR / P@k, **plus per-signal contribution diffs vs. the previous run** (debuggability > metric). Run via `make eval`, which delegates to `ci/eval.sh`; there is no separate `cmd/pql-eval/` binary.

Fixture vaults: `testdata/council-snapshot/` (a committed frozen copy of the Council vault, refreshed from that sibling checkout by `make refresh-fixtures`), plus synthetic vaults generated by `internal/fixture/` for eval corners.

**How to write the assertions inside these tiers is a decision, not a layout question**, and it lives in the DQR tree: **D-32** in `governance/decisions/testing.md` — choose the default that makes an omission safe. This section says which tests exist and how to run them; D-32 says how strong a claim each one should make.

## Build & release pipeline

**Makefile targets: `make help` is the list.** It is generated from the Makefile's own `##` comments, so it is the one copy that cannot drift. This document deliberately does not restate it, and neither does `CLAUDE.md` — a table here said `make lint` was `golangci-lint run` for months after the target became the full three-stage gate, and contradicted a correct description of the same command twelve lines further down its own page (T-115). `CLAUDE.md`'s "Build & test" section carries the judgment that `make help` has no room for: which targets are gates, what each gate actually contains, and the order `make pre-push` runs them in.

**CI scripts in `ci/`, GitHub Actions wrappers in `.github/workflows/`:**

The substance of CI lives in `ci/*.sh` so it can run identically locally and in CI. The workflows shell out to those scripts rather than restating their steps, so the provider can be replaced later without touching them. That property now holds for every script a workflow runs, and is the reason a Makefile target which re-listed a script's stages is treated as a bug here. It is recorded as **D-33** in `governance/decisions/process.md`, with the two incidents that earned it.

Which scripts a workflow runs, and which are local tools, is itself the distinction that drifted (T-70) — so each entry below states its caller.

- `ci/lint.sh` — `golangci-lint run`, `goreleaser check`, `govulncheck ./...`. Under 1 min. Run by `ci.yaml`, by `release.yaml`'s lint job, and by `make lint`.
- `ci/test.sh` — unit + `-race` + integration. Under 5 min budget on PR. Run by `ci.yaml` and by `make ci-test`.
- `ci/release.sh` — `goreleaser release --clean`. Run by `release.yaml`'s release job, after that job installs goreleaser at the version pinned in the workflow's `env:` — the same one the lint job ran `goreleaser check` with, so the config that was validated is the config that publishes. Never run it by hand; `make snapshot` dry-runs the same config without publishing.
- `ci/secrets.sh` — gitleaks over `<upstream>..HEAD`, preceded by `ci/secrets-selftest.sh`. **Local only**, from `make secrets` / `make pre-push`; no workflow runs it. See `CLAUDE.md`, "This repo is public".
- `ci/eval.sh` — ranking-quality eval. **Local only and not a gate**: nothing schedules it, there is no metrics sink, and the golden set is currently red (T-120). `make eval` is its only caller. Run it when changing a signal, a weight or candidate generation, and read the output as a diff against the previous run.

**The caller/script wiring is tested, not just documented** — `ci/workflows_test.go`, a plain unit test in `make test`. It asserts that every workflow parses, that every `${{ env.X }}` resolves to a defined key (an unresolvable one expands to the empty string rather than failing), that every inline `run:` block is valid bash, that every `./ci/*.sh` a workflow invokes exists and is executable, and — the T-70 check — that every `ci/*.sh` has at least one caller in a workflow or the Makefile. Deliberately not `actionlint`: it is better at expression syntax and action versions, but it would be a fourth pinned binary for every contributor, and the two properties that actually broke here (a workflow calling a missing script; a script nobody calls) need to know what this repo's `ci/` means. `gopkg.in/yaml.v3` was already a direct dependency, so this added nothing to install.

**What a release actually publishes**, per `.goreleaser.yaml`: 5 platforms (linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64), a `checksums.txt` of SHA256 sums, and git-derived release notes. **SBOM generation and cosign signing are configured but commented out** — they need `syft` and `cosign` installed in the release job, which is why the release path is a script that can grow those steps rather than an action invocation that cannot. The workflow's `id-token: write` permission is already in place for keyless signing when they are enabled.

**Distribution channels:**
- GitHub Releases — primary channel, binaries + SHA256SUMS. Signing and SBOM are configured-but-disabled in `.goreleaser.yaml`, not shipped; don't describe releases as signed until those blocks are uncommented and the release job installs `cosign` and `syft`.
- `go install github.com/postmeridiem/pql/cmd/pql@latest` for developers with a Go toolchain.
- `pql self-update` once v0.1 ships — hits the GitHub Releases API, downloads + replaces atomically, verifies SHA256.

## Observability and docs discipline

- **Telemetry on `--verbose`:** `internal/telemetry/` injects per-phase timings (`generate_ms`, `rank_ms`, per-signal ms) into the stderr diagnostic stream. This is how we'll tune weights honestly.
- **Schema-version negotiation:** `pql version --build-info` emits `{version, commit, date, go_version, schema_version}`. The skill checks `schema_version` against its own and refuses on mismatch with a remediation hint. Contract documented in `docs/compatibility.md`.
- **Catalog docs** sit at the top of `docs/`:
  - `docs/intents.md` — intent catalog + per-intent contract.
  - `docs/signals.md` — every signal: what it measures, where it shines, where it fails.
  - `governance/{decisions,questions,rejected}/<domain>.md` — the DQR tree, parsed by `pql decisions sync`. Per D-21; the flat `decisions/` this line used to name no longer exists. Domain is inferred from the filename stem, record type from the parent subdirectory.

## Verification

End-to-end once each milestone lands:

1. `make lint` + `make test` + `make test-race` → all green.
2. `make build` → `./bin/pql --version` prints stamped build info; `./bin/pql version --build-info` prints JSON including `schema_version`.
3. `make snapshot` → `dist/` contains 5 platform archives with checksums.
4. `make test-integration` against `testdata/council-snapshot/` → exit codes 0/65/66 all exercised (zero matches is part of `0`).
5. `make eval` with a seeded 3-query golden set → produces NDCG@5/MRR/P@5 report + per-signal contribution table.
6. Push a throwaway branch → `ci/lint.sh` + `ci/test.sh` complete locally under 5 min (the host pipeline shells out to these, so local-equals-CI by construction).
7. Release rehearsal: `make snapshot` locally for the full 5-platform build without publishing. The real path is not a tag push — `release.yaml` triggers on a push to `main` whose CHANGELOG section for `project.yaml`'s current version carries a date, then mints and pushes the tag itself and runs `./ci/release.sh`. So the release signal is the dated-section commit, and an undated section is a no-op by design.
8. Install binary + skill on a clean machine; run `pql files` in the Council vault → feels like a query engine (plain rows). Run an intent (`pql related <path>`) → same substrate, now with `connections[]` and `signals[]`. Confirms the simple-but-optionally-enriched surface.
9. Run an intent command (`pql related members/vaasa/persona.md`) with and without `--flat-search`: without the flag returns enriched bundle; with the flag returns the bare query result and zero connections/provenance. Confirms the off-switch is reachable from every entry point.
