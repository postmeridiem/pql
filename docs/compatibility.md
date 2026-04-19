# Compatibility

How the `pql` binary, the SQLite index, and the Claude Code skill negotiate compatibility across versions.

## Versioned things

| Thing | Where | Versioning |
|---|---|---|
| Binary | `pql --version` / `pql version --build-info` | semver tag (`v0.X.Y`) |
| Index schema | `index_meta.schema_version` row in SQLite | monotonic integer (`internal/version.SchemaVersion`) |
| Skill | `skill/SKILL.md` frontmatter | semver tag, declares **min binary schema_version** |
| Output contract | `docs/output-contract.md` | tied to `schema_version` — bump implies contract change |

## Binary ↔ index

The binary owns the schema. On startup, if `index_meta.schema_version` doesn't match `internal/version.SchemaVersion`, the binary **drops the DB file and rebuilds**. The index is a pure cache, so this is safe and cheap. There is no migration code.

Stderr emits one warning when this happens:
```json
{"level":"warn","code":"store.schema.rebuild","msg":"index schema version mismatch; rebuilding","hint":"first run after upgrade"}
```

## Binary ↔ skill

The skill declares the minimum binary schema version it supports in its SKILL.md frontmatter. On invocation, the skill should:

1. Run `pql version --build-info` and parse the JSON.
2. Compare `schema_version` against its own minimum.
3. On mismatch, abort with a clear remediation hint (`upgrade pql to v0.X.Y or later`).

This avoids the failure mode where an old skill assumes a result shape that newer binaries no longer emit (or vice versa).

## When to bump `schema_version`

Bump for any of:
- New required column or table (rebuild needed for old indexes).
- Removed or renamed table/column referenced by `query/`.
- Change to the JSON shape on stdout (added optional fields are safe; removed or renamed fields are not).
- Change to exit-code semantics.

Don't bump for:
- Internal SQL refactors that produce identical query results.
- Lint, dependency, or doc-only changes.
- New CLI flags that add behaviour without changing existing output.

## Version display

`pql --version` → bare semver string (`v0.1.3`).
`pql version --build-info` → JSON:
```json
{
  "version": "v0.1.3",
  "commit": "abc1234",
  "date": "2026-04-19T16:00:00Z",
  "go_version": "go1.25.0",
  "schema_version": 1
}
```
