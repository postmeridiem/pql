# `pql` — Project Query Language for structured prose repos

A Go CLI that indexes and queries any repo containing Markdown files with YAML frontmatter, wikilinks, tags, and Obsidian Bases. Ships as a single static binary, maintains a SQLite index under `~/.cache/pql/`, exposes a Dataview-compatible query dialect, and is designed to be drop-in for AI agents (Claude Code, primarily) that need structural introspection without brute-force grep+read.

> **Status note (2026-04):** this document is the original v1 design. Some of its framing has been superseded by `design-philosophy.md` (the binary as a *ranker* with intent-level surfaces) and the project structure approved in `project-structure.md`. The PQL DSL described below remains valid as the "flat" / escape-hatch surface; intent-level commands sit *above* it. Read all three documents together.

## Context and problem

Markdown knowledge bases — Obsidian vaults, Zettelkasten, personal wikis, even well-structured documentation repos — are effectively small databases: YAML frontmatter is typed metadata, wikilinks are foreign keys, tags are labels, Obsidian Bases are saved queries. Off the shelf today:

- **Obsidian + Dataview** runs inside Obsidian. Great inside the app. Nothing outside it.
- **ripgrep / grep / find** are raw text search. They don't understand frontmatter, links, or tag semantics.
- **jq / yq** handle single structured files, not cross-file queries.
- **Code-aware tools** (tree-sitter, LSP, ast-grep) target source code, not prose.

There's a gap: **structural, cross-file querying of a prose-structured repository from outside any specific editor.** That's what `pql` fills. AI agents driving these repos today walk the tree by hand — grepping, reading, guessing — and burn context on work that should be one query. `pql` is the tool-shaped hole in that workflow.

## Non-goals (explicit)

- Not a Dataview replacement inside Obsidian. It *imitates* DQL from outside.
- Not a generic "query anything" tool. Code → use tree-sitter/LSP. Configs → use jq/yq. Raw text → use ripgrep. `pql` is for Markdown + frontmatter + wikilinks + tags + Bases.
- Not a write tool. Queries and introspection only. Edits go through the filesystem directly (which Obsidian's watcher picks up).
- No `dataviewjs` or JavaScript evaluation.
- No inline-field parsing (`Rating:: 5` mid-paragraph) in v1. Revisit if users demand it.
- No GUI, no web UI, no network daemon. CLI in, JSON out.

## What ships

Three artefacts, one repo:

1. **`pql` binary** — single static Go executable, cross-compiled to `linux/{amd64,arm64}`, `darwin/{amd64,arm64}`, `windows/amd64`. Distributed via GitHub Releases at https://github.com/postmeridiem/pql with SHA256 sums and cosign signatures.
2. **SQLite index** at `~/.cache/pql/<vault-fingerprint>.sqlite` (XDG-cache on Linux, `~/Library/Caches/pql/` on macOS, `%LocalAppData%/pql/` on Windows). Regenerable from the vault; never lives inside the vault.
3. **Claude Code skill** at `skill/SKILL.md` inside this repo, with install instructions. Dropped into any project's `.claude/skills/pql/` or the user's `~/.claude/skills/pql/`. Documents trigger phrases, common query recipes, anti-patterns, and the install check.

## Architecture

Go 1.22+, single module, no cgo.

> The package layout below is the original v1 sketch. The current canonical layout lives in `project-structure.md`, which splits `query/`, `connect/`, and `intent/` as separate concerns and adds `extractor/` registry, `telemetry/`, and `fixture/` packages. Use that document as the source of truth for new code.

```
pql/
├── cmd/pql/main.go              # CLI entrypoint, flag parsing, subcommand dispatch
├── internal/
│   ├── config/                  # .pql.yaml resolution, vault-root discovery, env handling
│   ├── index/                   # vault walker, frontmatter parser, link extractor, tag extractor
│   ├── store/                   # SQLite schema, migrations, CRUD, query helpers
│   ├── lex/                     # PQL lexer
│   ├── parse/                   # PQL parser → AST
│   ├── eval/                    # AST evaluator against the store
│   ├── base/                    # Obsidian .base YAML → PQL AST
│   └── render/                  # JSON / JSONL / table / CSV output
├── skill/SKILL.md               # Claude Code skill package
├── docs/
│   ├── structure/initial-plan.md    # this file
│   ├── pql-grammar.md               # language spec (written alongside the parser)
│   ├── skill.md                     # skill install + usage
│   └── cli.md                       # subcommand reference
├── examples/                    # example vaults + example queries for docs/tests
├── testdata/                    # fixture vaults for integration tests
├── .goreleaser.yaml
├── ci/                          # provider-neutral CI scripts
├── go.mod
├── Makefile
├── README.md
└── LICENSE
```

### Key dependencies

| Purpose | Package | Why |
|---|---|---|
| SQLite (pure-Go, no cgo) | `modernc.org/sqlite` | Static binary without a C toolchain |
| YAML parsing | `gopkg.in/yaml.v3` | Correct, maintained |
| Markdown walking | `github.com/yuin/goldmark` + extensions | CommonMark-compliant; pluggable for wikilinks |
| CLI | `github.com/spf13/cobra` | Mature, widely understood |
| Globs | `github.com/bmatcuk/doublestar/v4` | `**` globs for include/exclude |
| Config file | `github.com/knadh/koanf/v2` | YAML-first, env-overlay friendly |
| Release pipeline | GoReleaser + GitHub Actions | Idiomatic for Go CLI distribution; publishes to GitHub Releases |

Pinned minimums; avoid dependency bloat. Review every add against a simple "does this pull its weight?" bar.

### Build / release

- `make build` → local dev binary at `./bin/pql`
- `make test` → unit + integration tests against fixture vaults in `testdata/`
- `make lint` → `golangci-lint run` with a strict config (errcheck, revive, gocritic, staticcheck)
- `make snapshot` → dry-run GoReleaser build for all platforms
- Tag push (`v0.X.Y`) → GitHub Actions invokes `ci/release.sh` → GoReleaser → multi-platform binaries + SHA256 sums + cosign signatures, published to GitHub Releases. The Actions workflow is a thin wrapper around the script; switching CI providers later means swapping the wrapper, not the release logic.

## The CLI

Design principle: **the default invocation is `pql <QUERY>`** — a positional PQL query. Subcommands handle setup, introspection, and operations that don't fit the query language.

### Subcommands

```
pql init                       # create .pql.yaml in current dir; seed sensible defaults
pql doctor                     # diagnose: resolved vault root, frontmatter dialect, index path, last scan, warnings
pql index                      # force a full reindex; usually lazy
pql schema                     # print the frontmatter schema inferred from the current vault

pql files [glob]               # list markdown files matching glob (or all)
pql meta <path>                # frontmatter + link metadata for a single file, as JSON
pql tags                       # all tags with counts
pql backlinks <path>           # files linking to <path>
pql outlinks <path>            # files <path> links to
pql base <name>                # execute an Obsidian .base file as a PQL query

pql <QUERY>                    # run PQL, positional arg
pql --file <path>              # run PQL from a file (for queries with regex, nested quotes, or >80 chars)
pql --stdin                    # run PQL from stdin

pql shell                      # interactive SQLite REPL against the index (read-only; escape hatch)
pql self-update                # update binary from the configured release endpoint
pql completions bash|zsh|fish|powershell
pql version --build-info       # version, build date, Go version, schema version
```

### Global flags

```
--vault <path>      # override vault-root discovery
--db <path>         # override index path
--pretty            # pretty-print JSON
--jsonl             # emit JSON lines instead of an array
--table             # human-readable ad-hoc table (colour auto-detect; --no-color override)
--csv               # CSV for spreadsheet import
--select <jsonpath> # project into a JSONPath expression (avoids piping to jq for simple cases)
--limit <n>         # clamp output rows; overrides PQL LIMIT
--flat-search       # disable connect/ enrichment; return raw query results only (see project-structure.md)
--quiet             # suppress stderr warnings
--verbose           # verbose diagnostics on stderr
```

### Output contract

- **Data on stdout**, always JSON by default.
- **Diagnostics on stderr**, also as JSON: `{"level":"warn|error","code":"...","msg":"...","hint":"..."}`. Line-delimited when multiple.
- **Exit codes:**
  - `0` — success with ≥1 match
  - `2` — success with 0 matches (intentional; not an error)
  - `64` (EX_USAGE) — bad CLI flag
  - `65` (EX_DATAERR) — PQL parse or evaluation error
  - `66` (EX_NOINPUT) — vault root not found / unreadable
  - `69` (EX_UNAVAILABLE) — index corruption / migration failure
  - `70` (EX_SOFTWARE) — internal error

This contract is what makes `pql` safe for Claude Code to call: exit 2 is distinct from exit 65, so a zero-row query never masquerades as a bug.

### Vault-root discovery

In order:
1. `--vault <path>` flag
2. `$PQL_VAULT` env var
3. Walk cwd up until a `.obsidian/` directory is found
4. Walk cwd up until a `.git/` directory is found (generic repo fallback)
5. Current working directory (with a stderr warning)

`pql doctor` prints which rule matched.

### Config file

Optional `.pql.yaml` at the resolved vault root, or `~/.config/pql/config.yaml` global:

```yaml
frontmatter: yaml              # yaml | toml (+++)
wikilinks: obsidian            # obsidian | pandoc | markdown
tags:
  sources: [inline, frontmatter]
exclude:
  - "**/.obsidian/**"
  - "**/node_modules/**"
  - "**/.git/**"
aliases:
  members: "type = 'council-member'"
  sessions: "type = 'council-session'"
git_metadata: true             # populate file.gitmtime, file.gitauthor via git log
fts: false                     # opt-in FTS5 index on note bodies
```

`pql init` seeds this with project-appropriate defaults.

## The query language (PQL)

A Dataview-compatible dialect, explicit about what's in and what's out. The goal is: queries copied from an Obsidian dataview block should mostly Just Work, and new queries can be written by anyone familiar with DQL.

### Grammar (v1)

```
query        := resultMode source [where] [sort] [limit]
resultMode   := "LIST" | "TABLE" [fieldSpec ("," fieldSpec)*]
fieldSpec    := expr ["AS" ident]

source       := "FROM" srcExpr
srcExpr      := srcAtom (("AND" | "OR") srcAtom)*
srcAtom      := srcFolder | srcTag | srcInlink | srcOutlink | "NOT" srcAtom | "(" srcExpr ")"
srcFolder    := STRING                    -- "path/to/folder"
srcTag       := "#" ident ("/" ident)*
srcInlink    := "[[" ident "]]"           -- files linking TO this note
srcOutlink   := "outgoing" "(" "[[" ident "]]" ")"

where        := "WHERE" expr
sort         := "SORT" sortItem ("," sortItem)*
sortItem     := expr ["ASC" | "DESC"]
limit        := "LIMIT" INT

expr         := orExpr
orExpr       := andExpr ("OR" andExpr)*
andExpr      := notExpr ("AND" notExpr)*
notExpr      := ["NOT"] cmp
cmp          := unary (("=" | "!=" | "<" | "<=" | ">" | ">=" | "=~" | "IN" | "CONTAINS") unary)?
unary        := ["-"] primary
primary      := literal | ident | fieldRef | call | "(" expr ")"
fieldRef     := ident ("." ident)*        -- e.g. file.name, file.tags
call         := ident "(" [expr ("," expr)*] ")"

literal      := STRING | INT | FLOAT | BOOL | DATE | DURATION | REGEX | LIST
LIST         := "[" [expr ("," expr)*] "]"
REGEX        := "/" ... "/" | "regex" "(" STRING ")"
```

### Supported functions

- String: `length`, `lower`, `upper`, `contains`, `startswith`, `endswith`, `split`, `trim`, `regex_match`
- List: `length`, `contains`, `any`, `all`, `join`
- Date: `date(today)`, `date(now)`, `date("YYYY-MM-DD")`, `year`, `month`, `day`, `weekday`
- Duration: `dur("7 days")`, arithmetic `date(today) - dur("7d")`
- Aggregation (v1.1, not v1.0): `count`, `sum`, `min`, `max` — only once GROUP BY lands

### Virtual `file.*` fields

| Field | Type |
|---|---|
| `file.name` | string (basename without `.md`) |
| `file.path` | string (relative to vault root) |
| `file.folder` | string |
| `file.link` | wikilink (renderable in output) |
| `file.mtime` | datetime |
| `file.ctime` | datetime |
| `file.size` | int (bytes) |
| `file.tags` | list of strings |
| `file.inlinks` | list of paths |
| `file.outlinks` | list of paths |
| `file.headings` | list of strings |
| `file.gitmtime` | datetime (if `git_metadata: true`) |
| `file.gitauthor` | string |

### Explicitly unsupported in v1

Every one of these errors with `exit 65` and a clear message pointing at this list:

- `TASK` and `CALENDAR` result modes
- `GROUP BY` and `FLATTEN`
- Inline fields in prose (`Rating:: 5` mid-paragraph)
- `dataviewjs` / any JavaScript evaluation
- Embeds and transclusions (`![[...]]`)
- Non-Obsidian markdown dialects (Logseq, Bear, Roam) — extractor-registry hook in v2

### Example queries

```
LIST FROM "members" WHERE voting = true SORT name ASC

TABLE winner, date, tied FROM #council-session WHERE tied = true SORT date DESC

TABLE file.link AS session, votes FROM "sessions" SORT file.mtime DESC LIMIT 5

LIST FROM "members" WHERE name =~ /^Dr\./

TABLE file.name, length(file.outlinks) AS outlinks
  FROM "sessions"
  WHERE file.mtime > date(today) - dur("30 days")
  SORT outlinks DESC
```

## The SQLite index

Not just a cache — the index **is** the query target. The evaluator reads from SQLite; the vault walker writes to SQLite.

### Schema (v1)

```sql
CREATE TABLE index_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- keys: schema_version, pql_version, config_hash, last_full_scan

CREATE TABLE files (
    path          TEXT PRIMARY KEY,
    mtime         INTEGER NOT NULL,   -- unix seconds
    ctime         INTEGER NOT NULL,
    size          INTEGER NOT NULL,
    content_hash  TEXT NOT NULL,
    last_scanned  INTEGER NOT NULL
);

CREATE TABLE frontmatter (
    path        TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value_json  TEXT NOT NULL,       -- canonical typed value
    value_text  TEXT,                -- for text ops / LIKE / =~
    value_num   REAL,                -- for numeric comparisons
    PRIMARY KEY (path, key)
);
CREATE INDEX idx_frontmatter_key_text ON frontmatter(key, value_text);
CREATE INDEX idx_frontmatter_key_num  ON frontmatter(key, value_num);

CREATE TABLE tags (
    path TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    tag  TEXT NOT NULL,
    PRIMARY KEY (path, tag)
);
CREATE INDEX idx_tags_tag ON tags(tag);

CREATE TABLE links (
    source_path TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    target_path TEXT NOT NULL,       -- not FK; dangling links are allowed
    alias       TEXT,
    link_type   TEXT NOT NULL,       -- wiki | md | embed
    line        INTEGER NOT NULL
);
CREATE INDEX idx_links_source ON links(source_path);
CREATE INDEX idx_links_target ON links(target_path);

CREATE TABLE headings (
    path         TEXT NOT NULL REFERENCES files(path) ON DELETE CASCADE,
    depth        INTEGER NOT NULL,
    text         TEXT NOT NULL,
    line_offset  INTEGER NOT NULL
);

CREATE TABLE bases (
    name      TEXT PRIMARY KEY,       -- .base file name without extension
    path      TEXT NOT NULL,
    ast_json  TEXT NOT NULL,          -- parsed Base → PQL AST
    mtime     INTEGER NOT NULL
);

-- Opt-in (`fts: true` in config):
CREATE VIRTUAL TABLE fts_bodies USING fts5(
    path UNINDEXED,
    body,
    tokenize='porter unicode61'
);
```

### Incremental update flow

Each invocation:
1. Load `index_meta.config_hash`; if different from computed hash → full rebuild. Else:
2. Walk vault with `doublestar` (respecting `exclude` patterns).
3. Stat each `.md`. For each file where `disk.mtime > files.mtime` OR row doesn't exist: reparse (frontmatter, tags, links, headings), upsert.
4. Find rows in `files` whose path is no longer on disk; delete (cascade removes child rows).
5. Optional: refresh `fts_bodies` for changed files.
6. Update `index_meta.last_full_scan`.

All writes in a single `BEGIN IMMEDIATE` transaction per invocation. Readers are unaffected (WAL mode).

### Migrations

`schema_version` in `index_meta`. On mismatch, drop the DB file and rebuild — the index is a cache; never store anything that isn't regenerable from the vault. This keeps migration code at zero. If/when we ever store user-authored state in the DB, we add migrations then and not before.

## Obsidian Bases as first-class queries

`.base` files are YAML with `filters`, `properties`, `views`, `sort`. The base parser reads them, compiles to PQL AST, and runs through the evaluator.

```
pql base council-sessions
pql base council-sessions --view "All sessions"
pql base                             # list known bases
```

This means any Base you maintain in Obsidian becomes a callable, scriptable query from outside. The Council project already maintains `council-sessions.base` and `council-members.base`; those become programmatically reachable on day one of pql integration.

## Claude Code integration

### The skill

`skill/SKILL.md` (distributed with releases) contains:

- **Trigger phrases:** "query the vault", "find notes where…", "which sessions/members…", "run a Base", "inspect frontmatter"
- **Precondition check:** confirm `pql` is on `$PATH` via `command -v pql`. If absent, the skill aborts and tells the user how to install.
- **Schema discovery:** the skill opens with "run `pql schema` first" so the agent knows which `type:` values and fields exist in *this* vault.
- **Cookbook:** 15–20 canonical patterns — per-type listings, backlink walks, tag intersections, time-bounded recent-activity queries, Base invocations.
- **Output contract:** JSON shape, exit codes (especially: 2 = zero-match, not an error), stderr-is-JSON.
- **Anti-patterns:**
  - No pipes to `jq` inline — use `--select` instead.
  - Queries >80 chars or with regex → `--file`, not positional.
  - No env-var prefix on the command (`FOO=bar pql …` breaks permission matching).
  - Set `PQL_VAULT` in the shell profile, not per-invocation.
- **When NOT to use pql:**
  - Raw text search inside note bodies → Grep/ripgrep (unless FTS is enabled, then `pql` can).
  - Single file's full content → Read directly.
  - Code structure → tree-sitter / LSP, not `pql`.

### Permissions

The consuming project's `.claude/settings.json` gets:

```json
{
  "permissions": {
    "allow": [
      "Bash(pql)",
      "Bash(pql *)"
    ]
  }
}
```

Two entries because the wildcard form requires at least one argument after `pql`; the bare form covers `pql --help` / `pql doctor` / etc.

No `pql`-related deny rules. It's read-only against the filesystem; nothing to deny.

## Distribution

### Platforms

- `linux/amd64` — primary desktop + server Linux
- `linux/arm64` — Raspberry Pi, ARM cloud, newer Linux laptops
- `darwin/arm64` — Apple Silicon (primary Mac target)
- `darwin/amd64` — Intel Mac legacy
- `windows/amd64` — desktop Windows

### Release pipeline

GoReleaser + GitHub Actions. On tag push (`v0.X.Y`):

1. Build for all five platforms in parallel.
2. Strip binaries (`-ldflags="-s -w"`); resulting binary ~8–12 MB.
3. Generate SHA256SUMS file + SBOM; cosign-sign artefacts.
4. Publish to GitHub Releases at https://github.com/postmeridiem/pql with auto-generated release notes.

The GitHub Actions workflow is a thin wrapper around `ci/release.sh`. Switching CI providers later means swapping the wrapper, not the release logic.

### Install paths

- **Manual:** download the `pql-<os>-<arch>` binary from https://github.com/postmeridiem/pql/releases/latest; verify SHA256; `chmod +x`; place on `$PATH` (e.g. `~/.local/bin/`).
- **Go toolchain:** `go install github.com/postmeridiem/pql/cmd/pql@latest` for developers who have Go.
- **Self-update:** `pql self-update` once a user has v0.1 installed — hits the GitHub Releases API, downloads + replaces atomically, verifies SHA256.

## Roadmap

### v0.1 — "indexer + shortcuts" (Days 1–2)

- [ ] Go module skeleton, Makefile, CI stub
- [ ] Vault walker + `doublestar` exclude support
- [ ] YAML frontmatter parser
- [ ] Wikilink + tag extractor
- [ ] SQLite schema + migrations
- [ ] `pql init`, `pql doctor`, `pql schema`
- [ ] `pql files`, `pql meta`, `pql tags`, `pql backlinks`, `pql outlinks`
- [ ] JSON / JSONL / `--pretty` output
- [ ] Integration test against a fixture vault (including the Council's `members/` + `sessions/` shape)
- [ ] First GoReleaser dry-run build

### v0.2 — "query language" (Days 3–7)

- [ ] Lexer + recursive-descent parser for the v1 grammar above
- [ ] AST evaluator: FROM sources, WHERE expressions, SORT, LIMIT
- [ ] All supported functions (string, list, date, duration, regex)
- [ ] LIST and TABLE output modes, field aliasing
- [ ] Clear parse-error messages with line/col pointers
- [ ] `--file` and `--stdin` input modes
- [ ] `--select` JSONPath projection

### v0.3 — "Bases + polish" (Days 8–10)

- [ ] `.base` file parser → PQL AST
- [ ] `pql base <name>` execution
- [ ] `.pql.yaml` config layer with alias expansion
- [ ] Git metadata ingestion (opt-in)
- [ ] `pql shell` (interactive SQLite REPL against the index)
- [ ] Shell completions (bash, zsh, fish, powershell)
- [ ] `pql version --build-info`
- [ ] Full GoReleaser pipeline behind `ci/release.sh`, first tagged release
- [ ] Skill package finalised and released alongside binary

### v0.4 and beyond — "when it bites" (not committed to a date)

- [ ] `GROUP BY` + aggregation functions
- [ ] FTS5 body search (opt-in via config)
- [ ] `self-update` command
- [ ] TOML frontmatter support (`+++` Hugo-style)
- [ ] `FLATTEN` if we ever need it
- [ ] Inline-field parsing if anyone asks
- [ ] Logseq/Roam dialect plugins via the extractor registry

## Open questions to resolve before coding

1. **Repository ownership / canonical Git host.** Decided: GitHub at https://github.com/postmeridiem/pql. Module path: `github.com/postmeridiem/pql`.
2. **License.** Decided: MIT. See `LICENSE`.
3. **Skill-package distribution channel.** Inside the `pql` repo at `skill/` is decided. Question remaining: ship it as an asset on each release alongside the binaries, or only as a path to copy from? Start with "just copy the directory"; revisit once there's a convention for Claude Code skill marketplaces.
4. **Tag syntax ambiguity.** Obsidian allows `#tag` inside code fences and inline code. Dataview excludes those from tag indexing. Decide: match Dataview's rule (probably yes).
5. **Link target resolution.** Obsidian resolves `[[Note]]` using "shortest path that unambiguously identifies" — i.e., basename match, falling back to path disambiguation. Implement that exact algorithm (important for Base compatibility) — but needs a small dedicated tie-breaker module.
6. **`file.inlinks` / `file.outlinks` — alias-aware?** If a note is linked with `[[Note|alias]]`, is "alias" recorded? Probably yes; accessible via a function (`alias(link)`) rather than polluting the default. Not critical to decide on day 1.

## Reference: the Council vault as first customer

This project is motivated by the Council repo at `/var/mnt/data/projects/council/`. Its frontmatter vocabulary:

| `type` value | Location | Key fields |
|---|---|---|
| `council-member` | `members/<slug>/persona.md` | name, prior_job, lens, voting, model |
| `council-journal` | `members/<slug>/journal.md` | slug |
| `council-on-the-user` | `members/<slug>/on-the-user.md` | slug |
| `council-revisit` | `members/<slug>/revisit.md` | slug |
| `council-brief` | `sessions/<slug>/brief.md` | date, slug, problem |
| `council-research` | `sessions/<slug>/research/*.md` | session, topic, round |
| `council-initial-answers` | `sessions/<slug>/initial-answers.md` | session |
| `council-revised-answers` | `sessions/<slug>/revised-answers.md` | session, had_round_2 |
| `council-votes` | `sessions/<slug>/votes.md` | session |
| `council-session` | `sessions/<slug>/outcome.md` | date, slug, problem, winner, tied, votes, tags, had_*_clarification |

Plus two Obsidian Bases: `council-sessions.base` and `council-members.base`.

These become the fixture set for integration tests — real, complex, hand-authored frontmatter across 40+ files, with wikilinks, tags, and two real Bases. Any regression in pql that breaks Council queries fails CI.

## Appendix: naming

- **Binary name:** `pql` (final)
- **Query language name:** PQL (Project Query Language)
- **Config file:** `.pql.yaml`
- **Env vars:** `PQL_VAULT`, `PQL_DB`, `PQL_CONFIG`
- **Cache dir:** `$XDG_CACHE_HOME/pql/` (Linux), `~/Library/Caches/pql/` (macOS), `%LocalAppData%/pql/` (Windows)
