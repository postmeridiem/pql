---
name: pql-testing
description: >
  Test the quality of the embedded pql skill (internal/skill/SKILL.md) the way
  a consuming agent experiences it — as documentation it must act on without
  reading pql's source. Use before shipping a release that changes the skill,
  after adding or changing a CLI surface, or whenever someone asks whether the
  skill is accurate, complete, or usable. Produces a findings report, not a
  pass/fail.
---

# Testing the pql skill

The embedded skill is a product surface. It is `//go:embed`'d into the binary,
installed into consumers by `pql init`, and sits permanently in the context of
every agent that uses pql. Nothing verifies it: the tests assert only that the
file exists and has frontmatter, so a skill can describe commands that do not
exist, omit commands that do, and still ship green.

This procedure exists because that has happened. Three ranked commands
(`search`, `related`, `context`) were absent from the skill for months while
its anti-patterns section actively directed agents to `grep` instead. A
`decisions coverage` command was documented after being removed. Both were
found by reading, not by testing.

## What "quality" means here

Judge the skill against four questions, in this order. The first two are
correctness and are objective; the last two are usability and take judgement.

1. **Is it true?** Does every documented command, flag and behaviour exist and
   behave as described?
2. **Is it complete?** Does every command a caller would want appear?
3. **Can it be acted on?** Given only the skill, can an agent do real work
   without guessing, re-reading, or falling back to source?
4. **Is it worth its context cost?** It is loaded permanently. Redundancy,
   duplication and inventory-for-its-own-sake are defects.

## Procedure

Work outside-in. Do not read pql's source until step 4 — the point is to
experience the skill as a consumer, and knowing the implementation makes gaps
invisible.

### Step 1 — Read only the skill

```bash
pql skill show --pretty
```

That echoes what the *running binary* ships, which is the thing under test.
Do not read `internal/skill/SKILL.md` from the repo — they can differ, and
that difference is itself a finding.

Note as you read: what you expect to be able to do, and anything you cannot
tell from the text alone.

### Step 2 — Verify every claim

For each documented command, check it exists and matches its description:

```bash
pql <command> --help
```

Then diff the other direction — the surface against the skill:

```bash
pql --help
pql ticket --help
pql decisions --help
pql plan --help
```

Every command in the CLI that is absent from the skill is a finding. Every
command in the skill that is absent from the CLI is a worse one. Check flags
too, not just command names: a documented flag that does not exist wastes a
round trip and teaches the agent to distrust the document.

### Step 3 — Run real work against a real vault

Verification catches lies; only use catches unusability. Use a vault with
substance — this repo, or another consumer — and attempt tasks a caller would
actually bring, using **only** what the skill told you.

Cover both surfaces and escalate in complexity. Suggested shape:

- **Simple retrieval** — list files under a folder; top tags; what links to a
  given file.
- **A structural question needing the DSL** — everything with a given
  frontmatter type, sorted by a frontmatter date. Did the skill teach enough
  syntax to write this without trial and error?
- **A ranked question** — what is related to this file; what is this file
  about. Does the result match what the skill led you to expect?
- **A multi-step planning task** — find open work in a given state, read one
  ticket's full context, follow its links to a decision record, and report
  what implements that decision.
- **A task with a known trap** — something the skill warns about, to check the
  warning is both findable and correct.

Record for each: what you ran, whether it worked first time, and if not, what
the skill should have said.

### Step 4 — Only now, read the source

With findings in hand, check `internal/skill/SKILL.md` and the CLI source to
confirm each finding is real and to locate the fix. This is also where you
catch skill-vs-binary drift from step 1.

### Step 5 — Report

Group findings by severity:

- **Wrong** — the skill says something untrue. Highest severity: it causes
  confidently incorrect action.
- **Missing** — a capability exists and is undocumented. An agent cannot use
  what it does not know about.
- **Unusable** — documented but insufficient to act on.
- **Bloat** — correct but not earning its context.

For each, give the evidence (the command run, the output seen) and a concrete
suggested edit. Do not fix the skill as part of this procedure unless asked —
the report is the deliverable, so the maintainer decides what changes.

## Rules

- **Never fix and test in the same pass.** Reading the source to fix something
  contaminates the outside-in perspective for everything after it.
- **Prefer evidence to opinion.** "The skill does not mention `pql search`" is
  a finding; "the tone could be friendlier" is not.
- **Test the installed skill, not the repo file**, and treat a difference
  between them as a finding in its own right.
- **A zero-finding report is a valid outcome** — but it should say what was
  exercised, or it is indistinguishable from not having looked.
