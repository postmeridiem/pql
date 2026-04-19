# PQL grammar

The PQL DSL is the **escape hatch** layer of the binary — raw query results, no ranking, no provenance. Intent-level commands sit above it. See `intents.md` and `output-contract.md`.

The full v1 grammar is specified in `structure/initial-plan.md` § "The query language (PQL)". This document will hold:

- the canonical EBNF (mirrored from the parser, not from `initial-plan.md`),
- worked examples grouped by feature,
- a mapping from each grammar rule to the lexer token / parser function that implements it,
- error-message catalogue with line/col examples,
- the list of explicitly unsupported v1 forms (with the exit code each produces).

It is filled in alongside `internal/query/dsl/` — the parser is the source of truth, this doc tracks it.

## Quick reference (v1)

```
LIST FROM "members" WHERE voting = true SORT name ASC

TABLE winner, date, tied FROM #council-session WHERE tied = true SORT date DESC

TABLE file.link AS session, votes FROM "sessions" SORT file.mtime DESC LIMIT 5

LIST FROM "members" WHERE name =~ /^Dr\./

TABLE file.name, length(file.outlinks) AS outlinks
  FROM "sessions"
  WHERE file.mtime > date(today) - dur("30 days")
  SORT outlinks DESC
```

## Explicitly unsupported in v1

Each of these errors with **exit code 65** and a clear message pointing here:

- `TASK` and `CALENDAR` result modes
- `GROUP BY` and `FLATTEN`
- Inline fields in prose (`Rating:: 5` mid-paragraph)
- `dataviewjs` / any JavaScript evaluation
- Embeds and transclusions (`![[...]]`)
- Non-Obsidian markdown dialects (Logseq, Bear, Roam) — extractor-registry hook in v2
