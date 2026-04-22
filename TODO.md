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

## Shipped in 1.0

- [x] **Enrichment layer** — five signals, normalization, ranking, neighborhood.
- [x] **Intents** — `related`, `search`, `context` with `--flat-search`.
- [x] **Eval harness** — NDCG@k, MRR, P@k with golden test cases.
- [x] **Telemetry** — per-phase timings on `--verbose`.
- [x] **Self-update** — GitHub Releases API, SHA256, atomic replace.

## Later (v1.x+)

- [ ] **Code-aware extractor** — `internal/index/extractor/code/` with
  tree-sitter.
- [ ] **Further users of `pql.db`** — skill install lock migration,
  ranking-weight overrides.

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
