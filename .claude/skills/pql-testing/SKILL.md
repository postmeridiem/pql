---
name: pql-testing
description: >
  Audit the embedded pql skill (internal/skill/SKILL.md) for accuracy,
  completeness and usability, by acting on it as a consuming agent would —
  without reading pql's source. Use before shipping a release that changes the
  skill, after adding or changing any CLI command or flag, when the skill and
  the CLI may have drifted, or when asked whether the skill is correct or
  usable. Produces a findings report; does not fix anything.
---

# Audit the pql skill

The skill is embedded in the binary and sits permanently in the context of
every agent using pql. Nothing verifies it — the tests assert only that the
file exists and has frontmatter, so it can document removed commands and omit
shipped ones while passing.

## Setup

Build a disposable vault. Never test against a repo anyone works in.

```bash
make scratch-vault
```

That gives `/tmp/pql-scratch-vault` with both surfaces: markdown and `.base`
files for the vault surface, real tickets and decisions for the planning
surface. Every verb is safe there — it is a copy. Re-run to reset.

Target it explicitly on every call:

```bash
pql --vault /tmp/pql-scratch-vault <command>
```

Rebuild before auditing, or you test a stale artefact:

```bash
make build && cp ./bin/pql ~/.local/bin/pql
```

## Procedure

Run the steps in order. Do not read pql's source before step 4 — knowing the
implementation makes omissions invisible.

### 1. Read the shipped skill

```bash
pql skill show | python3 -c 'import json,sys; print(json.load(sys.stdin)["files"]["SKILL.md"])'
```

The raw output is one JSON-escaped string and is not readable as-is; that
extraction is the only way to read it as prose.

Read what the binary ships, not the repo file. Note what you expect to be able
to do, and anything the text leaves ambiguous.

### 2. Verify both directions

Skill → CLI: every documented command, flag and flag *value* must exist and
behave as described.

CLI → skill: every real command must appear.

```bash
pql --help
pql ticket --help
pql decisions --help
pql plan --help
```

A documented thing that does not exist is the worst defect. A shipped thing
that is undocumented is the second worst.

### 3. Do real work using only the skill

Attempt tasks a caller would actually bring, using only what the skill told
you. Record for each: did it work first time, and if not, what was missing.

Cover, at minimum:

- A structural query needing the DSL, written from the skill's syntax notes
  alone.
- A ranked query, judged against what the skill led you to expect.
- A multi-step planning task: find work in a given state, read one item's full
  context, follow it to a decision record.
- **A set-level cross-surface question** — for example, which decisions have no
  tickets implementing them, or the largest cluster of open work under one
  parent. These expose the most defects, because they need flags and fields the
  skill tends not to mention.
- A write path, since half the surface mutates state. Safe in the scratch vault.
- Something the skill explicitly warns about, to check the warning is findable
  and correct.

### 4. Read the source

Confirm each finding and locate its fix. Also diff the shipped skill against
`internal/skill/SKILL.md` — see shape G below.

### 5. Report

Group by severity: **Wrong** (says something untrue) → **Missing** (exists,
undocumented) → **Unusable** (documented, insufficient to act on) → **Bloat**
(correct, not worth its context). Give the command run and the output seen for
each, plus a concrete suggested edit. Rank by severity.

State what you exercised and found clean. A report that only lists defects is
indistinguishable from one that stopped looking.

## Shapes to probe for

Test for these deliberately. Each is a class, not an anecdote — expect
instances the examples below do not name.

**A. Reads that write.** Any command touching an index, cache or database
mutates its target, however read-only it looks. Assume every verb writes until
proven otherwise; isolate with `--vault` and `--db`, or work on a copy.

**B. Invalid input that returns empty instead of failing.** Probe every
enum-valued flag with a wrong value. If an invalid filter returns an empty list
at exit 0, it is indistinguishable from a genuine no-match, and any caller told
"zero matches is success" will report absence that is not there. Also check
whether the valid set is documented, or only discoverable by triggering an
error.

**C. Degraded modes that look like answers.** A flag that falls back to a
broader or emptier result set returns something plausible rather than failing.
Compare its output against the unflagged call and against the naive baseline —
if a "raw" mode returns the same count as listing everything, it is not
answering the question.

**D. Aborts that poison every command.** One malformed input can fail an entire
index, making every unrelated command fail too. Feed a bad file and check
whether the skill names a recovery path.

**E. Claims derived from one observation.** A single probe yields a mechanism
that looks right and generalises wrongly. For any claim about *why* something
behaves as it does — ranking, weighting, matching — read the source rather than
inferring from one output. Prose that is internally consistent can still be
wrong, and verification will not catch it; only use will.

**F. Guidance that inverts the tool's real shape.** Check the skill is not
steering callers away from a capability that exists, or toward one that does
not do what its name suggests.

**G. Self-reports blind to their own drift.** A status or health command that
compares an artefact against a copy embedded in the same binary cannot see that
the source changed. Never accept such a report as proof. Verify across the
boundary the tool cannot see — here, diff the shipped skill against the repo
file directly.

**H. Omissions that block whole task classes.** After step 3, ask which flags
or fields you needed that the skill never named. Those omissions cost more than
any inaccuracy, because they make a task look impossible.

## Rules

- Do not fix anything. Reading source to fix contaminates the outside-in view
  for every finding after it. The report is the deliverable.
- Do not test against a live repo. If one is genuinely unavoidable, get the
  owner's agreement and stay read-only — but see shape A first, because
  "read-only" verbs usually are not.
- Evidence, not opinion. "The skill does not mention `--vault`" is a finding;
  "the tone could be warmer" is not.
- Report defects in this procedure alongside defects in the skill.
