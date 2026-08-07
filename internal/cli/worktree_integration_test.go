//go:build integration

// Vault resolution from inside a linked git worktree (T-58). These tests use
// no --vault flag on purpose: the point is what the binary resolves on its
// own, which is what every real worktree session gets.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

// runFrom invokes pqlBin with the working directory set to dir and no --vault
// flag, so the binary has to discover the vault itself. The --db override
// still points outside the fixture (it does not participate in vault
// resolution) so a stray write cannot dirty the tree under test.
func runFrom(t *testing.T, dir string, args ...string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "integration.sqlite")
	cmd := pqlCmd(t, append([]string{"--db", dbPath}, args...)...)
	cmd.Dir = dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		exitCode = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("invoke pql: %v\nstderr: %s", err, errBuf.String())
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

const mainDQR = `### D-1: Shared decision
- **Date:** 2026-08-07
- **Decision:** Present in both trees.
`

const worktreeOnlyDQR = `
### D-2: Worktree-only decision
- **Date:** 2026-08-07
- **Decision:** Written in the worktree, absent from main.
`

// worktreeFixture builds a main checkout carrying both markers (.git dir and
// .obsidian) plus a linked worktree nested inside it — the exact shape that
// resolved to the wrong tree. Returns the two roots.
func worktreeFixture(t *testing.T) (main, wt string) {
	t.Helper()
	main = initVaultIT(t)
	writeFileIT(t, filepath.Join(main, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFileIT(t, filepath.Join(main, ".obsidian", "app.json"), "{}\n")
	writeFileIT(t, filepath.Join(main, "governance", "decisions", "architecture.md"), mainDQR)

	wt = filepath.Join(main, ".worktrees", "feature")
	writeFileIT(t, filepath.Join(wt, ".git"),
		"gitdir: "+filepath.Join(main, ".git", "worktrees", "feature")+"\n")
	writeFileIT(t, filepath.Join(wt, "governance", "decisions", "architecture.md"),
		mainDQR+worktreeOnlyDQR)
	return main, wt
}

// The headline symptom: a worktree session silently reads and writes the main
// checkout's vault. doctor is the cheapest place to see it.
func TestIntegration_Worktree_DoctorResolvesWorktreeRoot(t *testing.T) {
	_, wt := worktreeFixture(t)

	stdout, stderr, code := runFrom(t, wt, "doctor")
	if code != 0 {
		t.Fatalf("doctor exit=%d\nstderr: %s", code, stderr)
	}
	var got struct {
		Vault struct {
			Path          string `json:"path"`
			DiscoveredVia string `json:"discovered_via"`
		} `json:"vault"`
	}
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got.Vault.Path != wt {
		t.Errorf("vault.path = %q, want the worktree %q", got.Vault.Path, wt)
	}
	if got.Vault.DiscoveredVia == "" {
		t.Error("discovered_via is empty; it is how a wrong resolution gets spotted")
	}
}

// decisions claim parses the DQR tree and touches no database, so the id it
// hands back is a clean read of *which tree was parsed*: main holds D-1 only,
// the worktree holds D-1 and D-2. Before the fix this returned D-2 — main's
// answer — which is the same false-positive class as validate greening
// against markdown the user never edited.
func TestIntegration_Worktree_DQRReadsWorktreeTree(t *testing.T) {
	_, wt := worktreeFixture(t)

	stdout, stderr, code := runFrom(t, wt, "decisions", "claim", "D", "architecture", "probe")
	if code != 0 {
		t.Fatalf("decisions claim exit=%d\nstderr: %s", code, stderr)
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got.ID != "D-3" {
		t.Errorf("claimed id = %q, want D-3 (the worktree tree holds D-1 and D-2); "+
			"D-2 means main's tree was parsed instead", got.ID)
	}
}

// validate has to run against the worktree's own markdown. A broken record in
// the worktree must fail even though main's tree is clean — the false
// positive reported from the field was exactly this, inverted.
func TestIntegration_Worktree_ValidateSeesWorktreeMarkdown(t *testing.T) {
	_, wt := worktreeFixture(t)
	// A duplicate id across two files is a structural error. It exists only in
	// the worktree, so validate can only fail if it read the worktree's tree.
	writeFileIT(t, filepath.Join(wt, "governance", "decisions", "platform.md"),
		"### D-1: Duplicate of the shared decision\n- **Date:** 2026-08-07\n- **Decision:** Same id, second file.\n")

	_, _, code := runFrom(t, wt, "decisions", "validate")
	if code == 0 {
		t.Error("validate passed from the worktree, but the worktree holds a malformed record; " +
			"it validated the main checkout instead")
	}
}

// The main checkout must keep resolving to itself — the fix moves worktrees,
// not ordinary sessions.
func TestIntegration_Worktree_MainCheckoutUnaffected(t *testing.T) {
	main, _ := worktreeFixture(t)

	stdout, stderr, code := runFrom(t, main, "decisions", "claim", "D", "architecture", "probe")
	if code != 0 {
		t.Fatalf("decisions claim exit=%d\nstderr: %s", code, stderr)
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(stdout, &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got.ID != "D-2" {
		t.Errorf("claimed id = %q, want D-2 (main's tree holds D-1 only)", got.ID)
	}
}
