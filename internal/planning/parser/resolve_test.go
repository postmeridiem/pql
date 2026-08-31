package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDQR lays out a two-file DQR tree and returns the repo root.
func writeDQR(t *testing.T, questions, decisions string) string {
	t.Helper()
	root := t.TempDir()
	for dir, body := range map[string]string{
		"governance/questions/architecture.md": questions,
		"governance/decisions/architecture.md": decisions,
	} {
		p := filepath.Join(root, filepath.FromSlash(dir))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return root
}

func recordsIn(t *testing.T, root string) map[string]Record {
	t.Helper()
	recs, _, err := ParseAll(filepath.Join(root, "governance"), root)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	byID := make(map[string]Record, len(recs))
	for _, r := range recs {
		byID[r.ID] = r
	}
	return byID
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

const twoQuestions = `# Open Questions

### Q-1: First question
- **Status:** Open
- **Question:** One?

### Q-2: Second question
- **Status:** Open
- **Question:** Two?
`

const twoDecisions = `# Decisions

### D-1: First decision
- **Date:** 2026-08-31
- **Decision:** Yes.

### D-2: Second decision
- **Date:** 2026-08-31
- **Decision:** Also yes.
- **Raised by:** A hallway conversation.
`

func TestResolveQuestionRewritesStatusAndAddsBacklink(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	out, err := ResolveQuestion(root, recs["Q-1"], recs["D-1"])
	if err != nil {
		t.Fatalf("ResolveQuestion: %v", err)
	}

	q := readFile(t, root, "governance/questions/architecture.md")
	want := "- **Status:** Resolved → [D-1](../decisions/architecture.md#d-1-first-decision)"
	if !strings.Contains(q, want) {
		t.Errorf("question status line not written; got:\n%s", q)
	}
	if out.StatusLine != want {
		t.Errorf("receipt StatusLine = %q, want %q", out.StatusLine, want)
	}

	d := readFile(t, root, "governance/decisions/architecture.md")
	if !strings.Contains(d, "- **Raised by:** Resolved [Q-1](../questions/architecture.md#q-1-first-question).") {
		t.Errorf("backlink not written; got:\n%s", d)
	}
}

// The whole risk of this verb is editing the wrong record's prose, so pin that
// neighbours are untouched rather than only that the target changed.
func TestResolveQuestionLeavesNeighbouringRecordsAlone(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	if _, err := ResolveQuestion(root, recs["Q-1"], recs["D-1"]); err != nil {
		t.Fatalf("ResolveQuestion: %v", err)
	}

	q := readFile(t, root, "governance/questions/architecture.md")
	if !strings.Contains(q, "### Q-2: Second question\n- **Status:** Open") {
		t.Errorf("Q-2 was modified; got:\n%s", q)
	}
	d := readFile(t, root, "governance/decisions/architecture.md")
	if !strings.Contains(d, "- **Raised by:** A hallway conversation.") {
		t.Errorf("D-2's provenance was modified; got:\n%s", d)
	}
	if strings.Count(d, "**Raised by:**") != 2 {
		t.Errorf("expected exactly 2 Raised-by lines (D-1 new, D-2 original), got %d:\n%s",
			strings.Count(d, "**Raised by:**"), d)
	}
}

// An existing **Raised by:** is free prose about where a record came from.
// Overwriting it to insert a link the database already holds would destroy
// something to duplicate something.
func TestResolveQuestionPreservesExistingRaisedBy(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	out, err := ResolveQuestion(root, recs["Q-2"], recs["D-2"])
	if err != nil {
		t.Fatalf("ResolveQuestion: %v", err)
	}
	if out.BackLink != "" {
		t.Errorf("BackLink = %q, want empty — D-2 already had the field", out.BackLink)
	}
	if out.BackLinkSkipped == "" {
		t.Error("BackLinkSkipped must explain why nothing was written")
	}
	d := readFile(t, root, "governance/decisions/architecture.md")
	if !strings.Contains(d, "- **Raised by:** A hallway conversation.") {
		t.Errorf("existing provenance was overwritten; got:\n%s", d)
	}
}

// A question with no Status line parses as open, so resolving it has to add
// the line rather than silently do nothing.
func TestResolveQuestionAddsMissingStatusLine(t *testing.T) {
	root := writeDQR(t, "# Open Questions\n\n### Q-1: No status here\n- **Question:** Eh?\n", twoDecisions)
	recs := recordsIn(t, root)

	if _, err := ResolveQuestion(root, recs["Q-1"], recs["D-1"]); err != nil {
		t.Fatalf("ResolveQuestion: %v", err)
	}
	q := readFile(t, root, "governance/questions/architecture.md")
	if !strings.Contains(q, "### Q-1: No status here\n- **Status:** Resolved → [D-1]") {
		t.Errorf("status line not inserted under the heading; got:\n%s", q)
	}
}

// The point of the edit: the next sync must read the record back as resolved.
// Without this the rewrite could be syntactically fine and semantically inert.
func TestResolveQuestionSurvivesReparse(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	if _, err := ResolveQuestion(root, recs["Q-1"], recs["D-1"]); err != nil {
		t.Fatalf("ResolveQuestion: %v", err)
	}

	after := recordsIn(t, root)
	if got := after["Q-1"].Status; got != statusResolved {
		t.Errorf("Q-1 status after reparse = %q, want %q", got, statusResolved)
	}
	if got := after["Q-2"].Status; got != statusOpen {
		t.Errorf("Q-2 status after reparse = %q, want %q", got, statusOpen)
	}

	var found bool
	for _, ref := range after["Q-1"].Refs {
		if ref.TargetID == "D-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("no ref to D-1 extracted from the rewritten line; refs = %+v", after["Q-1"].Refs)
	}
}

func TestAnchorMatchesRenderedHeadingSlug(t *testing.T) {
	for _, tc := range []struct{ id, title, want string }{
		{"D-13", "plan export and plan import for versioned planning snapshots",
			"d-13-plan-export-and-plan-import-for-versioned-planning-snapshots"},
		{"Q-8", "Occasional pql.db backups into git", "q-8-occasional-pqldb-backups-into-git"},
	} {
		if got := Anchor(Record{ID: tc.id, Title: tc.title}); got != tc.want {
			t.Errorf("Anchor(%s) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestResolveQuestionErrorsOnMissingRecord(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	ghost := recs["Q-1"]
	ghost.ID = "Q-404"
	if _, err := ResolveQuestion(root, ghost, recs["D-1"]); err == nil {
		t.Error("expected an error when the record is absent from its file")
	}
}
