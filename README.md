# pql

**Project Query Language.** A single-binary CLI that indexes and queries any repo containing Markdown with YAML frontmatter, wikilinks, tags, and Obsidian Bases.

Built for **markdown-first workspaces** where humans navigate via Obsidian and agents (Claude Code, primarily) need the same structural visibility — frontmatter, wikilinks, tags, Bases — without resorting to grep+read.

## Obsidian as IDE-light for Claude Code

When most of the work in a project is markdown management — research notes, design docs, briefs, vaults of structured prose — a conventional code IDE adds friction. We've shifted those workspaces to be **markdown-first**, with [Obsidian](https://obsidian.md/) as the human's IDE-equivalent: graph view for orientation, backlinks for navigation, Bases for saved queries, plugins for whatever the workflow needs.

The catch is that Claude Code can't see any of that. The graph, the backlinks, the Bases, the frontmatter conventions — they live behind Obsidian's app boundary. The agent falls back to grep+read and burns context reconstructing structure the human can see at a glance.

`pql` is the bridge. It indexes the same markdown the human is reading in Obsidian — frontmatter, wikilinks, tags, Bases — and exposes that structure to Claude Code via a single static binary. Two parallel surfaces over one substrate:

|  | Human (Obsidian) | Agent (Claude Code via pql) |
|---|---|---|
| Reads / edits | app pane | filesystem (Read / Write) |
| Navigates | graph view, backlinks pane | `pql backlinks`, `pql related` |
| Queries | Dataview blocks, Bases | `pql query <DSL>`, `pql base` |
| Discovers | tag pane, file tree | `pql tags`, `pql schema` |

The vault on disk is the source of truth; Obsidian and `pql` are both lenses on it. No Obsidian plugin, no daemon, no extra sync step — `pql` reads the filesystem directly and the agent and the human see the same structure.

This framing is what shapes the rest of the design: see [`docs/structure/design-philosophy.md`](docs/structure/design-philosophy.md) for the philosophy (the binary as a *ranker*, not just a query engine), and [`docs/structure/project-structure.md`](docs/structure/project-structure.md) for the layout that follows from it.

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

- Not a Dataview replacement inside Obsidian. It imitates DQL *from outside*.
- Not a universal repo-introspection tool. Code → tree-sitter/LSP. Raw text → ripgrep. Configs → jq/yq. This fills the remaining gap: **prose-structured-data querying**.
- Not an Obsidian plugin or daemon. The vault is read directly from disk; Obsidian doesn't need to be running.

## License

[MIT](LICENSE).
