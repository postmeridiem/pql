INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'bug', NULL, 'vault discovery resolves a git worktree to its main checkout', NULL, 'backlog', 'high', NULL, NULL, NULL, '2026-08-07 12:58:06.734', '2026-08-07 12:58:06.734', NULL, 'c26895a1ce047a502748748b454f47eb', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'bug', NULL, 'vault discovery resolves a git worktree to its main checkout', 'Source: feature-request.md FR-4 (settled-reach field hits 2026-07-08 and 2026-07-24).

Mechanism. `internal/config/discover.go` `walkUp()` accepts a marker only when
`os.Stat` reports a **directory** (`info.IsDir()`). In a linked git worktree `.git`
is a **file** holding a `gitdir:` pointer, so the `.git` pass skips the worktree
root and keeps ascending.

Reproduced in this repo on 2026-08-07 (probe worktrees since removed):

- Worktree at `<repo>/.worktrees/probe` — `pql doctor` reports
  `vault.path=/var/mnt/data/projects/pql`, `discovered_via=".git/ ancestor at
  /var/mnt/data/projects/pql"`, and `decisions list` returns main''s 39 records.
- Worktree outside the main checkout (`/tmp/.../wt`) — no marker matches at all;
  `discovered_via="cwd fallback"`. A second, different wrong answer.

Consequence from the field: `decisions validate` returns ok:true against main''s
unchanged markdown while the edits sit in the worktree (a false positive), and
`decisions sync` parses main''s DQR and rewrites main''s pql.db plus the tracked
`governance/README.md`, ignoring the worktree entirely. `--db <worktree>/.pql/pql.db`
alone does not rescue it; only `--vault` redirects both the parse side and the
vault-derived DB coherently.

Fix.

1. Treat a `.git` **file** as a repo marker. The vault root is the directory
   *containing* it — the worktree checkout — so the `gitdir:` pointer never needs
   to be followed.
2. Marker ordering is load-bearing and is not covered by (1) alone. `walkUp` runs
   two full ascents: `.obsidian` first, then `.git`. For a worktree nested under a
   vault whose root has `.obsidian`, the `.obsidian` pass still resolves to the main
   checkout before the `.git` pass ever runs. Replace the two passes with a single
   ascent that checks both markers at each level, first hit wins. Without this the
   fix only covers non-Obsidian repos (which is why settled-reach hit it and a
   vault-shaped repo would stay broken).
3. Consider a stderr warning from the mutating verbs (`decisions sync`, `plan
   rebuild`/`import`) when the resolved vault root is not the cwd''s own worktree
   root. FR-4''s "bonus" ask for doctor visibility is already satisfied — `pql doctor`
   leads with `vault.path` + `discovered_via`.

Acceptance: unit tests in `internal/config` for (a) a `.git` file marker, (b) a
worktree nested under an `.obsidian` vault root, (c) a worktree outside the main
checkout; an integration test running `decisions validate` from a worktree and
asserting it sees the worktree''s markdown.', 'backlog', 'high', NULL, NULL, NULL, '2026-08-07 12:58:06.734', '2026-08-07 12:58:10.361', NULL, '83078f6d4f645cb4ac6a978625c0c51f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'bug', NULL, 'changelog replay hash-tiebreaks equal-timestamp legacy rows', NULL, 'backlog', 'high', NULL, NULL, 'D-16', '2026-08-07 12:58:23.685', '2026-08-07 12:58:23.685', NULL, '45b6b4119137627ca3e79c0936dfec27', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'bug', NULL, 'changelog replay hash-tiebreaks equal-timestamp legacy rows', 'Source: feature-request.md FR-3 (settled-reach T-1057, authored 2026-06-12, loss
discovered 2026-07-08).

Half of this already shipped. 1.10.4 (commit 8d5a53b) moved write timestamps to
millisecond precision (`strftime(''%Y-%m-%d %H:%M:%f'',''now'')`), so newly written
same-second mutations no longer tie on `updated_at`.

What remains. Rows written *before* that release keep second granularity forever,
and the LWW guard in `internal/planning/changelog/exporter.go` still breaks an exact
`updated_at` tie on `excluded.hash > <table>.hash` — an ordering that is arbitrary
with respect to causality. Every replay (`plan rebuild`, fresh clone,
post-checkout/post-rewrite hooks) can therefore still revert a legacy row to an
earlier state. Measured in this repo on 2026-08-07: 113 second-granularity
timestamps vs 9 millisecond ones under `.pql/changelog/tickets/`. The settled-reach
T-1057 description is still lost on every rebuild today for exactly this reason —
the write-side fix does not reach back.

Fix (FR-3''s proposal). Tie-break equal `updated_at` by **changelog position**
rather than content hash: append order within a month file is already a total order
and it matches causality, so it repairs historical rows without rewriting them.
Alternative: a monotonic per-mutation sequence column on changelog rows — a schema
change plus a `canonical_version` bump, to be weighed against D-19''s
no-migration-runner stance. Keep the hash comparison only as the final
deterministic fallback.

Acceptance: replaying a month file whose two rows share `updated_at` yields the
later-appended row regardless of hash order; regression test in the T-1057 shape
(create placeholder, then immediate `ticket append`, same second); D-16 updated in
the same change if the guard''s documented rule moves (D-record and implementation
land together).', 'backlog', 'high', NULL, NULL, 'D-16', '2026-08-07 12:58:23.685', '2026-08-07 12:58:29.783', NULL, 'b0049817f50b7b7c9c5d02676af14756', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'task', NULL, 'add plan rebuild --verify to surface silent replay divergence', NULL, 'backlog', 'medium', NULL, NULL, 'D-16', '2026-08-07 12:58:39.239', '2026-08-07 12:58:39.239', NULL, 'd5624939522760a145f7ea725050a782', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'task', NULL, 'add plan rebuild --verify to surface silent replay divergence', 'Source: feature-request.md FR-3, third bullet of the proposal.

FR-3 went undetected for a month because a rebuild silently changed a row''s
contents — the live DB served a placeholder description while the real text
survived only as a changelog row.

Add `pql plan rebuild --verify`: snapshot row counts and per-row hashes before the
drop-and-replay, compare after, and report rows whose content changed or
disappeared. Emit a specific stderr diagnostic for the dangerous shape — an equal
`updated_at` tie resolved against the more recently appended row.

Independent of the ordering fix: that one prevents the class, `--verify` makes any
future replay divergence visible instead of silent. Follows the output contract —
result on stdout as JSON, diagnostics as JSON-per-line on stderr; a divergence is a
warning, not a non-zero exit, unless rows were actually lost.', 'backlog', 'medium', NULL, NULL, 'D-16', '2026-08-07 12:58:39.239', '2026-08-07 12:58:43.841', NULL, '853d6f79efb515ce875329091ac7019c', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'task', NULL, 'pql ticket decision <id> <D-NNN|none> to link an existing ticket to a decision', NULL, 'backlog', 'medium', NULL, NULL, 'D-20', '2026-08-07 12:58:48.653', '2026-08-07 12:58:48.653', NULL, 'c9e98783c50bb325847fb6f5c07221f7', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'task', NULL, 'pql ticket decision <id> <D-NNN|none> to link an existing ticket to a decision', 'Source: feature-request.md FR-5 (settled-reach, 2026-07-27). A delegated agent
created a 19-ticket tree implementing a freshly written D-record and omitted
`--decision` on every `ticket new`. Discovered on review, unrepairable.

Today `decision_ref` is create-time-or-never. `internal/cli/ticket.go` wires
`--decision` only onto `ticket new`; `repo.UpdateTicketFields`
(`internal/planning/repo/tickets.go:1049`) covers title/description/priority/type
only, and `refine write` decodes with `DisallowUnknownFields`, so a
`{"decision":"D-1"}` patch is rejected outright.

Every other structural attribute is repairable after the fact — `setparent` for
hierarchy, `block`/`unblock` for dependencies, `assign`, `team`, `label`, `status`,
even `relabel` for a colliding id. Decision linkage is the only one where a
delegation mistake is permanent, and it degrades D-20''s implementation-status view
(`decisions show <id> --with-tickets`) silently: that view is only as complete as
the links happen to be.

Ask — a dedicated subcommand matching the setparent/team/assign precedent:

    pql ticket decision T-1211 D-258        # set/replace
    pql ticket decision T-1211 none         # clear, mirroring `setparent … none`

Accept comma-batched ids (`T-1,T-2,T-3`) the way `setparent` does; the motivating
case is 19 tickets at once, and a repair verb that only takes one id at a time
invites the same delegation failure it exists to fix.

Scope: repo method, a `ticket_history` row per change, changelog write-through,
integration test, the embedded skill''s ticket-subcommand table
(`internal/skill/SKILL.md`), and CHANGELOG. Decide explicitly whether to validate
that the D-record exists: `ticket new` does not today (it stores the ref
unchecked), so warn-but-write matches existing behaviour while a hard reject is a
behaviour change worth stating in the record. Adding `decision` to `refine write`''s
writable set would serve equally well; the dedicated subcommand just matches the
established precedent.', 'backlog', 'medium', NULL, NULL, 'D-20', '2026-08-07 12:58:48.653', '2026-08-07 12:59:54.541', NULL, 'a499405b1d55eff42ff064b404a74654', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'epic', NULL, 'rank the planning dataset against document-length text', NULL, 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:00:45.089', '2026-08-07 13:00:45.089', NULL, '6f67051e5e3487c3f4d9896dbd342a1c', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'epic', NULL, 'rank the planning dataset against document-length text', 'Source: feature-request.md FR-2. Motivating failure: answering "how much of
`fable-ous.md` (25 KB, ~40 findings) is already captured as tickets?" required
hand-fanning three agents across ~425 tickets. Plain grep missed semantically
equivalent phrasings ("PTY leaks its master fd" vs. a ticket titled "fd not closed
on natural exit"). No surface ranks tickets by overlap with a body of text.

Measured gap (2026-06-17, clide vault):

- Corpus gap — every `pql search` result is a vault `.md` path. `search` reads
  `index.db` (414 KB of vault notes); tickets live in `pql.db` (2.6 MB), which
  `search` never opens. The ticket-coverage question is invisible to it.
- A specific finding query returned `[]`.
- No lexical signal participates in rank: `pql search "terminal"` scored its top
  hits almost entirely on recency (weight 0.25), with `link_overlap`, `tag_overlap`,
  `path_proximity` and `centrality` all 0.

So the blockers are corpus plus signal wiring, not embeddings — Path A is viable
precisely because the pipeline, signal framework, and eval harness already exist and
are simply not pointed at the planning dataset.

Children: the design record first (Path A''s shape, plus Path B as an open
question), then the corpus/signal implementation, an eval set built from an existing
labelled mapping, and the doc reframe of the "Why not vectors" section. Path B (a
vector/embedding index) stays deferred and question-shaped per
`docs/structure/design-philosophy.md` — if it is ever taken it is a design-philosophy
amendment carrying the observed failure data, not a feature ticket.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:00:45.089', '2026-08-07 13:01:02.024', NULL, '4d7629ba7a0e7154fc24e9aef59fd259', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'author the design record for planning-dataset ranking, plus the Path B question', NULL, 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:01:33.075', '2026-08-07 13:01:33.075', NULL, '6913ead5018ab63f8c4544297bf06505', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'author the design record for planning-dataset ranking, plus the Path B question', 'FR-2 is explicitly "needs a design decision before any code". This ticket produces
the governance records; it gates the implementation ticket.

Two records:

1. A D-record for Path A. What the planning corpus looks like to
   generate → rank → bundle (tickets and decisions out of `pql.db` — a second
   corpus alongside `index.db`, which is a real architectural statement given D-3''s
   two-store split); which signals carry it (a textual/lexical-match signal is the
   one that is missing today); the document-length input shape
   (`ticket search --text-file` versus a `plan related --text-file` intent); and how
   it stays inside the consumer-agnostic core — `internal/intent/` and
   `internal/query/` must not import `internal/cli/`, and the same applies to
   whatever corpus adapter is added.
2. A Q-record for Path B (a vector/embedding index), anchored to FR-2''s evidence, so
   the "no vectors" stance is revisited only against data — exactly as the philosophy
   demands.

Mechanics: `pql decisions claim D architecture "…"` and `pql decisions claim Q
architecture "…"` for the ids, write the bodies into `governance/`, then
`pql decisions sync`.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:01:33.075', '2026-08-07 13:01:53.251', NULL, 'a1b17cfa09b5d2ecadbf1806bd9fa65f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM7MBM4CF0E05K0D3AWMXC', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'implement Path A: planning corpus, lexical signal, document-length input', NULL, 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:02:00.797', '2026-08-07 13:02:00.797', NULL, '4805db48b52fbe60bc2e637690db8049', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM7MBM4CF0E05K0D3AWMXC', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'implement Path A: planning corpus, lexical signal, document-length input', 'Implement Path A per the D-record produced by the design ticket (blocker).

Sketch, subject to that record:

- A planning-corpus generator over `pql.db` rows (tickets and decisions) alongside
  the existing `index.db` generator. Generation stays wide and cheap; it must not
  import the ranker.
- A textual-match signal in `internal/connect/signal/`, with per-intent weight
  entries. Each signal returns `Contribution{Name, Raw, Normalized, Weight}` —
  provenance is data, not an `explain.go`.
- A document-length input path (`--text-file`, `--stdin`) treated as a long query
  rather than a new mode.
- Ships behind `--flat-search` like every other intent: the global flag
  short-circuits once in `runIntent`, with no per-subcommand wiring.

Tune against the golden eval set (sibling ticket) with `make eval` before declaring
it done — NDCG@k / MRR / P@k, plus per-signal contribution diffs vs. baseline.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:02:00.797', '2026-08-07 13:02:06.186', NULL, 'e3bda9febaa4b859c385083f6581020a', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM9C4J7N05CS7291SZXKMR', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'build the FR-2 golden eval set from clide T-359 finding-to-ticket mapping', NULL, 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:02:15.077', '2026-08-07 13:02:15.077', NULL, '5595cbe1ae2f09a930b5ffb4257e3995', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM9C4J7N05CS7291SZXKMR', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'build the FR-2 golden eval set from clide T-359 finding-to-ticket mapping', 'FR-2 hands over a labelled relevance mapping for free: clide''s epic T-359 with
children T-360 through T-386 map 1:1 onto the findings in `fable-ous.md`. That is a
gold standard for `internal/connect/rank/testdata/golden/`, produced by the same
session that surfaced the gap.

Convert it into a golden file so Path A can be tuned on NDCG@k / MRR / P@k via
`make eval` — before anyone argues the eval set is insufficient. Needs a snapshot of
the clide planning data plus the source document, following the fixture pattern
already used for `testdata/council-snapshot`.

Doable in parallel with the design record: this is the measurement, not the design.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:02:15.077', '2026-08-07 13:02:20.973', NULL, 'cd3882078c3d89d5a64bf167352aa7bd', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'reframe the design-philosophy vectors section', NULL, 'backlog', 'low', NULL, NULL, NULL, '2026-08-07 13:02:34.996', '2026-08-07 13:02:34.996', NULL, '776eced556d1b8db7f82035c13a62f48', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'reframe the design-philosophy vectors section', '`docs/structure/design-philosophy.md`, section "Why not vectors", fuses two
different claims and contradicts itself: line 17 calls the absence of a vector layer
"not a gap to be filled later" (permanent), while line 19 says "if semantic
retrieval becomes necessary, it will be because a specific failure mode has been
observed repeatedly in production use" (conditional). FR-2 is the first logged
instance of exactly that trigger, so the wording now decides an argument instead of
merely setting a tone.

Split what the section fuses:

- Permanent — the discipline. Exhaust the cheap, inspectable signals before reaching
  for anything opaque. This never expires, vectors or no vectors.
- For now — the mechanism. The exclusion of a vector store, gated on the evidence
  trigger the section already names.

Suggested minimal reframe, from the FR: "…is not a gap to be filled later." becomes
"…is a deliberate constraint for now, not a permanent verdict," optionally closing
with "the discipline is permanent; the exclusion is conditional."

Maintainer-side edit by design — the consumer repo flagged it rather than editing
this repo''s philosophy doc from the outside.', 'backlog', 'low', NULL, NULL, NULL, '2026-08-07 13:02:34.996', '2026-08-07 13:02:38.892', NULL, '2b9de704a1a1d3b7043f00f600c867c6', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVWCA6PCMMPYMPD1SB8', 'task', '06FB0TGPVY4DA9DZX232RFV8RW', 'Add ''ticket new --id-only'' for tree-creation scripts (#5)', 'Verified delivered on 2026-08-07 while triaging feature-request.md, closing retroactively. Shipped in 1.7.0: flag wired in internal/cli/ticket.go, dedicated regression test TestIntegration_TicketNew_IdOnly in internal/cli/writethrough_integration_test.go plus heavy use across the integration suite, documented in the embedded skill and in docs/output-contract.md as one of the two sanctioned plain-text output exceptions, CHANGELOG entry under Added.', 'backlog', 'medium', NULL, NULL, NULL, '2026-06-03 14:41:35', '2026-08-07 13:06:14.488', NULL, 'c00397f9ebe6305192d8c87f33c7b48f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVWCA6PCMMPYMPD1SB8', 'task', '06FB0TGPVY4DA9DZX232RFV8RW', 'Add ''ticket new --id-only'' for tree-creation scripts (#5)', 'Verified delivered on 2026-08-07 while triaging feature-request.md, closing retroactively. Shipped in 1.7.0: flag wired in internal/cli/ticket.go, dedicated regression test TestIntegration_TicketNew_IdOnly in internal/cli/writethrough_integration_test.go plus heavy use across the integration suite, documented in the embedded skill and in docs/output-contract.md as one of the two sanctioned plain-text output exceptions, CHANGELOG entry under Added.', 'done', 'medium', NULL, NULL, NULL, '2026-06-03 14:41:35', '2026-08-07 13:06:20.702', NULL, '120580958a2eaf8e8a266869d7ca9373', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'bug', NULL, 'vault discovery resolves a git worktree to its main checkout', 'Source: feature-request.md FR-4 (settled-reach field hits 2026-07-08 and 2026-07-24).

Mechanism. `internal/config/discover.go` `walkUp()` accepts a marker only when
`os.Stat` reports a **directory** (`info.IsDir()`). In a linked git worktree `.git`
is a **file** holding a `gitdir:` pointer, so the `.git` pass skips the worktree
root and keeps ascending.

Reproduced in this repo on 2026-08-07 (probe worktrees since removed):

- Worktree at `<repo>/.worktrees/probe` — `pql doctor` reports
  `vault.path=/var/mnt/data/projects/pql`, `discovered_via=".git/ ancestor at
  /var/mnt/data/projects/pql"`, and `decisions list` returns main''s 39 records.
- Worktree outside the main checkout (`/tmp/.../wt`) — no marker matches at all;
  `discovered_via="cwd fallback"`. A second, different wrong answer.

Consequence from the field: `decisions validate` returns ok:true against main''s
unchanged markdown while the edits sit in the worktree (a false positive), and
`decisions sync` parses main''s DQR and rewrites main''s pql.db plus the tracked
`governance/README.md`, ignoring the worktree entirely. `--db <worktree>/.pql/pql.db`
alone does not rescue it; only `--vault` redirects both the parse side and the
vault-derived DB coherently.

Fix.

1. Treat a `.git` **file** as a repo marker. The vault root is the directory
   *containing* it — the worktree checkout — so the `gitdir:` pointer never needs
   to be followed.
2. Marker ordering is load-bearing and is not covered by (1) alone. `walkUp` runs
   two full ascents: `.obsidian` first, then `.git`. For a worktree nested under a
   vault whose root has `.obsidian`, the `.obsidian` pass still resolves to the main
   checkout before the `.git` pass ever runs. Replace the two passes with a single
   ascent that checks both markers at each level, first hit wins. Without this the
   fix only covers non-Obsidian repos (which is why settled-reach hit it and a
   vault-shaped repo would stay broken).
3. Consider a stderr warning from the mutating verbs (`decisions sync`, `plan
   rebuild`/`import`) when the resolved vault root is not the cwd''s own worktree
   root. FR-4''s "bonus" ask for doctor visibility is already satisfied — `pql doctor`
   leads with `vault.path` + `discovered_via`.

Acceptance: unit tests in `internal/config` for (a) a `.git` file marker, (b) a
worktree nested under an `.obsidian` vault root, (c) a worktree outside the main
checkout; an integration test running `decisions validate` from a worktree and
asserting it sees the worktree''s markdown.', 'ready', 'high', NULL, NULL, NULL, '2026-08-07 12:58:06.734', '2026-08-07 13:07:22.719', NULL, '2c7b9439502267bd9790578b05ee717b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'bug', NULL, 'changelog replay hash-tiebreaks equal-timestamp legacy rows', 'Source: feature-request.md FR-3 (settled-reach T-1057, authored 2026-06-12, loss
discovered 2026-07-08).

Half of this already shipped. 1.10.4 (commit 8d5a53b) moved write timestamps to
millisecond precision (`strftime(''%Y-%m-%d %H:%M:%f'',''now'')`), so newly written
same-second mutations no longer tie on `updated_at`.

What remains. Rows written *before* that release keep second granularity forever,
and the LWW guard in `internal/planning/changelog/exporter.go` still breaks an exact
`updated_at` tie on `excluded.hash > <table>.hash` — an ordering that is arbitrary
with respect to causality. Every replay (`plan rebuild`, fresh clone,
post-checkout/post-rewrite hooks) can therefore still revert a legacy row to an
earlier state. Measured in this repo on 2026-08-07: 113 second-granularity
timestamps vs 9 millisecond ones under `.pql/changelog/tickets/`. The settled-reach
T-1057 description is still lost on every rebuild today for exactly this reason —
the write-side fix does not reach back.

Fix (FR-3''s proposal). Tie-break equal `updated_at` by **changelog position**
rather than content hash: append order within a month file is already a total order
and it matches causality, so it repairs historical rows without rewriting them.
Alternative: a monotonic per-mutation sequence column on changelog rows — a schema
change plus a `canonical_version` bump, to be weighed against D-19''s
no-migration-runner stance. Keep the hash comparison only as the final
deterministic fallback.

Acceptance: replaying a month file whose two rows share `updated_at` yields the
later-appended row regardless of hash order; regression test in the T-1057 shape
(create placeholder, then immediate `ticket append`, same second); D-16 updated in
the same change if the guard''s documented rule moves (D-record and implementation
land together).', 'ready', 'high', NULL, NULL, 'D-16', '2026-08-07 12:58:23.685', '2026-08-07 13:07:27.852', NULL, '25d6d521267a0c630a3c2b105528b35b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'task', NULL, 'add plan rebuild --verify to surface silent replay divergence', 'Source: feature-request.md FR-3, third bullet of the proposal.

FR-3 went undetected for a month because a rebuild silently changed a row''s
contents — the live DB served a placeholder description while the real text
survived only as a changelog row.

Add `pql plan rebuild --verify`: snapshot row counts and per-row hashes before the
drop-and-replay, compare after, and report rows whose content changed or
disappeared. Emit a specific stderr diagnostic for the dangerous shape — an equal
`updated_at` tie resolved against the more recently appended row.

Independent of the ordering fix: that one prevents the class, `--verify` makes any
future replay divergence visible instead of silent. Follows the output contract —
result on stdout as JSON, diagnostics as JSON-per-line on stderr; a divergence is a
warning, not a non-zero exit, unless rows were actually lost.', 'ready', 'medium', NULL, NULL, 'D-16', '2026-08-07 12:58:39.239', '2026-08-07 13:07:30.300', NULL, '31c723922a8c8f43fbbec0a5e5fd5ddc', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'task', NULL, 'pql ticket decision <id> <D-NNN|none> to link an existing ticket to a decision', 'Source: feature-request.md FR-5 (settled-reach, 2026-07-27). A delegated agent
created a 19-ticket tree implementing a freshly written D-record and omitted
`--decision` on every `ticket new`. Discovered on review, unrepairable.

Today `decision_ref` is create-time-or-never. `internal/cli/ticket.go` wires
`--decision` only onto `ticket new`; `repo.UpdateTicketFields`
(`internal/planning/repo/tickets.go:1049`) covers title/description/priority/type
only, and `refine write` decodes with `DisallowUnknownFields`, so a
`{"decision":"D-1"}` patch is rejected outright.

Every other structural attribute is repairable after the fact — `setparent` for
hierarchy, `block`/`unblock` for dependencies, `assign`, `team`, `label`, `status`,
even `relabel` for a colliding id. Decision linkage is the only one where a
delegation mistake is permanent, and it degrades D-20''s implementation-status view
(`decisions show <id> --with-tickets`) silently: that view is only as complete as
the links happen to be.

Ask — a dedicated subcommand matching the setparent/team/assign precedent:

    pql ticket decision T-1211 D-258        # set/replace
    pql ticket decision T-1211 none         # clear, mirroring `setparent … none`

Accept comma-batched ids (`T-1,T-2,T-3`) the way `setparent` does; the motivating
case is 19 tickets at once, and a repair verb that only takes one id at a time
invites the same delegation failure it exists to fix.

Scope: repo method, a `ticket_history` row per change, changelog write-through,
integration test, the embedded skill''s ticket-subcommand table
(`internal/skill/SKILL.md`), and CHANGELOG. Decide explicitly whether to validate
that the D-record exists: `ticket new` does not today (it stores the ref
unchecked), so warn-but-write matches existing behaviour while a hard reject is a
behaviour change worth stating in the record. Adding `decision` to `refine write`''s
writable set would serve equally well; the dedicated subcommand just matches the
established precedent.', 'ready', 'medium', NULL, NULL, 'D-20', '2026-08-07 12:58:48.653', '2026-08-07 13:07:32.702', NULL, '47dedbc3f244e79ed3db3fa3fef4f1ff', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'epic', NULL, 'rank the planning dataset against document-length text', 'Source: feature-request.md FR-2. Motivating failure: answering "how much of
`fable-ous.md` (25 KB, ~40 findings) is already captured as tickets?" required
hand-fanning three agents across ~425 tickets. Plain grep missed semantically
equivalent phrasings ("PTY leaks its master fd" vs. a ticket titled "fd not closed
on natural exit"). No surface ranks tickets by overlap with a body of text.

Measured gap (2026-06-17, clide vault):

- Corpus gap — every `pql search` result is a vault `.md` path. `search` reads
  `index.db` (414 KB of vault notes); tickets live in `pql.db` (2.6 MB), which
  `search` never opens. The ticket-coverage question is invisible to it.
- A specific finding query returned `[]`.
- No lexical signal participates in rank: `pql search "terminal"` scored its top
  hits almost entirely on recency (weight 0.25), with `link_overlap`, `tag_overlap`,
  `path_proximity` and `centrality` all 0.

So the blockers are corpus plus signal wiring, not embeddings — Path A is viable
precisely because the pipeline, signal framework, and eval harness already exist and
are simply not pointed at the planning dataset.

Children: the design record first (Path A''s shape, plus Path B as an open
question), then the corpus/signal implementation, an eval set built from an existing
labelled mapping, and the doc reframe of the "Why not vectors" section. Path B (a
vector/embedding index) stays deferred and question-shaped per
`docs/structure/design-philosophy.md` — if it is ever taken it is a design-philosophy
amendment carrying the observed failure data, not a feature ticket.', 'ready', 'medium', NULL, NULL, NULL, '2026-08-07 13:00:45.089', '2026-08-07 13:07:35.306', NULL, '8d5895b711465ea643f291199b9bc00f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'author the design record for planning-dataset ranking, plus the Path B question', 'FR-2 is explicitly "needs a design decision before any code". This ticket produces
the governance records; it gates the implementation ticket.

Two records:

1. A D-record for Path A. What the planning corpus looks like to
   generate → rank → bundle (tickets and decisions out of `pql.db` — a second
   corpus alongside `index.db`, which is a real architectural statement given D-3''s
   two-store split); which signals carry it (a textual/lexical-match signal is the
   one that is missing today); the document-length input shape
   (`ticket search --text-file` versus a `plan related --text-file` intent); and how
   it stays inside the consumer-agnostic core — `internal/intent/` and
   `internal/query/` must not import `internal/cli/`, and the same applies to
   whatever corpus adapter is added.
2. A Q-record for Path B (a vector/embedding index), anchored to FR-2''s evidence, so
   the "no vectors" stance is revisited only against data — exactly as the philosophy
   demands.

Mechanics: `pql decisions claim D architecture "…"` and `pql decisions claim Q
architecture "…"` for the ids, write the bodies into `governance/`, then
`pql decisions sync`.', 'ready', 'medium', NULL, NULL, NULL, '2026-08-07 13:01:33.075', '2026-08-07 13:07:38.236', NULL, 'ada2e0ba9012a58336460eeb5da75c2c', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'reframe the design-philosophy vectors section', '`docs/structure/design-philosophy.md`, section "Why not vectors", fuses two
different claims and contradicts itself: line 17 calls the absence of a vector layer
"not a gap to be filled later" (permanent), while line 19 says "if semantic
retrieval becomes necessary, it will be because a specific failure mode has been
observed repeatedly in production use" (conditional). FR-2 is the first logged
instance of exactly that trigger, so the wording now decides an argument instead of
merely setting a tone.

Split what the section fuses:

- Permanent — the discipline. Exhaust the cheap, inspectable signals before reaching
  for anything opaque. This never expires, vectors or no vectors.
- For now — the mechanism. The exclusion of a vector store, gated on the evidence
  trigger the section already names.

Suggested minimal reframe, from the FR: "…is not a gap to be filled later." becomes
"…is a deliberate constraint for now, not a permanent verdict," optionally closing
with "the discipline is permanent; the exclusion is conditional."

Maintainer-side edit by design — the consumer repo flagged it rather than editing
this repo''s philosophy doc from the outside.', 'ready', 'low', NULL, NULL, NULL, '2026-08-07 13:02:34.996', '2026-08-07 13:07:41.236', NULL, '9355964c35bea715c47fe49fa39b34db', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'bug', NULL, 'vault discovery resolves a git worktree to its main checkout', 'Source: feature-request.md FR-4 (settled-reach field hits 2026-07-08 and 2026-07-24).

Mechanism. `internal/config/discover.go` `walkUp()` accepts a marker only when
`os.Stat` reports a **directory** (`info.IsDir()`). In a linked git worktree `.git`
is a **file** holding a `gitdir:` pointer, so the `.git` pass skips the worktree
root and keeps ascending.

Reproduced in this repo on 2026-08-07 (probe worktrees since removed):

- Worktree at `<repo>/.worktrees/probe` — `pql doctor` reports
  `vault.path=/var/mnt/data/projects/pql`, `discovered_via=".git/ ancestor at
  /var/mnt/data/projects/pql"`, and `decisions list` returns main''s 39 records.
- Worktree outside the main checkout (`/tmp/.../wt`) — no marker matches at all;
  `discovered_via="cwd fallback"`. A second, different wrong answer.

Consequence from the field: `decisions validate` returns ok:true against main''s
unchanged markdown while the edits sit in the worktree (a false positive), and
`decisions sync` parses main''s DQR and rewrites main''s pql.db plus the tracked
`governance/README.md`, ignoring the worktree entirely. `--db <worktree>/.pql/pql.db`
alone does not rescue it; only `--vault` redirects both the parse side and the
vault-derived DB coherently.

Fix.

1. Treat a `.git` **file** as a repo marker. The vault root is the directory
   *containing* it — the worktree checkout — so the `gitdir:` pointer never needs
   to be followed.
2. Marker ordering is load-bearing and is not covered by (1) alone. `walkUp` runs
   two full ascents: `.obsidian` first, then `.git`. For a worktree nested under a
   vault whose root has `.obsidian`, the `.obsidian` pass still resolves to the main
   checkout before the `.git` pass ever runs. Replace the two passes with a single
   ascent that checks both markers at each level, first hit wins. Without this the
   fix only covers non-Obsidian repos (which is why settled-reach hit it and a
   vault-shaped repo would stay broken).
3. Consider a stderr warning from the mutating verbs (`decisions sync`, `plan
   rebuild`/`import`) when the resolved vault root is not the cwd''s own worktree
   root. FR-4''s "bonus" ask for doctor visibility is already satisfied — `pql doctor`
   leads with `vault.path` + `discovered_via`.

Acceptance: unit tests in `internal/config` for (a) a `.git` file marker, (b) a
worktree nested under an `.obsidian` vault root, (c) a worktree outside the main
checkout; an integration test running `decisions validate` from a worktree and
asserting it sees the worktree''s markdown.', 'in_progress', 'high', NULL, NULL, NULL, '2026-08-07 12:58:06.734', '2026-08-07 13:07:47.387', NULL, 'd72b80031719a61d76ffb39aa2cd69f4', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FM2BKV2CFG7JHXDMQA7SYBRW', 'bug', NULL, 'integration skill tests escape the sandbox: scope auto-resolve hits the real ~/.claude/skills', 'make test-integration mutates the developer''s HOME: the skill install/status/init tests invoke the binary without overriding HOME, and resolveSkillsRoot auto-resolves to user scope whenever any bundled skill is installed at ~/.claude/skills (true on any dev machine, never in CI). Observed 2026-07-08: six tests fail locally (StatusOnMissing, InstallIsIdempotent, InstallRefusesModified, Doctor_SkillFieldReportsState, Init_WithSkillYes/No) and the run overwrote ~/.claude/skills/pql/SKILL.md with the test binary''s embedded copy. Uninstall is safe (project-scope default, no auto-resolve). Fix: set HOME (and any config env) to a temp dir in the integration run() helper so the suite is hermetic; then these tests gate locally too.

Fixed 2026-08-07. The integration suite now builds every invocation through a pqlCmd/sandboxEnv helper that redirects HOME, USERPROFILE and the XDG dirs to a per-test temp dir and strips ambient PQL_ overrides, so skill scope auto-resolve can no longer see the developer''s installed skills. One HOME per test, stable across invocations within it, so install-then-status still works. Verified: the six tests pass, the full integration suite is green on this machine for the first time, and the mtime on the real skills dir stops changing across runs.', 'backlog', 'high', NULL, NULL, NULL, '2026-07-08 10:03:13.043', '2026-08-07 13:29:29.498', NULL, 'e835f6dcbc7dd37dcd893c134214a76b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FM2BKV2CFG7JHXDMQA7SYBRW', 'bug', NULL, 'integration skill tests escape the sandbox: scope auto-resolve hits the real ~/.claude/skills', 'make test-integration mutates the developer''s HOME: the skill install/status/init tests invoke the binary without overriding HOME, and resolveSkillsRoot auto-resolves to user scope whenever any bundled skill is installed at ~/.claude/skills (true on any dev machine, never in CI). Observed 2026-07-08: six tests fail locally (StatusOnMissing, InstallIsIdempotent, InstallRefusesModified, Doctor_SkillFieldReportsState, Init_WithSkillYes/No) and the run overwrote ~/.claude/skills/pql/SKILL.md with the test binary''s embedded copy. Uninstall is safe (project-scope default, no auto-resolve). Fix: set HOME (and any config env) to a temp dir in the integration run() helper so the suite is hermetic; then these tests gate locally too.

Fixed 2026-08-07. The integration suite now builds every invocation through a pqlCmd/sandboxEnv helper that redirects HOME, USERPROFILE and the XDG dirs to a per-test temp dir and strips ambient PQL_ overrides, so skill scope auto-resolve can no longer see the developer''s installed skills. One HOME per test, stable across invocations within it, so install-then-status still works. Verified: the six tests pass, the full integration suite is green on this machine for the first time, and the mtime on the real skills dir stops changing across runs.', 'done', 'high', NULL, NULL, NULL, '2026-07-08 10:03:13.043', '2026-08-07 13:29:43.245', NULL, 'a0947da3157eadccd69c47911d334b8d', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'bug', NULL, 'vault discovery resolves a git worktree to its main checkout', 'Source: feature-request.md FR-4 (settled-reach field hits 2026-07-08 and 2026-07-24).

Mechanism. `internal/config/discover.go` `walkUp()` accepts a marker only when
`os.Stat` reports a **directory** (`info.IsDir()`). In a linked git worktree `.git`
is a **file** holding a `gitdir:` pointer, so the `.git` pass skips the worktree
root and keeps ascending.

Reproduced in this repo on 2026-08-07 (probe worktrees since removed):

- Worktree at `<repo>/.worktrees/probe` — `pql doctor` reports
  `vault.path=/var/mnt/data/projects/pql`, `discovered_via=".git/ ancestor at
  /var/mnt/data/projects/pql"`, and `decisions list` returns main''s 39 records.
- Worktree outside the main checkout (`/tmp/.../wt`) — no marker matches at all;
  `discovered_via="cwd fallback"`. A second, different wrong answer.

Consequence from the field: `decisions validate` returns ok:true against main''s
unchanged markdown while the edits sit in the worktree (a false positive), and
`decisions sync` parses main''s DQR and rewrites main''s pql.db plus the tracked
`governance/README.md`, ignoring the worktree entirely. `--db <worktree>/.pql/pql.db`
alone does not rescue it; only `--vault` redirects both the parse side and the
vault-derived DB coherently.

Fix.

1. Treat a `.git` **file** as a repo marker. The vault root is the directory
   *containing* it — the worktree checkout — so the `gitdir:` pointer never needs
   to be followed.
2. Marker ordering is load-bearing and is not covered by (1) alone. `walkUp` runs
   two full ascents: `.obsidian` first, then `.git`. For a worktree nested under a
   vault whose root has `.obsidian`, the `.obsidian` pass still resolves to the main
   checkout before the `.git` pass ever runs. Replace the two passes with a single
   ascent that checks both markers at each level, first hit wins. Without this the
   fix only covers non-Obsidian repos (which is why settled-reach hit it and a
   vault-shaped repo would stay broken).
3. Consider a stderr warning from the mutating verbs (`decisions sync`, `plan
   rebuild`/`import`) when the resolved vault root is not the cwd''s own worktree
   root. FR-4''s "bonus" ask for doctor visibility is already satisfied — `pql doctor`
   leads with `vault.path` + `discovered_via`.

Acceptance: unit tests in `internal/config` for (a) a `.git` file marker, (b) a
worktree nested under an `.obsidian` vault root, (c) a worktree outside the main
checkout; an integration test running `decisions validate` from a worktree and
asserting it sees the worktree''s markdown.', 'done', 'high', NULL, NULL, NULL, '2026-08-07 12:58:06.734', '2026-08-07 13:29:43.251', NULL, 'a94a9df10ed3fee468d0b95c3ef469d4', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'task', NULL, 'add --fields projection to ticket show and decisions show', NULL, 'backlog', 'medium', NULL, NULL, 'D-27', '2026-08-07 13:35:08.477', '2026-08-07 13:35:08.477', NULL, '95d741b673d18e664e7c17a9873f98a9', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'task', NULL, 'add --fields projection to ticket show and decisions show', 'Source: observed twice in live sessions on 2026-08-07 — two different agents
reached for `pql ticket show <id> --fields id,status,title` and got exit 64,
`unknown flag: --fields`.

D-27 scoped `--fields` / `--oneline` to the *list* verbs on purpose: the problem
it solved was list dumps blowing tool-output budgets, and `show` is the
record-level surface that returns whole records by design
(`docs/output-contract.md:71` states this). The reasoning is sound, but the
surface is not guessable: an agent that has just learned `--fields` on `list`
has no way to know it stops there, and the failure costs a round trip plus a
retry every time.

Two ways out, and the second is the one asked for:

1. Sharpen the docs — say explicitly in the embedded skill that `--fields` and
   `--oneline` are list-verb flags, and that `show` always returns whole
   records.
2. Add `--fields` to `ticket show` and `decisions show`. Cheap, removes the
   footgun rather than documenting it, and keeps one projection vocabulary
   across the planning surface. `render.Project` already exists from T-56, so
   this is wiring plus tests, not new machinery.

Do both: the flag, and a skill line that states the projection vocabulary is
uniform across list and show.

Open questions for the implementer:

- `show` has shapes `list` does not: `--with-context`, `--with-blockers`,
  `--with-children`, `--tree` all attach nested join-trees. Decide whether
  `--fields` projects only the top-level record (simplest, and probably right)
  or recurses into the attached trees. State the choice in the help text.
- Comma-batched `show T-1,T-2` returns an array of show-trees; projection has
  to apply per element.
- D-27''s record should be amended to reflect the widened surface rather than
  left describing a list-only flag.', 'backlog', 'medium', NULL, NULL, 'D-27', '2026-08-07 13:35:08.477', '2026-08-07 13:35:32.727', NULL, 'a8294094143b356130980ce55f2c3a6c', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', NULL, 'automatic changelog format upgrades on load', NULL, 'backlog', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 13:37:14.814', NULL, '00e9fc9c311208dba8de1ae69f7ac2b8', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', NULL, 'automatic changelog format upgrades on load', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.', 'backlog', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 13:39:07.883', NULL, 'f8b1fc6fcf2b5aa1986a8bbedcaf7426', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', NULL, 'automatic changelog format upgrades on load', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.', 'backlog', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 13:39:39.806', NULL, '3dcf28e7462f670ae1225a15236c4c9b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'task', NULL, 'add --fields projection to ticket show and decisions show', 'Source: observed twice in live sessions on 2026-08-07 — two different agents
reached for `pql ticket show <id> --fields id,status,title` and got exit 64,
`unknown flag: --fields`.

D-27 scoped `--fields` / `--oneline` to the *list* verbs on purpose: the problem
it solved was list dumps blowing tool-output budgets, and `show` is the
record-level surface that returns whole records by design
(`docs/output-contract.md:71` states this). The reasoning is sound, but the
surface is not guessable: an agent that has just learned `--fields` on `list`
has no way to know it stops there, and the failure costs a round trip plus a
retry every time.

Two ways out, and the second is the one asked for:

1. Sharpen the docs — say explicitly in the embedded skill that `--fields` and
   `--oneline` are list-verb flags, and that `show` always returns whole
   records.
2. Add `--fields` to `ticket show` and `decisions show`. Cheap, removes the
   footgun rather than documenting it, and keeps one projection vocabulary
   across the planning surface. `render.Project` already exists from T-56, so
   this is wiring plus tests, not new machinery.

Do both: the flag, and a skill line that states the projection vocabulary is
uniform across list and show.

Open questions for the implementer:

- `show` has shapes `list` does not: `--with-context`, `--with-blockers`,
  `--with-children`, `--tree` all attach nested join-trees. Decide whether
  `--fields` projects only the top-level record (simplest, and probably right)
  or recurses into the attached trees. State the choice in the help text.
- Comma-batched `show T-1,T-2` returns an array of show-trees; projection has
  to apply per element.
- D-27''s record should be amended to reflect the widened surface rather than
  left describing a list-only flag.', 'ready', 'medium', NULL, NULL, 'D-27', '2026-08-07 13:35:08.477', '2026-08-07 13:39:55.146', NULL, '6b37ecdaae6e3786d74654e5ef7cb95e', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', NULL, 'automatic changelog format upgrades on load', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.', 'ready', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 13:39:57.943', NULL, 'a7cc330e3669c4320d0f791fa06e1f75', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'bug', NULL, 'changelog replay hash-tiebreaks equal-timestamp legacy rows', 'Source: feature-request.md FR-3 (settled-reach T-1057, authored 2026-06-12, loss
discovered 2026-07-08).

Half of this already shipped. 1.10.4 (commit 8d5a53b) moved write timestamps to
millisecond precision (`strftime(''%Y-%m-%d %H:%M:%f'',''now'')`), so newly written
same-second mutations no longer tie on `updated_at`.

What remains. Rows written *before* that release keep second granularity forever,
and the LWW guard in `internal/planning/changelog/exporter.go` still breaks an exact
`updated_at` tie on `excluded.hash > <table>.hash` — an ordering that is arbitrary
with respect to causality. Every replay (`plan rebuild`, fresh clone,
post-checkout/post-rewrite hooks) can therefore still revert a legacy row to an
earlier state. Measured in this repo on 2026-08-07: 113 second-granularity
timestamps vs 9 millisecond ones under `.pql/changelog/tickets/`. The settled-reach
T-1057 description is still lost on every rebuild today for exactly this reason —
the write-side fix does not reach back.

Fix (FR-3''s proposal). Tie-break equal `updated_at` by **changelog position**
rather than content hash: append order within a month file is already a total order
and it matches causality, so it repairs historical rows without rewriting them.
Alternative: a monotonic per-mutation sequence column on changelog rows — a schema
change plus a `canonical_version` bump, to be weighed against D-19''s
no-migration-runner stance. Keep the hash comparison only as the final
deterministic fallback.

Acceptance: replaying a month file whose two rows share `updated_at` yields the
later-appended row regardless of hash order; regression test in the T-1057 shape
(create placeholder, then immediate `ticket append`, same second); D-16 updated in
the same change if the guard''s documented rule moves (D-record and implementation
land together).', 'done', 'high', NULL, NULL, 'D-16', '2026-08-07 12:58:23.685', '2026-08-07 13:50:51.557', NULL, 'd6c602ae3ace875106e536bb04163c79', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', NULL, 'automatic changelog format upgrades on load', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', 'ready', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 13:58:00.484', NULL, 'a69a6760953a430a4755b169a76b7b33', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', NULL, 'automatic changelog format upgrades, triggered on pull', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', 'ready', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 13:58:06.465', NULL, '92d142ed7aa663173a6d39274aa2f8dd', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'task', NULL, 'pql ticket decision <id> <D-NNN|none> to link an existing ticket to a decision', 'Source: feature-request.md FR-5 (settled-reach, 2026-07-27). A delegated agent
created a 19-ticket tree implementing a freshly written D-record and omitted
`--decision` on every `ticket new`. Discovered on review, unrepairable.

Today `decision_ref` is create-time-or-never. `internal/cli/ticket.go` wires
`--decision` only onto `ticket new`; `repo.UpdateTicketFields`
(`internal/planning/repo/tickets.go:1049`) covers title/description/priority/type
only, and `refine write` decodes with `DisallowUnknownFields`, so a
`{"decision":"D-1"}` patch is rejected outright.

Every other structural attribute is repairable after the fact — `setparent` for
hierarchy, `block`/`unblock` for dependencies, `assign`, `team`, `label`, `status`,
even `relabel` for a colliding id. Decision linkage is the only one where a
delegation mistake is permanent, and it degrades D-20''s implementation-status view
(`decisions show <id> --with-tickets`) silently: that view is only as complete as
the links happen to be.

Ask — a dedicated subcommand matching the setparent/team/assign precedent:

    pql ticket decision T-1211 D-258        # set/replace
    pql ticket decision T-1211 none         # clear, mirroring `setparent … none`

Accept comma-batched ids (`T-1,T-2,T-3`) the way `setparent` does; the motivating
case is 19 tickets at once, and a repair verb that only takes one id at a time
invites the same delegation failure it exists to fix.

Scope: repo method, a `ticket_history` row per change, changelog write-through,
integration test, the embedded skill''s ticket-subcommand table
(`internal/skill/SKILL.md`), and CHANGELOG. Decide explicitly whether to validate
that the D-record exists: `ticket new` does not today (it stores the ref
unchecked), so warn-but-write matches existing behaviour while a hard reject is a
behaviour change worth stating in the record. Adding `decision` to `refine write`''s
writable set would serve equally well; the dedicated subcommand just matches the
established precedent.', 'done', 'medium', NULL, NULL, 'D-20', '2026-08-07 12:58:48.653', '2026-08-07 14:05:40.010', NULL, 'c60cf184c875677b825ee544b11752b2', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'epic', NULL, 'forward migration machinery across the version axes', NULL, 'backlog', 'high', NULL, NULL, 'D-19', '2026-08-07 14:34:16.212', '2026-08-07 14:34:16.212', NULL, '8b7195c515b2165fbcc08e53e9849703', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVXWVVHST6374CJDB8R', 'task', NULL, 'implement pql.db forward migrations (gate: third-party distribution)', 'Per the 2026-05-31 amendment to D-19: the ''no migration runner ever'' stance holds only while pql is single-author / pre-distribution — pql.db is a regenerable cache, so rm + rebuild from the changelog is acceptable recovery. Before distributing to third parties or onboarding users beyond the author, add an in-place forward-migration path: external users can''t be told to rm pql.db on every schema change, and their pql.db may hold state not yet in any shared changelog. Scope: revive the in-house runner D-6 sketched (a schema_migrations table + forward-only steps in internal/planning/schema.go), or equivalent. Gate: external distribution / multi-user adoption. Until then, schema-create-if-missing + verify + regenerate-from-changelog stands.', 'ready', 'medium', NULL, NULL, 'D-19', '2026-05-31 12:25:23', '2026-08-07 14:34:31.493', NULL, '7add80eb38e86794baa01d65a4d832d9', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'epic', NULL, 'forward migration machinery across the version axes', NULL, 'ready', 'high', NULL, NULL, 'D-19', '2026-08-07 14:34:16.212', '2026-08-07 14:34:34.041', NULL, 'da11ec79fab4ad7a6a9769b9f808382d', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'epic', NULL, 'forward migration machinery across the version axes', 'Raised 2026-08-07: T-44 and T-68 were filed months apart against different
symptoms and turn out to be the same machine pointed at two axes.

- **T-44** — forward migrations for the **pql.db schema** axis. Gated by D-19 on
  third-party distribution: an external user cannot be told to `rm pql.db` on
  every schema change, and their database may hold state no shared changelog has.
- **T-68** — forward migrations for the **changelog file format** axis. Forced
  early by T-59: the LWW guard is literal SQL inside already-committed changelog
  lines, so the fix does not reach existing repos until those files are rewritten.

What they share, and what should therefore be built once:

- An ordered list of forward-only steps, each with an id, a detector and a
  transformation — a new format or schema change becomes a new entry, not a new
  mechanism.
- A declared version per axis to compare a repo''s state against what the binary
  emits, with all declared versions living in `project.yaml` beside
  `schema_version` and surfaced through `pql version --build-info`.
- A trigger policy: on pull via the post-merge hook, plus the paths where an
  incompatible artefact would otherwise fail (`plan import`, `plan rebuild`), plus
  an explicit verb for the manual case.
- A loud diagnostic naming what was found and what was expected whenever a
  mismatch is detected and no upgrade has run. Never a silent proceed.
- A mapping of app version to each axis version, kept somewhere durable so an
  upgrade spanning several releases can be reasoned about after the fact.

Sequencing: T-68 goes first because it is forced now, and it establishes the
runner. T-44 then applies the same runner to the pql.db axis when its
distribution gate is actually reached — which keeps faith with D-19''s "no
migration framework before then" while not building the thing twice. The
counter-argument is worth stating: D-19 resists a migration runner precisely
because pql.db is regenerable, and the changelog axis is not the same kind of
artefact. If the two axes turn out to need genuinely different guarantees, split
them back apart deliberately rather than by drift.

Both axes want a decision record. The existing D-19 covers the pql.db stance;
whether this epic amends it or adds a companion record is the first thing to
settle.', 'ready', 'high', NULL, NULL, 'D-19', '2026-08-07 14:34:16.212', '2026-08-07 14:36:09.505', NULL, '3feb32bc3956b378e718f2bc73d2a59b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVXWVVHST6374CJDB8R', 'task', '06FXS9BB2GWPY0TWKS9X5V6WNW', 'implement pql.db forward migrations (gate: third-party distribution)', 'Per the 2026-05-31 amendment to D-19: the ''no migration runner ever'' stance holds only while pql is single-author / pre-distribution — pql.db is a regenerable cache, so rm + rebuild from the changelog is acceptable recovery. Before distributing to third parties or onboarding users beyond the author, add an in-place forward-migration path: external users can''t be told to rm pql.db on every schema change, and their pql.db may hold state not yet in any shared changelog. Scope: revive the in-house runner D-6 sketched (a schema_migrations table + forward-only steps in internal/planning/schema.go), or equivalent. Gate: external distribution / multi-user adoption. Until then, schema-create-if-missing + verify + regenerate-from-changelog stands.', 'ready', 'medium', NULL, NULL, 'D-19', '2026-05-31 12:25:23', '2026-08-07 14:36:17.809', NULL, '1826391001552ea959efad65f0136281', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', '06FXS9BB2GWPY0TWKS9X5V6WNW', 'automatic changelog format upgrades, triggered on pull', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', 'ready', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 14:36:17.816', NULL, '990612e6aa95391c20242a5357e4ca78', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'task', NULL, 'add plan rebuild --verify to surface silent replay divergence', 'Source: feature-request.md FR-3, third bullet of the proposal.

FR-3 went undetected for a month because a rebuild silently changed a row''s
contents — the live DB served a placeholder description while the real text
survived only as a changelog row.

Add `pql plan rebuild --verify`: snapshot row counts and per-row hashes before the
drop-and-replay, compare after, and report rows whose content changed or
disappeared. Emit a specific stderr diagnostic for the dangerous shape — an equal
`updated_at` tie resolved against the more recently appended row.

Independent of the ordering fix: that one prevents the class, `--verify` makes any
future replay divergence visible instead of silent. Follows the output contract —
result on stdout as JSON, diagnostics as JSON-per-line on stderr; a divergence is a
warning, not a non-zero exit, unless rows were actually lost.', 'done', 'medium', NULL, NULL, 'D-16', '2026-08-07 12:58:39.239', '2026-08-07 14:36:57.431', NULL, '86910a1cb35c560b3ff9de6db9f16490', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'bug', NULL, 'vault discovery resolves a git worktree to its main checkout', 'Source: feature-request.md FR-4 (settled-reach field hits 2026-07-08 and 2026-07-24).

Mechanism. `internal/config/discover.go` `walkUp()` accepts a marker only when
`os.Stat` reports a **directory** (`info.IsDir()`). In a linked git worktree `.git`
is a **file** holding a `gitdir:` pointer, so the `.git` pass skips the worktree
root and keeps ascending.

Reproduced in this repo on 2026-08-07 (probe worktrees since removed):

- Worktree at `<repo>/.worktrees/probe` — `pql doctor` reports
  `vault.path=/var/mnt/data/projects/pql`, `discovered_via=".git/ ancestor at
  /var/mnt/data/projects/pql"`, and `decisions list` returns main''s 39 records.
- Worktree outside the main checkout (`/tmp/.../wt`) — no marker matches at all;
  `discovered_via="cwd fallback"`. A second, different wrong answer.

Consequence from the field: `decisions validate` returns ok:true against main''s
unchanged markdown while the edits sit in the worktree (a false positive), and
`decisions sync` parses main''s DQR and rewrites main''s pql.db plus the tracked
`governance/README.md`, ignoring the worktree entirely. `--db <worktree>/.pql/pql.db`
alone does not rescue it; only `--vault` redirects both the parse side and the
vault-derived DB coherently.

Fix.

1. Treat a `.git` **file** as a repo marker. The vault root is the directory
   *containing* it — the worktree checkout — so the `gitdir:` pointer never needs
   to be followed.
2. Marker ordering is load-bearing and is not covered by (1) alone. `walkUp` runs
   two full ascents: `.obsidian` first, then `.git`. For a worktree nested under a
   vault whose root has `.obsidian`, the `.obsidian` pass still resolves to the main
   checkout before the `.git` pass ever runs. Replace the two passes with a single
   ascent that checks both markers at each level, first hit wins. Without this the
   fix only covers non-Obsidian repos (which is why settled-reach hit it and a
   vault-shaped repo would stay broken).
3. Consider a stderr warning from the mutating verbs (`decisions sync`, `plan
   rebuild`/`import`) when the resolved vault root is not the cwd''s own worktree
   root. FR-4''s "bonus" ask for doctor visibility is already satisfied — `pql doctor`
   leads with `vault.path` + `discovered_via`.

Acceptance: unit tests in `internal/config` for (a) a `.git` file marker, (b) a
worktree nested under an `.obsidian` vault root, (c) a worktree outside the main
checkout; an integration test running `decisions validate` from a worktree and
asserting it sees the worktree''s markdown.', 'done', 'high', NULL, NULL, 'D-3', '2026-08-07 12:58:06.734', '2026-08-07 14:37:46.943', NULL, 'e90f38f1f95c00767c17879547c3d8dd', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', '06FXS9BB2GWPY0TWKS9X5V6WNW', 'automatic changelog format upgrades, triggered on pull', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', 'in_progress', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 14:38:52.363', NULL, '94bfed080fe88ec7dbfaada8ca7ee75d', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', '06FXS9BB2GWPY0TWKS9X5V6WNW', 'automatic changelog format upgrades, triggered on pull', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', 'in_progress', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-07 14:39:39.713', NULL, 'c7819a4e3634697a38eb9b80f36e75d2', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'task', '06FXS9BB2GWPY0TWKS9X5V6WNW', 'automatic changelog format upgrades, triggered on pull', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
literal SQL text baked into every `INSERT ... ON CONFLICT` line already committed
under `.pql/changelog/`. Changing the exporter therefore only fixes rows written
from now on; every historical row keeps replaying under the old, causally wrong
tie-break. The consumers (clide, settled-reach, council) must not have to run a
manual repair, so the upgrade has to happen automatically when pql loads a
changelog it recognises as an older format.

Design (user''s, and the right one): **do the transformation in SQL, not with
regexes over the file text.** We already have a SQLite database at hand; use it.

1. **Detect.** Carry an explicit changelog format version — a marker in
   `.pql/changelog/` (alongside `0000-schema.sql`, which already uses key markers
   the importer reads). Absent marker = format 1. Compare against the version this
   binary emits; equal means no work, and the check must be cheap enough to run on
   every load.
2. **Stage.** Replay each table''s files into a staging table mirroring the
   canonical columns plus `seq INTEGER PRIMARY KEY AUTOINCREMENT` and the source
   file, so arrival order — the causal order the fix depends on — is captured as
   data. The staging load must be append-only: the ON CONFLICT tail has to be
   dropped before execution, otherwise the *old* guard resolves the rows during
   staging and bakes the very loss we are repairing into the output. Dropping a
   trailing clause is a structural edit, not a semantic rewrite of row contents.
3. **Transform in SQL.** Rename, restructure, re-resolve — whatever the format
   step needs — as ordinary statements against the staging table.
4. **Re-emit.** Write the files back from the staging rows, in `seq` order,
   through the current statement renderer, so the output carries the new guard.
   Same rows, same order, corrected conflict clause: the log is preserved, not
   collapsed to current state. Collapsing would discard intermediate mutations and
   make every clone''s files diverge textually.
5. **Stamp** the new format version and make the whole thing idempotent — a second
   run must be a no-op.

Structure it as a module with an ordered list of format steps, each with an id, a
detector and a transformation, so the next changelog format change is a new entry
rather than a new mechanism. This is the changelog-format axis, distinct from
pql.db schema migrations (D-19 / T-44) — worth saying out loud in the record so
the two do not get conflated, though they should probably share a vocabulary.

Risks to handle explicitly:

- The files are git-tracked. A rewrite produces a real diff in the consumer''s
  working tree; it must land in a commit, and the staging must be atomic enough
  that an interrupted upgrade cannot leave a half-rewritten changelog.
- Two clones upgrading independently must produce byte-identical output, or the
  next merge is a conflict storm. Determinism of the renderer is the requirement.
- Needs a decision record: automatic rewriting of tracked files on load is a
  behaviour worth stating deliberately, including whether it is opt-out.

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', 'done', 'high', NULL, NULL, 'D-15', '2026-08-07 13:37:14.814', '2026-08-08 08:07:22.134', NULL, '5946e664e2db30ce9475dda9749cdf8f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVXWVVHST6374CJDB8R', 'task', '06FXS9BB2GWPY0TWKS9X5V6WNW', 'implement pql.db forward migrations (gate: third-party distribution)', 'Per the 2026-05-31 amendment to D-19: the ''no migration runner ever'' stance holds only while pql is single-author / pre-distribution — pql.db is a regenerable cache, so rm + rebuild from the changelog is acceptable recovery. Before distributing to third parties or onboarding users beyond the author, add an in-place forward-migration path: external users can''t be told to rm pql.db on every schema change, and their pql.db may hold state not yet in any shared changelog. Scope: revive the in-house runner D-6 sketched (a schema_migrations table + forward-only steps in internal/planning/schema.go), or equivalent. Gate: external distribution / multi-user adoption. Until then, schema-create-if-missing + verify + regenerate-from-changelog stands.', 'done', 'medium', NULL, NULL, 'D-19', '2026-05-31 12:25:23', '2026-08-08 08:07:22.142', NULL, '3880837781af1cc7b00fbe4db9593c9b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'epic', NULL, 'forward migration machinery across the version axes', 'Raised 2026-08-07: T-44 and T-68 were filed months apart against different
symptoms and turn out to be the same machine pointed at two axes.

- **T-44** — forward migrations for the **pql.db schema** axis. Gated by D-19 on
  third-party distribution: an external user cannot be told to `rm pql.db` on
  every schema change, and their database may hold state no shared changelog has.
- **T-68** — forward migrations for the **changelog file format** axis. Forced
  early by T-59: the LWW guard is literal SQL inside already-committed changelog
  lines, so the fix does not reach existing repos until those files are rewritten.

What they share, and what should therefore be built once:

- An ordered list of forward-only steps, each with an id, a detector and a
  transformation — a new format or schema change becomes a new entry, not a new
  mechanism.
- A declared version per axis to compare a repo''s state against what the binary
  emits, with all declared versions living in `project.yaml` beside
  `schema_version` and surfaced through `pql version --build-info`.
- A trigger policy: on pull via the post-merge hook, plus the paths where an
  incompatible artefact would otherwise fail (`plan import`, `plan rebuild`), plus
  an explicit verb for the manual case.
- A loud diagnostic naming what was found and what was expected whenever a
  mismatch is detected and no upgrade has run. Never a silent proceed.
- A mapping of app version to each axis version, kept somewhere durable so an
  upgrade spanning several releases can be reasoned about after the fact.

Sequencing: T-68 goes first because it is forced now, and it establishes the
runner. T-44 then applies the same runner to the pql.db axis when its
distribution gate is actually reached — which keeps faith with D-19''s "no
migration framework before then" while not building the thing twice. The
counter-argument is worth stating: D-19 resists a migration runner precisely
because pql.db is regenerable, and the changelog axis is not the same kind of
artefact. If the two axes turn out to need genuinely different guarantees, split
them back apart deliberately rather than by drift.

Both axes want a decision record. The existing D-19 covers the pql.db stance;
whether this epic amends it or adds a companion record is the first thing to
settle.', 'done', 'high', NULL, NULL, 'D-19', '2026-08-07 14:34:16.212', '2026-08-08 08:07:53.503', NULL, '5e6f1ad1cfb5a655f0c5decb47f98e6e', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'epic', NULL, 'rank the planning dataset against document-length text', 'Source: feature-request.md FR-2. Motivating failure: answering "how much of
`fable-ous.md` (25 KB, ~40 findings) is already captured as tickets?" required
hand-fanning three agents across ~425 tickets. Plain grep missed semantically
equivalent phrasings ("PTY leaks its master fd" vs. a ticket titled "fd not closed
on natural exit"). No surface ranks tickets by overlap with a body of text.

Measured gap (2026-06-17, clide vault):

- Corpus gap — every `pql search` result is a vault `.md` path. `search` reads
  `index.db` (414 KB of vault notes); tickets live in `pql.db` (2.6 MB), which
  `search` never opens. The ticket-coverage question is invisible to it.
- A specific finding query returned `[]`.
- No lexical signal participates in rank: `pql search "terminal"` scored its top
  hits almost entirely on recency (weight 0.25), with `link_overlap`, `tag_overlap`,
  `path_proximity` and `centrality` all 0.

So the blockers are corpus plus signal wiring, not embeddings — Path A is viable
precisely because the pipeline, signal framework, and eval harness already exist and
are simply not pointed at the planning dataset.

Children: the design record first (Path A''s shape, plus Path B as an open
question), then the corpus/signal implementation, an eval set built from an existing
labelled mapping, and the doc reframe of the "Why not vectors" section. Path B (a
vector/embedding index) stays deferred and question-shaped per
`docs/structure/design-philosophy.md` — if it is ever taken it is a design-philosophy
amendment carrying the observed failure data, not a feature ticket.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:00:45.089', '2026-08-08 08:28:02.984', NULL, 'b07d0308553c77243618252b00771230', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'author the design record for planning-dataset ranking, plus the Path B question', 'FR-2 is explicitly "needs a design decision before any code". This ticket produces
the governance records; it gates the implementation ticket.

Two records:

1. A D-record for Path A. What the planning corpus looks like to
   generate → rank → bundle (tickets and decisions out of `pql.db` — a second
   corpus alongside `index.db`, which is a real architectural statement given D-3''s
   two-store split); which signals carry it (a textual/lexical-match signal is the
   one that is missing today); the document-length input shape
   (`ticket search --text-file` versus a `plan related --text-file` intent); and how
   it stays inside the consumer-agnostic core — `internal/intent/` and
   `internal/query/` must not import `internal/cli/`, and the same applies to
   whatever corpus adapter is added.
2. A Q-record for Path B (a vector/embedding index), anchored to FR-2''s evidence, so
   the "no vectors" stance is revisited only against data — exactly as the philosophy
   demands.

Mechanics: `pql decisions claim D architecture "…"` and `pql decisions claim Q
architecture "…"` for the ids, write the bodies into `governance/`, then
`pql decisions sync`.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-07 13:01:33.075', '2026-08-08 08:28:05.322', NULL, '5bef8612e8611d56acff1419c4be848e', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'task', '06FXRKYCM4CJQPRRTPBTWAWDRC', 'reframe the design-philosophy vectors section', '`docs/structure/design-philosophy.md`, section "Why not vectors", fuses two
different claims and contradicts itself: line 17 calls the absence of a vector layer
"not a gap to be filled later" (permanent), while line 19 says "if semantic
retrieval becomes necessary, it will be because a specific failure mode has been
observed repeatedly in production use" (conditional). FR-2 is the first logged
instance of exactly that trigger, so the wording now decides an argument instead of
merely setting a tone.

Split what the section fuses:

- Permanent — the discipline. Exhaust the cheap, inspectable signals before reaching
  for anything opaque. This never expires, vectors or no vectors.
- For now — the mechanism. The exclusion of a vector store, gated on the evidence
  trigger the section already names.

Suggested minimal reframe, from the FR: "…is not a gap to be filled later." becomes
"…is a deliberate constraint for now, not a permanent verdict," optionally closing
with "the discipline is permanent; the exclusion is conditional."

Maintainer-side edit by design — the consumer repo flagged it rather than editing
this repo''s philosophy doc from the outside.', 'backlog', 'low', NULL, NULL, NULL, '2026-08-07 13:02:34.996', '2026-08-08 08:28:07.484', NULL, '0cf648ce53d20e2ec549ef946cd9db2f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'bug', NULL, 'ci/release.sh is dead code; release.yaml calls goreleaser directly', NULL, 'backlog', 'medium', NULL, NULL, NULL, '2026-08-08 10:04:26.412', '2026-08-08 10:04:26.412', NULL, '758943f950618089e95dab833581b051', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'bug', NULL, 'ci/release.sh is dead code; release.yaml calls goreleaser directly', 'Found 2026-08-08 while auditing the Makefile for the divergence that broke the
2.0.0 release (`make lint` had drifted from `ci/lint.sh`).

`CLAUDE.md` states: "CI substance lives in `ci/{lint,test,release,eval}.sh`.
GitHub Actions workflows in `.github/workflows/` are thin wrappers around these —
keeps local and CI behaviour identical and lets the provider be swapped without
rewriting the scripts."

That is true for lint and test, and false for the other two:

- **`ci/release.sh` is invoked by nothing.** `release.yaml` uses
  `goreleaser/goreleaser-action@v6` with `args: release --clean` directly. The
  script runs `goreleaser release --clean`, so today they agree by coincidence —
  but nothing keeps them in step, and the stated swap-the-provider property does
  not hold for the one workflow that publishes binaries.
- **`ci/eval.sh` is invoked by nothing either.** Its header calls it a scheduled
  job, but no workflow schedules it. `make eval` now delegates to it, so it is at
  least exercised locally.

Options for release, in preference order:

1. Point `release.yaml` at `./ci/release.sh`, installing goreleaser the way the
   lint job does. Restores the documented property. Costs the action''s built-in
   caching and version pinning, which is worth checking before assuming it is
   free — the action pins a goreleaser version, the script uses whatever is on
   PATH.
2. Delete `ci/release.sh` and amend `CLAUDE.md` to say the release path
   deliberately uses the action. Honest, smaller, and gives up the swap property
   for that one workflow.

Either is fine; drifting docs are not. Pick one and make the doc match.

For eval: decide whether it is a scheduled job (add the schedule) or a manual
local tool (say so in the script header and in CLAUDE.md).

Deliberately not done during the 2.0.0 release — editing the release workflow
while a release is in flight re-triggers it.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-08 10:04:26.412', '2026-08-08 10:04:50.754', NULL, 'e267b7bfda6964ede8deaeae05f329b8', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'epic', NULL, 'close the surface defects found by the skill audit', NULL, 'backlog', 'high', NULL, NULL, NULL, '2026-08-08 12:08:42.220', '2026-08-08 12:08:42.220', NULL, '86ffb5819ac5ea5cbddfa46e1597b233', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'epic', NULL, 'close the surface defects found by the skill audit', 'Raised 2026-08-08 by the first two runs of the `pql-skill-auditor` agent
against 2.0.1. The audit exercises the embedded skill the way a consuming agent
does — twelve standardised retrievals plus exploration — and reports where the
documentation leaves that agent stuck or wrong.

Most of what it found was documentation, and is fixed. These are the residue:
defects in pql''s own surface that no amount of skill rewriting can paper over,
because the command genuinely does not do what a caller needs.

Three shapes recur across the children, worth naming because they suggest
where the next defects will be:

1. **Output that is correct but not workable.** The command succeeds and the
   data is right, yet the caller cannot act on it — a field missing from the
   default projection, a payload dominated by provenance, an answer that takes
   one call per record.
2. **Values that cannot be fed back in.** Several commands return link targets,
   paths or anchors in a form no other command accepts, so following a result
   to its source requires the caller to guess a transformation.
3. **Silent wrong answers.** A query that returns `[]` for a reason unrelated to
   the data, against a documented contract that says zero matches means nothing
   matched.

Related records rather than duplicates: the link-shape family is the user-facing
face of Q-6 (outlink target normalisation); the DSL grammar gap is T-45; and the
absence of any keyword or ranked surface over planning data is what T-62 exists
to design.', 'backlog', 'high', NULL, NULL, NULL, '2026-08-08 12:08:42.220', '2026-08-08 12:09:04.431', NULL, '9de09f5a121939a3a8525114f861f2da', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'backlinks misses path-shaped queries for wikilink targets', NULL, 'backlog', 'high', NULL, NULL, NULL, '2026-08-08 12:09:11.585', '2026-08-08 12:09:11.585', NULL, '07d3427fed29c3753c2508f6e9a6b4c8', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'backlinks misses path-shaped queries for wikilink targets', '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
because it matches the link *as written* rather than resolving it to a file.

Reproduced against `testdata/council-snapshot` (via `make scratch-vault`):

```
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona.md
[]
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona
[{"path":"members/vaasa/persona.md","name":"persona","line":12,"via":"wiki"}]
```

`internal/query/primitives/backlinks.go:40` takes `nameFromPath(opts.Path)`,
yielding the bare basename, which matches a wikilink written `[[persona]]` but
never a path-shaped one. The natural query — the vault-relative path every other
command accepts and every other command returns — is the one that fails.

Why this ranks high despite being one function: the output contract says zero
matches is success and should be reported as "nothing matched". A caller
following that rule reports "nothing links to this file" about a file that is
linked. A confident wrong answer is worse than an error, and it is reached by
doing exactly what the documentation says.

Note for whoever picks this up: the skill''s worked example used
`members/vaasa/persona.md`, and nothing in the fixture links to vaasa at all, so
the example returned `[]` correctly while appearing to demonstrate the command.
A broken example that looks like it works is how this survived two audits. The
example now shows both forms and the skill carries a caveat, but that caveat is
a workaround; this ticket is the fix.

Scope: resolve the query path to the set of link forms that can address it — at
minimum vault-relative path, extensionless path, and bare basename — then match
any of them. Anchors and relative prefixes are the same underlying problem and
belong to Q-6; this is the narrower "the documented form must work" fix.

Acceptance: `backlinks` returns the same rows for a file whether queried by
vault-relative path, extensionless path, or basename. Regression test over a
fixture holding both a wikilink and a markdown-link reference to one file.', 'backlog', 'high', NULL, NULL, NULL, '2026-08-08 12:09:11.585', '2026-08-08 12:11:56.513', NULL, '0507f2d3c40fe90a0dceb64f3586ed97', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'context returns paths no other command accepts', 'pql context is the skill''s first recommendation for ''what should I read alongside this file'', and its results cannot be fed to any other command. Two shapes come back in the same ''path'' key with nothing to distinguish them: extensionless file paths (members/koskela/persona, which pql meta rejects with exit 66 as ''file not indexed''), and bare heading anchors (#d-10-state-machine-for-ticket-status-transitions) that name no owning file at all. connections[] points back at the queried file rather than at the anchor''s owner, so the output contains no route to resolution. See internal/intent/context/context.go:49. Same root cause as T-72 and Q-6: link text is stored as written and never normalised. Acceptance: every path returned by context is accepted by meta or query, and an anchor result carries the file it lives in.', 'backlog', 'high', NULL, NULL, NULL, '2026-08-08 12:12:07.354', '2026-08-08 12:12:07.354', NULL, '5d0ee6458c65d79aaf98523be18b1b53', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'projection flags for the ranked commands', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-08 12:12:17.868', '2026-08-08 12:12:17.868', NULL, 'e72209366009ee06584fae86dfe3cea4', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JMD9401E6GTTN1C31EEGG', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'doctor emits index: null, breaking the omitted-not-null contract', 'The output contract says empty fields are omitted from JSON rather than set to null, so callers should check for presence. pql doctor breaks it: with no index.db it emits "index": null (declared at internal/cli/doctor.go:27, ''nil when DB does not exist''). A caller that presence-checks the key succeeds and then dereferences null. Two ways out: omit the key entirely when there is no index, which matches the contract, or keep null and carve doctor out of the contract explicitly. The first is better — doctor is the command an agent runs when something is already wrong, so it is the worst place to break a parsing rule. Found by the skill audit while testing the exit-70 recovery path.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-08 12:13:07.273', '2026-08-08 12:13:07.273', NULL, 'c0fe766bea79054f1343b75ee642f400', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JNM7W2X4857T6JQ5SSA4G', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'decisions show should batch ids like ticket show', 'ticket show takes comma-batched ids and returns an array; decisions show does not, and fails with exit 66 ''decision D-1,D-2 not found''. That asymmetry is invisible in the docs and turns one question into N calls: answering ''which decisions have nothing implementing them'' via the documented route is 41 sequential decisions show --with-tickets calls, one per record. The two-call workaround exists but needs ticket list --decision plus the field name decision_ref, both of which were undocumented until this audit. Making decisions show accept comma-batched ids mirrors an existing pattern, needs no new concepts, and collapses the common cross-surface question to one call. Acceptance: decisions show D-1,D-2 returns a two-element array, and --with-tickets composes with the batch form.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-08 12:13:17.247', '2026-08-08 12:13:17.247', NULL, '6c54e0e4734bcf0f31b2f54532b02b3d', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JPSWKSACAV6AGKWDWQ5W0', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'ticket board needs a status filter', 'ticket board takes only --team. On this repo 59 of 72 tickets are done, so roughly 82 percent of a 10 KB payload is a column nobody asked for, and there is no way to exclude it. The row shape is otherwise the best in the tool — id, type, title, status, priority, no descriptions. Two additions would fix it: a status filter to drop terminal columns, and honouring --fields the way the list verbs do. Also worth folding in: board columns carry the status name but not the display label from ticket statuslist, so a UI rendering columns has to join the two commands to get a human-readable header.', 'backlog', 'low', NULL, NULL, NULL, '2026-08-08 12:13:26.885', '2026-08-08 12:13:26.885', NULL, 'fbaf4fc4630901fe2e6793c5939dbbb4', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JXB8E7DNMENY6ZNWD09P0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'pqlignore is documented as a vault convention but is not read by default', 'CLAUDE.md calls .pqlignore one of the three vault-level conventions, alongside .pql/config.yaml and .pql/. In practice a .pqlignore at the vault root does nothing: ignore_files defaults to [.gitignore] (internal/config/config.go, and docs/pqlignore.md states the default plainly), so the file is inert until it is listed there. Verified on a clean vault with no config: a .pqlignore naming the offending file left indexing still aborting at exit 70; exclude: in config fixed it immediately. The docs are internally consistent — pqlignore.md does say the default is [.gitignore] — but the convention framing everywhere else implies the file works on its own, and pql init ships the prerequisite only as a commented-out example. Either make .pqlignore read by default when present, which matches the name and the framing, or stop calling it a convention and document it as opt-in. The first is the smaller surprise for a user who creates the file the docs are named after.', 'backlog', 'medium', NULL, NULL, NULL, '2026-08-08 12:14:20.483', '2026-08-08 12:14:20.483', NULL, '07f1a7248ba3e19de751ffcf68103cde', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'epic', NULL, 'close the surface defects found by the skill audit', 'Raised 2026-08-08 by the first two runs of the `pql-skill-auditor` agent
against 2.0.1. The audit exercises the embedded skill the way a consuming agent
does — twelve standardised retrievals plus exploration — and reports where the
documentation leaves that agent stuck or wrong.

Most of what it found was documentation, and is fixed. These are the residue:
defects in pql''s own surface that no amount of skill rewriting can paper over,
because the command genuinely does not do what a caller needs.

Three shapes recur across the children, worth naming because they suggest
where the next defects will be:

1. **Output that is correct but not workable.** The command succeeds and the
   data is right, yet the caller cannot act on it — a field missing from the
   default projection, a payload dominated by provenance, an answer that takes
   one call per record.
2. **Values that cannot be fed back in.** Several commands return link targets,
   paths or anchors in a form no other command accepts, so following a result
   to its source requires the caller to guess a transformation.
3. **Silent wrong answers.** A query that returns `[]` for a reason unrelated to
   the data, against a documented contract that says zero matches means nothing
   matched.

Related records rather than duplicates: the link-shape family is the user-facing
face of Q-6 (outlink target normalisation); the DSL grammar gap is T-45; and the
absence of any keyword or ranked surface over planning data is what T-62 exists
to design.', 'ready', 'high', NULL, NULL, NULL, '2026-08-08 12:08:42.220', '2026-08-08 12:14:28.200', NULL, 'bd6bb2f8484e517aca090e1add1e6e84', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'context returns paths no other command accepts', 'pql context is the skill''s first recommendation for ''what should I read alongside this file'', and its results cannot be fed to any other command. Two shapes come back in the same ''path'' key with nothing to distinguish them: extensionless file paths (members/koskela/persona, which pql meta rejects with exit 66 as ''file not indexed''), and bare heading anchors (#d-10-state-machine-for-ticket-status-transitions) that name no owning file at all. connections[] points back at the queried file rather than at the anchor''s owner, so the output contains no route to resolution. See internal/intent/context/context.go:49. Same root cause as T-72 and Q-6: link text is stored as written and never normalised. Acceptance: every path returned by context is accepted by meta or query, and an anchor result carries the file it lives in.', 'ready', 'high', NULL, NULL, NULL, '2026-08-08 12:12:07.354', '2026-08-08 12:14:28.207', NULL, '9601f164bfd930fbbc9039eb725e9bdc', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'backlinks misses path-shaped queries for wikilink targets', '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
because it matches the link *as written* rather than resolving it to a file.

Reproduced against `testdata/council-snapshot` (via `make scratch-vault`):

```
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona.md
[]
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona
[{"path":"members/vaasa/persona.md","name":"persona","line":12,"via":"wiki"}]
```

`internal/query/primitives/backlinks.go:40` takes `nameFromPath(opts.Path)`,
yielding the bare basename, which matches a wikilink written `[[persona]]` but
never a path-shaped one. The natural query — the vault-relative path every other
command accepts and every other command returns — is the one that fails.

Why this ranks high despite being one function: the output contract says zero
matches is success and should be reported as "nothing matched". A caller
following that rule reports "nothing links to this file" about a file that is
linked. A confident wrong answer is worse than an error, and it is reached by
doing exactly what the documentation says.

Note for whoever picks this up: the skill''s worked example used
`members/vaasa/persona.md`, and nothing in the fixture links to vaasa at all, so
the example returned `[]` correctly while appearing to demonstrate the command.
A broken example that looks like it works is how this survived two audits. The
example now shows both forms and the skill carries a caveat, but that caveat is
a workaround; this ticket is the fix.

Scope: resolve the query path to the set of link forms that can address it — at
minimum vault-relative path, extensionless path, and bare basename — then match
any of them. Anchors and relative prefixes are the same underlying problem and
belong to Q-6; this is the narrower "the documented form must work" fix.

Acceptance: `backlinks` returns the same rows for a file whether queried by
vault-relative path, extensionless path, or basename. Regression test over a
fixture holding both a wikilink and a markdown-link reference to one file.', 'ready', 'high', NULL, NULL, NULL, '2026-08-08 12:09:11.585', '2026-08-08 12:14:28.207', NULL, 'a10220dd445384a70f5abc39fb85cf6f', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'projection flags for the ranked commands', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.', 'ready', 'medium', NULL, NULL, NULL, '2026-08-08 12:12:17.868', '2026-08-08 12:14:28.208', NULL, '4d62521a892529068eea0767f19befa5', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JMD9401E6GTTN1C31EEGG', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'doctor emits index: null, breaking the omitted-not-null contract', 'The output contract says empty fields are omitted from JSON rather than set to null, so callers should check for presence. pql doctor breaks it: with no index.db it emits "index": null (declared at internal/cli/doctor.go:27, ''nil when DB does not exist''). A caller that presence-checks the key succeeds and then dereferences null. Two ways out: omit the key entirely when there is no index, which matches the contract, or keep null and carve doctor out of the contract explicitly. The first is better — doctor is the command an agent runs when something is already wrong, so it is the worst place to break a parsing rule. Found by the skill audit while testing the exit-70 recovery path.', 'ready', 'medium', NULL, NULL, NULL, '2026-08-08 12:13:07.273', '2026-08-08 12:14:28.208', NULL, 'dcfb02e4d501db190a0699c9c348856c', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JNM7W2X4857T6JQ5SSA4G', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'decisions show should batch ids like ticket show', 'ticket show takes comma-batched ids and returns an array; decisions show does not, and fails with exit 66 ''decision D-1,D-2 not found''. That asymmetry is invisible in the docs and turns one question into N calls: answering ''which decisions have nothing implementing them'' via the documented route is 41 sequential decisions show --with-tickets calls, one per record. The two-call workaround exists but needs ticket list --decision plus the field name decision_ref, both of which were undocumented until this audit. Making decisions show accept comma-batched ids mirrors an existing pattern, needs no new concepts, and collapses the common cross-surface question to one call. Acceptance: decisions show D-1,D-2 returns a two-element array, and --with-tickets composes with the batch form.', 'ready', 'medium', NULL, NULL, NULL, '2026-08-08 12:13:17.247', '2026-08-08 12:14:28.209', NULL, 'a292bc03401f92f8e736ef90568039bc', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JXB8E7DNMENY6ZNWD09P0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'pqlignore is documented as a vault convention but is not read by default', 'CLAUDE.md calls .pqlignore one of the three vault-level conventions, alongside .pql/config.yaml and .pql/. In practice a .pqlignore at the vault root does nothing: ignore_files defaults to [.gitignore] (internal/config/config.go, and docs/pqlignore.md states the default plainly), so the file is inert until it is listed there. Verified on a clean vault with no config: a .pqlignore naming the offending file left indexing still aborting at exit 70; exclude: in config fixed it immediately. The docs are internally consistent — pqlignore.md does say the default is [.gitignore] — but the convention framing everywhere else implies the file works on its own, and pql init ships the prerequisite only as a commented-out example. Either make .pqlignore read by default when present, which matches the name and the framing, or stop calling it a convention and document it as opt-in. The first is the smaller surprise for a user who creates the file the docs are named after.', 'ready', 'medium', NULL, NULL, NULL, '2026-08-08 12:14:20.483', '2026-08-08 12:14:28.209', NULL, 'd696b3cd05578b693f9f7338ad2f448d', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JPSWKSACAV6AGKWDWQ5W0', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'ticket board needs a status filter', 'ticket board takes only --team. On this repo 59 of 72 tickets are done, so roughly 82 percent of a 10 KB payload is a column nobody asked for, and there is no way to exclude it. The row shape is otherwise the best in the tool — id, type, title, status, priority, no descriptions. Two additions would fix it: a status filter to drop terminal columns, and honouring --fields the way the list verbs do. Also worth folding in: board columns carry the status name but not the display label from ticket statuslist, so a UI rendering columns has to join the two commands to get a human-readable header.', 'ready', 'low', NULL, NULL, NULL, '2026-08-08 12:13:26.885', '2026-08-08 12:14:28.209', NULL, 'ff559d39ebffe4a3edbf1ffb1a56c8b0', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'backlinks misses path-shaped queries for wikilink targets', '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
because it matches the link *as written* rather than resolving it to a file.

Reproduced against `testdata/council-snapshot` (via `make scratch-vault`):

```
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona.md
[]
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona
[{"path":"members/vaasa/persona.md","name":"persona","line":12,"via":"wiki"}]
```

`internal/query/primitives/backlinks.go:40` takes `nameFromPath(opts.Path)`,
yielding the bare basename, which matches a wikilink written `[[persona]]` but
never a path-shaped one. The natural query — the vault-relative path every other
command accepts and every other command returns — is the one that fails.

Why this ranks high despite being one function: the output contract says zero
matches is success and should be reported as "nothing matched". A caller
following that rule reports "nothing links to this file" about a file that is
linked. A confident wrong answer is worse than an error, and it is reached by
doing exactly what the documentation says.

Note for whoever picks this up: the skill''s worked example used
`members/vaasa/persona.md`, and nothing in the fixture links to vaasa at all, so
the example returned `[]` correctly while appearing to demonstrate the command.
A broken example that looks like it works is how this survived two audits. The
example now shows both forms and the skill carries a caveat, but that caveat is
a workaround; this ticket is the fix.

Scope: resolve the query path to the set of link forms that can address it — at
minimum vault-relative path, extensionless path, and bare basename — then match
any of them. Anchors and relative prefixes are the same underlying problem and
belong to Q-6; this is the narrower "the documented form must work" fix.

Acceptance: `backlinks` returns the same rows for a file whether queried by
vault-relative path, extensionless path, or basename. Regression test over a
fixture holding both a wikilink and a markdown-link reference to one file.

Fixed 2026-08-08. The diagnosis in this ticket was close but not exact, and the correction is worth recording: backlinks already matched three forms — the path as given, the bare basename, and basename+anchor — and nameFromPath was doing its job. The missing form was the extensionless FULL path, which is what Obsidian writes for a wikilink outside the current folder. Confirmed with pql outlinks: the stored target was members/koskela/persona, matching none of the three. The fix builds the set of spellings that address a file (path, path minus extension, basename), dedupes it so a top-level file does not double-count, and matches each with and without an anchor. Regression tests cover the extensionless full path from both query spellings, the anchored variant, and the no-duplicate case. Still out of scope and still Q-6: a link that reaches the file by a relative prefix is a different string and will not match. The skill''s caveat was narrowed to say exactly that rather than deleted.', 'ready', 'high', NULL, NULL, NULL, '2026-08-08 12:09:11.585', '2026-08-08 12:58:25.214', NULL, '3238932466b2247cbd89a1e812b13dc7', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'backlinks misses path-shaped queries for wikilink targets', '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
because it matches the link *as written* rather than resolving it to a file.

Reproduced against `testdata/council-snapshot` (via `make scratch-vault`):

```
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona.md
[]
$ pql --vault /tmp/pql-scratch-vault backlinks members/koskela/persona
[{"path":"members/vaasa/persona.md","name":"persona","line":12,"via":"wiki"}]
```

`internal/query/primitives/backlinks.go:40` takes `nameFromPath(opts.Path)`,
yielding the bare basename, which matches a wikilink written `[[persona]]` but
never a path-shaped one. The natural query — the vault-relative path every other
command accepts and every other command returns — is the one that fails.

Why this ranks high despite being one function: the output contract says zero
matches is success and should be reported as "nothing matched". A caller
following that rule reports "nothing links to this file" about a file that is
linked. A confident wrong answer is worse than an error, and it is reached by
doing exactly what the documentation says.

Note for whoever picks this up: the skill''s worked example used
`members/vaasa/persona.md`, and nothing in the fixture links to vaasa at all, so
the example returned `[]` correctly while appearing to demonstrate the command.
A broken example that looks like it works is how this survived two audits. The
example now shows both forms and the skill carries a caveat, but that caveat is
a workaround; this ticket is the fix.

Scope: resolve the query path to the set of link forms that can address it — at
minimum vault-relative path, extensionless path, and bare basename — then match
any of them. Anchors and relative prefixes are the same underlying problem and
belong to Q-6; this is the narrower "the documented form must work" fix.

Acceptance: `backlinks` returns the same rows for a file whether queried by
vault-relative path, extensionless path, or basename. Regression test over a
fixture holding both a wikilink and a markdown-link reference to one file.

Fixed 2026-08-08. The diagnosis in this ticket was close but not exact, and the correction is worth recording: backlinks already matched three forms — the path as given, the bare basename, and basename+anchor — and nameFromPath was doing its job. The missing form was the extensionless FULL path, which is what Obsidian writes for a wikilink outside the current folder. Confirmed with pql outlinks: the stored target was members/koskela/persona, matching none of the three. The fix builds the set of spellings that address a file (path, path minus extension, basename), dedupes it so a top-level file does not double-count, and matches each with and without an anchor. Regression tests cover the extensionless full path from both query spellings, the anchored variant, and the no-duplicate case. Still out of scope and still Q-6: a link that reaches the file by a relative prefix is a different string and will not match. The skill''s caveat was narrowed to say exactly that rather than deleted.', 'done', 'high', NULL, NULL, NULL, '2026-08-08 12:09:11.585', '2026-08-08 12:58:32.388', NULL, '384afc2419b8671b76160f1463ac8958', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'context returns paths no other command accepts', 'pql context is the skill''s first recommendation for ''what should I read alongside this file'', and its results cannot be fed to any other command. Two shapes come back in the same ''path'' key with nothing to distinguish them: extensionless file paths (members/koskela/persona, which pql meta rejects with exit 66 as ''file not indexed''), and bare heading anchors (#d-10-state-machine-for-ticket-status-transitions) that name no owning file at all. connections[] points back at the queried file rather than at the anchor''s owner, so the output contains no route to resolution. See internal/intent/context/context.go:49. Same root cause as T-72 and Q-6: link text is stored as written and never normalised. Acceptance: every path returned by context is accepted by meta or query, and an anchor result carries the file it lives in.

Fixed 2026-08-08. Cause: gatherCandidates selected links.target_path directly as a candidate path. target_path is link text as written, so the outbound branch emitted extensionless paths, anchored paths, and bare same-document anchors, while the inbound branch selected source_path and was fine all along. Outbound targets now strip any anchor and resolve against the files table by exact, plus-.md, or basename match, so every candidate is an indexed path. A bare #anchor addresses a heading in the same document, names no other file, and is dropped. Verified: context members/vaasa/persona.md now returns members/koskela/persona.md, which pql meta accepts, where it previously returned the extensionless form that meta rejected as not indexed. context governance/decisions/architecture.md now returns [] where it previously returned six bare #d-NN anchors — honest, since that file''s outbound links are all same-document cross-references between D-records. Correction worth recording: the first audit reported that context does not return heading anchors and I refuted it, pointing at a result path containing one, and kept ''returns heading anchors'' in the skill. That value was unresolved link text, not a designed feature. The claim is now removed. Residue, still Q-6: a relative-prefix target such as ../questions/architecture.md#q-1 does not resolve and is dropped rather than emitted broken.', 'ready', 'high', NULL, NULL, NULL, '2026-08-08 12:12:07.354', '2026-08-08 13:31:22.309', NULL, '0961ca595270b0beed4134fd5cc817f4', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'bug', '06FY2HM1XJV12S25YK3EHRV32C', 'context returns paths no other command accepts', 'pql context is the skill''s first recommendation for ''what should I read alongside this file'', and its results cannot be fed to any other command. Two shapes come back in the same ''path'' key with nothing to distinguish them: extensionless file paths (members/koskela/persona, which pql meta rejects with exit 66 as ''file not indexed''), and bare heading anchors (#d-10-state-machine-for-ticket-status-transitions) that name no owning file at all. connections[] points back at the queried file rather than at the anchor''s owner, so the output contains no route to resolution. See internal/intent/context/context.go:49. Same root cause as T-72 and Q-6: link text is stored as written and never normalised. Acceptance: every path returned by context is accepted by meta or query, and an anchor result carries the file it lives in.

Fixed 2026-08-08. Cause: gatherCandidates selected links.target_path directly as a candidate path. target_path is link text as written, so the outbound branch emitted extensionless paths, anchored paths, and bare same-document anchors, while the inbound branch selected source_path and was fine all along. Outbound targets now strip any anchor and resolve against the files table by exact, plus-.md, or basename match, so every candidate is an indexed path. A bare #anchor addresses a heading in the same document, names no other file, and is dropped. Verified: context members/vaasa/persona.md now returns members/koskela/persona.md, which pql meta accepts, where it previously returned the extensionless form that meta rejected as not indexed. context governance/decisions/architecture.md now returns [] where it previously returned six bare #d-NN anchors — honest, since that file''s outbound links are all same-document cross-references between D-records. Correction worth recording: the first audit reported that context does not return heading anchors and I refuted it, pointing at a result path containing one, and kept ''returns heading anchors'' in the skill. That value was unresolved link text, not a designed feature. The claim is now removed. Residue, still Q-6: a relative-prefix target such as ../questions/architecture.md#q-1 does not resolve and is dropped rather than emitted broken.', 'done', 'high', NULL, NULL, NULL, '2026-08-08 12:12:07.354', '2026-08-08 13:36:40.067', NULL, '085b1caca8e81a971f3ef6af89b07f2b', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'projection flags for the ranked commands', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.', 'in_progress', 'medium', NULL, NULL, NULL, '2026-08-08 12:12:17.868', '2026-08-08 14:00:47.164', NULL, 'a5c071828392db9c78a616c887fdfce7', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'projection flags for the ranked commands', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.

Delivered 2026-08-08 in 2.1.0. The same --fields/--oneline/--full trio the list verbs got in 1.11.0 now works on search, related and context, and their default projection drops signals[] and connections[], leaving path and score. Verified against the council snapshot: pql related members/vaasa/persona.md --limit 5 went from 1438 bytes to 183, and --full returns the original 1438 byte-for-byte. --fields path,signals recovers the provenance alone; --oneline emits path<TAB>score. Two things worth knowing beyond the ticket. Enriched.Signals gained omitempty, because Rank always fills it, so a ''signals'': null key would read as ''no ranking happened'' rather than ''the explanation was omitted'' — the same omitted-not-null rule the output contract already states, and exactly the defect T-75 files against doctor. And the projection applies under --flat-search too, where path is the only valid --fields name: nothing ranked those rows, so --fields score exits 64 naming path as the valid set rather than inventing a zero. Version call: this makes the unreleased section a minor, not the 2.0.1 hotfix it started as. Precedent is D-27, which changed ticket list''s default shape and shipped as 1.11.0. project.yaml and the CHANGELOG header both moved to 2.1.0. Helper renamed internal/cli/listproj.go -> projection.go (listProjection -> projection) since it is no longer list-only. D-27 amended rather than a new record: same mechanism, same principle, wider surface.', 'in_progress', 'medium', NULL, NULL, NULL, '2026-08-08 12:12:17.868', '2026-08-08 14:05:53.169', NULL, '770ad4448192c5a51868df6167ba61b1', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'task', '06FY2HM1XJV12S25YK3EHRV32C', 'projection flags for the ranked commands', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.

Delivered 2026-08-08 in 2.1.0. The same --fields/--oneline/--full trio the list verbs got in 1.11.0 now works on search, related and context, and their default projection drops signals[] and connections[], leaving path and score. Verified against the council snapshot: pql related members/vaasa/persona.md --limit 5 went from 1438 bytes to 183, and --full returns the original 1438 byte-for-byte. --fields path,signals recovers the provenance alone; --oneline emits path<TAB>score. Two things worth knowing beyond the ticket. Enriched.Signals gained omitempty, because Rank always fills it, so a ''signals'': null key would read as ''no ranking happened'' rather than ''the explanation was omitted'' — the same omitted-not-null rule the output contract already states, and exactly the defect T-75 files against doctor. And the projection applies under --flat-search too, where path is the only valid --fields name: nothing ranked those rows, so --fields score exits 64 naming path as the valid set rather than inventing a zero. Version call: this makes the unreleased section a minor, not the 2.0.1 hotfix it started as. Precedent is D-27, which changed ticket list''s default shape and shipped as 1.11.0. project.yaml and the CHANGELOG header both moved to 2.1.0. Helper renamed internal/cli/listproj.go -> projection.go (listProjection -> projection) since it is no longer list-only. D-27 amended rather than a new record: same mechanism, same principle, wider surface.', 'done', 'medium', NULL, NULL, NULL, '2026-08-08 12:12:17.868', '2026-08-08 14:05:57.677', NULL, 'bb9e0f9052d359b7b88bf01fe2a21957', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
