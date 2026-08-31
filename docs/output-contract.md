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
| `--grep <regex>` | keep only records with a matching value; emits one record per line (T-113) |
| `--flat-search` | force the primitive path on any subcommand; strips `signals[]` and `connections[]` |
| `--quiet` | suppress stderr warnings |
| `--verbose` | emit per-phase timing diagnostics on stderr (`internal/telemetry/`) |

Output is JSON only (default array, `--pretty`, or `--jsonl`), with two sanctioned opt-in plain-text exceptions: `ticket new --id-only` (bare `T-NNN`) and `--oneline` — both reject the JSON shaping flags with exit `64` rather than mixing modes.

**Projection** (D-27, extended by T-74) applies to the planning list verbs (`ticket list`, `decisions list`) and the ranked verbs (`search`, `related`, `context`): `--fields <csv>` returns JSON rows with only the named keys in the requested order (unknown names exit `64` listing the valid set), `--fields '*'` returns all of them, and `--oneline` emits a plain-text index — `id<TAB>status<TAB>title` on the list verbs, `path<TAB>score` on the ranked ones. Two defaults are trimmed because one key dominates the payload: `ticket list` omits `description`, and the ranked verbs omit `signals[]` and `connections[]`. `--full` restores whole rows on both, and naming the key in `--fields` always returns it. The record-level show verbs (`ticket show`, `decisions show`) take `--fields` too (T-67), narrowing the **top level only** — the join-trees attached by `--with-context`/`--with-blockers`/`--with-children`/`--tree` are all-or-nothing. They return complete records by default and reject `--oneline`/`--full` as unknown flags with exit `64`. `ticket board` takes neither: its rows nest inside columns, so a field list would be ambiguous about which level it projects.

**Content filtering** (T-113). `--grep <regex>` keeps only the records with a value matching a case-insensitive RE2 pattern, on every read verb. It exists so a caller never has to pipe: the permission rules match the whole command string, so a pipeline containing pql matches no `pql` allow rule and costs an approval prompt, and reaching for `python3` to avoid that trades a prompt for an unbounded write grant.

It filters the **output buffer** — it runs last, over exactly the rows the call was otherwise going to emit — so `pql X --limit 5 --grep p` means what `pql X --limit 5 | grep p` means. That ordering is load-bearing: several verbs push `--limit` into the query, and filtering first would search a page the caller never asked for. `internal/cli/integration_test.go` pins the equivalence directly (`TestIntegration_GrepEqualsPipedGrep`) by running each verb twice and comparing.

Matching rows are emitted **one per line with no enclosing array**, each line individually valid JSON — a filtered result stays parseable as JSONL rather than becoming a text dump. Zero matches writes zero bytes at exit `0`, as `--oneline` already does. An unparseable pattern exits `64` naming it. `--pretty` is refused at exit `64` (an indented array cannot also be one record per line); `--jsonl` is accepted and redundant; `--oneline` composes, matching the emitted line.

Two places it deliberately diverges from a literal piped `grep`, both because the pipe's behaviour there is an artefact of the JSON envelope: **keys are never matched**, only values (otherwise `--grep status` would match every ticket, since every ticket has that key), and **anchors bind to a value rather than the rendered line** (`^governance/` works, where through a pipe every line starts with `{"path":`). It sees only what pql emits, never markdown body prose — that is still `grep`/`rg`'s job.

Like `--fields`, `--grep` does not reach the mutation verbs (D-30): their output is a receipt, and one that could be suppressed confirms nothing. They reject it at exit `64`, validated in the root `PersistentPreRunE` so the refusal lands *before* the write rather than after it.

Non-JSON renderers (`--table`, `--csv`) and JSONPath projection (`--select`) remain **not implemented**.

**Never null.** No surface emits `null` for an empty value. "Empty" then takes one of two forms, and callers must handle both: a scalar with no value is **omitted** (a ticket with no parent has no `parent_id`; `pql doctor` omits `index` entirely when there is no database — branch on `db.exists`, T-75), while an empty **collection** is present and empty (`meta` returns `"tags": []`, `plan export` returns `"files_written": []`, T-81). Test for presence *or* emptiness; never dereference a present key assuming it is populated.

One deliberate exception: the DSL emits `null` for a frontmatter column a file does not carry (`SELECT name, fm.lens` over a file with no `lens`). That is load-bearing — a projection over heterogeneous files has to distinguish absent from empty, and omitting the key would make the rows ragged. Documented as an exception in the embedded skill rather than papered over.

## `--flat-search` semantics

A global short-circuit. When set:
- Skip `internal/connect/` entirely.
- Result rows omit `signals[]` and `connections[]` regardless of the subcommand's default.
- Telemetry still works (`generate_ms` reported; no `rank_ms` since ranking is skipped).
- Exit codes unchanged.

The DSL path (`pql query <DSL>`) is already flat by default. (There is no positional `pql <QUERY>` form — the DSL is always invoked via `pql query`.)
