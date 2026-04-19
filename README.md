# pql

**Project Query Language.** A single-binary CLI built to back a [Claude Code](https://claude.com/claude-code) skill: it indexes a markdown-structured repo into SQLite and exposes intent-level tools so the agent can ask structural questions (backlinks, frontmatter, related notes, saved queries) instead of falling back to grep+read.

The first dialect supported is Obsidian-flavored markdown — frontmatter, wikilinks, tags, `.base` files — because that's what the maintainer uses. Other dialects (Logseq, Roam, plain pandoc) land via the extractor registry as the need arises.

## Why a CLI, not an MCP server or plugin

A static binary on `$PATH` is the lowest-friction surface for Claude Code: one allow rule (`Bash(pql *)`) in `.claude/settings.json` and the agent can use it. No daemon, no network, no permission elevation. The output contract (stdout JSON, stderr JSON-per-line diagnostics, distinguished exit codes) is what makes it safe for an agent to call without a wrapper. See [`docs/structure/design-philosophy.md`](docs/structure/design-philosophy.md) for the deeper rationale (the binary as a *ranker*, not just a query engine) and [`docs/structure/project-structure.md`](docs/structure/project-structure.md) for the layout that follows from it.

## Obsidian support, kept as we go

Since the maintainer also uses [Obsidian](https://obsidian.md/) day-to-day, `pql` is built to read the same vault Obsidian reads. The two are independent surfaces over one substrate on disk:

|  | Human (Obsidian) | Agent (Claude Code via pql) |
|---|---|---|
| Reads / edits | app pane | filesystem (Read / Write) |
| Navigates | graph view, backlinks pane | `pql backlinks`, `pql related` |
| Queries | Dataview blocks, Bases | `pql query <DSL>`, `pql base` |
| Discovers | tag pane, file tree | `pql tags`, `pql schema` |

No Obsidian plugin, no daemon, no extra sync step — `pql` reads the filesystem directly. Obsidian doesn't need to be installed or running for `pql` to work; `pql` doesn't need to know about Obsidian beyond parsing its dialect.

The dual-surface property happens to be handy in markdown-first workspaces (research notes, design docs, vaults of structured prose) where a conventional code IDE adds friction — Claude Code gets the structural visibility a human gets from Obsidian's graph view and backlinks pane.

## Quick idea

```sh
pql 'LIST FROM "sessions" WHERE winner = "vaasa" SORT date DESC LIMIT 5'
pql 'TABLE name, prior_job FROM #council-member WHERE voting = true SORT name ASC'
pql base council-sessions
pql backlinks members/vaasa/persona.md
pql schema
```

One static Go binary. Drop into `~/.local/bin/`. Keeps a SQLite index under `~/.cache/pql/`. No network, no daemon, no Obsidian dependency.

## Status

Pre-v0.1. Scaffolding in place (build pipeline, CLI skeleton, full design docs); first features land per the roadmap in [`docs/structure/initial-plan.md`](docs/structure/initial-plan.md).

## What this is not

- Not a Dataview replacement. PQL is its own SQL-derived query language and runs outside Obsidian; Dataview lives inside the app and stays there.
- Not a universal repo-introspection tool. Code → tree-sitter/LSP. Raw text → ripgrep. Configs → jq/yq. This fills the remaining gap: **prose-structured-data querying**.
- Not an Obsidian plugin, daemon, or required-companion app. The vault is read directly from disk; Obsidian doesn't need to be running, or installed at all.

## Inspiration

Credit where it's due: the idea of querying markdown notes with a SQL-like language comes from [Dataview](https://blacksmithgu.github.io/obsidian-dataview/). PQL doesn't implement DQL, and the surface area we expose is shaped by different constraints (CLI-first, agent-callable, ranker-with-connections), but the core insight that a vault of frontmatter-bearing notes is a queryable database is Dataview's. This project wouldn't exist without it.

## License

[MIT](LICENSE).
