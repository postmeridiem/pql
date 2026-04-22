# Signals

Catalog of the ranking signals used by `internal/connect/signal/`. Per the design philosophy: *"a good ranker combines many weak signals rather than relying on one strong one."*

## How signals work

Each signal implements the `Signal` interface: given a query context and a candidate path, return a raw score. The ranker (`internal/connect/rank.go`) max-normalizes raw scores per signal across all candidates, then applies intent-specific weights. The weighted sum is the final score.

Provenance travels with every result as `signals[]` — each entry records `{name, raw, normalized, weight, weighted}` so rankings are traceable to specific signal contributions.

## Shipped signals

### link_overlap

**Measures:** number of shared outlink targets between the target path and a candidate.
**Source:** `links` table; self-join on `target_path`.
**Range:** 0 to N (unbounded integer count); normalized by max across candidates.
**Shines on:** "related to this file" queries where structural similarity matters.
**Fails on:** files with no outlinks; queries without a target path.
**Cost:** cheap (one indexed join per candidate).

### tag_overlap

**Measures:** number of shared tags between the target path and a candidate.
**Source:** `tags` table; self-join on `tag`.
**Range:** 0 to N; normalized by max.
**Shines on:** tag-heavy vaults where tagging discipline is consistent.
**Fails on:** vaults with no tags or inconsistent tagging; files with many generic tags (dilutes signal).
**Cost:** cheap.

### path_proximity

**Measures:** directory-tree distance between the target path and a candidate. Same directory = 1.0, decreasing with shared-prefix ratio.
**Source:** path strings (no DB query).
**Range:** 0.0 to 1.0 (already normalized).
**Shines on:** "near where I'm working" queries; vaults with meaningful directory structure.
**Fails on:** flat vaults; topical relevance that crosses folder boundaries.
**Cost:** trivial (string comparison).

### recency

**Measures:** how recently the candidate was modified. Linear decay from 1.0 (just now) to 0.0 (90 days ago).
**Source:** `files.mtime` column.
**Range:** 0.0 to 1.0 (already normalized).
**Shines on:** "what's relevant lately" queries; active editing sessions.
**Fails on:** stable core notes that are old but important; vaults where mtime is unreliable (git checkouts reset mtime).
**Cost:** cheap (one indexed lookup per candidate).

### centrality

**Measures:** inbound link count — how many distinct files link to this candidate.
**Source:** `links` table; COUNT(DISTINCT source_path) with pragmatic target resolution.
**Range:** 0 to N; normalized by max.
**Shines on:** finding hub/index notes; structurally important files.
**Fails on:** leaf notes that are valuable but rarely linked; new files.
**Cost:** cheap.

## Default weight profiles

| Signal | related | search | context |
|---|---|---|---|
| link_overlap | 0.35 | 0.10 | 0.30 |
| tag_overlap | 0.25 | 0.15 | 0.15 |
| path_proximity | 0.20 | 0.10 | 0.25 |
| recency | 0.05 | 0.25 | 0.05 |
| centrality | 0.15 | 0.40 | 0.25 |

Weights are defined per intent in `internal/intent/<name>/<name>.go`. The default profile in `internal/connect/bundle.go` is used when no intent-specific weights are provided.

## Normalization

Max-normalization per signal across the candidate set. Each signal's raw scores are divided by the maximum absolute value in that batch. This puts all signals on [0, 1] before weighting, preventing signals with larger raw ranges from dominating.

## Future signals

- **textual** — FTS5 BM25 over note bodies. Requires `fts: true` in config.
- **co-occurrence** — frequency two notes appear in the same outlink set.
- **identity** — exact name/wikilink/tag match against the query text.
