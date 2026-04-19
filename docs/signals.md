# Signals

Catalog of the ranking signals used by `internal/connect/signal/`. Per the design philosophy: *"a good ranker combines many weak signals rather than relying on one strong one."* This document lists every signal, what it measures, where it shines, and where it fails — so weight tuning is informed and regressions are diagnosable.

This catalog is filled in as signals land. Each entry follows the template below.

## Template

```
### <name>

**Measures:** what numerical thing this signal computes.
**Source:** where the input data comes from (FTS5, links table, file metadata, …).
**Range / scale:** raw output range; whether normalization is needed.
**Shines on:** query types where this signal is highly predictive.
**Fails on:** query types where this signal misleads or returns garbage.
**Cost:** indicative cost (cheap / moderate / expensive); whether it's safe to compute on every result or only top-K.
**Provenance form:** what the signal records in its `Contribution{Name, Raw, Normalized, Weight}` so callers can trace its effect.
```

## Planned signals (v1)

> *None implemented yet. Each lands with its first intent.*

- **textual** — FTS5 BM25 over note bodies. Shines on paraphrase-tolerant lookup; fails on identifier-heavy queries.
- **centrality** — graph centrality over the wikilink graph. Shines on "important" notes; fails on stable leaf utilities.
- **recency** — file mtime decay. Shines on "what's relevant lately"; fails on stable core notes.
- **path-proximity** — directory distance between candidate and query context. Shines on "near where I'm working"; fails when topical relevance crosses folder boundaries.
- **identity** — exact name / wikilink / tag match. Shines on "where is X defined"; fails on paraphrase.
- **co-occurrence** — frequency two notes appear together in the same outlink set. Shines on neighborhood discovery; fails on isolated notes.

## Signal heterogeneity

Different signals produce scores on different scales with different distributions. The combiner in `internal/connect/rank.go` normalizes per-signal before weighting — see ADR `0003-signal-normalization.md` (planned) for the chosen approach.
