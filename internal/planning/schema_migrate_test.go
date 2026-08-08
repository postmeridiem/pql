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
		t.Errorf("schema version = %d, want the current %d", got, version.PlanningSchemaVersion)
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
	axis := migrate.Axis{
		Name:    schemaAxisName,
		Current: version.PlanningSchemaVersion + 2,
		Found:   version.PlanningSchemaVersion,
		Steps: []migrate.Step{
			{To: version.PlanningSchemaVersion + 2, ID: "second", Apply: func(context.Context) error {
				applied = append(applied, "second")
				return nil
			}},
			{To: version.PlanningSchemaVersion + 1, ID: "first", Apply: func(context.Context) error {
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
	if got != version.PlanningSchemaVersion+2 {
		t.Errorf("ledger high-water = %d, want %d", got, version.PlanningSchemaVersion+2)
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
		t.Errorf("status = %+v, want both sides at %d", st, version.PlanningSchemaVersion)
	}
	if st.Name == "" {
		t.Error("status carries no axis name")
	}
}
