package cli

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postmeridiem/pql/internal/planning"
)

// TestBuildRecordsSection_SplitsResolvedQuestions confirms a resolved
// question renders under "Resolved questions", not "Open questions".
func TestBuildRecordsSection_SplitsResolvedQuestions(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := planning.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ins := func(id, typ, status, title string) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO decisions (id, type, domain, title, status, file_path) VALUES (?,?,?,?,?,?)`,
			id, typ, "architecture", title, status, "governance/"+typ+"/architecture.md"); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	ins("D-1", "confirmed", "active", "A decision")
	ins("Q-1", "question", "open", "An open question")
	ins("Q-2", "question", "resolved", "A resolved question")

	out, err := buildRecordsSection(ctx, db, t.TempDir())
	if err != nil {
		t.Fatalf("buildRecordsSection: %v", err)
	}

	openIdx := strings.Index(out, "## Open questions")
	resolvedIdx := strings.Index(out, "## Resolved questions")
	if openIdx < 0 {
		t.Fatalf("missing Open questions section:\n%s", out)
	}
	if resolvedIdx < 0 {
		t.Fatalf("missing Resolved questions section:\n%s", out)
	}
	openSec := out[openIdx:resolvedIdx]
	resolvedSec := out[resolvedIdx:]

	if !strings.Contains(openSec, "Q-1") {
		t.Errorf("open question Q-1 not under Open questions:\n%s", out)
	}
	if strings.Contains(openSec, "Q-2") {
		t.Errorf("resolved question Q-2 wrongly listed under Open questions:\n%s", out)
	}
	if !strings.Contains(resolvedSec, "Q-2") {
		t.Errorf("resolved question Q-2 not under Resolved questions:\n%s", out)
	}
}

// TestRegenerateDQRReadme_Idempotent locks in the fix for the #7
// false-dirty-diff: a second sync over an up-to-date README must be a
// no-op (changed == false), and resolved questions must stay parked under
// "Resolved questions" rather than drifting back into the open list.
func TestRegenerateDQRReadme_Idempotent(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := planning.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ins := func(id, typ, status, title string) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO decisions (id, type, domain, title, status, file_path) VALUES (?,?,?,?,?,?)`,
			id, typ, "architecture", title, status, "governance/"+typ+"/architecture.md"); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	ins("D-1", "confirmed", "active", "A decision")
	ins("Q-1", "question", "open", "An open question")
	ins("Q-2", "question", "resolved", "A resolved question")

	root := t.TempDir()
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("# Governance\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}

	changed, err := regenerateDQRReadme(ctx, db, root)
	if err != nil {
		t.Fatalf("first regenerate: %v", err)
	}
	if !changed {
		t.Fatal("first regenerate should have written the records section")
	}

	changed, err = regenerateDQRReadme(ctx, db, root)
	if err != nil {
		t.Fatalf("second regenerate: %v", err)
	}
	if changed {
		body, _ := os.ReadFile(readme)
		t.Fatalf("second regenerate changed an up-to-date README (false-dirty diff regressed); README:\n%s", body)
	}

	body, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if !strings.Contains(string(body), "## Resolved questions") {
		t.Errorf("Resolved questions heading dropped from regenerated README:\n%s", body)
	}
}
