# Intents

Intent-level commands sit above the primitive query surface. Each intent is a thin combination of (query primitives + enrichment profile + intent-specific signal weights).

This catalog is filled in as intents land. See `project-structure.md` for the architectural pattern (`internal/intent/<name>/` + `internal/cli/intent_<name>.go`).

## Catalog

> *No intents shipped yet. Below is the planned slate.*

### `pql related <path>`
Find files structurally and textually related to a given path. Default-on enrichment with one hop of neighborhood context.

### `pql search <query>`
Semantic-feeling textual search across the indexed corpus. Default-on enrichment; weights favour textual match + path proximity.

### `pql context <path>`
Assemble a primary + secondary context bundle around a path. Default-on enrichment; deeper neighborhood (up to 2 hops).

### `pql base <name>`
Execute an Obsidian `.base` file as a query. Bypasses enrichment by default — Bases describe exact filters, treat them as flat queries unless `--connect` is set.

## Per-intent contract

Every intent documents:

- **Trigger phrases** — what a caller is asking when they reach for this intent.
- **Inputs** — required and optional arguments, defaults.
- **Generation strategy** — which primitive queries it composes, which candidate sources it taps.
- **Weight profile** — which signals dominate, which are excluded.
- **Bundle shape** — primary set, secondary set, depth cap.
- **Output schema** — fields on each result row, including provenance (`signals[]`).

## Anti-pattern: the DSL is not an intent

`pql query <DSL>` and the positional `pql <QUERY>` form are the **escape hatch** — raw rows, no ranking, no provenance. They share the store and index but bypass `connect/`. Documented because the temptation to "just make the DSL an intent" recurs.

## The `--flat-search` flag

Any intent's enrichment can be disabled per-invocation with `--flat-search`. Reduces the intent to its underlying primitive query. Useful when the caller wants the exact result queried exactly where they're looking — see `output-contract.md`.
