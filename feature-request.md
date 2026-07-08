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

## FR-2 — Rank the planning dataset against arbitrary text ("coverage overlap")

- **Status:** idea — needs a design decision before any code.
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
