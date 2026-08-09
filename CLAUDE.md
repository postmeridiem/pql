## CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**Shipped and active** (current version in `project.yaml`; per-version detail in `CHANGELOG.md`). The query surface (`pql files|tags|backlinks|outlinks|meta|schema` + the `query` DSL), the enrichment layer (`internal/connect/{signal,rank,neighborhood,bundle}`), the intent surfaces (`internal/intent/{context,related,search}`), and the full planning surface (`internal/planning/…` backing `pql decisions`, `pql ticket`, `pql plan`) are all implemented. New packages still land alongside their first feature rather than sitting empty. Open work lives in the planning dataset, not a TODO file — `pql ticket list --status backlog` and `pql decisions list --type question --status open`.

**Read these before designing or writing code:**
- `project.yaml` — single source of truth for project metadata: name, description, current declared version, license, repo, module path, schema_version, maintainers. The Makefile sources `VERSION` from here; the skill checks `schema_version` against this. Bump fields here rather than scattering them.
- `docs/structure/design-philosophy.md` — binding "why" doc. The binary is a *ranker* with intent-level surfaces. Generate vs rank as separate phases. Provenance is data. Two stores (cache + user-state, per D-3). No vectors. Narrow scope.
- `docs/structure/project-structure.md` — canonical layout, build pipeline, test infrastructure, growth model.
- `docs/structure/initial-plan.md` — original v1 plan (PQL grammar, SQLite schema, CLI specifics). Some framing superseded by the philosophy + structure docs; grammar/schema/exit-codes still authoritative.
- `docs/structure/planning.md` — spec for `pql decisions` / `pql ticket` / `pql plan`. First real writer to the user-state DB (`pql.db`), distinct from the cache (`index.db`).
- `governance/decisions/architecture.md (D-3 + D-19)` — the cache (`index.db`) vs user-authored state (`pql.db`) split, and how pql.db evolves (no migration runner today; see below). Read before touching anything that persists under `.pql/`.
- `docs/vault-layout.md` — the three vault-level conventions (`.pql/config.yaml`, `.pqlignore`, `.pql/`). The index defaults to `<vault>/.pql/index.db` (in-vault, like `.git/`); falls back to user cache on read-only vaults. Planning state lives beside it at `<vault>/.pql/pql.db` — no cache fallback, writes to a read-only vault fail cleanly.
- `docs/pqlignore.md` — gitignore-compatible exclusion file spec.
- `docs/watching.md` — `pql watch` toggle command spec. Explicit user invocation only; one watcher per vault; no daemon, no cross-vault registry.

## Naming

The project is **`pql` / PQL (Project Query Language)**. The pre-rename name was `mql` / MQL — never use it in new code, docs, or commit messages. The `git-commit` skill enforces this on commits.

Hosted at https://github.com/postmeridiem/pql. Module path: `github.com/postmeridiem/pql`.

## This repo is public — keep the environment out of it

Everything committed here is published and indexed, including
`.pql/changelog/`. That last one is the trap: the changelog is committed *by
design*, because it is how tickets travel with a clone — so **ticket prose is
published prose**, even though typing `pql ticket new` feels like a local note.

Most bug reports here come from running pql on a real machine, so describe the
**condition, not the machine**:

| Do not commit | Write instead |
|---|---|
| a hostname | "a host where pql was installed from the release binary" |
| `/home/<user>/.local/bin/pql` | `~/.local/bin/pql` |
| a private repo or service name | "a consuming repo", "the vault" |
| an internal domain, IP or port | omit, or describe the role |

The condition is also the more useful report: a maintainer needs to know the
binary was outside `PATH`, not which box it was on.

Check before pushing, not after — once published it is cached and indexed even
if deleted:

```bash
git diff @{u}..HEAD | grep -nEi '<hostname>|/home/[a-z]+|<private-repo-names>'
```

## Architecture invariants

These are load-bearing — preserve them when implementing. Full rationale in the docs above.

- **Query → connect → bundle pipeline.** Primitive query rows are the spine. Enrichment (`internal/connect/{signal,rank,neighborhood,bundle}`) is an optional pass on top, attaching `connections[]` and per-result `signals[]` provenance. The DSL bypasses enrichment by default (raw rows = escape hatch).
- **`--flat-search` global flag** forces the primitive path on any subcommand, including intents that default to enriched. Single short-circuit in `cli/` (`runIntent`) — no per-subcommand wiring.
- **Generate vs rank as separate packages.** Neither imports the other. Generation is wide and cheap; ranking is bounded and careful.
- **Provenance is data, not a cross-cutting concern.** Each signal returns `Contribution{Name, Raw, Normalized, Weight}`; the combiner aggregates. Don't centralize into an `explain.go`.
- **Consumer-agnostic core.** `internal/intent/`, `internal/query/`, and `internal/planning/` must not import `internal/cli/`. CLI today, MCPs (plural) tomorrow — a query-surface MCP and a planning-surface MCP are different scopes, different permissions, different audiences. Every consumer is an adapter.
- **Two stores, two regimes.** `<vault>/.pql/index.db` is a pure cache — anything in it must be regenerable from the vault; on `schema_version` mismatch, drop and rebuild (no migration code). `<vault>/.pql/pql.db` is user-authored planning state. It has **no migration runner today** (D-19): schema is `CREATE TABLE IF NOT EXISTS` in `internal/planning/schema.go` with a `CanonicalVersion`, and pql.db is regenerated from the committed `.pql/changelog/` (D-15) rather than altered in place — recovery is `rm .pql/pql.db && pql plan rebuild`. Forward migrations are a planned step before third-party distribution (D-19 amendment, tracked by T-44); don't add a migration framework before then. See `governance/decisions/architecture.md (D-3 + D-19)`.
- **Output contract is load-bearing for agents.** stdout JSON, stderr JSON-per-line diagnostics, exit codes 0/64/65/66/69/70 — distinguished. Zero matches is success: exit `0` with an empty `[]`, never a distinct code (callers treat non-zero as failure). Exit `2` was retired per D-22. See `internal/diag/diag.go` and `docs/output-contract.md`.

## Build & test

Go 1.25+ is installed on the dev machine (`go.mod` declares the floor); `make build`, `make test`, and the rest of the targets below run locally.

| Command | Does |
|---|---|
| `make build` | binary at `./bin/pql` with version stamped via ldflags |
| `make test` | unit tests, fast |
| `make test-race` | unit tests with `-race` |
| `make test-integration` | `go test -tags=integration ./internal/cli/...` against fixture vaults (`internal/cli/integration_test.go`) |
| `make eval` | ranking-quality eval, `go test -tags=eval ./internal/connect/rank/...` (goldens under `testdata/golden/`) |
| `make lint` | `golangci-lint run` |
| `make vuln` | `govulncheck ./...` (pinned to v1.2.0 via `go run`, no local install) |
| `make pre-push` | lint + vuln + test + test-race. Wired by `.githooks/pre-push`; integration suite is deliberately excluded to keep the push gate fast |
| `make snapshot` | `goreleaser release --snapshot --clean` (builds 5 platforms, no publish) |

`make help` lists everything.

CI substance lives in `ci/{lint,test,release,eval}.sh`. GitHub Actions workflows in `.github/workflows/` are thin wrappers around these — keeps local and CI behaviour identical and lets the provider be swapped without rewriting the scripts.

**Pre-push hook.** Opt in once per clone with `git config core.hooksPath .githooks`. The hook runs `make pre-push`; a failing check aborts the push locally so nothing reaches the remote.

**Hook setup ordering.** Only `.githooks/pre-push` is tracked. The replication shims (`pre-commit`, `post-merge`, `post-checkout`, `post-rewrite`) are per-clone and planted by `pql init` into whatever `core.hooksPath` resolves to (T-52). So set `core.hooksPath .githooks` **before** running `pql init` — otherwise init plants them into `.git/hooks/` and they never fire under the redirected path. A contributor who doesn't use pql can skip `pql init` entirely and the repo still works.

**One command per invocation.** Claude Code is documented to split compound
commands on `&&`, `||`, `;`, `|` and newlines and match each segment against
`.claude/settings.json` independently — so a pipeline whose every part is
allowlisted *should* run unprompted. In practice piped commands still prompt
([claude-code#39438](https://github.com/anthropics/claude-code/issues/39438)),
so treat one-command-per-invocation as a workaround for that bug rather than a
property of the design, and revisit when it closes. Prefixing genuinely does
break matching, and always will: `PATH=… make build` starts with `PATH=`, so it
matches no `Bash(make *)` rule. Consequences worth knowing:

- Reach for a flag before a pipe (`--limit`, `--fields`, `--oneline`, `--pretty`).
- If something needs shell glue more than once, it belongs in a Makefile target
  or a script under `.claude/skills/*/scripts/`, not in an ad-hoc invocation.
  `make skill-drift` and `make scratch-vault` both started as one-liners that
  tripped this.
- Never blanket-allow an interpreter (`python3 *`, `sed *`, `gofmt *`) to get
  around it — that trades a prompt for an unbounded write grant. Write the
  repeatable thing instead.

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
| New pql.db table | `CREATE TABLE IF NOT EXISTS` in `internal/planning/schema.go`, bump `CanonicalVersion`, + repo helpers | schema edit + repo additions (no migration runner — D-19) |
| New consumer (MCPs) | `cmd/pql-mcp-query/` reusing `internal/intent/`+`internal/query/`; `cmd/pql-mcp-plan/` reusing `internal/planning/` | enabled by consumer-agnostic core discipline; query and planning ship as separate binaries |
