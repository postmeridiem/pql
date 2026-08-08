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
| Core retrievals run | <n> / 12 |
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
  visible as a low number rather than hiding behind a short findings list.
- Get skill size from the shipped copy, not the repo file.
