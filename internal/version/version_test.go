package version

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// project.yaml is the file a human reads to answer "what version of what does
// this project claim to be", and the constants in this package are what the
// binary actually acts on. They used to be kept in step by a comment asking
// whoever bumped one to remember the other. This test is that comment, enforced:
// a mismatch is a build failure rather than a consumer discovering the drift.
func TestDeclaredVersionsMatchProjectYAML(t *testing.T) {
	declared := parseProjectYAML(t)

	for _, tc := range []struct {
		key  string
		got  string
		what string
	}{
		{"schema_version", strconv.Itoa(SchemaVersion), "index.db schema"},
		{"canonical_version", strconv.Itoa(CanonicalVersion), "pql.db row canonicalisation"},
		{"planning_schema_version", PlanningSchemaVersion, "pql.db schema"},
		{"changelog_format", ChangelogFormat, ".pql/changelog/ file format"},
	} {
		want, ok := declared[tc.key]
		if !ok {
			t.Errorf("project.yaml declares no %s — every version axis must be declared there (%s)",
				tc.key, tc.what)
			continue
		}
		if want != tc.got {
			t.Errorf("%s: project.yaml says %s, internal/version says %s — bump both, they describe the same thing",
				tc.key, want, tc.got)
		}
	}
}

// The two migrated axes carry a release, not a counter. A bare number here
// would mean someone reverted the scheme without updating its rationale.
func TestMigratedAxesCarryAReleaseVersion(t *testing.T) {
	for name, v := range map[string]string{
		"PlanningSchemaVersion": PlanningSchemaVersion,
		"ChangelogFormat":       ChangelogFormat,
		"PreVersionedChangelog": PreVersionedChangelog,
	} {
		if strings.Count(v, ".") != 2 {
			t.Errorf("%s = %q, want a MAJOR.MINOR.PATCH release version", name, v)
		}
	}
}

// parseProjectYAML pulls the scalar top-level keys out of project.yaml, with
// surrounding quotes stripped. Deliberately not a YAML library: this is a drift
// guard, and it should not be able to fail for reasons unrelated to drift.
func parseProjectYAML(t *testing.T) map[string]string {
	t.Helper()
	path := findProjectYAML(t)
	body, err := os.ReadFile(path) //nolint:gosec // G304: path resolved by walking up from the test's own directory
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	out := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
			continue // nested key or comment
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return out
}

func findProjectYAML(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "project.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate project.yaml above the test directory")
		}
		dir = parent
	}
}

func TestInfoReportsEveryAxis(t *testing.T) {
	info := Info()
	if info.SchemaVersion != SchemaVersion ||
		info.PlanningSchemaVersion != PlanningSchemaVersion ||
		info.CanonicalVersion != CanonicalVersion ||
		info.ChangelogFormat != ChangelogFormat {
		t.Errorf("build info %+v does not carry every version axis", info)
	}
}
