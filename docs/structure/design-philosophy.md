# Design Philosophy

A local binary that indexes a repository and exposes it to Claude Code as a skill. This document describes how to think about the system, not how to build it. Implementation decisions follow from the philosophy; the philosophy does not follow from the implementation.

## The core bet

The binary is not a search engine. It is a **ranker that happens to have a query engine underneath**.

Everything downstream of that sentence — the tool surface, the result shapes, the permission model, the storage choices — should serve the premise that the binary's job is to turn a fuzzy intent into a small, well-ordered, explainable bundle of repository context. Not to return rows. Not to answer questions. To hand Claude the right next thing to look at, already pre-assembled.

If a design choice makes the binary a better *combiner*, it is probably correct. If it makes the binary a more capable *primitive*, it is probably wrong for this project, however technically impressive.

## Why not vectors

Vector retrieval is excluded by deliberate choice, not by accident. The available embedded options are not tier-one infrastructure, the tier-one options are not embeddable, and the class of queries where vectors clearly beat structured retrieval is narrower than the ecosystem narrative suggests — especially on code, where identifiers, paths, and structure carry most of the signal.

The absence of a vector layer is not a gap to be filled later. It is a design constraint that forces the ranker to do its job properly with the signals that are genuinely available: textual match, structural centrality, path proximity, symbol identity, recency, co-occurrence. These signals are cheap, inspectable, and composable. They reward careful weighting. They are the actual substance of the product.

If semantic retrieval becomes necessary, it will be because a specific failure mode has been observed repeatedly in production use — not because the roadmap assumed it would be.

## Intents, not primitives

The tool surface exposed to Claude is shaped around **what the caller wants to know**, not around **how the binary finds it**. A small number of intent-level tools, each backed by internal composition of many queries, beats a large number of primitive tools that Claude must chain.

This is not a stylistic preference. It is a response to three concrete properties of the agent environment:

- Each tool call is a permission event. Chains multiply friction and erode user trust in proportion to their length.
- Each tool call is a round trip. The binary's internal queries are orders of magnitude cheaper than the model's external ones.
- Each tool call is an opportunity for the model to drift. Composition inside the binary is deterministic; composition by the model is not.

The correct instinct when designing a new tool is to ask: *what does the caller actually want, at the level of intent, and what would a thoughtful human researcher hand back in response?* The answer to that question is the tool. Everything it does internally to produce that answer is implementation.

An escape hatch for low-level queries can exist, but it should be separately permissioned and rarely needed. If it is frequently needed, the intent-level surface is wrong and should be revisited — not supplemented.

## The binary orchestrates; the model decides

There is a tempting pattern where the model does the planning — chains primitive calls, combines results, decides what to fetch next — and the binary just answers one question at a time. This pattern is wrong for this project. It externalizes work the binary can do faster, cheaper, more consistently, and more testably.

The division of labor should be:

- **The model** handles semantic interpretation of the user's request, decides which intent to invoke, and reasons over the returned bundle to produce the user-facing answer.
- **The binary** handles local planning: candidate generation, signal computation, ranking, and assembly of the response bundle.

This mirrors a deeper pattern that shows up anywhere agent systems are built seriously: **the orchestrator is usually not the agent; the orchestrator is usually the tool.** The agent supplies goals and judgement. The tool supplies structured, bounded, reliable local work. Blurring this line in either direction produces systems that are slower, less predictable, and harder to debug.

## Candidate generation and ranking are different problems

Retrieval is two phases, not one. Treating them as one is a common source of mediocre results.

**Generation** is about recall. Cast a wide net cheaply: textual match, path glob, symbol lookup, fuzzy identifier match. Err toward including too many candidates rather than too few. The goal is to ensure the correct answer is *somewhere* in the candidate set.

**Ranking** is about precision. Spend real effort on a bounded set: compute expensive signals, combine them thoughtfully, return a small number of high-quality results. The goal is to ensure the correct answer is *near the top* of the returned set.

Keeping these phases architecturally separate has compounding benefits: generation stays fast even on large repositories, ranking stays tractable even with many signals, and the two can be tuned independently. Conflating them — for instance, by building expensive signals directly into SQL queries — makes both jobs harder.

## Ranking is the product

The quality of the ranker is the quality of the system. This deserves to be taken seriously as a first-class concern, not as an afterthought layered on top of a retrieval engine.

A good ranker has a few properties worth internalizing:

- **It combines many weak signals rather than relying on one strong one.** No single signal is reliable across query types. Textual match fails on paraphrase. Centrality fails on leaf utilities. Recency fails on stable core code. A weighted combination of signals, each individually mediocre, outperforms any one of them alone.

- **It is explicit about what signals it uses and why.** Hidden magic formulas are untunable and undebuggable. The ranker should know which signals it considered, what each contributed, and why the final order emerged. This isn't engineering fastidiousness; it's what makes the system improvable.

- **It weights signals differently for different intents.** A query asking "where is X defined" should weight exact symbol match and path specificity highly. A query asking "how does this codebase handle Y" should weight textual relevance and structural centrality highly. Same signals, different intents, different weights.

- **It handles signal heterogeneity honestly.** Different signals produce scores on different scales with different distributions. Pretending otherwise by naively summing raw scores produces a ranker dominated by whichever signal happens to have the largest numerical range. There are well-understood approaches to this problem; use one.

- **It degrades gracefully.** When signals are missing or unreliable for a given query, the ranker should notice and reweight rather than producing garbage with confidence.

## Explainability is not optional

Every result the binary returns should carry enough provenance for the caller to understand *why* it was returned. Not in a heavyweight audit-log sense — in a lightweight, routinely-consumed sense. Each hit says what matched, how it ranked, and what neighborhood it belongs to.

This has three concrete benefits:

- **The model can cite results to the user** without re-querying to verify them. The provenance travels with the result.
- **Wrong results become debuggable** in the binary, where the fault actually lives, rather than in the model's reasoning, where it doesn't.
- **The system becomes improvable over time** because misranked results are traceable to specific signal contributions rather than opaque overall scores.

Opaque ranking produces a system that is either right or mysteriously wrong. Transparent ranking produces a system that is right, or wrong in ways that suggest their own fix.

## Bundle shape matters more than result count

The instinct to return many results is almost always wrong. A bundle of ten well-chosen, well-annotated, pre-ranked results beats a bundle of a hundred that the caller has to re-filter. The binary has already done the work of knowing which results are good; withholding that judgement and dumping everything defeats the purpose of the ranker.

A good bundle has a shape: a primary set of directly-relevant hits, a secondary set of structurally adjacent context (callers, imports, siblings, referenced documentation), and enough metadata on each item that the caller can decide whether to drill further without another round trip. The next-obvious-move is pre-fetched. The caller rarely needs to chain, because the chain has already happened inside the binary.

Depth should be capped aggressively. One hop of structural context is almost always enough. Two hops is occasionally useful. Three hops is a sign that the intent was wrong for the query and the caller should be invoking a different tool.

## The storage substrate is load-bearing

The choice of SQLite with full-text search, fuzzy-match extensions, and relational joins is not incidental. It is the thing that makes the rest of the philosophy viable.

A single transactional store means indexing operations are atomic: a file and its derived data update together or not at all. A single on-disk artifact means backup, reset, and distribution are trivial. A rich query language means candidate generation and metadata joins can happen in one place rather than being stitched together in application code. Extension points for fuzzy matching and text search mean the signals the ranker depends on are built into the substrate rather than reimplemented above it.

Introducing a second store — for vectors, for caches, for anything — should be resisted unless the case is overwhelming. Every additional store is a new consistency problem, a new lifecycle to manage, a new failure mode, and a new thing for users to understand. The monolithic storage choice is doing real work; treat it as a feature, not as a limitation to route around.

## Indexing is a product surface too

How the binary builds its index is as important as how it serves queries. Two principles matter most:

- **Incremental, not wholesale.** Reindexing everything on every invocation is the easy path and the wrong one. The index should update in response to what has actually changed — file modifications, git state, explicit invalidation — and leave the rest alone. This is both a performance property and a correctness property: partial reindexes preserve derived state (centrality, cross-references) that would otherwise need to be recomputed.

- **Schema-aware and versioned.** The index carries metadata about how it was built: schema version, what was indexed, when. This makes it possible to detect when an index is stale relative to the binary's current expectations and reindex selectively. The absence of this metadata produces systems where "delete the cache and start over" becomes the universal debugging step, which is a sign the system has given up on reasoning about its own state.

The indexer is not a one-shot preprocessing step. It is a long-lived component whose job is to keep a useful representation of the repository continuously available, and it deserves the same design attention as the query path.

## What this system is not

It is worth being explicit about the things this system deliberately does not try to be, because the temptation to become them will arise.

- **It is not a code intelligence server.** It does not replace language servers, compilers, or tree-walking analysis tools that have access to types and semantics the index does not. Where those tools exist, they are better at their jobs.
- **It is not a universal RAG backend.** Its shape is tuned for repository context specifically. Generic document retrieval has different tradeoffs and should use different tools.
- **It is not a replacement for reading the code.** Its job is to help the model and the user find the right code to read, not to summarize it away.
- **It is not a search UI.** It is a programmatic interface consumed by an agent. Human-facing search has different ergonomics and different failure modes.

Staying narrow is the discipline that makes the system good at what it does.

## The governing intuitions

If the rest of this document is ever lost, these are the sentences worth keeping:

1. The binary is a ranker that happens to have a query engine underneath.
2. Expose intents, not primitives.
3. The orchestrator is the tool, not the agent.
4. Generate widely, rank carefully, return sparingly.
5. Every result carries its own explanation.
6. One store, one transaction, one artifact.
7. Staying narrow is the discipline.

Everything else follows.
