package parser

import (
	"os"
	"testing"
)

func TestParseAll_ClideDecisions(t *testing.T) {
	const decisionsDir = "/var/mnt/data/projects/clide/decisions"
	const repoRoot = "/var/mnt/data/projects/clide"

	if _, err := os.Stat(decisionsDir); err != nil {
		t.Skip("clide decisions dir not available")
	}

	records, warnings, err := ParseAll(decisionsDir, repoRoot)
	if err != nil {
		t.Fatalf("ParseAll: %v", err)
	}
	t.Logf("records: %d, warnings: %d", len(records), len(warnings))
	for _, w := range warnings {
		t.Logf("  warn: %s", w)
	}

	types := map[string]int{}
	for _, r := range records {
		types[r.Type]++
	}
	t.Logf("confirmed=%d question=%d rejected=%d",
		types["confirmed"], types["question"], types["rejected"])

	if types["confirmed"] < 30 {
		t.Errorf("expected >= 30 confirmed decisions, got %d", types["confirmed"])
	}
}
