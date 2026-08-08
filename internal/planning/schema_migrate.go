package planning

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/postmeridiem/pql/internal/planning/migrate"
	"github.com/postmeridiem/pql/internal/version"
)

// schemaMigrations is the ledger: which forward steps this database has had
// applied, and when.
//
// D-19 originally ruled out a migration runner on the grounds that pql.db is
// regenerable — delete it and replay the changelog. That reasoning held while
// pql was single-author, and it is still the right recovery for a database no
// step can reach. It stopped being sufficient once the *changelog* itself
// needed migrating (T-68): that artefact is the log of record and cannot be
// regenerated from anything. Rather than build the same machine twice, both
// axes now share one runner, and this is the pql.db half (D-28 supersedes
// D-19's no-runner clause).
// The version column is TEXT because it holds a pql release ("2.0.0"), not a
// counter — the same scheme the changelog format marker uses, so a ledger row
// says which release introduced the shape it recorded.
const schemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    id         TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
`

// schemaAxisName labels the axis in diagnostics.
const schemaAxisName = "pql.db schema"

// ensureLedgerShape repairs the ledger's own table when it was created under an
// earlier shape.
//
// The ledger is the bootstrap for every other migration, so it cannot be
// migrated by the mechanism it powers — a step recording its own application
// into a table it is in the middle of reshaping is circular. It has to be
// self-correcting instead, and it can be: the ledger holds no user data, only a
// claim about what shape the database is in, and that claim is re-derivable
// from the column check plus a baseline stamp. So a wrong shape is dropped and
// rebuilt rather than reasoned about.
//
// Concretely: `version` was briefly an INTEGER PRIMARY KEY, which SQLite
// enforces as a rowid alias and which therefore rejects a release version
// outright. CREATE TABLE IF NOT EXISTS leaves an existing table alone, so
// without this the database refuses every open with a datatype mismatch.
func ensureLedgerShape(ctx context.Context, db *sql.DB) error {
	var declared string
	err := db.QueryRowContext(ctx,
		`SELECT type FROM pragma_table_info('schema_migrations') WHERE name = 'version'`).Scan(&declared)
	if err == sql.ErrNoRows {
		return nil // no ledger yet; the CREATE below makes it correctly
	}
	if err != nil {
		return fmt.Errorf("planning: inspect schema_migrations: %w", err)
	}
	if strings.EqualFold(declared, "TEXT") {
		return nil
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE schema_migrations`); err != nil {
		return fmt.Errorf("planning: rebuild schema_migrations: %w", err)
	}
	return nil
}

// schemaSteps are the forward migrations for the pql.db schema, ordered by the
// version they produce.
//
// Deliberately empty. The schema is current, so there is no pending change to
// migrate, and inventing a step for a change nobody made would be worse than
// shipping an empty list — it would be untestable ceremony that the first real
// migration would have to work around. The runner, the ledger, detection,
// baseline stamping and the refusal paths are all live and exercised; adding
// the next schema change means adding one entry here and bumping
// version.PlanningSchemaVersion.
var schemaSteps []migrate.Step

// detectSchemaVersion reads how far this database has been migrated.
//
// The ledger is authoritative when it has entries. A database that predates the
// ledger reports 0, which is not necessarily "old" — it is "unknown", and the
// shape check that follows decides whether it is a current database that simply
// has not been stamped yet or one no step can reach.
// Versions are compared with migrate.Compare rather than SQL's MAX, which
// would order "10.0.0" before "2.0.0" as text.
func detectSchemaVersion(ctx context.Context, db *sql.DB) (migrate.Version, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return "", fmt.Errorf("planning: read schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var high migrate.Version
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return "", fmt.Errorf("planning: scan schema_migrations: %w", err)
		}
		// A stamp this binary cannot interpret is not a version to migrate
		// from — no step will ever chain to it, so honouring it would strand
		// the database and refuse every open. Ignoring it demotes the database
		// to "unstamped", where verifySchema decides on the actual column shape
		// and a correct one is re-stamped. Reached by databases written during
		// the brief window when this axis used bare counters.
		if !migrate.Version(v).IsRelease() {
			continue
		}
		if migrate.Compare(migrate.Version(v), high) > 0 {
			high = migrate.Version(v)
		}
	}
	return high, rows.Err()
}

// recordSchemaStep writes one applied step to the ledger.
func recordSchemaStep(ctx context.Context, db *sql.DB, v migrate.Version, id string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO schema_migrations (version, id) VALUES (?, ?)`, string(v), id)
	if err != nil {
		return fmt.Errorf("planning: record schema migration %s: %w", v, err)
	}
	return nil
}

// migrateSchema runs the pql.db schema axis.
//
// Returns without error when there is nothing to do, which is the common case:
// a fresh database is created at the current shape by the CREATE statements, so
// the only work left is stamping the baseline. Callers run verifySchema
// afterwards as the post-condition — a step that claimed success but produced
// the wrong shape must not pass silently.
func migrateSchema(ctx context.Context, db *sql.DB) error {
	found, err := detectSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if found == "" {
		// Unstamped. Steps cannot help a database whose shape predates them —
		// verifySchema owns that diagnosis and gives the better message — so
		// only run the axis once a baseline exists.
		return nil
	}

	axis := migrate.Axis{
		Name:     schemaAxisName,
		Current:  version.PlanningSchemaVersion,
		Found:    found,
		Steps:    schemaSteps,
		Recovery: schemaRecoveryHint,
	}
	_, err = migrate.Run(ctx, axis, func(s migrate.Step) error {
		return recordSchemaStep(ctx, db, s.To, s.ID)
	})
	return err
}

// stampSchemaBaseline records a verified-current database at the current schema
// version, so the ledger has a floor to migrate forward from.
//
// Runs after verifySchema, never before: stamping a database whose shape has
// not been checked would assert something unverified, and the next release's
// migration would trust it.
func stampSchemaBaseline(ctx context.Context, db *sql.DB) error {
	found, err := detectSchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if migrate.Compare(found, version.PlanningSchemaVersion) >= 0 {
		return nil
	}
	if err := recordSchemaStep(ctx, db, version.PlanningSchemaVersion, "baseline"); err != nil {
		return err
	}
	return pruneUninterpretableStamps(ctx, db)
}

// pruneUninterpretableStamps drops ledger rows whose version is not a release,
// once a real baseline exists. detectSchemaVersion already ignores them, so
// this is tidiness rather than correctness — but a ledger is meant to be read
// by a human debugging an upgrade, and a row nothing can interpret is noise
// that invites exactly the wrong conclusion.
func pruneUninterpretableStamps(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("planning: read schema_migrations: %w", err)
	}
	var stale []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return fmt.Errorf("planning: scan schema_migrations: %w", err)
		}
		if !migrate.Version(v).IsRelease() {
			stale = append(stale, v)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("planning: read schema_migrations: %w", err)
	}
	_ = rows.Close()

	for _, v := range stale {
		if _, err := db.ExecContext(ctx,
			`DELETE FROM schema_migrations WHERE version = ?`, v); err != nil {
			return fmt.Errorf("planning: prune schema_migrations row %q: %w", v, err)
		}
	}
	return nil
}

// schemaRecoveryHint is the escape hatch for a database no forward step can
// reach. pql.db is regenerable from the committed changelog, which is what
// makes this an acceptable answer here and *not* an acceptable answer for the
// changelog itself.
const schemaRecoveryHint = "delete pql.db and run `pql plan rebuild` to regenerate it from .pql/changelog/"

// SchemaAxisStatus reports the pql.db schema axis for `pql plan upgrade`.
type SchemaAxisStatus struct {
	Name    string          `json:"name"`
	Found   migrate.Version `json:"found"`
	Current migrate.Version `json:"current"`
}

// SchemaStatus describes where this database sits on the schema axis.
func SchemaStatus(ctx context.Context, db *sql.DB) (SchemaAxisStatus, error) {
	found, err := detectSchemaVersion(ctx, db)
	if err != nil {
		return SchemaAxisStatus{}, err
	}
	return SchemaAxisStatus{
		Name:    schemaAxisName,
		Found:   found,
		Current: version.PlanningSchemaVersion,
	}, nil
}
