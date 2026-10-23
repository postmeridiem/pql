INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GMQ1SV32QQ07YXRWVN4B73QR', 'story', NULL, 'A machine-readable command manifest, so adapters wrap the CLI without scraping --help', 'An adapter that shells out to the CLI — the MCP that D-5''s cost clause leaves room for — has no machine-readable way to learn which verbs exist, what they take, or whether they write. Its options are a hand-kept list, which drifts with every release, or parsing `--help`, which is prose and not a contract.

**Proposal:** a verb that emits the command tree as JSON (`pql manifest`, or `pql help --json`). One entry per command path:

- `path` (e.g. `ticket status`), `short` description
- positional `args`: name, required, variadic
- `flags`: name, type, default, description
- `mutates`: whether it writes `pql.db`, the changelog or vault files
- `exposed`: whether it is meant for adapters at all — `init`, `self-update`, `shell`, `watch` and `completion` are CLI-only

Generated from the cobra tree, with `mutates` and `exposed` as an annotation on each command, so the manifest cannot drift from the binary.

**Acceptance:**
- every command carries both annotations, and a test fails when one is missing — a new verb cannot ship unclassified;
- the manifest round-trips: each listed path resolves to a real command, and no exposed command is missing;
- the output follows the existing JSON contract (stdout data, stderr diagnostics, exit codes).

Downstream use: an adapter wraps every exposed verb, offering reads freely and asking before a write, and gains new verbs with a pql release instead of a code change of its own.', 'backlog', 'medium', NULL, NULL, NULL, '2026-10-17 20:21:56.121', '2026-10-17 20:21:56.121', NULL, 'eb0314a5ba8bc608ffe27aa983d30721', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GNC718QHYNBNACQRDJM8VGNC', 'task', NULL, 'A ticket''s priority cannot be changed after it is created', 'pql ticket has verbs for status, label, assign, team, parent and decision, but none for priority: a ticket filed at medium that turns out to be urgent stays medium, or the urgency lives only in an appended note where list --priority and the board cannot see it. Add a priority verb on the same shape as status (one or more tickets, a value from the configured vocabulary, recorded in the ticket''s history).', 'backlog', 'medium', NULL, NULL, NULL, '2026-10-19 21:40:48.444', '2026-10-19 21:40:48.444', NULL, '9bcc2bbcf85dd57b19079ebd24a109c9', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
INSERT INTO tickets (record_id, type, parent_record_id, title, description, status, priority, assigned_to, team, decision_ref, created_at, updated_at, deleted_at, hash, canonical_version) VALUES ('06GPN1R2JADNDCPAZEX7GV04MC', 'bug', NULL, 'Installed pre-commit hook fails every commit from a linked worktree', 'The `.git/hooks/pre-commit` that `pql init` installs sources the repo''s own pql hook with:

    _pql_hook="$(git rev-parse --show-toplevel)/.pql/hooks/pre-commit"; [ -f "$_pql_hook" ] && . "$_pql_hook"

**In a linked git worktree,** `.pql/hooks/pre-commit` doesn''t exist at the worktree''s top level when it''s untracked. The `[ -f … ]` test fails, its exit status is the script''s, and every commit from that worktree fails with exit 1 and no output.

**The fix:** use `if [ -f "$_pql_hook" ]; then . "$_pql_hook"; fi`, so a missing hook is a no-op. Also consider whether the sourced hook should resolve from the common git dir rather than the worktree''s top level.

Related: T-111. That one is the export landing in the wrong vault when the hook does run; this is the hook failing outright.', 'backlog', 'medium', NULL, NULL, NULL, '2026-10-23 20:49:55.346', '2026-10-23 20:49:55.346', NULL, 'e069ed44879ee3010df85ab8e72f102a', 2) ON CONFLICT(record_id) DO UPDATE SET type=excluded.type, parent_record_id=excluded.parent_record_id, title=excluded.title, description=excluded.description, status=excluded.status, priority=excluded.priority, assigned_to=excluded.assigned_to, team=excluded.team, decision_ref=excluded.decision_ref, updated_at=excluded.updated_at, deleted_at=excluded.deleted_at, hash=excluded.hash, canonical_version=excluded.canonical_version WHERE excluded.updated_at >= tickets.updated_at;
