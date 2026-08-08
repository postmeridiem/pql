# Excluding files from indexing

Three layers, in increasing scope of "user can override":

1. **Built-in non-overridable defaults.** Hardcoded in the walker (`internal/index/walker.go`). The list is intentionally narrow — only `.git/`, matching git's own behaviour: git auto-excludes its state dir without anyone listing it, and `.git/objects/*` packs hold what amount to multiple snapshots of the entire repo, so descending into it would re-index every file many times over. Everything else, including pql's own `.pql/`, is the user's call to make via the layers below.
2. **`ignore_files` from `.pql/config.yaml`.** A list of file names (gitignore syntax) the walker reads at the vault root. Default `[".gitignore", ".pqlignore"]`. A name that isn't there is skipped, so both are inert until the file exists: most projects already have a `.gitignore` and most of its rules apply identically to indexing (build outputs, vendored deps, scratch dirs), and `.pqlignore` is there so the file pql names after itself works the moment you create it. `pql init` auto-appends `.pql/` to the project's `.gitignore`, so pql's own state stays out of the index without any extra config.
3. **`exclude:` patterns in `.pql/config.yaml`.** A flat list of doublestar patterns for one-off in-config rules. Applied on top of the file-based exclusions.

## Defaults

Out of the box, both `.gitignore` and `.pqlignore` are consulted — no config file needed, and `pql init` seeds the same pair explicitly. Run `pql files` in a typical git repo and pql skips `node_modules/`, `dist/`, `*.log`, etc., without you doing anything.

Neither file has to exist. A name in `ignore_files` that isn't on disk is skipped, so the default silently no-ops in a vault that is not a git repo and has no `.pqlignore`. Note that the `exclude:` config key carries its own default of `[**/.obsidian/**, **/.git/**, **/node_modules/**]`, so those are skipped even with no ignore file present (override by setting `exclude:` explicitly).

## Deviating: pql-specific rules without polluting `.gitignore`

A common case: you want to commit `drafts/` to git but you don't want pql to index those files. Adding `drafts/` to `.gitignore` would stop git from tracking them. The fix is a `.pqlignore` at the vault root containing just `drafts/`. That is all — it takes effect on the next index, with no config edit, because it is in the default set.

`.gitignore` covers what git already knew about; `.pqlignore` is a tiny file containing only your pql-specific deviations (`drafts/`, `experiments/`, whatever). Order matters — later files win on per-pattern conflicts, and `.pqlignore` comes second, so it can re-include something `.gitignore` excluded with the standard `!` prefix.

## Other deviation patterns

- **Use only a pql-specific file:** `ignore_files: [.pqlignore]`. Reasonable when pql's exclusions diverge significantly from git's.
- **Disable file-based exclusions entirely:** `ignore_files: []`. The built-in defaults still apply (you can never index `.git/`); use `exclude:` for anything else.
- **Multiple sources beyond the conventional pair:** `ignore_files: [.gitignore, .pqlignore, .myignore]`. Order matters; later wins.
- **One-offs not worth a file:** add to `exclude:` in config.yaml.

## Syntax

Each file in `ignore_files` is parsed with **identical syntax and semantics to `.gitignore`** (per `gitignore(5)`). Quick summary:

- One pattern per line. Blank lines and lines starting with `#` are ignored.
- Trailing whitespace is ignored unless escaped with `\`.
- A pattern with **no `/`** matches by basename anywhere: `node_modules` matches at any depth.
- A pattern with a `/` not at the end is anchored to the file's directory: `/build` matches only the top-level `build/`.
- A pattern ending in `/` matches **directories only**.
- `*` matches anything except `/`. `?` matches one non-`/` character. `[abc]` matches a character class.
- `**` matches zero or more directory segments: `**/foo`, `foo/**`, `a/**/b`.
- A pattern starting with `!` **re-includes** a previously excluded path. Order matters; later patterns override earlier ones.
- A pattern starting with `\` escapes the leading character.

Patterns are matched against the vault-relative path with `/` as the separator on every platform.

## The directory short-circuit

gitignore semantics include one well-known gotcha pql inherits:

> If a parent directory is excluded, files inside it cannot be re-included.

So this **does not** work:

```gitignore
private/
!private/keep.md
```

`private/` is excluded → pql never descends into it → `keep.md` is never tested against the negation. To make `keep.md` index-visible, either be more specific (`private/**` instead of `private/`) or use a `!` rule against the directory and re-exclude individual files inside.

## Performance

Even with multiple files, exclusion is cheap. Excluded directories are *pruned* from the walk (not "walked then filtered"), so `.git/` with thousands of objects costs zero scanning time. Compiled gitignore matchers are O(rules) per match; typical vaults have well under a hundred rules across all configured files.

## CLI debugging surface

`pql doctor` prints `config.ignore_files` so you can confirm which files the walker would consult. Future: `pql files --explain-ignored <path>` (post-v0.1) will report the rule + source file that excluded a given path.

## Implementation pointers

For the eventual `internal/index/ignore/` package:

- **Library:** [`github.com/sabhiram/go-gitignore`](https://github.com/sabhiram/go-gitignore) — pure Go, zero indirect deps, gitignore.5-compatible.
- **Walker integration:** load each file in `Config.IgnoreFiles` order at the vault root; compile to matchers. Each candidate path runs through the matchers in order; later wins. The built-in defaults from `walker.builtinExcludes` apply unconditionally on top.
- **`exclude:` translation:** convert each YAML pattern to a leading-`/`-anchored gitignore line and treat as the innermost (highest-priority) source.
- **Caching:** cache compiled matchers keyed by `(path, mtime)` so re-indexes don't re-parse unchanged files.
