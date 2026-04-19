# Council

A Claude Code deliberation system with a persistent memory layer, browsable as an Obsidian vault.

You pose a question; a council of ten distinct personas (plus a Researcher and a Scribe) deliberates through a structured five-phase workflow, produces a voting table, and hands you the winning answer. Every session leaves a full transcript and persistent memory behind.

## Quick start

Inside the repo in Claude Code:

```
/council <your question>
```

The Moderator (the main Claude Code session) will interview you briefly for context, then kick off the Council. You'll be asked follow-up questions along the way — to clarify the brief, resolve research gaps, or break vote ties. At the end you get a voting table and the full winning answer; everything is archived to `sessions/<slug>/`.

## The Council

10 voting members, wild eclectic mix:

- **Elif Tavşan** — retired stage magician & cold-reading coach
- **Dr. Wren Okafor** — deep-sea marine biologist → systems ecologist
- **Magnus Holt** — bankruptcy lawyer → insolvency-law lecturer
- **Sister Beatrix Vale** — Benedictine nun → hospice counselor
- **Nikolai "Niko" Prochazka** — Soviet cosmonaut trainer → wilderness survival instructor
- **Dr. Ingrid Vaasa** — theoretical physicist (QFT) → independent researcher
- **Kai "Breaker" Lindholm** — pro skateboarder → skatepark architect
- **Cassian Vire** — investigative journalist → speculative-fiction novelist
- **Marcelo "Marco" Tintori** — trained painter → 20 years as a corrections officer
- **Mari Koskela** — marine engineer on cargo ships → anthropologist of maritime labor

Plus two non-voting roles:

- **Naima Quéré** (Researcher) — the only sub-agent with web access.
- **Ansel Voss** (Scribe) — transcribes proceedings and writes session files.

See `council-members.base` for the full roster as a table view in Obsidian.

## Browsing sessions in Obsidian

Open this repo as an Obsidian vault. The two Bases at the root provide table views:

- **`council-sessions.base`** — every session, sortable by date / winner / whether a tie happened.
- **`council-members.base`** — the full roster with prior jobs and lenses.

Individual session files live at `sessions/<slug>/`:
- `brief.md` — the Moderator's brief from the Phase-0 interview
- `research/*.md` — the Researcher's per-topic notes
- `initial-answers.md`, `revised-answers.md`, `votes.md` — phase transcripts
- `outcome.md` — the canonical session file (winner, tally, full winning answer)

## Deeper docs

- `CLAUDE.md` — guidance for Claude Code when working inside this repo
- `docs/structure/initial-plan.md` — the full design document
