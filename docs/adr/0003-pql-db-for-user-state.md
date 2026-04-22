# ADR 0003: Two SQLite stores — `index.db` (cache) and `pql.db` (state)

**Status:** accepted
**Date:** 2026-04-22

## Context

`pql` started as a read-only query engine over a markdown vault. The binary owns one SQLite file at `<vault>/.pql/index.db`, and a load-bearing invariant is that anything in it must be regenerable from the vault (drop-and-rebuild on schema-version mismatch; see `internal/store/` and `initial-plan.md`).

Two subsequent features broke that "one file, pure cache" framing:

1. The Claude Code skill install lock (`.pql-install.json`) — state the binary authored, not derivable from the vault.
2. The planning feature (decisions + tickets; `docs/structure/planning.md`) — ticket status, history, assignments, and the decision→ticket join graph. None of this is regenerable from markdown.

The skill lock sits outside SQLite so the invariant survived. Planning cannot. Status transitions, ticket history, and the D-record ↔ ticket join graph are user-authored state that must survive index rebuilds.

`TODO.md` already reserved the slot `<vault>/.pql/pql.db` for "a non-cache store, purpose not yet pinned down." Planning is the purpose.

## Decision

Two SQLite files under `<vault>/.pql/`:

- **`index.db`** — the regenerable cache. Vault walker writes; query primitives and the DSL read. Drop-and-rebuild on `schema_version` mismatch; no migrations.
- **`pql.db`** — user-authored state. Planning subcommands write (`pql decisions sync`, `pql ticket *`, etc.); skill install state may migrate in here later. Versioned forward with real migrations, because losing this data is a bug.

Both files live in `<vault>/.pql/` alongside `config.yaml`. The read-only-vault user-cache fallback (already defined for `index.db` in `docs/vault-layout.md`) applies to `pql.db` too — but only for the cache surface; a read-only vault where the user runs `pql ticket new` fails cleanly with a usage error, since writes to `pql.db` on a cache fallback would be invisible to the vault itself.

## Why

- **One store fails both tests.** As a cache, it's too durable; as user state, it's too fragile. Splitting the names clarifies the contract at the file-system level.
- **The CLI-today / MCPs-tomorrow split reinforces this.** A future query-surface MCP only needs `index.db`; a planning-surface MCP only needs `pql.db`. Different permission scopes, different audiences, different on-disk footprints.
- **`TODO.md` already reserved the slot.** The name `pql.db` was held back from being grabbed incidentally (the `.sqlite` → `.db` rename in 0.1.1-dev cleared the extension collision). Using it here costs nothing new.
- **Alternatives considered.** `.clide/clide.db` (couples pql's store to one consumer's dotfile); `.pql/planning.db` (invents a third name where a reserved one fits and locks us into per-feature filenames as state grows). Both rejected.

## Consequences

- The "index is a pure cache" invariant survives verbatim — it refers to `index.db` only. `CLAUDE.md` and `initial-plan.md` are updated to name the file explicitly so the rule doesn't read as applying to the whole `.pql/` dir.
- `pql.db` gets real schema migrations from day one of the planning feature. Migration runner lives in `internal/planning/schema.go` (scoped to this DB), not in `internal/store/`.
- `pql init` creates neither file up front; each is lazily created by the first writer. `pql doctor` reports both.
- Read-only-vault behavior: `index.db` falls back to user cache (unchanged); `pql.db` writes fail with a clear diagnostic and non-zero exit. No silent write to a cache dir the user would never find.
- `pql self-update` / release upgrades must not drop `pql.db`. Uninstalling pql leaves it in place; this is user data.

## When this might change

- If `pql.db` grows past the "planning + skill state" scope into something that genuinely wants a different engine (e.g. shared-team use over a network FS, concurrent writers), revisit. SQLite-single-writer is fine for single-user-per-vault, which is the current scope.
- If the skill install lock is promoted from `.pql-install.json` into `pql.db`, update `internal/skill/` and note it here. Not blocking for the planning feature.
