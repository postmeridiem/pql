package planning

import (
	"context"
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
