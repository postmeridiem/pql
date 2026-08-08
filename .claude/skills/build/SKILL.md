---
name: build
description: >-
  Build, install, and ship the pql binary after code changes. Use whenever
  asked to build, rebuild, install, or ship pql, or after a code change that
  should land in the PATH binary at ~/.local/bin/pql. The load-bearing step is
  the pre-build sync: the embedded skill doc and CHANGELOG must be brought into
  line with any user-facing surface change BEFORE building, because
  internal/skill/SKILL.md is //go:embed'd into the binary and nothing verifies
  its accuracy. Encodes that judgment step plus the version-bump → make build →
  install ritual, including when NOT to install.
---

# Build & ship pql

`make build` is deterministic, but deciding what the build should *carry* is
not. `internal/skill/SKILL.md` is embedded into the binary via `//go:embed`, so
the build always bakes in whatever the file says — and **no test or hook checks
that what it says is true.** A new flag or changed behavior silently ships with
a stale doc unless someone updates it first. That judgment is why this is a
skill and not a Makefile prerequisite.

Run the steps in order. Step 0 is the one that needs intelligence; the rest is
a fixed sequence.

## Step 0 — Sync docs to the surface (judgment)

Decide: **did this change alter the user-facing surface?** That means any of —
a new or changed subcommand or flag, a different output shape, exit codes, or
observable behavior (e.g. `whatsnext` now skipping blocked tickets).

- **Yes →** update both, to match reality, before building:
  - `internal/skill/SKILL.md` — the subcommand table in the relevant section
    (Query / Planning / skill management) **and**, when it's a workflow worth
    showing, a one-line recipe in the matching cookbook.
  - `CHANGELOG.md` — a line under the current working version section
    (`Added` / `Changed` / etc.), per the git-commit skill's Keep-a-Changelog
    rule.
- **No** (internal refactor, test-only, internal-doc edit) → skip the doc
  update. Don't manufacture changelog noise.

**Also — does the change deviate from a decision record?** This is a separate
trigger from user-facing-ness: a change can contradict a D-record without
touching the CLI surface (a retired option, a changed default, an error
downgraded to a warning, a different mechanism than the one the record
describes). If so, reconcile it in the **same commit** — amend the record with
an inline `**Amendment (YYYY-MM-DD):**` marker if the implementation is the
right call, or change the code if the decision is. Never ship a D-record that
contradicts the code; a stale record is worse than none. See the
`d-record-implementation-sync` memory.

Sanity check after editing: the documented flags/commands should be ones that
actually exist in `internal/cli/`. If you added a command, confirm it appears in
`pql <cmd> --help` (cobra generates help from the command's own `Short`/`Long`,
so help is never the stale surface — the cookbook is).

Then confirm what ships is what you wrote — the skill is `//go:embed`'d, so an
edit is invisible until the next build, and `pql skill status` reports "current"
regardless because it compares against the binary's own embed:

```bash
make skill-drift
```

**Release gate.** If this is a release *and* `internal/skill/SKILL.md` changed
since the last one, audit the skill before dating the CHANGELOG section — dating
it is what triggers the publish, and a shipped skill is frozen until the next
release. Use the `pql-skill-auditor` agent (procedure in
`.claude/skills/pql-testing/`), then fix what it finds and rebuild. It costs
roughly 100k tokens and half an hour, so it is a release-time step, not a
per-edit one; `make skill-drift` covers the everyday case.

## Step 1 — Version bump

Bump the **fix** version in `project.yaml` (e.g. `1.6.0` → `1.6.1`) unless the
user asked for a minor/major bump, and update the `status:` line to describe the
change. Skip the bump only on a no-op rebuild (no code/doc change since the last
build). See the `install-after-build` memory for the rationale.

## Step 2 — Gate

```
go test ./... && make lint
```

Cheap, and it catches a broken embed or test before you spend a build on it. Fix
failures and re-run; never build over red.

## Step 3 — Build

```
make build
```

Stamps the `project.yaml` version via ldflags and embeds the current SKILL.md.
Confirm the stamped version is clean (just the number — see `version-clean`).

## Step 4 — Install

```
cp ./bin/pql ~/.local/bin/pql && pql version
```

(`make install` does the same via `install -m 0755`.) The PATH binary is what
the user and scripts invoke; building to `./bin/pql` without copying leaves a
stale binary on PATH.

If Step 0 touched SKILL.md, verify the embed actually carried through:

```
pql skill show | python3 -c "import sys,json;print('updated text' in json.load(sys.stdin)['files']['SKILL.md'])"
```

## When NOT to install

**Do not install mid-flight during a multi-ticket sequence that changes the
`pql.db` schema or the canonical-hash projection.** Other users on the system
pick up the PATH binary and break against intermediate, not-yet-coherent states.
Hold installs until the sequence reaches a stable boundary the user signals as
releasable. Building to `./bin/pql` for local testing is fine; the *copy to
`~/.local/bin`* is what's gated.

## Committing

SKILL.md, CHANGELOG.md, and project.yaml land in the **same commit** as the
code they describe — follow the `git-commit` skill for message style, the
co-author trailer, and splitting unrelated concerns.
