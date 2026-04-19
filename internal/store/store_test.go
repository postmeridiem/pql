package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/postmeridiem/pql/internal/store/schema"
)

func tempDB(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "pql.sqlite")
}

func TestOpen_FreshDBCreatesSchema(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	v, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != schema.Version {
		t.Fatalf("schema_version = %d, want %d", v, schema.Version)
	}

	// All v1 tables should be present.
	want := []string{"index_meta", "files", "frontmatter", "tags", "links", "headings", "bases"}
	for _, table := range want {
		var n int
		err := s.DB().QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`,
			table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("probe %q: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %q missing after fresh open", table)
		}
	}
}

func TestOpen_ReopenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := tempDB(t)

	s1, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	// Insert a row so we can prove the second open doesn't wipe data.
	_, err = s1.DB().ExecContext(ctx,
		`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"x.md", 0, 0, 0, "deadbeef", 0,
	)
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	s2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()

	var count int
	if err := s2.DB().QueryRowContext(ctx, `SELECT count(*) FROM files`).Scan(&count); err != nil {
		t.Fatalf("count files: %v", err)
	}
	if count != 1 {
		t.Fatalf("files count after reopen = %d, want 1 (data wiped — rebuild fired unexpectedly)", count)
	}
}

func TestOpen_SchemaMismatchTriggersRebuild(t *testing.T) {
	ctx := context.Background()
	path := tempDB(t)

	s1, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	// Seed, then forge a future schema_version on disk.
	_, _ = s1.DB().ExecContext(ctx,
		`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
		 VALUES ('x.md', 0, 0, 0, 'deadbeef', 0)`,
	)
	_, err = s1.DB().ExecContext(ctx,
		`UPDATE index_meta SET value=? WHERE key='schema_version'`,
		"99",
	)
	if err != nil {
		t.Fatalf("forge schema_version: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	s2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open (expecting rebuild): %v", err)
	}
	defer s2.Close()

	v, err := s2.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion after rebuild: %v", err)
	}
	if v != schema.Version {
		t.Fatalf("schema_version after rebuild = %d, want %d", v, schema.Version)
	}

	// Data should be gone — rebuild dropped and recreated everything.
	var count int
	if err := s2.DB().QueryRowContext(ctx, `SELECT count(*) FROM files`).Scan(&count); err != nil {
		t.Fatalf("count files: %v", err)
	}
	if count != 0 {
		t.Fatalf("files count after rebuild = %d, want 0", count)
	}
}

func TestOpen_SetsWALMode(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	mode, err := s.JournalMode(ctx)
	if err != nil {
		t.Fatalf("JournalMode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestOpen_EnablesForeignKeys(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	on, err := s.ForeignKeysEnabled(ctx)
	if err != nil {
		t.Fatalf("ForeignKeysEnabled: %v", err)
	}
	if !on {
		t.Fatal("foreign_keys not enabled")
	}

	// Cascade behavior: deleting a file should cascade to its tags.
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
		 VALUES ('x.md', 0, 0, 0, 'h', 0)`,
	)
	if err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO tags (path, tag) VALUES ('x.md', 'foo')`,
	)
	if err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := s.DB().ExecContext(ctx, `DELETE FROM files WHERE path='x.md'`); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	var n int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM tags WHERE path='x.md'`).Scan(&n); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if n != 0 {
		t.Fatalf("tag count after cascade delete = %d, want 0", n)
	}
}

func TestOpen_EmptyPathRejected(t *testing.T) {
	if _, err := Open(context.Background(), ""); err == nil {
		t.Fatal("expected error from empty path, got nil")
	}
}
