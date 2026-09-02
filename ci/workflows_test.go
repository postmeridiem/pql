// Package ci holds the checks that guard this directory's own wiring.
//
// There is no Go source here — the substance of CI is the shell scripts
// alongside this file. What lives here is the test that keeps the scripts and
// the GitHub Actions workflows that call them honest about each other, which
// is the failure T-70 recorded: ci/release.sh was dead code for months while
// two documents described it as the release path, and nothing could tell.
//
// Deliberately not actionlint. That tool is better than this at the things it
// covers — expression syntax, deprecated action versions, runner labels — but
// it is a fourth pinned binary every contributor and CI job would have to
// install, and it cannot check the two properties that actually broke here: a
// workflow calling a script that does not exist, and a script no caller
// invokes. Those need to know what this repo's ci/ directory means. The YAML
// parser is already a direct dependency, so this costs nothing to run and
// runs inside `make test` on every push.
package ci

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// workflow is the subset of the GitHub Actions schema these checks read. The
// rest is deliberately not modelled: an unknown key is not an error here, and
// pretending to validate the whole schema is how a homegrown check starts
// claiming more than it does.
type workflow struct {
	Env  map[string]string `yaml:"env"`
	Jobs map[string]struct {
		Env   map[string]string `yaml:"env"`
		Steps []struct {
			Name string            `yaml:"name"`
			Run  string            `yaml:"run"`
			Uses string            `yaml:"uses"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

const (
	workflowDir = "../.github/workflows"
	repoRoot    = ".."
)

// envRef matches ${{ env.NAME }} with any interior spacing.
var envRef = regexp.MustCompile(`\$\{\{\s*env\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// scriptRef matches a ./ci/<name>.sh invocation inside a run: block.
var scriptRef = regexp.MustCompile(`\./(ci/[A-Za-z0-9._-]+\.sh)`)

// workflowFiles returns every workflow path, and fails if there are none.
//
// The emptiness check is the point: every test below iterates this slice, so
// a wrong directory or a renamed path would silently turn all of them into
// vacuous passes. Requiring a positive count is what stops "found nothing to
// check" from reading as "checked everything and it was fine".
func workflowFiles(t *testing.T) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(workflowDir, "*.y*ml"))
	if err != nil {
		t.Fatalf("globbing %s: %v", workflowDir, err)
	}
	if len(matches) == 0 {
		t.Fatalf("no workflow files under %s — the checks below would all pass vacuously", workflowDir)
	}
	sort.Strings(matches)
	return matches
}

func readWorkflow(t *testing.T, path string) (string, workflow) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var wf workflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("%s does not parse as YAML: %v", path, err)
	}
	return string(raw), wf
}

// TestWorkflowsParse is the floor. A workflow that does not parse is not
// rejected at push time — GitHub simply never runs it, so the failure looks
// like nothing happening.
func TestWorkflowsParse(t *testing.T) {
	for _, path := range workflowFiles(t) {
		_, wf := readWorkflow(t, path)
		if len(wf.Jobs) == 0 {
			t.Errorf("%s parses but declares no jobs", path)
		}
	}
}

// TestEnvReferencesResolve catches the typo that costs a release. An
// unresolvable ${{ env.X }} expands to the empty string rather than failing,
// so `go install pkg@${{ env.TYPO }}` becomes `go install pkg@` and the error
// surfaces several steps later as something else.
//
// Scope note: this collects env keys defined anywhere in the file rather than
// tracking which are in scope at each reference. That accepts a job-level key
// referenced from a different job, which is wrong but not the failure mode
// worth the extra machinery — a misspelling is, and this catches it.
func TestEnvReferencesResolve(t *testing.T) {
	for _, path := range workflowFiles(t) {
		raw, wf := readWorkflow(t, path)

		defined := map[string]bool{}
		for k := range wf.Env {
			defined[k] = true
		}
		for _, job := range wf.Jobs {
			for k := range job.Env {
				defined[k] = true
			}
			for _, step := range job.Steps {
				for k := range step.Env {
					defined[k] = true
				}
			}
		}

		for _, m := range envRef.FindAllStringSubmatch(raw, -1) {
			if !defined[m[1]] {
				t.Errorf("%s references ${{ env.%s }}, which no env: block defines "+
					"(it would expand to the empty string)", path, m[1])
			}
		}
	}
}

// TestRunBlocksAreValidShell parses every inline run: block. release.yaml
// mints and pushes the release tag from one, so a syntax error there fails
// with a tag already published.
func TestRunBlocksAreValidShell(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		// Not skipped. Every script in this directory is bash, the Makefile is
		// driven by it, and the pre-push gate shells out to it — a machine
		// without bash cannot run this repo's checks at all, so reporting
		// success here would be reporting on something never examined.
		t.Fatalf("bash not on PATH: %v", err)
	}

	for _, path := range workflowFiles(t) {
		_, wf := readWorkflow(t, path)
		for jobName, job := range wf.Jobs {
			for i, step := range job.Steps {
				if strings.TrimSpace(step.Run) == "" {
					continue
				}
				label := step.Name
				if label == "" {
					label = "step " + strconv.Itoa(i)
				}
				cmd := exec.Command(bash, "-n")
				cmd.Stdin = strings.NewReader(step.Run)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("%s: job %q, %s: run block is not valid bash: %v\n%s",
						path, jobName, label, err, out)
				}
			}
		}
	}
}

// TestWorkflowScriptsExist guards one half of T-70: a workflow that calls a
// script which is missing, or is present but not executable.
func TestWorkflowScriptsExist(t *testing.T) {
	for _, path := range workflowFiles(t) {
		raw, _ := readWorkflow(t, path)
		for _, m := range scriptRef.FindAllStringSubmatch(raw, -1) {
			script := filepath.Join(repoRoot, m[1])
			info, err := os.Stat(script)
			if err != nil {
				t.Errorf("%s invokes %s, which does not exist: %v", path, m[1], err)
				continue
			}
			if info.Mode()&0o111 == 0 {
				t.Errorf("%s invokes %s, which is not executable (mode %v)", path, m[1], info.Mode())
			}
		}
	}
}

// TestEveryScriptHasACaller guards the other half, and is the check T-70 would
// have failed. ci/release.sh sat unreferenced by any workflow while two
// documents called it the release path; nothing in the repo could say so.
//
// A caller is a workflow or the Makefile. Both are legitimate: ci/secrets.sh
// and ci/eval.sh are deliberately local-only and reached through make targets,
// while ci/release.sh has no make target and is reached only from a workflow.
// What is not legitimate is neither.
func TestEveryScriptHasACaller(t *testing.T) {
	scripts, err := filepath.Glob("*.sh")
	if err != nil {
		t.Fatalf("globbing ci/*.sh: %v", err)
	}
	if len(scripts) == 0 {
		t.Fatal("no ci/*.sh found — this check would pass vacuously")
	}

	callers := map[string]string{}
	makefile, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		t.Fatalf("reading Makefile: %v", err)
	}
	callers["Makefile"] = string(makefile)
	for _, path := range workflowFiles(t) {
		raw, _ := readWorkflow(t, path)
		callers[path] = raw
	}

	for _, script := range scripts {
		ref := "ci/" + script
		found := false
		for _, body := range callers {
			if strings.Contains(body, ref) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is invoked by no workflow and no Makefile target — "+
				"either wire it up or delete it (T-70)", ref)
		}
	}
}
