package parser

import (
	"os"
	"strings"
	"testing"
)

const sampleMD = `# Architecture Decisions

---

### D-001: CLI-first, not MCP
- **Date:** 2026-04-20
- **Decision:** Claude talks to clide exclusively via Bash.
- **Cross-reference:** [D-005](architecture.md#d-005-dart-core)

### D-002: Feature-first folder layout
- **Date:** 2026-04-21
- **Decision:** Organise by feature, not by layer.

---

### D-003: [SUPERSEDED] Old approach
- **Date:** 2026-04-19
- **Superseded by:** [D-001](architecture.md#d-001)
- **Decision:** MCP-based integration.
`

func TestParseText_Records(t *testing.T) {
	records := parseText(sampleMD, "architecture", "decisions/architecture.md")
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}

	r := records[0]
	if r.ID != "D-001" {
		t.Errorf("ID = %q, want D-001", r.ID)
	}
	if r.Type != "confirmed" {
		t.Errorf("Type = %q, want confirmed", r.Type)
	}
	if r.Domain != "architecture" {
		t.Errorf("Domain = %q, want architecture", r.Domain)
	}
	if r.Date != "2026-04-20" {
		t.Errorf("Date = %q, want 2026-04-20", r.Date)
	}
	if r.Status != "active" {
		t.Errorf("Status = %q, want active", r.Status)
	}
	if len(r.Refs) != 1 || r.Refs[0].TargetID != "D-005" {
		t.Errorf("Refs = %+v, want [{D-005 references ...}]", r.Refs)
	}
}

func TestParseText_Superseded(t *testing.T) {
	records := parseText(sampleMD, "architecture", "decisions/architecture.md")
	r := records[2]
	if r.ID != "D-003" {
		t.Fatalf("expected D-003, got %s", r.ID)
	}
	if r.Status != "superseded" {
		t.Errorf("Status = %q, want superseded", r.Status)
	}
}

func TestParseText_Questions(t *testing.T) {
	md := `# Questions

### Q-001: How should we handle X?
- **Status:** Open
- **Question:** ...

### Q-002: What about Y?
- **Status:** Resolved → [D-005]
- **Question:** ...

### Q-003: Partially resolved Z
- **Status:** Partially resolved → [D-010]
- **Question:** ...
`
	records := parseText(md, "architecture", "decisions/questions-architecture.md")
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}

	if records[0].Status != "open" {
		t.Errorf("Q-001 status = %q, want open", records[0].Status)
	}
	if records[1].Status != "resolved" {
		t.Errorf("Q-002 status = %q, want resolved", records[1].Status)
	}
	if records[2].Status != "open" {
		t.Errorf("Q-003 status = %q, want open", records[2].Status)
	}
}

func TestParseText_Rejected(t *testing.T) {
	md := `# Rejected

### R-001: Old idea
- **Rejected:** 2026-04-19
- **Reason:** Too complex.
- **Cross-reference:** [D-001](architecture.md#d-001)
`
	records := parseText(md, "rejected", "decisions/rejected.md")
	if len(records) != 1 {
		t.Fatalf("got %d, want 1", len(records))
	}
	r := records[0]
	if r.Type != "rejected" {
		t.Errorf("Type = %q, want rejected", r.Type)
	}
	if r.Status != "active" {
		t.Errorf("Status = %q, want active", r.Status)
	}
	if r.Date != "2026-04-19" {
		t.Errorf("Date = %q, want 2026-04-19", r.Date)
	}
}

func TestDomainFromFilename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"architecture.md", "architecture"},
		{"questions-architecture.md", "architecture"},
		{"rejected.md", "rejected"},
		{"questions.md", "questions"},
	}
	for _, tt := range tests {
		if got := domainFromFilename(tt.name); got != tt.want {
			t.Errorf("domainFromFilename(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNextID(t *testing.T) {
	records := []Record{
		{ID: "D-001"}, {ID: "D-002"}, {ID: "D-010"},
		{ID: "Q-001"}, {ID: "Q-005"},
		{ID: "R-001"},
	}
	tests := []struct {
		prefix string
		want   string
	}{
		{"D", "D-011"},
		{"Q", "Q-006"},
		{"R", "R-002"},
	}
	for _, tt := range tests {
		if got := NextID(records, tt.prefix); got != tt.want {
			t.Errorf("NextID(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}

func TestValidate_DuplicateID(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", `### D-001: First
- **Date:** 2026-01-01
`)
	writeFile(t, dir, "b.md", `### D-001: Duplicate
- **Date:** 2026-01-02
`)
	ok, errs := Validate(dir, dir)
	if ok {
		t.Fatal("expected validation failure")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicate id D-001") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate ID error, got: %v", errs)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
