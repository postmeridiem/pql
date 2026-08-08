# Version axes

pql versions four things independently. They move on their own schedules, and a
release that changes none of them is the common case. This document exists
because a consumer diagnosing an upgrade needs to compare what it holds on disk
against what a binary speaks, and needs to be able to reconstruct that
comparison months later.

All four are declared in `project.yaml` and mirrored in `internal/version`.
`internal/version/version_test.go` parses `project.yaml` and fails on drift, so
the two cannot diverge. Every one is reported by:

```
pql version --build-info
```

## The axes

| Axis | Governs | Recovery when a version is behind |
|---|---|---|
| `schema_version` | `index.db` — the vault query cache | Dropped and rebuilt. It is a pure cache; nothing is lost. |
| `planning_schema_version` | `pql.db` — the planning store's table shape | Forward migration steps, recorded in the database's `schema_migrations` ledger. Failing that, `rm .pql/pql.db && pql plan rebuild`. |
| `canonical_version` | The row-canonicalisation rules behind every planning row's content hash | Rows record the version they were hashed under, so re-hashing is lazy. A changelog declaring a version the binary does not speak is refused. |
| `changelog_format` | `.pql/changelog/**.sql` — the on-disk replication format | Forward migration in place via `pql plan upgrade`. **Not** regenerable: the changelog is the log of record. |

The asymmetry in that last column is the whole reason a migration runner exists
(D-28). `index.db` and `pql.db` can be thrown away and rebuilt. The changelog
cannot — there is nothing behind it to rebuild *from* — so it has to be carried
forward instead.

## What happens on a mismatch

| Situation | Behaviour |
|---|---|
| Artefact older than the binary, and a step exists | Migrated forward. For the changelog this happens on `git pull` (the `post-merge` hook) or on an explicit `pql plan upgrade`. |
| Artefact older, no step reaches the current version | Refused, with the axis's own recovery hint. |
| Artefact **newer** than the binary | Refused. A format this binary does not know may encode rows it would misread, and there is no backward step. Upgrade pql. |
| `plan import` / `plan rebuild` meeting an older changelog | Replays, with a loud `pql.plan.format_stale` warning naming both versions and the verb that fixes it. These paths never rewrite files. |

## Release map

Which release moved which axis. Add a row when an axis is bumped.

| Release | index.db `schema_version` | canonical | pql.db schema | changelog format |
|---|---|---|---|---|
| 1.6.0 | 1 | 1 | — | unversioned |
| 1.9.0 | 1 | 2 | — | unversioned |
| 1.11.0 | 1 | 2 | — | unversioned |
| 2.0.0 | 1 | 2 | **2.0.0** | **2.0.0** |

Note the two schemes. The **migrated** axes carry the release that introduced
their format, so a marker reading `2.0.0` tells you both that you are behind and
which pql changed it. The **counters** — index.db's `schema_version` and the
per-row `canonical_version` — stay integers: they are written per database or per
row and compared for equality, so widening them would be a data migration for no
readability gain.

"Unversioned" means the artefact carried no marker at all. A changelog with no
`0000-format.sql` is reported as `1.11.0`, the last release before formats were
versioned, because "your changelog is in the 1.11.0-era format" is actionable
where an empty string is not. `pql.db` had no declared schema version before
2.0.0; databases from earlier releases are stamped once their column shape
verifies.

## Adding an axis version

1. Bump the number in `project.yaml` **and** `internal/version/version.go`. The
   parity test fails if you do one and not the other.
2. For a migrated axis, add the `migrate.Step` that produces the new version —
   `planning.schemaSteps` for pql.db, `formatAxis` in
   `internal/planning/changelog/format.go` for the changelog.
3. Add a row to the release map above.
4. Note the bump in `CHANGELOG.md`, including what a consumer has to do.
