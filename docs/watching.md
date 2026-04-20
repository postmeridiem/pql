# `pql watch`

A toggleable filesystem watcher. Keeps a single vault's index hot by reacting to file changes in real time, instead of having every CLI invocation re-walk the tree.

This document is the spec; implementation lands under `internal/watch/` and `internal/cli/watch.go` once the design is approved.

## Why it exists

Every `pql <subcommand>` runs the incremental walker first to make sure the index is current. For a 56-file vault that's ~50ms; for a 5000-file vault it's hundreds of ms. Inside an active editing session — agent loops, repeated queries from a person taking notes — that overhead repeats over and over.

A watcher fixes that for the cases where it matters. When `pql watch` is running, file events trigger a debounced reindex automatically; subsequent CLI invocations skip the walk entirely and read straight from the up-to-date store.

## What it deliberately is not

- **Not a daemon.** `pql watch` is a foreground process the user starts. No systemd unit, no launchctl plist, no Windows service. If the user wants it backgrounded, they shell-background it themselves (`pql watch &` on Unix, `start /B pql watch` on Windows).
- **Not always-on.** There is no "pql is watching all your vaults in the background" mode. Keeping a watcher per vault you've ever opened is expensive (file descriptors, inotify watches, RSS, battery) and nobody asked for it. **The watcher only runs when the user explicitly types `pql watch`, and only for the vault they're in.**
- **Not a notification tool.** It silently keeps the index current. It does not pop alerts, send Slack messages, or print anything except diagnostics on stderr.
- **Not cross-vault.** No central registry of "all running watchers." Each vault tracks its own (single) watcher in its own `.pql/watch.pid`.

## CLI surface

```
pql watch                    # toggle: start a watcher for the cwd subtree, or stop the existing one
pql watch start [path]       # explicit start; errors if a watcher is already active in this vault
pql watch stop               # explicit stop; works from anywhere inside the vault
pql watch status             # report on the current vault's watcher (running / not running, scope, pid, started_at)
```

The bare `pql watch` is the ergonomic default — same command toggles state. `start` / `stop` / `status` are the explicit forms for scripting and for the case where the user wants to act on a watcher from a directory that isn't the watched scope.

## Toggle semantics

`pql watch` (no args) inspects `<vault>/.pql/watch.pid`:

| State of pid file | scope == cwd subtree? | Action |
|---|---|---|
| absent (or pid dead) | n/a | start a new watcher in the foreground for the cwd subtree |
| present, pid alive | yes | SIGTERM the running process; print "stopped"; exit |
| present, pid alive | no | error: "another watcher is running on `<scope>`. Run `pql watch stop` first, or `cd` into `<scope>` to toggle it." |

The "scope mismatch" rule is what keeps the toggle meaningful — running `pql watch` in a different subtree shouldn't silently kill the one that's already active.

## Scope

The watched scope is the directory where the user invoked `pql watch` (resolved to its absolute path). Constraints:

- Must be inside the vault root (or the vault root itself). Otherwise: error.
- Watched recursively — the subtree and everything under it.
- Files outside the scope still live in the index and remain queryable, but changes to them won't be picked up by the watcher; they refresh on the next CLI invocation that triggers the walker.
- **One watcher per vault** in v1. If a user wants `members/` and `sessions/` watched simultaneously they currently can't; they pick one or watch the whole vault.

## State on disk

Single file per vault: `<vault>/.pql/watch.pid`. JSON contents:

```json
{
  "pid": 12345,
  "scope": "/abs/path/to/vault/members",
  "started_at": "2026-04-20T10:15:00Z",
  "pql_version": "0.1.0-dev+abc1234"
}
```

Removed on graceful shutdown (SIGTERM / SIGINT / context cancel). Stale files (pid file present but the recorded pid is no longer alive) are treated as absent and silently overwritten.

`watch.pid` joins the existing list of `.pql/` contents documented in `vault-layout.md`.

## fsnotify integration

Library: [`github.com/fsnotify/fsnotify`](https://github.com/fsnotify/fsnotify) — pure Go, no cgo, single dep, mature. Per-platform behaviour:

- **Linux (inotify):** watches don't recurse natively; we walk the scope and add a watch per directory at startup, plus one whenever a new directory appears. The kernel's per-process watch limit defaults to 8192 (`/proc/sys/fs/inotify/max_user_watches`); typical vaults are well under this. If a vault exceeds it, fsnotify returns a clear error and `pql watch` aborts with a hint.
- **macOS:** fsnotify uses kqueue or FSEvents depending on version. Recursive watching works but events are coalesced (file-level granularity may be coarser than Linux).
- **Windows:** `ReadDirectoryChangesW` watches recursively natively — one watch handle for the whole subtree.

Built-in excludes (`.git/`, `.pql/`, sqlite sidecars) are honoured: the watcher ignores events from those paths so its own index updates don't trigger reindex storms.

## Debounce

Filesystem events arrive in bursts — a `git pull` can fire dozens per second; an editor save fires CREATE+RENAME+DELETE in quick succession. Without debounce we'd reindex repeatedly.

Strategy: collect events into a quiet window of **250 ms**. When 250 ms elapses with no new event, run one reindex pass. This handles both the "burst" case (one reindex covers the whole burst) and the "steady stream" case (reindex fires every 250 ms or so).

The deduped set of changed paths is passed to the indexer; in v1 the indexer ignores the set and runs a full incremental walk (which is already cheap thanks to mtime + content_hash short-circuiting). In v1.x the indexer can grow a `RunForFiles([]string)` API that only re-extracts the changed files.

## Reindex granularity

- **v1:** every event triggers `indexer.Run()` (full walk). Already incremental thanks to mtime + content_hash; the walk overhead is the cost.
- **v1.x:** `indexer.RunForFiles(paths []string)` re-extracts only the changed set. Same transaction discipline.

This staging means the v1 watcher works correctly from day one and gets faster as the indexer evolves.

## Lifecycle and signals

Foreground process. Runs until:

- SIGINT (Ctrl-C from the controlling terminal)
- SIGTERM (sent by `pql watch` when toggling off, or by the user)
- Context cancellation from a fatal error (fsnotify died, store became unwritable)

On any of those: stop fsnotify, drain the debounce queue (one final reindex if there are pending events), remove `.pql/watch.pid`, exit 0.

Logging is JSON-per-line on stderr per the output contract — quiet by default; `--verbose` adds per-event diagnostics for debugging editor-save patterns.

## Limitations

- **Network filesystems** (NFS, SMB, sshfs) often don't deliver fsnotify events. If `pql watch` is started on one, the watcher reports "no events received in 60s" once and then goes silent. Documented; not a v1 fix.
- **Inotify watch limit** on Linux. Already covered above.
- **Windows backgrounding** isn't friendly — Go programs don't detach cleanly. v1: shell-background only.
- **One watcher per vault**. v1.

## Implementation pointers

For the eventual `internal/watch/` package:

- `internal/watch/watcher.go` — fsnotify loop + debounce + reindex trigger
- `internal/watch/pidfile.go` — read/write `.pql/watch.pid`, stale detection
- `internal/cli/watch.go` — cobra subcommands (`watch`, `watch start|stop|status`)
- Reindex calls `indexer.New(store, cfg).Run(ctx)` (no API change to indexer in v1)
- `os.Signal` handling via `signal.NotifyContext`
- Tests: golden fsnotify event sequences, PID file lifecycle, scope-mismatch error path

## v1 scope

In:
- `pql watch` (toggle), `pql watch start|stop|status`
- One watcher per vault, foreground only
- 250 ms debounce, full `indexer.Run()` per debounced batch
- `<vault>/.pql/watch.pid` for state

Out (defer):
- Surgical per-file reindex (`indexer.RunForFiles`)
- Multiple simultaneous subtree watchers per vault
- IPC for live status streaming (`pql watch status` would talk to the running watcher)
- Backgrounding helper / detach mode
- Cross-vault registry of running watchers

These are all additive — none requires schema or contract changes.
