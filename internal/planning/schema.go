package planning

import (
	"context"
	"database/sql"
	"fmt"
)

// planningSchema is the full pql.db schema. Per D-19, pql.db has no
// migration framework — when the schema needs to change, edit this
// constant and bump CanonicalVersion. Existing databases are never
// altered in place; recovery for an out-of-date pql.db is to delete
// it and run `pql plan import` (or `pql plan rebuild` once T-21 lands).
//
// Every replication-relevant column (created_at, updated_at,
// deleted_at, hash, canonical_version) is part of the table from the
// first CREATE so fresh installs land at a coherent shape with no
// ALTER deltas. See D-15 through D-18 for what these columns serve.
const planningSchema = `
CREATE TABLE IF NOT EXISTS decisions (
    id                TEXT PRIMARY KEY,
    type              TEXT NOT NULL CHECK(type IN ('confirmed','question','rejected')),
    domain            TEXT NOT NULL,
    title             TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active'
                          CHECK(status IN ('active','superseded','resolved','open')),
    date              TEXT,
    file_path         TEXT NOT NULL,
    synced_at         TEXT NOT NULL DEFAULT (datetime('now')),
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER
);

CREATE TABLE IF NOT EXISTS decision_refs (
    source_id         TEXT NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    target_id         TEXT NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
    ref_type          TEXT NOT NULL
                          CHECK(ref_type IN ('supersedes','references','resolves','depends_on','amends')),
    note              TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER,
    PRIMARY KEY (source_id, target_id, ref_type)
);

CREATE TABLE IF NOT EXISTS tickets (
    id                TEXT PRIMARY KEY,
    type              TEXT NOT NULL CHECK(type IN ('initiative','epic','story','task','bug')),
    parent_id         TEXT REFERENCES tickets(id),
    title             TEXT NOT NULL,
    description       TEXT,
    status            TEXT NOT NULL DEFAULT 'backlog'
                          CHECK(status IN ('backlog','ready','in_progress','review','done','cancelled')),
    priority          TEXT DEFAULT 'medium'
                          CHECK(priority IN ('critical','high','medium','low')),
    assigned_to       TEXT,
    team              TEXT,
    decision_ref      TEXT REFERENCES decisions(id),
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER
);

CREATE TABLE IF NOT EXISTS ticket_deps (
    blocker_id        TEXT NOT NULL REFERENCES tickets(id),
    blocked_id        TEXT NOT NULL REFERENCES tickets(id),
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER,
    PRIMARY KEY (blocker_id, blocked_id)
);

CREATE TABLE IF NOT EXISTS ticket_history (
    ticket_id         TEXT NOT NULL REFERENCES tickets(id),
    field             TEXT NOT NULL,
    old_value         TEXT,
    new_value         TEXT,
    changed_by        TEXT,
    changed_at        TEXT NOT NULL DEFAULT (datetime('now')),
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT UNIQUE,
    canonical_version INTEGER
);

CREATE TABLE IF NOT EXISTS ticket_labels (
    ticket_id         TEXT NOT NULL REFERENCES tickets(id),
    label             TEXT NOT NULL,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER,
    PRIMARY KEY (ticket_id, label)
);

CREATE TABLE IF NOT EXISTS meta (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tickets_status        ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_team          ON tickets(team);
CREATE INDEX IF NOT EXISTS idx_tickets_decision_ref  ON tickets(decision_ref);
CREATE INDEX IF NOT EXISTS idx_tickets_assigned      ON tickets(assigned_to);
CREATE INDEX IF NOT EXISTS idx_decisions_domain      ON decisions(domain);
CREATE INDEX IF NOT EXISTS idx_decisions_type        ON decisions(type);
CREATE INDEX IF NOT EXISTS idx_decision_refs_target  ON decision_refs(target_id);
`

// replicationColumns are the columns every planning table carries to
// support changelog-based replication (D-15..D-18). Tables append their
// own table-specific columns to this base set.
var replicationColumns = []string{
	"created_at", "updated_at", "deleted_at", "hash", "canonical_version",
}

// expectedColumns lists the column set every planning table must
// carry under the current schema. Used by verifySchema to detect
// pql.db files built under an older shape (D-19: no in-place upgrade).
//
// The meta table is intentionally absent: it carries replica-local
// state (export/import markers) that doesn't participate in
// replication, so it doesn't need the replication-column conventions.
//
//nolint:goconst // schema column names — extracting each as a constant
// would obscure the schema's surface area for no real benefit; this
// matches the existing pattern in repo/tickets.go for status enums.
var expectedColumns = map[string][]string{
	"decisions": append(
		[]string{"id", "type", "domain", "title", "status", "date", "file_path", "synced_at"},
		replicationColumns...,
	),
	"decision_refs": append(
		[]string{"source_id", "target_id", "ref_type", "note"},
		replicationColumns...,
	),
	"tickets": append(
		[]string{"id", "type", "parent_id", "title", "description", "status", "priority",
			"assigned_to", "team", "decision_ref"},
		replicationColumns...,
	),
	"ticket_deps": append(
		[]string{"blocker_id", "blocked_id"},
		replicationColumns...,
	),
	"ticket_history": append(
		[]string{"ticket_id", "field", "old_value", "new_value", "changed_by", "changed_at"},
		replicationColumns...,
	),
	"ticket_labels": append(
		[]string{"ticket_id", "label"},
		replicationColumns...,
	),
}

// Schema returns the full planning schema as SQL for callers that
// need to embed it (e.g. pql init seeding .pql/changelog/<table>/0000-schema.sql).
func Schema() string { return planningSchema }

// Migrate ensures the planning schema exists and matches the current
// expected shape. Per D-19, there is no migration framework — Migrate
// either creates the schema fresh or refuses to proceed when an older
// shape is detected. Recovery for the latter is to delete pql.db and
// re-import.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, planningSchema); err != nil {
		return fmt.Errorf("planning: create schema: %w", err)
	}
	return verifySchema(ctx, db)
}

func verifySchema(ctx context.Context, db *sql.DB) error {
	for table, want := range expectedColumns {
		got, err := liveColumns(ctx, db, table)
		if err != nil {
			return err
		}
		for _, col := range want {
			if !got[col] {
				return fmt.Errorf(
					"planning: %s.%s missing — pql.db is from an earlier schema.\n"+
						"Recovery (delete the local pql.db first, then):\n"+
						"  • Repo with committed .pql/changelog/:  pql plan rebuild\n"+
						"  • Repo with only .pql/pql-plan.json:    pql init    "+
						"(autoImportPlan picks the legacy path)\n"+
						"  • or pql plan import --legacy .pql/pql-plan.json",
					table, col)
			}
		}
	}
	return nil
}

func liveColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, fmt.Errorf("planning: introspect %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("planning: scan column: %w", err)
		}
		out[name] = true
	}
	return out, rows.Err()
}
