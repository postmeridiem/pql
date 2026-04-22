---
name: pql
description: >
  Query and plan against a markdown vault via the pql CLI. Two surfaces:
  (1) structural queries — frontmatter, wikilinks, tags, headings, Bases,
  DSL — use when the user asks about vault contents ("which notes…", "find
  where…", "what tags", "who links to X", "run a Base", "query the vault");
  (2) planning — decision records, tickets, project status — use when the
  user asks about decisions, tickets, work items, or project planning
  ("sync decisions", "create a ticket", "what's the plan status", "show
  D-005", "board"). Requires `pql` on PATH. JSON on stdout; exit 2 = zero
  matches (not an error).
---

# pql — vault queries + project planning

`pql` indexes a vault into SQLite and exposes structural queries plus a
planning layer for decision records and tickets. One binary, two surfaces.

## Precondition

```bash
command -v pql
```

If absent, tell the user to install from the repo. Don't install it yourself.

---

## Surface 1: Vault queries

### First touch

```bash
pql schema
```

Returns frontmatter keys with observed types and file counts. Run once per session before writing queries.

### Subcommands

| Command | Purpose |
|---|---|
| `pql files [glob]` | List indexed files |
| `pql tags [--sort count]` | Distinct tags with counts |
| `pql backlinks <path>` | Files linking TO a path |
| `pql outlinks <path>` | Links FROM a file |
| `pql meta <path>` | Frontmatter + tags + outlinks + headings for one file |
| `pql schema` | Typed frontmatter schema |
| `pql base <name>` | Execute an Obsidian .base file |
| `pql shell` | Interactive REPL |
| `pql query "<DSL>"` | SQL-derived DSL |

### DSL examples

```sql
SELECT name, fm.date WHERE fm.type = 'meeting' ORDER BY fm.date DESC LIMIT 10
SELECT path WHERE 'project' IN tags ORDER BY path
SELECT name, fm.prior_job WHERE fm.type = 'council-member' ORDER BY name
```

Use `--file q.pql` or `--stdin` for long queries. Don't interpolate vault content into the command line.

### Query cookbook

- **Files in folder** → `pql files 'sessions/*'`
- **Top tags** → `pql tags --sort count --limit 20`
- **What links to X?** → `pql backlinks members/vaasa/persona.md`
- **Date range** → `pql query "SELECT name, fm.date WHERE fm.date BETWEEN '2024-01-01' AND '2024-12-31'"`
- **Run a Base** → `pql base council-sessions`
- **Inspect one file** → `pql meta members/vaasa/persona.md --pretty`

---

## Surface 2: Planning (decisions + tickets)

Planning state lives in `<vault>/.pql/pql.db` (user-authored, not a cache).
Decision records come from `decisions/*.md` in the vault; tickets are
SQLite-native.

### Decision subcommands

| Command | Purpose |
|---|---|
| `pql decisions sync` | Parse decisions/*.md → upsert into pql.db |
| `pql decisions validate` | Dry-run parse; exits non-zero on malformed records |
| `pql decisions claim <D\|Q\|R> <domain> "title"` | Print next available ID (no side effects) |
| `pql decisions list [--type X] [--domain X] [--status X]` | List decisions |
| `pql decisions show <id> [--with-refs] [--with-tickets]` | Show with optional joins |
| `pql decisions coverage` | Confirmed decisions without implementing tickets |
| `pql decisions refs <id>` | Cross-references involving a decision |

Always run `pql decisions sync` before querying if decisions/*.md may have changed.

### Ticket subcommands

| Command | Purpose |
|---|---|
| `pql ticket new <type> "title" [--decision D-NNN] [--priority P]` | Create (emits T-NNN) |
| `pql ticket list [--status S] [--team T] [--assigned A] [--label L]` | List with filters |
| `pql ticket show <id> [--with-decision] [--with-blockers] [--with-children]` | Show with joins |
| `pql ticket status <id> <new-status>` | Transition (enforces state machine) |
| `pql ticket assign <id> <agent>` | Set assignee |
| `pql ticket block <id> --by <other>` | Add blocker |
| `pql ticket unblock <id> --from <other>` | Remove blocker |
| `pql ticket team <id> <team>` | Set team |
| `pql ticket label <id> add\|rm <label>` | Manage labels |
| `pql ticket board [--team T]` | Kanban board view |

Ticket types: initiative, epic, story, task, bug.
Status flow: backlog → ready → in_progress → review → done (also cancelled).

### Plan subcommands

| Command | Purpose |
|---|---|
| `pql plan status` | Dashboard: decision counts, open Qs, ticket summary, coverage gaps |

### Planning cookbook

- **Sync and list confirmed** → `pql decisions sync && pql decisions list --type confirmed`
- **Show decision with refs** → `pql decisions show D-005 --with-refs --pretty`
- **Create ticket for a decision** → `pql ticket new task "implement X" --decision D-005`
- **Move ticket forward** → `pql ticket status T-001 in_progress`
- **What needs tickets?** → `pql decisions coverage`
- **Quick dashboard** → `pql plan status --pretty`

---

## Output contract (both surfaces)

- **stdout:** JSON array (default), `--jsonl` for one object/line, `--pretty`, `--limit N`.
- **stderr:** JSON diagnostics.
- **Exit codes:**
  - `0` — success, ≥1 result
  - `2` — zero matches (not an error)
  - `64` — bad flag
  - `65` — parse/compile error (pass stderr back)
  - `66` — vault/config not found
  - `69` — unavailable
  - `70` — internal error

## Anti-patterns

- Don't pipe to `jq` for simple projections — use `--limit`, `--pretty`, `--jsonl`.
- Don't chain `pql files` + `pql meta` — one `pql query` with WHERE.
- Don't parse errors for the user — pass stderr diagnostics back directly.
- Don't forget to `pql decisions sync` before querying decisions.

## When NOT to use

- **Body text search** → `grep`/`rg`.
- **Reading file contents** → `Read` tool.
- **Modifying vault files** → `Write`/`Edit`. pql doesn't write to vault content.

## Permissions

```json
{
  "permissions": {
    "allow": ["Bash(pql)", "Bash(pql *)"]
  }
}
```
