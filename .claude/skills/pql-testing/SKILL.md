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

### 3. Retrieve real data using only the skill

Run retrievals a caller would actually need, using only what the skill told
you. **You are testing the tool and the skill, not your own reasoning.** For
each retrieval, judge exactly three things:

1. **Routing** — did the skill get you to the right command and flags without
   guessing, retrying, or reading source?
2. **Workability** — is what came back something you could act on? Right
   fields present, shape parseable, volume manageable, one call rather than N.
3. **Unstated post-processing** — what did you have to do to the output that
   the skill never mentioned?

Do **not** evaluate whether your interpretation of the data was correct.
Whether the vault really has twelve open questions is not under test. Whether
pql returned them in a usable form, and whether the skill led you there, is.
If a retrieval requires you to fold, join or count client-side, that is a
finding about pql's surface — record the post-processing, not the answer.

#### Core set — run all of these, every time

Stated as goals, never as commands: working out which command serves the goal
is the thing being tested. Run them in order and give each its own friction-log
row, keeping the R-numbers so runs can be compared.

| # | Goal, in caller's terms |
|---|---|
| R1 | List every ticket in one given status |
| R2 | List tickets matching a status that does not exist |
| R3 | Get one ticket with everything needed to start work on it |
| R4 | List every ticket under one parent, however deep |
| R5 | Get an index of all tickets small enough to read in one go |
| R6 | For one decision, find what implements it |
| R7 | Across all decisions, find which have nothing implementing them |
| R8 | List every note carrying a given frontmatter value |
| R9 | Find the notes most related to one note |
| R10 | Find notes about a topic given two words to describe it |
| R11 | Create a ticket, then read it back to confirm it landed |
| R12 | Follow one thing the skill explicitly warns about |

R2, R7 and R10 are the load-bearing ones: they probe silent-empty filtering,
cross-surface joining, and the substring-match limit respectively. Do not skip
them because they look likely to fail — that they fail cleanly, loudly and with
guidance is exactly what is being checked.

#### Then explore

Beyond the core set, follow whatever the vault and the skill suggest. New
retrievals find defects a fixed list cannot. Number these E1, E2 … and log them
the same way.

### 4. Read the source

Confirm each finding and locate its fix. Also diff the shipped skill against
`internal/skill/SKILL.md` — see shape G below.

### 5. Report

Now — and not before — read `references/report-template.md` and fill it in.

It is a separate file on purpose. Knowing the report's shape while testing
bends the testing toward filling in its sections, the same way reading source
before step 4 hides omissions.

Keep a running friction note from step 3 onward: goal, command, whether the
skill routed you there, whether the output was workable, what post-processing
you needed. Reconstructing that at the end produces a tidy account of a process
that was not tidy, and the friction is the measurement.

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

**I. Output that is correct but not workable.** The command succeeds and the
data is right, yet the caller cannot use it: a field needed for the obvious
next step is absent from the default projection, the payload is large enough to
be truncated before it arrives, the shape forces a client-side join, or the
answer takes one call per record instead of one call. Judge every retrieval on
whether it hands back something actionable, not merely something true.

## Rules

- Do not fix anything. Reading source to fix contaminates the outside-in view
  for every finding after it. The report is the deliverable.
- Do not test against a live repo. If one is genuinely unavoidable, get the
  owner's agreement and stay read-only — but see shape A first, because
  "read-only" verbs usually are not.
- Evidence, not opinion. "The skill does not mention `--vault`" is a finding;
  "the tone could be warmer" is not.
- Report defects in this procedure alongside defects in the skill.
