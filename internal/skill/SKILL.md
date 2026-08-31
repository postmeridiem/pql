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

**If that fails with exit `70` naming a file**, indexing aborted on malformed
frontmatter:

```
{"code":"cli.exit","msg":"indexer: extract \"notes/x.md\": markdown: parse frontmatter: yaml: …"}
```

Every *vault* command then fails the same way until the file is excluded. The
planning surface is unaffected — `ticket` and `decisions` read `pql.db` and
keep working, so a broken index does not block planning work.

Exclude it either way: write the path into a `.pqlignore` at the vault root
(gitignore syntax, read by default, no config file needed), or add a doublestar
pattern to `exclude:` in `.pql/config.yaml`. Indexing stops at the *first* bad
file, so expect to repeat this if there are several.

**Both `.gitignore` and `.pqlignore` are read by default**, in that order, with
later files winning. So a file already excluded from git is already out of the
index — worth knowing when a file you expected to find is missing and there is
no `.pqlignore` in sight. `pql doctor` reports the active list under
`config.ignore_files`.

Do not use `pql doctor` to confirm the fix. It reports whatever the last
successful index left behind — a healthy-looking `index.files: 44` while every
command is still failing — because it never triggers indexing itself. Re-run
the command that failed; its diagnostic names the offending file.

## Global flags

These work on every command:

| Flag | Does |
|---|---|
| `--vault <path>` | Query a vault other than the current directory (env `PQL_VAULT`). **This is how you avoid `cd x && pql …`**, which the permission rules reject |
| `--db <path>` | Point at a different database (env `PQL_DB`) — use it to keep a probe from touching a vault's own state |
| `--config <path>` | Config override (env `PQL_CONFIG`) |
| `--pretty` · `--jsonl` · `-n/--limit N` | Output shaping |
| `--grep <regex>` | Keep only the records with a matching value. **This is how you avoid piping to `grep`, `jq` or `python3`** — see below |
| `--quiet` · `--verbose` | Suppress stderr warnings · add per-phase timings |
| `--flat-search` | Force the primitive path (see the caveat under ranked answers) |

## Choosing a command

| The question | Reach for |
|---|---|
| "What is this file about / what should I read alongside it?" | `pql context`, `pql related` |
| "Which notes mention this topic?" | `pql search` — but read the caveat below |
| "Which files match this exact structure?" | `pql query`, `pql files`, `pql tags` |
| "What values does this frontmatter key take?" | `pql query "SELECT DISTINCT fm.<key>"` — not `pql schema`, which gives types |
| "What links to / from this file?" | `pql backlinks`, `pql outlinks` |
| "What is in this one file?" | `pql meta` |
| "What saved views does this vault have?" | `pql base` (bare, to list them) |
| "What did we decide about X?" | `pql decisions` |
| "What should I work on?" | `pql plan whatsnext`, `pql ticket` |
| "Where is this vault resolving from?" | `pql doctor` |

---

# Surface 1: the vault

## Ranked answers

Three commands return **ranked** results rather than exact matches. All
three share one output shape: `path` and `score` by default, and — behind
`--full` — a `signals[]` array showing each signal's raw value, weight and
contribution, plus a `connections[]` array. So a result is always
accountable: you can see *why* it ranked where it did, when you ask.

| Command | Answers |
|---|---|
| `pql search <query>` | Which files are most relevant to this topic |
| `pql related <path>` | Which files sit near this one in the graph |
| `pql context <path>` | What to read to understand this file — the files it links to, that link to it, and that share its tags |

They differ in how they weight the same signals: `related` on link overlap
(0.35), `context` on link overlap and path proximity together (0.30/0.25),
`search` on centrality (0.40) with recency second (0.25).

Those weights only bite where the signal exists. In a link-sparse vault
centrality and link overlap are 0 on every candidate, so a vague query gets
decided by whatever is left — often recency or path proximity — and returns the
most recently touched or most adjacent files rather than the most relevant.

**If results look arbitrary, re-run with `--full` and read `signals[]`.** Do not
try to infer the mechanism from the score: a score that happens to equal one
weight does not mean that signal decided it, and the weights differ per command
(recency is 0.25 on `search` but 0.05 on `related` and `context`). `signals[]`
gives you the raw value, weight and contribution for every signal, which is the
whole reason it exists.

**`pql search` is a substring filter, not a search engine.** Read this
before using it. The query is matched as **one literal lowercase substring**
against file paths, tags, frontmatter values and headings — **never against
body prose** — and whatever survives that gate is then ranked structurally.
It does not split on words, so a multi-word query almost always returns
`[]`:

```bash
pql search "loss"              # matches plasticity-loss-2025.md
pql search "plasticity-loss"   # matches
pql search "plasticity loss"   # [] — the space kills it
```

Pass a single term or a hyphenated filename fragment. For anything in the
body of a document, use `grep`/`rg`. **An empty result is not evidence the
topic is absent** — never report it as such.

`--flat-search` on these three does not give you "the same candidates,
unranked" — it drops candidate selection too, degrading to a plain file
list. Use it to confirm the index is populated, not to get unranked results.

### Asking for the provenance

The default is the answer, not its derivation — five signal objects per result
is most of the payload and usually not what you asked for. Widen it when you
need to:

```bash
pql related notes/topic.md                       # path + score
pql related notes/topic.md --full                # + signals[] and connections[]
pql related notes/topic.md --fields path,signals # exactly these keys
pql related notes/topic.md --oneline             # path<TAB>score, plain text
```

`connections[]` (under `--full`) is `{path, relation}` with `relation` of
`inlink` or `outlink`. It is often the most useful part: it tells you *how* a
result relates. Naming a key in `--fields` always returns it, so
`--fields path,connections` gets that alone.

## Exact structure

| Command | Returns |
|---|---|
| `pql files [glob]` | Indexed files, optionally glob-filtered |
| `pql tags [--sort count]` | Distinct tags with counts |
| `pql backlinks <path>` | Links pointing **at** a path — one row per link occurrence, `{path, name, line, via}`. Read the matching caveat below before trusting `[]` |
| `pql outlinks <path>` | Links **from** a file, in document order — `{target, alias, line, via}`. `target` is the raw text of the link, not a resolved path |
| `pql meta <path>` | One file's frontmatter, tags, outlinks and headings |
| `pql schema` | Inferred frontmatter schema across the vault |
| `pql base [name] [--view V]` | Execute an Obsidian `.base` file. **Bare `pql base` lists the bases it found** with their names — the only discovery route, since `files '*.base'` returns `[]`. `--view` picks among a base's named views; there is no way to list those, so pass a wrong one and read the valid set off the exit-`65` error. Rows carry the columns the `.base` declares and nothing else — often no `path`, so results may not be feedable onward |

```bash
pql files 'sessions/*'          # note: * crosses /, so this is recursive
pql tags --sort count --limit 20
pql meta members/vaasa/persona.md --pretty
pql backlinks members/koskela/persona.md
```

### Links are matched as text, not resolved

This is the single biggest trap in the vault surface, and it cuts both ways.

**`backlinks` compares spellings.** It tries the path you give, that path
without `.md`, the bare basename, and each with a `#anchor` — but a link written
any other way is simply a different string. The common miss is a
directory-relative link: a file at `governance/README.md` writing
`[](decisions/architecture.md)` is *not* found by
`backlinks governance/decisions/architecture.md`, even though that is the
correct vault-relative path and the one every other pql command returns.

So `[]` from `backlinks` means "no link written in a spelling I recognise". It
is evidence, not proof. When it looks wrong:

```bash
pql outlinks governance/README.md          # find how the link is actually written
pql backlinks decisions/architecture.md    # then query that exact string
```

**`outlinks` returns the raw link text**, so its `target` is often not something
another command accepts:

```bash
pql outlinks notes/a.md     # -> {"target":"notes/b","alias":"B","line":12,"via":"wiki"}
pql meta notes/b            # exit 66 — file not indexed
pql meta notes/b.md         # works
```

Append `.md` when the target has no extension. If it still 66s, the link is
relative or shortest-path and you will have to resolve it yourself.

`pql context` is the exception worth knowing: it *does* resolve outbound targets
against the index, so every path it returns can be fed straight to another
command. Prefer it when you want "what does this file point at, as real paths".

## The DSL

`pql query` takes a SQL-derived language over the index. `pql shell` is the
same thing as a REPL — it indexes once, then runs a query per line, which is
worth it for more than a handful of queries.

```sql
SELECT name, fm.date WHERE fm.type = 'meeting' ORDER BY fm.date DESC LIMIT 10
SELECT path WHERE 'project' IN tags ORDER BY path
SELECT name, fm.date WHERE fm.date BETWEEN '2024-01-01' AND '2024-12-31'
```

**Columns.** `path`, `name`, `folder`, `size`, `mtime`, `ctime`,
`content_hash`, `last_scanned`, plus `fm.<key>` for any frontmatter key.
`tags`, `headings`, `inlinks` and `outlinks` are array columns — usable only on
the right of `IN`, not selected bare. An unknown name exits `65` and lists the
set, so the error is a usable lookup.

**Operators.** `=`, `!=`, `<`, `>`, `<=`, `>=`, `LIKE`, `BETWEEN`, `IN`, `AND`,
`OR`, `NOT`. `SELECT DISTINCT` works. Aggregates do not — `COUNT(*)` and
`GROUP BY` fail loudly at exit `65`, so count client-side or use `pql tags`.

**`SELECT DISTINCT` is how you find which values a frontmatter key takes.**
`pql schema` reports keys, types and file counts — never values. This is the
only route, and it is one call:

```bash
pql query "SELECT DISTINCT fm.status"
pql query "SELECT path, folder WHERE folder = 'members/vaasa'"
pql query "SELECT path WHERE 'Design notes' IN headings"
pql query "SELECT path, size WHERE path LIKE 'notes/%' AND size > 5000"
```

Use `--file q.pql` or `--stdin` for long queries, and never interpolate vault
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
| `pql decisions list [--type T] [--domain D] [--status S]` | List records. `--type confirmed\|question\|rejected`, `--status active\|superseded\|resolved\|open` |
| `pql decisions show <id[,id,…]> [--with-refs] [--with-tickets] [--fields …]` | One or more records, optionally with cross-references or the tickets implementing them. `--fields` narrows the top level; the joins are all-or-nothing |
| `pql decisions read <id>` | The record's full markdown body |
| `pql decisions refs <id>` | Cross-references involving a record |
| `pql decisions claim <D\|Q\|R> <domain> "title"` | Print the next free id. No side effects |
| `pql decisions resolve <Q-N> --into <D-N>` | Close a question into the decision that answers it |

Record type is `confirmed`, `question` or `rejected` — the D/Q/R of the tree —
and status is `active`, `superseded`, `resolved` or `open`. Passing `--type Q`
or `--status OPEN` is not an error; it returns an empty list at exit 0. See
the filter-value warning under Contracts.

### Which type to reach for

The type names read narrower than they are, and the surface pushes you that
way: the command, the directory and `--decision` all say *decision*. Two rules
recover most of the value.

**A deferral is a Q, not a sentence in a D.** When you decide to decide later,
write the question down. Left as prose inside some other record it is invisible
to `decisions list --status open`, and nothing will ever surface it again.

**A D is a home for durable documentation, not only for a choice between
alternatives.** The type is called `confirmed` rather than `decision` precisely
because it holds anything settled and worth keeping — an invariant, a
convention, the shape of a subsystem — not just a fork in the road with a
winner. If it is stable and someone will need it in six months, it is a D.

### Closing a question

`pql decisions resolve Q-2 --into D-23` marks the question resolved and records
which decision answered it. It **edits the markdown**, because the DQR tree is
the source of truth — a status written only to pql.db is reverted by the next
sync — and then re-syncs so the change is queryable immediately.

One line in the question's file is all it writes, and that is enough for both
records: `decisions refs Q-2` and `decisions show D-23 --with-refs` each
surface the link, because refs are looked up from either end. The decision also
gets a readable `**Raised by:**` line when it has none; when it already has one
that field is left alone, and the receipt says so.

Both ids are validated — an unknown id, or a `D-` where a `Q-` belongs, exits
non-zero naming the problem rather than doing nothing quietly.

An open question that is never formally closed stays open forever, so a vault
reporting many open questions is ambiguous between "genuinely undecided" and
"nobody had a cheap way to close them". Using this verb is what keeps that
count meaningful.

The markdown is the source of truth, so **run `pql decisions sync` before
querying** whenever the DQR files may have changed — otherwise you are
reading a stale copy. A record written but not synced simply will not be
found. If you cannot write — a read-only or review-only remit — do not run
`sync`; check the `synced_at` field on each record instead to judge how stale
the copy is, and say so rather than silently reporting possibly-old data.

`decisions show <id> --with-tickets` is the implementation-status view: it
answers "is this decision actually built?" and is only as complete as the
ticket links happen to be. Batch it — `decisions show D-1,D-2,D-3
--with-tickets` — to ask that across a whole set in one call rather than one
call per record. Like `ticket show`, one id returns an object and several
return an array.

### Asking what is *missing*

There are no negative filters — no `--unimplemented`, no `--decision none`, no
`--without-<x>`. Every one would be a join predicate in disguise, and a flag
per predicate does not compose. Spellings that imply one exit `64` rather than
returning an empty list you might read as an answer.

The pattern is one batched call plus a fold. To find decisions nothing
implements:

```bash
pql decisions list --oneline                              # source the ids
pql decisions show D-1,D-2,D-3 --with-tickets --fields id,tickets
```

Then keep the rows with no `tickets` key — absent, not empty, per the
omitted-not-null rule. **Name the join key in `--fields`**: `--fields id`
alone drops `tickets` and every row looks unimplemented.

Same shape for any absence question: batch the positive facts, fold client
side.

## Tickets

**Creating and editing**

| Command | Does |
|---|---|
| `pql ticket new <type> "title" [--parent T-N] [--decision D-N] [--priority P] [--assign A] [--team T] [--description ...] [--id-only]` | Create. Types: initiative, epic, story, task, bug. Returns `{"id":"T-N"}` and nothing else — confirming the other fields landed needs a follow-up `show`. `--id-only` drops the JSON wrapper and prints the bare id |
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
| `pql ticket list [--status S] [--team T] [--assigned A] [--label L] [--decision D-N] [--under T-N] [--leaf] [--unblocked]` | Filtered list, uncapped unless you pass `--limit`. `--decision` is the one that makes decision→ticket questions a single call; `--under` = all descendants (not the parent itself); `--leaf` = no children; `--unblocked` = every blocker reached a terminal status. No `--type` filter exists |
| `pql ticket show <id[,id,…]> [--with-context] [--with-blockers] [--with-children] [--tree] [--depth N] [--fields …]` | One or more full records. `--tree` = nested descendants plus the direct parent. `--fields` narrows the top level; the joins are all-or-nothing |
| `pql ticket board [--team T] [--open] [--status S,S]` | Kanban view: one column per status, each with compact rows and a display `label`. `--open` drops the terminal columns — usually most of the payload on a mature board. `--status` names an exact column set; an unknown name exits `64` here rather than returning empty. **Empty columns are omitted**, so a named column that holds nothing is simply absent — `[]` means no tickets in any requested column, not a bad name |
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

- **stdout:** JSON, and the shape follows one rule: **verbs that answer "which
  things" return an array; everything else returns a single object.** Arrays
  come from `files`, `tags`, `backlinks`, `outlinks`, `schema`, `base`, `query`,
  the ranked verbs, and the `list` verbs. Objects come from everything else —
  one record (`meta`, `ticket show <one-id>`, `decisions show <one-id>`,
  `decisions read`), a dashboard (`plan status`, `doctor`, `version
  --build-info`, `watch status`), or a summary of what a command did
  (`ticket new`, `decisions sync`, `plan export`).

  Two edges worth knowing. **The batching verbs switch shape**: `ticket show`
  and `decisions show` return an object for one id and an array for several.
  And `plan whatsnext` / `plan review` return `{"message": "..."}` instead of a
  record when there is nothing to hand back — reaching straight for `.id` gets
  you nothing and no error.

  Do not write one parser assuming an array. `--jsonl` for one object per line,
  `--pretty` for humans, `--limit N` to cap. Two surfaces opt out of JSON
  deliberately: `ticket new --id-only` prints a bare id, and `--oneline` prints
  `id<TAB>status<TAB>title` on the list verbs, `path<TAB>score` on the ranked
  ones. A third changes the shape without leaving JSON: `--grep` drops the
  enclosing array and emits one record per line, so an array parser breaks on
  it while a JSONL reader does not.
- **stderr:** JSON diagnostics, one per line. Codes come in two shapes:
  `pql.<phase>.<kind>` for index, parse, eval and plan problems
  (`pql.parse.unexpected_token`), and `cli.error` / `cli.exit` for flag and
  argument problems. Match on both; pass them back verbatim rather than
  paraphrasing.
- **Exit codes:** `0` success · `64` bad flag · `65` parse or data error ·
  `66` vault/config not found · `69` unavailable · `70` internal.

**Zero matches is success**: exit `0` with an empty `[]`. Report "nothing
matched", never "the command failed".

**But filter values are not validated.** A misspelled or invented value —
`--type Q`, `--status OPEN`, `--status active` on tickets — returns an empty
list at exit `0`, indistinguishable from a genuine no-match. Combined with
the rule above, that is how an agent ends up reporting "there are no open
questions" when there are twelve. **If a filtered query returns nothing,
re-run it unfiltered before concluding the data is absent.** Flag names *are*
validated: an unknown flag exits `64`.

Two exceptions, both checked against a known finite set: `--fields` (unknown
key exits `64` listing the valid ones) and `ticket board --status` (unknown
status exits `64` listing the vocabulary). Everywhere else, assume a filter
value is passed straight through.

**Nothing is ever `null` — but "empty" takes two forms.** On the planning
surface an empty field is **omitted**: a ticket with no parent has no
`parent_id` key, a decision with no tickets has no `tickets` key, `doctor` with
no index has no `index` key. Elsewhere an empty *collection* is present and
empty: `meta` returns `"tags": []`, `plan export` returns
`"files_written": []`. So test for presence **or** emptiness, and never
dereference assuming a key that is present is populated.

The one deliberate exception is the DSL, which does emit `null`:

```bash
pql query "SELECT name, fm.lens"   # {"fm.lens":null,"name":"NOTE"}
```

That is meaningful rather than sloppy — a `SELECT` over files where some carry
the key and some do not needs to say which is which, and a missing key would
make the rows ragged. Null-check frontmatter columns from `query`.

## Projection

`--fields a,b,c` returns only those keys in that order, `--fields '*'` returns
all of them, and `--oneline` gives a plain-text index. They work on the list
verbs (`ticket list`, `decisions list`) and on the ranked verbs (`search`,
`related`, `context`).

Two verbs trim their default output because one key dominates the payload:
`ticket list` omits `description`, and the ranked verbs omit `signals[]` and
`connections[]`. `--full` opts back in on both, and naming the key in
`--fields` always returns it.

`--fields` also works on `ticket show` and `decisions show`, with the same
vocabulary — one projection language across the planning surface. `--oneline`
and `--full` remain list-verb flags and exit `64` on `show`.

**On the show verbs, `--fields` selects the joins too.** A join key you do not
name is dropped even though you passed its `--with-…` flag — silently, at exit
`0`:

```bash
pql ticket show T-5 --with-context --fields id,title            # no ancestors
pql ticket show T-5 --with-context --fields id,title,ancestors  # ancestors present
```

So if you are trimming payload *and* asking for a join, name the join key. What
you cannot do is project *inside* a join: you get the whole subtree or none of
it.

Valid field names, since guessing them costs a round trip:

- **tickets** — `id`, `record_id`, `type`, `title`, `description`, `status`,
  `priority`, `parent_id`, `assigned_to`, `team`, `decision_ref`, `created_at`,
  `updated_at`. Note `decision_ref`, not `decision_id`.
- **ticket joins** — `ancestors` (`--with-context`), `children`
  (`--with-children`), `blockers` (`--with-blockers`), `subtree` (`--tree`),
  `decisions` (`--with-context`), and `message` for the empty-result shape.
- **decisions** — `id`, `type`, `domain`, `title`, `status`, `date`,
  `file_path`, `synced_at`; joins `tickets` (`--with-tickets`) and `refs`
  (`--with-refs`).
- **ranked results** — `path`, `score`, `signals`, `connections`. Under
  `--flat-search` there is no ranking, so `path` is the only valid name.

Projection does **not** reach the mutation verbs, deliberately: their return
value is a receipt confirming the change landed, and a receipt you trimmed to
`id` confirms nothing. `ticket assign`, `status`, `label` and friends reject
`--fields` at exit `64`.

Two receipt shapes, and which you get depends on how many records changed.
Changing **one** returns that whole record, description included — so a single
`ticket status` on a well-described ticket is a few KB. Changing **several**
returns a summary naming what was applied:

```bash
pql ticket label T-1,T-2 add urgent
# {"ticket_ids":["T-1","T-2"],"action":"add","label":"urgent"}
```

Not every batch verb has converged on the summary shape yet, so a multi-id
call may still return N whole records. Budget for that, and prefer batching to
looping either way.

An unknown name exits `64` and prints the valid set, so the error is a usable
lookup if you forget.

`--oneline` prints **nothing at all** on no match — zero bytes, not `[]`. Do
not read that as a failed call.

Prefer these over piping to `jq`: they are cheaper and they compose with
`--limit`.

## Filtering by content: `--grep`

`--fields` and `--oneline` trim *what each record shows*. `--grep <regex>`
trims *which records you get*, and it is the answer to "which of these mention
X" on any verb:

```bash
pql ticket list --grep changelog
pql decisions list --grep 'ticket|changelog' --oneline
pql ticket show T-99 --grep resolve
pql files --grep '^governance/'
```

**Reach for it instead of a pipe.** This is not a style preference — a pipe
costs the caller an approval prompt every time, because the permission rules
match the whole command string and a pipeline containing pql is not a pql
command. `pql ticket list | grep changelog` prompts; `pql ticket list --grep
changelog` does not. The same goes for `jq` and for `python3 -c`, and reaching
for an interpreter to dodge the prompt is worse than the prompt: blanket-
allowing one is an unbounded write grant.

It filters the output buffer, so it means exactly what the pipe meant:
`pql X --limit 5 --grep p` is `pql X --limit 5 | grep p`. `--limit` picks the
page, `--grep` filters that page.

The rules, all of which have a reason you can predict from:

- **Case-insensitive regex** (RE2 — alternation and anchors work,
  backreferences and lookaround do not). An unparseable pattern exits `64`
  naming it.
- **One record per line, no enclosing array.** Each line is valid JSON on its
  own, so the result is still machine-readable as JSONL — a filtered answer is
  not a text dump. Don't hand it to a parser expecting an array.
- **Zero matches emits zero bytes** at exit `0`, like `--oneline`. Success, and
  still not evidence of absence.
- **Values are matched, keys are not.** `--grep status` searches what the
  statuses *are*; it does not match every ticket because every ticket has that
  key. Whatever made a record match is visible in the record you get back.
- **Anchors bind to a value, not to the line.** `--grep '^members/vale'`
  matches paths starting with it. Through a pipe that anchor would be useless,
  since every line starts with `{"path":`.
- **It runs after projection.** With `--fields id,title`, `--grep` sees only
  those two values — what you see is what was matched.
- **`--pretty` is refused** at exit `64`: an indented array cannot also be one
  record per line. `--jsonl` is accepted and redundant. `--oneline` composes,
  and there `--grep` matches the text of the emitted line.
- **Mutation verbs refuse it** at exit `64`, for the same reason they refuse
  `--fields`: their output is a receipt, and a receipt that might be suppressed
  confirms nothing.

Its one real limit is the same as every other pql surface: it sees the fields
pql emits, **never the body prose of a markdown file**. For that, `grep`/`rg`
over the files is still the right tool.

## Anti-patterns

- **One command per invocation.** No `&&`, no pipes, no `$(…)`, no
  redirection — the permission rules match by prefix and a shell construction
  containing pql is not a pql command. See Permissions below for what to use
  instead.
- **Don't pipe pql's output to `grep`, `jq` or `python3`** — use `--grep` to
  filter, `--fields` to project, `--limit` to cap. That is what they are for,
  and each avoids a prompt the pipe would have cost.
- **Don't chain `files` then `meta`** to filter — one `query` with a `WHERE`
  does it in a single pass.
- **Don't parse error text** — pass the stderr diagnostic through.
- **Don't treat a ranked empty result as absence.** See the caveat under
  ranked answers.
- **Don't install or upgrade pql** — tell the user.

## When not to use pql

- **Literal string search in the body of a markdown file** → `grep`/`rg`. pql's
  ranking is structural and its `--grep` filters pql's own output, so neither
  reaches body prose.
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

**Write one command per invocation.** A compound command is meant to be split
on `&&`, `||`, `;` and `|`, with each segment matched separately — but piped
commands currently prompt anyway even when every segment is allowlisted, so in
practice each of these interrupts the caller:

```bash
pql decisions sync && pql decisions list   # chained
pql ticket show T-5 | head -20             # piped
pql ticket list | grep changelog           # piped — use --grep
pql ticket list | jq '.[].id'              # piped — use --fields or --oneline
id=$(pql ticket new task "x" --id-only)    # substituted
pql ticket list > out.json                 # redirected
PQL_VAULT=/some/path pql tags              # env-var prefixed
```

That last one matters because this document offers those env vars as an
alternative to the flags. They work when pql is run by a human; under a prefix
allowlist they do not, because the command string starts with `PQL_VAULT=` and
matches no `pql` rule. **Use the flag, not the env var.**

Run one command per invocation and read its output instead. Use `--grep` where
you would have piped to `grep`, `jq` or `python3`, `--fields` and `--oneline`
where you would have projected, `--limit` where you would have used `head`, and
`--pretty` where you would have formatted — all of them are cheaper than a pipe
anyway, since they cut the payload at the source rather than after it has been
produced.

A long or punctuation-heavy argument occasionally trips the same machinery — a
title containing `|` can read as a pipe even when quoted. This is inconsistent
rather than reliable, so do not pre-empt it. React to it: if an invocation is
refused as unparseable instead of failing on its own merits, move the content
into a file. `--file` and `--stdin` work on `ticket append`, `refine write` and
`query`, and handle newlines and embedded quotes cleanly as a bonus.

Two argument shapes fail deterministically and are worth knowing up front. A
positional argument starting with `-` is parsed as a flag, so a title like
`--fields is broken` exits `64` — put `--` before it:

```bash
pql ticket new bug -- "--fields is broken"
```

And any description containing backticks or `$` will be expanded by the shell
before pql ever sees it. Pass those through `--stdin` with a quoted heredoc.

If prompts persist after the allow rules are in place, check for a `deny`
entry that overrides them, and confirm which settings file is actually being
loaded — a project-level file does not merge the way you might expect with a
user-level one.

## Diagnosing

`pql doctor` prints what resolved and why: vault root and how it was found,
config path, database locations, index state, skill status. It is the first
thing to run when pql seems to be looking at the wrong place.

It follows the omitted-not-null rule like everything else: when no index
exists there is no `index` key at all, so branch on `db.exists` rather than
reaching into `index` and hoping.

## Keeping this skill current

`pql skill status` reports drift, `pql skill install`
writes or updates it (`--force` overrides hand edits), and `pql skill show
[name]` echoes what the running binary actually ships — useful for confirming
which version of this document an installed binary carries.

`pql skill show` returns a JSON bundle, so **add `--raw` to read a skill as
text**: it writes one file's bytes and nothing else. Without it you would have
to pipe the JSON through an extractor, and piping is what this document tells
you to avoid. `--raw --file <path>` reaches other files in a multi-file bundle.

The binary bundles more than this document. `pql skill status` lists every
bundled skill — currently `pql` and `clean-house`, a documentation-discipline
pass over a DQR tree — with each one's `lines` and `words` alongside its hash,
so you can size a skill without reading it. `pql skill show <name> --raw` reads
any of them.
