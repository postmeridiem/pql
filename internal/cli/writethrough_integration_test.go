//go:build integration

// Integration tests for the changelog write-through guarantee (D-23) and
// the ergonomics fixes that shipped alongside it. They shell out to a
// freshly built binary against throwaway vaults, exercising the real
// mutation → changelog → rebuild path the git hooks drive.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// changelogTable concatenates every <YYYY-MM>.sql data file under
// <vault>/.pql/changelog/<table>/, so a test can assert a mutation wrote
// through without knowing the current month.
func changelogTable(t *testing.T, vault, table string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(vault, ".pql", "changelog", table, "*.sql"))
	if err != nil {
		t.Fatalf("glob changelog %s: %v", table, err)
	}
	var sb strings.Builder
	for _, m := range matches {
		// 0000-schema.sql is planted by init, not by write-through; skip it.
		if strings.HasPrefix(filepath.Base(m), "0000") {
			continue
		}
		b, err := os.ReadFile(m) //nolint:gosec // test-controlled path
		if err != nil {
			t.Fatalf("read %s: %v", m, err)
		}
		sb.Write(b)
	}
	return sb.String()
}

// TestIntegration_TicketNew_WritesThroughToChangelog is the core #1
// regression: creating a ticket must materialise it in the git-tracked
// changelog immediately, with no explicit `pql plan export`.
func TestIntegration_TicketNew_WritesThroughToChangelog(t *testing.T) {
	vault := t.TempDir()
	out, errb, code := run(t, vault, "ticket", "new", "task", "scratch ticket", "--id-only")
	if code != 0 {
		t.Fatalf("ticket new: code=%d stderr=%s", code, errb)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		t.Fatal("ticket new --id-only produced no id")
	}
	if cl := changelogTable(t, vault, "tickets"); !strings.Contains(cl, id) {
		t.Fatalf("ticket %s not written through to changelog (no plan export ran); changelog:\n%s", id, cl)
	}
}

// TestIntegration_Rebuild_PreservesUncommittedTickets reproduces the
// headline data-loss bug from pql-improvements.md #1. `pql plan rebuild`
// is exactly what the post-checkout / post-rewrite hooks run: it truncates
// the replicated tables and replays the changelog from disk. With
// write-through (D-23) the ticket's changelog line is already on disk, so
// rebuild must replay it back rather than erase it.
func TestIntegration_Rebuild_PreservesUncommittedTickets(t *testing.T) {
	vault := t.TempDir()
	out, errb, code := run(t, vault, "ticket", "new", "task", "scratch ticket", "--id-only")
	if code != 0 {
		t.Fatalf("ticket new: code=%d stderr=%s", code, errb)
	}
	id := strings.TrimSpace(string(out))

	if _, errb, code := run(t, vault, "plan", "rebuild"); code != 0 {
		t.Fatalf("plan rebuild: code=%d stderr=%s", code, errb)
	}

	lst, errb, code := run(t, vault, "ticket", "list")
	if code != 0 {
		t.Fatalf("ticket list: code=%d stderr=%s", code, errb)
	}
	if !strings.Contains(string(lst), id) {
		t.Fatalf("ticket %s lost after rebuild — the #1 data-loss bug regressed; list:\n%s", id, lst)
	}
}

// TestIntegration_AllTicketMutationsWriteThrough guards against a future
// mutation verb forgetting its exportThrough call: after exercising each
// replicated table, its changelog file must be non-empty.
func TestIntegration_AllTicketMutationsWriteThrough(t *testing.T) {
	vault := t.TempDir()
	mustRun := func(args ...string) string {
		out, errb, code := run(t, vault, args...)
		if code != 0 {
			t.Fatalf("%v: code=%d stderr=%s", args, code, errb)
		}
		return strings.TrimSpace(string(out))
	}

	a := mustRun("ticket", "new", "task", "first", "--id-only")
	b := mustRun("ticket", "new", "task", "second", "--id-only")
	mustRun("ticket", "status", a, "ready")     // tickets + ticket_history
	mustRun("ticket", "block", b, "--by", a)    // ticket_deps
	mustRun("ticket", "label", a, "add", "now") // ticket_labels
	mustRun("ticket", "append", a, "a note")    // tickets + ticket_history

	for _, table := range []string{"tickets", "ticket_deps", "ticket_labels", "ticket_history"} {
		if strings.TrimSpace(changelogTable(t, vault, table)) == "" {
			t.Errorf("changelog table %q empty — a mutation did not write through (missing exportThrough?)", table)
		}
	}
}

// TestIntegration_TicketNew_IdOnly checks #5: --id-only prints just the
// bare T-NNN, no JSON envelope.
func TestIntegration_TicketNew_IdOnly(t *testing.T) {
	vault := t.TempDir()
	out, errb, code := run(t, vault, "ticket", "new", "task", "x", "--id-only")
	if code != 0 {
		t.Fatalf("ticket new --id-only: code=%d stderr=%s", code, errb)
	}
	got := strings.TrimSpace(string(out))
	if strings.Contains(got, "{") || strings.Contains(got, "\"id\"") {
		t.Errorf("--id-only should print bare id, got JSON: %q", got)
	}
	if !strings.HasPrefix(got, "T-") {
		t.Errorf("--id-only should print T-NNN, got %q", got)
	}
}

// TestIntegration_TicketFlagParity guards #6: the binary must accept the
// query flags the embedded skill documents, so the skill can never again
// advertise flags the paired binary lacks.
func TestIntegration_TicketFlagParity(t *testing.T) {
	vault := t.TempDir()
	id := strings.TrimSpace(mustNewTicket(t, vault))

	for _, args := range [][]string{
		{"ticket", "show", id, "--tree"},
		{"ticket", "show", id, "--with-children"},
		{"ticket", "show", id, "--with-context"},
		{"ticket", "list", "--leaf"},
		{"ticket", "list", "--unblocked"},
		{"ticket", "list", "--under", id},
	} {
		if _, errb, code := run(t, vault, args...); code != 0 {
			t.Errorf("%v rejected (code=%d) — skill/binary flag drift; stderr=%s", args, code, errb)
		}
	}
}

func mustNewTicket(t *testing.T, vault string) string {
	t.Helper()
	out, errb, code := run(t, vault, "ticket", "new", "task", "parity", "--id-only")
	if code != 0 {
		t.Fatalf("ticket new: code=%d stderr=%s", code, errb)
	}
	return string(out)
}
