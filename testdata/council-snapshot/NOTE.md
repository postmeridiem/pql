# council-snapshot — fixture provenance

This is a frozen snapshot of the Council vault from `/var/mnt/data/projects/council/`. It is the integration-test fixture for `pql`: every regression that breaks Council queries fails CI.

## What's included

- `members/` — 12 council-member personas with frontmatter (`type: council-member`, `name`, `prior_job`, `lens`, `voting`, `model`), plus per-member `journal.md`, `on-the-user.md`, `revisit.md`.
- `sessions/` — at least one full session with brief, research, initial/revised answers, votes, outcome.
- `council-members.base`, `council-sessions.base` — Obsidian Bases compiled by `pql base`.
- `README.md` — context for what the Council project is.

## What's deliberately **not** included

- `.obsidian/` — app state, irrelevant to indexing.
- `.claude/` — Council's own Claude Code tooling, unrelated to pql.
- `.git/` — would bloat the repo.
- `.env` — secrets.
- `daily/`, `plugins/`, `docs/` — internal scratch / not query-relevant.

## Refresh

When the Council vault evolves and the fixture goes stale (frontmatter schema change, new `type:` value, new Base, etc.), refresh with:

```sh
make refresh-fixtures
```

This re-copies the included paths from `/var/mnt/data/projects/council/`. The Make target only runs when explicitly invoked — it never runs as part of `make test` or CI. Snapshot drift is intentional: tests pin against a known good vault state, not against a moving target.

## Hand-editing

**Don't.** If a test needs a vault shape the snapshot doesn't have, add a synthetic vault under `testdata/<name>/` instead. The snapshot's value is its realism; hand-edits erode that.
