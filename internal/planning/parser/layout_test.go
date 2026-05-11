package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFileAt creates parent dirs as needed and writes the given body.
func writeFileAt(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestParseAll_GovernanceLayout covers D-21's subdir form: records
// land in decisions/, questions/, rejected/ and the walker descends
// into each, attributing type from the parent directory.
func TestParseAll_GovernanceLayout(t *testing.T) {
	root := t.TempDir()
	dqr := filepath.Join(root, "governance")

	writeFileAt(t, filepath.Join(dqr, "decisions", "architecture.md"), `### D-1: a decision
- **Date:** 2026-01-01
`)
	writeFileAt(t, filepath.Join(dqr, "questions", "architecture.md"), `### Q-1: a question
- **Date:** 2026-01-02
`)
	writeFileAt(t, filepath.Join(dqr, "rejected", "architecture.md"), `### R-1: a rejected proposal
- **Rejected:** 2026-01-03
`)
	// Ignored: README at the root, and any unrelated file.
	writeFileAt(t, filepath.Join(dqr, "README.md"), "# DQR\n")

	recs, warnings, err := ParseAll(dqr, root)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	byType := map[string]Record{}
	for _, r := range recs {
		byType[r.Type] = r
	}
	if byType["confirmed"].ID != "D-1" || byType["confirmed"].Domain != "architecture" {
		t.Errorf("D-record mis-routed: %+v", byType["confirmed"])
	}
	if byType["question"].ID != "Q-1" || byType["question"].Domain != "architecture" {
		t.Errorf("Q-record mis-routed: %+v", byType["question"])
	}
	if byType["rejected"].ID != "R-1" || byType["rejected"].Domain != "architecture" {
		t.Errorf("R-record mis-routed: %+v", byType["rejected"])
	}
}

// TestParseAll_SubdirTypeMismatch covers the warning fired when a
// record's heading prefix doesn't match its subdirectory.
func TestParseAll_SubdirTypeMismatch(t *testing.T) {
	root := t.TempDir()
	dqr := filepath.Join(root, "governance")

	// D-record planted in questions/ — must surface a warning.
	writeFileAt(t, filepath.Join(dqr, "decisions", "x.md"), `### D-1: ok
- **Date:** 2026-01-01
`)
	writeFileAt(t, filepath.Join(dqr, "questions", "mistake.md"), `### D-2: wrong subdir
- **Date:** 2026-01-01
`)

	recs, warnings, err := ParseAll(dqr, root)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("records = %d, want 2", len(recs))
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "subdir-type mismatch") && strings.Contains(w, "D-2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected subdir-type mismatch warning for D-2, got %v", warnings)
	}
}

// TestParseAll_FilenameStyleWarning regresses T-36: filenames that
// violate the lowercase-hyphenated convention surface as warnings.
func TestParseAll_FilenameStyleWarning(t *testing.T) {
	root := t.TempDir()
	dqr := filepath.Join(root, "governance")

	writeFileAt(t, filepath.Join(dqr, "decisions", "Bad_Name.md"), `### D-1: oops
- **Date:** 2026-01-01
`)
	_, warnings, err := ParseAll(dqr, root)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected style warning for Bad_Name.md")
	}
	if !strings.Contains(warnings[0], "filename style") {
		t.Errorf("expected filename style warning, got %q", warnings[0])
	}
}

// TestParseAll_LegacyFlatLayout covers the fallback: when no
// decisions/ / questions/ / rejected/ subdirectories are present at
// the dqrRoot, the walker uses the pre-D-21 flat behaviour. This
// keeps existing consumers (clide, settled-reach) parseable until
// they migrate.
func TestParseAll_LegacyFlatLayout(t *testing.T) {
	root := t.TempDir()
	dqr := filepath.Join(root, "decisions")
	writeFileAt(t, filepath.Join(dqr, "architecture.md"), `### D-1: legacy
- **Date:** 2026-01-01
`)
	writeFileAt(t, filepath.Join(dqr, "questions-architecture.md"), `### Q-1: legacy q
- **Date:** 2026-01-02
`)

	recs, warnings, err := ParseAll(dqr, root)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("legacy layout should not warn: %v", warnings)
	}
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	// Legacy: question-prefix file still gets domain = "architecture".
	for _, r := range recs {
		if r.Domain != "architecture" {
			t.Errorf("expected domain=architecture, got %q for %s", r.Domain, r.ID)
		}
	}
}
