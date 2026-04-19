---
name: git-commit
description: >
  Git commit conventions for this repo — message style, attribution trailer,
  and the safety rules that always apply. Use whenever the user says
  "commit", "commit this", "create a commit", "make a commit", "amend",
  or asks Claude to check work into git in any form.
---

# Git commit rule — this repo

Follow these conventions whenever you create a commit in this repository. These are in addition to the standard Claude Code git-safety protocol (no `--no-verify`, no force-push to main, prefer new commits over `--amend`, etc.).

## Message style

- **First line:** imperative mood, ≤ 70 characters. Examples: `add base parser`, `fix vault discovery on bare git repos`, `update PQL grammar for IN operator`.
- **Body (optional):** wrap at ~72 chars. Explain the *why* — the reason this change exists. The diff already shows the *what*; don't restate it in prose.
- **No emojis.** Anywhere.
- **Don't prefix with types** like `feat:` or `fix:` — this repo isn't Conventional Commits.
- **Don't reference the current task or flow** (`for the v0.2 milestone`, `used by the evaluator`) — that context belongs in the PR description and rots as the repo evolves.
- **Naming:** the project is `pql` / PQL (Project Query Language). Never write `mql` / MQL in commit messages — that's the obsolete pre-rename name.

## Logically-separated commits

Default to **one logical change per commit**, even when a lot of work lands in the tree at once. When there's a pile of uncommitted or untracked files:

1. **Read `git status` + `git diff` first** — never stage the whole tree blind.
2. **Group by concern**, not by file location. Typical concerns to separate:
   - **Bookkeeping** — `.gitignore`, editor/IDE config, lockfiles, `go.sum` updates.
   - **Documentation** — `README.md`, `CLAUDE.md`, design notes under `docs/`.
   - **General-purpose tooling / skills** — things that aren't project-specific (reusable skills, shared scripts).
   - **Project-specific conventions** — this repo's own rules.
   - **Feature or subsystem** — one cohesive change per commit; a parser change and an evaluator change for the same feature can land together, but two unrelated features should split.
   - **Layer changes** — index/store/lex/parse/eval/render are separate concerns; prefer separate commits when the changes are independent.
3. **Sequence the commits** so each one is cleanly scoped, but don't obsess about whether each intermediate commit "works" — for scaffolding PRs it's fine if the full picture only snaps together at the end.
4. **Prefer many small focused commits over one large mixed one** — a reviewer can read, revert, or cherry-pick a focused commit; they can't do any of those to a blob.
5. **Use `git add <specific paths>`** — never `git add -A` or `git add .` when splitting, or you'll sweep in the next commit's work by accident.
6. **Verify between commits** with `git status` and `git log -1` to confirm the split landed as intended.

Corollary: if a commit's subject line needs the word "and" to be accurate, it probably should have been two commits.

## Attribution trailer

Every commit ends with the Claude co-author line:

```
Co-Authored-By: Claude <noreply@anthropic.com>
```

The model-identifier variant produced by the Claude Code harness (e.g. `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`) is also accepted — don't rewrite it if the harness emits that form.

## HEREDOC discipline

Pass commit messages via HEREDOC so multi-line formatting survives:

```bash
git commit -m "$(cat <<'EOF'
short imperative summary

Optional longer body explaining why this change was needed,
wrapped at about 72 characters.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

Never pass multi-line messages via `-m "line1\nline2"` or multiple `-m` flags — Git's behavior differs between shells and quoting regimes and the trailer can end up in the wrong place.

## What not to commit

- `.env` and any `*.env.local` — see `.gitignore`.
- `.claude/settings.local.json` — user-specific Claude Code settings, ignored.
- Build artefacts: `bin/`, `dist/`, `pql`, `pql.exe` — gitignored.
- SQLite index files (`*.sqlite`, `*.sqlite-wal`, `*.sqlite-shm`, `*.db`) — these are caches generated against a local vault and must never land in the repo. Gitignored as a defensive measure.
- Coverage / test output (`*.out`, `coverage.*`, `*.test`) — gitignored.

## Safety reminders (reinforced from the global Claude Code protocol)

- **Never** `--no-verify`. If a pre-commit hook fails, fix the underlying issue and create a new commit.
- **Never** `--amend` a commit unless the user explicitly asks. A failed-hook commit didn't land, so amending would overwrite the *previous* commit and lose work.
- **Never** force-push to `main` or `master`. Warn the user if they ask.
- When staging, prefer naming specific files over `git add -A` / `git add .` — those can sweep in secrets or unintended files.
- `git status` before staging, `git diff --staged` before committing, `git log -1` after committing to confirm.
