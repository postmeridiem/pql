# PQL grammar

PQL — **Project Query Language** — is a SQL-derived dialect for querying a markdown vault as if it were a relational store. The PQL DSL is the **escape hatch** layer of the binary (raw rows, no ranking, no provenance); intent-level commands sit above it. See `intents.md` for the intent surface and `output-contract.md` for the JSON shape on stdout.

## Why SQL-derived (not DQL-shaped)

Earlier sketches of this project described a Dataview-compatible dialect, but PQL is being designed to run *outside* Obsidian and to be primarily called by an agent. SQL gives us:

- **Familiarity** — every developer (and every model trained on text) reads SQL.
- **Composability** — predictable precedence, scope, and operator semantics out of the box.
- **Backend leverage** — we run on SQLite; matching SQLite's grammar where possible means fewer surprises and a smaller translation surface.

Dataview's framing (`LIST FROM "folder"`, `TABLE … FROM #tag`, `SORT`) was useful inside the Obsidian app where the surrounding context is implicit. Outside it, plain `SELECT … WHERE …` is clearer. We keep credit where due (see the README's *Inspiration* section).

A migration table for users coming from DQL is at the end of this document.

## Scope of v1

Inside scope:
- `SELECT [DISTINCT] … [FROM …] [WHERE …] [ORDER BY …] [LIMIT …]`
- A single canonical table (`files`) with scalar, array, and object virtual columns
- Frontmatter access via `fm.<key>` (typed JSON extraction)
- SQLite-style comparison and pattern operators (`=`, `<>`, `<`, `LIKE`, `GLOB`, `REGEXP`, `IN`, `BETWEEN`, `IS NULL`)
- FTS5 body search via `MATCH` when `fts: true` in config
- A small, sharp set of string / array / date / path / type functions

Out of scope in v1 (each errors with **exit code 65** and a hint pointing here):
- `JOIN` (any kind), subqueries, set ops (`UNION` / `INTERSECT` / `EXCEPT`), CTEs (`WITH`), window functions
- `GROUP BY` / `HAVING` / aggregates (`COUNT`, `SUM`, `AVG`, `MIN`, `MAX`)
- Any write operation (`INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER`) — PQL is read-only by design
- Inline fields in prose (`Rating:: 5` mid-paragraph)
- `dataviewjs` or any code evaluation
- Embeds (`![[…]]`) as a query target
- Non-Obsidian markdown dialects (Logseq, Roam) — extractor-registry hook in v2

The single-table model is sufficient for every intent currently planned; deferring `JOIN` and `GROUP BY` keeps the parser, evaluator, and planner small and predictable. They land in v1.1+ when a concrete use case forces the issue.

## EBNF (v1)

```
query        := select [ from ] [ where ] [ order_by ] [ limit ]

select       := "SELECT" [ "DISTINCT" ] proj_list
proj_list    := "*" | proj_item ( "," proj_item )*
proj_item    := expr [ "AS" ident ]

from         := "FROM" ident                  -- defaults to "files" when omitted
where        := "WHERE" expr
order_by     := "ORDER" "BY" sort_item ( "," sort_item )*
sort_item    := expr [ "ASC" | "DESC" ] [ "NULLS" ( "FIRST" | "LAST" ) ]
limit        := "LIMIT" int [ "OFFSET" int ]

expr         := or_expr
or_expr      := and_expr ( "OR" and_expr )*
and_expr     := not_expr ( "AND" not_expr )*
not_expr     := [ "NOT" ] cmp_expr
cmp_expr     := add_expr ( cmp_op add_expr )*
cmp_op       := "=" | "!=" | "<>" | "<" | "<=" | ">" | ">="
              | "LIKE" | "GLOB" | "REGEXP" | "MATCH"
              | "IN" | "NOT" "IN" | "BETWEEN" | "IS" [ "NOT" ] "NULL"
add_expr     := mul_expr ( ( "+" | "-" | "||" ) mul_expr )*
mul_expr     := unary ( ( "*" | "/" | "%" ) unary )*
unary        := [ "-" | "+" | "NOT" ] primary
primary      := literal
              | ref
              | call
              | "(" expr ")"
              | "(" expr ( "," expr )+ ")"     -- tuple for IN
ref          := ident ( "." ident | "[" STRING "]" )*
call         := ident "(" [ expr ( "," expr )* ] ")"

literal      := STRING | INT | FLOAT | BOOL | "NULL"
STRING       := "'" ( any-char | "''" )* "'"
ident        := unquoted_ident | "\"" any "\""
```

**Lexical notes:**
- Keywords are case-insensitive (`select`, `Select`, `SELECT` all parse). Convention: uppercase in written queries.
- Identifiers are case-sensitive and unquoted by default (`fm.voting`); double-quote them when they collide with reserved words or contain odd characters: `fm."order"`, `fm["key with spaces"]`.
- String literals use single quotes; `''` escapes a quote inside a string. No double-quoted strings (those are identifiers).
- Comments: `-- line` and `/* block */`, SQL-standard.
- Whitespace is irrelevant.

## The `files` table — virtual columns

Every query is implicitly `FROM files` unless stated otherwise. One row per indexed `.md` file.

### Scalar columns

| Column | Type | Notes |
|---|---|---|
| `path` | text | relative to vault root, e.g. `members/vaasa/persona.md` |
| `name` | text | basename without `.md` |
| `folder` | text | directory portion of `path` |
| `mtime` | datetime | filesystem modification time |
| `ctime` | datetime | filesystem creation time |
| `size` | int | bytes |

### Array columns (JSON arrays under the hood)

| Column | Element type | Notes |
|---|---|---|
| `tags` | text | every tag attached to the file (frontmatter + inline, configurable) |
| `inlinks` | text | paths of files that link **to** this one |
| `outlinks` | text | paths this file links **to** |
| `headings` | text | heading texts in document order (depths flattened) |

Array columns are queryable with `IN`, `NOT IN`, and the array helpers (`length`, `contains`, `position`, `slice`).

### Object column

`fm` — the file's frontmatter as a typed object. Access via `fm.<key>` or `fm['<key>']`. Returns the typed value (string / int / float / bool / array / null). Returns `NULL` when the key is absent. Underlying implementation: SQLite JSON1 (`json_extract`) on the `frontmatter.value_json` column.

### Conditional columns

| Column | Type | Available when |
|---|---|---|
| `body` | text | `fts: true` in `.pql/config.yaml` — only valid on the right of `MATCH` |
| `gitmtime` | datetime | `git_metadata: true` |
| `gitauthor` | text | `git_metadata: true` |

## Operators

Standard SQL: `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=`, `+`, `-`, `*`, `/`, `%`, `||` (string concat).

**Membership:** `<value> IN <array | tuple>` — true if the value appears in an array column or in a parenthesised tuple. Right-hand side is one of:
- An array column: `'council-member' IN tags`
- A tuple of literals: `fm.type IN ('council-member', 'council-session')`

**Pattern:**
- `LIKE` — SQL `LIKE` with `%` and `_` wildcards (case-insensitive on ASCII by default).
- `GLOB` — SQLite `GLOB` with `*` and `?` (case-sensitive, supports `**` for path globbing).
- `REGEXP` — POSIX regex. Pattern is a string literal.
- `MATCH` — FTS5 query. Only valid against `body`. Right-hand side is FTS5 query syntax (boolean operators, phrase quoting, prefix `*`).

**Range:** `BETWEEN x AND y` (inclusive both ends).

**Null:** `IS NULL`, `IS NOT NULL`. Comparison with `=`/`!=` against `NULL` is always `NULL` (i.e. false in `WHERE`), per SQL.

**Boolean:** `AND`, `OR`, `NOT` with standard precedence (`NOT` > `AND` > `OR`).

## Functions (v1)

**String:** `length(s)`, `upper(s)`, `lower(s)`, `trim(s)`, `ltrim(s)`, `rtrim(s)`, `substr(s, start[, len])`, `replace(s, from, to)`, `instr(s, substr)`, `concat(...)`, `printf(fmt, ...)`.

**Array:** `length(a)`, `contains(a, x)`, `position(a, x)`, `slice(a, start[, len])`. Note: `length` is polymorphic — works on strings and arrays.

**Date / time:** SQLite-style.
- `date(value [, modifier...])`, `time(...)`, `datetime(...)`, `julianday(...)`, `strftime(fmt, value [, modifier...])`
- `now()` — alias for `datetime('now')`
- `today()` — alias for `date('now')`

Modifiers follow SQLite: `'+1 day'`, `'-30 days'`, `'start of month'`, etc.

**Math:** `abs`, `min`, `max`, `round`, `ceil`, `floor`. (These are scalar — multi-argument `min`/`max` over expression lists, not aggregates.)

**Type / null:** `cast(x AS type)`, `coalesce(x, y, ...)`, `nullif(x, y)`. Types: `TEXT`, `INTEGER`, `REAL`, `BOOL`, `JSON`.

**Path:** `dirname(p)`, `basename(p)`, `extension(p)`.

**JSON:** `json(s)`, `json_extract(j, path)`, `json_array_length(j)`. Most users won't need these — `fm.<key>` is the ergonomic form for frontmatter access.

## Examples

Files in a folder (`FROM files` omitted, `name` projection):

```sql
SELECT name WHERE folder = 'members'
```

Files with a tag:

```sql
SELECT name WHERE 'council-member' IN tags
```

Files with multiple tags (intersection):

```sql
SELECT name WHERE 'council-member' IN tags AND 'voting' IN tags
```

Project frontmatter fields:

```sql
SELECT name, fm.prior_job
WHERE fm.type = 'council-member' AND fm.voting = true
ORDER BY name
```

Recently modified, top 10:

```sql
SELECT name, mtime
WHERE mtime > date('now', '-30 days')
ORDER BY mtime DESC
LIMIT 10
```

Files linking to a specific note (backlinks):

```sql
SELECT name WHERE 'members/vaasa/persona.md' IN outlinks
```

The targets a specific note links out to (one-row-with-array result; the agent unwraps client-side, or use `pql outlinks` for the ergonomic form):

```sql
SELECT outlinks WHERE path = 'members/vaasa/persona.md'
```

Path glob:

```sql
SELECT name WHERE path GLOB 'sessions/**/*.md'
```

Regex on file name:

```sql
SELECT name WHERE name REGEXP '^Dr\.'
```

Frontmatter range:

```sql
SELECT name, fm.date
WHERE fm.date BETWEEN date('2024-01-01') AND date('2024-12-31')
ORDER BY fm.date
```

Combined source/predicate:

```sql
SELECT name, fm.winner, fm.tied
WHERE 'council-session' IN tags AND fm.tied = true
ORDER BY fm.date DESC
```

FTS body search (when `fts: true`):

```sql
SELECT path WHERE body MATCH 'consensus AND vote' ORDER BY mtime DESC
```

Computed projection:

```sql
SELECT name, length(outlinks) AS outlink_count
WHERE folder = 'sessions'
ORDER BY outlink_count DESC
LIMIT 5
```

`DISTINCT` projection:

```sql
SELECT DISTINCT fm.type WHERE fm.type IS NOT NULL ORDER BY fm.type
```

## `.base` file compilation

Obsidian `.base` YAML is parsed and compiled to PQL AST, then run through the same evaluator. Example:

```yaml
filters:
  and:
    - file.tags contains "council-member"
    - voting == true
properties: [name, prior_job]
sort:
  - field: name
    direction: ASC
```

Compiles to:

```sql
SELECT name, fm.prior_job
WHERE 'council-member' IN tags AND fm.voting = true
ORDER BY name ASC
```

`pql base <name>` runs the compiled query. The mapping is documented in `internal/query/dsl/base/README.md` once that package lands.

## Migration from DQL

For users coming from Dataview, here are the common shapes side by side. PQL doesn't aim for source-level compatibility, but the conceptual mapping is direct.

| DQL | PQL |
|---|---|
| `LIST FROM "members"` | `SELECT name WHERE folder = 'members'` |
| `LIST FROM #council-member` | `SELECT name WHERE 'council-member' IN tags` |
| `TABLE name, prior_job FROM "members"` | `SELECT name, fm.prior_job WHERE folder = 'members'` |
| `WHERE voting = true` | `WHERE fm.voting = true` |
| `SORT name ASC` | `ORDER BY name ASC` |
| `LIMIT 5` | `LIMIT 5` |
| `WHERE name =~ /^Dr\./` | `WHERE name REGEXP '^Dr\.'` |
| `WHERE file.mtime > date(today) - dur("30 days")` | `WHERE mtime > date('now', '-30 days')` |
| `length(file.outlinks)` | `length(outlinks)` |
| `file.link` | `path` (renderers can format wikilinks via `--table` / `--csv`) |

DQL features without a PQL v1 equivalent (each errors with exit 65):

- `TASK` / `CALENDAR` result modes
- `GROUP BY` / `FLATTEN`
- Inline fields (`Rating:: 5` mid-paragraph)
- `dataviewjs`

## Errors

PQL parse and evaluation errors return **exit code 65** with a stderr diagnostic of the form:

```json
{"level":"error","code":"pql.parse.unexpected_token","msg":"unexpected token 'FOO' at line 1, col 23","hint":"see docs/pql-grammar.md"}
```

Error codes follow the pattern `pql.<phase>.<kind>`:
- `pql.lex.<kind>` — lexer errors (e.g. `unterminated_string`)
- `pql.parse.<kind>` — parser errors (e.g. `unexpected_token`, `expected_keyword`)
- `pql.eval.<kind>` — evaluation errors (e.g. `unknown_column`, `type_mismatch`, `unsupported_v1`)

Every parse/eval error includes line and column when applicable. For multi-line queries (`--file`, `--stdin`), positions are relative to the source.

## Reserved words

The keywords below are reserved at the top level. Use them as identifiers only when double-quoted (`fm."order"` or `fm['order']`):

```
SELECT  DISTINCT  FROM  WHERE  ORDER  BY  ASC  DESC  NULLS  FIRST  LAST  LIMIT  OFFSET
AND  OR  NOT  IS  NULL  IN  BETWEEN  LIKE  GLOB  REGEXP  MATCH  AS  CAST
TRUE  FALSE
```

## Implementation pointer

This document tracks the spec; the parser in `internal/query/dsl/parse/` is the source of truth. When the two diverge, the parser wins and this document is updated. Each grammar rule above maps to a parser function of the same name (e.g. `parseOrExpr`); the lexer's token kinds map onto the terminal symbols.

The base-file compiler lives at `internal/query/dsl/base/` and produces the same AST the parser produces.
