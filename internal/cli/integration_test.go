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

func TestIntegration_Backlinks_RequiresPathArg(t *testing.T) {
	vault := councilVault(t)
	_, _, code := run(t, vault, "backlinks")
	if code != 64 {
		t.Errorf("exit = %d, want 64 (Usage)", code)
	}
}

func TestIntegration_Backlinks_NoMatchExits2(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "backlinks", "members/nonexistent/file.md")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("stdout = %q, want []", got)
	}
}

func TestIntegration_Backlinks_FindsSessionReferences(t *testing.T) {
	// In the Council snapshot, the session outcome.md file links to multiple
	// council members via wikilinks. Backlinks for any persona file should
	// surface at least one hit (the session referencing it).
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "backlinks", "members/vaasa/persona.md")
	if code != 0 && code != 2 {
		t.Fatalf("exit=%d (want 0 or 2)\nstderr: %s", code, stderr)
	}
	if code == 2 {
		t.Skip("Council snapshot has no backlinks to vaasa — fixture may have been refreshed")
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	for i, r := range rows {
		path, _ := r["path"].(string)
		if path == "" {
			t.Errorf("row %d missing path: %#v", i, r)
		}
		if path == "members/vaasa/persona.md" {
			t.Errorf("row %d: self-reference should be excluded: %#v", i, r)
		}
		via, _ := r["via"].(string)
		if via != "wiki" && via != "embed" && via != "md" {
			t.Errorf("row %d: unexpected via %q", i, via)
		}
	}
}

func TestIntegration_Outlinks_RequiresPathArg(t *testing.T) {
	vault := councilVault(t)
	_, _, code := run(t, vault, "outlinks")
	if code != 64 {
		t.Errorf("exit = %d, want 64 (Usage)", code)
	}
}

func TestIntegration_Outlinks_UnknownFileExits2(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "outlinks", "nope/nope.md")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("stdout = %q, want []", got)
	}
}

func TestIntegration_Meta_RequiresPathArg(t *testing.T) {
	vault := councilVault(t)
	_, _, code := run(t, vault, "meta")
	if code != 64 {
		t.Errorf("exit = %d, want 64 (Usage)", code)
	}
}

func TestIntegration_Meta_UnknownFileExits66(t *testing.T) {
	vault := councilVault(t)
	_, _, code := run(t, vault, "meta", "ghost/never/seen.md")
	if code != 66 {
		t.Errorf("exit = %d, want 66 (NoInput)", code)
	}
}

func TestIntegration_Meta_VaasaPersona(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "meta", "members/vaasa/persona.md")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var m struct {
		Path        string                     `json:"path"`
		Name        string                     `json:"name"`
		Size        int64                      `json:"size"`
		Mtime       int64                      `json:"mtime"`
		Frontmatter map[string]json.RawMessage `json:"frontmatter"`
		Tags        []string                   `json:"tags"`
		Outlinks    []map[string]any           `json:"outlinks"`
		Headings    []map[string]any           `json:"headings"`
	}
	if err := json.Unmarshal(stdout, &m); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if m.Path != "members/vaasa/persona.md" {
		t.Errorf("path = %q", m.Path)
	}
	if m.Name != "persona" {
		t.Errorf("name = %q, want persona", m.Name)
	}
	if m.Size == 0 {
		t.Errorf("size should be set")
	}
	if len(m.Frontmatter) == 0 {
		t.Errorf("frontmatter should be non-empty")
	}
	// Verify raw JSON pass-through: the type field should decode as the
	// string "council-member", not as a wrapper object.
	if raw, ok := m.Frontmatter["type"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || s != "council-member" {
			t.Errorf("frontmatter[type] = %s (err=%v), want \"council-member\"", raw, err)
		}
	} else {
		t.Errorf("frontmatter[type] missing")
	}
}

func TestIntegration_Skill_StatusOnMissingExits2(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "status")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected ExitError, got %v\nstderr: %s", err, errBuf.String())
	}
	if ee.ExitCode() != 2 {
		t.Errorf("exit = %d, want 2 (missing == no-match)", ee.ExitCode())
	}
	var st map[string]any
	if err := json.Unmarshal(outBuf.Bytes(), &st); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, outBuf.String())
	}
	if st["state"] != "missing" {
		t.Errorf("state = %v, want missing", st["state"])
	}
}

func TestIntegration_Skill_InstallIsIdempotent(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")

	runSkill := func(args ...string) (int, []byte) {
		full := append([]string{"--vault", vault, "--db", dbPath}, args...)
		cmd := exec.Command(pqlBin, full...)
		out, err := cmd.Output()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), out
		}
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}
		return 0, out
	}

	if code, _ := runSkill("skill", "install"); code != 0 {
		t.Errorf("first install exit = %d, want 0", code)
	}
	code, out := runSkill("skill", "status")
	if code != 0 {
		t.Errorf("status after install exit = %d, want 0", code)
	}
	var st map[string]any
	_ = json.Unmarshal(out, &st)
	if st["state"] != "current" {
		t.Errorf("state after install = %v, want current", st["state"])
	}
	// Second install on a current state → still 0, still current.
	if code, _ := runSkill("skill", "install"); code != 0 {
		t.Errorf("idempotent install exit = %d, want 0", code)
	}

	// Verify the files landed at the documented path.
	if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", "pql", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not at documented path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", "pql", ".pql-install.json")); err != nil {
		t.Errorf("lock file not at documented path: %v", err)
	}
}

func TestIntegration_Skill_InstallRefusesModifiedWithoutForce(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")

	if err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "install").Run(); err != nil {
		t.Fatalf("seed install: %v", err)
	}
	skillFile := filepath.Join(vault, ".claude", "skills", "pql", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("hand edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}

	cmd := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "install")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected refusal, got %v", err)
	}
	if ee.ExitCode() != 64 {
		t.Errorf("exit = %d, want 64 (Usage)", ee.ExitCode())
	}
	if !bytes.Contains(errBuf.Bytes(), []byte("modified")) {
		t.Errorf("stderr should mention modified state: %s", errBuf.String())
	}
	// File should be unchanged.
	body, _ := os.ReadFile(skillFile)
	if string(body) != "hand edited\n" {
		t.Errorf("file was overwritten despite refusal: %q", body)
	}

	// With --force it succeeds.
	if err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "install", "--force").Run(); err != nil {
		t.Errorf("--force install failed: %v", err)
	}
}

func TestIntegration_Skill_UninstallRemovesFiles(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	if err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "install").Run(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "uninstall")
	out, err := cmd.Output()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected ExitError (state=missing post-uninstall), got %v", err)
	}
	if ee.ExitCode() != 2 {
		t.Errorf("exit = %d, want 2 (missing after uninstall)", ee.ExitCode())
	}
	var st map[string]any
	_ = json.Unmarshal(out, &st)
	if st["state"] != "missing" {
		t.Errorf("state = %v, want missing", st["state"])
	}
	if _, err := os.Stat(filepath.Join(vault, ".claude", "skills", "pql", "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("SKILL.md still exists after uninstall: %v", err)
	}
}

func TestIntegration_Query_PositionalDSL(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "query", "SELECT path WHERE folder = 'members/vaasa' ORDER BY path")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one members/vaasa file, got zero")
	}
	for i, r := range rows {
		p, _ := r["path"].(string)
		if !strings.HasPrefix(p, "members/vaasa/") {
			t.Errorf("row %d path = %q, want members/vaasa/* prefix", i, p)
		}
	}
}

func TestIntegration_Query_TagMembership(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "query", "SELECT path WHERE 'volt' IN tags")
	if code != 0 && code != 2 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	if code == 2 {
		t.Skip("no 'volt' tag in current fixture")
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Errorf("expected at least one file tagged 'volt'")
	}
}

func TestIntegration_Query_FmAccess(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "query", "SELECT path, fm.type WHERE fm.type = 'council-member' ORDER BY path")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one council-member")
	}
	for i, r := range rows {
		if r["fm.type"] != "council-member" {
			t.Errorf("row %d fm.type = %v", i, r["fm.type"])
		}
	}
}

func TestIntegration_Query_ParseErrorExits65(t *testing.T) {
	vault := councilVault(t)
	_, stderr, code := run(t, vault, "query", "SELECT FROM WHERE")
	if code != 65 {
		t.Errorf("exit = %d, want 65 (DataErr); stderr=%s", code, stderr)
	}
	if !bytes.Contains(stderr, []byte("pql.")) {
		t.Errorf("stderr should carry pql.* diagnostic code: %s", stderr)
	}
}

func TestIntegration_Query_UnknownColumnExits65(t *testing.T) {
	vault := councilVault(t)
	_, stderr, code := run(t, vault, "query", "SELECT typo_col")
	if code != 65 {
		t.Errorf("exit = %d, want 65 (DataErr)", code)
	}
	if !bytes.Contains(stderr, []byte("unknown_column")) {
		t.Errorf("stderr should mention unknown_column: %s", stderr)
	}
}

func TestIntegration_Query_NoInputModeExits64(t *testing.T) {
	vault := councilVault(t)
	_, _, code := run(t, vault, "query")
	if code != 64 {
		t.Errorf("exit = %d, want 64 (Usage)", code)
	}
}

func TestIntegration_Query_FromFile(t *testing.T) {
	vault := councilVault(t)
	qfile := filepath.Join(t.TempDir(), "q.pql")
	if err := os.WriteFile(qfile, []byte("SELECT path WHERE folder = 'members/holt'"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, stderr, code := run(t, vault, "query", "--file", qfile)
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, r := range rows {
		p, _ := r["path"].(string)
		if !strings.HasPrefix(p, "members/holt/") {
			t.Errorf("--file query returned unexpected path %q", p)
		}
	}
}

func TestIntegration_Query_NoMatchExits2(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "query", "SELECT path WHERE folder = 'nope-never'")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("stdout = %q, want []", got)
	}
}

func TestIntegration_Doctor_FreshVaultBeforeIndex(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "doctor")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pql doctor: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	// DB shouldn't exist yet — index field should be nil/absent.
	db, _ := rep["db"].(map[string]any)
	if exists, _ := db["exists"].(bool); exists {
		t.Errorf("db.exists = true on fresh dir, want false")
	}
	if rep["index"] != nil {
		t.Errorf("index should be null when DB doesn't exist, got %v", rep["index"])
	}
	// Vault should still be reported.
	v, _ := rep["vault"].(map[string]any)
	if path, _ := v["path"].(string); path == "" {
		t.Errorf("vault.path should be set, got %#v", v)
	}
}

func TestIntegration_Doctor_PopulatedAfterIndex(t *testing.T) {
	vault := councilVault(t)
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	// First, run a query to materialise the index.
	if err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "files", "--limit", "1").Run(); err != nil {
		t.Fatalf("warm up index: %v", err)
	}
	// Now doctor should report a populated DB.
	out, err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "doctor").Output()
	if err != nil {
		t.Fatalf("pql doctor: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	db, _ := rep["db"].(map[string]any)
	if exists, _ := db["exists"].(bool); !exists {
		t.Errorf("db.exists should be true after index run")
	}
	if size, _ := db["size_bytes"].(float64); size <= 0 {
		t.Errorf("db.size_bytes should be > 0, got %v", size)
	}
	if v, _ := db["schema_version"].(float64); v < 1 {
		t.Errorf("db.schema_version should be ≥1, got %v", v)
	}
	idx, _ := rep["index"].(map[string]any)
	if idx == nil {
		t.Fatalf("index should be populated, got nil")
	}
	if files, _ := idx["files"].(float64); files < 30 {
		t.Errorf("index.files = %v, want ≥30 (Council snapshot)", files)
	}
}

func TestIntegration_Doctor_SkillFieldReportsState(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	// Fresh vault: skill should be missing.
	out, err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "doctor").Output()
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	var rep struct {
		Skill struct {
			Project struct {
				State string `json:"state"`
			} `json:"project"`
			EmbeddedHash    string `json:"embedded_hash"`
			EmbeddedVersion string `json:"embedded_version"`
		} `json:"skill"`
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rep.Skill.Project.State != "missing" {
		t.Errorf("project.state = %q, want missing", rep.Skill.Project.State)
	}
	if rep.Skill.EmbeddedHash == "" {
		t.Errorf("embedded_hash should be set")
	}

	// After installing, doctor should report current.
	if err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "skill", "install").Run(); err != nil {
		t.Fatalf("skill install: %v", err)
	}
	out, err = exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "doctor").Output()
	if err != nil {
		t.Fatalf("doctor (post-install): %v", err)
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rep.Skill.Project.State != "current" {
		t.Errorf("project.state after install = %q, want current", rep.Skill.Project.State)
	}
}

func TestIntegration_Doctor_VersionMatchesBinary(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	out, err := exec.Command(pqlBin, "--vault", vault, "--db", dbPath, "doctor").Output()
	if err != nil {
		t.Fatalf("pql doctor: %v", err)
	}
	var rep struct {
		Version struct {
			SchemaVersion int    `json:"schema_version"`
			GoVersion     string `json:"go_version"`
		} `json:"version"`
	}
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rep.Version.SchemaVersion < 1 {
		t.Errorf("version.schema_version = %d, want ≥1", rep.Version.SchemaVersion)
	}
	if rep.Version.GoVersion == "" {
		t.Errorf("version.go_version should be set")
	}
}

func TestIntegration_Init_FreshDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "init")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pql init: %v", err)
	}
	var result struct {
		Directory string `json:"directory"`
		Config    struct {
			Path        string `json:"path"`
			Created     bool   `json:"created"`
			Overwritten bool   `json:"overwritten"`
		} `json:"config"`
		Gitignore struct {
			Exists   bool `json:"exists"`
			Appended bool `json:"appended"`
		} `json:"gitignore"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !result.Config.Created || result.Config.Overwritten {
		t.Errorf("config = %#v, want Created=true", result.Config)
	}
	// Verify the file actually exists.
	if _, err := os.Stat(filepath.Join(dir, ".pql.yaml")); err != nil {
		t.Errorf(".pql.yaml not created: %v", err)
	}
	if result.Gitignore.Exists {
		t.Errorf("gitignore.Exists should be false in fresh dir")
	}
}

func TestIntegration_Init_IsIdempotentOnExistingConfig(t *testing.T) {
	dir := t.TempDir()
	original := []byte("frontmatter: toml\n")
	if err := os.WriteFile(filepath.Join(dir, ".pql.yaml"), original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	// Skip the skill prompt; we're testing config behaviour.
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "init", "--with-skill=no")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Config struct {
			Skipped     bool `json:"skipped"`
			Created     bool `json:"created"`
			Overwritten bool `json:"overwritten"`
		} `json:"config"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !result.Config.Skipped || result.Config.Created || result.Config.Overwritten {
		t.Errorf("config sub-stat = %#v, want Skipped=true (others false)", result.Config)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".pql.yaml"))
	if !bytes.Equal(body, original) {
		t.Errorf("existing config was modified: %q", body)
	}
}

func TestIntegration_Init_WithSkillYesInstalls(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "init", "--with-skill=yes")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Skill struct {
			Mode  string `json:"mode"`
			State string `json:"state"`
		} `json:"skill"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if result.Skill.State != "current" {
		t.Errorf("skill.state = %q, want current", result.Skill.State)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "pql", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not installed: %v", err)
	}
}

func TestIntegration_Init_WithSkillNoSkips(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "init", "--with-skill=no")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Skill struct {
			Mode  string `json:"mode"`
			State string `json:"state"`
		} `json:"skill"`
	}
	_ = json.Unmarshal(out, &result)
	if result.Skill.Mode != "no" {
		t.Errorf("skill.mode = %q, want no", result.Skill.Mode)
	}
	if result.Skill.State != "missing" {
		t.Errorf("skill.state = %q, want missing", result.Skill.State)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "pql", "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("SKILL.md should not exist after --with-skill=no: %v", err)
	}
}

func TestIntegration_Init_WithSkillPromptSkipsWithoutTTY(t *testing.T) {
	// In integration tests stdin is a pipe (not a TTY), so prompt mode
	// should defer cleanly without hanging.
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "init", "--with-skill=prompt")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	var result struct {
		Skill struct {
			Mode string `json:"mode"`
			Note string `json:"note"`
		} `json:"skill"`
	}
	_ = json.Unmarshal(out, &result)
	if result.Skill.Mode != "prompt-skipped-no-tty" {
		t.Errorf("skill.mode = %q, want prompt-skipped-no-tty", result.Skill.Mode)
	}
}

func TestIntegration_Init_AppendsToExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("seed gitignore: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := exec.Command(pqlBin, "--vault", dir, "--db", dbPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("pql init: %v", err)
	}
	body, _ := os.ReadFile(gi)
	if !strings.Contains(string(body), ".pql/") {
		t.Errorf(".pql/ not appended to gitignore: %s", body)
	}
}

func TestIntegration_Schema_CouncilSnapshot(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "schema")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) == 0 {
		t.Fatal("expected non-empty schema from Council snapshot")
	}
	// Spot-check shape on each row.
	for i, r := range rows {
		key, _ := r["key"].(string)
		if key == "" {
			t.Errorf("row %d missing key: %#v", i, r)
		}
		types, _ := r["types"].([]any)
		if len(types) == 0 {
			t.Errorf("row %d (key=%q): types empty", i, key)
		}
		count, _ := r["count"].(float64)
		if count < 1 {
			t.Errorf("row %d (key=%q): count = %v", i, key, count)
		}
	}
	// Spot-check known key: type appears on every council-member persona,
	// always as a string.
	for _, r := range rows {
		if r["key"] == "type" {
			types, _ := r["types"].([]any)
			if len(types) != 1 || types[0] != "string" {
				t.Errorf("schema for 'type' = %v, want [\"string\"]", types)
			}
		}
	}
}

func TestIntegration_Schema_SortByCount(t *testing.T) {
	vault := councilVault(t)
	stdout, stderr, code := run(t, vault, "schema", "--sort", "count")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
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

func TestIntegration_Outlinks_OnSessionOutcome(t *testing.T) {
	// The Council session outcome.md is a known-link-rich file.
	vault := councilVault(t)
	// Discover the actual session outcome path (only one session in fixture).
	stdout, stderr, code := run(t, vault, "files", "sessions/*/outcome.md")
	if code != 0 {
		t.Fatalf("locate outcome: exit=%d stderr=%s", code, stderr)
	}
	var files []map[string]any
	if err := json.Unmarshal(stdout, &files); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no session outcome.md in fixture")
	}
	target, _ := files[0]["path"].(string)

	stdout, stderr, code = run(t, vault, "outlinks", target)
	if code != 0 && code != 2 {
		t.Fatalf("outlinks: exit=%d stderr=%s", code, stderr)
	}
	if code == 2 {
		t.Skipf("session outcome %s has no outlinks in this fixture", target)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for i, r := range rows {
		t1, _ := r["target"].(string)
		if t1 == "" {
			t.Errorf("row %d missing target: %#v", i, r)
		}
		via, _ := r["via"].(string)
		if via != "wiki" && via != "embed" && via != "md" {
			t.Errorf("row %d: unexpected via %q", i, via)
		}
	}
}
