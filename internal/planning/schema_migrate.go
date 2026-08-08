package planning

import (
	"context"
	"database/sql"
	"fmt"

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
const schemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    id         TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
`

// schemaAxisName labels the axis in diagnostics.
const schemaAxisName = "pql.db schema"

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
func detectSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var found sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&found)
	if err != nil {
		return 0, fmt.Errorf("planning: read schema_migrations: %w", err)
	}
	if !found.Valid {
		return 0, nil
	}
	return int(found.Int64), nil
}

// recordSchemaStep writes one applied step to the ledger.
func recordSchemaStep(ctx context.Context, db *sql.DB, v int, id string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO schema_migrations (version, id) VALUES (?, ?)`, v, id)
	if err != nil {
		return fmt.Errorf("planning: record schema migration %d: %w", v, err)
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
	if found == 0 {
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
	if found >= version.PlanningSchemaVersion {
		return nil
	}
	return recordSchemaStep(ctx, db, version.PlanningSchemaVersion, "baseline")
}

// schemaRecoveryHint is the escape hatch for a database no forward step can
// reach. pql.db is regenerable from the committed changelog, which is what
// makes this an acceptable answer here and *not* an acceptable answer for the
// changelog itself.
const schemaRecoveryHint = "delete pql.db and run `pql plan rebuild` to regenerate it from .pql/changelog/"

// SchemaAxisStatus reports the pql.db schema axis for `pql plan upgrade`.
type SchemaAxisStatus struct {
	Name    string `json:"name"`
	Found   int    `json:"found"`
	Current int    `json:"current"`
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
