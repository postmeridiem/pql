# Intents

Intent commands sit above the primitive query surface. Each one generates candidates, scores them with ranked signals, and returns results with provenance — so you get "the most relevant files" instead of "all files matching a filter."

Use intents when you want ranked answers. Use primitives (`pql files`, `pql tags`, `pql query`) when you want exact results.

## Available intents

### `pql related <path>`

Find files structurally related to a given file.

```sh
pql related notes/design.md --pretty --limit 5
```

**When to use:** "What else should I read alongside this file?"

**How it works:** Gathers candidates from inlinks, shared tags, and same-directory files. Weights favor link overlap (0.35) and tag overlap (0.25) — structural similarity matters most.

**Output:** Ranked results with `signals[]` and `connections[]` (outlinks, inlinks, shared-tag siblings).

### `pql search <query>`

Ranked search across the vault by file paths, tags, frontmatter values, and headings.

```sh
pql search "authentication" --pretty --limit 10
```

**When to use:** "Find files about X, ranked by importance."

**How it works:** Matches the query text against paths, tags, frontmatter values, and heading text. Weights favor centrality (0.40) and recency (0.25) — hub notes and recently-edited files rank higher.

**Output:** Ranked results. Most central files that match the query come first.

### `pql context <path>`

Build a context bundle for understanding a file — what you'd want to read first.

```sh
pql context notes/design.md --pretty --limit 5
```

**When to use:** "What do I need to read to understand this file?"

**How it works:** Gathers candidates from both inlinks and outlinks plus tag siblings. Weights balance path proximity (0.25) and centrality (0.25) — nearby important files rank highest.

**Output:** The most useful neighbors of the target file, ranked.

## The `--flat-search` flag

Any intent can be stripped to raw rows with `--flat-search`:

```sh
pql related notes/design.md --flat-search
```

This bypasses the enrichment layer entirely — no signals, no connections, no ranking. Useful when you want the candidate set without opinions.

## For agents

Intent commands are designed for single-tool-call use by AI agents. One call returns a pre-ranked, explained context bundle — the agent doesn't need to chain primitives or compose queries. The `signals[]` array lets the agent cite why a result was returned.

The primitive surface (`pql query`, `pql files`, etc.) remains available as the escape hatch for exact queries.
