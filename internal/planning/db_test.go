package planning

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenPath_Memory(t *testing.T) {
	ctx := context.Background()
	db, err := OpenPath(ctx, ":memory:")
	if err != nil {
		t.Fatalf("OpenPath: %v", err)
	}
	defer func() { _ = db.Close() }()

	if db.Path() != ":memory:" {
		t.Errorf("Path = %q, want :memory:", db.Path())
	}

	// Verify tables exist by inserting a decision.
	_, err = db.SQL().ExecContext(ctx,
		`INSERT INTO decisions (id, type, domain, title, file_path)
		 VALUES ('D-001', 'confirmed', 'test', 'test decision', 'test.md')`)
	if err != nil {
		t.Fatalf("insert decision: %v", err)
	}
}

func TestOpen_CreatesDir(t *testing.T) {
	ctx := context.Background()
	vault := t.TempDir()

	db, err := Open(ctx, vault)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if db.Path() == "" {
		t.Fatal("empty path")
	}

	_, err = db.SQL().ExecContext(ctx,
		`INSERT INTO decisions (id, type, domain, title, file_path)
		 VALUES ('D-001', 'confirmed', 'arch', 'test', 'arch.md')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// TestOpenPath_SingleConnection guards the 1.10.2 concurrency fix: the
// planning DB caps the pool at one connection so the busy_timeout PRAGMA
// governs every write. Without it, a write on a fresh pooled connection has
// busy_timeout=0 and fails immediately with SQLITE_BUSY (exit 69) when a
// concurrent process holds the write lock.
func TestOpenPath_SingleConnection(t *testing.T) {
	ctx := context.Background()
	d, err := OpenPath(ctx, filepath.Join(t.TempDir(), "pql.db"))
	if err != nil {
		t.Fatalf("OpenPath: %v", err)
	}
	defer func() { _ = d.Close() }()
	if got := d.SQL().Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1 (busy_timeout must govern every write)", got)
	}
}
