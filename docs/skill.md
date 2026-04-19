# Skill: install + usage

The `pql` Claude Code skill lives at `skill/SKILL.md` in this repo. This document covers installation, the schema-version handshake, and what callers can expect.

This catalog is filled in alongside the skill. For now it's a stub.

## Install

Copy the skill directory into one of:
- `~/.claude/skills/pql/` — user-level (any project picks it up)
- `<project>/.claude/skills/pql/` — project-level (only this project)

Skill itself is just `SKILL.md` plus optional `references/`. No build step.

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

Filled in once intents land. Examples expected:
- "query the vault" / "find notes where…"
- "which sessions/members…"
- "run a Base"
- "inspect frontmatter"
- "what's related to <path>"
