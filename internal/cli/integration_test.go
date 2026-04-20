//go:build integration

// Integration tests build the pql binary fresh in TestMain and shell out
// to it against testdata/. They exercise the full output contract — JSON
// shape on stdout, JSON-per-line diagnostics on stderr, distinguished
// exit codes — that the rest of the test suite can't validate.
//
// Run with: go test -tags=integration ./internal/cli/...
// Or via:   make test-integration
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var pqlBin string // set in TestMain

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "pql-integration-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: tempdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	pqlBin = filepath.Join(tmp, "pql")
	build := exec.Command("go", "build", "-o", pqlBin, "../../cmd/pql")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: build: %v\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// run invokes pqlBin with the given args and an out-of-vault --db so the
// test fixture stays clean. Returns stdout, stderr, and the exit code.
func run(t *testing.T, vault string, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "integration.sqlite")
	full := append([]string{"--vault", vault, "--db", dbPath}, args...)
	cmd := exec.Command(pqlBin, full...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		exitCode = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("invoke pql: %v\nstderr: %s", err, errBuf.String())
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

func councilVault(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		c := filepath.Join(dir, "testdata", "council-snapshot")
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate testdata/council-snapshot from %s", wd)
		}
		dir = parent
	}
}

// --- tests ----------------------------------------------------------------

func TestIntegration_Version(t *testing.T) {
	cmd := exec.Command(pqlBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pql --version: %v", err)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		t.Errorf("expected version string, got empty")
	}
}

func TestIntegration_VersionBuildInfo(t *testing.T) {
	cmd := exec.Command(pqlBin, "version", "--build-info")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pql version --build-info: %v", err)
	}
	var info struct {
		Version       string `json:"version"`
		Commit        string `json:"commit"`
		Date          string `json:"date"`
		GoVersion     string `json:"go_version"`
		SchemaVersion int    `json:"schema_version"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if info.SchemaVersion < 1 {
		t.Errorf("schema_version should be ≥1, got %d", info.SchemaVersion)
	}
	if info.GoVersion == "" {
		t.Errorf("go_version should be set")
	}
}

func TestIntegration_Files_CouncilSnapshot(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "files")
	if code != 0 {
		t.Fatalf("exit=%d (expected 0)\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) < 30 {
		t.Errorf("expected ≥30 files, got %d", len(rows))
	}
	// Spot-check shape on the first row.
	if first := rows[0]; first["path"] == nil || first["name"] == nil {
		t.Errorf("first row missing path/name fields: %#v", first)
	}
}

func TestIntegration_Files_GlobNarrowsResults(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "files", "members/vaasa/*")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	for _, r := range rows {
		p, _ := r["path"].(string)
		if !strings.HasPrefix(p, "members/vaasa/") {
			t.Errorf("path %q outside requested glob", p)
		}
	}
	if len(rows) == 0 {
		t.Errorf("expected at least one members/vaasa/* match")
	}
}

func TestIntegration_Files_NoMatchExits2(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "files", "nope/no-such-folder/*")
	if code != 2 {
		t.Errorf("exit = %d, want 2 (zero matches)", code)
	}
	// JSON path emits "[]" when zero rows; JSONL emits nothing. Default is JSON.
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("stdout = %q, want []", got)
	}
}

func TestIntegration_Files_JSONLFormat(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "--jsonl", "--limit", "3", "files")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	lines := bytes.Split(bytes.TrimRight(stdout, "\n"), []byte("\n"))
	if len(lines) != 3 {
		t.Errorf("expected 3 jsonl lines, got %d (output: %q)", len(lines), stdout)
	}
	for i, line := range lines {
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			t.Errorf("line %d not valid JSON: %v (%q)", i, err, line)
		}
	}
}

func TestIntegration_Files_PrettyAndJSONLMutuallyExclusive(t *testing.T) {
	vault := councilVault(t)
	_, stderr, code := run(t, vault, "--pretty", "--jsonl", "files")
	if code != 64 {
		t.Errorf("exit = %d, want 64 (Usage)", code)
	}
	if !bytes.Contains(stderr, []byte("mutually exclusive")) {
		t.Errorf("stderr should mention mutual exclusion: %s", stderr)
	}
}

func TestIntegration_VaultNotFoundExits66(t *testing.T) {
	stdout, stderr, code := run(t, "/nonexistent/vault/path", "files")
	if code != 66 {
		t.Errorf("exit = %d, want 66 (NoInput); stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestIntegration_Tags_CouncilSnapshot(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "tags")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) == 0 {
		t.Fatalf("expected non-empty tag list from Council snapshot")
	}
	// Verify shape on every row: tag is a non-empty string, count is a
	// positive integer. Avoids hardcoding which specific tags happen to be
	// in the fixture (the snapshot can be refreshed).
	for i, r := range rows {
		tag, _ := r["tag"].(string)
		if tag == "" {
			t.Errorf("row %d: tag is empty (%#v)", i, r)
		}
		count, _ := r["count"].(float64)
		if count < 1 {
			t.Errorf("row %d (tag=%q): count = %v, expected ≥1", i, tag, count)
		}
	}
}

func TestIntegration_Tags_SortByCount(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "tags", "--sort", "count")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) < 2 {
		t.Skipf("need ≥2 tags to verify sort order; got %d", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		prev, _ := rows[i-1]["count"].(float64)
		cur, _ := rows[i]["count"].(float64)
		if cur > prev {
			t.Errorf("counts not descending at index %d: %v then %v", i, prev, cur)
			break
		}
	}
}

func TestIntegration_Tags_InvalidSortRejected(t *testing.T) {
	vault := councilVault(t)
	_, stderr, code := run(t, vault, "tags", "--sort", "garbage")
	// runQuery wraps the primitive error into Software (70) since the
	// primitive itself returns the validation error.
	if code != 70 {
		t.Errorf("exit = %d, want 70; stderr=%s", code, stderr)
	}
}
