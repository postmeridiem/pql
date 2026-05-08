# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The current section header tracks `project.yaml`'s `version:` field — both
move together. On release, the post-release commit bumps `project.yaml`'s
version and renames the matching section here to the released version with
a date (e.g. `## [0.1.0] - 2026-05-01`), then opens a new working section
matching the bumped version (e.g. `## [0.1.1-dev]`).

## [1.4.25] - 2026-05-08

### Changed

- The post-merge hook now also runs `pql decisions sync` after
  `pql plan import`, so a pull that brought in `decisions/*.md`
  edits propagates to pql.db's decisions and decision_refs tables
  without a manual follow-up. Closes the symmetric gap that the
  init-time decisions-sync fix already addressed; T-25's scenario
  5 (decision divergence) now passes without intervention.

## [1.4.24] - 2026-05-08

### Added

- `pql init` now runs `pql decisions sync` automatically as a
  separate step so a fresh clone lands with both halves of the
  schema populated — the changelog-replicated tables via the
  existing autoImportPlan path, and the markdown-sourced decisions
  + decision_refs tables via the new step. No more "init then run
  decisions sync separately" follow-up. Skipped silently when the
  vault has no `decisions/` directory (tickets-only setups).
- `decisions_sync` field added to the init JSON result so callers
  can see how many records / refs landed and whether the parser
  flagged any broken cross-references.

## [1.4.23] - 2026-05-08

### Changed

- T-16 implementation tree closed at the dev-repo round-trip
  level: T-24 (schema-migration generation) folded into T-22 +
  D-19 — the per-table `0000-schema.sql` already carries version
  markers and the no-ALTER stance from D-19 made numbered
  schema-NNN files unnecessary. T-25 (clide integration test
  scenario) flagged ready for a manual validation pass against
  the postmeridiem/clide repo.

## [1.4.22] - 2026-05-08

### Changed

- `pql init`'s autoImportPlan step now prefers `.pql/changelog/`
  over `.pql/pql-plan.json` (T-23). Bootstrap order: changelog
  replay if changelog files exist → legacy JSON import if they
  don't → empty pql.db otherwise. This makes a fresh clone of an
  upgraded repo populate pql.db from the committed changelog
  without manual `pql plan import` invocation.
- This repo's own `.pql/pql-plan.json` removed (now redundant — the
  changelog files committed in v1.4.18+ carry the full state, and
  `pql init` recovers from them via the new autoImportPlan path).

## [1.4.21] - 2026-05-08

### Added

- `pql init` now bootstraps the full replication lifecycle (T-22,
  D-18):
  - Creates `.pql/changelog/<table>/` for every replicated planning
    table (tickets, ticket_deps, ticket_labels, ticket_history) and
    plants `0000-schema.sql` in each — full CREATE statements with
    a `pql:created_by` / `pql:canonical_version` header. The
    importer parses those markers and refuses replay when the
    declared canonical_version mismatches this binary, catching
    schema drift across pql versions before it can corrupt state.
  - Installs `post-checkout` (rebuild on branch switch only,
    `$3 == 1`) and `post-rewrite` (rebuild on rebase / amend) hooks
    alongside the existing pre-commit and post-merge ones. Both new
    hooks call `pql plan rebuild` with a visible status message —
    branch-switch cleanup is no longer hidden behind random later
    slowdowns.
  - Updates the post-merge hook to call `pql plan import` (the new
    changelog-replay path) instead of the legacy JSON-snapshot
    diff-and-import logic.
  - Appends `.pql/changelog/*.sql merge=union` to `.gitattributes`
    so same-line conflicts on the changelog (rare; `updated_at`
    distinguishes lines) resolve as a union — both sides land and
    the inline LWW guard sorts it out at replay.
- Hook installer now upgrades an existing pql-managed block in place
  while preserving any user customizations outside it.

### Changed

- Replay (`pql plan import` / `pql plan rebuild`) now disables foreign
  keys for the duration of the SQL execution. Per-directory replay
  ordering doesn't match FK dependency order, and the data being
  replayed was already FK-valid when written — re-enforcing buys
  nothing and breaks bootstrap.

## [1.4.20] - 2026-05-08

### Added

- `pql plan rebuild` — truncate the replicated planning tables
  (tickets, ticket_deps, ticket_labels, ticket_history) and replay
  every file under `.pql/changelog/` from scratch (T-21, D-18).
  Used by the post-checkout and post-rewrite hooks to handle the
  case incremental replay can't: rows that existed on the previous
  branch but not the new one. Decisions and decision_refs are
  markdown-sourced (D-8) and are intentionally left untouched —
  follow rebuild with `pql decisions sync` if the decisions/ tree
  changed.

## [1.4.19] - 2026-05-08

### Changed

- `pql plan import` now replays `.pql/changelog/` files into pql.db
  by default, instead of unmarshalling a single `pql-plan.json`
  (T-20, D-15). Per-file mtime check skips already-replayed
  content; inline LWW guards from T-19 make replay idempotent and
  order-free, so the same file can be replayed any number of
  times without duplicates or stale overwrites. New `--legacy
  <path>` flag preserves the old JSON-snapshot import path for
  upgrading repos that still only have `pql-plan.json` (T-23 will
  automate this).
- New `last_import_marker` key in the `meta` table tracks per-replica
  replay progress.

## [1.4.18] - 2026-05-08

### Changed

- `pql plan export` now writes per-table monthly SQL upsert files
  under `.pql/changelog/<table>/<YYYY-MM>.sql` instead of a single
  `pql-plan.json` snapshot (T-19, D-15). Files are append-only,
  byte-deterministic across replicas, and carry inline LWW guards
  (`WHERE excluded.updated_at > target.updated_at OR …`) so replay
  is order-free and idempotent (D-16). `--stage` flag now `git add`s
  the touched changelog files instead of the snapshot. Decisions and
  decision_refs are intentionally excluded — they are markdown-sourced
  per D-8 and travel with their `.md` files. The `--to` flag is
  removed (no single output path under the new format).
- New `meta` table for replica-local state (export/import markers).
  Not part of replication; ignored by `verifySchema`.
- `ticket_history.hash` gains a `UNIQUE` constraint so replay can
  dedupe identical audit events idempotently via
  `ON CONFLICT(hash) DO NOTHING`.

## [1.4.17] - 2026-05-08

### Changed

- **Breaking for in-place upgrades.** The pql.db schema runner (D-6)
  is retired in favour of a single CREATE-TABLE-only schema (D-19,
  supersedes D-6). pql.db files built under earlier internal builds
  (including v1.4.16) cannot be upgraded in place — recover via
  `rm .pql/pql.db && pql plan import` (the existing `pql-plan.json`
  is the source of truth). The migration runner, `schema_migrations`
  table, and `goFunc` migration steps are removed; schema changes
  going forward update the CREATE statements and bump
  `CanonicalVersion`.
- Soft deletes via `deleted_at` column on every planning table
  (T-18, D-17). User-facing delete operations (`pql ticket unblock`,
  `pql ticket label rm`) now soft-delete; re-attaching (`block`,
  `label add`) resurrects the row by clearing `deleted_at`. Read
  queries default-filter `WHERE deleted_at IS NULL`. The canonical
  hash projection now includes `deleted_at`, so a soft-delete event
  changes the row's hash and replays through the changelog as a
  state change rather than as a missing row.

## [1.4.16] - 2026-05-08

### Changed

- Planning database schema migrates to v2 on first invocation after
  upgrade. Every planning table grows `hash` (MD5 of the canonical
  row projection) and `canonical_version` (= 1) columns; tables
  that lacked them also get `created_at`/`updated_at` with sensible
  defaults. Hashes are backfilled in the same migration
  transaction. Foundation for the upcoming changelog-based
  replication (D-15..D-18, T-16 tree) — repo writes now keep the
  hash column current on every INSERT/UPDATE so future export can
  rely on it.

## [1.4.15] - 2026-05-06

### Changed

- The clean-house skill no longer carries its own version. It
  ships embedded in the pql binary, so its version is the pql
  version. Catalog and procedure churn are tracked here (and in
  `git log internal/skill/clean-house/`) instead of in a parallel
  changelog inside `references/rules.md`. SKILL.md's output
  template drops the `v1.1` label.

## [1.4.14] - 2026-05-06

### Changed

- clean-house skill bumped to v1.1, surfacing fixes from the first
  end-to-end run on this repo:
  - `references/rules.md`: RULE-ANCHOR-DRIFT detection rewritten to
    use file-level slug indexes (anchor-only links resolve against
    the source file's full heading set, not a single record's body
    headings — the previous logic produced false positives because
    record-level `### D-N: …` headings live as siblings in the
    file). RULE-DEAD-FILE-REFERENCE detection tightened with
    placeholder filtering, source-relative resolution, and a
    basename-fallback before flagging — the previous regex
    produced 6/6 false positives. Each rule now declares a
    `Finding ID:` so the state file can recognize the same
    finding across runs.
  - `SKILL.md`: new step 3 "Probe conventions" reads
    `decisions/.clean-house.yaml` (or infers heading depth /
    backlink phrasing); step 5 explicitly filters
    `legacy/`-prefixed records; step 7 defines "stop asking" as
    a global halt and "show diff first" as a re-prompt loop;
    step 9 merges the skip ledger with run history into a single
    `decisions/.clean-house-state.md` so promotion-candidate
    computation has real data to compute against.

## [1.4.13] - 2026-05-06

### Changed

- `pql skill install` (no `--user`) now auto-resolves which scope to
  operate on. If any bundled skill is already installed at user-scope
  (`~/.claude/skills/<name>/`), the whole suite installs/updates
  there. Otherwise installs at project-scope. The first install at
  user-scope must still be done explicitly via `pql skill install
  --user`.
- When auto-resolving to user-scope, a pristine project-scope
  install (state in {current, stale, missing}) is tidied up
  automatically. Hand-edited or unknown project-scope content is
  preserved with a note.
- `pql skill status`, `pql init`'s skill step, and the per-skill JSON
  output gain a `scope` field (`"user"` | `"project"`) so callers
  can tell which install they're looking at.
- `pql skill uninstall` (no `--user`) operates on project-scope only;
  removing user-scope still requires `--user` explicitly. Avoids the
  surprise of a random uninstall wiping out global state.
- `pql doctor` reports both scopes per skill (`user` and `project`
  fields) so drift between them stays visible.

## [1.4.12] - 2026-05-06

### Added

- The clean-house skill now ships in the binary alongside the pql
  skill. `pql skill install` (and `pql init`'s skill step) install
  both — pql's skill at `.claude/skills/pql/`, clean-house at
  `.claude/skills/clean-house/` with its `references/rules.md`
  bundled in.
- `pql skill` package surface gains `Bundled`, `ByName`, `InspectAll`,
  `InstallAll`, `UninstallAll`, plus per-skill `(*Skill).Inspect/Install/Uninstall`.
  Hash is now per-bundle (sha256 over sorted `path\x00content`
  pairs) so editing a reference file marks the skill modified, not
  just edits to SKILL.md.

### Changed

- **Breaking JSON output:** `pql skill status`, `pql skill install`,
  and `pql skill uninstall` now emit a JSON array of skill statuses
  (one per bundled skill), not a single object. `pql doctor`'s
  `skill` field is now `skills` and is also an array. Each status
  carries a `name` field. Consumers parsing the singular form need
  to update.
- `pql init`'s output `skill` field is now `skills` (array). The
  interactive prompt covers all bundled skills together — one
  question, one decision — instead of per-skill prompts.

## [1.4.11] - 2026-05-06

### Added

- `pql decisions read` JSON output now includes a `headings` array
  with `{level, text, slug}` for each ATX-style heading in the
  decision's body. Slugs follow the GitHub-flavored markdown anchor
  convention (lowercase-hyphen, underscores preserved, duplicates
  disambiguated with -1/-2/...). Foundation for anchor-resolution
  tools — e.g. the upcoming clean-house skill's RULE-ANCHOR-DRIFT.

## [1.4.10] - 2026-05-06

### Changed

- `pql decisions show` now routes through a `buildDecisionTree`
  helper that mirrors `buildShowTree` for tickets — same factor-out
  pattern, single place for future decision-anchored verbs to render
  through. Output shape is unchanged.

## [1.4.9] - 2026-05-06

### Fixed

- `pql init` now honors `core.hooksPath` when planting the pre-commit
  / post-merge shim. Repos that redirect git's hook directory (often
  to a tracked `.githooks/`) used to get a dead shim under
  `.git/hooks/` that git would never invoke; the export-on-commit
  flow silently no-op'd as a result. Re-run `pql init` after the
  upgrade to plant the shim in the right place.

## [1.4.8] - 2026-05-06

### Changed

- **Breaking JSON output:** `pql plan whatsnext` and `pql plan review`
  now emit the same shape as `ticket show` / `ticket refine next` —
  ticket fields are promoted to the top level instead of nested under
  `"ticket": {...}`. Optional `ancestors`, `decisions`, `blockers`,
  `children`, `message` siblings. The empty-queue case renders as
  `{"message": "..."}`. Consumers parsing the old `ticket` envelope
  need to flatten one level.

## [1.4.7] - 2026-05-06

### Added

- `pql plan export --stage` runs `git add` on the exported file after
  writing. Idempotent across untracked / tracked / unchanged states.

### Changed

- The pre-commit and post-merge hooks installed by `pql init` now bake
  the absolute path of the pql binary into the script (resolved from
  `os.Executable()` at install time). Git's hook shell does not
  inherit the user's interactive PATH, so the prior `command -v pql`
  guard could silently no-op. Re-run `pql init` after moving the
  binary.
- The pre-commit hook calls `pql plan export --stage` instead of
  shelling around bare export with a separate `git add`. One verb,
  one place to maintain.

## [1.4.6] - 2026-05-06

### Fixed

- `pql init`'s pre-commit hook now stages `.pql/pql-plan.json` even on
  the first commit after install. The previous gate (`git diff
  --quiet`) treated untracked files as unchanged and silently skipped
  the bootstrap snapshot.

## [1.4.5] - 2026-05-06

### Added

- `pql ticket refine list` / `next [--skip N]` / `write <id> <json|--file|--stdin>` —
  surfaces tickets with empty descriptions and lets a writer patch
  `title`, `description`, `priority`, or `type` via JSON. `refine next`
  returns the same join-tree as `ticket show --with-context
  --with-blockers --with-children`, plus a `refinement` envelope with
  remaining/skipped counts.

## [1.4.0] - 2026-04-23

### Added

- `pql decisions read <id>` — returns a decision/question/rejected
  record with its full markdown body extracted from the source file.
  All existing metadata fields are included alongside the new `body`
  field.
- `pql init` installs a post-merge hook that auto-imports planning
  state when `.pql/pql-plan.json` changes on pull/merge. Companion
  to the existing pre-commit export hook.
- `pql plan whatsnext` — surfaces the single best ticket to work on
  (in-progress first, then highest-priority ready) with ancestor tree,
  children, and linked decisions for instant context. Review tickets
  are excluded to prevent author-reviews-own-work.
- `pql plan review` — surfaces the next ticket awaiting review, with
  the same context bundle as whatsnext.

### Changed

- `pql ticket show` replaces `--with-decision` with `--with-context`,
  which includes ancestors, children, and all linked decisions from
  the chain (same context bundle as whatsnext/review).
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
  planning subcommands. See `decisions/architecture.md (D-3)`.
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
