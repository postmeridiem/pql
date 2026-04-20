---
name: pql
description: >
  Structural queries over a markdown vault — frontmatter, wikilinks, tags,
  headings, and Obsidian Bases — from outside Obsidian. Use whenever the
  user asks about the contents or shape of a vault: "which sessions…",
  "find notes where…", "what tags are used", "who links to X", "inspect
  frontmatter", "run a Base", "query the vault". Requires the `pql` CLI
  on PATH. Returns structured JSON on stdout; exit 2 means "ran cleanly,
  zero matches" (not an error).
---

# pql — querying a markdown vault from the CLI

`pql` indexes a vault's frontmatter, wikilinks, tags, and headings into a
local SQLite database and exposes them through a small set of subcommands
plus a SQL-derived DSL. Reach for this skill instead of grep+read when
the user's question is **structural** (about shape, metadata, relationships)
rather than textual (about body content).

## Precondition check

Before the first `pql` call in a session, verify the binary is installed:

```bash
command -v pql
```

If absent, tell the user:

> `pql` isn't on PATH. Install the latest static binary from
> https://github.com/postmeridiem/pql/releases/latest (drop into
> `~/.local/bin/` and `chmod +x`), then re-run.

Don't try to install it yourself. Don't fall back to grep unless the user
explicitly asks for raw text search.

## First-touch: learn the vault's shape

Before answering a structural question, run:

```bash
pql schema
```

This returns one row per frontmatter key across the vault, with the
observed types (string / number / bool / list / object) and the number of
files that use each key. Read this once per session to know which keys
and types the queries you're about to write can rely on.

## Subcommands

Short-form queries (no DSL needed):

| Command | Purpose |
|---|---|
| `pql files [glob]` | list indexed files; optional GLOB filter on path |
| `pql tags [--sort count]` | distinct tags with counts |
| `pql backlinks <path>` | files that link TO `<path>` |
| `pql outlinks <path>` | links FROM `<path>` |
| `pql meta <path>` | all metadata for one file: frontmatter, tags, outlinks, headings |
| `pql schema` | typed frontmatter schema across the vault |
| `pql base <name>` | execute an Obsidian `.base` file as a query |
| `pql doctor` | report resolved vault, config, DB, index state |

DSL for anything else:

```bash
pql query "SELECT name, fm.prior_job WHERE fm.type = 'council-member' ORDER BY name"
```

See the grammar at https://github.com/postmeridiem/pql/blob/main/docs/pql-grammar.md.

## Cookbook

These are the patterns that come up most. Pick the closest and edit.

**"List all council members"** →
```bash
pql query "SELECT name, fm.prior_job WHERE fm.type = 'council-member' ORDER BY name"
```

**"Which files are tagged X?"** →
```bash
pql query "SELECT path WHERE 'council-member' IN tags ORDER BY path"
```

**"Most-used tags"** →
```bash
pql tags --sort count --limit 20
```

**"Files in this folder"** →
```bash
pql files 'members/vaasa/*'
```

**"Recently modified"** →
```bash
pql query "SELECT name, mtime WHERE mtime > strftime('%s','now','-30 days') ORDER BY mtime DESC LIMIT 10"
```

**"What links to this file?"** →
```bash
pql backlinks members/vaasa/persona.md
```

**"What does this file link to?"** →
```bash
pql outlinks sessions/2026-04-19/outcome.md
```

**"Sessions where Vaasa won"** →
```bash
pql query "SELECT name, fm.date FROM files WHERE fm.winner = 'vaasa' ORDER BY fm.date DESC"
```

**"Files by frontmatter range"** →
```bash
pql query "SELECT name, fm.date WHERE fm.date BETWEEN '2024-01-01' AND '2024-12-31' ORDER BY fm.date"
```

**"Inspect one file's structure"** →
```bash
pql meta members/vaasa/persona.md --pretty
```

**"Run a saved Obsidian Base"** →
```bash
pql base council-sessions
```

## Output contract

- **stdout:** JSON. Default is a single array; `--jsonl` for one object per
  line; `--pretty` to indent; `--limit N` to cap.
- **stderr:** JSON-per-line diagnostics with `{"level":"warn|error","code":"pql.<phase>.<kind>","msg":"…","hint":"…"}`.
- **Exit codes** are distinguished — check these before interpreting output:
  - `0` — success with ≥1 result
  - `2` — **zero matches (NOT an error)** — the query ran cleanly, nothing matched. Tell the user "no matches" rather than claiming failure.
  - `64` — bad CLI flag
  - `65` — DSL parse/compile error (user's query is malformed; pass the stderr message back)
  - `66` — vault / config not found
  - `69` — index corrupt or unavailable
  - `70` — internal error (bug report worthy)

When you check `$?` or invoke via subprocess, distinguish 2 from 0.

## Anti-patterns

- **Don't pipe to `jq` for simple projections.** Use `--limit`, `--pretty`, `--jsonl` first; reach for `jq` only for non-trivial restructuring.
- **Don't interpolate vault content into the command line.** Put long queries in a file and use `pql query --file q.pql`, or pipe via `--stdin`. Unquoted `#`, `&`, `|` will break the shell.
- **Don't chain `pql files` + `pql meta` to iterate.** One `pql query` with a `WHERE` clause is one subprocess instead of N+1.
- **Don't parse the DSL for the user.** If their query has a syntax error, pass the stderr diagnostic back — it carries line/col pointers.
- **Don't try to install or upgrade `pql`.** If the binary is missing, instruct the user; otherwise assume the version on PATH is the one they want.

## When NOT to use pql

- **Raw text search inside note bodies** → `Grep` / `rg`. pql's FTS5 is opt-in and usually off.
- **Reading a single file's full contents** → `Read`. pql's `meta` is for structure, not prose.
- **Code structure** → tree-sitter / LSP. pql is for prose, not code.
- **Modifying files** → `Write` / `Edit`. pql is read-only.

## Permissions

The consuming project's `.claude/settings.json` should allow:

```json
{
  "permissions": {
    "allow": ["Bash(pql)", "Bash(pql *)"]
  }
}
```

Two entries because `Bash(pql *)` requires at least one argument; `Bash(pql)` covers bare invocations like `pql --help`. No deny rules needed — pql is read-only against the filesystem.

## Updating the skill

`pql skill status` reports whether the installed skill is current relative to the binary. `pql skill install` writes/updates; `--force` overrides hand-edits (which the skill preserves by default). `pql doctor` also surfaces skill drift alongside the rest of the resolved state.
