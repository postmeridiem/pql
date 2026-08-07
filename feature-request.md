# pql feature requests

Pre-triage inbox for feature ideas surfaced from consumer use (primarily
clide dogfooding the planning surface). Accepted items move into the
planning dataset (`pql ticket new …`); this file is the scratchpad before
that, not a parallel backlog.

---

## FR-1 — `ticket list` projection *(accepted 2026-07-08 → T-56, shipped in 1.11.0)*

Moved to the planning dataset per the triage flow above. Shipped as D-27:
`--fields` / `--oneline` on `ticket list` and `decisions list`, and
`ticket list` drops `description` from its default projection (`--full`
opts back in). Evidence and measurements live in the D-27 record
(`governance/decisions/architecture.md`) and `pql ticket show T-56`.

---

## FR-3 — BUG: same-second create→append silently loses the append on changelog rebuild *(accepted 2026-08-07 → T-59, T-60; write side already shipped in 1.10.4)*

- **Status:** accepted (bug, data loss). Triage: the write-side half shipped
  before this was filed — 1.10.4 (commit `8d5a53b`) moved write timestamps to
  millisecond precision, so *new* same-second mutations no longer tie. The
  residual is that pre-1.10.4 rows keep second granularity forever and the LWW
  guard still tie-breaks them on content hash → **T-59**. The `--verify`
  proposal → **T-60**.
- **Source:** settled-reach, 2026-07-08 — ticket T-1057's description was authored as
  create-placeholder + immediate `ticket append` in one script pass (2026-06-12).
  The live DB served the placeholder for a month; the real text survived only as a
  changelog row.
- **Severity:** silent data loss on a documented, recommended flow (`--id-only`
  create → `append` is the tree-creation-script pattern the skill itself teaches).

### Mechanism (diagnosed against the settled-reach vault)

Both the creation row and the append row carried the identical `updated_at`
(`2026-06-12 10:40:59` — same wall-clock second). On `pql plan rebuild` /
`post-checkout` replay, the LWW `ON CONFLICT` upsert tie-breaks equal
`updated_at` by comparing row-hash strings; the placeholder's hash
(`965ad2…`) lexicographically beat the real append's (`60d7cd…`), so the
rebuilt `pql.db` kept `"(description follows in first append)"` and dropped
the appended description. Write-through order was correct at authoring time —
the loss only manifests on replay/rebuild, i.e. on every fresh clone and
branch switch thereafter.

### Proposal

- Tie-break equal `updated_at` by **changelog file order** (append position is
  already a total order within a month file) or add a monotonic per-mutation
  sequence column to changelog rows — not by content-hash comparison, which is
  arbitrary w.r.t. causality.
- Cheap hardening alternative/addition: sub-second precision on `updated_at`
  (the column is TEXT; `strftime('%Y-%m-%d %H:%M:%f')` suffices).
- A `pql plan rebuild --verify` that diffs pre/post row counts+hashes and warns
  on dropped-newer-row candidates would have surfaced this a month earlier.

---

## FR-4 — BUG: vault-root discovery walks up for a `.git` *directory*, so it resolves a worktree to its main checkout *(accepted 2026-08-07 → T-58, high)*

- **Status:** accepted (bug, silent wrong-target). Reproduced in the pql repo
  itself on 2026-08-07. Triage added one finding the report did not have: the
  `.git`-file fix alone is insufficient, because `walkUp` runs a **full**
  `.obsidian` ascent before the `.git` one — a worktree nested under an
  Obsidian-shaped vault still resolves to the main checkout. The fix is a
  single ascent checking both markers per level. Detail in T-58.
- **Source:** settled-reach, 2026-07-08 — a subagent editing `governance/*.md` inside a
  `git worktree` ran `pql decisions sync`/`validate` from the worktree and it silently
  read/wrote the **main checkout's** vault, not the worktree's.

### Mechanism

`internal/config/discover.go:70` (`walkUp`) locates the vault root by ascending until it
finds a `.git` **directory**. In a linked `git worktree`, `.git` is a **file** (a
`gitdir:` pointer), not a directory — so `walkUp` skips it and keeps ascending to the
main working tree, resolving the vault to `main`. Consequences from the field:
- `pql decisions validate` returned `ok:true` against main's unchanged markdown while the
  actual edits sat in the worktree — a **false positive** (validated the wrong files).
- `pql decisions sync` would parse main's DQR and rewrite **main's** `pql.db` + the
  tracked `governance/README.md`, ignoring the worktree edits entirely.
- Workaround that works: `pql --vault <worktree-abs-path> decisions validate`. But the
  default (no `--vault`) is silently wrong, which is the dangerous part.
- **Second field hit (settled-reach, 2026-07-24):** a new D-record written in a worktree
  synced 389-not-390 records (main's tree) with `validate` green — the false-positive
  class again. New datapoint: `--db <worktree>/.pql/pql.db` **alone does not rescue it**
  (`decisions show` still reported not-found); only `--vault` redirects both the parse
  side and the vault-derived DB coherently. Until fixed, worktree sessions must pass
  `--vault` on **every** decisions call, not just sync.

### Proposal

- In `walkUp`, treat a `.git` **file** as a repo marker too (parse its `gitdir:` line to
  find the real git dir, but the *vault root* is the directory containing the `.git`
  file — the worktree checkout — which is what the user means).
- Minimum: stop requiring `.git` to be a directory — `os.Stat` match on either a dir or a
  file named `.git` fixes the resolution to the worktree.
- Bonus: `pql doctor` could print the resolved vault root prominently so a wrong-target
  resolution is visible before a destructive sync.

---

## FR-5 — `--decision` is settable only at `ticket new`; there is no way to link an existing ticket to a D-record *(accepted 2026-08-07 → T-61)*

- **Status:** accepted (friction, hit for real — settled-reach, 2026-07-27).
  Taken as the dedicated-subcommand shape, with comma-batched ids like
  `setparent`, since the motivating case is 19 tickets at once.
- **Severity:** low mechanically, but it silently degrades the feature it belongs to
  — `pql decisions show <id> --with-tickets` is the project's implementation-status
  view (D-20), and it is only as complete as the links happen to be.

**What happened.** A delegated agent created a 19-ticket tree implementing a freshly
written D-record and omitted `--decision` on every `ticket new`. Discovered on review.
There is no way to repair it: `--decision` exists only on `ticket new`, and
`ticket refine write` explicitly rejects anything outside `title`, `description`,
`priority`, `type` ("Status, parent, assignee, team, and labels have dedicated
subcommands"). Decision has neither a writable field nor a dedicated subcommand.

Every other structural attribute of a ticket is repairable after the fact — `setparent`
for hierarchy, `block`/`unblock` for dependencies, `assign`, `team`, `label`, `status`,
even `relabel` for a colliding id. Decision linkage is the only one that is
create-time-or-never, which makes it the only one where a delegation mistake is
permanent. The realistic alternatives are all bad: recreate the tickets (loses the
ids, and the changelog already recorded them), or leave the D-record's
`--with-tickets` view lying about its own implementation status.

**Ask.** A dedicated subcommand mirroring the ones that already exist, e.g.

```
pql ticket decision T-1211 D-258        # set/replace
pql ticket decision T-1211 none         # clear, mirroring `setparent … none`
```

Adding `decision` to `refine write`'s writable set would work equally well; the
dedicated-subcommand shape just matches the precedent set by `setparent`/`team`/`assign`.

---

## FR-2 — Rank the planning dataset against arbitrary text ("coverage overlap") *(accepted 2026-08-07 → epic T-62)*

- **Status:** accepted as an epic — still needs a design decision before any
  code. Children: **T-63** (the D-record for Path A + the Q-record keeping Path
  B question-shaped; gates the rest), **T-64** (implement Path A — blocked by
  T-63), **T-65** (the golden eval set from the T-359 mapping), **T-66** (the
  doc reframe below).
- **Source:** same session. Motivating task: *"how much of `fable-ous.md`
  (a 25 KB review doc) is already captured as tickets?"*

### The observed failure mode

Answering that required manually fanning out **three agents** to
cross-reference the document's ~40 findings against ~425 tickets. Plain
`grep` missed semantically-equivalent phrasings (a bug described as "PTY
leaks its master fd" vs. a ticket titled "fd not closed on natural exit");
there is no surface that ranks tickets by overlap with a body of text.

This matters because `docs/structure/design-philosophy.md` §"Why not
vectors" names exactly one valid trigger for reconsidering semantic
retrieval: *"a specific failure mode … observed repeatedly in production
use — not because the roadmap assumed it would be."* This is one such
observation, logged here as evidence rather than as a demand for vectors.

### What exists vs. what's missing

- `pql search <query>` already ranks **with the signal ranker** — but it
  targets `index.db` (vault notes), and takes a short query string.
- Missing: rank the **`pql.db` planning dataset** (tickets/decisions), and
  accept a **document** as the input, not just a short query.
- Gap, stated plainly: *"rank tickets/decisions by overlap with this text."*

### Tested (2026-06-17) — `pql search` as-is does not serve this

Ran it against ground truth (the T-359 finding→ticket mapping) on the clide
vault:

- **Corpus gap, confirmed.** Every `pql search` result is a vault `.md` file
  (a `path`), never a ticket. `index.db` (414 KB) indexes vault notes;
  tickets live in `pql.db` (2.6 MB), which `search` doesn't read. The
  ticket-coverage question is simply invisible to it.
- **A specific finding query returned `[]`** — `pql search "PTY master file
  descriptor leak on natural child exit"` → zero hits.
- **No lexical signal participates in the rank phase.** `pql search
  "terminal"` scored its top hits almost entirely on `recency` (weight
  0.25); `link_overlap`, `tag_overlap`, `path_proximity`, and `centrality`
  were all 0, and there is no text-match signal among them. On a 40-finding
  document this yields recency-noise, not relevance.

Conclusion: the blockers are **corpus + signal wiring, not embeddings**.
Path A is viable precisely because the pipeline, signal framework, and eval
harness already exist — they are just not pointed at the planning dataset
and not carrying a lexical signal.

### Proposal — in-philosophy path first

**Path A (preferred, no vectors).** Extend the existing generate→rank→bundle
pipeline to range over the planning dataset and accept document-length
input — e.g. `pql ticket search --text-file fable-ous.md` or a
`plan related --text-file …` intent. A document is just a long query; the
signals the philosophy already endorses — textual match, co-occurrence,
recency, title/identifier overlap — are the right substance for this, and
they stay cheap and inspectable. Ship behind `--flat-search` like every
other intent.

**Ready-made eval set:** the work this session produced a *labeled relevance
mapping* for free — epic **T-359** with children **T-360–T-386** map 1:1
onto `fable-ous.md`'s findings. That is a gold standard for
`internal/connect/rank/testdata/golden/` to tune Path A against (NDCG@k /
MRR), before anyone argues it's insufficient.

**Path B (deferred, out-of-philosophy).** A vector/embedding index. This
violates two standing invariants — §"Why not vectors" and the single-store
("introducing a second store … should be resisted") rule. Per the doctrine,
do **not** take this path until Path A is built and *demonstrably* fails on
this use case. If it is ever taken, it is a design-philosophy **amendment**
(a D-record with the observed failure data), not a feature ticket.

### Doc note — "No vectors" is the right sentiment, the wrong mechanism

The sentiment is correct and permanent: lean on cheap, inspectable signals;
resist opaque infrastructure you can't account for. The *mechanism* — an
absolute "No vectors" prohibition — is what's wrong: it encodes a for-now
gate as a permanent verdict. `design-philosophy.md` §"Why not vectors" even
contradicts itself, calling the absence *"not a gap to be filled later"*
(permanent) one paragraph above *"if semantic retrieval becomes necessary…"*
(conditional). Split what the section fuses:

- **Permanent (the sentiment)** — exhaust the cheap signals before reaching
  for anything opaque. Never expires, vectors or no vectors.
- **For now (the mechanism)** — the *exclusion of a vector store*, gated on
  the evidence trigger the section already names.

Suggested minimal reframe (maintainer to apply in that doc — flagged here,
not edited from the consumer repo): *"…is not a gap to be filled later."* →
*"…is a deliberate constraint for now, not a permanent verdict,"* optionally
closing with *"the discipline is permanent; the exclusion is conditional."*

### Recommendation

File Path A as a planning ticket; keep Path B as an open question
(`pql decisions … --type question`) anchored to this evidence, so the
"No vectors" stance is revisited only against data, exactly as the
philosophy demands.

---

*Logged from clide (the consumer), not authored in a pql session — review
and triage/commit from pql.*
