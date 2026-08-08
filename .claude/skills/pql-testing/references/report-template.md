# Report template

Read this only when wrapping up. Reading it earlier shapes the testing toward
filling in the sections, which is the opposite of what the audit is for.

Fill in every section. Write "none" rather than dropping one, so a reader can
tell an empty section from an unexamined one.

---

```markdown
# pql skill audit — <binary version> (<commit>)

Vault: <path>   Surfaces exercised: <vault | planning | both>

## Statistics

| Measure | Value |
|---|---|
| Core retrievals run | <n> / 15 |
| Skill warnings checked | <n> / <n the skill gives> |
| Exploratory retrievals | <n> |
| Routed from the skill alone | <n> / <total> |
| Workable as returned | <n> / <total> |
| Needed unstated post-processing | <n> |
| Calls made vs. minimum needed | <made> vs <minimum> |
| CLI commands verified | <n> / <n in `pql --help`> |
| Findings: Wrong / Missing / Unusable / Bloat | <w> / <m> / <u> / <b> |
| Skill size | <lines> lines, ~<words> words |

## Verdict

<Two or three sentences. Is the skill fit to ship as it stands? If not, name
the smallest set of fixes that would make it so.>

## Conclusions

<3–6 bullets. What this audit says about the state of the skill — patterns, not
individual defects. "Filter vocabularies are absent across every list verb" is
a conclusion; "--type Q returns empty" is a finding.>

## Friction log

One row per retrieval, including the ones that went fine. Keep the R-numbers
from the core set so runs can be compared; number exploratory ones E1, E2 …

| # | Goal | Command(s) run | Routed from skill alone? | Workable as returned? | Post-processing needed |
|---|---|---|---|---|---|
| R1 | <goal> | <what you ran> | yes / no — what was missing | yes / no — why | none / <what you had to do> |

## Findings

Ranked by severity, worst first. For each: what the skill says, the command
run, the output seen, and a concrete suggested edit.

### Wrong
### Missing
### Unusable
### Bloat

## Exercised and clean

<What you actively tested that held up. Be specific — this separates "tested
and sound" from "not looked at".>

## Procedure defects

<Anything wrong with this audit procedure, the agent definition, or this
template, found while following them. Write "none" if none.>
```

---

## Filling the statistics

- **Routed from the skill alone** counts retrievals where you reached the right
  command and flags without guessing, retrying or reading source.
- **Calls made vs. minimum needed** is the friction number that matters most.
  If answering one question took 28 calls because the flag for a single-call
  answer is undocumented, that gap is the finding.
- **CLI commands verified** measures step 2's coverage, so a thin audit is
  visible as a low number rather than hiding behind a short findings list. Count
  against the *auditable* surface, not everything `pql --help` prints: exclude
  `completion` and `help` (cobra boilerplate), `shell` (interactive, cannot be
  driven non-interactively), and `self-update` (the skill forbids it). State the
  exclusions so the denominator is comparable between runs — otherwise a careful
  auditor scores worse than one who counted boilerplate.
- **Skill size** comes from `pql skill status`, which reports `lines` and
  `words` on the `embedded` snapshot — the copy this binary ships, which is what
  the number should describe. Do not measure the repo file (it can differ from
  the binary) and do not pipe `skill show --raw` into `wc` (piping is what the
  procedure and the skill both tell callers to avoid).

## Aborting

A run that could not execute is a legitimate outcome and has its own shape.
Fill the template as normal, with the statistics zeroed, the denial or failure
log complete, and the verdict stated as **no verdict possible** plus the
smallest thing that would make one possible.

Do not substitute a source-versus-skill code review for the behavioural audit.
It is not the job, and a review written from the implementation reads as
authoritative while resting on zero observations — which is worse than an
honest abort. Unverified leads from a static read are welcome, but label them
as leads and keep them out of the findings count.
