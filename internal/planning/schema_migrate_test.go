package planning

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/postmeridiem/pql/internal/planning/migrate"
	"github.com/postmeridiem/pql/internal/version"

	_ "modernc.org/sqlite"
)

func migrateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// A fresh database is created at the current shape, so the only work is
// recording that it is there — which is what gives the next release's
// migration a floor to run from.
func TestMigrate_StampsBaselineOnAFreshDatabase(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	got, err := detectSchemaVersion(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if got != version.PlanningSchemaVersion {
		t.Errorf("schema version = %s, want the current %s", got, version.PlanningSchemaVersion)
	}

	var id string
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM schema_migrations WHERE version = ?`, got).Scan(&id); err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if id != "baseline" {
		t.Errorf("ledger entry id = %q, want baseline", id)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	for i := 0; i < 3; i++ {
		if err := Migrate(ctx, db); err != nil {
			t.Fatalf("Migrate run %d: %v", i+1, err)
		}
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("ledger holds %d rows after three migrates, want 1", n)
	}
}

// A database that predates every step must keep getting verifySchema's detailed
// message — which names the missing column and both recovery paths — rather
// than a generic "no step reaches this version" from the runner.
func TestMigrate_StaleShapeStillGetsTheDetailedRecoveryHint(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)

	// A tickets table from an earlier shape: CREATE IF NOT EXISTS will leave it
	// alone, so the column check is what has to catch it.
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE tickets (id TEXT PRIMARY KEY, title TEXT)`); err != nil {
		t.Fatalf("seed old table: %v", err)
	}

	err := Migrate(ctx, db)
	if err == nil {
		t.Fatal("expected Migrate to refuse a pre-ledger database with an older shape")
	}
	if !strings.Contains(err.Error(), "pql plan rebuild") {
		t.Errorf("error should carry the recovery hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "earlier schema") {
		t.Errorf("error should be verifySchema's diagnosis, got: %v", err)
	}
}

// The ledger cannot be migrated by the mechanism it powers, so it has to fix
// its own shape. A `version INTEGER PRIMARY KEY` column — SQLite's rowid alias,
// which rejects a release version outright — is the shape that actually
// shipped in an interim build.
func TestMigrate_RepairsTheLedgersOwnShape(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
		    version    INTEGER PRIMARY KEY,
		    id         TEXT NOT NULL,
		    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
		);
		INSERT INTO schema_migrations (version, id) VALUES (1, 'baseline');
	`); err != nil {
		t.Fatalf("seed old ledger: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate over an INTEGER-keyed ledger: %v — it must repair its own shape", err)
	}

	var declared string
	if err := db.QueryRowContext(ctx,
		`SELECT type FROM pragma_table_info('schema_migrations') WHERE name = 'version'`).
		Scan(&declared); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if declared != "TEXT" {
		t.Errorf("version column is %s, want TEXT", declared)
	}
	got, err := detectSchemaVersion(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if got != version.PlanningSchemaVersion {
		t.Errorf("schema version = %q, want %s after the rebuild", got, version.PlanningSchemaVersion)
	}
}

// A ledger stamp this binary cannot interpret must not strand the database.
// Reached for real: databases stamped during the window when this axis used
// bare counters would otherwise refuse every open, with a recovery hint telling
// the user to delete a database whose shape is perfectly fine.
func TestMigrate_HealsAnUninterpretableLedgerStamp(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	// Rewrite the ledger the way the counter-based scheme left it.
	if _, err := db.ExecContext(ctx, `DELETE FROM schema_migrations`); err != nil {
		t.Fatalf("clear ledger: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, id) VALUES ('1', 'baseline')`); err != nil {
		t.Fatalf("seed counter stamp: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate over a counter-stamped ledger: %v — it should heal, not refuse", err)
	}

	got, err := detectSchemaVersion(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if got != version.PlanningSchemaVersion {
		t.Errorf("schema version = %q, want it re-stamped at %s", got, version.PlanningSchemaVersion)
	}

	var stale int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = '1'`).Scan(&stale); err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if stale != 0 {
		t.Error("the uninterpretable row is still in the ledger")
	}
}

// The step list ships empty on purpose, but the runner it feeds must work.
// Synthetic steps prove ordering, ledger recording and no-op re-runs without
// inventing a schema change nobody made.
func TestSchemaAxis_AppliesAndRecordsSyntheticSteps(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var applied []string
	const (
		next  = "2.1.0"
		after = "2.2.0"
	)
	axis := migrate.Axis{
		Name:    schemaAxisName,
		Current: after,
		Found:   version.PlanningSchemaVersion,
		Steps: []migrate.Step{
			{From: next, To: after, ID: "second", Apply: func(context.Context) error {
				applied = append(applied, "second")
				return nil
			}},
			{From: version.PlanningSchemaVersion, To: next, ID: "first", Apply: func(context.Context) error {
				applied = append(applied, "first")
				return nil
			}},
		},
		Recovery: schemaRecoveryHint,
	}

	record := func(s migrate.Step) error { return recordSchemaStep(ctx, db, s.To, s.ID) }
	if _, err := migrate.Run(ctx, axis, record); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(applied, ",") != "first,second" {
		t.Errorf("applied %v, want first then second", applied)
	}

	got, err := detectSchemaVersion(ctx, db)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if got != after {
		t.Errorf("ledger high-water = %s, want %s", got, after)
	}

	// Re-running with the ledger now ahead is a no-op.
	axis.Found = got
	applied = nil
	if _, err := migrate.Run(ctx, axis, record); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("re-run applied %v, want nothing", applied)
	}
}

func TestSchemaStatus_ReportsBothSides(t *testing.T) {
	ctx := context.Background()
	db := migrateTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st, err := SchemaStatus(ctx, db)
	if err != nil {
		t.Fatalf("SchemaStatus: %v", err)
	}
	if st.Found != version.PlanningSchemaVersion || st.Current != version.PlanningSchemaVersion {
		t.Errorf("status = %+v, want both sides at %s", st, version.PlanningSchemaVersion)
	}
	if st.Name == "" {
		t.Error("status carries no axis name")
	}
}
