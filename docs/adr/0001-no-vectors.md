# ADR 0001: No vector retrieval in v1

**Status:** accepted
**Date:** 2026-04-19

## Context

The default modern instinct for "build retrieval over a corpus" is "add a vector index." `pql` deliberately does not.

## Decision

No vector retrieval in v1 (or beyond, unless a specific failure mode forces it). Retrieval is built from cheap, inspectable, structured signals: textual match, structural centrality, path proximity, symbol identity, recency, co-occurrence.

## Why

Distilled from `structure/design-philosophy.md`:

- The available embedded options are not tier-one infrastructure; the tier-one options are not embeddable as a single static binary.
- Shipping vectors means shipping a second store — new consistency problem, new lifecycle, new failure mode.
- The class of queries where vectors clearly beat structured retrieval is narrower than the ecosystem narrative suggests, especially on code-and-prose repos where identifiers, paths, and structure carry most of the signal.
- The absence of a vector layer is not a gap — it is a constraint that forces the ranker to do its job properly with the signals genuinely available.

## Consequences

- The ranker is the product. Weight tuning and signal coverage are first-class engineering.
- Some paraphrase queries will miss. That is expected; users reach for ripgrep or grep when they need raw text recall.
- If a specific repeatable failure mode emerges in production where structured signals demonstrably can't compensate, revisit. Until then, declined.

## When this might change

- A specific class of agent task is shown, with examples, to fail on structured retrieval and succeed on semantic.
- A pure-Go embedded vector store reaches tier-one quality (size, recall, latency, build complexity).
- Both of the above, together. Either alone is insufficient.
