# `.pqlignore`

Per-vault ignore file with **identical syntax and semantics to `.gitignore`**. Tells `pql` which files and directories to skip when indexing.

This document is the spec; the implementation lands under `internal/index/ignore/` as part of v0.1's walker work.

## Why a separate file (not just `exclude:` in `.pql.yaml`)

The current `exclude:` field in `.pql.yaml` is a flat list of doublestar patterns. That's enough for "exclude `.obsidian/` and `node_modules/`" but breaks down for:

- **Per-subtree rules.** "Index `members/` but not `members/*/scratch/`" is awkward as a single global glob list.
- **Negations.** "Exclude `drafts/` except `drafts/published/`" needs `!` re-inclusion, which doublestar doesn't model.
- **Familiarity.** Every developer reads `.gitignore` syntax fluently. Inventing a parallel system makes users translate their mental model for no gain.
- **Reuse.** Vaults that are already git repos often have a `.gitignore` whose rules apply identically to indexing — see "Honoring `.gitignore`" below.

`.pqlignore` and `.pql.yaml`'s `exclude:` coexist (additive) so existing configs keep working. See "Interaction with `exclude:`" below.

## Syntax

Identical to `gitignore(5)`. Verbatim summary for convenience; the canonical reference is the git man page:

- One pattern per line. Blank lines and lines starting with `#` are ignored.
- Trailing whitespace is ignored unless escaped with `\`.
- A pattern with **no `/`** matches by basename in any directory: `node_modules` matches `members/foo/node_modules` and `node_modules` at the root.
- A pattern with **a `/` not at the end** is anchored to the directory containing the `.pqlignore`: `/build` matches only the top-level `build/`, not `members/build/`.
- A pattern ending in `/` matches **directories only**: `tmp/` matches a directory called `tmp` but not a file with that name.
- `*` matches anything except `/`. `?` matches one non-`/` character. `[abc]` matches a character class.
- `**` matches zero or more directory segments: `**/foo` matches `foo` at any depth; `foo/**` matches everything under `foo`; `a/**/b` matches `a/b`, `a/x/b`, `a/x/y/b`, …
- A pattern starting with `!` **re-includes** a previously excluded path. Order matters: later patterns override earlier ones.
- A pattern starting with `\` escapes the leading character (e.g. `\!literal-bang`, `\#literal-hash`).

Patterns are matched against the path **relative to the `.pqlignore`'s directory**, using `/` as the separator on every platform (Windows backslashes are normalized).

### Examples

```gitignore
# Cache and build dirs
.obsidian/
.cache/
build/

# Hidden scratch files anywhere
*.tmp
**/scratch/

# Index drafts, EXCEPT the published subfolder
drafts/
!drafts/published/

# Anchored: only the root-level archive, not nested ones
/archive/
```

## File locations and load order

`pql` looks in this order; rules from each source are concatenated (later sources override earlier on a per-pattern basis via `!`):

1. **Built-in non-overridable defaults.** Internal constants — never indexed regardless of any user file:
   - `.git/`
   - `.pql/` — pql's own per-vault state directory (see [`vault-layout.md`](vault-layout.md))
   - `*.sqlite`, `*.sqlite-wal`, `*.sqlite-shm`, `*.sqlite-journal` — SQLite artefacts in case a user `--db` ever lands in-vault outside `.pql/`

   These exist because indexing them is either useless (binary blobs) or recursive (indexing your own index).

2. **Global `.pqlignore`** at `~/.config/pql/ignore` (mirrors `~/.config/git/ignore`). Optional. For personal cross-vault preferences.

3. **`.gitignore` files in the vault**, but only when `respect_gitignore: true` in `.pql.yaml`. Off by default — explicit opt-in. Loaded with the same per-directory cascade git uses.

4. **Vault-root `.pqlignore`.** The primary file. Always honored when present.

5. **Nested `.pqlignore` files in subdirectories.** Each subtree may have its own; rules apply only within that subtree. Same cascade behaviour as git.

6. **`exclude:` patterns from `.pql.yaml`.** Concatenated as anchored gitignore patterns (see next section).

When evaluating a candidate path, sources are tested in order; the **last matching pattern wins**. So a `!drafts/published/` in the vault root can re-include something that a global `~/.config/pql/ignore` excluded.

## Interaction with `exclude:` in `.pql.yaml`

The `exclude:` field stays. It is **translated to gitignore patterns** at load time and appended after `.pqlignore` rules:

```yaml
# .pql.yaml
exclude:
  - "**/.obsidian/**"
  - "**/node_modules/**"
```

becomes (conceptually):

```gitignore
# from .pql.yaml exclude:
**/.obsidian/**
**/node_modules/**
```

Because the YAML rules come last in the load order, they take precedence over `.pqlignore` rules — **unless** the user added a negation in their vault-root `.pqlignore`, which is itself overridden by anything later. This matches the gitignore mental model: more local + later-loaded wins.

`exclude:` is **not deprecated** in v1. It stays useful for one-off CLI-friendly overrides and machine-generated configs. We may revisit if user feedback says it's confusing to have two ways to express the same thing.

## Honoring `.gitignore`

Off by default. Opt in via:

```yaml
# .pql.yaml
respect_gitignore: true
```

When on, every `.gitignore` in the vault is loaded as if it were a `.pqlignore` (same per-directory cascade, same negation rules). `.git/info/exclude` is **not** honored — it's per-checkout and rarely useful for indexing semantics. Add later if there's demand.

Why off by default: a project may want to publish documentation alongside `.gitignore` rules that exclude build outputs from version control but **should** be indexed (or vice versa). Forcing the equivalence breaks both directions; opting in keeps the choice explicit.

## Negation and the directory short-circuit

gitignore semantics include one well-known gotcha that `.pqlignore` inherits:

> If a parent directory is ignored, files inside it cannot be re-included.

So this **does not** work:

```gitignore
private/
!private/keep.md
```

`private/` is ignored, so `pql` never descends into it, so `keep.md` is never tested against the negation. To make `keep.md` index-visible, the user must either (a) be more specific in the original exclusion (`private/**` instead of `private/`), or (b) use a `!` rule against the directory and then re-exclude individual files inside.

This isn't a `pql` quirk — it's git's behaviour, preserved deliberately for the "identical semantics" promise. `pql doctor --explain-ignored <path>` (planned) will surface the matching rule so users can see why a path was skipped.

## Performance

Per-path matching with nested cascades looks expensive but isn't, in practice:

- The walker pushes a compiled matcher onto a stack when entering a directory containing a `.pqlignore`, pops it on exit. Each candidate path runs through a small stack, not the entire tree's worth of rules.
- Compiled gitignore matchers are O(rules) per match. Typical vaults have <100 rules total across all `.pqlignore` files.
- Excluded directories are pruned from the walk entirely (not "walked then filtered"), so `.obsidian/` with thousands of internal files costs zero scanning time.

For a 10k-file vault with five nested `.pqlignore` files, the ignore subsystem should add <50ms to a full reindex. Profile-confirmed once the indexer lands.

## CLI debugging surface

Two planned subcommands surface ignore behaviour for users:

- `pql doctor` includes a section listing every loaded ignore source (path + line count) and a summary of effective top-level rules.
- `pql files --explain-ignored` (post-v0.1) emits the ignored paths alongside the matching rule and source file: `"members/scratch/draft.md  (matched: \"**/scratch/\" in /vault/.pqlignore:7)"`.

Both write to stderr per the JSON-per-line diagnostic contract in `docs/output-contract.md`.

## Implementation pointers

For the eventual `internal/index/ignore/` package:

- **Library:** [`github.com/sabhiram/go-gitignore`](https://github.com/sabhiram/go-gitignore) is the recommended dependency — pure Go, zero indirect deps, gitignore.5-compatible, simple `CompileIgnoreLines([]string)` / `MatchesPath(string) bool` API. Alternatives (`go-git/...gitignore`, `denormal/go-gitignore`) work but pull more weight.
- **Per-directory cascade:** the walker maintains a stack of matchers. On entering a directory, check for `.pqlignore` and `.gitignore` (if respected) and push compiled matchers; on exit, pop. Match each candidate against the stack from outermost to innermost.
- **Built-in defaults:** apply as a synthetic outermost matcher loaded from an embedded constant, never popped.
- **YAML `exclude:` translation:** convert each pattern to a leading-`/`-anchored gitignore line and append as the innermost matcher (highest priority).
- **Caching:** cache compiled matchers keyed by `(path, mtime)` so re-indexes don't re-parse unchanged files.

## v1 scope

In:
- File-level `.pqlignore` per the syntax above
- Vault-root + nested cascade
- `respect_gitignore: true` opt-in
- Built-in non-overridable defaults
- `exclude:` from `.pql.yaml` translated and appended

Out (defer to later):
- `~/.config/pql/ignore` global file (cheap to add; landing later avoids speculation about precedence ergonomics)
- `pql files --explain-ignored`
- Honoring `.git/info/exclude`
- Honoring `.gitignore_global` (git's own global)

These are all additive — none requires schema or contract changes.

## Naming notes

- File name: `.pqlignore` (mirrors `.gitignore`, `.dockerignore`, `.npmignore`, `.eslintignore` — the convention is well-established).
- Global location: `~/.config/pql/ignore` (mirrors git's `~/.config/git/ignore`).
- The config flag opting into git rules is `respect_gitignore` (snake_case to match other `.pql.yaml` keys).
