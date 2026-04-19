# mql

**Markdown Query Language.** A single-binary CLI that indexes and queries any repo containing Markdown with YAML frontmatter, wikilinks, tags, and Obsidian Bases.

Built for AI agents that need structural introspection of prose-structured repos without brute-force grep+read, and for humans who want Dataview-style queries outside of Obsidian.

## Status

Design-stage. See [`docs/structure/initial-plan.md`](docs/structure/initial-plan.md) for the full plan — architecture, query-language grammar, SQLite schema, distribution, and roadmap.

## Quick idea

```sh
mql 'LIST FROM "sessions" WHERE winner = "vaasa" SORT date DESC LIMIT 5'
mql 'TABLE name, prior_job FROM #council-member WHERE voting = true SORT name ASC'
mql base council-sessions
mql backlinks members/vaasa/persona.md
mql schema
```

One static Go binary. Drop into `~/.local/bin/`. Keeps a SQLite index under `~/.cache/mql/`. No network, no daemon, no Obsidian dependency.

## What this is not

- Not a Dataview replacement inside Obsidian. It imitates DQL *from outside*.
- Not a universal repo-introspection tool. Code → tree-sitter/LSP. Raw text → ripgrep. Configs → jq/yq. This fills the remaining gap: **prose-structured-data querying**.

## License

TBD before first code commit.
