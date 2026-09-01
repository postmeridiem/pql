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

func TestCloseRewritesStatusAndAddsBacklink(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	out, err := Close(root, recs["Q-1"], ptr(recs["D-1"]))
	if err != nil {
		t.Fatalf("Close: %v", err)
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
func TestCloseLeavesNeighbouringRecordsAlone(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	if _, err := Close(root, recs["Q-1"], ptr(recs["D-1"])); err != nil {
		t.Fatalf("Close: %v", err)
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
func TestClosePreservesExistingRaisedBy(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	out, err := Close(root, recs["Q-2"], ptr(recs["D-2"]))
	if err != nil {
		t.Fatalf("Close: %v", err)
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
func TestCloseAddsMissingStatusLine(t *testing.T) {
	root := writeDQR(t, "# Open Questions\n\n### Q-1: No status here\n- **Question:** Eh?\n", twoDecisions)
	recs := recordsIn(t, root)

	if _, err := Close(root, recs["Q-1"], ptr(recs["D-1"])); err != nil {
		t.Fatalf("Close: %v", err)
	}
	q := readFile(t, root, "governance/questions/architecture.md")
	if !strings.Contains(q, "### Q-1: No status here\n- **Status:** Resolved → [D-1]") {
		t.Errorf("status line not inserted under the heading; got:\n%s", q)
	}
}

// The point of the edit: the next sync must read the record back as resolved.
// Without this the rewrite could be syntactically fine and semantically inert.
func TestCloseSurvivesReparse(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	if _, err := Close(root, recs["Q-1"], ptr(recs["D-1"])); err != nil {
		t.Fatalf("Close: %v", err)
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

func TestCloseErrorsOnMissingRecord(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	ghost := recs["Q-1"]
	ghost.ID = "Q-404"
	if _, err := Close(root, ghost, ptr(recs["D-1"])); err == nil {
		t.Error("expected an error when the record is absent from its file")
	}
}

// --- supersession detection (T-119) ---------------------------------------

// Supersession is written into the status field in practice, not as the
// standalone field the original regex expected. Reading only the latter left
// real superseded decisions reporting `active`, which looks plausible and so
// went unnoticed.
func TestInferStatusReadsSupersessionFromStatusLine(t *testing.T) {
	for _, tc := range []struct {
		name, recType, statusLine, want string
	}{
		{"status-line form", typeConfirmed,
			"- **Status:** Superseded by [D-15](#d-15-x)", statusSuperseded},
		{"standalone field form", typeConfirmed,
			"- **Superseded by:** [D-15](#d-15-x)", statusSuperseded},
		{"plain active record", typeConfirmed,
			"- **Status:** Active", statusActive},

		// A record replaced only in part is still live. D-19 says exactly
		// this and is still cited as an invariant, so marking it superseded
		// would retire a rule the code still follows.
		{"superseded in part", typeConfirmed,
			"- **Status:** Superseded in part by [D-28](#d-28-x) — one clause only", statusActive},
		{"partially superseded", typeConfirmed,
			"- **Status:** Partially superseded by [D-28](#d-28-x)", statusActive},

		// The same hedge on a question keeps it open rather than resolved.
		{"question resolved", typeQuestion,
			"- **Status:** Resolved → [D-13](#d-13-x)", statusResolved},
		{"question partially resolved", typeQuestion,
			"- **Status:** Partially resolved by [D-23](#d-23-x); the rest is open", statusOpen},
		{"question open", typeQuestion, "- **Status:** Open", statusOpen},
		{"question superseded", typeQuestion,
			"- **Status:** Superseded by [Q-13](#q-13-x)", statusSuperseded},
	} {
		if got := inferStatus(tc.recType, []string{tc.statusLine}); got != tc.want {
			t.Errorf("%s: inferStatus = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// ptr is a helper for passing a record as the optional target.
func ptr(r Record) *Record { return &r }

// --- close relations (T-118) ----------------------------------------------

func TestRelationForDerivesFromTypePair(t *testing.T) {
	q := Record{ID: "Q-1", Type: typeQuestion}
	q2 := Record{ID: "Q-2", Type: typeQuestion}
	d := Record{ID: "D-1", Type: typeConfirmed}
	d2 := Record{ID: "D-2", Type: typeConfirmed}
	r := Record{ID: "R-1", Type: typeRejected}

	for _, tc := range []struct {
		name             string
		subject, target  Record
		want             Relation
		wantErrSubstring string
	}{
		{name: "question answered by decision", subject: q, target: d, want: RelResolved},
		{name: "question answered no", subject: q, target: r, want: RelResolved},
		{name: "decision replaced", subject: d, target: d2, want: RelSuperseded},

		// The pair that produced this verb's original design mistake. The
		// message has to name the routes out, not just refuse.
		{name: "question into question", subject: q, target: q2,
			wantErrSubstring: "cross-reference"},
		// D -> D is legal, so without a self guard a decision could supersede
		// itself and reparse as superseded-by-nothing.
		{name: "self close", subject: d, target: d,
			wantErrSubstring: "cannot close into itself"},
		{name: "decision into question", subject: d, target: q2,
			wantErrSubstring: "cannot close into a question"},
		{name: "rejection has nothing to close", subject: r, target: d,
			wantErrSubstring: "no further state to close into"},
	} {
		got, err := RelationFor(tc.subject, tc.target)
		if tc.wantErrSubstring != "" {
			if err == nil {
				t.Errorf("%s: expected an error", tc.name)
			} else if !strings.Contains(err.Error(), tc.wantErrSubstring) {
				t.Errorf("%s: error = %q, want it to mention %q", tc.name, err, tc.wantErrSubstring)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		} else if got != tc.want {
			t.Errorf("%s: relation = %q, want %q", tc.name, got, tc.want)
		}
	}
}

const supersedable = `# Decisions

### D-1: First decision
- **Date:** 2026-08-31
- **Decision:** Yes.

### D-2: Second decision
- **Date:** 2026-08-31
- **Decision:** Actually no.
`

// Supersession writes a pair across two records, and both halves must land or
// the tree contradicts itself — the old record would still read active while
// the new one claims to have replaced it.
func TestCloseSupersedesWritesBothSides(t *testing.T) {
	root := writeDQR(t, twoQuestions, supersedable)
	recs := recordsIn(t, root)

	out, err := Close(root, recs["D-1"], ptr(recs["D-2"]))
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if out.Relation != RelSuperseded {
		t.Errorf("relation = %q, want %q", out.Relation, RelSuperseded)
	}

	d := readFile(t, root, "governance/decisions/architecture.md")
	if !strings.Contains(d, "- **Status:** Superseded by [D-2](#d-2-second-decision)") {
		t.Errorf("old record not marked superseded; got:\n%s", d)
	}
	if !strings.Contains(d, "- **Supersedes:** [D-1](#d-1-first-decision)") {
		t.Errorf("new record does not claim the supersession; got:\n%s", d)
	}

	// And the parser must read it back, or the pair is decorative (T-119).
	after := recordsIn(t, root)
	if got := after["D-1"].Status; got != statusSuperseded {
		t.Errorf("D-1 after reparse = %q, want %q", got, statusSuperseded)
	}
	if got := after["D-2"].Status; got != statusActive {
		t.Errorf("D-2 after reparse = %q, want %q — the newer record stays live", got, statusActive)
	}
}

// Same-file supersession must not need a relative path: both records live in
// decisions/architecture.md, so the link is a bare anchor.
func TestCloseSupersedeLinkIsBareAnchorWithinAFile(t *testing.T) {
	root := writeDQR(t, twoQuestions, supersedable)
	recs := recordsIn(t, root)

	out, err := Close(root, recs["D-1"], ptr(recs["D-2"]))
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if strings.Contains(out.StatusLine, "../") || strings.Contains(out.StatusLine, ".md") {
		t.Errorf("status line = %q, want a bare #anchor for a same-file target", out.StatusLine)
	}
}

func TestCloseObsoleteTakesNoTarget(t *testing.T) {
	root := writeDQR(t, twoQuestions, twoDecisions)
	recs := recordsIn(t, root)

	out, err := Close(root, recs["Q-1"], nil)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if out.Relation != RelObsolete {
		t.Errorf("relation = %q, want %q", out.Relation, RelObsolete)
	}
	if out.TargetFile != "" || out.BackLink != "" {
		t.Errorf("obsolete close touched a target: %+v", out)
	}

	// The trap this guards: a status the parser does not recognise leaves the
	// record in the open count forever, which is the failure the verb exists
	// to fix arriving by another door.
	after := recordsIn(t, root)
	if got := after["Q-1"].Status; got != statusResolved {
		t.Errorf("obsolete Q-1 reparsed as %q, want %q — it must leave the open count", got, statusResolved)
	}
}
