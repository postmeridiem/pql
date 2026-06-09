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
  D-5", "board", "refine tickets", "tickets without descriptions").
  Requires `pql` on PATH. JSON on stdout; zero matches is success (exit 0,
  empty `[]`), not an error.
---

# pql — vault queries + project planning

`pql` indexes a vault into SQLite and exposes structural queries plus a
planning layer for decision records and tickets. One binary, two surfaces.

## Precondition

```bash
command -v pql
```

If absent, tell the user to install from
https://github.com/postmeridiem/pql/releases/latest. Don't install it
yourself. Don't fall back to grep unless the user explicitly asks.

## First touch: learn the vault

```bash
pql schema
```

Returns one row per frontmatter key with observed types and file counts.
Run once per session before writing queries.

---

## Surface 1: Vault queries

### Subcommands

| Command | Purpose |
|---|---|
| `pql files [glob]` | List indexed files; optional glob filter |
| `pql tags [--sort count]` | Distinct tags with counts |
| `pql backlinks <path>` | Files linking TO a path |
| `pql outlinks <path>` | Links FROM a file |
| `pql meta <path>` | Frontmatter + tags + outlinks + headings for one file |
| `pql schema` | Typed frontmatter schema |
| `pql base <name>` | Execute an Obsidian .base file |
| `pql shell` | Interactive REPL (indexes once, then query per line) |
| `pql query "<DSL>"` | SQL-derived DSL for complex queries |
| `pql doctor` | Resolved vault/config/DB/index state |

### DSL examples

```sql
SELECT name, fm.date WHERE fm.type = 'meeting' ORDER BY fm.date DESC LIMIT 10
SELECT path WHERE 'project' IN tags ORDER BY path
SELECT name, fm.prior_job WHERE fm.type = 'council-member' ORDER BY name
```

Use `--file q.pql` or `--stdin` for long queries. Don't interpolate vault
content into the command line.

### Query cookbook

- **Files in folder** → `pql files 'sessions/*'`
- **Top tags** → `pql tags --sort count --limit 20`
- **What links to X?** → `pql backlinks members/vaasa/persona.md`
- **Date range** → `pql query "SELECT name, fm.date WHERE fm.date BETWEEN '2024-01-01' AND '2024-12-31'"`
- **Run a Base** → `pql base council-sessions`
- **Inspect one file** → `pql meta members/vaasa/persona.md --pretty`

---

## Surface 2: Planning (decisions + tickets)

Planning state lives in `<vault>/.pql/pql.db` (user-authored state, not a
cache). Decision records come from the DQR tree — `governance/{decisions,
questions,rejected}/<domain>.md` by default (D-21), configurable via
`dqr_dir` in `.pql/config.yaml` or the `PQL_DQR_DIR` env var (env > file >
default); a legacy flat `decisions/` is auto-detected as a fallback.
Tickets are SQLite-native.

### Decision subcommands

| Command | Purpose |
|---|---|
| `pql decisions sync [--no-style]` | Parse the DQR tree → upsert into pql.db; surfaces style warnings (filename, subdir-type, domain pairing/conflicts) unless `--no-style` |
| `pql decisions validate [--no-style]` | Dry-run parse; structural errors exit non-zero, style issues warn (suppress with `--no-style`) |
| `pql decisions claim <D\|Q\|R> <domain> "title"` | Print next available ID |
| `pql decisions list [--type X] [--domain X] [--status X]` | List decisions |
| `pql decisions show <id> [--with-refs] [--with-tickets]` | Show with joins |
| `pql decisions coverage` | Confirmed decisions without tickets |
| `pql decisions refs <id>` | Cross-references involving a decision |

Always `pql decisions sync` before querying if decisions/*.md may have changed.

### Ticket subcommands

| Command | Purpose |
|---|---|
| `pql ticket new <type> "title" [--decision D-NNN] [--priority P] [--id-only]` | Create (emits T-NNN; `--id-only` prints the bare id for tree-creation scripts) |
| `pql ticket list [--status S] [--team T] [--assigned A] [--label L] [--under T-NNN] [--leaf] [--unblocked]` | List with filters. `--under` = recursive descendants of a ticket; `--leaf` = no children; `--unblocked` = blockers all reached a terminal status |
| `pql ticket show <id[,id,...]> [--with-context] [--with-blockers] [--with-children] [--tree] [--depth N]` | Show one or more (comma-batch → array of show-trees). `--with-children` = direct children; `--tree` = nested descendant subtree + direct parent (cap with `--depth N`) |
| `pql ticket status <id> <new-status> [--force]` | Change status. Closing (terminal status) is blocked while the ticket has open children; `--force` cascades that status to all not-yet-closed descendants and lists them |
| `pql ticket statuslist` | List the configured status vocabulary (name, label, class, order, is_default, is_terminal) — what a UI reads to render columns |
| `pql ticket assign <id> <agent>` | Set assignee |
| `pql ticket append <id> <text\|--file\|--stdin>` | Append to the description (blank-line separated); never round-trips existing text |
| `pql ticket block <id> --by <other>` | Add blocker |
| `pql ticket unblock <id> --from <other>` | Remove blocker |
| `pql ticket team <id> <team>` | Set team |
| `pql ticket label <id> add\|rm <label>` | Manage labels |
| `pql ticket board [--team T]` | Kanban board view |
| `pql ticket refine list` | Tickets with empty descriptions, status-priority-sorted |
| `pql ticket refine next [--skip N]` | Head of the unrefined queue with full show-tree + remaining count |
| `pql ticket refine write <id> <json\|--file\|--stdin>` | Patch writable fields (title, description, priority, type) |

Ticket types: initiative, epic, story, task, bug.
Statuses are a per-vault vocabulary (`ticket_statuses` in `.pql/config.yaml`),
defaulting to: backlog, ready, in_progress, review, done, cancelled. Each status
has a class — initial, active, review, terminal — that the engine reasons about.
Run `pql ticket statuslist` to discover the live set. Any status can transition
to any other — pql does not enforce a state machine — except that a ticket
cannot reach a terminal status while it has open children (use `--force` to
cascade the close down the subtree).

### Plan subcommands

| Command | Purpose |
|---|---|
| `pql plan status` | Dashboard: decision counts, open Qs, ticket summary, coverage gaps |
| `pql plan whatsnext` | Next ticket to work on (active work, then the "ready" lane) with full context bundle |
| `pql plan review` | Next ticket awaiting review with full context bundle |
| `pql plan export [--stage]` | Append changed planning rows to `.pql/changelog/<table>/<YYYY-MM>.sql` (the git-tracked log of record); `--stage` also `git add`s them. Normally a no-op — mutations already write through |
| `pql plan import [--legacy FILE]` | Replay `.pql/changelog/` into `pql.db` (or one-time `--legacy pql-plan.json` migration from the pre-D-15 snapshot) |
| `pql plan rebuild` | Drop replicated tables and replay `.pql/changelog/` from scratch. Warns on stderr (`changelog.ticket_id_collision`) + lists `collisions` in the result if one ticket id was filed twice across clones |

### Versioning planning state

`pql.db` is gitignored — the durable, git-tracked artifact is
`.pql/changelog/` (D-15/D-16). Ticket mutations **write through** to the
changelog synchronously, so it is always current; you never have to
remember to "export". The hooks installed by `pql init` do the rest:

- `pre-commit` stages `.pql/changelog/` so it lands in the same commit as
  the change that produced it.
- `post-merge` replays incoming changelog edits (`pql plan import`) and
  re-syncs decisions from their markdown.
- `post-checkout` / `post-rewrite` rebuild `pql.db` from the changelog.

On a fresh clone, `pql plan import` (run automatically on first open)
replays the changelog into a new `pql.db`. There is **no** `pql-plan.json`
snapshot — that artifact is retired; `pql plan export` is now only a
manual catch-up/reconcile.

A ticket mutation (create / status transition / any write) leaves
`.pql/changelog/` dirty by design — the `pre-commit` hook stages it onto
the next `git commit`. This is expected, not a problem to flag. Don't
narrate "the ticket won't persist until committed" on every edit; either
fold the bookkeeping into a commit or trust the normal commit flow.

### Planning cookbook

- **Sync and list confirmed** → `pql decisions sync && pql decisions list --type confirmed`
- **Show with refs** → `pql decisions show D-5 --with-refs --pretty`
- **Read full body** → `pql decisions read D-5`
- **Create ticket** → `pql ticket new task "implement X" --decision D-5`
- **Create ticket, capture id for a script** → `id=$(pql ticket new task "implement X" --id-only)` — prints just `T-NNN`
- **Batch close** → `pql ticket status T-1,T-2,T-3 done`
- **Full context** → `pql ticket show T-5 --with-context --pretty`
- **Batch show** → `pql ticket show T-1,T-2,T-3 --pretty`
- **Refine next ticket** → `pql ticket refine next --pretty`, then `pql ticket refine write T-N '{"description":"..."}'`
- **Append a note** → `pql ticket append T-5 "benchmarked; TTL now 5m"` — blank-line separated, never overwrites; use `--file note.md` or `--stdin` for longer content
- **Subtree of an epic** → `pql ticket show T-2 --tree --pretty` — nested `subtree` + direct parent in `ancestors`; add `--depth N` to cap levels
- **Ready leaf work under an epic** → `pql ticket list --under T-2 --leaf --unblocked` — leaf tickets beneath T-2 whose blockers have all reached a terminal status; the batch complement to `plan whatsnext`
- **What's next?** → `pql plan whatsnext --pretty`
- **Review queue** → `pql plan review --pretty`
- **Coverage gaps** → `pql decisions coverage`
- **Dashboard** → `pql plan status --pretty`
- **Force a changelog catch-up** → `pql plan export` (normally a no-op; mutations already write through to `.pql/changelog/`)

---

## Output contract (both surfaces)

- **stdout:** JSON array (default); `--jsonl` for one object/line; `--pretty`; `--limit N`.
- **stderr:** JSON diagnostics `{"level":"…","code":"pql.<phase>.<kind>","msg":"…"}`.
- **Exit codes:**
  - `0` — success, including zero matches (empty `[]` — say "no matches", not "failed")
  - `64` — bad flag
  - `65` — parse/compile error (pass stderr back)
  - `66` — vault/config not found
  - `69` — unavailable
  - `70` — internal error

## Anti-patterns

- Don't pipe to `jq` for simple projections — use `--limit`, `--pretty`, `--jsonl`.
- Don't chain `pql files` + `pql meta` — one `pql query` with WHERE.
- Don't parse errors — pass stderr diagnostics back directly.
- Don't forget `pql decisions sync` before querying decisions.
- Don't try to install or upgrade pql — instruct the user if missing.

## When NOT to use

- **Body text search** → `grep`/`rg`.
- **Reading file contents** → `Read` tool.
- **Code structure** → tree-sitter / LSP.
- **Modifying vault files** → `Write`/`Edit`. pql doesn't write to vault content.

## Permissions

The consuming project's `.claude/settings.json` should allow:

```json
{
  "permissions": {
    "allow": ["Bash(pql)", "Bash(pql *)"]
  }
}
```

## Updating the skill

`pql skill status` reports drift. `pql skill install` writes/updates;
`--force` overrides hand-edits. `pql doctor` also surfaces skill state.
`pql skill show [name]` echoes the skill content embedded in *this*
binary (defaults to the `pql` skill; pass a name like `clean-house` for
another) — JSON keyed by file path, `--pretty` to read as text. Works
from any directory, no vault required; handy for confirming exactly
which skill a given binary ships.
