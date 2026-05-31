package cli

import (
	"context"
	"database/sql"
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
