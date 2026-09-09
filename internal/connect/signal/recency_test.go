package signal

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func recencyDB(t *testing.T, rows map[string]int64) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE files (path TEXT PRIMARY KEY, mtime INTEGER)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	for path, mtime := range rows {
		if _, err := db.Exec(`INSERT INTO files (path, mtime) VALUES (?, ?)`, path, mtime); err != nil {
			t.Fatalf("insert %s: %v", path, err)
		}
	}
	return db
}

// T-126: the reference instant is the Context's, not the wall clock at the
// moment each candidate is visited. With Now pinned, scores are exact and
// identical across calls — no time.Now() in the path.
func TestRecencyScoresAgainstPinnedNow(t *testing.T) {
	ref := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	db := recencyDB(t, map[string]int64{
		"fresh.md":   ref.Add(-1 * time.Hour).Unix(),
		"midlife.md": ref.Add(-45 * 24 * time.Hour).Unix(), // exactly half the 90-day decay
		"ancient.md": ref.Add(-100 * 24 * time.Hour).Unix(),
	})
	ctx := &Context{DB: db, Ctx: context.Background(), Now: ref}

	for _, tc := range []struct {
		path string
		want float64
	}{
		{"fresh.md", 1.0 - 1.0/(90*24)},
		{"midlife.md", 0.5},
		{"ancient.md", 0}, // past the clamp
	} {
		for run := range 2 {
			got, err := (Recency{}).Score(ctx, tc.path)
			if err != nil {
				t.Fatalf("Score(%s): %v", tc.path, err)
			}
			if got != tc.want {
				t.Errorf("run %d: Score(%s) = %v, want exactly %v", run, tc.path, got, tc.want)
			}
		}
	}
}

// A Context built without Now must stay correct — the zero value falls back
// to the current time rather than scoring everything from 1970.
func TestRecencyZeroNowFallsBackToWallClock(t *testing.T) {
	db := recencyDB(t, map[string]int64{
		"fresh.md": time.Now().Add(-1 * time.Hour).Unix(),
	})
	ctx := &Context{DB: db, Ctx: context.Background()}

	got, err := (Recency{}).Score(ctx, "fresh.md")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if got <= 0.9 {
		t.Errorf("Score(fresh.md) with zero Now = %v, want near 1.0", got)
	}
}
