# pql

**Project Query Language.** A single-binary CLI that indexes a markdown vault into SQLite and gives you structural queries, ranked search, and lightweight project planning — all from the command line.

Built to back a [Claude Code](https://claude.com/claude-code) skill, but works fine standalone. One static Go binary, no daemon, no network, no Obsidian dependency.

## Install

Download the latest release from [GitHub Releases](https://github.com/postmeridiem/pql/releases/latest), or build from source:

```sh
go install github.com/postmeridiem/pql/cmd/pql@latest
```

Or clone and build:

```sh
git clone https://github.com/postmeridiem/pql.git
cd pql
make build        # binary at ./bin/pql
make install      # copies to ~/.local/bin/pql
```

After installing, initialize your vault:

```sh
cd /path/to/your/vault
pql init
```

This creates `.pql/config.yaml`, appends `.pql/` to your `.gitignore`, optionally installs the Claude Code skill, and adds `pql` to your Claude Code permissions.

## What it does

### Query your vault

```sh
pql files 'sessions/*'                    # list files by glob
pql tags --sort count --limit 10          # most-used tags
pql backlinks notes/design.md             # who links to this file?
pql meta notes/design.md --pretty         # frontmatter + tags + links + headings
pql schema                                # what frontmatter keys exist?

# SQL-derived DSL for anything else
pql query "SELECT name, fm.date WHERE fm.type = 'meeting' ORDER BY fm.date DESC"
pql query "SELECT path WHERE 'project-x' IN tags"
pql query "SELECT path WHERE 'design.md' IN outlinks"

# Interactive REPL
pql shell

# Run an Obsidian .base file
pql base council-sessions
```

### Ranked search with provenance

```sh
pql related notes/design.md --pretty      # structurally related files
pql search "authentication" --pretty      # ranked search across the vault
pql context notes/design.md --pretty      # context bundle for understanding a file
```

Each result carries `signals[]` (what contributed to its score) and `connections[]` (structural neighbors). Use `--flat-search` on any command to get raw rows without enrichment.

### Project planning

```sh
# Decision records (from decisions/*.md)
pql decisions sync                        # parse markdown → pql.db
pql decisions list --type confirmed       # list confirmed decisions
pql decisions show D-005 --with-refs      # show with cross-references
pql decisions coverage                    # decisions without tickets

# Tickets (SQLite-native)
pql ticket new task "implement auth"      # create a ticket
pql ticket status T-001 in_progress       # transition status
pql ticket board                          # kanban view
pql plan status --pretty                  # dashboard
```

### Keep the index hot

```sh
pql watch start                           # watch for file changes (foreground)
pql watch status                          # is a watcher running?
pql watch stop                            # stop it
```

### Stay up to date

```sh
pql self-update                           # download + verify + replace
pql completion bash > ~/.bashrc.d/pql     # shell completions
```

## Output contract

- **stdout:** JSON array (default), `--jsonl` for streaming, `--pretty` for humans, `--limit N` to cap.
- **stderr:** JSON diagnostics.
- **Exit codes:** `0` = success, `2` = zero matches (not an error), `64` = bad usage, `65` = parse error, `66` = vault not found, `69` = unavailable, `70` = internal error.

Agent-friendly by design: exit code 2 means "ran cleanly, nothing matched" — not a failure.

## Claude Code integration

The skill installs with `pql init` or `pql skill install`. It teaches Claude when and how to use pql instead of grep+read for structural questions about your vault.

Add to your project's `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": ["Bash(pql)", "Bash(pql *)"]
  }
}
```

`pql init` does this automatically.

## How it works

pql indexes your vault's frontmatter, wikilinks, tags, and headings into a local SQLite database at `<vault>/.pql/index.db`. This index is a pure cache — delete it anytime, it rebuilds from the vault on next run.

Planning state (decisions, tickets) lives in a separate `<vault>/.pql/pql.db` with real migrations, because that data can't be regenerated.

The ranking layer combines five signals (link overlap, tag overlap, path proximity, recency, centrality) with intent-specific weight profiles to produce ranked results with full provenance.

## Obsidian compatibility

pql reads the same vault Obsidian reads — frontmatter, `[[wikilinks]]`, `#tags`, `.base` files. No plugin needed; Obsidian doesn't need to be running or installed. The two are independent surfaces over one directory of markdown files.

## Documentation

- [`docs/structure/design-philosophy.md`](docs/structure/design-philosophy.md) — why pql is a ranker, not just a query engine
- [`docs/pql-grammar.md`](docs/pql-grammar.md) — the DSL grammar
- [`docs/signals.md`](docs/signals.md) — what each ranking signal measures
- [`docs/output-contract.md`](docs/output-contract.md) — stdout/stderr/exit-code contract
- [`docs/vault-layout.md`](docs/vault-layout.md) — what lives in `.pql/`
- [`docs/watching.md`](docs/watching.md) — filesystem watcher spec

## License

[MIT](LICENSE).
