# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The current section header tracks `project.yaml`'s `version:` field — both
move together. On release, the post-release commit bumps `project.yaml`'s
version and renames the matching section here to the released version with
a date (e.g. `## [0.1.0] - 2026-05-01`), then opens a new working section
matching the bumped version (e.g. `## [0.1.1-dev]`).

## [1.4.0] - 2026-04-23

### Added

- `pql decisions read <id>` — returns a decision/question/rejected
  record with its full markdown body extracted from the source file.
  All existing metadata fields are included alongside the new `body`
  field.

### Changed

- Ticket status transitions are no longer enforced by pql. Any valid
  status can move to any other valid status; callers can layer their
  own transition rules on top.

## [1.3.1] - 2026-04-23

### Fixed

- Decision list sorts by type prefix (D, Q, R) first, then by number.
  Previously numeric-only sort interleaved record types.

## [1.3.0] - 2026-04-23

### Fixed

- Reference parser no longer matches substrings like "D-3" inside
  "BSD-3-Clause" or "Q-1" inside "FAQ-1". Added `\b` word boundaries
  to the `refIDRe` regex.
- `pql decisions sync` now prunes stale records that no longer exist
  in the markdown source, preventing ghost entries after ID renames.

### Changed

- Record IDs are no longer zero-padded. `NextID` and `nextTicketID`
  produce `D-14`, `T-44` instead of `D-014`, `T-044`. Existing
  zero-padded IDs in pql.db are accepted; owners clean up at their
  own pace.
- SQL `ORDER BY id` replaced with numeric sort
  (`CAST(SUBSTR(id, 3) AS INTEGER)`) so IDs above 999 sort correctly.
- `pql plan export` writes to `.pql/pql-plan.json` (was `pql-plan.json`
  at vault root). `pql plan import` falls back to the legacy location.
- `pql init` gitignore entry changed from `.pql/` to `.pql/*` with
  explicit includes for `.pql/pql-plan.json` and `.pql/hooks/`.

### Added

- `pql ticket setparent <id[,...]> <parent-id | none>` — set or clear
  a ticket's parent after creation. Supports batching. Idempotent:
  no-op if the parent already matches.
- Pre-commit hook auto-installed by `pql init` at `.pql/hooks/pre-commit`.
  Runs `pql plan export` and stages the file if it changed. A thin shim
  in `.git/hooks/pre-commit` sources it.
- Schema test expectations now track `len(migrations)` instead of a
  hardcoded version number.

## [1.2.0] - 2026-04-22

### Added

- `pql plan export` and `pql plan import` — snapshot planning state
  to a committed JSON file (`pql-plan.json` by default) for version
  control. pql.db stays gitignored; the export is the portable,
  diffable artifact. Wire the trigger to your workflow: pre-push hook,
  sprint skill, or manual.

## [1.1.0] - 2026-04-22

### Changed

- Ticket subcommands `status`, `assign`, `team`, and `label` now accept
  comma-separated IDs for batch operations:
  `pql ticket status T-001,T-002,T-003 done`. Single IDs still work
  unchanged. Batch results return an array; single results return an
  object (preserving backward compatibility).

## [1.0.0] - 2026-04-22

### Added

- Enrichment layer (`internal/connect/`): five signals (link_overlap,
  tag_overlap, path_proximity, recency, centrality), max-normalization,
  intent-specific weight profiles, one-hop neighborhood connections.
- Intent subcommands: `pql related <path>`, `pql search <query>`,
  `pql context <path>`. Each returns ranked, provenance-carrying results
  with `signals[]` and `connections[]`.
- `--flat-search` global flag forces the primitive path on any subcommand,
  bypassing enrichment entirely.
- `pql self-update` — downloads latest release from GitHub, verifies
  SHA256, replaces atomically.
- Per-phase telemetry on `--verbose` (config, store_open, index, enrich).
- Ranking-quality eval harness (`go test -tags=eval`): NDCG@k, MRR,
  P@k against golden test cases.

## [0.2.0] - 2026-04-22

### Added

- `TODO.md` at the repo root: forward-looking work queue. Companion
  to this file — the changelog records what shipped under each
  version header, `TODO.md` records what's next, so a fresh session
  has a clean starting point without grepping commit history.
- `make pre-push` target and `.githooks/pre-push` wrapper running
  lint + vuln + test + test-race. Opt in per clone with
  `git config core.hooksPath .githooks`. Integration tests are
  deliberately excluded to keep the gate fast; they belong in CI.
- `make install-dev` target — symlinks `$(INSTALL_DIR)/pql-dev` to
  the repo's `./bin/pql` so the dev binary tracks every `make build`
  without re-copying. Companion to `make install`, which stays a
  plain copy for the stable binary.
- `pql shell` — interactive REPL for PQL DSL queries. Indexes the
  vault once at startup, then reads one query per line. Prompt shown
  only on TTY; piped input works silently. Blank lines, `--` comments,
  `exit`/`quit`, and Ctrl-D all behave as expected. Respects
  `--pretty`/`--jsonl`/`--limit`.
- `pql base <name>` — compile and run an Obsidian `.base` YAML file
  as a PQL query. Discovers `.base` files at the vault root; `--view`
  selects a named view (default: first). `pql base` with no arguments
  lists available bases. Filters, properties, and sort compile to the
  same AST the DSL uses, so all output and exit-code contracts apply.
- `pql completion {bash,zsh,fish,powershell}` — generates shell
  completion scripts. Source directly (`eval "$(pql completion bash)"`)
  or save to the appropriate shell-specific location.
- `internal/planning/` package skeleton with `pql.db` schema and
  forward-only migration runner. Six tables (decisions, decision_refs,
  tickets, ticket_deps, ticket_history, ticket_labels) for the
  planning subcommands. See `docs/adr/0003-pql-db-for-user-state.md`.
- `pql decisions` subcommand tree: `sync`, `validate`, `claim`,
  `list`, `show`, `coverage`, `refs`. Parses `decisions/*.md` using
  the D/Q/R-NNN heading convention, upserts into `pql.db`, and
  queries with filters and joins. Compatible with databases created
  by the Python stopgap.
- `pql ticket` subcommand tree: `new`, `list`, `show`, `status`,
  `assign`, `block`/`unblock`, `team`, `label`, `board`. Full
  lifecycle from backlog to done with state-machine enforcement,
  history tracking, decision joins, and kanban board view.
- `pql plan status` — planning dashboard showing decision counts,
  open questions, ticket summary by status, and coverage gaps.

- `pql watch start|stop|status` — filesystem watcher that keeps the
  index hot by reindexing on file changes. Foreground process with
  250ms debounce. One watcher per vault, explicit start/stop control.
  See `docs/watching.md`.
- `outlinks`, `inlinks`, and `headings` are now usable in the PQL DSL
  as array columns: `'brief' IN outlinks`, `'file.md' IN inlinks`,
  `'Title' IN headings`. Supports `NOT IN` as well. Inlinks use
  pragmatic resolution (full path, basename, basename without .md).

### Changed

- Embedded Claude Code skill (`internal/skill/SKILL.md`) updated to
  cover both vault-query and planning surfaces. Consumers running
  `pql skill status` will see `stale`; `pql skill install` updates.

- `make vuln` now runs `go run golang.org/x/vuln/cmd/govulncheck@v1.2.0`
  instead of requiring a local `govulncheck` install. Version is
  pinned in the Makefile so every dev (and the pre-push hook) runs
  the same checker.
- `.golangci.yaml` migrated to v2 schema (the locally-installed
  linter is v2.11.4 and v2 dropped v1-config support). Renamed
  `render.RenderOne` → `render.One` per the `stutters` rule; 70
  pre-existing findings cleared in one pass so `make lint` now
  exits 0.
- Default index filename renamed `index.sqlite` → `index.db` (and
  WAL sidecars `index.db-wal` / `index.db-shm`). `.db` is the more
  idiomatic SQLite extension; leaves room for a future non-cache
  `pql.db` store alongside the index without the extension carrying
  type information. `IndexFileName` in `internal/config/paths.go`
  and docs updated; override paths (`--db`, `PQL_DB`, `db:`) are
  unaffected.

## [0.1.0] - 2026-04-20

First milestone release. Read-only CLI + PQL DSL + Claude Code skill
scaffolding. Not yet distributed — `goreleaser` is wired but unrun;
no archives, no GitHub Release. Bumped purely to mark the line in the
sand: everything below this header is "what shipped in v0.1".

Backfilled from git history. Each bullet maps to one commit (or a tightly
related cluster). Not every refactor or doc-only change made the cut —
internal-only work is folded into adjacent feature lines where it's part
of the same user-visible story.

### Added

- `CHANGELOG.md` itself, in [Keep a Changelog](https://keepachangelog.com/)
  format. The git-commit skill now requires a CHANGELOG entry on every
  commit with user-visible impact; the working section header tracks
  `project.yaml`'s `version:` field exactly.
- Initial design plan, README, .gitignore, and Claude scaffolding (the
  `bootstrap` commit).
- MIT LICENSE.
- Foundational docs: `design-philosophy.md`, `project-structure.md`,
  CLAUDE.md.
- Documentation catalog stubs: `intents.md`, `signals.md`,
  `output-contract.md`, `compatibility.md`, `pql-grammar.md`,
  `skill.md`, plus ADRs `0001-no-vectors.md` and
  `0002-intents-not-primitives.md`.
- Build pipeline scaffolding: `Makefile`, `.editorconfig`,
  `.golangci.yaml`, `.goreleaser.yaml`, `ci/{lint,test,release,eval}.sh`.
- Initial Go module + minimal `pql` binary: `cmd/pql/main.go`,
  `internal/{cli,version,diag}` with `pql version` / `pql --version`.
- Claude Code project conventions: `.claude/settings.json`,
  `.claude/skills/{git-commit,skill-create}/`.
- GitHub Actions workflows for `ci` (lint + test + snapshot) and
  `release` (tag push → GoReleaser → GitHub Releases). Later disabled
  via `workflow_dispatch`-only triggers while iterating locally.
- Council vault snapshot as integration-test fixture under
  `testdata/council-snapshot/`, plus `make refresh-fixtures`.
- `project.yaml` — single source of truth for project metadata; the
  Makefile sources `VERSION` from it and stamps via ldflags.
- `internal/store` — pure-Go SQLite via `modernc.org/sqlite`, v1
  schema, WAL + foreign keys, drop-and-rebuild migration policy.
- `internal/config` — vault discovery (`--vault` → `PQL_VAULT` →
  `.obsidian/` ancestor → `.git/` ancestor → cwd-with-warning),
  `.pql.yaml` load with strict YAML decoding, fingerprinted DB path.
- Design docs: `docs/pqlignore.md` (gitignore-compatible exclusions),
  `docs/vault-layout.md` (`.pql.yaml` + `.pqlignore` + `.pql/`).
- `internal/index` walker with doublestar pruning of excluded
  directories. Forward-slash paths regardless of host OS.
- Markdown extractor (`internal/index/extractor/markdown`):
  frontmatter splitter + parser, `[[wikilinks]]` + embeds + standard
  `[md](links)`, `#tags` (inline + frontmatter), ATX headings.
  Code-fence-aware so links/tags inside fences don't get extracted.
- Schema v2 with explicit `type` column on the `frontmatter` table
  for type-aware introspection by `pql schema`.
- Indexer orchestrator (`internal/index/indexer.go`): walk → extract
  → upsert in a single `BEGIN IMMEDIATE` transaction. Mtime +
  content-hash incremental short-circuit; stale-row pruning;
  `index_meta.{config_hash,last_full_scan}` tracking.
- Design doc: `docs/watching.md` for `pql watch`. Later refined to
  explicit `start`/`stop`/`status` semantics with TTY-gated
  interactivity and the "not a daemon, not always-on, not a
  scheduler" stance triple-anchored.
- `internal/cli/render` — JSON, pretty JSON, and JSONL output
  formatters; generic over row type; honours `--limit`.
- `internal/query/primitives` package with `Files()`. (Tags,
  Backlinks, Outlinks, Meta, Schema, TagCount types follow with
  the matching subcommands below.)
- `pql files [glob]` subcommand. Persistent flags on the root for
  `--vault`, `--db`, `--config`, `--pretty`, `--jsonl`, `--limit`,
  `--quiet`, `--verbose`. Sentinel `exitError` type carries process
  exit codes through cobra's error path; mapping is 0 / 2 / 64 / 65 /
  66 / 69 / 70 per `docs/output-contract.md`.
- Shared `runQuery[T any]` helper for read-only subcommands —
  load config → open store → refresh index → render → exit-code
  mapping in one place.
- `pql tags` subcommand with `--sort tag|count` and `--min-count`.
- `pql backlinks <path>` subcommand. Resolution is pragmatic v1:
  matches full path, basename, or basename#anchor; self-references
  excluded.
- `pql outlinks <path>` subcommand. Document-order links from one file.
- `pql meta <path>` subcommand: per-file aggregate (frontmatter as raw
  JSON pass-through, tags, outlinks, headings). Adds `runQueryOne[T]`
  + `render.RenderOne[T]` helpers for single-record subcommands.
- `pql schema` subcommand: type-aware introspection across the vault
  (the original motivation for the explicit `type` column).
- `pql init` subcommand: seeds `.pql.yaml`, appends `.pql/` to an
  existing `.gitignore`. Later reshaped to be idempotent (see
  Changed).
- `pql doctor` subcommand: read-only diagnostic JSON of vault /
  config / DB / index / version state.
- PQL DSL lexer (`internal/query/dsl/lex`): tokenises source into
  typed tokens with line/col positions. Keywords, identifiers
  (quoted + unquoted), strings (with `''` escape), numbers,
  operators, comments. `pql.lex.<kind>` errors with positions.
- PQL DSL parser + AST (`internal/query/dsl/parse`): recursive-descent
  with standard precedence cascade. `IN <ref>` vs `IN <tuple>`,
  `BETWEEN`, `IS [NOT] NULL`, function calls, dotted/bracketed refs.
  `pql.parse.<kind>` errors with positions.
- PQL DSL → SQLite SQL compiler (`internal/query/dsl/eval/compile.go`):
  file columns (path/mtime/ctime/size), `name`/`folder` via SUBSTR,
  `fm.<key>` via type-dispatching subquery against the v2 schema,
  `'x' IN tags` via `EXISTS` subquery, full operator support.
  `pql.eval.<kind>` errors for unsupported references.
- DSL executor (`internal/query/dsl/eval/exec.go`) with `Row`
  shape, JSON-aware `normalise()` for value_json round-tripping.
  `reverse(text)` UDF registered on the store for the compiler's
  basename/dirname derivations.
- `pql query <DSL>` subcommand with `--file` / `--stdin` input modes.
- Claude Code skill: `internal/skill/SKILL.md` (frontmatter triggers,
  precondition check, schema-discovery preamble, ten-recipe cookbook,
  output-contract notes, anti-patterns, when-NOT-to-use). Embedded
  into the binary via `go:embed`. `internal/skill` package with
  `Inspect`/`Install`/`Uninstall` and four-state drift detection
  (`missing`/`current`/`stale`/`modified`/`unknown`) backed by a
  `.pql-install.json` lock file with SHA-256 hash.
- `pql skill {status,install,uninstall}` subcommands. `--user` flag
  targets `~/.claude/skills/pql/` instead of the project. `--force`
  on install overrides modified/unknown.
- `pql init` skill prompt: TTY-gated `--with-skill=yes|no|prompt`
  (default `prompt`); state-aware question (missing → install,
  stale → update, current → no-op, modified → preserve).
- `pql doctor` skill report: project-level skill state + embedded
  hash + version, surfacing drift without writing.
- Walker now consults `ignore_files` (default `[.gitignore]`).
  New `internal/index/ignore` package concatenates the named files
  in order then compiles via `github.com/sabhiram/go-gitignore`, so
  `!`-rules in a later file can re-include paths an earlier file
  excluded. Missing files are silently skipped. Excluded directories
  are pruned (not just filtered), matching the existing built-in
  exclude perf characteristics.

### Changed

- Project renamed `mql` → `pql` across docs and tracked artefacts.
  All `mql.yaml`, env vars, package paths, and Markdown→Project
  Query Language wording followed.
- README reframed twice: first as "the agent half of an Obsidian
  markdown-first workflow", then as "Claude-Code-skill-first" with
  Obsidian as the kept-because-we-use-it surface and a Dataview
  inspiration credit.
- PQL redesigned as a SQL-derived language. Earlier sketches were
  Dataview-compatible; the new grammar is `SELECT … FROM … WHERE …
  ORDER BY … LIMIT …` with a single canonical table (`files`),
  `fm.<key>` access, `IN tags` membership.
- `internal/cli` rebuilt around cobra; existing `version` subcommand
  preserved. Subcommand-per-file shape adopted.
- DB defaults moved to `<vault>/.pql/index.sqlite` (analogous to
  `.git/`); read-only vaults fall back to `<cache>/pql/<fingerprint>/`.
- `.gitignore` broadened after the first real `make snapshot`:
  added `.pql/`, `*.prof`/`*.pprof`, `go.work`/`go.work.sum`,
  `._*` (macOS resource forks).
- GitHub Actions auto-runs disabled — both workflows reduced to
  `workflow_dispatch` only while CI prerequisites stabilise.
- `pql init` is idempotent. Existing `.pql/config.yaml` is preserved
  (Skipped=true), not errored. `--force` opts into reseeding.
- Per-vault config moved from `<vault>/.pql.yaml` to
  `<vault>/.pql/config.yaml`. Anything pql lives under one dir,
  same way `.git/config` lives inside `.git/`.
- Replaced `respect_gitignore: bool` with `ignore_files: []string`
  (default `[.gitignore]`). Plural so users can deviate without
  re-copying their entire `.gitignore` — typical pattern is
  `[.gitignore, .pqlignore]` with a tiny pql-only file carrying just
  the deviations.
- Walker `builtinExcludes` trimmed to just `.git/` (matches git's
  own built-in exactly). Pql's `.pql/` is excluded via the
  user-config layer (`pql init` appends `.pql/` to project
  `.gitignore`); the user owns every other "what to skip" decision.

### Fixed

- Goreleaser `archives.format_overrides.format` → `formats: [zip]`
  per the v2 deprecation, unblocking `make snapshot`.
- Folder-derivation SQL was off-by-one
  (`members/vaasa/persona.md` → `members/vaas` instead of
  `members/vaasa`). Surfaced once the executor ran the compiled SQL
  end-to-end against the Council snapshot.

### Pending design (not yet implemented)

Moved to `TODO.md` — the changelog records what shipped under each
version header; forward-looking work lives there from now on.
