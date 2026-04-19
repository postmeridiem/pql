# `mql` — Markdown Query Language for structured prose repos

A Go CLI that indexes and queries any repo containing Markdown files with YAML frontmatter, wikilinks, tags, and Obsidian Bases. Ships as a single static binary, maintains a SQLite index under `~/.cache/mql/`, exposes a Dataview-compatible query dialect, and is designed to be drop-in for AI agents (Claude Code, primarily) that need structural introspection without brute-force grep+read.

## Context and problem

Markdown knowledge bases — Obsidian vaults, Zettelkasten, personal wikis, even well-structured documentation repos — are effectively small databases: YAML frontmatter is typed metadata, wikilinks are foreign keys, tags are labels, Obsidian Bases are saved queries. Off the shelf today:

- **Obsidian + Dataview** runs inside Obsidian. Great inside the app. Nothing outside it.
- **ripgrep / grep / find** are raw text search. They don't understand frontmatter, links, or tag semantics.
- **jq / yq** handle single structured files, not cross-file queries.
- **Code-aware tools** (tree-sitter, LSP, ast-grep) target source code, not prose.

There's a gap: **structural, cross-file querying of a prose-structured repository from outside any specific editor.** That's what `mql` fills. AI agents driving these repos today walk the tree by hand — grepping, reading, guessing — and burn context on work that should be one query. `mql` is the tool-shaped hole in that workflow.

## Non-goals (explicit)

- Not a Dataview replacement inside Obsidian. It *imitates* DQL from outside.
- Not a generic "query anything" tool. Code → use tree-sitter/LSP. Configs → use jq/yq. Raw text → use ripgrep. `mql` is for Markdown + frontmatter + wikilinks + tags + Bases.
- Not a write tool. Queries and introspection only. Edits go through the filesystem directly (which Obsidian's watcher picks up).
- No `dataviewjs` or JavaScript evaluation.
- No inline-field parsing (`Rating:: 5` mid-paragraph) in v1. Revisit if users demand it.
- No GUI, no web UI, no network daemon. CLI in, JSON out.

## What ships

Three artefacts, one repo:

1. **`mql` binary** — single static Go executable, cross-compiled to `linux/{amd64,arm64}`, `darwin/{amd64,arm64}`, `windows/amd64`. Distributed via GitHub Releases with SHA256 sums; Homebrew tap once stable.
2. **SQLite index** at `~/.cache/mql/<vault-fingerprint>.sqlite` (XDG-cache on Linux, `~/Library/Caches/mql/` on macOS, `%LocalAppData%/mql/` on Windows). Regenerable from the vault; never lives inside the vault.
3. **Claude Code skill** at `skill/SKILL.md` inside this repo, with install instructions. Dropped into any project's `.claude/skills/mql/` or the user's `~/.claude/skills/mql/`. Documents trigger phrases, common query recipes, anti-patterns, and the install check.

## Architecture

Go 1.22+, single module, no cgo.

```
mql/
├── cmd/mql/main.go              # CLI entrypoint, flag parsing, subcommand dispatch
├── internal/
│   ├── config/                  # .mql.yaml resolution, vault-root discovery, env handling
│   ├── index/                   # vault walker, frontmatter parser, link extractor, tag extractor
│   ├── store/                   # SQLite schema, migrations, CRUD, query helpers
│   ├── lex/                     # MQL lexer
│   ├── parse/                   # MQL parser → AST
│   ├── eval/                    # AST evaluator against the store
│   ├── base/                    # Obsidian .base YAML → MQL AST
│   └── render/                  # JSON / JSONL / table / CSV output
├── skill/SKILL.md               # Claude Code skill package
├── docs/
│   ├── structure/initial-plan.md   # this file
│   ├── mql-grammar.md              # language spec (written alongside the parser)
│   ├── skill.md                    # skill install + usage
│   └── cli.md                      # subcommand reference
├── examples/                    # example vaults + example queries for docs/tests
├── testdata/                    # fixture vaults for integration tests
├── .goreleaser.yaml
├── .github/workflows/release.yaml
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
| Release pipeline | GoReleaser + GitHub Actions | Idiomatic for Go CLI distribution |

Pinned minimums; avoid dependency bloat. Review every add against a simple "does this pull its weight?" bar.

### Build / release

- `make build` → local dev binary at `./bin/mql`
- `make test` → unit + integration tests against fixture vaults in `testdata/`
- `make lint` → `golangci-lint run` with a strict config (errcheck, revive, gocritic, staticcheck)
- `make release-snapshot` → dry-run GoReleaser build for all platforms
- Tag push (`v0.X.Y`) → GitHub Actions runs GoReleaser → multi-platform binaries + SHA256 sums + Homebrew formula update

## The CLI

Design principle: **the default invocation is `mql <QUERY>`** — a positional MQL query. Subcommands handle setup, introspection, and operations that don't fit the query language.

### Subcommands

```
mql init                       # create .mql.yaml in current dir; seed sensible defaults
mql doctor                     # diagnose: resolved vault root, frontmatter dialect, index path, last scan, warnings
mql index                      # force a full reindex; usually lazy
mql schema                     # print the frontmatter schema inferred from the current vault

mql files [glob]               # list markdown files matching glob (or all)
mql meta <path>                # frontmatter + link metadata for a single file, as JSON
mql tags                       # all tags with counts
mql backlinks <path>           # files linking to <path>
mql outlinks <path>            # files <path> links to
mql base <name>                # execute an Obsidian .base file as an MQL query

mql <QUERY>                    # run MQL, positional arg
mql --file <path>              # run MQL from a file (for queries with regex, nested quotes, or >80 chars)
mql --stdin                    # run MQL from stdin

mql shell                      # interactive SQLite REPL against the index (read-only; escape hatch)
mql self-update                # update binary from GitHub Releases
mql completions bash|zsh|fish|powershell
mql version --build-info       # version, build date, Go version, schema version
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
--limit <n>         # clamp output rows; overrides MQL LIMIT
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
  - `65` (EX_DATAERR) — MQL parse or evaluation error
  - `66` (EX_NOINPUT) — vault root not found / unreadable
  - `69` (EX_UNAVAILABLE) — index corruption / migration failure
  - `70` (EX_SOFTWARE) — internal error

This contract is what makes `mql` safe for Claude Code to call: exit 2 is distinct from exit 65, so a zero-row query never masquerades as a bug.

### Vault-root discovery

In order:
1. `--vault <path>` flag
2. `$MQL_VAULT` env var
3. Walk cwd up until a `.obsidian/` directory is found
4. Walk cwd up until a `.git/` directory is found (generic repo fallback)
5. Current working directory (with a stderr warning)

`mql doctor` prints which rule matched.

### Config file

Optional `.mql.yaml` at the resolved vault root, or `~/.config/mql/config.yaml` global:

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

`mql init` seeds this with project-appropriate defaults.

## The query language (MQL)

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
- Non-Obsidian markdown dialects (Logseq, Bear, Roam) — plugin hook in v2

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
-- keys: schema_version, mql_version, config_hash, last_full_scan

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
    ast_json  TEXT NOT NULL,          -- parsed Base → MQL AST
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

`.base` files are YAML with `filters`, `properties`, `views`, `sort`. The base parser reads them, compiles to MQL AST, and runs through the evaluator.

```
mql base council-sessions
mql base council-sessions --view "All sessions"
mql base                             # list known bases
```

This means any Base you maintain in Obsidian becomes a callable, scriptable query from outside. The Council project already maintains `council-sessions.base` and `council-members.base`; those become programmatically reachable on day one of mql integration.

## Claude Code integration

### The skill

`skill/SKILL.md` (distributed with releases) contains:

- **Trigger phrases:** "query the vault", "find notes where…", "which sessions/members…", "run a Base", "inspect frontmatter"
- **Precondition check:** confirm `mql` is on `$PATH` via `command -v mql`. If absent, the skill aborts and tells the user how to install.
- **Schema discovery:** the skill opens with "run `mql schema` first" so the agent knows which `type:` values and fields exist in *this* vault.
- **Cookbook:** 15–20 canonical patterns — per-type listings, backlink walks, tag intersections, time-bounded recent-activity queries, Base invocations.
- **Output contract:** JSON shape, exit codes (especially: 2 = zero-match, not an error), stderr-is-JSON.
- **Anti-patterns:**
  - No pipes to `jq` inline — use `--select` instead.
  - Queries >80 chars or with regex → `--file`, not positional.
  - No env-var prefix on the command (`FOO=bar mql …` breaks permission matching).
  - Set `MQL_VAULT` in the shell profile, not per-invocation.
- **When NOT to use mql:**
  - Raw text search inside note bodies → Grep/ripgrep (unless FTS is enabled, then `mql` can).
  - Single file's full content → Read directly.
  - Code structure → tree-sitter / LSP, not `mql`.

### Permissions

The consuming project's `.claude/settings.json` gets:

```json
{
  "permissions": {
    "allow": [
      "Bash(mql)",
      "Bash(mql *)"
    ]
  }
}
```

Two entries because the wildcard form requires at least one argument after `mql`; the bare form covers `mql --help` / `mql doctor` / etc.

No `mql`-related deny rules. It's read-only against the filesystem; nothing to deny.

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
3. Generate SHA256SUMS file; optionally cosign-sign.
4. Publish to GitHub Releases with auto-generated release notes.
5. Update Homebrew tap formula (`<user>/homebrew-mql`) pinned to the new tag.

### Install paths

- **Manual:** `curl -L https://github.com/<user>/mql/releases/latest/download/mql-$(uname -s)-$(uname -m) -o ~/.local/bin/mql && chmod +x ~/.local/bin/mql`
- **Homebrew:** `brew install <user>/mql/mql`
- **Go toolchain:** `go install github.com/<user>/mql/cmd/mql@latest` (for developers who have Go; not the primary channel)
- **Self-update:** `mql self-update` once a user has v0.1 installed — hits the GitHub Releases API, downloads + replaces atomically, verifies SHA256.

## Roadmap

### v0.1 — "indexer + shortcuts" (Days 1–2)

- [ ] Go module skeleton, Makefile, CI stub
- [ ] Vault walker + `doublestar` exclude support
- [ ] YAML frontmatter parser
- [ ] Wikilink + tag extractor
- [ ] SQLite schema + migrations
- [ ] `mql init`, `mql doctor`, `mql schema`
- [ ] `mql files`, `mql meta`, `mql tags`, `mql backlinks`, `mql outlinks`
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

- [ ] `.base` file parser → MQL AST
- [ ] `mql base <name>` execution
- [ ] `.mql.yaml` config layer with alias expansion
- [ ] Git metadata ingestion (opt-in)
- [ ] `mql shell` (interactive SQLite REPL against the index)
- [ ] Shell completions (bash, zsh, fish, powershell)
- [ ] `mql version --build-info`
- [ ] Full GoReleaser pipeline in CI, first tagged release
- [ ] Skill package finalised and released alongside binary

### v0.4 and beyond — "when it bites" (not committed to a date)

- [ ] `GROUP BY` + aggregation functions
- [ ] FTS5 body search (opt-in via config)
- [ ] `self-update` command
- [ ] Homebrew tap
- [ ] TOML frontmatter support (`+++` Hugo-style)
- [ ] `FLATTEN` if we ever need it
- [ ] Inline-field parsing if anyone asks
- [ ] Logseq/Roam dialect plugins via a small parser-registration interface

## Open questions to resolve before coding

1. **Repository ownership / namespace.** GitHub username to host under — affects import path (`github.com/<user>/mql`), Homebrew tap name, release URLs. Decide before `go mod init`.
2. **License.** MIT is the obvious choice for a developer CLI tool; Apache-2.0 if patent grant matters. Pick one before first commit of any code.
3. **Module import path.** Tied to ownership; `github.com/<user>/mql` is the default.
4. **Skill-package distribution channel.** Inside the `mql` repo at `skill/` is decided. Question remaining: do we ship it zipped in GitHub Releases (`mql-skill-v0.1.tar.gz`) alongside the binaries, or only as a path to copy-from? Start with "just copy the directory"; revisit once there's a convention for Claude Code skill marketplaces.
5. **Tag syntax ambiguity.** Obsidian allows `#tag` inside code fences and inline code. Dataview excludes those from tag indexing. Decide: match Dataview's rule (probably yes).
6. **Link target resolution.** Obsidian resolves `[[Note]]` using "shortest path that unambiguously identifies" — i.e., basename match, falling back to path disambiguation. Implement that exact algorithm (important for Base compatibility) — but needs a small dedicated tie-breaker module.
7. **`file.inlinks` / `file.outlinks` — alias-aware?** If a note is linked with `[[Note|alias]]`, is "alias" recorded? Probably yes; accessible via a function (`alias(link)`) rather than polluting the default. Not critical to decide on day 1.

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

These become the fixture set for integration tests — real, complex, hand-authored frontmatter across 40+ files, with wikilinks, tags, and two real Bases. Any regression in mql that breaks Council queries fails CI.

## Appendix: naming

- **Binary name:** `mql` (final)
- **Query language name:** MQL (Markdown Query Language)
- **Config file:** `.mql.yaml`
- **Env vars:** `MQL_VAULT`, `MQL_DB`, `MQL_CONFIG`
- **Cache dir:** `$XDG_CACHE_HOME/mql/` (Linux), `~/Library/Caches/mql/` (macOS), `%LocalAppData%/mql/` (Windows)
