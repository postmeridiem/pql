# Skill: install + usage

The `pql` Claude Code skill lives at `internal/skill/SKILL.md` and is embedded into the binary at build time via `go:embed`. `pql skill install` writes it to the consuming project's `.claude/skills/pql/` directory (or `~/.claude/skills/pql/` with `--user`). This document covers installation, the schema-version handshake, and what callers can expect.

## Commands

`pql skill` manages the embedded skill(s) (`pql` and the bundled `clean-house`):

| Command | Does |
|---|---|
| `pql skill install [--user] [--force]` | Write each bundled skill's files + a `.pql-install.json` lock (version + bundle hash). Idempotent. Without `--user`, scope auto-resolves: user-scope (`~/.claude/skills/`) if any bundled skill already lives there, else project-scope (`<vault>/.claude/skills/`). `--force` overwrites a hand-edited install. |
| `pql skill status [--user]` | Report install state per skill: `missing` / `current` / `stale` (older binary, pristine) / `modified` (hand-edited) / `unknown` (no lock). |
| `pql skill show [name]` | Echo a bundled skill's content embedded in *this* binary (defaults to `pql`); JSON keyed by file path, `--pretty` to read. Works from anywhere, no vault. |
| `pql skill uninstall [--user]` | Remove the bundled skills from the target scope (project-scope only unless `--user`). |

`pql doctor` also surfaces skill install state. The skill is just `SKILL.md` plus optional `references/` — no build step.

## Permissions

In the consuming project's `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": ["Bash(pql)", "Bash(pql *)"]
  }
}
```

Two entries because the wildcard form requires at least one argument after `pql`; the bare form covers `pql --help` / `pql doctor` / etc.

No deny rules — `pql` is read-only against the filesystem.

## Schema-version handshake

The skill should run `pql version --build-info` once on first invocation, parse `schema_version`, and abort if it's older than the skill's declared minimum. See `compatibility.md`.

## Trigger phrases

The authoritative trigger list lives in the embedded `SKILL.md` `description` (run `pql skill show --pretty` to read it). It covers both surfaces — structural queries ("which notes…", "find where…", "what tags", "who links to X", "run a Base") and planning ("decisions", "tickets", "plan status", "show D-5", "board", "refine tickets").
