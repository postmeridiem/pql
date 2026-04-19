# Output contract

This contract is what makes `pql` safe for AI agents to call. Stable across versions; changes require a `schema_version` bump and a compatibility note.

## Streams

- **stdout:** data, always JSON. JSON array by default; `--jsonl` for one object per line.
- **stderr:** diagnostics, JSON-per-line. Each line: `{"level":"warn|error","code":"…","msg":"…","hint":"…"}`. See `internal/diag/diag.go`.

## Exit codes

| Code | Name | Meaning |
|---:|---|---|
| `0` | OK | success with ≥1 result |
| `2` | NoMatch | success with **zero** results — intentional, **not an error** |
| `64` | EX_USAGE | bad CLI flag |
| `65` | EX_DATAERR | PQL parse or evaluation error |
| `66` | EX_NOINPUT | vault root not found / unreadable |
| `69` | EX_UNAVAILABLE | index corruption / migration failure |
| `70` | EX_SOFTWARE | internal error |

The distinction between `0` and `2` is load-bearing. A zero-row query must never look like a bug.

## Result row shape

Primitive query (no enrichment):

```json
{ "path": "members/vaasa/persona.md", "name": "vaasa", "tags": ["council-member"], … }
```

Enriched (`--connect` or default-on intent):

```json
{
  "path": "members/vaasa/persona.md",
  "name": "vaasa",
  "tags": ["council-member"],
  "signals": [
    { "name": "textual",        "raw": 0.82, "normalized": 0.91, "weight": 0.4 },
    { "name": "centrality",     "raw": 0.13, "normalized": 0.31, "weight": 0.2 },
    { "name": "path-proximity", "raw": 0.50, "normalized": 0.62, "weight": 0.4 }
  ],
  "score": 0.71,
  "connections": [
    { "path": "sessions/<slug>/outcome.md", "via": "outlink", "hops": 1 },
    { "path": "members/vaasa/journal.md",   "via": "co-folder", "hops": 1 }
  ]
}
```

`signals[]` and `connections[]` are absent in primitive output and present in enriched output. There is no half-state.

## Global flags affecting output

| Flag | Effect |
|---|---|
| `--pretty` | pretty-print stdout JSON |
| `--jsonl` | emit JSON lines instead of an array |
| `--table` | human-readable ad-hoc table (auto-colour; `--no-color` to override) |
| `--csv` | CSV for spreadsheet import |
| `--select <jsonpath>` | project into a JSONPath expression (avoids piping to `jq`) |
| `--limit <n>` | clamp output rows; overrides PQL `LIMIT` |
| `--flat-search` | force the primitive path on any subcommand; strips `signals[]` and `connections[]` |
| `--quiet` | suppress stderr warnings |
| `--verbose` | emit per-phase timing diagnostics on stderr (`internal/telemetry/`) |

## `--flat-search` semantics

A global short-circuit. When set:
- Skip `internal/connect/` entirely.
- Result rows omit `signals[]` and `connections[]` regardless of the subcommand's default.
- Telemetry still works (`generate_ms` reported; no `rank_ms` since ranking is skipped).
- Exit codes unchanged.

The DSL path (`pql query <DSL>`, positional `pql <QUERY>`) is already flat by default.
