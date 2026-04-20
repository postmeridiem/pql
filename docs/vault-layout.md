# Vault layout

When `pql` operates on a vault it looks for a small set of conventional files and directories at (or under) the vault root. This page is the map of what each does.

| Name | Authored by | Purpose |
|---|---|---|
| `.pql/` | pql | Per-vault state directory (SQLite index + future caches + the user-edited config.yaml). Analogous to `.git/`. |
| `.pql/config.yaml` | User | Per-vault settings — overrides binary defaults. Optional. |
| `.gitignore` | User | Default exclusion source. Honored by pql out of the box; users can swap or extend via `ignore_files` in config (see [`pqlignore.md`](pqlignore.md)). |

The convention is the same shape developers already know from git: a tool-managed state dir at the vault root that also hosts the user-edited config (just like `.git/` hosts `.git/config`). Exclusions piggy-back on the project's existing `.gitignore` by default — most projects already keep their "what to skip" list there and the rules apply identically to indexing. Users who want pql-specific deviations add a small `.pqlignore` and update `ignore_files: [.gitignore, .pqlignore]`.

## `.pql/` — per-vault state

`pql` stores its index and any derived per-vault data inside the vault at `<vault>/.pql/`. Contents in v1:

- `index.sqlite` — the SQLite index described in `structure/initial-plan.md` § "The SQLite index".
- `index.sqlite-wal`, `index.sqlite-shm` — SQLite WAL sidecar files.

Files that may join later (additive, no schema break):

- `bases/<name>.ast.json` — compiled `.base` ASTs (cache for `pql base <name>`)
- `ranking/weights.json` — exportable weight profile if a user wants to version-control it alongside the vault
- `skill/version.json` — last successful skill compatibility handshake
- `watch.pid` — set by `pql watch` while a foreground watcher is running for this vault; removed on graceful shutdown. See [`watching.md`](watching.md).

### Why in-vault instead of a user cache directory

Co-locating pql's state with the vault matches the mental model from `.git/`:

- **Moving the vault moves its index.** Copy a vault to a new machine and pql picks up where it left off — no warm-cache step.
- **No orphans.** Delete the vault, the cache goes with it. No `rm ~/.cache/pql` hygiene.
- **Discoverable.** `ls -a` shows `.pql/` next to `.git/` and `.pqlignore`. Users know where to look without reading docs.
- **Per-vault by construction.** Two vaults on one machine never share state, same as `.git/` never shares.

The trade-off is that pql writes inside the vault directory. This does not violate the "pql doesn't modify user content" rule from `structure/design-philosophy.md` — `.pql/` is metadata, not content, in exactly the way `.git/` is metadata. Notes, frontmatter, and links are never touched.

### Read-only vaults / shared checkouts

For vaults where pql can't write — read-only mounts, ephemeral CI checkouts, shared network filesystems — pql falls back to a per-user cache at `<user-cache>/pql/<vault-fingerprint>/`:

- Linux: `${XDG_CACHE_HOME:-~/.cache}/pql/<fingerprint>/index.sqlite`
- macOS: `~/Library/Caches/pql/<fingerprint>/index.sqlite`
- Windows: `%LocalAppData%\pql\<fingerprint>\index.sqlite`

The fallback triggers when creating `<vault>/.pql/` fails with `EROFS`, `EACCES`, or `EPERM`. `pql doctor` reports which path is in use and which rule chose it.

### Overrides

Explicit control bypasses the default chain:

- `--db <path>` flag → exact DB file path
- `PQL_DB` env var → same
- `db: <path>` in `.pql/config.yaml` → same, vault-scoped

In every case `<path>` is the DB **file** itself, not a directory; sidecar WAL/shm files land next to it.

### Should `.pql/` be committed to version control?

No. Add `.pql/` to your repo's `.gitignore`. The index is regenerable from the vault contents and caching it in git would bloat the repo with binary data that changes on every invocation. `pql init` (v0.1) appends `.pql/` to `.gitignore` automatically when one exists and the entry isn't already there.

## Excluding paths from indexing

Three layers, fully documented in [`pqlignore.md`](pqlignore.md). Quick summary:

- **`ignore_files` in `.pql/config.yaml`** — defaults to `[.gitignore]`. Walker reads each named file with gitignore syntax. Add `.pqlignore` (or anything else) for pql-specific deviations; order matters.
- **`exclude:` in `.pql/config.yaml`** — flat list of doublestar patterns for one-off in-config rules.
- **Built-in non-overridable defaults** (`.git/`, `.pql/`, sqlite sidecars) — always excluded regardless of user config. Same idea as `.gitignore` not needing to list `.git/`: the tool knows what it owns.

## `.pql/config.yaml` — per-vault configuration

Optional. When present, tunes the indexer and query layers for this vault.

Fields, defaults, and the YAML→Go validation contract live in `internal/config/config.go`; the user-facing example is in `structure/initial-plan.md` § "Config file". `pql init` (v0.1) seeds a sensible default.

A representative example:

```yaml
frontmatter: yaml
wikilinks: obsidian
tags:
  sources: [inline, frontmatter]
ignore_files: [.gitignore]
exclude:
  - "**/draft/**"
git_metadata: false
fts: false
aliases:
  members: "type = 'council-member'"
```

## Resolution at startup

A single `pql` invocation, in order:

1. Resolve the vault root (CLI/env/walk-up — see `internal/config/discover.go`).
2. Load `.pql/config.yaml` if present (vault-local, then `~/.config/pql/config.yaml` global).
3. Resolve the index location: `--db` / `PQL_DB` / `db:` override → otherwise `<vault>/.pql/index.sqlite` → fall back to user cache if the vault is read-only.
4. Open or create the index, applying or verifying the v1 schema.
5. Compose the active ignore matcher stack: built-in non-overridable defaults + each file in `ignore_files` (in order, later wins) + `exclude:` patterns from YAML (highest priority).
6. Hand control to the requested subcommand.

`pql doctor` prints exactly what each step resolved and why, so users never have to guess.
