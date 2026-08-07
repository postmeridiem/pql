package changelog

import (
	"context"
	"testing"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

func TestVerifiedRebuild_CleanVaultReportsNoDivergence(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	seedTicket(t, db, "T-2", "2025-05-08 11:00:01.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	_, report, err := VerifiedRebuild(ctx, db, vault)
	if err != nil {
		t.Fatalf("VerifiedRebuild: %v", err)
	}
	if len(report.Divergences) != 0 {
		t.Errorf("clean rebuild reported divergences: %+v", report.Divergences)
	}
	if report.RowsLost != 0 {
		t.Errorf("rows_lost = %d, want 0", report.RowsLost)
	}
	if report.RowsBefore["tickets"] != 2 || report.RowsAfter["tickets"] != 2 {
		t.Errorf("counts = %d before / %d after, want 2 / 2",
			report.RowsBefore["tickets"], report.RowsAfter["tickets"])
	}
}

// A row that is present but wrong after a replay is the failure --verify
// exists for: counts match, so nothing else notices. Here the live row holds
// content that the changelog does not, which is what an unexported mutation —
// or a replay that resolved a tie the other way — looks like.
func TestVerifiedRebuild_ReportsChangedRow(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Mutate the live row without exporting it, so the replay will restore the
	// exported content over the top.
	if _, err := db.ExecContext(ctx, `
		UPDATE tickets SET description = 'only in the live db', hash = 'deadbeef'
		WHERE record_id = 'T-1'
	`); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	_, report, err := VerifiedRebuild(ctx, db, vault)
	if err != nil {
		t.Fatalf("VerifiedRebuild: %v", err)
	}
	if len(report.Divergences) != 1 {
		t.Fatalf("want exactly one divergence, got %+v", report.Divergences)
	}
	d := report.Divergences[0]
	if d.Kind != "changed" || d.Table != "tickets" || d.Key != "T-1" {
		t.Errorf("divergence = %+v, want a changed tickets/T-1 row", d)
	}
	if d.HashBefore != "deadbeef" || d.HashAfter == "" || d.HashAfter == d.HashBefore {
		t.Errorf("hashes = %q → %q, want the pre-replay hash and a different post-replay one",
			d.HashBefore, d.HashAfter)
	}
	if report.RowsLost != 0 {
		t.Errorf("rows_lost = %d, want 0 — the row changed, it did not vanish", report.RowsLost)
	}
}

// A row present in the live database but absent from the changelog disappears
// on rebuild. That is real loss and has to be counted as such, not folded in
// with content changes.
func TestVerifiedRebuild_ReportsMissingRow(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	// Never exported, so the changelog has no record of it.
	seedTicket(t, db, "T-2", "2025-05-08 11:00:02.000")

	_, report, err := VerifiedRebuild(ctx, db, vault)
	if err != nil {
		t.Fatalf("VerifiedRebuild: %v", err)
	}
	var missing []RowDivergence
	for _, d := range report.Divergences {
		if d.Kind == "missing" {
			missing = append(missing, d)
		}
	}
	if len(missing) == 0 {
		t.Fatalf("no missing rows reported; divergences: %+v", report.Divergences)
	}
	if report.RowsLost != len(missing) {
		t.Errorf("rows_lost = %d, want %d", report.RowsLost, len(missing))
	}
	found := false
	for _, d := range missing {
		if d.Table == "tickets" && d.Key == "T-2" {
			found = true
		}
	}
	if !found {
		t.Errorf("T-2 not reported missing; got %+v", missing)
	}
}

// The T-1057 shape, from the verification side: a ticket created and appended
// to within one timestamp tick, then replayed. Under the retired hash
// tiebreaker the replay reinstated the placeholder and nothing said so; the
// point here is that --verify reports the row as changed either way, which is
// the month of silence it exists to end. Attributing the change *to* the tie
// needs T-68's staging table — see the note in verify.go.
func TestVerifiedRebuild_ReportsDivergenceOnSameTickMutation(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	const ts = "2025-06-12 10:40:59"

	seedTicket(t, db, "T-1", ts)
	if _, err := db.ExecContext(ctx, `
		UPDATE tickets SET description = '(description follows in first append)' WHERE record_id = 'T-1'
	`); err != nil {
		t.Fatalf("seed placeholder: %v", err)
	}
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("export placeholder: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE tickets SET description = 'the real description' WHERE record_id = 'T-1'
	`); err != nil {
		t.Fatalf("seed append: %v", err)
	}
	if err := repo.WriteMeta(ctx, db, repo.MetaLastExportMarker, ""); err != nil {
		t.Fatalf("reset marker: %v", err)
	}
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("export append: %v", err)
	}

	// Diverge the live row so the tie-carrying key shows up in the report.
	if _, err := db.ExecContext(ctx,
		`UPDATE tickets SET hash = 'deadbeef' WHERE record_id = 'T-1'`); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	_, report, err := VerifiedRebuild(ctx, db, vault)
	if err != nil {
		t.Fatalf("VerifiedRebuild: %v", err)
	}
	if len(report.Divergences) != 1 {
		t.Fatalf("want one divergence for the same-tick row, got %+v", report.Divergences)
	}
	d := report.Divergences[0]
	if d.Kind != "changed" || d.Key != "T-1" {
		t.Errorf("divergence = %+v, want a changed tickets/T-1 row", d)
	}

	// And the replay itself kept the later-appended text (T-59) — a rebuild
	// that reverts to the placeholder is the bug this pair of tickets closes.
	var desc string
	if err := db.QueryRowContext(ctx,
		`SELECT description FROM tickets WHERE record_id = 'T-1'`).Scan(&desc); err != nil {
		t.Fatalf("read: %v", err)
	}
	if desc != "the real description" {
		t.Errorf("description after rebuild = %q, want the later-appended text", desc)
	}
}
