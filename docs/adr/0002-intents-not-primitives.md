# ADR 0002: Intent-level surface for agents, primitive surface as escape hatch

**Status:** accepted
**Date:** 2026-04-19

## Context

`pql` is consumed primarily by AI agents (Claude Code via the skill) and secondarily by humans on the command line. The natural design instinct is to expose many small primitive tools that the agent chains together. The design philosophy argues against this.

## Decision

Expose intent-level commands (`pql related`, `pql search`, `pql context`, `pql base`, …) as the primary surface. Each intent does its own internal candidate generation, ranking, and bundle assembly — agents call **one** tool per request and get back a pre-ranked, pre-explained context bundle.

The PQL DSL and primitive query subcommands (`pql files`, `pql tags`, `pql backlinks`, …) remain available as the **escape hatch** layer — flat results, no enrichment, no provenance. Documented in `docs/intents.md` as the "give me only what I asked for" path.

A global `--flat-search` flag reduces any intent command to its primitive layer for one invocation.

## Why

Distilled from `structure/design-philosophy.md`:

- Each tool call is a permission event. Long chains erode user trust.
- Each tool call is a round trip. The binary's internal queries are orders of magnitude cheaper than the model's.
- Each tool call is an opportunity for the model to drift. Composition inside the binary is deterministic; composition by the model is not.

The user's clarification: *"feel simple as a query engine, but dynamically offer connections if they are available."* This decision encodes that — primitives feel like a query engine, intents add connections, `--flat-search` strips them when the caller wants exact-and-only.

## Consequences

- Adding a new agent capability typically means adding a new intent, not adding a new primitive.
- The intent layer must not import the CLI layer (`internal/intent/` consumer-agnostic). This makes a future MCP-server consumer cheap.
- Documentation discipline: every intent declares its weight profile and bundle shape (`docs/intents.md`). Without this, intents become opaque "magic" tools.
- If the agent reaches for the escape hatch frequently, the intent surface is wrong and should be revisited — not supplemented with more primitives.

## When this might change

- A class of agent task emerges that genuinely needs primitive composition (e.g. cross-intent joins). At that point we extend the intent surface, not abandon it.
- A consumer materializes that wants raw rows (analytics, dashboards). They use `--flat-search` or the DSL directly.
