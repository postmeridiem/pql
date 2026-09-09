package changelog

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postmeridiem/pql/internal/planning"
)

const leak = "hostname.internal"

// seedLeakyTicket plants a ticket whose description carries the leak, plus a
// history row whose old_value carries it too — the fixed-forward shape T-130
// observed, where correcting a description writes the leaked text a second
// time.
func seedLeakyTicket(t *testing.T, db *sql.DB, recordID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tickets (record_id, type, title, description, status, priority, created_at, updated_at)
		VALUES (?, 'task', 'a title', ?, 'backlog', 'medium', '2025-05-08 11:00:00', '2025-05-08 11:00:00')
	`, recordID, "deployed on "+leak+" last week"); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_idmap (record_id, ticket_id, created_at, updated_at)
		VALUES (?, 'T-1', '2025-05-08 11:00:00', '2025-05-08 11:00:00')
	`, recordID); err != nil {
		t.Fatalf("seed idmap: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO ticket_history (ticket_record_id, field, old_value, new_value, changed_at, created_at, updated_at)
		VALUES (?, 'description', ?, 'a clean description', '2025-05-08 11:05:00', '2025-05-08 11:05:00', '2025-05-08 11:05:00')
	`, recordID, "deployed on "+leak+" last week"); err != nil {
		t.Fatalf("seed history: %v", err)
	}
	if err := planning.RehashTicket(ctx, db, recordID); err != nil {
		t.Fatalf("rehash ticket: %v", err)
	}
	if err := planning.RehashTicketIDMap(ctx, db, recordID); err != nil {
		t.Fatalf("rehash idmap: %v", err)
	}
	var rowid int64
	if err := db.QueryRowContext(ctx,
		`SELECT rowid FROM ticket_history WHERE ticket_record_id = ?`, recordID).Scan(&rowid); err != nil {
		t.Fatalf("history rowid: %v", err)
	}
	if err := planning.RehashTicketHistory(ctx, db, rowid); err != nil {
		t.Fatalf("rehash history: %v", err)
	}
}

func grepTree(t *testing.T, root, needle string) []string {
	t.Helper()
	var hits []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, err := os.ReadFile(path) //nolint:gosec // test walking its own temp dir
		if err != nil {
			return err
		}
		if strings.Contains(string(b), needle) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return hits
}

// The core promise: after a redact, the value is gone from the database, gone
// from the changelog files, the untouched second ticket's line is
// byte-identical, and the rewritten changelog still replays into a fresh
// replica whose rows carry the redacted text with verifying hashes.
func TestRedact_RemovesValueEverywhereAndStillReplays(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedLeakyTicket(t, db, "REC1")
	seedTicket(t, db, "REC2", "2025-05-08 12:00:00") // clean neighbour
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	root := filepath.Join(vault, ".pql", "changelog")
	ticketsFile := grepTree(t, filepath.Join(root, "tickets"), "REC2")
	if len(ticketsFile) != 1 {
		t.Fatalf("expected REC2 in one tickets file, got %v", ticketsFile)
	}
	before, err := os.ReadFile(ticketsFile[0]) //nolint:gosec // test reading its own temp file
	if err != nil {
		t.Fatal(err)
	}
	rec2LineBefore := ""
	for _, l := range strings.Split(string(before), "\n") {
		if strings.Contains(l, "REC2") {
			rec2LineBefore = l
		}
	}

	res, err := Redact(ctx, db, vault, "REC1", leak, "a build host")
	if err != nil {
		t.Fatalf("Redact: %v", err)
	}
	if res.TicketRows != 1 || res.HistoryRows != 1 {
		t.Errorf("rows rewritten = %d tickets, %d history; want 1, 1", res.TicketRows, res.HistoryRows)
	}
	if res.StagedLines < 2 {
		t.Errorf("changelog lines rewritten = %d, want >= 2 (a tickets line and a history line)", res.StagedLines)
	}

	// Gone from the database…
	var desc string
	if err := db.QueryRowContext(ctx,
		`SELECT description FROM tickets WHERE record_id = 'REC1'`).Scan(&desc); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(desc, leak) || !strings.Contains(desc, "a build host") {
		t.Errorf("db description not redacted: %q", desc)
	}
	// …and from every file.
	if hits := grepTree(t, root, leak); len(hits) != 0 {
		t.Errorf("leak still present in %v", hits)
	}

	// The clean neighbour's line did not move by a byte.
	after, err := os.ReadFile(ticketsFile[0]) //nolint:gosec // test reading its own temp file
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range strings.Split(string(after), "\n") {
		if l == rec2LineBefore {
			found = true
		}
	}
	if !found {
		t.Error("untouched REC2 line changed bytes during redact")
	}

	// The rewritten changelog replays into a fresh replica, and the replayed
	// rows' stored hashes verify against a database-side rehash.
	freshVault, freshDB := setupVault(t)
	copyTree(t, root, filepath.Join(freshVault, ".pql", "changelog"))
	if _, err := Import(ctx, freshDB, freshVault); err != nil {
		t.Fatalf("Import of redacted changelog: %v", err)
	}
	var got, storedHash string
	if err := freshDB.QueryRowContext(ctx,
		`SELECT description, hash FROM tickets WHERE record_id = 'REC1'`).Scan(&got, &storedHash); err != nil {
		t.Fatalf("replayed row: %v", err)
	}
	if strings.Contains(got, leak) || !strings.Contains(got, "a build host") {
		t.Errorf("replayed description = %q, want redacted", got)
	}
	if err := planning.RehashTicket(ctx, freshDB, "REC1"); err != nil {
		t.Fatal(err)
	}
	var recomputed string
	if err := freshDB.QueryRowContext(ctx,
		`SELECT hash FROM tickets WHERE record_id = 'REC1'`).Scan(&recomputed); err != nil {
		t.Fatal(err)
	}
	if storedHash != recomputed {
		t.Errorf("replayed hash %s does not verify against recompute %s", storedHash, recomputed)
	}
}

func TestRedact_ValueNotFoundErrors(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "REC1", "2025-05-08 11:00:00")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if _, err := Redact(ctx, db, vault, "REC1", "no-such-value", ""); err == nil {
		t.Fatal("Redact of absent value: nil error, want not-found")
	}
}

func TestRedact_EmptyValueRejected(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	if _, err := Redact(ctx, db, vault, "REC1", "", "x"); err == nil {
		t.Fatal("Redact with empty value: nil error, want rejection")
	}
}

// The push boundary: once the leak is in a remote's copy of the changelog,
// redacting locally is refused (D-35). Requires git on PATH.
func TestRedact_RefusesWhenPushed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	ctx := context.Background()
	vault, db := setupVault(t)
	seedLeakyTicket(t, db, "REC1")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", vault}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	remote := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, out)
	}
	run("init", "-q")
	run("add", ".pql/changelog")
	run("commit", "-q", "-m", "changelog")
	run("remote", "add", "origin", remote)
	run("push", "-q", "origin", "HEAD")
	// push does not create the remote-tracking ref without fetch config; make it explicit
	run("fetch", "-q", "origin")

	_, err := Redact(ctx, db, vault, "REC1", leak, "x")
	if err == nil {
		t.Fatal("Redact of pushed value: nil error, want refusal")
	}
	if !strings.Contains(err.Error(), "pushed") {
		t.Errorf("refusal should name the pushed state, got: %v", err)
	}
}

// Before the push, the same vault-with-git redacts fine: commits alone are
// not the boundary, remotes are.
func TestRedact_LocalCommitsAreDraft(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	ctx := context.Background()
	vault, db := setupVault(t)
	seedLeakyTicket(t, db, "REC1")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", vault}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", ".pql/changelog")
	run("commit", "-q", "-m", "changelog")

	if _, err := Redact(ctx, db, vault, "REC1", leak, "x"); err != nil {
		t.Fatalf("Redact with only local commits: %v, want success", err)
	}
	if hits := grepTree(t, filepath.Join(vault, ".pql", "changelog"), leak); len(hits) != 0 {
		t.Errorf("leak still present in %v", hits)
	}
}
