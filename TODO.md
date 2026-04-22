# pql — TODO

Active work queue. This is a *forward-looking* companion to `CHANGELOG.md`:
the changelog records what *shipped* under each version header; this file
records what's *next*. When an item lands, move it out of here — the changelog
entry under the appropriate `[0.x.y-dev]` section is the record that replaces it.

Items are ordered by *when* we'd do them, not by size. The "Next" bucket is
what a fresh session should pick up; "Soon" and "Later" are runway.

## Next (pick up first)

- [x] **`pql shell` REPL** — small, contained; builds confidence in the DSL
  surface before heavier work. Read a DSL line, eval via the existing
  compiler/executor, print rows in the current `--format`. Needs: readline
  or equivalent (prefer stdlib `bufio` + terminal detection, not a new dep).
  Lands in `internal/cli/shell.go` + one integration test.
- [x] **`.base` compiler + `pql base <name>`** — translate Obsidian `.base`
  files into the PQL DSL AST, then run via the existing executor. Spec lives
  in `docs/pql-grammar.md`. Council vault has `testdata/council-snapshot/
  council-members.base` and `council-sessions.base` as ready-made fixtures.
  Lands in `internal/query/dsl/base/`.
- [x] **Shell completions** — `pql completion {bash,zsh,fish}`. Cobra
  generates these natively; wire the subcommand and ship the files under
  `docs/` or embed. Unblocks "can tab-complete subcommand and flag names"
  demo.

## Soon (v0.1.1 → v0.2)

- [x] **Planning subcommands (`decisions` + `ticket` + `plan`)** — full
  spec in `docs/structure/planning.md`; architectural rationale in
  `docs/adr/0003-pql-db-for-user-state.md`. Replaces clide's Python
  stopgap at `/var/mnt/data/projects/clide/tools/scripts/plan` (the
  stopgap's sunset gate is feature parity against the same
  `<vault>/.pql/pql.db` file). P-Q-003 resolved: in-house migration
  runner (~50 lines). FTS search and ticket export deferred — see
  open questions in `planning.md`.
- [x] **First distributed release** — v0.2.0 tagged. Goreleaser
  verified with snapshot build; release.yaml triggers on v* tags.
- [x] **Re-enable CI workflows** — ci.yaml on push+PR, release.yaml
  on v* tags. Both keep workflow_dispatch for manual runs.
- [x] **Outlinks/inlinks/headings access in the DSL** — `'x' IN
  outlinks`, `'x' IN inlinks`, `'x' IN headings` compile to EXISTS
  subqueries. Body access deferred (requires FTS5).
- [x] **`pql watch`** — fsnotify loop with 250ms debounce, pid file
  coordination, start/stop/status subcommands.

## Later (v0.3+)

- [ ] **Enrichment layer (`internal/connect/`)** — the query → connect →
  bundle pipeline. Packages: `connect/signal/`, `connect/rank/`,
  `connect/neighborhood/`, `connect/bundle/`. Wires up via the `--connect`
  flag (off by default on primitives, on by default on intents). Global
  `--flat-search` short-circuits enrichment from any subcommand. See
  `docs/structure/design-philosophy.md` and the pipeline diagram in
  `docs/structure/project-structure.md`.
- [ ] **First intents (`internal/intent/`)** — `related`, `search`,
  `context`. Each is `internal/intent/<name>/` (weights + query composition)
  + `internal/cli/intent_<name>.go` (cobra wiring). Two files per intent.
- [ ] **Ranking-quality eval harness** — `internal/connect/rank/eval_test.go`
  gated by `//go:build eval`. Goldens at `internal/connect/rank/testdata/
  golden/*.json`. Reports NDCG@k / MRR / P@k + per-signal contribution
  diffs vs. the previous baseline. `make eval` + `make eval-baseline`.
- [ ] **Telemetry (`internal/telemetry/`)** — per-phase timings
  (`generate_ms`, `rank_ms`, per-signal ms) into the stderr diagnostic
  stream on `--verbose`. Load-bearing for honest weight-tuning.
- [ ] **`pql self-update`** — once v0.1 is distributed. Hits the configured
  release endpoint, verifies SHA256, replaces atomically. Design in
  `docs/structure/initial-plan.md`.
- [ ] **Code-aware extractor** — `internal/index/extractor/code/` with
  tree-sitter. Doesn't require changes to `store/`, `connect/`, or `query/`
  (the point of the registry pattern).
- [ ] **Further users of `<vault>/.pql/pql.db`** — the user-state store
  gets cut in "Soon" alongside the planning subcommands (see
  `docs/adr/0003-pql-db-for-user-state.md`). Once planning is in, consider
  moving the skill install lock (`.pql-install.json`) into a `skill_state`
  table there, and keep the door open for ranking-weight overrides once
  the eval harness makes "hand-tuned weights per vault" a thing worth
  persisting. Neither is urgent; file under "when the first caller appears."

## Documentation debt

- [ ] **Backfill two ADRs**: `docs/adr/0001-no-vectors.md` and
  `0002-intents-not-primitives.md`. The slots exist in the plan, the
  records don't.
- [ ] **`docs/signals.md`** — signal catalog: what each measures, where
  it shines, where it fails. Lands alongside the first `connect/signal/`
  implementation, not before.
- [ ] **`docs/compatibility.md`** — binary↔skill `schema_version`
  negotiation. Stub exists; fill in when the skill actually reads
  `pql version --build-info` output to gate itself.

## Housekeeping (ambient)

- [ ] Turn the `ci/*.sh` scripts into the actual CI substance — today the
  GitHub Actions workflows call `golangci-lint-action` / `goreleaser-action`
  directly instead of `ci/lint.sh` and `ci/release.sh`. The indirection was
  the design intent; the workflows drifted.
- [ ] Add a tool directive (Go 1.24+) to `go.mod` pinning `golangci-lint`
  alongside the `govulncheck@v1.2.0` pin in the Makefile, so contributors
  don't need a separate brew install.
