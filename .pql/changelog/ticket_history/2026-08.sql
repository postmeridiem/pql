INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'description', NULL, 'Source: feature-request.md FR-4 (settled-reach field hits 2026-07-08 and 2026-07-24).

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
asserting it sees the worktree''s markdown.', NULL, '2026-08-07 12:58:10', '2026-08-07 12:58:10.361', '2026-08-07 12:58:10.361', NULL, 'c00d04b480dee72e90a44678e2d80619', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'description', NULL, 'Source: feature-request.md FR-3 (settled-reach T-1057, authored 2026-06-12, loss
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
land together).', NULL, '2026-08-07 12:58:29', '2026-08-07 12:58:29.783', '2026-08-07 12:58:29.783', NULL, '3b00bfb5fabdcbb91ec5f80ca556c67b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'description', NULL, 'Source: feature-request.md FR-3, third bullet of the proposal.

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
warning, not a non-zero exit, unless rows were actually lost.', NULL, '2026-08-07 12:58:43', '2026-08-07 12:58:43.841', '2026-08-07 12:58:43.841', NULL, '57444ee42616341b3b392af9b507ce10', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'description', NULL, 'Source: feature-request.md FR-5 (settled-reach, 2026-07-27). A delegated agent
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
established precedent.', NULL, '2026-08-07 12:59:54', '2026-08-07 12:59:54.541', '2026-08-07 12:59:54.541', NULL, 'dc5d6d2c83fb57ed9d82811ba249fcc7', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'description', NULL, 'Source: feature-request.md FR-2. Motivating failure: answering "how much of
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
amendment carrying the observed failure data, not a feature ticket.', NULL, '2026-08-07 13:01:02', '2026-08-07 13:01:02.024', '2026-08-07 13:01:02.024', NULL, 'faf73b4b3edb6d9df1de7d4e2a61b8a5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'description', NULL, 'FR-2 is explicitly "needs a design decision before any code". This ticket produces
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
`pql decisions sync`.', NULL, '2026-08-07 13:01:53', '2026-08-07 13:01:53.251', '2026-08-07 13:01:53.251', NULL, 'a77669e66f3e3e3ac9e782d2c14bf684', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM7MBM4CF0E05K0D3AWMXC', 'description', NULL, 'Implement Path A per the D-record produced by the design ticket (blocker).

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
it done — NDCG@k / MRR / P@k, plus per-signal contribution diffs vs. baseline.', NULL, '2026-08-07 13:02:06', '2026-08-07 13:02:06.186', '2026-08-07 13:02:06.186', NULL, '6f5b484843c01471f982989f89f32d3e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM9C4J7N05CS7291SZXKMR', 'description', NULL, 'FR-2 hands over a labelled relevance mapping for free: clide''s epic T-359 with
children T-360 through T-386 map 1:1 onto the findings in `fable-ous.md`. That is a
gold standard for `internal/connect/rank/testdata/golden/`, produced by the same
session that surfaced the gap.

Convert it into a golden file so Path A can be tuned on NDCG@k / MRR / P@k via
`make eval` — before anyone argues the eval set is insufficient. Needs a snapshot of
the clide planning data plus the source document, following the fixture pattern
already used for `testdata/council-snapshot`.

Doable in parallel with the design record: this is the measurement, not the design.', NULL, '2026-08-07 13:02:20', '2026-08-07 13:02:20.973', '2026-08-07 13:02:20.973', NULL, '14b3188cea5c3bf43b2bf6d917e84905', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'description', NULL, '`docs/structure/design-philosophy.md`, section "Why not vectors", fuses two
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
this repo''s philosophy doc from the outside.', NULL, '2026-08-07 13:02:38', '2026-08-07 13:02:38.892', '2026-08-07 13:02:38.892', NULL, 'aae63b9696622f1a2abe8fb991285d1e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVWCA6PCMMPYMPD1SB8', 'description', NULL, 'Verified delivered on 2026-08-07 while triaging feature-request.md, closing retroactively. Shipped in 1.7.0: flag wired in internal/cli/ticket.go, dedicated regression test TestIntegration_TicketNew_IdOnly in internal/cli/writethrough_integration_test.go plus heavy use across the integration suite, documented in the embedded skill and in docs/output-contract.md as one of the two sanctioned plain-text output exceptions, CHANGELOG entry under Added.', NULL, '2026-08-07 13:06:14', '2026-08-07 13:06:14.488', '2026-08-07 13:06:14.488', NULL, '6376ab0d9f70fa218561e923f5a683e8', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVWCA6PCMMPYMPD1SB8', 'status', 'backlog', 'done', NULL, '2026-08-07 13:06:20', '2026-08-07 13:06:20.702', '2026-08-07 13:06:20.702', NULL, '66d2cdc2afe04c3dcb307b2d3e1bb121', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:22', '2026-08-07 13:07:22.719', '2026-08-07 13:07:22.719', NULL, '23b413e0492ff0c6ac4011d977994327', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:27', '2026-08-07 13:07:27.853', '2026-08-07 13:07:27.853', NULL, '9167d3fc587afe694ed1c9aace33766d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:30', '2026-08-07 13:07:30.300', '2026-08-07 13:07:30.300', NULL, '38029fe069684d0eeb76891be96e3e85', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:32', '2026-08-07 13:07:32.703', '2026-08-07 13:07:32.703', NULL, 'bc149d075d67d359d6407c8a4b3041f4', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:35', '2026-08-07 13:07:35.306', '2026-08-07 13:07:35.306', NULL, '5cafe34f66b0672f3e5a20c30d2b5b2f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:38', '2026-08-07 13:07:38.236', '2026-08-07 13:07:38.236', NULL, 'fce590726ed9bb50fbf2ad84cbe689ae', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:07:41', '2026-08-07 13:07:41.236', '2026-08-07 13:07:41.236', NULL, '83e8b8180eb356641f720d189d2a6703', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'status', 'ready', 'in_progress', NULL, '2026-08-07 13:07:47', '2026-08-07 13:07:47.387', '2026-08-07 13:07:47.387', NULL, '72b3ca7fc4cb2189ca1bacb17d3322d2', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FM2BKV2CFG7JHXDMQA7SYBRW', 'description', 'make test-integration mutates the developer''s HOME: the skill install/status/init tests invoke the binary without overriding HOME, and resolveSkillsRoot auto-resolves to user scope whenever any bundled skill is installed at ~/.claude/skills (true on any dev machine, never in CI). Observed 2026-07-08: six tests fail locally (StatusOnMissing, InstallIsIdempotent, InstallRefusesModified, Doctor_SkillFieldReportsState, Init_WithSkillYes/No) and the run overwrote ~/.claude/skills/pql/SKILL.md with the test binary''s embedded copy. Uninstall is safe (project-scope default, no auto-resolve). Fix: set HOME (and any config env) to a temp dir in the integration run() helper so the suite is hermetic; then these tests gate locally too.', 'make test-integration mutates the developer''s HOME: the skill install/status/init tests invoke the binary without overriding HOME, and resolveSkillsRoot auto-resolves to user scope whenever any bundled skill is installed at ~/.claude/skills (true on any dev machine, never in CI). Observed 2026-07-08: six tests fail locally (StatusOnMissing, InstallIsIdempotent, InstallRefusesModified, Doctor_SkillFieldReportsState, Init_WithSkillYes/No) and the run overwrote ~/.claude/skills/pql/SKILL.md with the test binary''s embedded copy. Uninstall is safe (project-scope default, no auto-resolve). Fix: set HOME (and any config env) to a temp dir in the integration run() helper so the suite is hermetic; then these tests gate locally too.

Fixed 2026-08-07. The integration suite now builds every invocation through a pqlCmd/sandboxEnv helper that redirects HOME, USERPROFILE and the XDG dirs to a per-test temp dir and strips ambient PQL_ overrides, so skill scope auto-resolve can no longer see the developer''s installed skills. One HOME per test, stable across invocations within it, so install-then-status still works. Verified: the six tests pass, the full integration suite is green on this machine for the first time, and the mtime on the real skills dir stops changing across runs.', NULL, '2026-08-07 13:29:29', '2026-08-07 13:29:29.498', '2026-08-07 13:29:29.498', NULL, 'b3a3cb94d6f0a003ef6d26c90bb07001', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FM2BKV2CFG7JHXDMQA7SYBRW', 'status', 'backlog', 'done', NULL, '2026-08-07 13:29:43', '2026-08-07 13:29:43.245', '2026-08-07 13:29:43.245', NULL, '46a58c2e293695659a8419cc60b8b900', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'status', 'in_progress', 'done', NULL, '2026-08-07 13:29:43', '2026-08-07 13:29:43.252', '2026-08-07 13:29:43.252', NULL, 'cd018e62f1d3fe8869614b54521a401e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'description', NULL, 'Source: observed twice in live sessions on 2026-08-07 — two different agents
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
  left describing a list-only flag.', NULL, '2026-08-07 13:35:32', '2026-08-07 13:35:32.727', '2026-08-07 13:35:32.727', NULL, 'd6cf03991ad740bb9543d7c70f632b44', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'description', NULL, 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
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
  behaviour worth stating deliberately, including whether it is opt-out.', NULL, '2026-08-07 13:39:07', '2026-08-07 13:39:07.883', '2026-08-07 13:39:07.883', NULL, '770b3c784f96e2042184a7daebb747e3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'description', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
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
  behaviour worth stating deliberately, including whether it is opt-out.', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
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

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.', NULL, '2026-08-07 13:39:39', '2026-08-07 13:39:39.807', '2026-08-07 13:39:39.807', NULL, 'e0252561e5822e009935af7575ec081a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:39:55', '2026-08-07 13:39:55.146', '2026-08-07 13:39:55.146', NULL, '8ef5cd07f4868b9bf3907f9431c65784', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'status', 'backlog', 'ready', NULL, '2026-08-07 13:39:57', '2026-08-07 13:39:57.948', '2026-08-07 13:39:57.948', NULL, 'f2b56efb15b10a1a8391bd8e43aba4b4', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKD48PHFF1QJ8W2NVKZCSG', 'status', 'ready', 'done', NULL, '2026-08-07 13:50:51', '2026-08-07 13:50:51.557', '2026-08-07 13:50:51.557', NULL, 'edce102882b7e6e2f2826b489f00115c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'description', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
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

Version-tracking requirement (raised 2026-08-07): this introduces a fourth version axis, and they now need to be legible together. Today there is project.yaml version (the app), project.yaml schema_version (index.db), planning.CanonicalVersion (pql.db row hashing), and with this ticket a changelog format version. A consumer holding format 1 needs to know which binary emits format 2, and pql needs to answer that without the user reading source. Requirements: declare the changelog format version in project.yaml beside schema_version so all declared versions live in one file; expose it plus CanonicalVersion from pql version --build-info, which already carries schema_version, so a consumer can diff what it has against what the binary emits; and keep a mapping of app version to each schema/format version somewhere durable (a table in docs or a section in CHANGELOG) so an upgrade across several releases can be reasoned about after the fact. The upgrade module should read the declared version rather than a constant buried in a package.', 'Raised 2026-08-07 while fixing T-59. The LWW guard is not a runtime rule — it is
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

Trigger revision (2026-08-07): run the upgrade on pull, driven by a version change, rather than on every load. The post-merge hook already owns the replication lifecycle (D-18) and already runs plan import, so a pull is where a working-tree change to tracked files is expected and unsurprising; rewriting .pql/changelog during an arbitrary read command would surprise the user and can race an open editor. Detection therefore compares the changelog''s declared format version against the binary''s on the hook path, not on every invocation, which also removes the requirement that the check be cheap enough to run constantly. Two gaps to cover so the trigger is not only a pull: a binary can be upgraded without any pull, and a fresh clone has no merge to hook. So also check at plan import and plan rebuild, which is where a format-incompatible changelog would otherwise fail, and provide an explicit verb for the manual case. On mismatch without an upgrade having run, the right behaviour is a loud diagnostic naming the format version found and the one expected, never a silent replay under the wrong rules.', NULL, '2026-08-07 13:58:00', '2026-08-07 13:58:00.484', '2026-08-07 13:58:00.484', NULL, 'b231dfaeba1ea06efd90eae7aa4c2d1c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'title', 'automatic changelog format upgrades on load', 'automatic changelog format upgrades, triggered on pull', NULL, '2026-08-07 13:58:06', '2026-08-07 13:58:06.465', '2026-08-07 13:58:06.465', NULL, 'f0312ae0ed13ea79782cfd6934c7d035', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKG5SNB3T4ZW5THC2C6YE4', 'status', 'ready', 'done', NULL, '2026-08-07 14:05:40', '2026-08-07 14:05:40.010', '2026-08-07 14:05:40.010', NULL, 'd31e6edd08c8b8dfdf77707ef0425a52', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVXWVVHST6374CJDB8R', 'status', 'backlog', 'ready', NULL, '2026-08-07 14:34:31', '2026-08-07 14:34:31.494', '2026-08-07 14:34:31.494', NULL, '7484ef56d0046a4eeaf24b6b02094c4f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'status', 'backlog', 'ready', NULL, '2026-08-07 14:34:34', '2026-08-07 14:34:34.041', '2026-08-07 14:34:34.041', NULL, '3729debd763643af1b8304b8a25f97ad', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'description', NULL, 'Raised 2026-08-07: T-44 and T-68 were filed months apart against different
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
settle.', NULL, '2026-08-07 14:36:09', '2026-08-07 14:36:09.505', '2026-08-07 14:36:09.505', NULL, 'b75bf3183772a810477d3d26c19d336b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVXWVVHST6374CJDB8R', 'parent_id', NULL, 'T-69', NULL, '2026-08-07 14:36:17', '2026-08-07 14:36:17.809', '2026-08-07 14:36:17.809', NULL, '0e0550e409d7eb9fd4164ec582f985a0', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'parent_id', NULL, 'T-69', NULL, '2026-08-07 14:36:17', '2026-08-07 14:36:17.816', '2026-08-07 14:36:17.816', NULL, 'a58fd6542394b54b0ac8a94a355e7048', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKF10Y9FZ967PAR80YQXD4', 'status', 'ready', 'done', NULL, '2026-08-07 14:36:57', '2026-08-07 14:36:57.431', '2026-08-07 14:36:57.431', NULL, '069c4edc5b0e680976a3c8589f515dd9', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKB21T80RKVFZ81PJH2H8M', 'decision_ref', NULL, 'D-3', NULL, '2026-08-07 14:37:46', '2026-08-07 14:37:46.943', '2026-08-07 14:37:46.943', NULL, '5895c6200b7a220e00be9a50b52a7ab6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'status', 'ready', 'in_progress', NULL, '2026-08-07 14:38:52', '2026-08-07 14:38:52.363', '2026-08-07 14:38:52.363', NULL, 'c73dc9f954c1e58bd568ed971d7e1a25', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'status', 'in_progress', 'in_progress', NULL, '2026-08-07 14:39:39', '2026-08-07 14:39:39.713', '2026-08-07 14:39:39.713', NULL, 'fa75de98d82770775628b02b8266fe8a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRW9P7VASFHMVSBD1BWB0B8', 'status', 'in_progress', 'done', NULL, '2026-08-08 08:07:22', '2026-08-08 08:07:22.135', '2026-08-08 08:07:22.135', NULL, '209d9828c86455ac2b590863f816e016', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FB0TGPVXWVVHST6374CJDB8R', 'status', 'ready', 'done', NULL, '2026-08-08 08:07:22', '2026-08-08 08:07:22.143', '2026-08-08 08:07:22.143', NULL, '906acc2165e3e50047ad498cc8af438a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXS9BB2GWPY0TWKS9X5V6WNW', 'status', 'ready', 'done', NULL, '2026-08-08 08:07:53', '2026-08-08 08:07:53.503', '2026-08-08 08:07:53.503', NULL, '744e18aeaa031034c338e06252071a69', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRKYCM4CJQPRRTPBTWAWDRC', 'status', 'ready', 'backlog', NULL, '2026-08-08 08:28:02', '2026-08-08 08:28:02.984', '2026-08-08 08:28:02.984', NULL, 'c1164b95c593876f09564042cc836564', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRM482B08DQ71BZQY9257XC', 'status', 'ready', 'backlog', NULL, '2026-08-08 08:28:05', '2026-08-08 08:28:05.322', '2026-08-08 08:28:05.322', NULL, '797ce0bf562430da3ad93ae668e7071a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRMBSYFHEMSA8D0RVF9B9SG', 'status', 'ready', 'backlog', NULL, '2026-08-08 08:28:07', '2026-08-08 08:28:07.485', '2026-08-08 08:28:07.485', NULL, '7b183e8dd839784df8a3f474c3570339', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY1N5XNKDTCQ354QCFAPQAA4', 'description', NULL, 'Found 2026-08-08 while auditing the Makefile for the divergence that broke the
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
while a release is in flight re-triggers it.', NULL, '2026-08-08 10:04:50', '2026-08-08 10:04:50.755', '2026-08-08 10:04:50.755', NULL, '6ca43640e931bc66b0b41fe7bf10f9f3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'description', NULL, 'Raised 2026-08-08 by the first two runs of the `pql-skill-auditor` agent
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
to design.', NULL, '2026-08-08 12:09:04', '2026-08-08 12:09:04.431', '2026-08-08 12:09:04.431', NULL, '5fd0ef981858d48de576c60c7f13ed0e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'description', NULL, '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
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
fixture holding both a wikilink and a markdown-link reference to one file.', NULL, '2026-08-08 12:11:56', '2026-08-08 12:11:56.513', '2026-08-08 12:11:56.513', NULL, 'b8f1fe545f8a125dc5cb805bd4c778ec', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.200', '2026-08-08 12:14:28.200', NULL, '58e9a7fd4b14116199b49bb44252f4d1', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.207', '2026-08-08 12:14:28.207', NULL, '50ac202a78fe2930b54b300f4b20174e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.207', '2026-08-08 12:14:28.207', NULL, '65406c4af0fbb02c356ea71d9e777741', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.208', '2026-08-08 12:14:28.208', NULL, '49181b0fd599bd1e2b4f2ef7af3b00d1', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JMD9401E6GTTN1C31EEGG', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.208', '2026-08-08 12:14:28.208', NULL, 'b6a7f325067bc0e5e27bbae622beee1b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JNM7W2X4857T6JQ5SSA4G', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.209', '2026-08-08 12:14:28.209', NULL, '2fb766b720f79dc7dbe68de845dd6abf', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JPSWKSACAV6AGKWDWQ5W0', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.209', '2026-08-08 12:14:28.209', NULL, '936926fabf72b3af21459514b9311228', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JXB8E7DNMENY6ZNWD09P0', 'status', 'backlog', 'ready', NULL, '2026-08-08 12:14:28', '2026-08-08 12:14:28.210', '2026-08-08 12:14:28.210', NULL, '7ad5f66ba411cc230d7ecc69a214f894', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'description', '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
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
fixture holding both a wikilink and a markdown-link reference to one file.', '`pql backlinks <vault-path>` returns `[]` for files that are genuinely linked,
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

Fixed 2026-08-08. The diagnosis in this ticket was close but not exact, and the correction is worth recording: backlinks already matched three forms — the path as given, the bare basename, and basename+anchor — and nameFromPath was doing its job. The missing form was the extensionless FULL path, which is what Obsidian writes for a wikilink outside the current folder. Confirmed with pql outlinks: the stored target was members/koskela/persona, matching none of the three. The fix builds the set of spellings that address a file (path, path minus extension, basename), dedupes it so a top-level file does not double-count, and matches each with and without an anchor. Regression tests cover the extensionless full path from both query spellings, the anchored variant, and the no-duplicate case. Still out of scope and still Q-6: a link that reaches the file by a relative prefix is a different string and will not match. The skill''s caveat was narrowed to say exactly that rather than deleted.', NULL, '2026-08-08 12:58:25', '2026-08-08 12:58:25.215', '2026-08-08 12:58:25.215', NULL, '9a425213a19a5546aa51eb76216a308e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HQMM5Z82QGWFTSFGH0NM0', 'status', 'ready', 'done', NULL, '2026-08-08 12:58:32', '2026-08-08 12:58:32.388', '2026-08-08 12:58:32.388', NULL, '6463f9ef3c4e6686b235822a04712b6b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'description', 'pql context is the skill''s first recommendation for ''what should I read alongside this file'', and its results cannot be fed to any other command. Two shapes come back in the same ''path'' key with nothing to distinguish them: extensionless file paths (members/koskela/persona, which pql meta rejects with exit 66 as ''file not indexed''), and bare heading anchors (#d-10-state-machine-for-ticket-status-transitions) that name no owning file at all. connections[] points back at the queried file rather than at the anchor''s owner, so the output contains no route to resolution. See internal/intent/context/context.go:49. Same root cause as T-72 and Q-6: link text is stored as written and never normalised. Acceptance: every path returned by context is accepted by meta or query, and an anchor result carries the file it lives in.', 'pql context is the skill''s first recommendation for ''what should I read alongside this file'', and its results cannot be fed to any other command. Two shapes come back in the same ''path'' key with nothing to distinguish them: extensionless file paths (members/koskela/persona, which pql meta rejects with exit 66 as ''file not indexed''), and bare heading anchors (#d-10-state-machine-for-ticket-status-transitions) that name no owning file at all. connections[] points back at the queried file rather than at the anchor''s owner, so the output contains no route to resolution. See internal/intent/context/context.go:49. Same root cause as T-72 and Q-6: link text is stored as written and never normalised. Acceptance: every path returned by context is accepted by meta or query, and an anchor result carries the file it lives in.

Fixed 2026-08-08. Cause: gatherCandidates selected links.target_path directly as a candidate path. target_path is link text as written, so the outbound branch emitted extensionless paths, anchored paths, and bare same-document anchors, while the inbound branch selected source_path and was fine all along. Outbound targets now strip any anchor and resolve against the files table by exact, plus-.md, or basename match, so every candidate is an indexed path. A bare #anchor addresses a heading in the same document, names no other file, and is dropped. Verified: context members/vaasa/persona.md now returns members/koskela/persona.md, which pql meta accepts, where it previously returned the extensionless form that meta rejected as not indexed. context governance/decisions/architecture.md now returns [] where it previously returned six bare #d-NN anchors — honest, since that file''s outbound links are all same-document cross-references between D-records. Correction worth recording: the first audit reported that context does not return heading anchors and I refuted it, pointing at a result path containing one, and kept ''returns heading anchors'' in the skill. That value was unresolved link text, not a designed feature. The claim is now removed. Residue, still Q-6: a relative-prefix target such as ../questions/architecture.md#q-1 does not resolve and is dropped rather than emitted broken.', NULL, '2026-08-08 13:31:22', '2026-08-08 13:31:22.309', '2026-08-08 13:31:22.309', NULL, '90ba3ce66a4a86d7e3ea49d9aa87c57a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JD37A2R9QN6C49Z4YJZ04', 'status', 'ready', 'done', NULL, '2026-08-08 13:36:40', '2026-08-08 13:36:40.067', '2026-08-08 13:36:40.067', NULL, '3042c6d9f960e660552da7a283b7b5f8', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'status', 'ready', 'in_progress', NULL, '2026-08-08 14:00:47', '2026-08-08 14:00:47.164', '2026-08-08 14:00:47.164', NULL, 'dd4ac0a10ea3e153975db66f359394c6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'description', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.', 'search, related and context have the worst signal-to-payload ratio in the tool and no way to trim it. Measured: pql related on one file returned 1438 bytes to deliver 88 bytes of paths, a 16x overhead, most of it five signal objects per result carrying zeros. --fields and --oneline exist on ticket list and decisions list and are explicitly scoped away from the ranked verbs, so the caller''s only option is to take the whole thing. At --limit 20 on a real vault this is most of an agent tool-output budget spent on provenance nobody asked for. The provenance is worth keeping — it is why a result is accountable — but it should be opt-in rather than mandatory. Suggest --fields over path/score, or a --no-signals flag, or making signals[] opt-in behind a flag. Whichever shape, the default should be the answer and the provenance the extra.

Delivered 2026-08-08 in 2.1.0. The same --fields/--oneline/--full trio the list verbs got in 1.11.0 now works on search, related and context, and their default projection drops signals[] and connections[], leaving path and score. Verified against the council snapshot: pql related members/vaasa/persona.md --limit 5 went from 1438 bytes to 183, and --full returns the original 1438 byte-for-byte. --fields path,signals recovers the provenance alone; --oneline emits path<TAB>score. Two things worth knowing beyond the ticket. Enriched.Signals gained omitempty, because Rank always fills it, so a ''signals'': null key would read as ''no ranking happened'' rather than ''the explanation was omitted'' — the same omitted-not-null rule the output contract already states, and exactly the defect T-75 files against doctor. And the projection applies under --flat-search too, where path is the only valid --fields name: nothing ranked those rows, so --fields score exits 64 naming path as the valid set rather than inventing a zero. Version call: this makes the unreleased section a minor, not the 2.0.1 hotfix it started as. Precedent is D-27, which changed ticket list''s default shape and shipped as 1.11.0. project.yaml and the CHANGELOG header both moved to 2.1.0. Helper renamed internal/cli/listproj.go -> projection.go (listProjection -> projection) since it is no longer list-only. D-27 amended rather than a new record: same mechanism, same principle, wider surface.', NULL, '2026-08-08 14:05:53', '2026-08-08 14:05:53.169', '2026-08-08 14:05:53.169', NULL, '1034399528790350718ec2b6c30d30a2', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JEC9GPY21V4ESRKM453WR', 'status', 'in_progress', 'done', NULL, '2026-08-08 14:05:57', '2026-08-08 14:05:57.677', '2026-08-08 14:05:57.677', NULL, '8ac40dd28ff6480646d322b22538de1f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JMD9401E6GTTN1C31EEGG', 'status', 'ready', 'in_progress', NULL, '2026-08-08 14:06:21', '2026-08-08 14:06:21.330', '2026-08-08 14:06:21.330', NULL, '55fcd0c1059e59c5bccc8b9c94b82d4c', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JMD9401E6GTTN1C31EEGG', 'description', 'The output contract says empty fields are omitted from JSON rather than set to null, so callers should check for presence. pql doctor breaks it: with no index.db it emits "index": null (declared at internal/cli/doctor.go:27, ''nil when DB does not exist''). A caller that presence-checks the key succeeds and then dereferences null. Two ways out: omit the key entirely when there is no index, which matches the contract, or keep null and carve doctor out of the contract explicitly. The first is better — doctor is the command an agent runs when something is already wrong, so it is the worst place to break a parsing rule. Found by the skill audit while testing the exit-70 recovery path.', 'The output contract says empty fields are omitted from JSON rather than set to null, so callers should check for presence. pql doctor breaks it: with no index.db it emits "index": null (declared at internal/cli/doctor.go:27, ''nil when DB does not exist''). A caller that presence-checks the key succeeds and then dereferences null. Two ways out: omit the key entirely when there is no index, which matches the contract, or keep null and carve doctor out of the contract explicitly. The first is better — doctor is the command an agent runs when something is already wrong, so it is the worst place to break a parsing rule. Found by the skill audit while testing the exit-70 recovery path.

Fixed 2026-08-08 in 2.1.0. doctorReport.Index gained omitempty, so the key is absent rather than null when there is no database. Took the first option in the ticket, for the reason the ticket gives: doctor is what an agent runs when something is already wrong, so it is the worst place to break a parsing rule. Callers branch on db.exists. Worth recording why this survived: TestIntegration_Doctor_FreshVaultBeforeIndex already asserted on it, but as rep[''index''] != nil — and unmarshalling into map[string]any gives nil for both an absent key and a null one, so the test could not distinguish the thing it existed to check. It now checks key presence via the two-value map read. Audited the rest of the output surface for the same defect rather than fixing only the reported instance: every other pointer or slice field that reaches stdout is either always populated at construction (plan rebuild --verify''s verify), or initialised to an empty slice before marshalling (meta''s tags/outlinks/headings return [] not null, doctor''s skills, config''s tag_sources/exclude/ignore_files). doctor.index was the only violation. The rule is now stated in docs/output-contract.md as binding on every surface including diagnostics, where it had only been implied.', NULL, '2026-08-08 14:32:13', '2026-08-08 14:32:13.593', '2026-08-08 14:32:13.593', NULL, '67cee07520f3766681a07cabfdf31dc8', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JMD9401E6GTTN1C31EEGG', 'status', 'in_progress', 'done', NULL, '2026-08-08 14:32:13', '2026-08-08 14:32:13.613', '2026-08-08 14:32:13.613', NULL, 'f42fc05d9ee3e77e9ec47848386a4370', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JXB8E7DNMENY6ZNWD09P0', 'description', 'CLAUDE.md calls .pqlignore one of the three vault-level conventions, alongside .pql/config.yaml and .pql/. In practice a .pqlignore at the vault root does nothing: ignore_files defaults to [.gitignore] (internal/config/config.go, and docs/pqlignore.md states the default plainly), so the file is inert until it is listed there. Verified on a clean vault with no config: a .pqlignore naming the offending file left indexing still aborting at exit 70; exclude: in config fixed it immediately. The docs are internally consistent — pqlignore.md does say the default is [.gitignore] — but the convention framing everywhere else implies the file works on its own, and pql init ships the prerequisite only as a commented-out example. Either make .pqlignore read by default when present, which matches the name and the framing, or stop calling it a convention and document it as opt-in. The first is the smaller surprise for a user who creates the file the docs are named after.', 'CLAUDE.md calls .pqlignore one of the three vault-level conventions, alongside .pql/config.yaml and .pql/. In practice a .pqlignore at the vault root does nothing: ignore_files defaults to [.gitignore] (internal/config/config.go, and docs/pqlignore.md states the default plainly), so the file is inert until it is listed there. Verified on a clean vault with no config: a .pqlignore naming the offending file left indexing still aborting at exit 70; exclude: in config fixed it immediately. The docs are internally consistent — pqlignore.md does say the default is [.gitignore] — but the convention framing everywhere else implies the file works on its own, and pql init ships the prerequisite only as a commented-out example. Either make .pqlignore read by default when present, which matches the name and the framing, or stop calling it a convention and document it as opt-in. The first is the smaller surprise for a user who creates the file the docs are named after.

Fixed 2026-08-08 in 2.1.0. Took the first option in the ticket: ignore_files now defaults to [.gitignore, .pqlignore] rather than [.gitignore], so a .pqlignore at the vault root works the moment it is created. ignore.Load already skips a name that is not on disk, so the second entry is inert in vaults that have no .pqlignore — this adds a capability without changing behaviour for anyone who was not creating the file. Order is load-bearing: .pqlignore comes second so it can re-include something .gitignore excluded with a leading !. The walker never needed changing; it has honoured multiple ignore files since the feature landed, and TestWalk_MultipleIgnoreFiles covered exactly this list. The whole defect was the default. Verified end to end on a config-free scratch vault: with no .pqlignore both keep.md and drafts/skip.md index; adding a bare .pqlignore containing drafts/ drops skip.md. Regression-guarded by TestIntegration_PqlignoreWorksWithoutConfig, which asserts no config.yaml is present first so it cannot pass for the wrong reason. Consumer note: the config hash changes, so the first indexed run after upgrading rebuilds index.db. Pure cache, nothing lost, and a vault pinning ignore_files: explicitly is unaffected. Docs realigned rather than left describing the old default — pqlignore.md, vault-layout.md (which now lists .pqlignore as its own row in the conventions table), the pql init config template, and the embedded skill, whose exit-70 recovery section had been telling agents that .pqlignore is not read by default and to use exclude: instead.', NULL, '2026-08-08 14:34:54', '2026-08-08 14:34:54.762', '2026-08-08 14:34:54.762', NULL, 'cd0f751f48257428abf8554c3950867d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JXB8E7DNMENY6ZNWD09P0', 'status', 'ready', 'done', NULL, '2026-08-08 14:34:54', '2026-08-08 14:34:54.786', '2026-08-08 14:34:54.786', NULL, '42d0cc1b858ea27962e8039fc75b92be', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JNM7W2X4857T6JQ5SSA4G', 'status', 'ready', 'in_progress', NULL, '2026-08-08 14:36:42', '2026-08-08 14:36:42.906', '2026-08-08 14:36:42.906', NULL, '5daa262d113e9175bc6123bad8c68ced', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JPSWKSACAV6AGKWDWQ5W0', 'status', 'ready', 'in_progress', NULL, '2026-08-08 14:41:14', '2026-08-08 14:41:14.522', '2026-08-08 14:41:14.522', NULL, '414634629eff74d7f1378da4df2ee783', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JNM7W2X4857T6JQ5SSA4G', 'description', 'ticket show takes comma-batched ids and returns an array; decisions show does not, and fails with exit 66 ''decision D-1,D-2 not found''. That asymmetry is invisible in the docs and turns one question into N calls: answering ''which decisions have nothing implementing them'' via the documented route is 41 sequential decisions show --with-tickets calls, one per record. The two-call workaround exists but needs ticket list --decision plus the field name decision_ref, both of which were undocumented until this audit. Making decisions show accept comma-batched ids mirrors an existing pattern, needs no new concepts, and collapses the common cross-surface question to one call. Acceptance: decisions show D-1,D-2 returns a two-element array, and --with-tickets composes with the batch form.', 'ticket show takes comma-batched ids and returns an array; decisions show does not, and fails with exit 66 ''decision D-1,D-2 not found''. That asymmetry is invisible in the docs and turns one question into N calls: answering ''which decisions have nothing implementing them'' via the documented route is 41 sequential decisions show --with-tickets calls, one per record. The two-call workaround exists but needs ticket list --decision plus the field name decision_ref, both of which were undocumented until this audit. Making decisions show accept comma-batched ids mirrors an existing pattern, needs no new concepts, and collapses the common cross-surface question to one call. Acceptance: decisions show D-1,D-2 returns a two-element array, and --with-tickets composes with the batch form.

Delivered 2026-08-08 in 2.1.0. decisions show now takes comma-batched ids via the same parseIDs helper ticket show uses, loops GetDecision + buildDecisionTree per id, and renders One for a single id or an array for several — identical rule to ticket show, so a caller who learned one knows the other. --with-tickets and --with-refs compose with the batch. Side effect worth noting: the not-found error now names the individual id that is missing rather than echoing the whole comma string back, which was the confusing part of the original report (''decision D-1,D-2 not found'' reads as though a record with that literal name was sought). Regression-guarded by TestIntegration_DecisionsShow_BatchesIDs, which covers array order, --with-tickets composing, the single-id object shape, and the exit-66 path asserting the error names D-99 alone and not the comma string. The ticket also noted that ticket list --decision and the field name decision_ref were undocumented until the audit; both are in the skill now, and the batched form is documented alongside the implementation-status view so the cross-surface question reads as one call.', NULL, '2026-08-08 14:54:16', '2026-08-08 14:54:16.930', '2026-08-08 14:54:16.930', NULL, '0f9c459752070e3d06e3e24e0ad004a3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JNM7W2X4857T6JQ5SSA4G', 'status', 'in_progress', 'done', NULL, '2026-08-08 14:54:16', '2026-08-08 14:54:16.951', '2026-08-08 14:54:16.951', NULL, 'efe2ea9be073adb7014f634afb1fcc6e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JPSWKSACAV6AGKWDWQ5W0', 'description', 'ticket board takes only --team. On this repo 59 of 72 tickets are done, so roughly 82 percent of a 10 KB payload is a column nobody asked for, and there is no way to exclude it. The row shape is otherwise the best in the tool — id, type, title, status, priority, no descriptions. Two additions would fix it: a status filter to drop terminal columns, and honouring --fields the way the list verbs do. Also worth folding in: board columns carry the status name but not the display label from ticket statuslist, so a UI rendering columns has to join the two commands to get a human-readable header.', 'ticket board takes only --team. On this repo 59 of 72 tickets are done, so roughly 82 percent of a 10 KB payload is a column nobody asked for, and there is no way to exclude it. The row shape is otherwise the best in the tool — id, type, title, status, priority, no descriptions. Two additions would fix it: a status filter to drop terminal columns, and honouring --fields the way the list verbs do. Also worth folding in: board columns carry the status name but not the display label from ticket statuslist, so a UI rendering columns has to join the two commands to get a human-readable header.

Delivered 2026-08-08 in 2.1.0, two of the three asks. --open drops every column the vault classes as terminal; --status takes an explicit comma-separated column set. Measured on this repo: 10883 bytes unfiltered, 1755 with --open. Each column now also carries label alongside status, so a UI rendering headers no longer joins against ticket statuslist to turn in_progress into ''In Progress''. Deliberate departure recorded in boardColumns: --status here validates against the vocabulary and exits 64 listing it, where ticket list --status passes any value through to SQL and returns empty at exit 0. That asymmetry is intentional and now documented in the skill''s filter-value warning as one of the two checked flags. Reason: a mistyped row filter returns fewer rows, a mistyped column name renders an empty board that is indistinguishable from a finished one. --status and --open are mutually exclusive, exit 64, since --open is just the shorthand for the non-terminal set. Not delivered: --fields on board. Rows are nested inside columns, so a field list is ambiguous about which level it projects — columns or tickets — and resolving that needs projection machinery render.Project does not have, for a row shape this ticket itself calls the best in the tool (5 small fields, no descriptions). The 82 percent problem was the terminal columns, and that is fixed. Stated in the board help text and the CHANGELOG rather than left silent; ticket list --fields covers the case where an exact key set is genuinely wanted.', NULL, '2026-08-08 14:55:09', '2026-08-08 14:55:09.645', '2026-08-08 14:55:09.645', NULL, '4ad792367cc5fbfb603a05c4db56886a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2JPSWKSACAV6AGKWDWQ5W0', 'status', 'in_progress', 'done', NULL, '2026-08-08 14:55:09', '2026-08-08 14:55:09.662', '2026-08-08 14:55:09.662', NULL, '0d8e6e13525c98b6ca0f964d78f3c3ca', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'status', 'ready', 'in_progress', NULL, '2026-08-08 14:55:54', '2026-08-08 14:55:54.244', '2026-08-08 14:55:54.244', NULL, 'aa4c0ae2b69e13b4401ac3f6dc33a241', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'status', 'ready', 'in_progress', NULL, '2026-08-08 14:55:54', '2026-08-08 14:55:54.261', '2026-08-08 14:55:54.261', NULL, '1ccad6c6265dff8dbddb818b0b978e06', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'description', 'Source: observed twice in live sessions on 2026-08-07 — two different agents
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
  left describing a list-only flag.', 'Source: observed twice in live sessions on 2026-08-07 — two different agents
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
  left describing a list-only flag.

Delivered 2026-08-08 in 2.1.0. Answers to the three open questions the ticket left the implementer. (1) --fields projects the top level only. The join-trees from --with-context, --with-blockers, --with-children and --tree are all-or-nothing: naming a key inside a nested tree needs a path syntax --fields does not have, and inventing one would be a second projection language for the rarer case. Stated in both verbs'' help text as the ticket asked. (2) Batched show projects per element via render.Project over the tree slice; a single id keeps its object shape, several render an array. Shared helper renderShowRecords in internal/cli/projection.go, used by both verbs. (3) D-27 amended, not left describing a list-only flag. The amendment separates what the record got right from what it got wrong: the default is still whole records, which is what D-27 was actually reasoning about; making the flag list-only was the error, because the boundary is not guessable. --oneline and --full stay list-only and remain unknown flags on show, exit 64 — --oneline would turn a record-level verb into an index, and --full has nothing to restore where nothing is dropped. Found a real bug doing this, filed nowhere because it is fixed in the same change: ticket show --with-children returned nothing, ever, and so did the children half of --with-context. repo.ChildrenOf is keyed on parent_record_id but the CLI passed it the friendly T-NNN, so the query matched no row from the moment D-26 split the two identifiers. Silent, because an epic with no children is a legitimate answer. It surfaced only because projecting children on T-71, which has seven, came back empty. Guarded by TestIntegration_TicketShow_ChildrenJoinResolves, which checks both flags populate and that a genuine leaf still reports none, so the fix cannot pass by always populating.', NULL, '2026-08-08 15:25:58', '2026-08-08 15:25:58.637', '2026-08-08 15:25:58.637', NULL, 'd079acb91c5caafd3547d2cd275d5f5e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FXRVT8QNJ0V4WPGZQW9DNH5C', 'status', 'in_progress', 'done', NULL, '2026-08-08 15:25:58', '2026-08-08 15:25:58.655', '2026-08-08 15:25:58.655', NULL, '678de5b201488a95e56eaf9a320e7394', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'description', 'Raised 2026-08-08 by the first two runs of the `pql-skill-auditor` agent
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
to design.', 'Raised 2026-08-08 by the first two runs of the `pql-skill-auditor` agent
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
to design.

Epic closed 2026-08-08, all seven children delivered in 2.1.0. What the audit bought, in order of severity. Two silent wrong answers: backlinks missed the extensionless full path Obsidian writes for cross-folder wikilinks, so querying a file by the spelling every other command returns reported no backlinks (T-72); and context emitted raw link text as result paths, so its output could not be fed to any other command and one governance file returned six same-document heading anchors ranked against each other (T-73). Two contract breaches: doctor emitted index: null against the omitted-not-null rule, in the one command you run when something is already wrong (T-75), and .pqlignore was documented as a vault convention while being inert without a config edit (T-78). Three ergonomics defects that each cost round trips: ranked results carried five signal objects per row with no way to trim, 16x payload for the answer (T-74); decisions show would not batch, making a cross-surface question 41 calls (T-76); ticket board had no way to drop terminal columns, 82 percent of a mature board (T-77). Plus T-67 from the same class, filed a day earlier from live sessions. Two things worth carrying forward. First, an eighth defect fell out of fixing the seventh: ticket show --with-children had returned nothing since D-26 split record_id from ticket_id, silent because an empty children list is a legitimate answer, and it surfaced only because --fields let me ask for a key I knew had seven entries. Second, the pattern across all of these is that none were reachable from inside. Every one needed a consumer with no repo knowledge using the documented surface and reporting what actually happened. That is the argument for keeping the audit on the release-mint hook rather than running it ad hoc. Also corrected in this cycle: I refuted the first audit''s finding on context heading anchors and was wrong; the second audit proved it. The skill no longer claims anchors are a feature.', NULL, '2026-08-08 15:26:37', '2026-08-08 15:26:37.418', '2026-08-08 15:26:37.418', NULL, '297054dcf7876e0ee461ed4a5019e6e6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY2HM1XJV12S25YK3EHRV32C', 'status', 'in_progress', 'done', NULL, '2026-08-08 15:26:37', '2026-08-08 15:26:37.435', '2026-08-08 15:26:37.435', NULL, '56db78d5358214bd93dd1c6c5e0093ca', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'parent_id', NULL, 'T-79', NULL, '2026-08-08 18:05:06', '2026-08-08 18:05:06.580', '2026-08-08 18:05:06.580', NULL, '810e2e08eb2c88d17f1b56657f220d36', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'description', NULL, 'Audit findings Wrong #5 and Missing #9.

  ticket show T-63 --with-context --fields id,title
  -> {"id":"T-63","title":"..."}    exit 0, no ancestors, no warning

  ticket show T-63 --with-context --fields id,ancestors
  -> {"id":"T-63","ancestors":[...]}

So --fields does select the joins; a join key not named is dropped even though its --with- flag was passed. The skill says --fields ''narrows the top level only: the join-trees are all-or-nothing'', which describes what happens to a join that survives but reads as a promise the join arrives whole, and gives no hint a lever exists.

Compounded by Missing #9: the skill''s --fields valid-name lists give 13 ticket fields and 8 decision fields, while the exit-64 error lists 19. ancestors, blockers, children, decisions, subtree and message are all absent from the skill — and that section opens with ''Valid field names, since guessing them costs a round trip''.

Failure mode: ask for context, ask to trim, get neither, notice nothing. Both halves are from T-67, shipped the same day.

Fix, skill-side: state that --fields also selects joins and that an unnamed join key is dropped, list the join keys, and keep the true part — join contents are never projected. Decide deliberately whether passing --with-X while omitting X from --fields warrants a stderr warning; a silent drop of something explicitly requested is what diagnostics are for, but warning on a legitimate top-level-only call would be noise.

Filing note: this ticket could not be created with its title as a positional argument — a title beginning with -- is parsed as a flag and exits 64. `ticket new bug -- "--fields ..."` works. Worth a line in the skill.', NULL, '2026-08-08 18:05:06', '2026-08-08 18:05:06.597', '2026-08-08 18:05:06.597', NULL, '53dcd437674571e6aca0299119360786', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'priority', 'medium', 'high', NULL, '2026-08-08 18:05:06', '2026-08-08 18:05:06.597', '2026-08-08 18:05:06.597', NULL, 'a89ac277edd680da4a136852e0a77f25', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52S461Z9QKWVK0JTJEFKN8', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.115', '2026-08-08 18:05:28.115', NULL, '4af2811777f0adf3147f8e6ae6c591db', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52TK0B80DXVEKGVGBRMNVR', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.122', '2026-08-08 18:05:28.122', NULL, '01d4987673a7bd1e31a3f1bf21cf407a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52VTK08206BJ7Z21JMZRQM', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.122', '2026-08-08 18:05:28.122', NULL, '512f6f507dd07d5809b75da9cfc0210d', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52X079W1G0SPCBAVPYABXR', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.122', '2026-08-08 18:05:28.122', NULL, 'b049b51efc7c7d79ba313fe96e8d747b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.123', '2026-08-08 18:05:28.123', NULL, '478c4ee28c236af4ff045395ffac81fd', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52Y65C1K85YMW8HAJS831G', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.123', '2026-08-08 18:05:28.123', NULL, 'ea3956b4fb21097ab532a4f03d325a05', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY537DDR64R4DYT04X86JWZW', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.124', '2026-08-08 18:05:28.124', NULL, '184ee69b57227b7556c0e54d4470416f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY538NZECFSTEWA3831TA410', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:05:28', '2026-08-08 18:05:28.124', '2026-08-08 18:05:28.124', NULL, 'a73ac186b0a6617836be5131039c2ee1', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53V23XT3Y1NBT9Y4E0R438', 'description', 'The pql-testing procedure''s own defects, found by the third audit run while following it. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

PD-1 — the setup''s version check cannot detect the thing it exists to detect. It compares pql --version against ./bin/pql --version and says to reinstall if they differ. Both printed 2.1.0 while doctor showed the PATH binary was built at 951e4f9 and ./bin/pql at 1159a8f. Because version strings are deliberately clean with no SHA (D-12), version equality is the wrong comparison — two binaries from different commits agree whenever the release version has not been bumped. Here the intervening commits touched only the audit skill, but the check would have passed just as happily across a functional change. Fix: compare doctor''s version.commit against git rev-parse --short HEAD, or add make binary-drift alongside make skill-drift.

PD-2 — the sanctioned poison test has no sanctioned recovery test. make scratch-poison exists; nothing lets the auditor exercise the .pqlignore recovery the skill documents, because writing a file needs shell redirection. Reading the scratch vault is also denied — it sits outside the session''s read roots — so warning W2 went unchecked and finding #23 could not be resolved to pql''s fault or the .base file''s. Fix: make scratch-ignore PATTERN=..., and add the scratch vault to the audit''s allowed read roots.

PD-3 — no safe way to exercise pql init. Documented command, documented side effects, and neither the skill nor --help says whether it targets cwd or --vault, so running it risks writing into this repo. Fix: make scratch-init, or pin the target directory in the skill.

PD-4 — exit codes are only observable when non-zero, since 127 is a compound and refused. A successful exit 0 is inferred from the absence of an error banner. Fine in practice, but R2 is load-bearing and is verified by inference; say so in the procedure so future auditors do not spend calls rediscovering it.

PD-5 — step 4 says read the source and locate each fix; the tasking said not to read source at all. The auditor honoured the tasking, so no finding carries a source location. The two documents must agree. Worth deciding deliberately: the no-source rule is what keeps the outside-in view honest, and step 4 may simply belong to whoever triages the report rather than to the auditor.

PD-6 — the report cannot be written where it was asked for. The agent has no write tool by design, redirection is denied, and the target path is outside its read roots. It returned the report as a message and the orchestrating session transcribed it. Fix: either drop the write-to-file instruction from the tasking, or add a make audit-report target reading stdin.

PD-7 — the cost estimate is low. The procedure predicts roughly 100k tokens and 90+ tool calls; this run made ~130 pql invocations plus ~14 help and make calls, with 26 warnings checked. State 130+ if the core set, a full warnings pass and real exploration are all expected.', 'The pql-testing procedure''s own defects, found by the third audit run while following it. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

PD-1 — the setup''s version check cannot detect the thing it exists to detect. It compares `pql --version` against `./bin/pql --version` and says to reinstall if they differ. Both printed 2.1.0 while `doctor` showed the PATH binary was built at 951e4f9 and ./bin/pql at 1159a8f. Because version strings are deliberately clean with no SHA (D-12), version equality is the wrong comparison — two binaries from different commits agree whenever the release version has not been bumped. Here the intervening commits touched only the audit skill, but the check would have passed just as happily across a functional change. Fix: compare `doctor`''s version.commit against `git rev-parse --short HEAD`, or add `make binary-drift` alongside `make skill-drift`.

PD-2 — the sanctioned poison test has no sanctioned recovery test. `make scratch-poison` exists; nothing lets the auditor exercise the .pqlignore recovery the skill documents, because writing a file needs shell redirection. Reading the scratch vault is also denied — it sits outside the session''s read roots — so warning W2 went unchecked and finding #23 could not be resolved to pql''s fault or the .base file''s. Fix: `make scratch-ignore PATTERN=...`, and add the scratch vault to the audit''s allowed read roots.

PD-3 — no safe way to exercise `pql init`. Documented command, documented side effects, and neither the skill nor --help says whether it targets cwd or --vault, so running it risks writing into this repo. Fix: `make scratch-init`, or pin the target directory in the skill.

PD-4 — exit codes are only observable when non-zero, because appending a semicolon and an echo of the status variable makes the invocation a compound, which is refused. A successful exit 0 is inferred from the absence of an error banner. Fine in practice, but R2 is load-bearing and is verified by inference; say so in the procedure so future auditors do not spend calls rediscovering it.

PD-5 — step 4 says read the source and locate each fix; the tasking said not to read source at all. The auditor honoured the tasking, so no finding carries a source location. The two documents must agree. Worth deciding deliberately: the no-source rule is what keeps the outside-in view honest, and step 4 may simply belong to whoever triages the report rather than to the auditor.

PD-6 — the report cannot be written where it was asked for. The agent has no write tool by design, redirection is denied, and the target path is outside its read roots. It returned the report as a message and the orchestrating session transcribed it. Fix: either drop the write-to-file instruction from the tasking, or add a `make audit-report` target reading stdin.

PD-7 — the cost estimate is low. The procedure predicts roughly 100k tokens and 90+ tool calls; this run made ~130 pql invocations plus ~14 help and make calls, with 26 warnings checked. State 130+ if the core set, a full warnings pass and real exploration are all expected.

Filing note, and an eighth defect by demonstration: the first attempt at this description was written with backticks around a shell example, and bash ran it as command substitution — so the ticket landed with the string 127 where the example should have been. Exactly the anti-pattern the skill under audit warns about, committed while filing the audit''s own findings. Use a quoted heredoc through --stdin for any description containing shell syntax.', NULL, '2026-08-08 18:08:24', '2026-08-08 18:08:24.942', '2026-08-08 18:08:24.942', NULL, 'ef6bb24143854ed660cb86b6af191460', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52S461Z9QKWVK0JTJEFKN8', 'parent_id', NULL, 'T-89', NULL, '2026-08-08 18:08:53', '2026-08-08 18:08:53.660', '2026-08-08 18:08:53.660', NULL, 'e5cb1ab3d3a0e1f283079134114c56af', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53BSK27Y4K571GPGD3JA2R', 'parent_id', NULL, 'T-89', NULL, '2026-08-08 18:08:53', '2026-08-08 18:08:53.667', '2026-08-08 18:08:53.667', NULL, '0b9f8c767e8d5d75a5f33bdf537f8d10', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53V23XT3Y1NBT9Y4E0R438', 'parent_id', NULL, 'T-89', NULL, '2026-08-08 18:08:53', '2026-08-08 18:08:53.667', '2026-08-08 18:08:53.667', NULL, '28266b12a794c066a6c752980b0efc3a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY541P00NKK9Q7PD5CP02PTM', 'status', 'backlog', 'ready', NULL, '2026-08-08 18:08:53', '2026-08-08 18:08:53.683', '2026-08-08 18:08:53.683', NULL, 'b80e0adfe369cd356042c50035ded4b4', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52S461Z9QKWVK0JTJEFKN8', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.828', '2026-08-08 18:10:43.828', NULL, 'e6ac75b57014d3a06d0d173e2802fb66', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52TK0B80DXVEKGVGBRMNVR', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.834', '2026-08-08 18:10:43.834', NULL, '880f27a873ef74741dbcdf2e37f6be64', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52Y65C1K85YMW8HAJS831G', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.835', '2026-08-08 18:10:43.835', NULL, 'c39f3382f4aae670058ff7dc4b6a041f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52VTK08206BJ7Z21JMZRQM', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.835', '2026-08-08 18:10:43.835', NULL, 'cd6786b5430e4c8e2326255ac1dd4e84', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52X079W1G0SPCBAVPYABXR', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.835', '2026-08-08 18:10:43.835', NULL, 'f435fbd51f0d69d54a55ea3734b0f665', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.836', '2026-08-08 18:10:43.836', NULL, '2d3337dd33828a9565b7ffc42057d853', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY538NZECFSTEWA3831TA410', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.836', '2026-08-08 18:10:43.836', NULL, '646056e9161445718538cad39989a423', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY537DDR64R4DYT04X86JWZW', 'status', 'ready', 'in_progress', NULL, '2026-08-08 18:10:43', '2026-08-08 18:10:43.836', '2026-08-08 18:10:43.836', NULL, '7f91acc03a2174fe5d7d81defb6fc7c5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52TK0B80DXVEKGVGBRMNVR', 'description', 'Audit finding Wrong #1; confirms leads C1 and C2.

The Contracts section enumerates the verbs returning a single object: ticket show <one-id>, decisions show, plan status, plan whatsnext, doctor, version --build-info, watch status, ticket new. Two defects in one list.

C1: decisions show is listed unqualified while ticket show gets <one-id>, and the Decisions section says the opposite — one id returns an object, several return an array. Verified: decisions show D-1,D-2 --fields id returns an array. Batched decisions show is new in 2.1.0 (T-76), so this is a stale sentence the rewrite missed.

C2: plan review is absent, though plan whatsnext is listed and the two are described identically.

Beyond the leads, all of these return objects and none is listed: meta <path>, decisions read, decisions claim, decisions validate, decisions sync, ticket refine next, ticket assign, ticket label, plan export, plan rebuild, plan upgrade. meta is the omission that matters most — it is a core vault verb in the routing table.

The section is written in a register that invites the caller to treat it as exhaustive (''Do not write one parser assuming an array''), which is what makes an incomplete enumeration worse than none.

Fix: replace the enumeration with a rule — list and query verbs return arrays, everything else returns an object — plus the genuine exceptions. An enumeration assembled from the verbs the author had in mind will drift again on the next verb added.', 'Audit finding Wrong #1; confirms leads C1 and C2.

The Contracts section enumerates the verbs returning a single object: ticket show <one-id>, decisions show, plan status, plan whatsnext, doctor, version --build-info, watch status, ticket new. Two defects in one list.

C1: decisions show is listed unqualified while ticket show gets <one-id>, and the Decisions section says the opposite — one id returns an object, several return an array. Verified: decisions show D-1,D-2 --fields id returns an array. Batched decisions show is new in 2.1.0 (T-76), so this is a stale sentence the rewrite missed.

C2: plan review is absent, though plan whatsnext is listed and the two are described identically.

Beyond the leads, all of these return objects and none is listed: meta <path>, decisions read, decisions claim, decisions validate, decisions sync, ticket refine next, ticket assign, ticket label, plan export, plan rebuild, plan upgrade. meta is the omission that matters most — it is a core vault verb in the routing table.

The section is written in a register that invites the caller to treat it as exhaustive (''Do not write one parser assuming an array''), which is what makes an incomplete enumeration worse than none.

Fix: replace the enumeration with a rule — list and query verbs return arrays, everything else returns an object — plus the genuine exceptions. An enumeration assembled from the verbs the author had in mind will drift again on the next verb added.

Fixed 2026-08-08 in 2.1.0. The enumeration is gone, replaced by a rule: verbs that answer ''which things'' return an array, everything else returns an object, with the array verbs named. An enumeration assembled from the verbs the author had in mind drifts on the next verb added — this one had already missed eleven. Two edges are now stated explicitly rather than left to the reader: the batching verbs switch shape (one id gives an object, several give an array, which is what C1 caught), and plan whatsnext / plan review return {message: ...} instead of a record when there is nothing to hand back, so reaching for .id gets nothing and no error. That second one was audit finding #12 and is folded in here rather than deferred, since it is the same defect — a caller told the shape is an object still cannot parse it.', NULL, '2026-08-08 18:22:31', '2026-08-08 18:22:31.248', '2026-08-08 18:22:31.248', NULL, '22dce542f709824326b727a3e5795169', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52VTK08206BJ7Z21JMZRQM', 'description', 'Audit finding Wrong #2.

The skill states, and docs/output-contract.md now states as binding on every surface: empty fields are omitted from JSON rather than set to null; check for presence, not for null. Two shipped verbs emit null.

  query ''SELECT name, fm.lens LIMIT 2''  ->  [{''fm.lens'':null,''name'':''NOTE''}, ...]
  plan export                           ->  {''files_written'':null,''rows_written'':0}

A caller obeying the documented rule reads fm.lens as present on a file carrying no lens key.

Correction to record: when T-75 was closed I claimed the whole output surface had been audited and doctor.index was the only violation. That was wrong twice over. The audit was a grep over Go struct tags, so it could not see the DSL, which builds dynamic maps rather than structs. And FilesWritten []string appeared in that same grep output and was dismissed without testing the zero-write case. Generalising from the cases checked is shape E, which this repo''s own audit procedure names.

Two-part fix. Code: omitempty on the changelog result structs — plan export''s FilesWritten, and check the siblings in rebuild.go, importer.go and collision.go for the same shape. Skill and output-contract: the DSL null is arguably correct and should stay, since SELECT fm.lens over files that lack the key needs to distinguish absent from empty — so scope the rule rather than claim a universal that is not true.', 'Audit finding Wrong #2.

The skill states, and docs/output-contract.md now states as binding on every surface: empty fields are omitted from JSON rather than set to null; check for presence, not for null. Two shipped verbs emit null.

  query ''SELECT name, fm.lens LIMIT 2''  ->  [{''fm.lens'':null,''name'':''NOTE''}, ...]
  plan export                           ->  {''files_written'':null,''rows_written'':0}

A caller obeying the documented rule reads fm.lens as present on a file carrying no lens key.

Correction to record: when T-75 was closed I claimed the whole output surface had been audited and doctor.index was the only violation. That was wrong twice over. The audit was a grep over Go struct tags, so it could not see the DSL, which builds dynamic maps rather than structs. And FilesWritten []string appeared in that same grep output and was dismissed without testing the zero-write case. Generalising from the cases checked is shape E, which this repo''s own audit procedure names.

Two-part fix. Code: omitempty on the changelog result structs — plan export''s FilesWritten, and check the siblings in rebuild.go, importer.go and collision.go for the same shape. Skill and output-contract: the DSL null is arguably correct and should stay, since SELECT fm.lens over files that lack the key needs to distinguish absent from empty — so scope the rule rather than claim a universal that is not true.

Fixed 2026-08-08 in 2.1.0, both halves. Code: Export and Import now initialise their file-list slices, so plan export, plan import and plan rebuild report [] rather than null. Checked the siblings — RebuildResult builds from a make() slice and Import''s result, UpgradeResult already used omitempty, TicketCollision.Records is only constructed non-empty. Chose [] over omitempty deliberately: ''nothing was written'' is the answer to what the verb was asked, so the key stays present. Skill and output-contract: the universal claim was replaced with the invariant that actually holds — never null — plus the two forms empty takes. Scalars are omitted, collections are present and empty. meta has returned tags: [] all along, so ''check for presence, not for null'' was wrong even before this. The DSL null is documented as a deliberate exception and kept: a SELECT over files where some carry a frontmatter key and some do not has to distinguish absent from empty, and omitting the key would make the rows ragged.', NULL, '2026-08-08 18:22:31', '2026-08-08 18:22:31.266', '2026-08-08 18:22:31.266', NULL, 'ec01f9a8bd349152c8cd7e408276c82a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52X079W1G0SPCBAVPYABXR', 'description', 'Audit finding Wrong #3.

The skill claims backlinks ''accepts any spelling of the file — full path, extensionless, or bare basename — and matches links written in any of them''. It does not.

Verified in this repo: backlinks governance/decisions/architecture.md returns 1 row, while backlinks decisions/architecture.md returns 28. The correct vault-relative path — the spelling every other pql command accepts and returns — largely misses, while a path that does not exist as a file succeeds. In the audit''s scratch vault the bare basename never worked at all: backlinks persona, backlinks architecture, backlinks koskela/persona all returned [].

The failing case is an ordinary sibling-relative markdown link written from a subdirectory index — the most common shape in a governance/README.md — and the skill''s caveat only names ../-prefixed links. So the caveat is narrower than the defect.

This is the documentation half of T-72. That fix added the extensionless-full-path form and was correct as far as it went; the claim written alongside it overstated the result. Resolution to a real path is Q-6, tracked separately and not attempted here.

Fix, skill-side: describe what actually happens — the query is matched against link spellings as written — and give the recovery route, which is to run outlinks on a file expected to link there and query the exact target string it returns. That route works and is undiscoverable from the current text.', 'Audit finding Wrong #3.

The skill claims backlinks ''accepts any spelling of the file — full path, extensionless, or bare basename — and matches links written in any of them''. It does not.

Verified in this repo: backlinks governance/decisions/architecture.md returns 1 row, while backlinks decisions/architecture.md returns 28. The correct vault-relative path — the spelling every other pql command accepts and returns — largely misses, while a path that does not exist as a file succeeds. In the audit''s scratch vault the bare basename never worked at all: backlinks persona, backlinks architecture, backlinks koskela/persona all returned [].

The failing case is an ordinary sibling-relative markdown link written from a subdirectory index — the most common shape in a governance/README.md — and the skill''s caveat only names ../-prefixed links. So the caveat is narrower than the defect.

This is the documentation half of T-72. That fix added the extensionless-full-path form and was correct as far as it went; the claim written alongside it overstated the result. Resolution to a real path is Q-6, tracked separately and not attempted here.

Fix, skill-side: describe what actually happens — the query is matched against link spellings as written — and give the recovery route, which is to run outlinks on a file expected to link there and query the exact target string it returns. That route works and is undiscoverable from the current text.

Fixed 2026-08-08 in 2.1.0, documentation only — the resolution defect itself is Q-6 and untouched. The claim that backlinks ''accepts any spelling'' is replaced by what actually happens: it compares spellings, trying the path given, that path without .md, the bare basename, and each with an anchor. The failing case is now named specifically, because it is the common one rather than the exotic one the old caveat described: a file at governance/README.md writing a directory-relative link is not found by querying the correct vault-relative path. And the recovery route is given, which is the part that was missing entirely — run outlinks on a file expected to link there, read the raw target, query that string. Verified live: outlinks governance/README.md returns targets spelled decisions/architecture.md#anchor, which is exactly the string backlinks wants. Two related findings folded in rather than deferred, since they are the same trap seen from the other side. outlinks now has its row shape documented ({target, alias, line, via}) with the explicit warning that target is raw link text, plus the append-.md hop and what to do when that still 66s. And backlinks is described as one row per link occurrence, not per file, so counting linking files needs a dedupe — 28 rows for one file surprised the auditor. Also noted: context does resolve outbound targets against the index (T-73), so it is the verb to prefer when real paths are wanted.', NULL, '2026-08-08 18:22:45', '2026-08-08 18:22:45.383', '2026-08-08 18:22:45.383', NULL, '1a86663d3da4d3e9bf33a3ec497abe0a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52Y65C1K85YMW8HAJS831G', 'description', 'Audit finding Wrong #4. Third time this passage has been wrong.

The skill gives a diagnostic: ''in a link-sparse vault centrality is 0 on every candidate, so recency decides... a score exactly equal to the recency weight means nothing else contributed.'' The named per-command weights are all correct. What is missing is that recency is 0.25 on search but 0.05 on related and context.

The auditor hit it live: related returned three rows at exactly 0.2500, applied the skill''s rule, and concluded nothing but recency contributed. --full showed the opposite — path_proximity weighted 0.20, recency 0.05. The rule produced a confidently wrong reading of correct output, which is worse than no rule.

History worth recording, because the passage keeps failing the same way: the first audit found the weights section wrong, the correction named centrality but missed that recency decides in link-sparse vaults, and this correction adds that the recency weight is not one number. Each fix has been a patch to a numeric shortcut that keeps not surviving contact with a real vault.

Fix: drop the numeric shortcut. Say ''run --full and read signals[]'' — which is now one flag away by design (T-74) and cannot go stale. If a per-command number is kept, state all three.', 'Audit finding Wrong #4. Third time this passage has been wrong.

The skill gives a diagnostic: ''in a link-sparse vault centrality is 0 on every candidate, so recency decides... a score exactly equal to the recency weight means nothing else contributed.'' The named per-command weights are all correct. What is missing is that recency is 0.25 on search but 0.05 on related and context.

The auditor hit it live: related returned three rows at exactly 0.2500, applied the skill''s rule, and concluded nothing but recency contributed. --full showed the opposite — path_proximity weighted 0.20, recency 0.05. The rule produced a confidently wrong reading of correct output, which is worse than no rule.

History worth recording, because the passage keeps failing the same way: the first audit found the weights section wrong, the correction named centrality but missed that recency decides in link-sparse vaults, and this correction adds that the recency weight is not one number. Each fix has been a patch to a numeric shortcut that keeps not surviving contact with a real vault.

Fix: drop the numeric shortcut. Say ''run --full and read signals[]'' — which is now one flag away by design (T-74) and cannot go stale. If a per-command number is kept, state all three.

Fixed 2026-08-08 in 2.1.0 by deleting the shortcut rather than correcting it a third time. The passage now says: if results look arbitrary, re-run with --full and read signals[], and do not infer mechanism from the score. It states that a score equal to one weight does not mean that signal decided it, and gives the per-command difference (recency 0.25 on search, 0.05 on related and context) as the reason not to trust the arithmetic. Rationale for removal over repair: this is the third correction to the same numeric heuristic. Audit one found the weights section wrong; the fix named centrality but missed that recency decides in link-sparse vaults; audit three found the recency weight is not one number. Each repair kept the shape that keeps failing — a derived rule that has to be re-derived whenever a weight profile changes. signals[] is one flag away by design since T-74 and cannot go stale, so pointing at it is strictly better than any number in prose.', NULL, '2026-08-08 18:22:45', '2026-08-08 18:22:45.402', '2026-08-08 18:22:45.402', NULL, 'ad0ce70075b82a27ceac3d10a506d783', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'description', 'Audit findings Wrong #5 and Missing #9.

  ticket show T-63 --with-context --fields id,title
  -> {"id":"T-63","title":"..."}    exit 0, no ancestors, no warning

  ticket show T-63 --with-context --fields id,ancestors
  -> {"id":"T-63","ancestors":[...]}

So --fields does select the joins; a join key not named is dropped even though its --with- flag was passed. The skill says --fields ''narrows the top level only: the join-trees are all-or-nothing'', which describes what happens to a join that survives but reads as a promise the join arrives whole, and gives no hint a lever exists.

Compounded by Missing #9: the skill''s --fields valid-name lists give 13 ticket fields and 8 decision fields, while the exit-64 error lists 19. ancestors, blockers, children, decisions, subtree and message are all absent from the skill — and that section opens with ''Valid field names, since guessing them costs a round trip''.

Failure mode: ask for context, ask to trim, get neither, notice nothing. Both halves are from T-67, shipped the same day.

Fix, skill-side: state that --fields also selects joins and that an unnamed join key is dropped, list the join keys, and keep the true part — join contents are never projected. Decide deliberately whether passing --with-X while omitting X from --fields warrants a stderr warning; a silent drop of something explicitly requested is what diagnostics are for, but warning on a legitimate top-level-only call would be noise.

Filing note: this ticket could not be created with its title as a positional argument — a title beginning with -- is parsed as a flag and exits 64. `ticket new bug -- "--fields ..."` works. Worth a line in the skill.', 'Audit findings Wrong #5 and Missing #9.

  ticket show T-63 --with-context --fields id,title
  -> {"id":"T-63","title":"..."}    exit 0, no ancestors, no warning

  ticket show T-63 --with-context --fields id,ancestors
  -> {"id":"T-63","ancestors":[...]}

So --fields does select the joins; a join key not named is dropped even though its --with- flag was passed. The skill says --fields ''narrows the top level only: the join-trees are all-or-nothing'', which describes what happens to a join that survives but reads as a promise the join arrives whole, and gives no hint a lever exists.

Compounded by Missing #9: the skill''s --fields valid-name lists give 13 ticket fields and 8 decision fields, while the exit-64 error lists 19. ancestors, blockers, children, decisions, subtree and message are all absent from the skill — and that section opens with ''Valid field names, since guessing them costs a round trip''.

Failure mode: ask for context, ask to trim, get neither, notice nothing. Both halves are from T-67, shipped the same day.

Fix, skill-side: state that --fields also selects joins and that an unnamed join key is dropped, list the join keys, and keep the true part — join contents are never projected. Decide deliberately whether passing --with-X while omitting X from --fields warrants a stderr warning; a silent drop of something explicitly requested is what diagnostics are for, but warning on a legitimate top-level-only call would be noise.

Filing note: this ticket could not be created with its title as a positional argument — a title beginning with -- is parsed as a flag and exits 64. `ticket new bug -- "--fields ..."` works. Worth a line in the skill.

Fixed 2026-08-08 in 2.1.0, documentation. The skill now states plainly that --fields selects the joins too, with the worked pair showing --with-context --fields id,title returning no ancestors and --fields id,title,ancestors returning them. The true part is kept: you cannot project inside a join, so it is the whole subtree or none of it. What changed is that ''all-or-nothing'' is no longer left to carry a meaning it does not have. Missing #9 fixed alongside, since the two compound: the join keys are now in the valid-name list — ancestors, children, blockers, subtree, decisions and message on tickets, tickets and refs on decisions. That section opens by promising to save a round trip and was omitting six of nineteen names. Also added: projection does not reach the mutation verbs, which reject --fields at 64 and several of which return the whole record including description (audit findings #13 and #25, folded in as one line here rather than deferred, because a caller budgeting payload needs it in the same place). Decided against a stderr warning when --with-X is passed without X in --fields. It would fire on the legitimate ''I want the top level only'' call, which is common, and the skill line costs nothing and misfires never. Revisit if the silent drop is reported again by someone who had read the fixed text.', NULL, '2026-08-08 18:22:58', '2026-08-08 18:22:58.701', '2026-08-08 18:22:58.701', NULL, 'da20e8fa09416a4aaca748904c392cfb', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY537DDR64R4DYT04X86JWZW', 'description', 'Audit finding Wrong #6.

  pql query --help
  Exit codes follow docs/output-contract.md: 0 with results, 2 with zero
  matches, 65 on parse/compile errors ...

D-22 retired exit 2; zero matches is exit 0 with an empty array, and both the skill and the output-contract doc say so. The binary contradicts them on demand.

Why this is Wrong rather than a stray doc nit: the skill tells an agent that --help is the way to fill a gap in its own coverage. An agent that does so here is told to treat every empty result as a distinct failure code — which is precisely the misreading D-22 existed to prevent, delivered by the tool itself.

Fix: correct the Long text on the query command. Then grep every other Long/Short string for exit-code claims, since one stale copy implies the others were never swept when D-22 landed.', 'Audit finding Wrong #6.

  pql query --help
  Exit codes follow docs/output-contract.md: 0 with results, 2 with zero
  matches, 65 on parse/compile errors ...

D-22 retired exit 2; zero matches is exit 0 with an empty array, and both the skill and the output-contract doc say so. The binary contradicts them on demand.

Why this is Wrong rather than a stray doc nit: the skill tells an agent that --help is the way to fill a gap in its own coverage. An agent that does so here is told to treat every empty result as a distinct failure code — which is precisely the misreading D-22 existed to prevent, delivered by the tool itself.

Fix: correct the Long text on the query command. Then grep every other Long/Short string for exit-code claims, since one stale copy implies the others were never swept when D-22 landed.

Fixed 2026-08-08 in 2.1.0. The query command''s Long text no longer claims exit 2 for zero matches; it states exit 0 including zero matches, citing D-22, and 65 for parse, unknown-column and evaluation errors. Swept the rest of the CLI for the same staleness — grep for exit-code claims across every Long and Short string found this as the only instance, so D-22''s cleanup missed exactly one site. Took the opportunity to make the same help text carry the DSL vocabulary (T-86), since an agent that consults --help to fill a gap in the skill should find the answer rather than a second stale document.', NULL, '2026-08-08 18:22:58', '2026-08-08 18:22:58.720', '2026-08-08 18:22:58.720', NULL, '6e5cdbbc5b453ff6f3f0e6fef7c3545f', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY538NZECFSTEWA3831TA410', 'description', 'Audit findings Missing #7 and #8. The auditor calls this the omission blocking the most task classes.

The skill documents the DSL as: ''fm.<key> reads frontmatter; tags, path and name are built in.'' All of these also work and none is mentioned:

  query ''SELECT name, folder WHERE folder = "members/vaasa"''
  query ''SELECT path WHERE "Mari Koskela" IN headings''
  query ''SELECT path, size, mtime WHERE size > 50000''
  query ''SELECT DISTINCT fm.type''
  query ''SELECT path WHERE path LIKE "members/%"''

folder even appears in query --help''s own example. So ''notes in this folder'' and ''notes with this heading'' look impossible from the skill alone.

DISTINCT matters most: it is the single-call answer to ''which values does this frontmatter key actually take'', which the skill documents no route for at all. schema gives keys, types and counts — never values. An agent following the skill either gives up or selects every row and folds client-side. The auditor guessed DISTINCT; it should not have had to.

Worth stating too: COUNT(*) and GROUP BY are correctly unsupported and fail loudly at exit 65. Saying so saves a caller the attempt.

Second half, discoverability. Unlike --fields, the unknown-column error is a dead end:

  query ''SELECT bogus_column''
  -> pql.eval.unknown_column at line 1, col 8: unknown column "bogus_column"

and the only pointer is docs/pql-grammar.md, which is not shipped to consumers and which a consuming agent cannot read. Make the error list the valid set, the way the --fields error does — the audit singles that one out as ''genuinely a usable lookup''.', 'Audit findings Missing #7 and #8. The auditor calls this the omission blocking the most task classes.

The skill documents the DSL as: ''fm.<key> reads frontmatter; tags, path and name are built in.'' All of these also work and none is mentioned:

  query ''SELECT name, folder WHERE folder = "members/vaasa"''
  query ''SELECT path WHERE "Mari Koskela" IN headings''
  query ''SELECT path, size, mtime WHERE size > 50000''
  query ''SELECT DISTINCT fm.type''
  query ''SELECT path WHERE path LIKE "members/%"''

folder even appears in query --help''s own example. So ''notes in this folder'' and ''notes with this heading'' look impossible from the skill alone.

DISTINCT matters most: it is the single-call answer to ''which values does this frontmatter key actually take'', which the skill documents no route for at all. schema gives keys, types and counts — never values. An agent following the skill either gives up or selects every row and folds client-side. The auditor guessed DISTINCT; it should not have had to.

Worth stating too: COUNT(*) and GROUP BY are correctly unsupported and fail loudly at exit 65. Saying so saves a caller the attempt.

Second half, discoverability. Unlike --fields, the unknown-column error is a dead end:

  query ''SELECT bogus_column''
  -> pql.eval.unknown_column at line 1, col 8: unknown column "bogus_column"

and the only pointer is docs/pql-grammar.md, which is not shipped to consumers and which a consuming agent cannot read. Make the error list the valid set, the way the --fields error does — the audit singles that one out as ''genuinely a usable lookup''.

Fixed 2026-08-08 in 2.1.0, both halves. Documentation: the skill''s DSL section now lists every column — path, name, folder, size, mtime, ctime, content_hash, last_scanned, fm.<key> — and names tags, headings, inlinks and outlinks as array columns usable only on the right of IN. Operators are listed including LIKE, BETWEEN and DISTINCT, with the explicit note that COUNT and GROUP BY are unsupported and fail at 65, so a caller does not spend a call finding out. SELECT DISTINCT fm.<key> gets its own callout as the single-call route to ''which values does this key take'', which had no documented route at all, and a row in the Choosing a command table pointing away from pql schema, which reports types and counts rather than values. Discoverability: the unknown-column error now lists the vocabulary rather than repeating the bad name back. That error was the only discovery route a consuming agent had — docs/pql-grammar.md is not shipped with the binary — and it was a dead end. It now reads like the --fields error, which the audit singled out as genuinely usable. Guarded by TestFileColumns_MatchesErrorList, which checks both directions: every name the error advertises is accepted by fileColumn, and a set of plausible additions is either rejected or listed. A switch cannot be enumerated by reflection, so the second direction probes candidates rather than proving completeness — imperfect, but it catches the realistic case of a column added without a line in the list. Same help text also fixed for T-85.', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.215', '2026-08-08 18:23:17.215', NULL, '197e70d33f84c172b1916f9124269634', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52TK0B80DXVEKGVGBRMNVR', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.260', '2026-08-08 18:23:17.260', NULL, 'f1f68d9ea4a0f458ff534ce6148b15e3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52VTK08206BJ7Z21JMZRQM', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.262', '2026-08-08 18:23:17.262', NULL, '36ff02ff229840b00fe9a5031d9de12b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52X079W1G0SPCBAVPYABXR', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.263', '2026-08-08 18:23:17.263', NULL, 'd880621eeb8bc8ca509a62cf2453160b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53064TNJBNG3SAW3HJ438G', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.264', '2026-08-08 18:23:17.264', NULL, '9ec9e24e22ab646fa33211a06c172a0a', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52Y65C1K85YMW8HAJS831G', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.264', '2026-08-08 18:23:17.264', NULL, 'ec1af0e47db52e2844875687ec3b9ae6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY537DDR64R4DYT04X86JWZW', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.265', '2026-08-08 18:23:17.265', NULL, '2882d17820c4109ab661c3b4ed607642', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY538NZECFSTEWA3831TA410', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.266', '2026-08-08 18:23:17.266', NULL, 'bc353bcefb50c2c936e6b810d14cacb6', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52S461Z9QKWVK0JTJEFKN8', 'description', 'Raised 2026-08-08 by the third run of the pql-skill-auditor against 2.1.0 — the first run that executed (the two before it aborted on permission denials, which turned out to be workspace trust never having been accepted for this repo).

A real audit: 15/15 core retrievals, 44 exploratory, 26 warnings checked, ~130 pql invocations. Verdict: not fit to ship as it stands, but close. 26 findings — 6 Wrong, 15 Missing, 5 Unusable, 0 Bloat — plus 7 procedure defects, and all six leads from the aborted run''s static read confirmed.

The pattern worth recording: three of the six Wrong findings are defects in work shipped the same day, by the author who then documented it. T-72''s backlinks fix, T-67''s --fields on the show verbs, and T-75''s omitted-not-null audit each shipped with a skill claim that live use disproves. That is the case for the gate existing, and the case for it running on an agent that has not read the source.

This epic covers the fix set the verdict names, plus the DSL documentation gap the auditor calls the omission blocking the most task classes. The remaining Missing and Unusable findings are filed separately for 2.2.0.

Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md', 'Raised 2026-08-08 by the third run of the pql-skill-auditor against 2.1.0 — the first run that executed (the two before it aborted on permission denials, which turned out to be workspace trust never having been accepted for this repo).

A real audit: 15/15 core retrievals, 44 exploratory, 26 warnings checked, ~130 pql invocations. Verdict: not fit to ship as it stands, but close. 26 findings — 6 Wrong, 15 Missing, 5 Unusable, 0 Bloat — plus 7 procedure defects, and all six leads from the aborted run''s static read confirmed.

The pattern worth recording: three of the six Wrong findings are defects in work shipped the same day, by the author who then documented it. T-72''s backlinks fix, T-67''s --fields on the show verbs, and T-75''s omitted-not-null audit each shipped with a skill claim that live use disproves. That is the case for the gate existing, and the case for it running on an agent that has not read the source.

This epic covers the fix set the verdict names, plus the DSL documentation gap the auditor calls the omission blocking the most task classes. The remaining Missing and Unusable findings are filed separately for 2.2.0.

Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

Closed 2026-08-08. All seven delivered in 2.1.0 and each verified against the installed binary rather than the edit: plan export returns [] not null, the DSL still returns null by design, SELECT DISTINCT works and is documented, query --help no longer mentions exit 2, --with-context --fields id,ancestors returns ancestors, and outlinks governance/README.md returns the raw spelling that the backlinks recovery route tells you to query. make skill-drift agrees across all three copies at adb43628. Two judgement calls worth recording. Several findings filed against T-87 for 2.2.0 were folded in here instead, where they were the same defect seen from another angle rather than separate work: #12 (plan whatsnext/review message shape) went into the output-shape rule, #10 and #11 (outlinks shape, backlinks per-occurrence rows) into the link-matching section, #13 and #25 (mutation verbs return whole records, reject --fields) into the projection section as one line. Deferring those would have left the corrected sections still incomplete on the exact question a caller was asking. And T-83 was fixed by deleting the heuristic rather than correcting it, on the third correction to the same passage — the shape that keeps failing is a derived rule that must be re-derived whenever a weight changes.', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.281', '2026-08-08 18:23:17.281', NULL, 'a8ea775044a7d021e37daff743acc4e8', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY52S461Z9QKWVK0JTJEFKN8', 'status', 'in_progress', 'done', NULL, '2026-08-08 18:23:17', '2026-08-08 18:23:17.295', '2026-08-08 18:23:17.295', NULL, '2b0bd370278d00f52a84a45253462106', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53V23XT3Y1NBT9Y4E0R438', 'description', 'The pql-testing procedure''s own defects, found by the third audit run while following it. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

PD-1 — the setup''s version check cannot detect the thing it exists to detect. It compares `pql --version` against `./bin/pql --version` and says to reinstall if they differ. Both printed 2.1.0 while `doctor` showed the PATH binary was built at 951e4f9 and ./bin/pql at 1159a8f. Because version strings are deliberately clean with no SHA (D-12), version equality is the wrong comparison — two binaries from different commits agree whenever the release version has not been bumped. Here the intervening commits touched only the audit skill, but the check would have passed just as happily across a functional change. Fix: compare `doctor`''s version.commit against `git rev-parse --short HEAD`, or add `make binary-drift` alongside `make skill-drift`.

PD-2 — the sanctioned poison test has no sanctioned recovery test. `make scratch-poison` exists; nothing lets the auditor exercise the .pqlignore recovery the skill documents, because writing a file needs shell redirection. Reading the scratch vault is also denied — it sits outside the session''s read roots — so warning W2 went unchecked and finding #23 could not be resolved to pql''s fault or the .base file''s. Fix: `make scratch-ignore PATTERN=...`, and add the scratch vault to the audit''s allowed read roots.

PD-3 — no safe way to exercise `pql init`. Documented command, documented side effects, and neither the skill nor --help says whether it targets cwd or --vault, so running it risks writing into this repo. Fix: `make scratch-init`, or pin the target directory in the skill.

PD-4 — exit codes are only observable when non-zero, because appending a semicolon and an echo of the status variable makes the invocation a compound, which is refused. A successful exit 0 is inferred from the absence of an error banner. Fine in practice, but R2 is load-bearing and is verified by inference; say so in the procedure so future auditors do not spend calls rediscovering it.

PD-5 — step 4 says read the source and locate each fix; the tasking said not to read source at all. The auditor honoured the tasking, so no finding carries a source location. The two documents must agree. Worth deciding deliberately: the no-source rule is what keeps the outside-in view honest, and step 4 may simply belong to whoever triages the report rather than to the auditor.

PD-6 — the report cannot be written where it was asked for. The agent has no write tool by design, redirection is denied, and the target path is outside its read roots. It returned the report as a message and the orchestrating session transcribed it. Fix: either drop the write-to-file instruction from the tasking, or add a `make audit-report` target reading stdin.

PD-7 — the cost estimate is low. The procedure predicts roughly 100k tokens and 90+ tool calls; this run made ~130 pql invocations plus ~14 help and make calls, with 26 warnings checked. State 130+ if the core set, a full warnings pass and real exploration are all expected.

Filing note, and an eighth defect by demonstration: the first attempt at this description was written with backticks around a shell example, and bash ran it as command substitution — so the ticket landed with the string 127 where the example should have been. Exactly the anti-pattern the skill under audit warns about, committed while filing the audit''s own findings. Use a quoted heredoc through --stdin for any description containing shell syntax.', 'The pql-testing procedure''s own defects, found by the third audit run while following it. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

PD-1 — the setup''s version check cannot detect the thing it exists to detect. It compares `pql --version` against `./bin/pql --version` and says to reinstall if they differ. Both printed 2.1.0 while `doctor` showed the PATH binary was built at 951e4f9 and ./bin/pql at 1159a8f. Because version strings are deliberately clean with no SHA (D-12), version equality is the wrong comparison — two binaries from different commits agree whenever the release version has not been bumped. Here the intervening commits touched only the audit skill, but the check would have passed just as happily across a functional change. Fix: compare `doctor`''s version.commit against `git rev-parse --short HEAD`, or add `make binary-drift` alongside `make skill-drift`.

PD-2 — the sanctioned poison test has no sanctioned recovery test. `make scratch-poison` exists; nothing lets the auditor exercise the .pqlignore recovery the skill documents, because writing a file needs shell redirection. Reading the scratch vault is also denied — it sits outside the session''s read roots — so warning W2 went unchecked and finding #23 could not be resolved to pql''s fault or the .base file''s. Fix: `make scratch-ignore PATTERN=...`, and add the scratch vault to the audit''s allowed read roots.

PD-3 — no safe way to exercise `pql init`. Documented command, documented side effects, and neither the skill nor --help says whether it targets cwd or --vault, so running it risks writing into this repo. Fix: `make scratch-init`, or pin the target directory in the skill.

PD-4 — exit codes are only observable when non-zero, because appending a semicolon and an echo of the status variable makes the invocation a compound, which is refused. A successful exit 0 is inferred from the absence of an error banner. Fine in practice, but R2 is load-bearing and is verified by inference; say so in the procedure so future auditors do not spend calls rediscovering it.

PD-5 — step 4 says read the source and locate each fix; the tasking said not to read source at all. The auditor honoured the tasking, so no finding carries a source location. The two documents must agree. Worth deciding deliberately: the no-source rule is what keeps the outside-in view honest, and step 4 may simply belong to whoever triages the report rather than to the auditor.

PD-6 — the report cannot be written where it was asked for. The agent has no write tool by design, redirection is denied, and the target path is outside its read roots. It returned the report as a message and the orchestrating session transcribed it. Fix: either drop the write-to-file instruction from the tasking, or add a `make audit-report` target reading stdin.

PD-7 — the cost estimate is low. The procedure predicts roughly 100k tokens and 90+ tool calls; this run made ~130 pql invocations plus ~14 help and make calls, with 26 warnings checked. State 130+ if the core set, a full warnings pass and real exploration are all expected.

Filing note, and an eighth defect by demonstration: the first attempt at this description was written with backticks around a shell example, and bash ran it as command substitution — so the ticket landed with the string 127 where the example should have been. Exactly the anti-pattern the skill under audit warns about, committed while filing the audit''s own findings. Use a quoted heredoc through --stdin for any description containing shell syntax.

Closed 2026-08-08. All seven, plus the eighth found while filing them.

PD-1: make binary-drift compares the ldflags commit stamp against HEAD, and make audit-ready runs it beside skill-drift. Demonstrated itself on first run — the installed binary was stamped 1159a8f against HEAD bfea7d8 while both --version outputs agreed on 2.1.0, which is exactly the case the old check could not see. Also flags an uncommitted working tree, since anything edited and not rebuilt is not in the binary under test.

PD-2: make scratch-ignore PATTERN=... Verified the full loop the audit could not: poison the vault, every command fails at 70, add the pattern, tags returns rows again. So warning W2 is now checkable and checks out. The read-roots half is a session-config matter, not a repo one, and is left to whoever launches the audit.

PD-3: make scratch-init aims pql init at the scratch vault, and the procedure says not to run it any other way.

PD-4: the procedure now says exit 0 is inferred from the absence of an error rather than observed, and to write it that way in the report. R2 turns on the distinction.

PD-5: the read-the-source step is deleted rather than reconciled. It contradicted the rule the whole audit rests on, and an auditor told both things has to pick one. Confirming findings against the implementation is now explicitly the triager''s job. Shape G survives as the one cross-boundary check, satisfied by audit-ready in setup.

PD-6: both the procedure and the agent definition now say the report is the final message and that a path in the tasking cannot be honoured. Removes the instruction that produced the defect rather than adding a target to satisfy it — the agent having no write tool is the design, not the gap.

PD-7: estimate moved to 130-plus calls, from the measured run.

Eighth, unprompted: check-drift.sh was extracting the skill body from JSON with python3 — an undeclared dependency that existed only because there was no way to get a skill as text. pql skill show --raw exists now, so it is a sha256sum of a pipe inside a script, which is where pipes belong.', NULL, '2026-08-08 18:38:07', '2026-08-08 18:38:07.977', '2026-08-08 18:38:07.977', NULL, '41b096af06226a7d4862cf50fc9a0171', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53V23XT3Y1NBT9Y4E0R438', 'status', 'backlog', 'done', NULL, '2026-08-08 18:38:08', '2026-08-08 18:38:08.019', '2026-08-08 18:38:08.019', NULL, '12f81b427d4852167cd474aed6e9d2f4', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53BSK27Y4K571GPGD3JA2R', 'description', 'The Missing and Unusable findings from the 2.1.0 skill audit that were deliberately deferred. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

Deferred because each needs a surface decision rather than a text fix, and rushing those into a patch release is how the audit''s Wrong findings got there in the first place.

Missing, output shapes — the audit''s second conclusion is that output shapes are documented for the planning surface and undocumented for the vault surface: backlinks, outlinks, meta, refs, base and the DSL each get a one-line description of what they mean and nothing about what they return. That asymmetry breaks the second hop of any link-following task.
- #10 outlinks: shape {target,alias,line,via} undocumented, and target is the raw spelling — meta rejects it. outlinks --help says resolution is not applied; the skill does not. Since the skill also advertises backlinks as spelling-tolerant, the natural generalisation is exactly wrong.
- #11 backlinks rows are per-link-occurrence, not per-file. 28 rows for one linking file. Counting linking files needs a client-side dedupe the skill never mentions.
- #12 plan whatsnext and plan review can return {message: ...} instead of a record. A caller reaching for .id gets nothing and no error.
- #13 ticket mutation verbs return undocumented and mutually inconsistent shapes — assign returns a whole record with a 2 KB description, label returns a summary object, and neither accepts --fields.

Missing, surface facts:
- #14 .gitignore is read by default alongside .pqlignore; the skill mentions only the latter. An agent debugging ''why is this file not indexed'' will not think to look.
- #15 the documented PQL_VAULT/PQL_DB/PQL_CONFIG env routes cannot be invoked under the permission model the same skill describes — an env-var assignment is a prefix, so it breaks the allowlist match, and the anti-patterns list names only && || ; |  and >.
- #16 pql init''s target directory is unstated and the skill''s claims about what it writes are contradicted by its own --help. The auditor would not run it, correctly.
- #17 ticket board omits empty columns, so a named column cannot be distinguished from an empty one.
- #18 base --view names are discoverable only by triggering exit 65.
- #19 skill status lines/words and the second bundled skill (clean-house) are undocumented.
- #20 pql completion absent from the skill. Low value, noted for completeness.
- #21 pql base absent from the Choosing a command routing table, as are schema and shell.

Unusable:
- #22 ''which decisions have nothing implementing them'' forces a client-side join. No --unimplemented filter; ticket list --decision none returns empty silently rather than erroring.
- #23 base output carries no path or name, so rows cannot be fed onward. Also fm.voting: 1 where meta returns voting: true — an undocumented bool coercion. The auditor could not determine whether the column set comes from the .base file or from pql, because reading the scratch vault was denied.
- #24 ranked verbs accept a nonexistent path and return [] at exit 0, where meta exits 66. A typo reads as ''nothing is related to this file'' — the shape-B trap the skill devotes a Contracts paragraph to for filter values.
- #25 mutation verbs return whole records with no way to trim: a ten-ticket status change returns ten descriptions.
- #26 backlinks duplicate rows, see #11.', 'The Missing and Unusable findings from the 2.1.0 skill audit that were deliberately deferred. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

Deferred because each needs a surface decision rather than a text fix, and rushing those into a patch release is how the audit''s Wrong findings got there in the first place.

Missing, output shapes — the audit''s second conclusion is that output shapes are documented for the planning surface and undocumented for the vault surface: backlinks, outlinks, meta, refs, base and the DSL each get a one-line description of what they mean and nothing about what they return. That asymmetry breaks the second hop of any link-following task.
- #10 outlinks: shape {target,alias,line,via} undocumented, and target is the raw spelling — meta rejects it. outlinks --help says resolution is not applied; the skill does not. Since the skill also advertises backlinks as spelling-tolerant, the natural generalisation is exactly wrong.
- #11 backlinks rows are per-link-occurrence, not per-file. 28 rows for one linking file. Counting linking files needs a client-side dedupe the skill never mentions.
- #12 plan whatsnext and plan review can return {message: ...} instead of a record. A caller reaching for .id gets nothing and no error.
- #13 ticket mutation verbs return undocumented and mutually inconsistent shapes — assign returns a whole record with a 2 KB description, label returns a summary object, and neither accepts --fields.

Missing, surface facts:
- #14 .gitignore is read by default alongside .pqlignore; the skill mentions only the latter. An agent debugging ''why is this file not indexed'' will not think to look.
- #15 the documented PQL_VAULT/PQL_DB/PQL_CONFIG env routes cannot be invoked under the permission model the same skill describes — an env-var assignment is a prefix, so it breaks the allowlist match, and the anti-patterns list names only && || ; |  and >.
- #16 pql init''s target directory is unstated and the skill''s claims about what it writes are contradicted by its own --help. The auditor would not run it, correctly.
- #17 ticket board omits empty columns, so a named column cannot be distinguished from an empty one.
- #18 base --view names are discoverable only by triggering exit 65.
- #19 skill status lines/words and the second bundled skill (clean-house) are undocumented.
- #20 pql completion absent from the skill. Low value, noted for completeness.
- #21 pql base absent from the Choosing a command routing table, as are schema and shell.

Unusable:
- #22 ''which decisions have nothing implementing them'' forces a client-side join. No --unimplemented filter; ticket list --decision none returns empty silently rather than erroring.
- #23 base output carries no path or name, so rows cannot be fed onward. Also fm.voting: 1 where meta returns voting: true — an undocumented bool coercion. The auditor could not determine whether the column set comes from the .base file or from pql, because reading the scratch vault was denied.
- #24 ranked verbs accept a nonexistent path and return [] at exit 0, where meta exits 66. A typo reads as ''nothing is related to this file'' — the shape-B trap the skill devotes a Contracts paragraph to for filter values.
- #25 mutation verbs return whole records with no way to trim: a ten-ticket status change returns ten descriptions.
- #26 backlinks duplicate rows, see #11.

Closed 2026-08-08. Resolved three ways, not one — the ticket bundled findings that turned out to need different treatment.

Folded into 2.1.0, because each was the same defect the T-79 work was already correcting, seen from another angle: #10 and #11 (outlinks row shape, backlinks per-occurrence rows) into the link-matching section, #12 (whatsnext/review message shape) into the output-shape rule, #13 and #25 (mutation verbs return whole records and reject --fields) into the projection section. Deferring those would have left sections corrected in 2.1.0 still wrong on the exact question a caller was asking.

Closed here as documentation, no decision needed: #14 .gitignore is read by default alongside .pqlignore and doctor reports the active list, #17 board omits empty columns so [] means no tickets rather than a bad name, #18 base --view names are discoverable only off the exit-65 error and base rows carry only the columns the .base declares, #19 skill status lists every bundled skill with lines and words and clean-house is the second one, #21 base is in the routing table. #20 (completion absent) stays unwritten deliberately — cobra boilerplate an agent has no use for, and the skill is better without the line.

Decided, with records: D-29 for #24, D-30 for #13/#25''s behavioural half, D-31 for #22. Implementation filed as T-90, T-91, T-92, each linked to its record.

Only #23 is unresolved, and honestly so. Whether pql drops path from base output or the .base file simply declares those columns could not be determined — the auditor was denied reads inside the scratch vault, and I have not gone looking since. It is noted on the base row in the skill as ''rows carry the columns the .base declares and nothing else — often no path'', which is true either way and warns the caller. If it turns out pql is dropping a path it has, that is a bug and gets its own ticket.', NULL, '2026-08-08 18:45:03', '2026-08-08 18:45:03.513', '2026-08-08 18:45:03.513', NULL, 'c320e7270e888ab374e2fe2c0fdac2af', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53BSK27Y4K571GPGD3JA2R', 'status', 'backlog', 'done', NULL, '2026-08-08 18:45:03', '2026-08-08 18:45:03.531', '2026-08-08 18:45:03.531', NULL, 'b95ce6b3322ac63dc6afd8989558db34', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY541P00NKK9Q7PD5CP02PTM', 'description', 'Umbrella for everything the third pql-skill-auditor run produced against 2.1.0 — the first run that executed. The two before it aborted on permission denials, which turned out to be workspace trust never having been accepted for this repo, so the project allowlist was read and not applied.

The run: 15/15 core retrievals, 44 exploratory, 26 warnings checked, ~130 pql invocations. Verdict — not fit to ship as it stands, but close. 26 findings (6 Wrong, 15 Missing, 5 Unusable, 0 Bloat), 7 procedure defects, and all six leads from the aborted run''s static read confirmed.

Three children, split by what each is waiting on rather than by finding type:

  T-79  the fix set the verdict names, plus the DSL gap — blocks 2.1.0
  T-87  Missing and Unusable findings needing a surface decision — 2.2.0
  T-88  defects in the audit procedure itself

The pattern the audit exposes, and the reason the gate is worth its cost: three of the six Wrong findings are defects in work shipped the same day, by the author who then documented it. T-72''s backlinks fix, T-67''s --fields on the show verbs and T-75''s omitted-not-null sweep each shipped with a skill claim that live use disproves. None was reachable from the inside — each needed a consumer with no repo knowledge using the documented surface and reporting what happened. The same was true of the previous audit''s seven findings, and of the eighth defect that fell out of fixing the seventh.

Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md', 'Umbrella for everything the third pql-skill-auditor run produced against 2.1.0 — the first run that executed. The two before it aborted on permission denials, which turned out to be workspace trust never having been accepted for this repo, so the project allowlist was read and not applied.

The run: 15/15 core retrievals, 44 exploratory, 26 warnings checked, ~130 pql invocations. Verdict — not fit to ship as it stands, but close. 26 findings (6 Wrong, 15 Missing, 5 Unusable, 0 Bloat), 7 procedure defects, and all six leads from the aborted run''s static read confirmed.

Three children, split by what each is waiting on rather than by finding type:

  T-79  the fix set the verdict names, plus the DSL gap — blocks 2.1.0
  T-87  Missing and Unusable findings needing a surface decision — 2.2.0
  T-88  defects in the audit procedure itself

The pattern the audit exposes, and the reason the gate is worth its cost: three of the six Wrong findings are defects in work shipped the same day, by the author who then documented it. T-72''s backlinks fix, T-67''s --fields on the show verbs and T-75''s omitted-not-null sweep each shipped with a skill claim that live use disproves. None was reachable from the inside — each needed a consumer with no repo knowledge using the documented surface and reporting what happened. The same was true of the previous audit''s seven findings, and of the eighth defect that fell out of fixing the seventh.

Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

Closed 2026-08-08. All three branches resolved: T-79 fixed the six Wrong findings plus the DSL gap and shipped in 2.1.0; T-87 resolved its Missing and Unusable findings three ways — folded into 2.1.0, closed as documentation, or decided with a record; T-88 closed all seven procedure defects plus an eighth found while filing them.

Three decisions came out of it: D-29 (an argument that names a thing is validated, a value that filters a set is not), D-30 (mutation verbs return the record they changed; projection stays on the read surface), D-31 (pql answers what is, the caller folds for what is absent). Implementation is T-90, T-91, T-92 for 2.2.0.

What the audit was worth, stated plainly so the cost is arguable next time. Three of the six Wrong findings were defects in work shipped the same day by the author who then documented it — T-72''s backlinks fix, T-67''s --fields on the show verbs, T-75''s omitted-not-null sweep. Each had passed tests, review and my own explicit check. None was reachable from inside; every one needed a consumer with no repo knowledge using the documented surface and reporting what happened. The T-75 case is the sharpest: I claimed to have audited the whole output surface and named doctor.index as the only violation, having grepped struct tags, which cannot see the DSL''s dynamic maps and which I then read past for plan export. A confident wrong claim, and only use disproved it.

The procedure improved as much as the skill. Its version check could not detect a stale binary, its poison test had no recovery test, and its read-the-source step contradicted the rule the whole audit rests on. All three were invisible until someone followed it end to end.

One residue: finding #23, whether pql drops path from base output or the .base file declares those columns, is unresolved — the auditor was denied reads inside the scratch vault. Documented in a way that is true either way, and worth a ticket if it turns out to be pql''s.', NULL, '2026-08-08 18:45:26', '2026-08-08 18:45:26.191', '2026-08-08 18:45:26.191', NULL, '536789a7989779219cfc75b99c19d1b3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY541P00NKK9Q7PD5CP02PTM', 'status', 'ready', 'done', NULL, '2026-08-08 18:45:26', '2026-08-08 18:45:26.210', '2026-08-08 18:45:26.210', NULL, 'aedc302d71c9a699cf6057335d08ff4e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5H3J0H3JC5ZQZ9QCPT3P9R', 'description', 'Resolves the open half of audit finding #23, and it is a real defect rather than the .base file''s doing.

Same key, same file, two answers:

  pql meta members/holt/persona.md   ->  "voting": true
  pql query "SELECT fm.voting ..."   ->  {"fm.voting": 1}

pql base inherits it, since base compiles to the DSL — which is where the auditor met it.

Cause, in internal/query/dsl/eval/compile.go around line 345. The fm.<key> reference compiles to a type-dispatching subquery: CASE type WHEN ''string'' THEN value_text WHEN ''number'' THEN value_num WHEN ''bool'' THEN value_num ELSE value_json END. For a bool that yields SQLite''s 1/0. The comment says the shape is chosen so SQLite can compare directly against literals, which is true and is why WHERE fm.voting = true works — the parser lowers true to 1. The problem is that the same expression is used for projection, where the comparison-friendly shape is the wrong one and the schema already stores the right one in value_json.

So the fix is not to change the CASE. Comparison genuinely wants value_num; projection wants value_json, or a type-aware decode on the way out. Changing it in one place breaks the other — verified that both = true and = 1 currently match, and both should keep matching.

Scope check before fixing: bool is the visible case, but the same CASE returns value_num for ''number'' too, so check whether an integer frontmatter value round-trips as an integer or as a REAL — value_num is REAL, so a year like 2026 may well be emitting 2026.0. And ''list''/''object'' fall to value_json, which is a JSON string in a JSON field, so confirm whether those come back as nested structures or as escaped strings. Worth settling all three at once rather than patching bool alone.

Not filed against the base surface: pql base returns exactly the columns the .base declares, which the same investigation confirmed — see the note on T-87.', 'Resolves the open half of audit finding #23, and it is a real defect rather than the .base file''s doing.

Same key, same file, two answers:

  pql meta members/holt/persona.md   ->  "voting": true
  pql query "SELECT fm.voting ..."   ->  {"fm.voting": 1}

pql base inherits it, since base compiles to the DSL — which is where the auditor met it.

Cause, in internal/query/dsl/eval/compile.go around line 345. The fm.<key> reference compiles to a type-dispatching subquery: CASE type WHEN ''string'' THEN value_text WHEN ''number'' THEN value_num WHEN ''bool'' THEN value_num ELSE value_json END. For a bool that yields SQLite''s 1/0. The comment says the shape is chosen so SQLite can compare directly against literals, which is true and is why WHERE fm.voting = true works — the parser lowers true to 1. The problem is that the same expression is used for projection, where the comparison-friendly shape is the wrong one and the schema already stores the right one in value_json.

So the fix is not to change the CASE. Comparison genuinely wants value_num; projection wants value_json, or a type-aware decode on the way out. Changing it in one place breaks the other — verified that both = true and = 1 currently match, and both should keep matching.

Scope check before fixing: bool is the visible case, but the same CASE returns value_num for ''number'' too, so check whether an integer frontmatter value round-trips as an integer or as a REAL — value_num is REAL, so a year like 2026 may well be emitting 2026.0. And ''list''/''object'' fall to value_json, which is a JSON string in a JSON field, so confirm whether those come back as nested structures or as escaped strings. Worth settling all three at once rather than patching bool alone.

Not filed against the base surface: pql base returns exactly the columns the .base declares, which the same investigation confirmed — see the note on T-87.

Scope settled by probe, and narrower than the ticket first guessed. Planted a file carrying one of each frontmatter type and read it through both paths:

  DSL   {"fm.abool":0,  "fm.alist":["a","b"], "fm.anobj":{"k":"v"}, "fm.num_float":1.5, "fm.num_int":2026}
  meta  {"abool":false, "alist":["a","b"],    "anobj":{"k":"v"},    "num_float":1.5,    "num_int":2026}

So bool is the only divergence. The speculation in the description above was wrong on both counts and is retracted: integers do not come back as REAL — 2026 stays 2026, not 2026.0 — and list/object values decode into real nested JSON rather than escaped strings, so the ELSE value_json branch is already doing the right thing.

That makes the fix small and local: the CASE needs one more arm so a bool projects as its JSON form while still comparing numerically. Everything else in the expression is correct as written. Both spellings — WHERE fm.abool = false and = 0 — must keep matching after the change; they both match today.', NULL, '2026-08-08 19:09:55', '2026-08-08 19:09:55.213', '2026-08-08 19:09:55.213', NULL, 'a7b61215ad62b1b2230e879553d7f6f3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY53BSK27Y4K571GPGD3JA2R', 'description', 'The Missing and Unusable findings from the 2.1.0 skill audit that were deliberately deferred. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

Deferred because each needs a surface decision rather than a text fix, and rushing those into a patch release is how the audit''s Wrong findings got there in the first place.

Missing, output shapes — the audit''s second conclusion is that output shapes are documented for the planning surface and undocumented for the vault surface: backlinks, outlinks, meta, refs, base and the DSL each get a one-line description of what they mean and nothing about what they return. That asymmetry breaks the second hop of any link-following task.
- #10 outlinks: shape {target,alias,line,via} undocumented, and target is the raw spelling — meta rejects it. outlinks --help says resolution is not applied; the skill does not. Since the skill also advertises backlinks as spelling-tolerant, the natural generalisation is exactly wrong.
- #11 backlinks rows are per-link-occurrence, not per-file. 28 rows for one linking file. Counting linking files needs a client-side dedupe the skill never mentions.
- #12 plan whatsnext and plan review can return {message: ...} instead of a record. A caller reaching for .id gets nothing and no error.
- #13 ticket mutation verbs return undocumented and mutually inconsistent shapes — assign returns a whole record with a 2 KB description, label returns a summary object, and neither accepts --fields.

Missing, surface facts:
- #14 .gitignore is read by default alongside .pqlignore; the skill mentions only the latter. An agent debugging ''why is this file not indexed'' will not think to look.
- #15 the documented PQL_VAULT/PQL_DB/PQL_CONFIG env routes cannot be invoked under the permission model the same skill describes — an env-var assignment is a prefix, so it breaks the allowlist match, and the anti-patterns list names only && || ; |  and >.
- #16 pql init''s target directory is unstated and the skill''s claims about what it writes are contradicted by its own --help. The auditor would not run it, correctly.
- #17 ticket board omits empty columns, so a named column cannot be distinguished from an empty one.
- #18 base --view names are discoverable only by triggering exit 65.
- #19 skill status lines/words and the second bundled skill (clean-house) are undocumented.
- #20 pql completion absent from the skill. Low value, noted for completeness.
- #21 pql base absent from the Choosing a command routing table, as are schema and shell.

Unusable:
- #22 ''which decisions have nothing implementing them'' forces a client-side join. No --unimplemented filter; ticket list --decision none returns empty silently rather than erroring.
- #23 base output carries no path or name, so rows cannot be fed onward. Also fm.voting: 1 where meta returns voting: true — an undocumented bool coercion. The auditor could not determine whether the column set comes from the .base file or from pql, because reading the scratch vault was denied.
- #24 ranked verbs accept a nonexistent path and return [] at exit 0, where meta exits 66. A typo reads as ''nothing is related to this file'' — the shape-B trap the skill devotes a Contracts paragraph to for filter values.
- #25 mutation verbs return whole records with no way to trim: a ten-ticket status change returns ten descriptions.
- #26 backlinks duplicate rows, see #11.

Closed 2026-08-08. Resolved three ways, not one — the ticket bundled findings that turned out to need different treatment.

Folded into 2.1.0, because each was the same defect the T-79 work was already correcting, seen from another angle: #10 and #11 (outlinks row shape, backlinks per-occurrence rows) into the link-matching section, #12 (whatsnext/review message shape) into the output-shape rule, #13 and #25 (mutation verbs return whole records and reject --fields) into the projection section. Deferring those would have left sections corrected in 2.1.0 still wrong on the exact question a caller was asking.

Closed here as documentation, no decision needed: #14 .gitignore is read by default alongside .pqlignore and doctor reports the active list, #17 board omits empty columns so [] means no tickets rather than a bad name, #18 base --view names are discoverable only off the exit-65 error and base rows carry only the columns the .base declares, #19 skill status lists every bundled skill with lines and words and clean-house is the second one, #21 base is in the routing table. #20 (completion absent) stays unwritten deliberately — cobra boilerplate an agent has no use for, and the skill is better without the line.

Decided, with records: D-29 for #24, D-30 for #13/#25''s behavioural half, D-31 for #22. Implementation filed as T-90, T-91, T-92, each linked to its record.

Only #23 is unresolved, and honestly so. Whether pql drops path from base output or the .base file simply declares those columns could not be determined — the auditor was denied reads inside the scratch vault, and I have not gone looking since. It is noted on the base row in the skill as ''rows carry the columns the .base declares and nothing else — often no path'', which is true either way and warns the caller. If it turns out pql is dropping a path it has, that is a bug and gets its own ticket.', 'The Missing and Unusable findings from the 2.1.0 skill audit that were deliberately deferred. Full report: /var/mnt/data/projects/claude/scratch/pql-audit-2.1.0.md

Deferred because each needs a surface decision rather than a text fix, and rushing those into a patch release is how the audit''s Wrong findings got there in the first place.

Missing, output shapes — the audit''s second conclusion is that output shapes are documented for the planning surface and undocumented for the vault surface: backlinks, outlinks, meta, refs, base and the DSL each get a one-line description of what they mean and nothing about what they return. That asymmetry breaks the second hop of any link-following task.
- #10 outlinks: shape {target,alias,line,via} undocumented, and target is the raw spelling — meta rejects it. outlinks --help says resolution is not applied; the skill does not. Since the skill also advertises backlinks as spelling-tolerant, the natural generalisation is exactly wrong.
- #11 backlinks rows are per-link-occurrence, not per-file. 28 rows for one linking file. Counting linking files needs a client-side dedupe the skill never mentions.
- #12 plan whatsnext and plan review can return {message: ...} instead of a record. A caller reaching for .id gets nothing and no error.
- #13 ticket mutation verbs return undocumented and mutually inconsistent shapes — assign returns a whole record with a 2 KB description, label returns a summary object, and neither accepts --fields.

Missing, surface facts:
- #14 .gitignore is read by default alongside .pqlignore; the skill mentions only the latter. An agent debugging ''why is this file not indexed'' will not think to look.
- #15 the documented PQL_VAULT/PQL_DB/PQL_CONFIG env routes cannot be invoked under the permission model the same skill describes — an env-var assignment is a prefix, so it breaks the allowlist match, and the anti-patterns list names only && || ; |  and >.
- #16 pql init''s target directory is unstated and the skill''s claims about what it writes are contradicted by its own --help. The auditor would not run it, correctly.
- #17 ticket board omits empty columns, so a named column cannot be distinguished from an empty one.
- #18 base --view names are discoverable only by triggering exit 65.
- #19 skill status lines/words and the second bundled skill (clean-house) are undocumented.
- #20 pql completion absent from the skill. Low value, noted for completeness.
- #21 pql base absent from the Choosing a command routing table, as are schema and shell.

Unusable:
- #22 ''which decisions have nothing implementing them'' forces a client-side join. No --unimplemented filter; ticket list --decision none returns empty silently rather than erroring.
- #23 base output carries no path or name, so rows cannot be fed onward. Also fm.voting: 1 where meta returns voting: true — an undocumented bool coercion. The auditor could not determine whether the column set comes from the .base file or from pql, because reading the scratch vault was denied.
- #24 ranked verbs accept a nonexistent path and return [] at exit 0, where meta exits 66. A typo reads as ''nothing is related to this file'' — the shape-B trap the skill devotes a Contracts paragraph to for filter values.
- #25 mutation verbs return whole records with no way to trim: a ten-ticket status change returns ten descriptions.
- #26 backlinks duplicate rows, see #11.

Closed 2026-08-08. Resolved three ways, not one — the ticket bundled findings that turned out to need different treatment.

Folded into 2.1.0, because each was the same defect the T-79 work was already correcting, seen from another angle: #10 and #11 (outlinks row shape, backlinks per-occurrence rows) into the link-matching section, #12 (whatsnext/review message shape) into the output-shape rule, #13 and #25 (mutation verbs return whole records and reject --fields) into the projection section. Deferring those would have left sections corrected in 2.1.0 still wrong on the exact question a caller was asking.

Closed here as documentation, no decision needed: #14 .gitignore is read by default alongside .pqlignore and doctor reports the active list, #17 board omits empty columns so [] means no tickets rather than a bad name, #18 base --view names are discoverable only off the exit-65 error and base rows carry only the columns the .base declares, #19 skill status lists every bundled skill with lines and words and clean-house is the second one, #21 base is in the routing table. #20 (completion absent) stays unwritten deliberately — cobra boilerplate an agent has no use for, and the skill is better without the line.

Decided, with records: D-29 for #24, D-30 for #13/#25''s behavioural half, D-31 for #22. Implementation filed as T-90, T-91, T-92, each linked to its record.

Only #23 is unresolved, and honestly so. Whether pql drops path from base output or the .base file simply declares those columns could not be determined — the auditor was denied reads inside the scratch vault, and I have not gone looking since. It is noted on the base row in the skill as ''rows carry the columns the .base declares and nothing else — often no path'', which is true either way and warns the caller. If it turns out pql is dropping a path it has, that is a bug and gets its own ticket.

Follow-up 2026-08-08: finding #23 is resolved, both halves, and it splits into one non-bug and one real bug.

Not a bug — pql base dropping path. Read the .base file the auditor was using: council-members.base declares only note.name, note.prior_job, note.lens, note.voting, note.model. No file.path, so pql returning no path is faithful to the file. Confirmed pql does support it by writing a probe base declaring file.path and file.name: both come back, and path is a real vault-relative path (members/holt/persona.md) that other commands accept. Bare names in a view''s order: prefer a declared note property over the file column of the same name, which is why ''name'' resolved to fm.name; file.name gets the file column explicitly. So the skill wording added in 2.1.0 — rows carry the columns the .base declares, often no path — is correct, and the actionable advice is to declare file.path in the base.

Real bug — the fm.voting: 1 versus voting: true discrepancy the auditor noticed alongside it. Same key, same file, meta returns true and the DSL returns 1, and base inherits it because base compiles to the DSL. Cause is the type-dispatching subquery in compile.go choosing value_num for bools so SQLite can compare against literals; the same expression is used for projection, where the comparison shape is wrong and value_json already holds the right one. Probed every frontmatter type: bool is the only one affected — integers stay integers, lists and objects decode to real nested JSON. Filed as T-93.', NULL, '2026-08-08 19:10:07', '2026-08-08 19:10:07.193', '2026-08-08 19:10:07.193', NULL, '933fc0da01cc05c27eb0ebef7403950e', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C6ZEE2W3DEZX0MGF3EYF0', 'status', 'backlog', 'ready', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.392', '2026-08-08 19:33:56.392', NULL, 'fe06d1b85f63c424f93ad11f1c8a79c5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C99ZKBZWAYDVY8PFYFHEM', 'status', 'backlog', 'ready', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.399', '2026-08-08 19:33:56.399', NULL, '1a9cda5445c54c45deeb584cd29493e5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5H3J0H3JC5ZQZ9QCPT3P9R', 'status', 'backlog', 'ready', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.399', '2026-08-08 19:33:56.399', NULL, '77e3b329eb74ef4ee06635facc6bfe65', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C83X7F5WDSXK11XWKM47C', 'status', 'backlog', 'ready', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.399', '2026-08-08 19:33:56.399', NULL, 'b23fe8e935aeffc4e08f35eaeea3b838', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5PDDXT59VGNRYSC4E4XRSR', 'status', 'backlog', 'ready', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.400', '2026-08-08 19:33:56.400', NULL, 'cb04f4638afbaa47023ae5a20a9117b1', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C6ZEE2W3DEZX0MGF3EYF0', 'status', 'ready', 'in_progress', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.413', '2026-08-08 19:33:56.413', NULL, 'c825e445feffe64cf88ced66a9579d66', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C99ZKBZWAYDVY8PFYFHEM', 'status', 'ready', 'in_progress', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.415', '2026-08-08 19:33:56.415', NULL, '64385b86c32480a763490519741b795b', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5H3J0H3JC5ZQZ9QCPT3P9R', 'status', 'ready', 'in_progress', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.415', '2026-08-08 19:33:56.415', NULL, 'b02aa2295b00d87f2df7769e26a9ceb3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5PDDXT59VGNRYSC4E4XRSR', 'status', 'ready', 'in_progress', NULL, '2026-08-08 19:33:56', '2026-08-08 19:33:56.416', '2026-08-08 19:33:56.416', NULL, '9756036a5a901ed0021b845c21ef3be7', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5H3J0H3JC5ZQZ9QCPT3P9R', 'description', 'Resolves the open half of audit finding #23, and it is a real defect rather than the .base file''s doing.

Same key, same file, two answers:

  pql meta members/holt/persona.md   ->  "voting": true
  pql query "SELECT fm.voting ..."   ->  {"fm.voting": 1}

pql base inherits it, since base compiles to the DSL — which is where the auditor met it.

Cause, in internal/query/dsl/eval/compile.go around line 345. The fm.<key> reference compiles to a type-dispatching subquery: CASE type WHEN ''string'' THEN value_text WHEN ''number'' THEN value_num WHEN ''bool'' THEN value_num ELSE value_json END. For a bool that yields SQLite''s 1/0. The comment says the shape is chosen so SQLite can compare directly against literals, which is true and is why WHERE fm.voting = true works — the parser lowers true to 1. The problem is that the same expression is used for projection, where the comparison-friendly shape is the wrong one and the schema already stores the right one in value_json.

So the fix is not to change the CASE. Comparison genuinely wants value_num; projection wants value_json, or a type-aware decode on the way out. Changing it in one place breaks the other — verified that both = true and = 1 currently match, and both should keep matching.

Scope check before fixing: bool is the visible case, but the same CASE returns value_num for ''number'' too, so check whether an integer frontmatter value round-trips as an integer or as a REAL — value_num is REAL, so a year like 2026 may well be emitting 2026.0. And ''list''/''object'' fall to value_json, which is a JSON string in a JSON field, so confirm whether those come back as nested structures or as escaped strings. Worth settling all three at once rather than patching bool alone.

Not filed against the base surface: pql base returns exactly the columns the .base declares, which the same investigation confirmed — see the note on T-87.

Scope settled by probe, and narrower than the ticket first guessed. Planted a file carrying one of each frontmatter type and read it through both paths:

  DSL   {"fm.abool":0,  "fm.alist":["a","b"], "fm.anobj":{"k":"v"}, "fm.num_float":1.5, "fm.num_int":2026}
  meta  {"abool":false, "alist":["a","b"],    "anobj":{"k":"v"},    "num_float":1.5,    "num_int":2026}

So bool is the only divergence. The speculation in the description above was wrong on both counts and is retracted: integers do not come back as REAL — 2026 stays 2026, not 2026.0 — and list/object values decode into real nested JSON rather than escaped strings, so the ELSE value_json branch is already doing the right thing.

That makes the fix small and local: the CASE needs one more arm so a bool projects as its JSON form while still comparing numerically. Everything else in the expression is correct as written. Both spellings — WHERE fm.abool = false and = 0 — must keep matching after the change; they both match today.', 'Resolves the open half of audit finding #23, and it is a real defect rather than the .base file''s doing.

Same key, same file, two answers:

  pql meta members/holt/persona.md   ->  "voting": true
  pql query "SELECT fm.voting ..."   ->  {"fm.voting": 1}

pql base inherits it, since base compiles to the DSL — which is where the auditor met it.

Cause, in internal/query/dsl/eval/compile.go around line 345. The fm.<key> reference compiles to a type-dispatching subquery: CASE type WHEN ''string'' THEN value_text WHEN ''number'' THEN value_num WHEN ''bool'' THEN value_num ELSE value_json END. For a bool that yields SQLite''s 1/0. The comment says the shape is chosen so SQLite can compare directly against literals, which is true and is why WHERE fm.voting = true works — the parser lowers true to 1. The problem is that the same expression is used for projection, where the comparison-friendly shape is the wrong one and the schema already stores the right one in value_json.

So the fix is not to change the CASE. Comparison genuinely wants value_num; projection wants value_json, or a type-aware decode on the way out. Changing it in one place breaks the other — verified that both = true and = 1 currently match, and both should keep matching.

Scope check before fixing: bool is the visible case, but the same CASE returns value_num for ''number'' too, so check whether an integer frontmatter value round-trips as an integer or as a REAL — value_num is REAL, so a year like 2026 may well be emitting 2026.0. And ''list''/''object'' fall to value_json, which is a JSON string in a JSON field, so confirm whether those come back as nested structures or as escaped strings. Worth settling all three at once rather than patching bool alone.

Not filed against the base surface: pql base returns exactly the columns the .base declares, which the same investigation confirmed — see the note on T-87.

Scope settled by probe, and narrower than the ticket first guessed. Planted a file carrying one of each frontmatter type and read it through both paths:

  DSL   {"fm.abool":0,  "fm.alist":["a","b"], "fm.anobj":{"k":"v"}, "fm.num_float":1.5, "fm.num_int":2026}
  meta  {"abool":false, "alist":["a","b"],    "anobj":{"k":"v"},    "num_float":1.5,    "num_int":2026}

So bool is the only divergence. The speculation in the description above was wrong on both counts and is retracted: integers do not come back as REAL — 2026 stays 2026, not 2026.0 — and list/object values decode into real nested JSON rather than escaped strings, so the ELSE value_json branch is already doing the right thing.

That makes the fix small and local: the CASE needs one more arm so a bool projects as its JSON form while still comparing numerically. Everything else in the expression is correct as written. Both spellings — WHERE fm.abool = false and = 0 — must keep matching after the change; they both match today.

Fixed 2026-08-08 for 2.2.0, commit 09df675. The compiler now tracks whether it is emitting the SELECT list; the bool arm of the type-dispatching CASE projects CAST(value_json AS BLOB) there and keeps value_num everywhere else.

The BLOB is the load-bearing part and worth recording. normalise() in exec.go turns a []byte holding valid JSON into json.RawMessage but deliberately refuses to reinterpret a bare string ''true'' — otherwise a frontmatter string whose value happens to be the word true would come back as a JSON bool. Selecting value_json as text would have produced that second wrong answer. The BLOB is what separates ''this is JSON'' from ''this is text that looks like JSON'', and only the bool branch can make the claim because only it has consulted the type column.

Verified: fm.abool -> false, fm.btrue -> true, fm.strtrue (a string ''true'') -> "true", fm.num_int -> 2026, fm.alist -> ["a","b"]. Comparison unchanged — = true, = 1, = false, = 0 all behave as before, and ORDER BY still sorts numerically. base council-members now returns fm.voting: true, matching meta, which is where the audit met this.

Guarded by TestBoolProjection_DiffersFromComparison, which asserts the BLOB cast appears in SELECT and does not appear in WHERE or ORDER BY. TestCompile_FmTypeDispatchInSubquery had required value_num in a SELECT, so it was pinning the bug rather than catching it; that assertion is removed and the reason recorded in the test.', NULL, '2026-08-08 19:36:32', '2026-08-08 19:36:32.759', '2026-08-08 19:36:32.759', NULL, '1f8370ee85f929b0065f61ac9dd8c3b5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5H3J0H3JC5ZQZ9QCPT3P9R', 'status', 'in_progress', 'done', NULL, '2026-08-08 19:36:32', '2026-08-08 19:36:32.782', '2026-08-08 19:36:32.782', NULL, 'b3c26a4d93b3e09c7f9827ca3fa8e007', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5PDDXT59VGNRYSC4E4XRSR', 'description', 'Found while fixing T-93, and it is the worse of the two.

  pql query "SELECT path, name"
  [{"name":"albu","path":"album.md"},
   {"name":"diagra","path":"diagram.md"},
   {"name":"secon","path":"second.md"},
   {"name":"README","path":"README.md"}]

  pql meta second.md  ->  "name": "second"

Cause: fileColumn("name") in internal/query/dsl/eval/compile.go derives the basename and then calls rtrim(..., ''.md''). SQLite''s two-argument rtrim removes any trailing characters that appear in the second argument treated as a SET, not as a literal suffix. So it strips the extension and then keeps going through any trailing d, m or period in the stem. README survives because it ends in E; types survives because it ends in s; album, diagram, second, keyword, system, method, problem do not.

Severity: name is a documented built-in column, it appears in the skill''s DSL examples and in the routing table, and the failure is silent — a wrong string, not an error. Any vault with a file ending in m or d gets corrupted output from a core column, and those endings are common.

meta is unaffected because it derives the name in Go, which is also why the audit never caught this: it exercised meta and read name from ranked output, never SELECT name against a filename in the affected set.

Fix: replace the rtrim with a real suffix strip — CASE WHEN <basename> LIKE ''%.md'' THEN substr(<basename>, 1, length(<basename>) - 3) ELSE <basename> END. Verbose because the basename expression repeats, but correct. Check whether folder has the same class of defect while there — it uses instr/substr arithmetic rather than rtrim, so probably not, but confirm rather than assume.

Regression test must cover a name ending in each of d, m and period, plus one unaffected, plus a file with no extension.', 'Found while fixing T-93, and it is the worse of the two.

  pql query "SELECT path, name"
  [{"name":"albu","path":"album.md"},
   {"name":"diagra","path":"diagram.md"},
   {"name":"secon","path":"second.md"},
   {"name":"README","path":"README.md"}]

  pql meta second.md  ->  "name": "second"

Cause: fileColumn("name") in internal/query/dsl/eval/compile.go derives the basename and then calls rtrim(..., ''.md''). SQLite''s two-argument rtrim removes any trailing characters that appear in the second argument treated as a SET, not as a literal suffix. So it strips the extension and then keeps going through any trailing d, m or period in the stem. README survives because it ends in E; types survives because it ends in s; album, diagram, second, keyword, system, method, problem do not.

Severity: name is a documented built-in column, it appears in the skill''s DSL examples and in the routing table, and the failure is silent — a wrong string, not an error. Any vault with a file ending in m or d gets corrupted output from a core column, and those endings are common.

meta is unaffected because it derives the name in Go, which is also why the audit never caught this: it exercised meta and read name from ranked output, never SELECT name against a filename in the affected set.

Fix: replace the rtrim with a real suffix strip — CASE WHEN <basename> LIKE ''%.md'' THEN substr(<basename>, 1, length(<basename>) - 3) ELSE <basename> END. Verbose because the basename expression repeats, but correct. Check whether folder has the same class of defect while there — it uses instr/substr arithmetic rather than rtrim, so probably not, but confirm rather than assume.

Regression test must cover a name ending in each of d, m and period, plus one unaffected, plus a file with no extension.

Fixed 2026-08-08 for 2.2.0, commit 09df675. rtrim(basename, ''.md'') replaced with a literal-suffix strip: CASE WHEN <basename> LIKE ''%.md'' THEN substr(<basename>, 1, length(<basename>) - 3) ELSE <basename> END. The basename expression is hoisted to a sqlBasename constant and the strip to stripMDSuffix, since SQLite has no scalar LET binding and the expression would otherwise appear four times inline.

Verified across the affected endings and the ones that hid it: album.md -> album, diagram.md -> diagram, second.md -> second, README.md -> README, types.md -> types, deep/er/keyword.md -> keyword, and folder still resolves on nested paths.

Scope check from the ticket, answered: folder does not share the defect. It uses instr/substr arithmetic with no rtrim, and nested paths return the right prefix.

Guarded by TestName_StripsOnlyTheExtension, which covers a stem ending in each of d and m, an unaffected one, a nested path and a single-character stem in the set. TestCompile_NameAndFolderUseSubstr had required the rtrim to be present, so it was holding the bug in place; it now asserts the opposite — no rtrim, and a LIKE ''%.md'' guard — with the reason recorded.

Worth noting for the audit record: this was not found by the skill audit. The auditor exercised meta, which derives the name in Go and is correct, and read name from ranked output rather than SELECT name. It surfaced only because a T-93 probe happened to create a file called second.md.', NULL, '2026-08-08 19:36:43', '2026-08-08 19:36:43.706', '2026-08-08 19:36:43.706', NULL, '18602561d65c1d6d49c18f6a5759e860', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5PDDXT59VGNRYSC4E4XRSR', 'status', 'in_progress', 'done', NULL, '2026-08-08 19:36:43', '2026-08-08 19:36:43.725', '2026-08-08 19:36:43.725', NULL, 'a13aa08748b35b6e3db8f1f1a500c175', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C6ZEE2W3DEZX0MGF3EYF0', 'description', 'Implements D-29 on the three ranked verbs.

pql related, pql context and pql search take a path argument and do not check it exists. A typo returns [] at exit 0, which reads as ''nothing is related to this file'' when the truth is ''there is no such file''. pql meta on the same typo exits 66, so the identical mistake is loud on one verb and silent on the neighbouring one — and the silent one is the verb the skill tells an agent to reach for first.

D-29 settles the rule: an argument that names a thing is validated, a value that filters a set is not. These three arguments name things.

Fix: resolve the path against files before ranking, exit 66 naming the path when absent. context already does exactly this for its outbound targets (T-73), so the query shape exists. One extra query on a path that already opens the store.

Not in scope: backlinks and outlinks, whose argument is matched against link text rather than resolved. Validating there needs the normalization Q-6 is still open about, so they keep returning [] and the skill says why. Nor the DSL, where WHERE path = ''nope.md'' is a filter and correctly empty.

Behaviour change — a caller relying on the empty gets 66. Ships in a minor with a compatibility note.', 'Implements D-29 on the three ranked verbs.

pql related, pql context and pql search take a path argument and do not check it exists. A typo returns [] at exit 0, which reads as ''nothing is related to this file'' when the truth is ''there is no such file''. pql meta on the same typo exits 66, so the identical mistake is loud on one verb and silent on the neighbouring one — and the silent one is the verb the skill tells an agent to reach for first.

D-29 settles the rule: an argument that names a thing is validated, a value that filters a set is not. These three arguments name things.

Fix: resolve the path against files before ranking, exit 66 naming the path when absent. context already does exactly this for its outbound targets (T-73), so the query shape exists. One extra query on a path that already opens the store.

Not in scope: backlinks and outlinks, whose argument is matched against link text rather than resolved. Validating there needs the normalization Q-6 is still open about, so they keep returning [] and the skill says why. Nor the DSL, where WHERE path = ''nope.md'' is a filter and correctly empty.

Behaviour change — a caller relying on the empty gets 66. Ships in a minor with a compatibility note.

Implemented 2026-08-08 for 2.2.0, commit 66ee8b4. related and context resolve their path argument against files before ranking and exit 66 naming it when absent. Exact match only — accepting spellings here would re-introduce the guessing Q-6 is about, and the argument is the vault-relative path every other command returns.

Applied before the --flat-search short-circuit, so the primitive path enforces it too: the argument is still a name there.

search is untouched, per D-29''s line. Its argument is a query, a query is a filter, and an unmatched filter is correctly empty at exit 0. TestIntegration_RankedVerbs_RejectUnindexedPath asserts that explicitly rather than leaving it implied, so a future change that over-generalises the check fails a test instead of quietly breaking search.

Verified: related and context on an unindexed path exit 66 naming it, --flat-search likewise, a real path still ranks, and search on an unmatched query still returns [] at exit 0.

Behaviour change, so it lands in a minor with a compatibility note in the CHANGELOG — a caller relying on the empty result for a bad path now gets 66.', NULL, '2026-08-08 19:36:58', '2026-08-08 19:36:58.369', '2026-08-08 19:36:58.369', NULL, '115efcdee0f386d1262e0961f41d12a5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C6ZEE2W3DEZX0MGF3EYF0', 'status', 'in_progress', 'done', NULL, '2026-08-08 19:36:58', '2026-08-08 19:36:58.391', '2026-08-08 19:36:58.391', NULL, '28b7bee0580c0ab727454564b5f35ae5', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C99ZKBZWAYDVY8PFYFHEM', 'description', 'Implements D-31.

D-31 declines to add absence filters — no decisions list --unimplemented, no --without-<x> family — on the grounds that each is a join predicate in disguise and adding them one at a time turns a query surface into an ad-hoc ORM. The supported route is a batched positive call plus a client-side fold, which is cheap now that batching and projection both work.

Two things make that decision honest, and neither is done.

First, the skill has to carry a worked example, because an agent will not discover a fold pattern from a flag list. Something like: decisions list --oneline to source the ids, decisions show <ids> --with-tickets --fields id,tickets, then filter for the missing tickets key. That is the route the audit reconstructed by hand across four calls.

Second, ticket list --decision none currently returns empty silently, which is worse than not supporting the question — it looks like a supported filter that found nothing. Per D-29''s closed-set rule it should exit 64 naming the supported values. Same for any other spelling that reads as a negative filter and is not one.', 'Implements D-31.

D-31 declines to add absence filters — no decisions list --unimplemented, no --without-<x> family — on the grounds that each is a join predicate in disguise and adding them one at a time turns a query surface into an ad-hoc ORM. The supported route is a batched positive call plus a client-side fold, which is cheap now that batching and projection both work.

Two things make that decision honest, and neither is done.

First, the skill has to carry a worked example, because an agent will not discover a fold pattern from a flag list. Something like: decisions list --oneline to source the ids, decisions show <ids> --with-tickets --fields id,tickets, then filter for the missing tickets key. That is the route the audit reconstructed by hand across four calls.

Second, ticket list --decision none currently returns empty silently, which is worse than not supporting the question — it looks like a supported filter that found nothing. Per D-29''s closed-set rule it should exit 64 naming the supported values. Same for any other spelling that reads as a negative filter and is not one.

Implemented 2026-08-08 for 2.2.0, commits 66ee8b4 and a1e5172. Both halves.

Code: ticket list --decision now refuses none, null, nil, empty, unset and - at exit 64, with a message naming what the flag does take and pointing at the pattern that answers the question. A real id remains a filter, so an unmatched one still returns [] at exit 0 — the refusal must not turn every value into a lookup, and the test asserts that direction too.

Documentation: the skill now carries the absence pattern as a worked example under decisions — list the ids, batch-show them with --with-tickets, fold for the absent tickets key — with the warning that --fields id alone drops the join and makes every row look unimplemented. That last point is the trap the audit actually fell into, so it is stated rather than implied. Generalised in one line: batch the positive facts, fold client side.

Scope note: the case-insensitive match means --decision None is refused too, which is right — the caller meant the same thing.', NULL, '2026-08-08 19:36:58', '2026-08-08 19:36:58.406', '2026-08-08 19:36:58.406', NULL, 'b9e1462358ab99c3bd1824b552bf8e02', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C99ZKBZWAYDVY8PFYFHEM', 'status', 'in_progress', 'done', NULL, '2026-08-08 19:36:58', '2026-08-08 19:36:58.422', '2026-08-08 19:36:58.422', NULL, 'aa059700f7b283f6cf38c434129472e3', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY5C83X7F5WDSXK11XWKM47C', 'description', 'Implements D-30.

The mutation verbs return whatever each one happened to return. ticket assign gives a whole record including a 2 KB description; ticket label gives {ticket_ids, action, label}. Neither is documented, and --fields is rejected, so the verbs with the fattest output are the ones with no way to trim.

D-30 keeps projection off the mutation surface — a mutation''s return value is a receipt, and a caller who trimmed it to id has confirmed nothing — and standardises the shapes instead. One record changed returns that record whole. Several changed return a summary object naming the ids and what was applied, which is what ticket label already does.

Work: audit every mutation verb (status, assign, team, label, setparent, decision, block, unblock, append, refine write) for which shape it currently returns, converge the batch cases on the summary, and document both shapes in the skill. The documentation half is worth doing first and separately — it is the part a caller is blocked on today, and it does not change behaviour.

The shape change lands in a minor.', 'Implements D-30.

The mutation verbs return whatever each one happened to return. ticket assign gives a whole record including a 2 KB description; ticket label gives {ticket_ids, action, label}. Neither is documented, and --fields is rejected, so the verbs with the fattest output are the ones with no way to trim.

D-30 keeps projection off the mutation surface — a mutation''s return value is a receipt, and a caller who trimmed it to id has confirmed nothing — and standardises the shapes instead. One record changed returns that record whole. Several changed return a summary object naming the ids and what was applied, which is what ticket label already does.

Work: audit every mutation verb (status, assign, team, label, setparent, decision, block, unblock, append, refine write) for which shape it currently returns, converge the batch cases on the summary, and document both shapes in the skill. The documentation half is worth doing first and separately — it is the part a caller is blocked on today, and it does not change behaviour.

The shape change lands in a minor.

Split 2026-08-08. The documentation half shipped in 2.2.0 (commit a1e5172): the skill now states that mutation verbs reject --fields deliberately — a receipt trimmed to id confirms nothing — that changing one record returns that record whole including its description, that changing several returns a summary object, and that not every batch verb has converged on the summary yet.

That was the half blocking callers, since the shapes were entirely undocumented. What remains is the convergence itself: audit status, assign, team, label, setparent, decision, block, unblock, append and refine write for which shape each returns today, and move the batch cases onto the summary.

Held back from 2.2.0 deliberately. It changes ten verbs'' output shapes with real regression surface, immediately after a release, and none of it is blocking anyone now that the current behaviour is written down. Stays ready rather than backlog — it is well-specified and can be picked up whenever, it just should not have been rushed alongside two behaviour changes.', NULL, '2026-08-08 19:37:06', '2026-08-08 19:37:06.514', '2026-08-08 19:37:06.514', NULL, 'deb5163fe41777d73510cc1a0deeb430', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FY7JHX6RH2BQK3R9VVP1ZX68', 'description', 'Observed 2026-08-09 in pql''s own repo. Running ''pql --vault <repo> ticket new'' where .pql/pql.db does not exist but .pql/changelog/ holds a full history does NOT import the changelog first. It creates an empty db and mints the new ticket as T-1, colliding with the existing T-1. The vault already ran to T-94. Nothing is corrupted - identity is the ULID and the changelog is append-only - but the label collision is silent at creation time. It surfaced only on the next plan rebuild, which reported it correctly and clearly. Two contributing gaps. First, the docs say plan import runs automatically on a fresh clone; that holds only where pql init has planted the hooks. pql''s own repo has never been inited (doctor reports config.loaded false, no .pql/config.yaml), so nothing auto-imports there. Second, and the real fix: any planning mutation should detect changelog-present-but-db-empty and either import first or refuse with a diagnostic. Silently starting the label sequence at 1 is the one behaviour that produces a collision. Recovery, for the record: plan rebuild --verify restores everything and reports the collision, then relabel by record_id. Note relabel T-1 fails with a circular message telling you to run relabel; the ambiguous label cannot address either record, so the ULID is required. Worth a clearer diagnostic there too.', 'Observed 2026-08-09 in pql''s own repo. Running ''pql --vault <repo> ticket new'' where .pql/pql.db does not exist but .pql/changelog/ holds a full history does NOT import the changelog first. It creates an empty db and mints the new ticket as T-1, colliding with the existing T-1. The vault already ran to T-94. Nothing is corrupted - identity is the ULID and the changelog is append-only - but the label collision is silent at creation time. It surfaced only on the next plan rebuild, which reported it correctly and clearly. Two contributing gaps. First, the docs say plan import runs automatically on a fresh clone; that holds only where pql init has planted the hooks. pql''s own repo has never been inited (doctor reports config.loaded false, no .pql/config.yaml), so nothing auto-imports there. Second, and the real fix: any planning mutation should detect changelog-present-but-db-empty and either import first or refuse with a diagnostic. Silently starting the label sequence at 1 is the one behaviour that produces a collision. Recovery, for the record: plan rebuild --verify restores everything and reports the collision, then relabel by record_id. Note relabel T-1 fails with a circular message telling you to run relabel; the ambiguous label cannot address either record, so the ULID is required. Worth a clearer diagnostic there too.

CORRECTION to the paragraph above about this repo never having been inited.

That claim was overstated and was used to explain the wrong thing. What is true: this repo has no .pql/config.yaml and no planted hooks, and that is still why the clone-time import never fires, which is the substance of this ticket. What is not true is the inference drawn from it at the time - that the vault held no decisions. It holds 44, in governance/decisions/architecture.md, D-1 through D-31 plus questions and rejected records. They were simply never synced into the database; one decisions sync populated all of them and reported 69 cross-references and nothing broken.

The mistake underneath it is worth recording because it is the same shape as this ticket''s own subject. pql doctor reports db.exists for index.db, the query cache. Planning state is a different database, pql.db, and it did exist. Reading one field as evidence about the other produced a confident and wrong conclusion about the repo''s state - exactly the way an empty label sequence produced a confident and wrong T-1 above.

Nothing about the proposed fix changes. The guard still belongs in the mutation path rather than in the hooks, for the reason already given: the hooks are what is missing whenever this bites.', NULL, '2026-08-09 02:17:44', '2026-08-09 02:17:44.881', '2026-08-09 02:17:44.881', NULL, 'da10ae67cabc34262886af7a3a5768d2', 2) ON CONFLICT(hash) DO NOTHING;
INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_by, changed_at, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06FYBX6214TDP22Q7PASAAN8YG', 'description', 'pql can mint a record id but cannot close one. ''decisions claim <D|Q|R>'' is the only authoring verb, and there is no counterpart for the Q -> D transition.

The schema already has the states: type question and status resolved both exist and are queryable. What is missing is the transition and the link. So every vault invents its own convention for it. In one vault I work in, the convention is a ''Resolved -> D-N'' line in the question body plus a hand-edited status field - written into a file header ad hoc, enforced by nobody, and not surfaced by ''decisions refs'' unless it happens to parse as a cross-reference.

Suggested shape, matching the existing surface:

  pql decisions resolve Q-2 --into D-23

sets the question status to resolved, records the link so ''decisions refs Q-2'' and ''decisions show D-23 --with-refs'' both show it, and rejects an unknown or non-question id the way ''ticket decision'' already rejects an unknown D. The markdown stays the source of truth, so it would write the backlink line rather than only touching pql.db - same direction as ''ticket relabel --fix-prose''.

Why it matters beyond tidiness: an open question that is never formally closed stays open in the index forever. This vault reports 12 open questions against 13 total, which is either accurate or an artefact of there being no cheap way to close one - and no way to tell which from the outside.

Second, smaller point, docs rather than code. The bundled SKILL.md documents the D/Q/R vocabulary but not when to reach for each. Two rules would carry most of the value: a deferral is written as a Q rather than left as prose, and a D record is a home for durable documentation and not only for a choice between alternatives. The type is already named ''confirmed'' rather than ''decision'', which fits both readings - but the command, the directory and the --decision flag all say decision, so readers narrow it. A sentence in the skill fixes that; renaming anything would not be worth it.', 'pql can mint a record id but cannot close one. ''decisions claim <D|Q|R>'' is the only authoring verb, and there is no counterpart for the Q -> D transition.

The schema already has the states: type question and status resolved both exist and are queryable. What is missing is the transition and the link. So every vault invents its own convention for it. In one vault I work in, the convention is a ''Resolved -> D-N'' line in the question body plus a hand-edited status field - written into a file header ad hoc, enforced by nobody, and not surfaced by ''decisions refs'' unless it happens to parse as a cross-reference.

Suggested shape, matching the existing surface:

  pql decisions resolve Q-2 --into D-23

sets the question status to resolved, records the link so ''decisions refs Q-2'' and ''decisions show D-23 --with-refs'' both show it, and rejects an unknown or non-question id the way ''ticket decision'' already rejects an unknown D. The markdown stays the source of truth, so it would write the backlink line rather than only touching pql.db - same direction as ''ticket relabel --fix-prose''.

Why it matters beyond tidiness: an open question that is never formally closed stays open in the index forever. This vault reports 12 open questions against 13 total, which is either accurate or an artefact of there being no cheap way to close one - and no way to tell which from the outside.

Second, smaller point, docs rather than code. The bundled SKILL.md documents the D/Q/R vocabulary but not when to reach for each. Two rules would carry most of the value: a deferral is written as a Q rather than left as prose, and a D record is a home for durable documentation and not only for a choice between alternatives. The type is already named ''confirmed'' rather than ''decision'', which fits both readings - but the command, the directory and the --decision flag all say decision, so readers narrow it. A sentence in the skill fixes that; renaming anything would not be worth it.', NULL, '2026-08-09 09:58:29', '2026-08-09 09:58:29.474', '2026-08-09 09:58:29.474', NULL, 'f8271ad7618deac98fe33622e170427d', 2) ON CONFLICT(hash) DO NOTHING;
