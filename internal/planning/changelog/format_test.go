package changelog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postmeridiem/pql/internal/planning/repo"
	"github.com/postmeridiem/pql/internal/version"
)

// oldGuard is the format-1 conflict clause: the content-hash tiebreak T-59
// retired. Tests build format-1 fixtures by rewriting current output back to
// it, so the fixtures stay in step with the real column sets.
func toFormatOne(t *testing.T, vault string) {
	t.Helper()
	root := filepath.Join(vault, ChangelogDir)
	for _, spec := range changelogTables {
		if spec.AppendOnly {
			continue
		}
		dir := filepath.Join(root, spec.Name)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") || isSchemaFile(e.Name()) {
				continue
			}
			path := filepath.Join(dir, e.Name())
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			old := strings.ReplaceAll(string(body),
				"WHERE excluded.updated_at >= "+spec.Name+".updated_at;",
				"WHERE excluded.updated_at > "+spec.Name+".updated_at OR (excluded.updated_at = "+
					spec.Name+".updated_at AND excluded.hash > "+spec.Name+".hash);")
			if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
		}
	}
	_ = os.Remove(filepath.Join(root, formatMarkerFile))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestDetectFormat_NoMarkerIsFormatOne(t *testing.T) {
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(context.Background(), db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	got, err := DetectFormat(vault)
	if err != nil {
		t.Fatalf("DetectFormat: %v", err)
	}
	if got != 1 {
		t.Errorf("format = %d, want 1 — a changelog written before formats were versioned is format 1", got)
	}
}

// A fresh vault has no changelog at all and must not be reported as stale.
func TestDetectFormat_AbsentChangelogIsCurrent(t *testing.T) {
	got, err := DetectFormat(t.TempDir())
	if err != nil {
		t.Fatalf("DetectFormat: %v", err)
	}
	if got != version.ChangelogFormat {
		t.Errorf("format = %d, want the current %d for a vault with no changelog",
			got, version.ChangelogFormat)
	}
}

// The headline case: T-1057's shape, replayed. Before the upgrade the committed
// files carry the hash tiebreak and the placeholder wins; after it, the
// later-appended text does.
func TestUpgrade_RepairsTheSameTickTieOnReplay(t *testing.T) {
	ctx := context.Background()
	srcVault, srcDB := setupVault(t)
	const ts = "2025-06-12 10:40:59"

	seedTicket(t, srcDB, "T-1", ts)
	if _, err := srcDB.ExecContext(ctx, `
		UPDATE tickets SET description = '(description follows in first append)',
		                   hash = 'ffffffffffffffff'
		WHERE record_id = 'T-1'
	`); err != nil {
		t.Fatalf("seed placeholder: %v", err)
	}
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("export placeholder: %v", err)
	}
	if _, err := srcDB.ExecContext(ctx, `
		UPDATE tickets SET description = 'the real description', hash = '0000000000000000'
		WHERE record_id = 'T-1'
	`); err != nil {
		t.Fatalf("seed append: %v", err)
	}
	if err := repo.WriteMeta(ctx, srcDB, repo.MetaLastExportMarker, ""); err != nil {
		t.Fatalf("reset marker: %v", err)
	}
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("export append: %v", err)
	}
	toFormatOne(t, srcVault)

	// Replay the format-1 files: the placeholder wins, which is the bug.
	before, beforeDB := setupVault(t)
	copyTree(t, filepath.Join(srcVault, ChangelogDir), filepath.Join(before, ChangelogDir))
	if _, err := Import(ctx, beforeDB, before); err != nil {
		t.Fatalf("Import before upgrade: %v", err)
	}
	var got string
	if err := beforeDB.QueryRowContext(ctx,
		`SELECT description FROM tickets WHERE record_id = 'T-1'`).Scan(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "(description follows in first append)" {
		t.Fatalf("fixture is not reproducing the bug: description = %q", got)
	}

	// Upgrade the files, then replay again.
	if _, err := Upgrade(ctx, srcVault, false); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	after, afterDB := setupVault(t)
	copyTree(t, filepath.Join(srcVault, ChangelogDir), filepath.Join(after, ChangelogDir))
	if _, err := Import(ctx, afterDB, after); err != nil {
		t.Fatalf("Import after upgrade: %v", err)
	}
	if err := afterDB.QueryRowContext(ctx,
		`SELECT description FROM tickets WHERE record_id = 'T-1'`).Scan(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "the real description" {
		t.Errorf("description after upgrade = %q, want the later-appended text — "+
			"the upgrade did not reach the committed rows", got)
	}
}

// Two clones upgrading independently must produce identical bytes, or the next
// merge is a conflict storm. Same requirement as idempotency, so one test.
func TestUpgrade_IsIdempotentAndByteStable(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	seedTicket(t, db, "T-2", "2025-05-08 11:00:01.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	toFormatOne(t, vault)

	path := filepath.Join(vault, ChangelogDir, "tickets", "2025-05.sql")

	first, err := Upgrade(ctx, vault, false)
	if err != nil {
		t.Fatalf("first Upgrade: %v", err)
	}
	if len(first.FilesRewritten) == 0 {
		t.Fatal("first upgrade rewrote nothing")
	}
	afterFirst := readFile(t, path)

	second, err := Upgrade(ctx, vault, false)
	if err != nil {
		t.Fatalf("second Upgrade: %v", err)
	}
	if !second.UpToDate() {
		t.Errorf("second upgrade found format %d, want it to see the stamped %d",
			second.FoundFormat, second.CurrentFormat)
	}
	if len(second.Steps) != 0 || len(second.FilesRewritten) != 0 {
		t.Errorf("second upgrade was not a no-op: steps=%v files=%v",
			second.Steps, second.FilesRewritten)
	}
	if got := readFile(t, path); got != afterFirst {
		t.Errorf("re-running the upgrade changed the bytes:\nfirst:\n%s\nsecond:\n%s", afterFirst, got)
	}
}

// An upgraded file must be exactly what a normal export would have written, or
// the two writers drift apart and every clone disagrees.
func TestUpgrade_OutputMatchesAFreshExport(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	path := filepath.Join(vault, ChangelogDir, "tickets", "2025-05.sql")
	fresh := readFile(t, path)

	toFormatOne(t, vault)
	if _, err := Upgrade(ctx, vault, false); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if got := readFile(t, path); got != fresh {
		t.Errorf("upgraded output differs from a fresh export.\nexport:\n%s\nupgrade:\n%s", fresh, got)
	}
}

// Values carrying quotes, newlines and NULLs are exactly where a regex-based
// rewrite would corrupt the log; staging exists so SQLite does the parsing.
func TestUpgrade_PreservesAwkwardValues(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	nasty := "it's got 'quotes', a\nnewline, a ; semicolon and the text ON CONFLICT(record_id)"
	if _, err := db.ExecContext(ctx,
		`UPDATE tickets SET description = ? WHERE record_id = 'T-1'`, nasty); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	toFormatOne(t, vault)
	if _, err := Upgrade(ctx, vault, false); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	dst, dstDB := setupVault(t)
	copyTree(t, filepath.Join(vault, ChangelogDir), filepath.Join(dst, ChangelogDir))
	if _, err := Import(ctx, dstDB, dst); err != nil {
		t.Fatalf("Import: %v", err)
	}
	var got string
	if err := dstDB.QueryRowContext(ctx,
		`SELECT description FROM tickets WHERE record_id = 'T-1'`).Scan(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != nasty {
		t.Errorf("value did not survive the round trip.\n got: %q\nwant: %q", got, nasty)
	}
}

// A union merge between an upgraded and a non-upgraded clone leaves the same
// row twice, differing only in the guard. The next upgrade should heal it.
func TestUpgrade_CollapsesRowsDoubledByAUnionMerge(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	path := filepath.Join(vault, ChangelogDir, "tickets", "2025-05.sql")
	current := readFile(t, path)

	// Simulate the merge result: the new-format line plus the old-format line
	// for the same row.
	toFormatOne(t, vault)
	old := readFile(t, path)
	if err := os.WriteFile(path, []byte(old+current), 0o644); err != nil {
		t.Fatalf("write doubled file: %v", err)
	}

	if _, err := Upgrade(ctx, vault, false); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	got := readFile(t, path)
	if n := strings.Count(got, "INSERT INTO tickets"); n != 1 {
		t.Errorf("file holds %d rows after the upgrade, want 1 — the doubled row was not collapsed:\n%s", n, got)
	}
	if got != current {
		t.Errorf("healed file differs from a fresh export:\n got:\n%s\nwant:\n%s", got, current)
	}
}

func TestUpgrade_DryRunReportsWithoutWriting(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	toFormatOne(t, vault)
	path := filepath.Join(vault, ChangelogDir, "tickets", "2025-05.sql")
	before := readFile(t, path)

	res, err := Upgrade(ctx, vault, true)
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if len(res.FilesRewritten) == 0 {
		t.Error("dry run reported no files, but the changelog is format 1")
	}
	if !res.DryRun {
		t.Error("result does not report itself as a dry run")
	}
	if got := readFile(t, path); got != before {
		t.Error("dry run modified the changelog")
	}
	if got, _ := DetectFormat(vault); got != 1 {
		t.Errorf("dry run stamped the marker: format is now %d", got)
	}
}

// The marker is written last, so an upgrade interrupted before stamping leaves
// files that are already correct and a marker that still says "old". The next
// run redoes the work and converges on identical bytes.
func TestUpgrade_ResumesAfterInterruptionBeforeStamping(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(ctx, db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}
	toFormatOne(t, vault)

	if _, err := Upgrade(ctx, vault, false); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	path := filepath.Join(vault, ChangelogDir, "tickets", "2025-05.sql")
	complete := readFile(t, path)

	// Roll back only the marker, as an interruption between the last write and
	// the stamp would.
	if err := os.Remove(filepath.Join(vault, ChangelogDir, formatMarkerFile)); err != nil {
		t.Fatalf("remove marker: %v", err)
	}

	res, err := Upgrade(ctx, vault, false)
	if err != nil {
		t.Fatalf("resumed Upgrade: %v", err)
	}
	if got := readFile(t, path); got != complete {
		t.Errorf("resumed upgrade produced different bytes:\n got:\n%s\nwant:\n%s", got, complete)
	}
	if len(res.FilesRewritten) != 0 {
		t.Errorf("resumed upgrade rewrote %v, but the files were already correct", res.FilesRewritten)
	}
	if got, _ := DetectFormat(vault); got != version.ChangelogFormat {
		t.Errorf("marker not restored: format = %d", got)
	}
}

func TestCheckFormat_OlderWarnsAndNewerRefuses(t *testing.T) {
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00.000")
	if _, err := Export(context.Background(), db, vault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Format 1 on disk: replay is allowed, with a warning naming the verb.
	found, warning, err := CheckFormat(vault)
	if err != nil {
		t.Fatalf("CheckFormat: %v", err)
	}
	if found != 1 || warning == "" {
		t.Errorf("found=%d warning=%q, want format 1 with a warning", found, warning)
	}
	if !strings.Contains(warning, "pql plan upgrade") {
		t.Errorf("warning should name the verb that fixes it, got %q", warning)
	}

	// A format from the future is refused: this binary cannot know what it encodes.
	marker := filepath.Join(vault, ChangelogDir, formatMarkerFile)
	if err := os.WriteFile(marker, []byte("-- pql:changelog_format: 99\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if _, _, err := CheckFormat(vault); err == nil {
		t.Error("expected a refusal for a changelog newer than the binary")
	} else if !strings.Contains(err.Error(), "99") || !strings.Contains(err.Error(), "upgrade pql") {
		t.Errorf("refusal should name the version found and say to upgrade, got: %v", err)
	}
}

func TestStripConflictClause_LeavesValuesAlone(t *testing.T) {
	stmt := `INSERT INTO tickets (record_id, title) VALUES ('R1', 'text with ) ON CONFLICT( inside') ` +
		`ON CONFLICT(record_id) DO UPDATE SET title=excluded.title WHERE excluded.updated_at >= tickets.updated_at;`
	got := stripConflictClause(stmt)
	want := `INSERT INTO tickets (record_id, title) VALUES ('R1', 'text with ) ON CONFLICT( inside');`
	if got != want {
		t.Errorf("strip cut in the wrong place.\n got: %s\nwant: %s", got, want)
	}
}
