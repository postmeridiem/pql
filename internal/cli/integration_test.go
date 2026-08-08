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
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
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
	if runtime.GOOS == "windows" {
		pqlBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", pqlBin, "../../cmd/pql")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: build: %v\n%s\n", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// sandboxHomes holds one fake HOME per test, so repeated invocations inside a
// single test see each other's writes (install → status) while separate tests
// stay isolated.
var sandboxHomes sync.Map // test name → home dir

// pqlCmd builds an *exec.Cmd for the integration binary with a sandboxed
// environment. Every invocation in this suite goes through it.
//
// Skill scope auto-resolves to *user* scope whenever a bundled skill is already
// installed under $HOME — true on any developer machine, never in CI. With the
// real HOME inherited, six tests failed locally for reasons unrelated to the
// code under test, and the run rewrote the developer's ~/.claude/skills/ with
// the test binary's embedded copy (T-57). Redirecting HOME, the XDG dirs, and
// any ambient PQL_* overrides makes the suite hermetic, so it gates locally
// exactly as it does in CI.
func pqlCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(pqlBin, args...)
	cmd.Env = sandboxEnv(t)
	return cmd
}

func sandboxEnv(t *testing.T) []string {
	t.Helper()
	stored, ok := sandboxHomes.Load(t.Name())
	if !ok {
		stored = t.TempDir()
		sandboxHomes.Store(t.Name(), stored)
		t.Cleanup(func() { sandboxHomes.Delete(t.Name()) })
	}
	home := stored.(string)

	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch {
		case key == "HOME", key == "USERPROFILE",
			key == "XDG_CONFIG_HOME", key == "XDG_CACHE_HOME",
			strings.HasPrefix(key, "PQL_"):
			continue // replaced below, or dropped so the dev shell cannot leak in
		}
		env = append(env, kv)
	}
	return append(env,
		"HOME="+home,
		"USERPROFILE="+home, // os.UserHomeDir reads this one on Windows
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
	)
}

// run invokes pqlBin with the given args and an out-of-vault --db so the
// test fixture stays clean. Returns stdout, stderr, and the exit code.
func run(t *testing.T, vault string, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "integration.sqlite")
	full := append([]string{"--vault", vault, "--db", dbPath}, args...)
	cmd := pqlCmd(t, full...)
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

// skillStates decodes the []skill.Status array emitted by the skill
// subcommands (one entry per bundled skill) and returns state keyed by
// skill name. Decodes into an independent struct on purpose: the JSON
// keys are the output contract, not the source structs.
func skillStates(t *testing.T, out []byte) map[string]string {
	t.Helper()
	var statuses []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &statuses); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	states := make(map[string]string, len(statuses))
	for _, st := range statuses {
		states[st.Name] = st.State
	}
	return states
}

// initSkillStats decodes the per-skill stats array from `pql init`
// output, keyed by skill name. Fails the test if no entries decode, so
// a renamed key can't pass as a vacuous loop.
func initSkillStats(t *testing.T, out []byte) map[string]struct{ Mode, State string } {
	t.Helper()
	var result struct {
		Skills []struct {
			Name  string `json:"name"`
			Mode  string `json:"mode"`
			State string `json:"state"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(result.Skills) == 0 {
		t.Fatalf("no skills array in init output:\n%s", out)
	}
	stats := make(map[string]struct{ Mode, State string }, len(result.Skills))
	for _, s := range result.Skills {
		stats[s.Name] = struct{ Mode, State string }{s.Mode, s.State}
	}
	return stats
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
	cmd := pqlCmd(t, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pql --version: %v", err)
	}
	if len(bytes.TrimSpace(out)) == 0 {
		t.Errorf("expected version string, got empty")
	}
}

func TestIntegration_VersionBuildInfo(t *testing.T) {
	cmd := pqlCmd(t, "version", "--build-info")
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

func TestIntegration_Files_NoMatchExits0(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "files", "nope/no-such-folder/*")
	if code != 0 {
		t.Errorf("exit = %d, want 0 (zero matches is success)", code)
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

func TestIntegration_Backlinks_NoMatchExits0(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "backlinks", "members/nonexistent/file.md")
	if code != 0 {
		t.Errorf("exit = %d, want 0 (zero matches is success)", code)
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
	if code != 0 {
		t.Fatalf("exit=%d (want 0)\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) == 0 {
		t.Skip("Council snapshot has no backlinks to vaasa — fixture may have been refreshed")
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

func TestIntegration_Outlinks_UnknownFileExits0(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "outlinks", "nope/nope.md")
	if code != 0 {
		t.Errorf("exit = %d, want 0 (zero matches is success)", code)
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

func TestIntegration_Skill_StatusOnMissingExits0(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "status")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("exit = %v, want 0 (missing state reported in data)\nstderr: %s", err, errBuf.String())
	}
	states := skillStates(t, outBuf.Bytes())
	if len(states) < 2 {
		t.Errorf("got %d bundled skills, want at least 2 (pql + clean-house)", len(states))
	}
	for _, name := range []string{"pql", "clean-house"} {
		if states[name] != "missing" {
			t.Errorf("%s state = %q, want missing", name, states[name])
		}
	}
}

func TestIntegration_Skill_InstallIsIdempotent(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")

	runSkill := func(args ...string) (int, []byte) {
		full := append([]string{"--vault", vault, "--db", dbPath}, args...)
		cmd := pqlCmd(t, full...)
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
	if states := skillStates(t, out); states["pql"] != "current" {
		t.Errorf("pql state after install = %q, want current", states["pql"])
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

	if err := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "install").Run(); err != nil {
		t.Fatalf("seed install: %v", err)
	}
	skillFile := filepath.Join(vault, ".claude", "skills", "pql", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("hand edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}

	cmd := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "install")
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
	if err := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "install", "--force").Run(); err != nil {
		t.Errorf("--force install failed: %v", err)
	}
}

func TestIntegration_Skill_UninstallRemovesFiles(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	if err := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "install").Run(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "uninstall")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("exit = %v, want 0 (state=missing reported in data post-uninstall)", err)
	}
	if states := skillStates(t, out); states["pql"] != "missing" {
		t.Errorf("pql state = %q, want missing", states["pql"])
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
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Skip("no 'volt' tag in current fixture")
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

func TestIntegration_Query_NoMatchExits0(t *testing.T) {
	vault := councilVault(t)
	stdout, _, code := run(t, vault, "query", "SELECT path WHERE folder = 'nope-never'")
	if code != 0 {
		t.Errorf("exit = %d, want 0 (zero matches is success)", code)
	}
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("stdout = %q, want []", got)
	}
}

func TestIntegration_Doctor_FreshVaultBeforeIndex(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "doctor")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pql doctor: %v", err)
	}
	var rep map[string]any
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	// DB shouldn't exist yet, so there are no row counts to report. The key
	// must be *absent*, not null (T-75): the output contract says empty
	// fields are omitted, so a caller presence-checks the key — and `!= nil`
	// alone cannot tell the two apart, which is how `"index": null` survived
	// this test in the first place. Check the key.
	db, _ := rep["db"].(map[string]any)
	if exists, _ := db["exists"].(bool); exists {
		t.Errorf("db.exists = true on fresh dir, want false")
	}
	if v, ok := rep["index"]; ok {
		t.Errorf("index key should be absent when there is no DB, got %v", v)
	}
	if !strings.Contains(string(out), `"db"`) {
		t.Fatalf("sanity: report does not look like a doctor report:\n%s", out)
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
	if err := pqlCmd(t, "--vault", vault, "--db", dbPath, "files", "--limit", "1").Run(); err != nil {
		t.Fatalf("warm up index: %v", err)
	}
	// Now doctor should report a populated DB.
	out, err := pqlCmd(t, "--vault", vault, "--db", dbPath, "doctor").Output()
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
	// pqlEntry runs doctor and returns the "pql" entry from the
	// per-skill skills array.
	pqlEntry := func() (projectState, embeddedHash string) {
		t.Helper()
		out, err := pqlCmd(t, "--vault", vault, "--db", dbPath, "doctor").Output()
		if err != nil {
			t.Fatalf("doctor: %v", err)
		}
		var rep struct {
			Skills []struct {
				Name    string `json:"name"`
				Project *struct {
					State string `json:"state"`
				} `json:"project"`
				EmbeddedHash string `json:"embedded_hash"`
			} `json:"skills"`
		}
		if err := json.Unmarshal(out, &rep); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		for _, s := range rep.Skills {
			if s.Name != "pql" {
				continue
			}
			if s.Project != nil {
				projectState = s.Project.State
			}
			return projectState, s.EmbeddedHash
		}
		t.Fatalf("no pql entry in doctor skills array:\n%s", out)
		return "", ""
	}

	// Fresh vault: skill should be missing.
	state, hash := pqlEntry()
	if state != "missing" {
		t.Errorf("project.state = %q, want missing", state)
	}
	if hash == "" {
		t.Errorf("embedded_hash should be set")
	}

	// After installing, doctor should report current.
	if err := pqlCmd(t, "--vault", vault, "--db", dbPath, "skill", "install").Run(); err != nil {
		t.Fatalf("skill install: %v", err)
	}
	if state, _ := pqlEntry(); state != "current" {
		t.Errorf("project.state after install = %q, want current", state)
	}
}

func TestIntegration_Doctor_VersionMatchesBinary(t *testing.T) {
	vault := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	out, err := pqlCmd(t, "--vault", vault, "--db", dbPath, "doctor").Output()
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
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "init")
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
	// Verify the file actually exists at the new in-.pql/ location.
	if _, err := os.Stat(filepath.Join(dir, ".pql", "config.yaml")); err != nil {
		t.Errorf(".pql/config.yaml not created: %v", err)
	}
	if result.Gitignore.Exists {
		t.Errorf("gitignore.Exists should be false in fresh dir")
	}
}

func TestIntegration_Init_IsIdempotentOnExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pql", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := []byte("frontmatter: toml\n")
	if err := os.WriteFile(configPath, original, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	// Skip the skill prompt; we're testing config behaviour.
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "init", "--with-skill=no")
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
	body, _ := os.ReadFile(configPath)
	if !bytes.Equal(body, original) {
		t.Errorf("existing config was modified: %q", body)
	}
}

func TestIntegration_Init_WithSkillYesInstalls(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "init", "--with-skill=yes")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	skills := initSkillStats(t, out)
	for name, st := range skills {
		if st.State != "current" {
			t.Errorf("skills[%s].state = %q, want current", name, st.State)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "pql", "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not installed: %v", err)
	}
}

func TestIntegration_Init_WithSkillNoSkips(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "init", "--with-skill=no")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for name, st := range initSkillStats(t, out) {
		if st.Mode != "no" {
			t.Errorf("skills[%s].mode = %q, want no", name, st.Mode)
		}
		if st.State != "missing" {
			t.Errorf("skills[%s].state = %q, want missing", name, st.State)
		}
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
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "init", "--with-skill=prompt")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for name, st := range initSkillStats(t, out) {
		if st.Mode != "prompt-skipped-no-tty" {
			t.Errorf("skills[%s].mode = %q, want prompt-skipped-no-tty", name, st.Mode)
		}
	}
}

func TestIntegration_Init_AppendsToExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("seed gitignore: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "pql.sqlite")
	cmd := pqlCmd(t, "--vault", dir, "--db", dbPath, "init")
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
	if code != 0 {
		t.Fatalf("outlinks: exit=%d stderr=%s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rows) == 0 {
		t.Skipf("session outcome %s has no outlinks in this fixture", target)
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

// --- ticket statuslist (configurable status vocabulary, D-24) -------------

func TestIntegration_StatusList_Default(t *testing.T) {
	vault := t.TempDir()
	stdout, stderr, code := run(t, vault, "ticket", "statuslist")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	wantNames := []string{"backlog", "ready", "in_progress", "review", "done", "cancelled"}
	if len(rows) != len(wantNames) {
		t.Fatalf("got %d statuses, want %d: %s", len(rows), len(wantNames), stdout)
	}
	for i, want := range wantNames {
		if name, _ := rows[i]["name"].(string); name != want {
			t.Errorf("row %d name = %q, want %q", i, name, want)
		}
		if order, _ := rows[i]["order"].(float64); int(order) != i {
			t.Errorf("row %d order = %v, want %d", i, rows[i]["order"], i)
		}
	}
	// backlog: initial + default; done: terminal.
	if rows[0]["class"] != "initial" || rows[0]["is_default"] != true {
		t.Errorf("backlog row = %#v, want initial/default", rows[0])
	}
	if rows[4]["class"] != "done" && rows[4]["is_terminal"] != true {
		t.Errorf("done row = %#v, want is_terminal", rows[4])
	}
	if lbl, _ := rows[2]["label"].(string); lbl != "In Progress" {
		t.Errorf("in_progress label = %q, want %q", lbl, "In Progress")
	}
}

func TestIntegration_StatusList_CustomConfig(t *testing.T) {
	vault := t.TempDir()
	writeFileIT(t, filepath.Join(vault, ".pql", "config.yaml"), `
ticket_statuses:
  - { name: triage,  label: Triage, class: initial, is_default: true }
  - { name: doing,   class: active }
  - { name: shipped, class: terminal }
`)
	stdout, stderr, code := run(t, vault, "ticket", "statuslist")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d statuses, want 3 (custom): %s", len(rows), stdout)
	}
	if rows[0]["name"] != "triage" || rows[0]["label"] != "Triage" || rows[0]["is_default"] != true {
		t.Errorf("first custom status = %#v", rows[0])
	}
	if rows[2]["name"] != "shipped" || rows[2]["is_terminal"] != true {
		t.Errorf("terminal custom status = %#v", rows[2])
	}
}

func TestIntegration_StatusList_InvalidConfigRejected(t *testing.T) {
	vault := t.TempDir()
	// Two defaults — config validation must reject this.
	writeFileIT(t, filepath.Join(vault, ".pql", "config.yaml"), `
ticket_statuses:
  - { name: a, class: initial, is_default: true }
  - { name: b, class: initial, is_default: true }
  - { name: c, class: terminal }
`)
	_, stderr, code := run(t, vault, "ticket", "statuslist")
	if code == 0 {
		t.Fatalf("expected non-zero exit for invalid config, got 0\nstderr: %s", stderr)
	}
}

func writeFileIT(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// --- duplicate-label collision detection at replay (D-26) -----------------

// ticketChangelogRow / idmapChangelogRow render minimal new-format changelog
// INSERTs (rebuild truncates first, so plain INSERTs replay cleanly). A
// duplicate label is two idmap rows with distinct record_ids but one ticket_id.
func ticketChangelogRow(recordID, title string) string {
	return fmt.Sprintf("INSERT INTO tickets (record_id, type, title, status, priority, created_at, updated_at, canonical_version) VALUES ('%s','task','%s','backlog','medium','2025-05-01 10:00:00','2025-05-01 10:00:00',2);\n", recordID, title)
}

func idmapChangelogRow(recordID, label string) string {
	return fmt.Sprintf("INSERT INTO ticket_idmap (record_id, ticket_id, created_at, updated_at, canonical_version) VALUES ('%s','%s','2025-05-01 10:00:00','2025-05-01 10:00:00',2);\n", recordID, label)
}

func TestIntegration_Rebuild_WarnsOnTicketIdCollision(t *testing.T) {
	vault := t.TempDir()
	// Two distinct records both labelled T-9 (the cross-clone clash, D-26).
	writeFileIT(t, filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"),
		ticketChangelogRow("rec-a", "Alpha")+ticketChangelogRow("rec-b", "Beta"))
	writeFileIT(t, filepath.Join(vault, ".pql", "changelog", "ticket_idmap", "2025-05.sql"),
		idmapChangelogRow("rec-a", "T-9")+idmapChangelogRow("rec-b", "T-9"))

	stdout, stderr, code := run(t, vault, "plan", "rebuild")
	if code != 0 {
		t.Fatalf("rebuild exit=%d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(string(stderr), "changelog.ticket_id_collision") {
		t.Errorf("stderr missing collision warning code:\n%s", stderr)
	}
	if !strings.Contains(string(stderr), "T-9") {
		t.Errorf("stderr warning should name T-9:\n%s", stderr)
	}
	var res map[string]any
	if err := json.Unmarshal(stdout, &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	cols, _ := res["collisions"].([]any)
	if len(cols) != 1 {
		t.Fatalf("result collisions = %v, want 1", res["collisions"])
	}
}

func TestIntegration_Rebuild_NoCollisionNoWarning(t *testing.T) {
	vault := t.TempDir()
	writeFileIT(t, filepath.Join(vault, ".pql", "changelog", "tickets", "2025-05.sql"),
		ticketChangelogRow("rec-1", "Only ticket"))
	writeFileIT(t, filepath.Join(vault, ".pql", "changelog", "ticket_idmap", "2025-05.sql"),
		idmapChangelogRow("rec-1", "T-1"))

	stdout, stderr, code := run(t, vault, "plan", "rebuild")
	if code != 0 {
		t.Fatalf("rebuild exit=%d\nstderr: %s", code, stderr)
	}
	if strings.Contains(string(stderr), "collision") {
		t.Errorf("clean changelog should emit no collision warning:\n%s", stderr)
	}
	if strings.Contains(string(stdout), "collisions") {
		t.Errorf("clean result should omit collisions field:\n%s", stdout)
	}
}

// --- child-completeness guard + --force cascade (D-25) --------------------

func TestIntegration_Status_BlocksCloseWithOpenChildren(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "epic", "parent epic", "--id-only")                   // T-1
	pqlIT(t, vault, "ticket", "new", "task", "child task", "--parent", "T-1", "--id-only") // T-2

	_, stderr, code := run(t, vault, "ticket", "status", "T-1", "done")
	if code == 0 {
		t.Fatalf("closing a parent with an open child should fail, got exit 0")
	}
	if !strings.Contains(string(stderr), "T-2") || !strings.Contains(string(stderr), "--force") {
		t.Errorf("error should name the open child and mention --force:\n%s", stderr)
	}
}

func TestIntegration_Status_ForceCascadesToSubtree(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "epic", "epic", "--id-only")                         // T-1
	pqlIT(t, vault, "ticket", "new", "story", "story", "--parent", "T-1", "--id-only")    // T-2
	pqlIT(t, vault, "ticket", "new", "task", "deep task", "--parent", "T-2", "--id-only") // T-3

	stdout, stderr, code := run(t, vault, "ticket", "status", "T-1", "cancelled", "--force")
	if code != 0 {
		t.Fatalf("force close exit=%d\nstderr: %s", code, stderr)
	}
	// Output lists every closed ticket (the whole subtree).
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	got := map[string]string{}
	for _, r := range rows {
		id, _ := r["id"].(string)
		status, _ := r["status"].(string)
		got[id] = status
	}
	for _, id := range []string{"T-1", "T-2", "T-3"} {
		if got[id] != "cancelled" {
			t.Errorf("%s = %q after force cascade, want cancelled (full output: %v)", id, got[id], got)
		}
	}
}

// --- list projection: --fields / --full / --oneline (D-27) ----------------

func TestIntegration_TicketList_DefaultOmitsDescription(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "heavy ticket", "--description", "a very long body", "--id-only")

	stdout := pqlIT(t, vault, "ticket", "list")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if _, ok := rows[0]["description"]; ok {
		t.Errorf("default projection should omit description, got: %v", rows[0])
	}
	if rows[0]["id"] != "T-1" || rows[0]["status"] != "backlog" {
		t.Errorf("default projection lost core fields: %v", rows[0])
	}

	// --full opts back into whole rows; --fields can also name
	// description explicitly.
	for _, args := range [][]string{
		{"ticket", "list", "--full"},
		{"ticket", "list", "--fields", "id,description"},
	} {
		out := pqlIT(t, vault, args...)
		if !strings.Contains(out, "a very long body") {
			t.Errorf("%v should include the description:\n%s", args, out)
		}
	}
}

func TestIntegration_TicketList_FieldsProjectsAndOrders(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "one", "--id-only")

	stdout := pqlIT(t, vault, "ticket", "list", "--fields", "status, id")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("want exactly the 2 requested keys, got: %s", stdout)
	}
	// Keys come out in the requested order (status before id).
	if !strings.Contains(stdout, `{"status":"backlog","id":"T-1"}`) {
		t.Errorf("requested field order not preserved:\n%s", stdout)
	}
}

func TestIntegration_TicketList_UnknownFieldExits64(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "one", "--id-only")

	_, stderr, code := run(t, vault, "ticket", "list", "--fields", "titel")
	if code != 64 {
		t.Fatalf("unknown field should exit 64, got %d", code)
	}
	if !strings.Contains(string(stderr), "titel") || !strings.Contains(string(stderr), "title") {
		t.Errorf("error should name the bad field and the valid set:\n%s", stderr)
	}
}

func TestIntegration_TicketList_Oneline(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "first", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "second", "--id-only")

	stdout := pqlIT(t, vault, "ticket", "list", "--oneline")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), stdout)
	}
	if lines[0] != "T-1\tbacklog\tfirst" {
		t.Errorf("line 0 = %q, want id<TAB>status<TAB>title", lines[0])
	}

	// Plain-text mode refuses the JSON shaping flags.
	for _, extra := range []string{"--pretty", "--jsonl", "--full"} {
		_, _, code := run(t, vault, "ticket", "list", "--oneline", extra)
		if code != 64 {
			t.Errorf("--oneline %s should exit 64, got %d", extra, code)
		}
	}
}

// A bare .pqlignore at the vault root must exclude, with no config file to
// register it first (T-78). It was documented as one of the three vault
// conventions while `ignore_files` defaulted to [.gitignore] alone, so the
// file the tool is named after did nothing until you edited config.yaml.
func TestIntegration_PqlignoreWorksWithoutConfig(t *testing.T) {
	vault := t.TempDir()
	for _, f := range []struct{ path, body string }{
		{"keep.md", "# Keep\n"},
		{"drafts/skip.md", "# Skip\n"},
		{".pqlignore", "drafts/\n"},
	} {
		full := filepath.Join(vault, f.path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(f.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", f.path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(vault, ".pql", "config.yaml")); err == nil {
		t.Fatalf("this test is only meaningful with no config file present")
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(pqlIT(t, vault, "files")), &rows); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	paths := make([]string, 0, len(rows))
	for _, r := range rows {
		p, _ := r["path"].(string)
		paths = append(paths, p)
	}
	if !slices.Contains(paths, "keep.md") {
		t.Errorf("keep.md should be indexed, got %v", paths)
	}
	if slices.Contains(paths, "drafts/skip.md") {
		t.Errorf(".pqlignore did not exclude drafts/, got %v", paths)
	}
}

// `--decision none` reads as a supported negative filter and is not one. It
// matched no decision ref and returned empty, which is indistinguishable from
// a real no-match — so pql now refuses the spelling rather than lying quietly
// (D-31, T-92).
func TestIntegration_TicketList_RejectsAbsenceSpellings(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "unlinked", "--id-only")

	for _, v := range []string{"none", "None", "null", "unset", "-"} {
		_, stderr, code := run(t, vault, "ticket", "list", "--decision", v)
		if code != 64 {
			t.Errorf("--decision %s: exit %d, want 64", v, code)
		}
		if !strings.Contains(string(stderr), "no negative filter") {
			t.Errorf("--decision %s should explain why:\n%s", v, stderr)
		}
	}

	// A real id is still a filter, and an unmatched one is still empty at
	// exit 0 — the refusal must not have turned every value into a lookup.
	stdout, _, code := run(t, vault, "ticket", "list", "--decision", "D-99")
	if code != 0 {
		t.Errorf("unmatched decision id: exit %d, want 0", code)
	}
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("unmatched decision id = %q, want []", got)
	}
}

// A path argument names a thing, so an unknown one is an error rather than an
// empty result (D-29). These verbs used to return [] at exit 0 for a typo,
// which reads as "nothing is related to this file" — while `meta` on the same
// typo exited 66, so the identical mistake was loud on one verb and silent on
// the neighbouring one (T-90).
func TestIntegration_RankedVerbs_RejectUnindexedPath(t *testing.T) {
	vault := councilVault(t)

	for _, verb := range []string{"related", "context"} {
		_, stderr, code := run(t, vault, verb, "members/nobody/persona.md")
		if code != 66 {
			t.Errorf("%s on an unindexed path: exit %d, want 66", verb, code)
		}
		if !strings.Contains(string(stderr), "members/nobody/persona.md") {
			t.Errorf("%s error should name the path:\n%s", verb, stderr)
		}
	}

	// --flat-search takes the primitive path but the argument is still a
	// name, so the same rule applies.
	if _, _, code := run(t, vault, "related", "members/nobody/persona.md", "--flat-search"); code != 66 {
		t.Errorf("--flat-search on an unindexed path: exit %d, want 66", code)
	}

	// A real path still works, so the check is not simply refusing everything.
	if rows := rankedRowsIT(t, vault, "related", "members/vaasa/persona.md"); len(rows) == 0 {
		t.Error("a real path should still rank")
	}

	// `search` takes a query, not a path. A query is a filter, and an
	// unmatched filter is correctly empty at exit 0 — D-29 draws the line
	// here, so guard it against an over-eager future change.
	stdout, _, code := run(t, vault, "search", "zzz-no-such-topic-zzz")
	if code != 0 {
		t.Errorf("search with no matches: exit %d, want 0", code)
	}
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Errorf("search with no matches = %q, want []", got)
	}
}

// --- ranked projection: search / related / context (T-74) -----------------

// rankedRowsIT runs a ranked verb and decodes its rows, failing on a non-zero
// exit or an empty result — a projection assertion over zero rows passes
// vacuously and would hide the very regression these tests exist to catch.
func rankedRowsIT(t *testing.T, vault string, args ...string) []map[string]any {
	t.Helper()
	stdout, stderr, code := run(t, vault, args...)
	if code != 0 {
		t.Fatalf("%v: exit=%d\nstderr: %s", args, code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal(stdout, &rows); err != nil {
		t.Fatalf("%v: invalid JSON: %v\n%s", args, err, stdout)
	}
	if len(rows) == 0 {
		t.Fatalf("%v returned no rows; the assertion would pass vacuously", args)
	}
	return rows
}

func TestIntegration_Ranked_DefaultOmitsProvenance(t *testing.T) {
	vault := councilVault(t)

	for _, verb := range [][]string{
		{"related", "members/vaasa/persona.md"},
		{"context", "members/vaasa/persona.md"},
		{"search", "persona"},
	} {
		rows := rankedRowsIT(t, vault, verb...)
		for i, r := range rows {
			if _, ok := r["signals"]; ok {
				t.Errorf("%v row %d should omit signals by default: %v", verb, i, r)
			}
			if _, ok := r["connections"]; ok {
				t.Errorf("%v row %d should omit connections by default: %v", verb, i, r)
			}
			if _, ok := r["path"]; !ok {
				t.Errorf("%v row %d lost path: %v", verb, i, r)
			}
			if _, ok := r["score"]; !ok {
				t.Errorf("%v row %d lost score: %v", verb, i, r)
			}
		}
	}
}

// The provenance is what makes a ranking accountable, so it must stay exactly
// one flag away — and an omitted key must never be mistakable for a null one.
func TestIntegration_Ranked_FullRestoresProvenance(t *testing.T) {
	vault := councilVault(t)
	target := "members/vaasa/persona.md"

	for _, args := range [][]string{
		{"related", target, "--full"},
		{"related", target, "--fields", "*"},
		{"related", target, "--fields", "path,signals"},
	} {
		rows := rankedRowsIT(t, vault, args...)
		sigs, ok := rows[0]["signals"].([]any)
		if !ok || len(sigs) == 0 {
			t.Fatalf("%v: want a populated signals array, got %v", args, rows[0]["signals"])
		}
		first, ok := sigs[0].(map[string]any)
		if !ok || first["name"] == "" || first["weight"] == nil {
			t.Errorf("%v: signal entry missing name/weight: %v", args, sigs[0])
		}
	}
}

func TestIntegration_Ranked_Oneline(t *testing.T) {
	vault := councilVault(t)

	stdout, stderr, code := run(t, vault, "related", "members/vaasa/persona.md", "--oneline")
	if code != 0 {
		t.Fatalf("exit=%d\nstderr: %s", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("--oneline produced nothing:\n%s", stdout)
	}
	for i, ln := range lines {
		parts := strings.Split(ln, "\t")
		if len(parts) != 2 {
			t.Fatalf("line %d = %q, want path<TAB>score", i, ln)
		}
		if !strings.HasSuffix(parts[0], ".md") {
			t.Errorf("line %d path = %q, want an indexed file path", i, parts[0])
		}
		if _, err := strconv.ParseFloat(parts[1], 64); err != nil {
			t.Errorf("line %d score = %q, not a number", i, parts[1])
		}
	}

	// Plain-text mode refuses the JSON shaping flags here too.
	for _, extra := range []string{"--pretty", "--jsonl", "--full"} {
		_, _, code := run(t, vault, "related", "members/vaasa/persona.md", "--oneline", extra)
		if code != 64 {
			t.Errorf("--oneline %s should exit 64, got %d", extra, code)
		}
	}
}

// --flat-search bypasses ranking, so there is no score to project. The error
// must say so rather than silently returning nothing.
func TestIntegration_Ranked_FlatSearchRejectsScoreField(t *testing.T) {
	vault := councilVault(t)

	_, stderr, code := run(t, vault, "related", "members/vaasa/persona.md", "--flat-search", "--fields", "score")
	if code != 64 {
		t.Fatalf("unknown field under --flat-search should exit 64, got %d", code)
	}
	if !strings.Contains(string(stderr), "score") || !strings.Contains(string(stderr), "path") {
		t.Errorf("error should name the bad field and the valid set:\n%s", stderr)
	}
}

// toFormatOneIT rewrites a vault's changelog back to the format-1 conflict
// clause and drops the marker, standing in for a changelog committed before
// formats were versioned.
func toFormatOneIT(t *testing.T, vault string) {
	t.Helper()
	root := filepath.Join(vault, ".pql", "changelog")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".sql") {
			return err
		}
		if strings.HasSuffix(path, "0000-schema.sql") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		table := filepath.Base(filepath.Dir(path))
		old := strings.ReplaceAll(string(body),
			"WHERE excluded.updated_at >= "+table+".updated_at;",
			"WHERE excluded.updated_at > "+table+".updated_at OR (excluded.updated_at = "+
				table+".updated_at AND excluded.hash > "+table+".hash);")
		return os.WriteFile(path, []byte(old), 0o644)
	})
	if err != nil {
		t.Fatalf("downgrade changelog: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "0000-format.sql")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove marker: %v", err)
	}
}

func TestIntegration_PlanUpgrade_MigratesChangelogForward(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "one", "--id-only")
	toFormatOneIT(t, vault)

	// Dry run first: reports, changes nothing.
	dry := pqlIT(t, vault, "plan", "upgrade", "--dry-run")
	var dryRes struct {
		FoundFormat    string   `json:"found_format"`
		CurrentFormat  string   `json:"current_format"`
		FilesRewritten []string `json:"files_rewritten"`
		DryRun         bool     `json:"dry_run"`
	}
	if err := json.Unmarshal([]byte(dry), &dryRes); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, dry)
	}
	if dryRes.FoundFormat != "1.11.0" || dryRes.CurrentFormat != "2.0.0" {
		t.Errorf("dry run reported found=%s current=%s, want the pre-versioned 1.11.0 and 2.0.0",
			dryRes.FoundFormat, dryRes.CurrentFormat)
	}
	if len(dryRes.FilesRewritten) == 0 || !dryRes.DryRun {
		t.Errorf("dry run should list files and mark itself: %s", dry)
	}

	ticketsFile := filepath.Join(vault, ".pql", "changelog", "tickets")
	entries, err := os.ReadDir(ticketsFile)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no ticket changelog files: %v", err)
	}

	// Real run.
	out := pqlIT(t, vault, "plan", "upgrade")
	var res struct {
		Steps          []struct{ ID string } `json:"steps"`
		FilesRewritten []string              `json:"files_rewritten"`
		Schema         struct {
			Found   string `json:"found"`
			Current string `json:"current"`
		} `json:"schema"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(res.Steps) != 1 || res.Steps[0].ID != "changelog-guard-by-position" {
		t.Errorf("steps = %+v, want the guard step", res.Steps)
	}
	if len(res.FilesRewritten) == 0 {
		t.Error("upgrade rewrote nothing")
	}
	if res.Schema.Found != res.Schema.Current {
		t.Errorf("schema axis = %+v, want a stamped, current database", res.Schema)
	}

	// Idempotent: a second run finds nothing to do.
	again := pqlIT(t, vault, "plan", "upgrade")
	var second struct {
		Steps          []struct{} `json:"steps"`
		FilesRewritten []string   `json:"files_rewritten"`
	}
	if err := json.Unmarshal([]byte(again), &second); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, again)
	}
	if len(second.Steps) != 0 || len(second.FilesRewritten) != 0 {
		t.Errorf("second upgrade was not a no-op: %s", again)
	}
}

// An older format still replays — the rows are readable — but never silently.
func TestIntegration_PlanImport_WarnsOnStaleFormat(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "one", "--id-only")
	toFormatOneIT(t, vault)

	stdout, stderr, code := run(t, vault, "plan", "import")
	if code != 0 {
		t.Fatalf("import exit = %d, want 0 — an older format is replayable\nstdout: %s", code, stdout)
	}
	if !strings.Contains(string(stderr), "format_stale") {
		t.Errorf("stderr should carry the stale-format diagnostic, got: %s", stderr)
	}
	if !strings.Contains(string(stderr), "plan upgrade") {
		t.Errorf("diagnostic should name the verb that fixes it, got: %s", stderr)
	}
}

// A format from the future is refused rather than replayed under rules this
// binary does not know.
func TestIntegration_PlanRebuild_RefusesNewerFormat(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "one", "--id-only")
	marker := filepath.Join(vault, ".pql", "changelog", "0000-format.sql")
	if err := os.WriteFile(marker, []byte("-- pql:changelog_format: 99\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	_, stderr, code := run(t, vault, "plan", "rebuild")
	if code != 65 {
		t.Errorf("exit = %d, want 65 for a changelog newer than the binary", code)
	}
	if !strings.Contains(string(stderr), "99") {
		t.Errorf("refusal should name the format found, got: %s", stderr)
	}
}

func TestIntegration_VersionBuildInfo_ReportsEveryAxis(t *testing.T) {
	cmd := pqlCmd(t, "version", "--build-info")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version --build-info: %v", err)
	}
	var info map[string]any
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{
		"schema_version", "planning_schema_version", "canonical_version", "changelog_format",
	} {
		if _, ok := info[key]; !ok {
			t.Errorf("build info is missing the %s axis: %s", key, out)
		}
	}
}

// A clean vault verifies clean, and --verify stays exit 0 — the flag is a
// report, not a gate (T-60).
func TestIntegration_PlanRebuildVerify_CleanVaultExits0(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "one", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "two", "--id-only")

	stdout := pqlIT(t, vault, "plan", "rebuild", "--verify")
	var res struct {
		Verify struct {
			RowsBefore  map[string]int `json:"rows_before"`
			RowsAfter   map[string]int `json:"rows_after"`
			RowsLost    int            `json:"rows_lost"`
			Divergences []struct {
				Kind string `json:"kind"`
			} `json:"divergences"`
		} `json:"verify"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(res.Verify.Divergences) != 0 || res.Verify.RowsLost != 0 {
		t.Errorf("clean vault reported divergences: %s", stdout)
	}
	if res.Verify.RowsBefore["tickets"] != 2 || res.Verify.RowsAfter["tickets"] != 2 {
		t.Errorf("ticket counts wrong either side of the replay: %s", stdout)
	}
}

// Rows that vanish are the one case that exits non-zero — a report nobody
// reads is not a safety net.
func TestIntegration_PlanRebuildVerify_LostRowsExit65(t *testing.T) {
	vault := initVaultIT(t)
	dbPath := filepath.Join(t.TempDir(), "plan.sqlite")
	pql := func(args ...string) (string, []byte, int) {
		t.Helper()
		full := append([]string{"--vault", vault, "--db", dbPath}, args...)
		cmd := pqlCmd(t, full...)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		code := 0
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			} else {
				t.Fatalf("invoke: %v", err)
			}
		}
		return outBuf.String(), errBuf.Bytes(), code
	}

	if _, _, code := pql("ticket", "new", "task", "kept", "--id-only"); code != 0 {
		t.Fatalf("seed ticket: exit %d", code)
	}
	// Delete the changelog the ticket wrote through to: the row now exists only
	// in the database, so a replay cannot bring it back.
	if err := os.RemoveAll(filepath.Join(vault, ".pql", "changelog")); err != nil {
		t.Fatalf("drop changelog: %v", err)
	}

	stdout, stderr, code := pql("plan", "rebuild", "--verify")
	if code != 65 {
		t.Errorf("exit = %d, want 65 when rows are lost\nstdout: %s", code, stdout)
	}
	if !strings.Contains(string(stderr), "rebuild_divergence") {
		t.Errorf("stderr should carry a divergence diagnostic, got: %s", stderr)
	}
}

// `pql skill show` wrapped the skill in JSON, so reading a shipped skill as
// prose needed a pipe into an extractor — and the skill itself tells callers
// never to pipe. The help even claimed --pretty made it readable; it only
// indents the envelope. --raw is the way out.
func TestIntegration_SkillShowRaw(t *testing.T) {
	vault := initVaultIT(t)

	raw := pqlIT(t, vault, "skill", "show", "--raw")
	if !strings.HasPrefix(raw, "---\nname: pql\n") {
		t.Errorf("--raw should emit the markdown verbatim, got: %.80q", raw)
	}

	// The invariant is byte equality with what the JSON mode carries — the
	// point of --raw is unwrapping, not reformatting. Don't test for the
	// absence of a `\n` two-character sequence: a skill body may legitimately
	// contain one in an example, which is how the first version of this test
	// failed on clean-house's rules.md.
	var bundle struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal([]byte(pqlIT(t, vault, "skill", "show")), &bundle); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if bundle.Files["SKILL.md"] != raw {
		t.Errorf("--raw and the JSON bundle disagree about SKILL.md")
	}

	// --file reaches other files in a multi-file bundle.
	rules := pqlIT(t, vault, "skill", "show", "clean-house", "--raw", "--file", "references/rules.md")
	if !strings.HasPrefix(rules, "# clean-house rule catalog") {
		t.Errorf("--file did not produce that file's raw text: %.80q", rules)
	}

	// Naming a file that isn't in the bundle lists what is.
	_, stderr, code := run(t, vault, "skill", "show", "--raw", "--file", "nope.md")
	if code != 66 {
		t.Fatalf("unknown bundle file should exit 66, got %d", code)
	}
	if !strings.Contains(string(stderr), "SKILL.md") {
		t.Errorf("error should list the files the bundle has:\n%s", stderr)
	}

	// --file without --raw selects nothing, so it is a usage error rather
	// than a silently ignored flag.
	if _, _, code := run(t, vault, "skill", "show", "--file", "SKILL.md"); code != 64 {
		t.Errorf("--file without --raw should exit 64, got %d", code)
	}
}

// `--fields` was scoped to the list verbs, so two agents in live sessions ran
// `ticket show <id> --fields id,status,title` and got exit 64 — the surface is
// not guessable from having learned it on `list` (T-67). It projects the top
// level only; the join-trees are all-or-nothing.
func TestIntegration_ShowVerbs_FieldsProjection(t *testing.T) {
	vault := initVaultIT(t)
	writeFileIT(t, filepath.Join(vault, "governance", "decisions", "architecture.md"), `### D-1: Only decision
- **Date:** 2026-08-08
- **Decision:** One.
`)
	pqlIT(t, vault, "decisions", "sync")
	pqlIT(t, vault, "ticket", "new", "epic", "parent", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "child", "--parent", "T-1", "--description", "a heavy body", "--id-only")

	// Single id keeps the object shape, projected to exactly the named keys.
	var one map[string]any
	out := pqlIT(t, vault, "ticket", "show", "T-2", "--fields", "id,status,title")
	if err := json.Unmarshal([]byte(out), &one); err != nil {
		t.Fatalf("single show should stay an object: %v\n%s", err, out)
	}
	if len(one) != 3 || one["id"] != "T-2" {
		t.Fatalf("want exactly the 3 requested keys, got %s", out)
	}
	if _, ok := one["description"]; ok {
		t.Errorf("description was not requested: %s", out)
	}

	// Batched, the projection applies per element.
	var many []map[string]any
	out = pqlIT(t, vault, "ticket", "show", "T-1,T-2", "--fields", "id")
	if err := json.Unmarshal([]byte(out), &many); err != nil {
		t.Fatalf("batch should render an array: %v\n%s", err, out)
	}
	if len(many) != 2 || len(many[0]) != 1 {
		t.Fatalf("want 2 single-key rows, got %s", out)
	}

	// Same vocabulary and same error shape as the list verbs.
	_, stderr, code := run(t, vault, "ticket", "show", "T-1", "--fields", "titel")
	if code != 64 {
		t.Fatalf("unknown field should exit 64, got %d", code)
	}
	if !strings.Contains(string(stderr), "titel") || !strings.Contains(string(stderr), "title") {
		t.Errorf("error should name the bad field and the valid set:\n%s", stderr)
	}

	// decisions show takes it too. Fresh map: unmarshalling into a populated
	// one merges rather than replaces, so leftover keys would inflate the
	// count and fail an assertion the output actually satisfies.
	var dec map[string]any
	out = pqlIT(t, vault, "decisions", "show", "D-1", "--fields", "id,title")
	if err := json.Unmarshal([]byte(out), &dec); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(dec) != 2 || dec["id"] != "D-1" {
		t.Fatalf("decisions show projection wrong: %s", out)
	}
}

// ChildrenOf is keyed on parent_record_id but `ticket show` passed it the
// friendly T-NNN, so --with-children and the children half of --with-context
// returned nothing at all from the moment D-26 split the two identifiers —
// silently, since an epic with no children is a legitimate answer.
func TestIntegration_TicketShow_ChildrenJoinResolves(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "epic", "parent", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "first child", "--parent", "T-1", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "second child", "--parent", "T-1", "--id-only")

	childIDs := func(args ...string) []string {
		t.Helper()
		var shown struct {
			Children []struct {
				ID string `json:"id"`
			} `json:"children"`
		}
		out := pqlIT(t, vault, append([]string{"ticket", "show", "T-1"}, args...)...)
		if err := json.Unmarshal([]byte(out), &shown); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		ids := make([]string, 0, len(shown.Children))
		for _, c := range shown.Children {
			ids = append(ids, c.ID)
		}
		return ids
	}

	for _, flag := range []string{"--with-children", "--with-context"} {
		got := childIDs(flag)
		if !slices.Equal(got, []string{"T-2", "T-3"}) {
			t.Errorf("%s children = %v, want [T-2 T-3]", flag, got)
		}
	}

	// A genuine leaf still reports none, so the fix did not turn the join
	// into something that always populates.
	var leaf map[string]any
	out := pqlIT(t, vault, "ticket", "show", "T-2", "--with-children")
	if err := json.Unmarshal([]byte(out), &leaf); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if _, ok := leaf["children"]; ok {
		t.Errorf("a leaf should have no children key, got %s", out)
	}
}

// `ticket board` emitted every column, and on a mature board the terminal
// ones are most of the payload — 82% on this repo's own dataset, with no way
// to drop them (T-77). It also carried the status name but not its display
// label, forcing a second call to `ticket statuslist` to render a header.
func TestIntegration_TicketBoard_ColumnFilterAndLabels(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "open one", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "shipped", "--id-only")
	pqlIT(t, vault, "ticket", "status", "T-2", "done")

	type column struct {
		Status  string `json:"status"`
		Label   string `json:"label"`
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	board := func(args ...string) []column {
		t.Helper()
		var cols []column
		out := pqlIT(t, vault, append([]string{"ticket", "board"}, args...)...)
		if err := json.Unmarshal([]byte(out), &cols); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		return cols
	}

	// Unfiltered: both columns, each carrying a human-readable label.
	all := board()
	if len(all) != 2 {
		t.Fatalf("want backlog and done columns, got %d: %v", len(all), all)
	}
	for _, c := range all {
		if c.Label == "" {
			t.Errorf("column %q has no label; a caller must not need statuslist to render a header", c.Status)
		}
	}

	// --open drops terminal columns.
	open := board("--open")
	if len(open) != 1 || open[0].Status != "backlog" {
		t.Fatalf("--open should leave only backlog, got %v", open)
	}

	// --status names an exact set.
	only := board("--status", "done")
	if len(only) != 1 || only[0].Status != "done" || only[0].Label != "Done" {
		t.Fatalf("--status done wrong: %v", only)
	}

	// An unknown column fails loudly rather than rendering an empty board
	// that looks exactly like a finished one.
	_, stderr, code := run(t, vault, "ticket", "board", "--status", "dnoe")
	if code != 64 {
		t.Fatalf("unknown status should exit 64, got %d", code)
	}
	if !strings.Contains(string(stderr), "dnoe") || !strings.Contains(string(stderr), "backlog") {
		t.Errorf("error should name the bad status and the vocabulary:\n%s", stderr)
	}

	// The two filters mean overlapping things; combining them is a mistake
	// worth naming rather than silently resolving.
	if _, _, code := run(t, vault, "ticket", "board", "--status", "backlog", "--open"); code != 64 {
		t.Errorf("--status with --open should exit 64, got %d", code)
	}
}

// `ticket show` has always batched on comma; `decisions show` did not, and
// failed with "decision D-1,D-2 not found" — so "which of these decisions has
// anything implementing it" was one call per record, 41 of them on this repo
// (T-76). Same batching rule now, including the single-id object shape.
func TestIntegration_DecisionsShow_BatchesIDs(t *testing.T) {
	vault := initVaultIT(t)
	writeFileIT(t, filepath.Join(vault, "governance", "decisions", "architecture.md"), `### D-1: First
- **Date:** 2026-08-08
- **Decision:** One.

### D-2: Second
- **Date:** 2026-08-08
- **Decision:** Two.
`)
	pqlIT(t, vault, "decisions", "sync")
	pqlIT(t, vault, "ticket", "new", "task", "implements D-2", "--decision", "D-2", "--id-only")

	// A batch is an array, in the order given, and --with-tickets composes.
	var rows []struct {
		ID      string `json:"id"`
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	out := pqlIT(t, vault, "decisions", "show", "D-2,D-1", "--with-tickets")
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("batch should render an array: %v\n%s", err, out)
	}
	if len(rows) != 2 || rows[0].ID != "D-2" || rows[1].ID != "D-1" {
		t.Fatalf("want [D-2 D-1] in the order given, got %s", out)
	}
	if len(rows[0].Tickets) != 1 {
		t.Errorf("--with-tickets did not compose with the batch: %s", out)
	}
	if len(rows[1].Tickets) != 0 {
		t.Errorf("D-1 has no tickets; got %v", rows[1].Tickets)
	}

	// One id keeps the single-object shape it has always had.
	single := pqlIT(t, vault, "decisions", "show", "D-1")
	var one map[string]any
	if err := json.Unmarshal([]byte(single), &one); err != nil {
		t.Fatalf("single id should render an object: %v\n%s", err, single)
	}
	if one["id"] != "D-1" {
		t.Errorf("single show = %s, want the D-1 record", single)
	}

	// An unknown id in a batch fails the call and names the id that is
	// missing — not the whole comma string, which is what it used to do.
	_, stderr, code := run(t, vault, "decisions", "show", "D-1,D-99")
	if code != 66 {
		t.Fatalf("unknown id in a batch should exit 66, got %d", code)
	}
	if !strings.Contains(string(stderr), "D-99") || strings.Contains(string(stderr), "D-1,D-99") {
		t.Errorf("error should name D-99 alone, got: %s", stderr)
	}
}

// Decision linkage used to be settable only at `ticket new`, so a tree created
// without --decision could never be repaired and the decision's own
// implementation-status view (D-20) quietly under-reported (T-61).
func TestIntegration_TicketDecision_LinksAfterCreation(t *testing.T) {
	vault := initVaultIT(t)
	writeFileIT(t, filepath.Join(vault, "governance", "decisions", "architecture.md"), `### D-1: Linked later
- **Date:** 2026-08-07
- **Decision:** Tickets can be attached after the fact.
`)
	pqlIT(t, vault, "decisions", "sync")

	// Two tickets created the way a delegated agent gets it wrong: no --decision.
	pqlIT(t, vault, "ticket", "new", "task", "first", "--id-only")
	pqlIT(t, vault, "ticket", "new", "task", "second", "--id-only")

	decisionRefs := func() []string {
		t.Helper()
		out := pqlIT(t, vault, "decisions", "show", "D-1", "--with-tickets")
		var shown struct {
			Tickets []struct {
				ID string `json:"id"`
			} `json:"tickets"`
		}
		if err := json.Unmarshal([]byte(out), &shown); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, out)
		}
		ids := make([]string, 0, len(shown.Tickets))
		for _, tk := range shown.Tickets {
			ids = append(ids, tk.ID)
		}
		return ids
	}
	if got := decisionRefs(); len(got) != 0 {
		t.Fatalf("decision starts with tickets %v, want none", got)
	}

	// Batch repair, the shape the motivating case needed.
	pqlIT(t, vault, "ticket", "decision", "T-1,T-2", "D-1")
	if got := decisionRefs(); len(got) != 2 {
		t.Errorf("after linking, --with-tickets reports %v, want both tickets", got)
	}

	// And it clears, mirroring `setparent … none`.
	pqlIT(t, vault, "ticket", "decision", "T-1", "none")
	if got := decisionRefs(); len(got) != 1 || got[0] != "T-2" {
		t.Errorf("after clearing T-1, --with-tickets reports %v, want [T-2]", got)
	}
}

func TestIntegration_TicketDecision_UnknownDecisionExits65(t *testing.T) {
	vault := initVaultIT(t)
	pqlIT(t, vault, "ticket", "new", "task", "orphan", "--id-only")

	_, stderr, code := run(t, vault, "ticket", "decision", "T-1", "D-404")
	if code != 65 {
		t.Errorf("exit = %d, want 65 for an unknown decision", code)
	}
	if !strings.Contains(string(stderr), "D-404") {
		t.Errorf("stderr should name the missing decision, got: %s", stderr)
	}
}

func TestIntegration_DecisionsList_FieldsAndOneline(t *testing.T) {
	vault := initVaultIT(t)
	writeFileIT(t, filepath.Join(vault, "governance", "decisions", "architecture.md"), `### D-1: Test decision
- **Date:** 2026-07-08
- **Decision:** Keep it small.
`)
	pqlIT(t, vault, "decisions", "sync")

	oneline := pqlIT(t, vault, "decisions", "list", "--oneline")
	if !strings.HasPrefix(oneline, "D-1\tactive\tTest decision") {
		t.Errorf("decisions --oneline = %q, want id<TAB>status<TAB>title", oneline)
	}

	stdout := pqlIT(t, vault, "decisions", "list", "--fields", "id,domain")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 || len(rows[0]) != 2 || rows[0]["domain"] != "architecture" {
		t.Errorf("decisions --fields projection wrong: %s", stdout)
	}
}

// initVaultIT returns a fresh vault; pql.db is created lazily by the first
// ticket write, so no explicit init is needed.
func initVaultIT(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// pqlIT runs pql against vault and fails the test on a non-zero exit.
func pqlIT(t *testing.T, vault string, args ...string) string {
	t.Helper()
	stdout, stderr, code := run(t, vault, args...)
	if code != 0 {
		t.Fatalf("pql %v exit=%d\nstderr: %s", args, code, stderr)
	}
	return string(stdout)
}
