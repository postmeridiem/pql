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

-- Identity split (D-26): a ticket's stable, collision-proof identity is its
-- record_id (a locally-generated ULID, planning.NewRecordID); the friendly
-- T-NNN label lives in ticket_idmap and may be reconciled. Every structural
-- reference (parent, deps, history, labels) targets record_id, so a label
-- clash never corrupts the graph — only ticket_idmap needs a relabel.
CREATE TABLE IF NOT EXISTS tickets (
    record_id         TEXT PRIMARY KEY,
    type              TEXT NOT NULL CHECK(type IN ('initiative','epic','story','task','bug')),
    parent_record_id  TEXT REFERENCES tickets(record_id),
    title             TEXT NOT NULL,
    description       TEXT,
    -- No CHECK enumeration: the ticket status vocabulary is per-vault
    -- configurable (ticket_statuses in .pql/config.yaml). Validation lives
    -- in Go (planning.StatusSet), so adding/renaming statuses needs no
    -- schema change. The DEFAULT is a harmless fallback — CreateTicket
    -- always inserts the configured default explicitly.
    status            TEXT NOT NULL DEFAULT 'backlog',
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

-- ticket_idmap maps a record_id to its current friendly label (T-NNN).
-- ticket_id is intentionally NOT globally unique: two uncoordinated clones
-- can mint the same label, which surfaces as a duplicate-label collision
-- (detected at replay) and is fixed with "pql ticket relabel".
CREATE TABLE IF NOT EXISTS ticket_idmap (
    record_id         TEXT PRIMARY KEY REFERENCES tickets(record_id),
    ticket_id         TEXT NOT NULL,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER
);

CREATE TABLE IF NOT EXISTS ticket_deps (
    blocker_record_id TEXT NOT NULL REFERENCES tickets(record_id),
    blocked_record_id TEXT NOT NULL REFERENCES tickets(record_id),
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER,
    PRIMARY KEY (blocker_record_id, blocked_record_id)
);

CREATE TABLE IF NOT EXISTS ticket_history (
    ticket_record_id  TEXT NOT NULL REFERENCES tickets(record_id),
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
    ticket_record_id  TEXT NOT NULL REFERENCES tickets(record_id),
    label             TEXT NOT NULL,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at        TEXT,
    hash              TEXT,
    canonical_version INTEGER,
    PRIMARY KEY (ticket_record_id, label)
);

CREATE TABLE IF NOT EXISTS meta (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// planningIndexes is created after verifySchema so that an out-of-date
// pql.db (missing a renamed column) surfaces the friendly schema-mismatch
// error from verifySchema, rather than a raw "no such column" failure when
// an index references a column the old shape lacks (e.g. parent_record_id).
const planningIndexes = `
CREATE INDEX IF NOT EXISTS idx_tickets_status        ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_team          ON tickets(team);
CREATE INDEX IF NOT EXISTS idx_tickets_decision_ref  ON tickets(decision_ref);
CREATE INDEX IF NOT EXISTS idx_tickets_assigned      ON tickets(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tickets_parent        ON tickets(parent_record_id);
CREATE INDEX IF NOT EXISTS idx_ticket_idmap_label    ON ticket_idmap(ticket_id);
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
// would obscure the schema's surface area for no real benefit; this
// matches the existing pattern in repo/tickets.go for status enums.
//
//nolint:goconst // schema column names — extracting each as a constant
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
		[]string{"record_id", "type", "parent_record_id", "title", "description", "status", "priority",
			"assigned_to", "team", "decision_ref"},
		replicationColumns...,
	),
	"ticket_idmap": append(
		[]string{"record_id", "ticket_id"},
		replicationColumns...,
	),
	"ticket_deps": append(
		[]string{"blocker_record_id", "blocked_record_id"},
		replicationColumns...,
	),
	"ticket_history": append(
		[]string{"ticket_record_id", "field", "old_value", "new_value", "changed_by", "changed_at"},
		replicationColumns...,
	),
	"ticket_labels": append(
		[]string{"ticket_record_id", "label"},
		replicationColumns...,
	),
}

// Schema returns the full planning schema as SQL for callers that
// need to embed it (e.g. pql init seeding .pql/changelog/<table>/0000-schema.sql).
func Schema() string { return planningSchema + planningIndexes }

// Migrate ensures the planning schema exists, is carried forward by any
// applicable migration step, and matches the current expected shape.
//
// The order is deliberate. CREATE IF NOT EXISTS brings a fresh database to the
// current shape and is a no-op on an existing one. Forward steps then run
// against a database that has a ledger to migrate from. verifySchema is the
// post-condition — a step that reported success but produced the wrong shape
// must not pass, and a database that predates every step still gets the
// detailed recovery hint it always did, which is better than a generic
// "no step reaches this version". Only then is the baseline stamped: recording
// a version for a shape nobody verified would be an assertion the next
// release's migration would trust.
//
// D-28 supersedes D-19's no-runner clause; see schema_migrate.go for why.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, planningSchema); err != nil {
		return fmt.Errorf("planning: create schema: %w", err)
	}
	if err := ensureLedgerShape(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, schemaMigrations); err != nil {
		return fmt.Errorf("planning: create schema_migrations: %w", err)
	}
	if err := migrateSchema(ctx, db); err != nil {
		return err
	}
	// Verify column shape before creating indexes: an out-of-date pql.db
	// would otherwise fail on an index over a renamed column with a raw
	// SQLite error instead of the friendly recovery hint.
	if err := verifySchema(ctx, db); err != nil {
		return err
	}
	if err := stampSchemaBaseline(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, planningIndexes); err != nil {
		return fmt.Errorf("planning: create indexes: %w", err)
	}
	return nil
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
						"  • Recommended — repo with committed .pql/changelog/:  pql plan rebuild\n"+
						"  • Legacy repo (pre-D-15, only .pql/pql-plan.json):    "+
						"pql plan import --legacy .pql/pql-plan.json\n"+
						"    (NOTE: pql-plan.json is no longer written and may be stale — "+
						"prefer the changelog rebuild whenever .pql/changelog/ exists)",
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
