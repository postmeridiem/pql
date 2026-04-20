# Vault layout

When `pql` operates on a vault it looks for a small set of conventional files and directories at (or under) the vault root. This page is the map of what each does.

| Name | Authored by | Purpose |
|---|---|---|
| `.pql.yaml` | User | Per-vault configuration. Optional. |
| `.pqlignore` | User | Gitignore-syntax exclusions. Optional. Spec: [`pqlignore.md`](pqlignore.md). |
| `.pql/` | pql | pql's per-vault state directory (SQLite index + future caches). Analogous to `.git/`. |

The convention follows the same shape developers already know from git: one user-authored config file, one user-authored ignore file, one tool-owned state directory. Users who recognise `.gitignore` + `.git/` will recognise `.pqlignore` + `.pql/` immediately.

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
- `db: <path>` in `.pql.yaml` → same, vault-scoped

In every case `<path>` is the DB **file** itself, not a directory; sidecar WAL/shm files land next to it.

### Should `.pql/` be committed to version control?

No. Add `.pql/` to your repo's `.gitignore`. The index is regenerable from the vault contents and caching it in git would bloat the repo with binary data that changes on every invocation. `pql init` (v0.1) appends `.pql/` to `.gitignore` automatically when one exists and the entry isn't already there.

## `.pqlignore` — excluding paths from indexing

Gitignore-compatible file. Full spec: [`pqlignore.md`](pqlignore.md). Quick summary:

- Vault-root `.pqlignore` is the primary file; nested `.pqlignore` files cascade like `.gitignore`.
- Set `respect_gitignore: true` in `.pql.yaml` to honor `.gitignore` rules as well.
- The `exclude:` list in `.pql.yaml` remains supported (additive, translated to gitignore-style patterns internally).
- A small set of paths (`.git/`, `.pql/`, sqlite sidecar files) is always excluded regardless of user config.

## `.pql.yaml` — per-vault configuration

Optional. When present, tunes the indexer and query layers for this vault.

Fields, defaults, and the YAML→Go validation contract live in `internal/config/config.go`; the user-facing example is in `structure/initial-plan.md` § "Config file". `pql init` (v0.1) seeds a sensible default.

A representative example:

```yaml
frontmatter: yaml
wikilinks: obsidian
tags:
  sources: [inline, frontmatter]
exclude:
  - "**/draft/**"
respect_gitignore: false
git_metadata: false
fts: false
aliases:
  members: "type = 'council-member'"
```

## Resolution at startup

A single `pql` invocation, in order:

1. Resolve the vault root (CLI/env/walk-up — see `internal/config/discover.go`).
2. Load `.pql.yaml` if present (vault-local, then `~/.config/pql/config.yaml` global).
3. Resolve the index location: `--db` / `PQL_DB` / `db:` override → otherwise `<vault>/.pql/index.sqlite` → fall back to user cache if the vault is read-only.
4. Open or create the index, applying or verifying the v1 schema.
5. Compose the active `.pqlignore` matcher stack: built-in defaults + `~/.config/pql/ignore` (planned) + nested `.pqlignore` files + `.gitignore` files (if `respect_gitignore: true`) + `exclude:` from YAML.
6. Hand control to the requested subcommand.

`pql doctor` prints exactly what each step resolved and why, so users never have to guess.
