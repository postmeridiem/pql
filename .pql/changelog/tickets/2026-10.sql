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
