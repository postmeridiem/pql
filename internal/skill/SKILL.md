---
name: pql
description: >
  Query and plan against a markdown vault via the pql CLI. Two surfaces:
  (1) the vault — ranked search, structurally related files, frontmatter,
  wikilinks, tags, headings, Bases, a SQL-derived DSL — use when the user
  asks what is in the vault ("which notes…", "find where…", "what links to
  X", "what's related to this", "what tags", "run a Base", "query the
  vault"); (2) planning — decision records, tickets, project status — use
  when the user asks about decisions, tickets, work items, or planning
  ("sync decisions", "create a ticket", "what's next", "show D-5", "board",
  "refine tickets"). Requires `pql` on PATH. JSON on stdout; zero matches is
  success (exit 0, empty `[]`), not an error.
---

# pql — vault queries + project planning

`pql` indexes a markdown vault into SQLite and answers questions about it.
One binary, two surfaces: **the vault** (what is written down) and
**planning** (what has been decided and what is being worked on). They share
an output contract and a config, and are otherwise independent — you can use
either without the other.

## Before the first query

```bash
command -v pql
```

If it is missing, tell the user to install from
https://github.com/postmeridiem/pql/releases/latest. Do not install or
upgrade it yourself, even though `pql self-update` exists — that is the
user's call.

Then learn the vault's shape once per session:

```bash
pql schema
```

One row per frontmatter key, with observed types and file counts. Write
queries against what it reports, not against what you assume is there.

## Choosing a command

| The question | Reach for |
|---|---|
| "What is this file about / what should I read alongside it?" | `pql context`, `pql related` |
| "Which notes mention this topic?" | `pql search` — but read the caveat below |
| "Which files match this exact structure?" | `pql query`, `pql files`, `pql tags` |
| "What links to / from this file?" | `pql backlinks`, `pql outlinks` |
| "What is in this one file?" | `pql meta` |
| "What did we decide about X?" | `pql decisions` |
| "What should I work on?" | `pql plan whatsnext`, `pql ticket` |
| "Where is this vault resolving from?" | `pql doctor` |

---

# Surface 1: the vault

## Ranked answers

Three commands return **ranked** results rather than exact matches. All
three share one output shape — `path`, `score`, and a `signals[]` array
showing each signal's raw value, weight and contribution — so a result is
always accountable: you can see *why* it ranked where it did.

| Command | Answers |
|---|---|
| `pql search <query>` | Which files are most relevant to this topic |
| `pql related <path>` | Which files sit near this one in the graph |
| `pql context <path>` | What to read to understand this file (returns heading anchors, not just paths) |

They differ in how they weight the same signals — `related` leans on link
overlap, `context` on path proximity, `search` on recency.

**Know what ranking means here.** These rank on *structural* signals — link
overlap, tag overlap, path proximity, centrality, recency. There is no
text-match signal today. So `pql search "some exact phrase"` can return `[]`
even when the phrase is in the vault, and a broad term can rank on recency
alone. Use these to explore the neighbourhood of a topic; use `grep`/`rg`
when you need a literal string. Do not present a ranked result as proof that
something is or is not written down.

Add `--flat-search` to any of them to force the primitive path — raw rows,
no scoring, no enrichment.

## Exact structure

| Command | Returns |
|---|---|
| `pql files [glob]` | Indexed files, optionally glob-filtered |
| `pql tags [--sort count]` | Distinct tags with counts |
| `pql backlinks <path>` | Files linking **to** a path |
| `pql outlinks <path>` | Links **from** a file, in document order |
| `pql meta <path>` | One file's frontmatter, tags, outlinks and headings |
| `pql schema` | Inferred frontmatter schema across the vault |
| `pql base <name>` | Execute an Obsidian `.base` file |

```bash
pql files 'sessions/*'
pql tags --sort count --limit 20
pql backlinks members/vaasa/persona.md
pql meta members/vaasa/persona.md --pretty
```

## The DSL

`pql query` takes a SQL-derived language over the index. `pql shell` is the
same thing as a REPL — it indexes once, then runs a query per line, which is
worth it for more than a handful of queries.

```sql
SELECT name, fm.date WHERE fm.type = 'meeting' ORDER BY fm.date DESC LIMIT 10
SELECT path WHERE 'project' IN tags ORDER BY path
SELECT name, fm.date WHERE fm.date BETWEEN '2024-01-01' AND '2024-12-31'
```

`fm.<key>` reads frontmatter; `tags`, `path` and `name` are built in. Use
`--file q.pql` or `--stdin` for long queries, and never interpolate vault
content into the command line.

## Keeping the index current

The index refreshes on demand, so most sessions need nothing. For live
editing, `pql watch start` runs a foreground watcher; `pql watch status` and
`pql watch stop` manage it. One watcher per vault, explicitly started — there
is no daemon.

---

# Surface 2: planning

Planning state lives in `<vault>/.pql/pql.db`. Unlike the index it is
**user-authored data, not a cache** — it is never silently discarded.

Decision records are parsed from markdown in a DQR tree
(`governance/{decisions,questions,rejected}/<domain>.md` by default,
configurable via `dqr_dir` in `.pql/config.yaml` or `PQL_DQR_DIR`; a flat
`decisions/` is detected as a fallback). Tickets have no markdown source —
they live in SQLite and travel via the changelog described below.

## Decisions

| Command | Does |
|---|---|
| `pql decisions sync [--no-style]` | Parse the DQR tree into pql.db. Also reports style problems (filename, subdir/type mismatch, domain conflicts) unless suppressed |
| `pql decisions validate [--no-style]` | Dry run. Structural errors exit non-zero; style issues only warn |
| `pql decisions list [--type X] [--domain X] [--status X]` | List records |
| `pql decisions show <id> [--with-refs] [--with-tickets]` | One record, optionally with cross-references or the tickets implementing it |
| `pql decisions read <id>` | The record's full markdown body |
| `pql decisions refs <id>` | Cross-references involving a record |
| `pql decisions claim <D\|Q\|R> <domain> "title"` | Print the next free id. No side effects |

The markdown is the source of truth, so **run `pql decisions sync` before
querying** whenever the DQR files may have changed — otherwise you are
reading a stale copy. A record written but not synced simply will not be
found.

`decisions show <id> --with-tickets` is the implementation-status view: it
answers "is this decision actually built?" and is only as complete as the
ticket links happen to be.

## Tickets

**Creating and editing**

| Command | Does |
|---|---|
| `pql ticket new <type> "title" [--parent T-N] [--decision D-N] [--priority P] [--description ...] [--id-only]` | Create. Types: initiative, epic, story, task, bug. `--id-only` prints just the id, for scripts |
| `pql ticket refine write <id> <json\|--file\|--stdin>` | Patch title, description, priority or type from a JSON payload |
| `pql ticket append <id> <text\|--file\|--stdin>` | Append to the description, blank-line separated. Never rewrites existing text |
| `pql ticket refine list` / `refine next [--skip N]` | The queue of tickets with empty descriptions |

**Structure** — each of these is repairable after the fact; none is
create-time-only.

| Command | Does |
|---|---|
| `pql ticket setparent <id[,id,…]> <parent \| none>` | Set or clear the **hierarchy** link (epic → story → task) |
| `pql ticket block <id> --by <other>` / `unblock <id> --from <other>` | Add or remove a **blocker** — a dependency, *not* hierarchy |
| `pql ticket decision <id[,id,…]> <D-N \| none>` | Link tickets to the decision they implement. An unknown id is rejected |
| `pql ticket assign <id> <agent>` · `team <id> <team>` · `label <id> add\|rm <label>` | Assignee, team, labels |
| `pql ticket status <id[,id,…]> <status> [--force]` | Change status. Blocked while open children exist; `--force` cascades to descendants and lists them |
| `pql ticket relabel <id> [--new-label T-N] [--fix-prose]` | Move a friendly label after a collision. Identity and the graph are untouched; `--fix-prose` updates stale mentions in DQR markdown |

**Reading**

| Command | Does |
|---|---|
| `pql ticket list [--status S] [--team T] [--assigned A] [--label L] [--under T-N] [--leaf] [--unblocked]` | Filtered list. `--under` = all descendants; `--leaf` = no children; `--unblocked` = every blocker reached a terminal status |
| `pql ticket show <id[,id,…]> [--with-context] [--with-blockers] [--with-children] [--tree] [--depth N]` | One or more full records. `--tree` = nested descendants plus the direct parent |
| `pql ticket board [--team T]` | Kanban view |
| `pql ticket statuslist` | The configured status vocabulary — what a UI reads to build columns |

Identity: the `T-NNN` you see is a friendly *label* over a stable underlying
`record_id` (also in the output). Two clones can mint the same label without
corrupting anything; `relabel` reconciles it.

Statuses are per-vault (`ticket_statuses` in `.pql/config.yaml`), defaulting
to backlog, ready, in_progress, review, done, cancelled. Each carries a class
— initial, active, review, terminal — which is what the engine reasons about,
so a renamed status still works. Any status may follow any other; the one
rule is that a ticket cannot reach a terminal status while it has open
children.

## Plan-level views

| Command | Does |
|---|---|
| `pql plan status` | Dashboard: decision counts, open questions, ticket totals by status |
| `pql plan whatsnext` | The next ticket to pick up, with its full context bundle |
| `pql plan review` | The next ticket awaiting review, with context |

## How planning state persists

`pql.db` is gitignored. The durable, git-tracked artefact is
`.pql/changelog/` — per-table monthly SQL files. Every ticket mutation writes
through to it synchronously, so it is always current and you never have to
remember to export.

The hooks `pql init` installs carry the rest: `pre-commit` stages the
changelog so it lands with the change that produced it, `post-merge` migrates
and replays what a pull brought in and re-syncs decisions, and
`post-checkout`/`post-rewrite` rebuild `pql.db` after a branch switch.

**A ticket mutation leaves `.pql/changelog/` dirty by design.** The
pre-commit hook stages it. This is expected — do not narrate "the ticket
won't persist until you commit" after every edit.

| Command | When |
|---|---|
| `pql plan upgrade [--dry-run]` | Migrate the changelog forward to the format this binary writes. Runs from `post-merge`; rewrites tracked files, so the result belongs in a commit |
| `pql plan rebuild [--verify]` | Drop the replicated tables and replay from scratch. `--verify` compares every row's hash either side and reports anything that came back changed or missing |
| `pql plan import` | Replay the changelog into `pql.db`. Runs automatically on a fresh clone |
| `pql plan export [--stage]` | Manual catch-up. Normally a no-op, since mutations already write through |

The changelog carries a format version. An older one replays with a loud
`pql.plan.format_stale` warning and is fixed by `pql plan upgrade`; one
*newer* than the binary is refused outright, and the fix is to upgrade pql.
`pql version --build-info` reports every version axis the binary speaks.

---

# Contracts

## Output

- **stdout:** a JSON array. `--jsonl` for one object per line, `--pretty` for
  humans, `--limit N` to cap. Two commands opt out of JSON deliberately:
  `ticket new --id-only` prints a bare id, and `--oneline` on the list verbs
  prints `id<TAB>status<TAB>title`.
- **stderr:** JSON diagnostics, one per line —
  `{"level":"…","code":"pql.<phase>.<kind>","msg":"…"}`. Pass these back
  verbatim rather than paraphrasing them.
- **Exit codes:** `0` success · `64` bad flag · `65` parse or data error ·
  `66` vault/config not found · `69` unavailable · `70` internal.

**Zero matches is success**: exit `0` with an empty `[]`. Report "nothing
matched", never "the command failed".

## Projection

The list verbs (`ticket list`, `decisions list`) take `--fields id,status,title`
to return only those keys in that order, and `--oneline` for a plain-text
index. `ticket list` omits `description` by default because it dominates the
payload; `--full` opts back in. Record-level commands like `ticket show`
always return whole records — projection flags do not apply there.

Prefer these over piping to `jq`: they are cheaper and they compose with
`--limit`.

## Anti-patterns

- **One command per invocation.** No `&&`, no pipes, no `$(…)`, no
  redirection — the permission rules match by prefix and a shell construction
  containing pql is not a pql command. See Permissions below for what to use
  instead.
- **Don't chain `files` then `meta`** to filter — one `query` with a `WHERE`
  does it in a single pass.
- **Don't parse error text** — pass the stderr diagnostic through.
- **Don't treat a ranked empty result as absence.** See the caveat under
  ranked answers.
- **Don't install or upgrade pql** — tell the user.

## When not to use pql

- **Literal string search** → `grep`/`rg`. pql's ranking is structural.
- **Reading a file** → the `Read` tool.
- **Code structure** → tree-sitter or an LSP.
- **Editing vault content** → `Write`/`Edit`. pql never writes to your
  markdown; it only reads it and writes its own state under `.pql/`.

---

# Setup and upkeep

`pql init` brings a directory to a known-good state — config, changelog
scaffolding, git hooks, gitignore entries — and is idempotent, so re-running
it after a pql upgrade is how hooks pick up new behaviour.

## Permissions

`pql init` writes these into the consuming project's
`.claude/settings.json`. If prompts appear on every call, they are missing:

```json
{
  "permissions": {
    "allow": ["Bash(pql)", "Bash(pql *)"]
  }
}
```

**These rules match by prefix, and that shapes how commands must be
written.** `Bash(pql *)` matches an invocation that *is* a pql command. It
does not match a shell construction that merely contains one, so each of
these prompts even though the pql part is allowed:

```bash
pql decisions sync && pql decisions list   # chained
pql ticket show T-5 | head -20             # piped
id=$(pql ticket new task "x" --id-only)    # substituted
pql ticket list > out.json                 # redirected
```

Run one command per invocation and read its output instead. Use `--limit`,
`--fields` and `--oneline` where you would have piped, and `--pretty` where
you would have formatted.

**Metacharacters inside quoted arguments can also break the match**, which is
less obvious. A title or description containing `<`, `>`, `|` or backticks
can cause the whole invocation to be rejected as unparseable even though the
characters are safely quoted:

```bash
pql ticket new task "add <id> | none handling"   # may be refused
```

Pass content like that through a file instead — `--file` and `--stdin` are
available on `ticket append`, `refine write` and `query`, and they avoid the
problem entirely while also handling newlines and quotes cleanly.

If prompts persist after the allow rules are in place, check for a `deny`
entry that overrides them, and confirm which settings file is actually being
loaded — a project-level file does not merge the way you might expect with a
user-level one.

## Diagnosing

`pql doctor` prints what resolved and why: vault root and how it was found,
config path, database locations, index state, skill status. It is the first
thing to run when pql seems to be looking at the wrong place.

## Keeping this skill current

`pql skill status` reports drift, `pql skill install`
writes or updates it (`--force` overrides hand edits), and `pql skill show
[name]` echoes what the running binary actually ships — useful for confirming
which version of this document an installed binary carries.
