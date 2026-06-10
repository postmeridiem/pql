# Planning subcommands — decisions + tickets

Canonical spec for the `pql decisions`, `pql ticket`, and `pql plan`
command trees. Read alongside `design-philosophy.md` (the "why"),
`project-structure.md` (canonical layout), and `decisions/architecture.md (D-3)`
(the cache vs state split this feature is the first real user of).

This document originated as a hand-off from clide's April 2026 planning
cycle (clide's `D-039` committed "pql owns the planning subcommands;
clide consumes via shell-out"; `D-040` / `R-011` define the sunset
condition for clide's Python stopgap). What's below is what pql needs
to build to replace that stopgap and become the canonical planning
CLI for any project adopting clide's `decisions/` convention
(`D-NNN` confirmed, `Q-NNN` open, `R-NNN` rejected).

Reference materials outside this repo:

- Clide's live convention: `/var/mnt/data/projects/clide/decisions/`
  (markdown source of truth + `README.md` documenting record shape).
- Clide's stopgap: `/var/mnt/data/projects/clide/tools/scripts/plan`
  + `tools/scripts/planning/*.py`. Verb shape, flag names, and
  output formats define the API contract pql must match so the
  migration is a call-site find-replace.
- Settled-reach's originals (Python, Scrum-heavy):
  `/var/mnt/data/projects/settled-reach/main/tooling/db/`
  (`decisions_sync.py`, `ticket`, `decision`) and `db/schema.sql`.

## Locked decisions

1. **Go implementation.** Planning subcommands are new packages under
   `internal/planning/`. No Python, no shellout to the clide stopgap.
2. **DB location: `<vault>/.pql/pql.db`.** Resolves the original
   P-Q-001. Rationale in `decisions/architecture.md (D-3)`:
   `index.db` stays the regenerable cache, `pql.db` is user-authored
   state. Discovered the same way as `.pql/config.yaml` (walk up from
   cwd); created lazily by the first writer. Read-only vault +
   write-intent command fails with a clear diagnostic (we do **not**
   fall back to the user cache for writes — would be invisible to the
   vault itself).
3. **Record IDs as TEXT PKs.** `D-42`, `Q-13`, `T-7` are strings,
   not integers. Easier to preserve identity across imports; no
   translation layer; matches markdown verbatim.
4. **Ticket source of truth = SQLite, for now.** Markdown-mirror
   (`tickets/T-NNN.md` auto-written) is the long-term answer (clide's
   `Q-022`); v1 of this feature is SQLite-only so we can ship without
   the merge-conflict story solved. `pql ticket export --to tickets/`
   is the half-step toward the mirror.
5. **Joined views ship on day one.** The whole point was "get ticket +
   linked D-record in one call." Minimum: `--with-decision`,
   `--with-blockers`, `--with-tickets`, `--with-refs`.
6. **Consumer-agnostic core.** Planning core packages
   (`internal/planning/{schema,parser,repo,format}`) must not import
   cobra or `internal/cli/`. CLI wiring lives in
   `internal/cli/{decisions,ticket,plan}_*.go`. Same discipline the
   query side already follows; keeps the door open for a separate
   planning-surface MCP later without a core rewrite.
7. **Real migrations, not drop-and-rebuild.** `pql.db` holds data that
   cannot be regenerated; the migration runner in
   `internal/planning/schema.go` maintains a `schema_migrations`
   table and applies forward-only. Different regime from `internal/store/`
   (which drops and rebuilds `index.db` on mismatch).

## Schema (Go-native, SQLite-backed)

Adapted from `/var/mnt/data/projects/settled-reach/main/db/schema.sql`,
Scrum layer stripped. Lives in `pql.db`, wholly distinct from the
`files/frontmatter/tags/links/headings/bases` schema in `index.db`.

```sql
CREATE TABLE decisions (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL CHECK(type IN ('confirmed','question','rejected')),
    domain      TEXT NOT NULL,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active'
                    CHECK(status IN ('active','superseded','resolved','open')),
    date        TEXT,
    file_path   TEXT NOT NULL,
    synced_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE decision_refs (
    source_id   TEXT NOT NULL REFERENCES decisions(id),
    target_id   TEXT NOT NULL REFERENCES decisions(id),
    ref_type    TEXT NOT NULL
                    CHECK(ref_type IN ('supersedes','references','resolves','depends_on','amends')),
    note        TEXT,
    PRIMARY KEY (source_id, target_id, ref_type)
);

-- Identity split (D-26): record_id (a ULID) is the stable underwater
-- identity and the target of every reference; the friendly T-NNN label
-- lives in ticket_idmap and is reconcilable. CanonicalVersion is 2.
CREATE TABLE tickets (
    record_id        TEXT PRIMARY KEY,        -- ULID, collision-proof
    type             TEXT NOT NULL CHECK(type IN ('initiative','epic','story','task','bug')),
    parent_record_id TEXT REFERENCES tickets(record_id),
    title            TEXT NOT NULL,
    description      TEXT,
    status           TEXT NOT NULL DEFAULT 'backlog',
                    -- No CHECK enumeration: the status vocabulary is per-vault
                    -- configurable (ticket_statuses in .pql/config.yaml, four
                    -- classes initial/active/review/terminal). Validation lives
                    -- in Go (planning.StatusSet). See D-24.
    priority     TEXT DEFAULT 'medium'
                    CHECK(priority IN ('critical','high','medium','low')),
    assigned_to  TEXT,
    team         TEXT,
    decision_ref TEXT REFERENCES decisions(id),
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE ticket_idmap (             -- record_id <-> friendly T-NNN label
    record_id    TEXT PRIMARY KEY REFERENCES tickets(record_id),
    ticket_id    TEXT NOT NULL           -- NOT globally unique; a dup is a collision
);

CREATE TABLE ticket_deps (
    blocker_record_id TEXT NOT NULL REFERENCES tickets(record_id),
    blocked_record_id TEXT NOT NULL REFERENCES tickets(record_id),
    PRIMARY KEY (blocker_record_id, blocked_record_id)
);

CREATE TABLE ticket_history (
    ticket_record_id TEXT NOT NULL REFERENCES tickets(record_id),
    field        TEXT NOT NULL,
    old_value    TEXT,
    new_value    TEXT,
    changed_by   TEXT,
    changed_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE ticket_labels (
    ticket_record_id TEXT NOT NULL REFERENCES tickets(record_id),
    label        TEXT NOT NULL,
    PRIMARY KEY (ticket_record_id, label)
);

CREATE INDEX idx_tickets_status        ON tickets(status);
CREATE INDEX idx_tickets_team          ON tickets(team);
CREATE INDEX idx_tickets_decision_ref  ON tickets(decision_ref);
CREATE INDEX idx_tickets_assigned      ON tickets(assigned_to);
CREATE INDEX idx_decisions_domain      ON decisions(domain);
CREATE INDEX idx_decisions_type        ON decisions(type);
CREATE INDEX idx_decision_refs_target  ON decision_refs(target_id);
```

## Go package layout

Core stays consumer-agnostic; CLI wiring sits in `internal/cli/`
next to the existing `query_*.go` / `intent_*.go` files.

```
projects/pql/
  internal/
    planning/
      db.go                   # opens <vault>/.pql/pql.db, creates schema if missing
      schema.go               # CREATE TABLE IF NOT EXISTS + CanonicalVersion (no migration runner, D-19)
      canonical.go, rehash.go # canonical row hashing for changelog replication
      parser/
        decisions.go          # DQR markdown → []Record (type from subdir, domain from filename stem)
        headings.go           # heading / anchor helpers
      repo/
        decisions.go          # upsert, list, get
        tickets.go            # crud, joins, history
        meta.go               # local-only meta key/value
        snapshot.go           # legacy pql-plan.json import/export
      changelog/
        exporter.go           # write changed rows → .pql/changelog/<table>/<YYYY-MM>.sql (D-15)
        importer.go           # replay changelog into pql.db, inline LWW guards (D-16)
        rebuild.go            # drop replicated tables + full replay
    cli/
      decisions.go            # pql decisions <verb>  (sync/validate/claim/list/show/read/refs)
      ticket.go, ticket_refine.go  # pql ticket <verb>
      plan.go                 # pql plan <verb>  (status/whatsnext/review/export/rebuild/import)
  cmd/
    pql/main.go               # wires verbs into the cobra root (rendering lives in cli/render, not a planning/format pkg)
```

Rule of thumb: anything that imports cobra lives under `internal/cli/`;
anything the planning core needs to do without cobra lives under
`internal/planning/`. An MCP-style consumer can import
`internal/planning/` directly without dragging the CLI in.

## CLI surface

### `pql decisions`

| command | behaviour |
|---|---|
| `pql decisions sync [--no-style]` | Parse the DQR tree (`governance/…` by default, per D-21) → upsert into `pql.db`. Idempotent; regenerates the README records section. Surfaces style warnings unless `--no-style`. |
| `pql decisions validate [--no-style]` | Parser dry-run; structural errors (duplicate IDs, empty titles, broken refs) exit non-zero, style issues warn (`--no-style` to suppress). Cheap; safe for pre-push hooks. |
| `pql decisions claim D\|Q\|R <domain> "title"` | Print next-available ID. No side effects. |
| `pql decisions list [--domain X] [--type confirmed\|question\|rejected] [--status active\|resolved\|…]` | JSON array (default); `--pretty`/`--jsonl` for shape. |
| `pql decisions show <id> [--with-tickets] [--with-refs]` | Render the decision; optional joins pull linked tickets / cross-refs. |
| `pql decisions read <id>` | Render the decision with its full markdown body. |
| `pql decisions refs <id>` | Graph-walk `decision_refs` in both directions. |

(`pql decisions coverage` was removed per D-20 — implementation status is tracked via initiative tickets and `show --with-tickets`, not a coverage report.)

### `pql ticket`

| command | behaviour |
|---|---|
| `pql ticket new <type> "title" [--parent T-NNN] [--priority P] [--decision D-NNN] [--team T]` | Create. Emits the new `T-NNN`. |
| `pql ticket list [--status S] [--team T] [--assigned A] [--decision D] [--label L] [--under T-NNN] [--leaf] [--unblocked]` | Columnar list; `--output json`. `--under` restricts to recursive descendants of a ticket; `--leaf` to childless tickets; `--unblocked` to tickets whose blockers have all reached a terminal status. Composed (`--under E --leaf --unblocked`), they are the batch complement to `pql plan whatsnext`. |
| `pql ticket show <id> [--with-context] [--with-blockers] [--with-children] [--tree] [--depth N]` | Ticket body + optional joins. `--with-children` lists direct children only; `--tree` nests the full descendant subtree (cap with `--depth N`) and includes the direct parent for context. `--with-context` pulls the full ancestor spine to the root plus linked decisions. `--with-blockers` walks `ticket_deps`. |
| `pql ticket status <id[,id,…]> <new-status> [--force]` | Transition (any status → any status; pql does **not** enforce a state machine, D-14). Status set is the configured vocabulary (D-24); unknown statuses are rejected. A move to a terminal status is blocked while the ticket has open children (D-25); `--force` cascades that terminal status to every not-yet-closed descendant and lists them. Records in `ticket_history`. Comma-batch supported. |
| `pql ticket statuslist` | Emit the configured status vocabulary (D-24) in board order: `name, label, class, order, is_default, is_terminal`. The discovery surface for UIs (e.g. clide). |
| `pql ticket relabel <id\|record_id> [--new-label] [--fix-prose]` | Reassign a ticket's friendly T-NNN label to reconcile a duplicate-label collision (D-26). Updates only the `ticket_idmap` row (identity `record_id` and the structural graph are untouched); reports/optionally rewrites stale T-NNN prose in the DQR tree. |
| `pql ticket assign <id> <agent>` | Set `assigned_to`. |
| `pql ticket setparent <id[,id,…]> <parent-id\|none>` | Set or clear a ticket's parent. |
| `pql ticket append <id> [text] [--file F] [--stdin]` | Append text to the description, separated from existing content by a blank line. Unlike `refine write` (replace-from-JSON-patch), never round-trips the existing text. Records a `description` history row. |
| `pql ticket block <id> --by <other>` / `pql ticket unblock <id> --from <other>` | Maintain `ticket_deps`. |
| `pql ticket team <id> <team>` | Set `team`. |
| `pql ticket label <id> add\|rm <label>` | Manage `ticket_labels`. |
| `pql ticket board [--team T]` | Print kanban columns as JSON, one per configured status in board order (default `backlog`/`ready`/`in_progress`/`review`/`done`/`cancelled`; see `ticket_statuses`). |
| `pql ticket refine list\|next\|write` | Triage tickets with empty descriptions: `list` the queue, `next [--skip N]` zooms in with a full show-tree, `write <id> <json\|--file\|--stdin>` patches writable fields. |

(`pql ticket search` (FTS, Q-2) and `pql ticket export`/markdown mirror (Q-1) are not yet built — see the open questions.)

### `pql plan` (cross-cutting)

| command | behaviour |
|---|---|
| `pql plan status` | Dashboard: decision counts (by type, open Qs) + ticket counts by status. |
| `pql plan whatsnext` | The single next ticket to work on (class `active`, then the "ready" lane — the most-advanced `initial` status — unblocked) with full context bundle. |
| `pql plan review` | The next ticket awaiting review, with full context bundle. |
| `pql plan export [--stage]` | Append changed planning rows to `.pql/changelog/<table>/<YYYY-MM>.sql` (git-committed replication, D-15). `--stage` also `git add`s them. |
| `pql plan rebuild` | Drop replicated tables and replay `.pql/changelog/` from scratch. Warns (`changelog.ticket_id_collision`) when one ticket id is claimed by two lineages — see below (T-54). |
| `pql plan import [--legacy FILE]` | Replay `.pql/changelog/` into `pql.db` (or migrate a legacy `pql-plan.json`). Also surfaces ticket-id collisions. |

**Ticket-ID collision detection (T-54).** IDs are `max(existing)+1` against the
local changelog, so cross-clone/branch filing between syncs can reuse a `T-NNN`.
`tickets` replays with `ON CONFLICT(id) DO UPDATE`, which would silently merge
two unrelated tickets. Since `created_at` is immutable across a ticket's update
rows, replay flags any id carrying two distinct `created_at` lineages as a
non-fatal warning (and in the result JSON's `collisions`), so the maintainer can
re-file one under a fresh id rather than discover a wrong title later.

(`pql plan search` is not built; cross-cutting FTS is part of Q-2.)

## What gets imported to pql's repo

Settled-reach's `tooling/db/` is Python; this is a **reimplementation**
in Go, not a copy:

- Schema — copy SQL verbatim (strip Scrum), translate to Go migration runner.
- Parser — re-implement in Go. Settled-reach's regex patterns are the reference:
  - `^### (D|Q|R)-(\d+): (.+)$` — heading.
  - `^- \*\*Date:\*\* (.+)$` etc. for field extraction.
  - `\*\*Amendment \((.+?)\):\*\*` — inline amendment marker.
  - `[DQRT]-\d+` anywhere in body — cross-ref extraction.
- CLI — re-implement in Cobra alongside pql's existing subcommands.
- Status-inference heuristics — port the `[SUPERSEDED]` title marker
  and `Status: Resolved` body matching, but fix the substring-match
  fragility that settled-reach's `infer_status()` has.

## Open questions

- **P-Q-002:** Markdown mirror strategy (clide's `Q-022` answer feeds
  back here). When does `pql ticket export` become automatic?
  On every mutation? On explicit command only?
- **P-Q-003:** Migration ceremony. Real forward-only migrations in
  `internal/planning/schema.go` is locked; the open question is
  whether we use an off-the-shelf runner (golang-migrate, goose) or
  a thin in-house one. Lean: in-house, ~50 lines, no extra dep.
- **P-Q-004:** FTS — SQLite FTS5 or external index? FTS5 is built-in,
  zero deps; good enough for N < 10k records.
- **P-Q-005:** Permissions / multi-user. Settled-reach is
  single-user-per-DB. If we ever want a team-shared DB (network or
  fileserver), what's the locking story? Probably out of scope until
  `Q-022` markdown-mirror lands.
- **P-Q-006:** Validator strictness. Settled-reach's parser is
  permissive (missing fields don't error); clide's plan wants
  `pql decisions validate` to exit non-zero on malformed records.
  Where's the line?

P-Q-001 (DB location) is resolved — see Locked Decision 2 above +
ADR 0003.

## Commits (tentative)

Refine during implementation. This was the planned landing sequence
(all shipped — see `CHANGELOG.md` and the planning dataset for status):

1. `add internal/planning/ package skeleton + pql.db schema + migration runner`
2. `implement markdown parser for decisions/`
3. `implement decisions subcommands — sync, validate, claim, list, show`
4. `implement ticket subcommands — new, list, show, status, assign`
5. `implement ticket subcommands — block, unblock, team, label, board, search`
6. `implement cross-cutting pql plan — status, search, export`
7. `document new subcommands in pql README + skill cookbook`

## Verification

Sunset gate for clide's stopgap: pql must (a) open the same
`<vault>/.pql/pql.db` the stopgap wrote (stopgap will be updated
to target this filename once the first commit lands so the two
tools share the same file during the transition), (b) produce
equivalent output for the stopgap's verified round-trip, and (c)
pass a migration test that runs both tools against a fixture
database and diffs output.

Specific verifications:

1. From clide's repo, `pql decisions sync` populates `pql.db`.
2. `pql decisions list --type confirmed` returns ~39 D-records (post-backfill at 2026-04-21).
3. `pql decisions show D-005 --with-refs` shows supersedence of R-002.
4. `pql decisions show D-020 --with-tickets` — empty today, populated once tickets land.
5. `pql ticket new task "probe" --decision D-005` creates `T-1`.
6. `pql ticket show T-1 --with-decision` renders ticket + D-005 body.
7. `pql ticket status T-1 in_progress` — updates DB, records in `ticket_history`.
8. `pql ticket board` — columnar kanban view with `T-1` under `in_progress`.
9. `pql decisions coverage` — flags every D-record without a ticket.
10. `pql plan status` — prints dashboard.
