# Compatibility

How pql handles version mismatches between the binary, the SQLite index, and the Claude Code skill.

## The short version

- **Index out of date?** pql drops it and rebuilds automatically. The index is a cache.
- **Skill out of date?** Run `pql skill install` to update. `pql skill status` tells you if it's stale.
- **Binary out of date?** Run `pql self-update` to grab the latest release.

## What's versioned

| Component | Where it lives | How to check |
|---|---|---|
| Binary | `~/.local/bin/pql` | `pql version` |
| Index schema | `<vault>/.pql/index.db` | `pql version --build-info` → `schema_version` |
| Skill | `.claude/skills/pql/SKILL.md` | `pql skill status` |
| Planning DB | `<vault>/.pql/pql.db` | `schema_migrations` table |

## Binary ↔ index

The binary owns the index schema. If the on-disk `schema_version` doesn't match what the binary expects, it drops and rebuilds the index. This is safe because the index is a pure cache — everything in it is regenerated from your vault files.

You'll see a stderr warning when this happens:
```
{"level":"warn","code":"store.schema.rebuild","msg":"index schema version mismatch; rebuilding"}
```

No action needed. Next run is fast again.

## Binary ↔ skill

The skill is embedded in the binary and installed into your project by `pql skill install` or `pql init`. When you upgrade the binary, the embedded skill may be newer than the installed copy.

```sh
pql skill status     # shows: current, stale, modified, or missing
pql skill install    # updates to match the binary
```

If you've hand-edited the installed SKILL.md, `pql skill install` won't overwrite it. Use `--force` to replace it.

## Binary ↔ planning DB

The planning database (`pql.db`) uses forward-only migrations — it never drops data. When you upgrade the binary and new tables or columns are needed, they're added automatically on first access.

## Upgrading

```sh
pql self-update          # downloads latest, verifies SHA256, replaces atomically
pql skill install        # updates the Claude Code skill to match
```

Or manually: download from [Releases](https://github.com/postmeridiem/pql/releases/latest), replace the binary, run `pql skill install`.

## When schema_version bumps

The `schema_version` integer bumps when the index schema changes in a way that affects query results. This triggers an automatic rebuild on next run. It does **not** bump for:
- Internal refactors with identical output
- New CLI flags that add behavior without changing existing output
- Documentation or dependency changes
