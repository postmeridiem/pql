# Output contract

This contract is what makes `pql` safe for AI agents to call. Stable across versions; changes require a `schema_version` bump and a compatibility note.

## Streams

- **stdout:** data, always JSON. JSON array by default; `--jsonl` for one object per line.
- **stderr:** diagnostics, JSON-per-line. Each line: `{"level":"warn|error","code":"…","msg":"…","hint":"…"}`. See `internal/diag/diag.go`.

## Exit codes

| Code | Name | Meaning |
|---:|---|---|
| `0` | OK | success — **including zero results** (empty `[]` on stdout) |
| `64` | EX_USAGE | bad CLI flag |
| `65` | EX_DATAERR | PQL parse or evaluation error |
| `66` | EX_NOINPUT | vault root not found / unreadable |
| `69` | EX_UNAVAILABLE | index corruption / migration failure |
| `70` | EX_SOFTWARE | internal error |

**Zero results is success, not a special code.** A query that matched nothing
exits `0` and emits an empty array `[]` on stdout (JSONL emits nothing). The
empty result lives in the data, never in the exit code — so callers using the
near-universal "non-zero means failure" convention (`set -e`, `subprocess`,
CI steps, agent harnesses) treat a zero-row query correctly without special
casing. Errors are the `64`–`70` range, always paired with a stderr
diagnostic. Exit code `2` was retired in this regime (it formerly meant
"no match"); see [D-22](../governance/decisions/architecture.md#d-22-zero-results-returns-exit-0-no-distinct-no-match-code).

## Result row shape

Primitive query (no enrichment):

```json
{ "path": "members/vaasa/persona.md", "name": "vaasa", "tags": ["council-member"], … }
```

Enriched (default-on intent):

```json
{
  "path": "members/vaasa/persona.md",
  "name": "vaasa",
  "tags": ["council-member"],
  "signals": [
    { "name": "link_overlap",   "raw": 0.82, "normalized": 0.91, "weight": 0.40, "weighted": 0.36 },
    { "name": "centrality",     "raw": 0.13, "normalized": 0.31, "weight": 0.20, "weighted": 0.06 },
    { "name": "path_proximity", "raw": 0.50, "normalized": 0.62, "weight": 0.40, "weighted": 0.25 }
  ],
  "score": 0.71,
  "connections": [
    { "path": "sessions/<slug>/outcome.md", "relation": "outlink" },
    { "path": "members/vaasa/journal.md",   "relation": "shared_tags" }
  ]
}
```

`signals[]` and `connections[]` are absent in primitive output and present in enriched output. There is no half-state.

## Global flags affecting output

| Flag | Effect |
|---|---|
| `--pretty` | pretty-print stdout JSON |
| `--jsonl` | emit JSON lines instead of an array |
| `--limit <n>` | clamp output rows; overrides PQL `LIMIT` |
| `--flat-search` | force the primitive path on any subcommand; strips `signals[]` and `connections[]` |
| `--quiet` | suppress stderr warnings |
| `--verbose` | emit per-phase timing diagnostics on stderr (`internal/telemetry/`) |

Output is JSON only (default array, `--pretty`, or `--jsonl`), with two sanctioned opt-in plain-text exceptions: `ticket new --id-only` (bare `T-NNN`) and `--oneline` — both reject the JSON shaping flags with exit `64` rather than mixing modes.

**Projection** (D-27, extended by T-74) applies to the planning list verbs (`ticket list`, `decisions list`) and the ranked verbs (`search`, `related`, `context`): `--fields <csv>` returns JSON rows with only the named keys in the requested order (unknown names exit `64` listing the valid set), `--fields '*'` returns all of them, and `--oneline` emits a plain-text index — `id<TAB>status<TAB>title` on the list verbs, `path<TAB>score` on the ranked ones. Two defaults are trimmed because one key dominates the payload: `ticket list` omits `description`, and the ranked verbs omit `signals[]` and `connections[]`. `--full` restores whole rows on both, and naming the key in `--fields` always returns it. Record-level verbs (`ticket show`, `decisions show`) return complete records and reject the flags with exit `64`.

Non-JSON renderers (`--table`, `--csv`) and JSONPath projection (`--select`) remain **not implemented**; pipe to `jq` for anything beyond field selection.

**Empty means omitted, never null.** A key with no value is left out of the object rather than emitted as `null`, so a caller checks for presence and never dereferences a null it just presence-checked. This binds every surface, including diagnostics: `pql doctor` omits `index` entirely when there is no database to count rows in (T-75) — branch on `db.exists`.

## `--flat-search` semantics

A global short-circuit. When set:
- Skip `internal/connect/` entirely.
- Result rows omit `signals[]` and `connections[]` regardless of the subcommand's default.
- Telemetry still works (`generate_ms` reported; no `rank_ms` since ranking is skipped).
- Exit codes unchanged.

The DSL path (`pql query <DSL>`) is already flat by default. (There is no positional `pql <QUERY>` form — the DSL is always invoked via `pql query`.)
