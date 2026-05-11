package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/changelog"
	"github.com/postmeridiem/pql/internal/planning/repo"
	"github.com/postmeridiem/pql/internal/skill"
	"github.com/postmeridiem/pql/internal/version"
)

// initResult is the JSON shape `pql init` emits on stdout. Each sub-stat
// describes one of the project-state fixers init runs through.
type initResult struct {
	Directory         string             `json:"directory"`
	Config            initConfigStat     `json:"config"`
	Gitignore         initGitignore      `json:"gitignore"`
	Skills            []initSkillStat    `json:"skills"`
	Permissions       initPermissions    `json:"permissions"`
	DQR               initDQRStruct      `json:"dqr"`
	PlanImport        initPlanImport     `json:"plan_import"`
	DecisionsSync     initDecisionsSync  `json:"decisions_sync"`
	Changelog         initChangelogStat  `json:"changelog"`
	GitAttributes     initGitAttribute   `json:"gitattributes"`
	Hook              initHookStat       `json:"hook"`
	PostMergeHook     initHookStat       `json:"post_merge_hook"`
	PostCheckoutHook  initHookStat       `json:"post_checkout_hook"`
	PostRewriteHook   initHookStat       `json:"post_rewrite_hook"`
}

// initDecisionsSync mirrors repo.SyncResult for the init JSON
// surface, plus a Skipped reason when the decisions/ directory is
// absent or sync can't be run.
type initDecisionsSync struct {
	Synced  int    `json:"synced"`
	Refs    int    `json:"refs"`
	Broken  int    `json:"broken"`
	Skipped string `json:"skipped,omitempty"`
}

type initChangelogStat struct {
	Root         string   `json:"root,omitempty"`
	TablesSeeded []string `json:"tables_seeded,omitempty"`
	Skipped      string   `json:"skipped,omitempty"`
}

type initGitAttribute struct {
	Path     string `json:"path,omitempty"`
	Appended bool   `json:"appended"`
	Existed  bool   `json:"existed"`
	Skipped  string `json:"skipped,omitempty"`
}

type initHookStat struct {
	Path     string `json:"path,omitempty"`
	Installed bool  `json:"installed"`
	Existed   bool  `json:"existed"`
	Skipped  string `json:"skipped,omitempty"`
}

type initPlanImport struct {
	File     string `json:"file,omitempty"`
	Imported bool   `json:"imported"`
	Count    int    `json:"count,omitempty"`
	Skipped  string `json:"skipped,omitempty"`
}

type initPermissions struct {
	Path    string   `json:"path,omitempty"`
	Added   []string `json:"added,omitempty"`
	Existed bool     `json:"existed"`
}

type initConfigStat struct {
	Path        string `json:"path"`
	Created     bool   `json:"created"`
	Overwritten bool   `json:"overwritten"`
	Skipped     bool   `json:"skipped,omitempty"` // true when file existed and --force not set
}

type initGitignore struct {
	Path     string `json:"path,omitempty"`
	Exists   bool   `json:"exists"`
	Appended bool   `json:"appended"`
	Entry    string `json:"entry,omitempty"`
}

// modePreserved is the Mode value emitted when an install was kept
// as-is (skill is current, modified, or unknown — no overwrite).
const modePreserved = "preserved"

type initSkillStat struct {
	Name  string `json:"name"`            // bundled skill name (e.g. "pql", "clean-house")
	Scope string `json:"scope,omitempty"` // "user" | "project" — resolved scope for this install
	Mode  string `json:"mode"`            // "yes" | "no" | "prompt-declined" | "prompt-accepted" | "prompt-skipped-no-tty" | modePreserved
	State string `json:"state"`           // post-action state per internal/skill
	Path  string `json:"path,omitempty"`  // install directory
	Hash  string `json:"hash,omitempty"`  // bundle hash (when present)
	Note  string `json:"note,omitempty"`  // human-readable explanation
}

const defaultConfigBody = `# pql configuration. See docs/structure/initial-plan.md and
# docs/vault-layout.md for the documented shape.

# Frontmatter dialect: yaml | toml.
frontmatter: yaml

# Wikilink dialect: obsidian | pandoc | markdown.
wikilinks: obsidian

# Where to draw tag information from: any subset of [inline, frontmatter].
tags:
  sources: [inline, frontmatter]

# Glob patterns to exclude from indexing. Built-in defaults (.git/, .pql/,
# sqlite sidecars) are always honoured. Use .pqlignore for richer rules.
exclude:
  - "**/.obsidian/**"
  - "**/node_modules/**"

# Per-vault ignore files the indexer consults when walking. Each name is a
# file at the vault root with gitignore-syntax rules. Default follows
# .gitignore so pql piggy-backs on what most projects already exclude
# (build outputs, vendored deps). To carry pql-specific deviations
# without polluting .gitignore, add a tiny .pqlignore alongside:
#   ignore_files: [.gitignore, .pqlignore]
# Order matters — later files win on per-pattern conflicts. Set to [] to
# disable file-based exclusions entirely (.git/ and .pql/ are still
# excluded; that's a non-overridable built-in).
ignore_files: [.gitignore]

# Populate file.gitmtime / file.gitauthor from git log.
git_metadata: false

# Build an FTS5 index of note bodies. Off by default; only worth turning on
# if you actually use 'body MATCH ...' queries.
fts: false

# Optional aliases — short names usable as PQL macros.
# aliases:
#   members: "type = 'council-member'"
`

func newInitCmd() *cobra.Command {
	var (
		force     bool
		withSkill string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bring this directory to a known-good pql project state (idempotent)",
		Long: `Idempotent project fixer. Each step is safe to re-run:

  - .pql/config.yaml → created with defaults if missing; left alone if it
                       exists (use --force to overwrite).
  - .gitignore       → if one exists and doesn't already mention .pql/,
                       append it. Never created from scratch.
  - SKILL.md         → see --with-skill below.

Skill install behaviour follows --with-skill:

  --with-skill=yes      always install (or update a stale install)
  --with-skill=no       never touch the skill install
  --with-skill=prompt   (default) interactively ask if stdin is a TTY,
                        otherwise behave as 'no'

Output is one JSON object describing what changed. Idempotent re-runs
report all sub-stats with their final state so scripting can detect
drift.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := initTargetDir(cmd)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			cfgPath := filepath.Join(dir, config.VaultStateDir, "config.yaml")
			cfgStat, err := writeDefaultConfig(cfgPath, force)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			giPath := filepath.Join(dir, ".gitignore")
			giStat, err := ensurePqlGitignore(giPath)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			skillStats, err := initSkillStep(dir, withSkill, cmd.InOrStdin(), cmd.OutOrStderr())
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			permStat := ensurePqlPermissions(dir)
			dqrStat := ensureDQRStructure(dir)
			planStat := autoImportPlan(cmd.Context(), dir)
			decisionsStat := ensureDecisionsSynced(cmd.Context(), dir)
			readmeUpdated := regenerateReadmeStep(cmd.Context(), dir, &dqrStat)
			_ = readmeUpdated // result captured on dqrStat.ReadmeUpdated
			changelogStat := ensureChangelogDirs(dir)
			gitAttrStat := ensureGitAttributes(dir)
			hookStat := ensurePlanExportHook(dir)
			postMergeStat := ensurePlanImportHook(dir)
			postCheckoutStat := ensurePostCheckoutHook(dir)
			postRewriteStat := ensurePostRewriteHook(dir)

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			result := &initResult{
				Directory:        dir,
				Config:           cfgStat,
				Gitignore:        giStat,
				Skills:           skillStats,
				Permissions:      permStat,
				DQR:              dqrStat,
				PlanImport:       planStat,
				DecisionsSync:    decisionsStat,
				Changelog:        changelogStat,
				GitAttributes:    gitAttrStat,
				Hook:             hookStat,
				PostMergeHook:    postMergeStat,
				PostCheckoutHook: postCheckoutStat,
				PostRewriteHook:  postRewriteStat,
			}
			if _, err := render.One(result, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing .pql/config.yaml")
	cmd.Flags().StringVar(&withSkill, "with-skill", "prompt", "skill install: yes | no | prompt (TTY)")
	return cmd
}

// initTargetDir is cwd, unless --vault is set in which case we honour that.
// We deliberately do NOT walk up looking for .obsidian/.git markers: a user
// running `pql init` is declaring this is the vault root, not asking us to
// guess.
func initTargetDir(cmd *cobra.Command) (string, error) {
	flag, _ := cmd.Flags().GetString("vault")
	if flag != "" {
		abs, err := filepath.Abs(flag)
		if err != nil {
			return "", fmt.Errorf("resolve --vault %q: %w", flag, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("--vault %q: %w", flag, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("--vault %q is not a directory", flag)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return cwd, nil
}

// writeDefaultConfig is idempotent: if path exists and force is false,
// nothing is written and Skipped=true. Hand-edited configs are
// preserved by default; --force reseeds with the embedded defaults.
//
// Creates parent directories (e.g. <vault>/.pql/) as needed.
func writeDefaultConfig(path string, force bool) (initConfigStat, error) {
	stat := initConfigStat{Path: path}
	_, err := os.Stat(path)
	exists := err == nil
	if exists && !force {
		stat.Skipped = true
		return stat, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return stat, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(defaultConfigBody), 0o600); err != nil {
		return stat, fmt.Errorf("write %s: %w", path, err)
	}
	if exists {
		stat.Overwritten = true
	} else {
		stat.Created = true
	}
	return stat, nil
}

// ensureGitignoreEntry appends `entry` to the gitignore at path if such a
// file exists and the entry isn't already present (treating each line as
// trimmed text, ignoring leading slashes and trailing whitespace). Doesn't
// create a gitignore if one isn't already there.
func ensureGitignoreEntry(path, entry string) (initGitignore, error) {
	stat := initGitignore{}
	body, err := os.ReadFile(path) //nolint:gosec // G304: path is the project's own .gitignore, resolved by the caller
	if errors.Is(err, os.ErrNotExist) {
		return stat, nil
	}
	if err != nil {
		return stat, fmt.Errorf("read %s: %w", path, err)
	}
	stat.Path = path
	stat.Exists = true

	scanner := bufio.NewScanner(bytes.NewReader(body))
	wanted := strings.TrimSuffix(strings.TrimPrefix(entry, "/"), "/")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimSuffix(strings.TrimPrefix(line, "/"), "/")
		if line == wanted {
			return stat, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, fmt.Errorf("scan %s: %w", path, err)
	}

	var buf bytes.Buffer
	buf.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(entry)
	buf.WriteByte('\n')

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return stat, fmt.Errorf("write %s: %w", path, err)
	}
	stat.Appended = true
	stat.Entry = entry
	return stat, nil
}

const (
	pqlDirIgnore = ".pql/"
	pqlGlobIgnore = ".pql/*"
)

// ensurePqlGitignore manages the pql block in .gitignore. Uses .pql/*
// (not .pql/) so individual files can be un-ignored. Adds explicit
// includes for the plan export and hooks directory.
func ensurePqlGitignore(path string) (initGitignore, error) {
	body, err := os.ReadFile(path) //nolint:gosec // G304: project's own .gitignore
	if errors.Is(err, os.ErrNotExist) {
		return initGitignore{}, nil
	}
	if err != nil {
		return initGitignore{}, fmt.Errorf("read %s: %w", path, err)
	}
	stat := initGitignore{Path: path, Exists: true}

	lines := strings.Split(string(body), "\n")

	// Intentional omissions from the managed list:
	//   .pql/hooks/        — per-clone state (T-28). Per-developer
	//                        absolute pql paths bake in; tracking
	//                        produces multi-dev drift.
	//   .pql/pql-plan.json — pre-D-15 snapshot artefact (T-41).
	//                        Replication moved to .pql/changelog/;
	//                        plan export still writes the file as a
	//                        manual backup but consumers decide
	//                        whether to commit it.
	// Existing repos that still carry either exception keep it — we
	// don't actively prune user content. New repos get a clean
	// ignore.
	needed := map[string]bool{
		pqlGlobIgnore:      true,
		"!.pql/changelog/": true,
	}
	hasDirForm := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		delete(needed, trimmed)
		if trimmed == pqlDirIgnore {
			hasDirForm = true
		}
	}

	if hasDirForm {
		for i, line := range lines {
			if strings.TrimSpace(line) == pqlDirIgnore {
				lines[i] = pqlGlobIgnore
				delete(needed, pqlGlobIgnore)
				break
			}
		}
	}

	if len(needed) == 0 && !hasDirForm {
		return stat, nil
	}

	var buf bytes.Buffer
	buf.WriteString(strings.Join(lines, "\n"))
	content := buf.String()
	if content != "" && content[len(content)-1] != '\n' {
		buf.WriteByte('\n')
	}

	for _, entry := range []string{pqlGlobIgnore, "!.pql/changelog/"} {
		if needed[entry] {
			buf.WriteString(entry)
			buf.WriteByte('\n')
		}
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return stat, fmt.Errorf("write %s: %w", path, err)
	}
	stat.Appended = true
	return stat, nil
}

const pqlHookMarker = "# --- pql plan export ---"

// renderPreCommitHook bakes the absolute pql binary path into the
// pre-commit hook. PATH is unreliable in git's hook shell (witnessed
// the bare `pql` form silently no-op when ~/.local/bin wasn't on
// hook-shell PATH), so we resolve the binary at install time and write
// it in. If the binary later moves, re-run `pql init` to refresh.
func renderPreCommitHook(pqlPath string) string {
	return `# --- pql plan export ---
# Auto-installed by pql init. Exports planning state and stages the
# snapshot so it lands in the same commit as the change that produced
# it. The absolute pql path is baked in at install time — re-run
# 'pql init' if the binary moves.
` + shellQuote(pqlPath) + ` plan export --stage 2>/dev/null || true
# --- end pql ---
`
}

func ensurePlanExportHook(dir string) initHookStat {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return initHookStat{Skipped: "not a git repository"}
	}

	hookDir := filepath.Join(dir, ".pql", "hooks")
	hookPath := filepath.Join(hookDir, "pre-commit")
	stat := initHookStat{Path: hookPath}

	if err := os.MkdirAll(hookDir, 0o750); err != nil {
		stat.Skipped = "mkdir: " + err.Error()
		return stat
	}

	body := renderPreCommitHook(resolvePqlPath())

	// Always (re-)plant the shim so existing installs pick up changes
	// to where git actually looks for hooks (e.g. core.hooksPath set
	// after the initial install). The shim itself is idempotent.
	defer ensureGitHookShim(dir, "pre-commit")

	existing, err := os.ReadFile(hookPath) //nolint:gosec // G304: known hook path
	if err == nil {
		stat.Existed = true
		if strings.Contains(string(existing), pqlHookMarker) {
			return stat
		}
		// Prepend our block to existing hook content.
		var buf bytes.Buffer
		buf.WriteString(body)
		buf.WriteByte('\n')
		buf.Write(existing)
		if err := os.WriteFile(hookPath, buf.Bytes(), 0o750); err != nil { //nolint:gosec // G306: hook must be executable
			stat.Skipped = "write: " + err.Error()
			return stat
		}
		stat.Installed = true
		return stat
	}

	content := "#!/bin/sh\n" + body
	if err := os.WriteFile(hookPath, []byte(content), 0o750); err != nil { //nolint:gosec // G306: hook must be executable
		stat.Skipped = "write: " + err.Error()
		return stat
	}
	stat.Installed = true
	return stat
}

const pqlPostMergeMarker = "# --- pql plan import ---"

// renderPostMergeHook bakes the absolute pql binary path in for the
// same reason renderPreCommitHook does — git's hook PATH isn't the
// user's interactive PATH.
func renderPostMergeHook(pqlPath string) string {
	q := shellQuote(pqlPath)
	return `# --- pql plan import ---
# Auto-installed by pql init. Replays new changelog files into
# pql.db, then re-syncs decisions from decisions/*.md so the
# markdown-sourced half stays in step with whatever the merge
# brought in. Both ops are idempotent — a no-op merge is a no-op
# hook (D-8, D-16).
` + q + ` plan import 2>/dev/null || true
` + q + ` decisions sync 2>/dev/null || true
# --- end pql ---
`
}

const pqlPostCheckoutMarker = "# --- pql plan rebuild (post-checkout) ---"

// renderPostCheckoutHook fires only on branch switches ($3 == "1");
// file checkouts ($3 == "0") are no-op. Branch switch lands on a
// different changelog tree — rebuild is the only correct response
// because incremental replay can't remove rows that lived on the
// previous branch but not the new one (D-18).
func renderPostCheckoutHook(pqlPath string) string {
	return `# --- pql plan rebuild (post-checkout) ---
# Auto-installed by pql init. On branch checkout (third arg == 1),
# rebuild pql.db from the changelog so the local cache reflects the
# new branch's planning state. File-level checkouts ($3 == 0) are
# skipped.
if [ "$3" = "1" ]; then
    echo "pql: rebuilding planning database from changelog..." >&2
    ` + shellQuote(pqlPath) + ` plan rebuild >/dev/null 2>&1 || true
fi
# --- end pql ---
`
}

const pqlPostRewriteMarker = "# --- pql plan rebuild (post-rewrite) ---"

// renderPostRewriteHook fires after rebase or `git commit --amend`.
// Both can rewrite the changelog history visible to this clone, so
// rebuild is the only safe response (D-18).
func renderPostRewriteHook(pqlPath string) string {
	return `# --- pql plan rebuild (post-rewrite) ---
# Auto-installed by pql init. Fires after rebase / amend, both of
# which can rewrite the local view of the changelog. Rebuild is
# safe and deterministic; incremental replay would miss removals.
echo "pql: rebuilding planning database after rewrite..." >&2
` + shellQuote(pqlPath) + ` plan rebuild >/dev/null 2>&1 || true
# --- end pql ---
`
}

// shellQuote single-quotes a path for safe inclusion in a /bin/sh
// script. Single quotes inside the path are escaped via the standard
// `'\''` sequence.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// resolveHooksPath returns the directory git uses for hooks in the
// repo at dir. Honors core.hooksPath when set (often pointing at a
// tracked directory like .githooks/ that the project shares across
// clones); falls back to .git/hooks/ otherwise. Relative values from
// git config are resolved against the repo root, matching how git
// itself interprets them.
func resolveHooksPath(dir string) string {
	cmd := exec.Command("git", "-C", dir, "config", "--get", "core.hooksPath") //nolint:gosec // G204: dir is the resolved repo root, args are constants
	out, err := cmd.Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(dir, p)
			}
			return p
		}
	}
	return filepath.Join(dir, ".git", "hooks")
}

// resolvePqlPath returns the absolute path of the running pql binary
// for use in generated hook scripts. Falls back to the bare name "pql"
// if resolution fails — the hook then relies on PATH (the prior
// behaviour).
func resolvePqlPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "pql"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

func ensurePlanImportHook(dir string) initHookStat {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return initHookStat{Skipped: "not a git repository"}
	}

	hookDir := filepath.Join(dir, ".pql", "hooks")
	hookPath := filepath.Join(hookDir, "post-merge")
	stat := initHookStat{Path: hookPath}

	if err := os.MkdirAll(hookDir, 0o750); err != nil {
		stat.Skipped = "mkdir: " + err.Error()
		return stat
	}

	body := renderPostMergeHook(resolvePqlPath())

	// See ensurePlanExportHook — always (re-)plant the shim.
	defer ensureGitHookShim(dir, "post-merge")

	existing, err := os.ReadFile(hookPath) //nolint:gosec // G304: known hook path
	if err == nil {
		stat.Existed = true
		if strings.Contains(string(existing), pqlPostMergeMarker) {
			return stat
		}
		var buf bytes.Buffer
		buf.WriteString(body)
		buf.WriteByte('\n')
		buf.Write(existing)
		if err := os.WriteFile(hookPath, buf.Bytes(), 0o750); err != nil { //nolint:gosec // G306: hook must be executable
			stat.Skipped = "write: " + err.Error()
			return stat
		}
		stat.Installed = true
		return stat
	}

	content := "#!/bin/sh\n" + body
	if err := os.WriteFile(hookPath, []byte(content), 0o750); err != nil { //nolint:gosec // G306: hook must be executable
		stat.Skipped = "write: " + err.Error()
		return stat
	}
	stat.Installed = true
	return stat
}

// hookEndMarker bookends every pql-managed hook block. installNamedHook
// uses the (start-marker, end-marker) pair to locate and replace its
// own block while preserving any user customizations outside it.
const hookEndMarker = "# --- end pql ---"

// installNamedHook factors the four hooks' shared shape: ensure
// .pql/hooks/<name>, replace the pql-managed block in-place if it
// already exists (preserves any user content outside the block),
// prepend it if the file exists without a block, or create the file
// from scratch. Always (re-)plants the matching git hook shim.
func installNamedHook(dir, name, marker, body string) initHookStat {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return initHookStat{Skipped: "not a git repository"}
	}
	hookDir := filepath.Join(dir, ".pql", "hooks")
	hookPath := filepath.Join(hookDir, name)
	stat := initHookStat{Path: hookPath}
	if err := os.MkdirAll(hookDir, 0o750); err != nil {
		stat.Skipped = "mkdir: " + err.Error()
		return stat
	}
	defer ensureGitHookShim(dir, name)

	existing, err := os.ReadFile(hookPath) //nolint:gosec // G304: known hook path
	if err == nil {
		stat.Existed = true
		updated, replaced := replaceHookBlock(string(existing), marker, body)
		if !replaced {
			// File exists but our block isn't there — prepend.
			updated = body + "\n" + string(existing)
		}
		if updated == string(existing) {
			return stat
		}
		if err := os.WriteFile(hookPath, []byte(updated), 0o750); err != nil { //nolint:gosec // G306: hook must be executable
			stat.Skipped = "write: " + err.Error()
			return stat
		}
		stat.Installed = true
		return stat
	}
	content := "#!/bin/sh\n" + body
	if err := os.WriteFile(hookPath, []byte(content), 0o750); err != nil { //nolint:gosec // G306: hook must be executable
		stat.Skipped = "write: " + err.Error()
		return stat
	}
	stat.Installed = true
	return stat
}

// replaceHookBlock locates the (marker … hookEndMarker) span in
// existing content and replaces it with newBlock. Returns
// (updatedContent, true) on success; (existing, false) if the start
// marker isn't found. End marker without start marker is treated as
// "no block" — the caller falls back to prepend.
func replaceHookBlock(existing, marker, newBlock string) (string, bool) {
	startIdx := strings.Index(existing, marker)
	if startIdx < 0 {
		return existing, false
	}
	tail := existing[startIdx:]
	endIdx := strings.Index(tail, hookEndMarker)
	if endIdx < 0 {
		return existing, false
	}
	endIdx += len(hookEndMarker)
	// Consume the trailing newline if present so we don't accumulate
	// blank lines on every replacement.
	if endIdx < len(tail) && tail[endIdx] == '\n' {
		endIdx++
	}
	return existing[:startIdx] + newBlock + existing[startIdx+endIdx:], true
}

func ensurePostCheckoutHook(dir string) initHookStat {
	return installNamedHook(dir, "post-checkout", pqlPostCheckoutMarker,
		renderPostCheckoutHook(resolvePqlPath()))
}

func ensurePostRewriteHook(dir string) initHookStat {
	return installNamedHook(dir, "post-rewrite", pqlPostRewriteMarker,
		renderPostRewriteHook(resolvePqlPath()))
}

func ensureGitHookShim(dir, hookName string) {
	shimPath := filepath.Join(resolveHooksPath(dir), hookName)
	marker := "# pql: source .pql/hooks/" + hookName
	// Guard with -f so a clone that hasn't run pql init yet doesn't
	// break commits/merges with a missing-source error.
	sourceLine := `_pql_hook="$(git rev-parse --show-toplevel)/.pql/hooks/` + hookName + `"; [ -f "$_pql_hook" ] && . "$_pql_hook"`

	existing, err := os.ReadFile(shimPath) //nolint:gosec // G304: known git hook path
	if err == nil {
		if strings.Contains(string(existing), marker) {
			return
		}
		var buf bytes.Buffer
		lines := strings.SplitN(string(existing), "\n", 2)
		if strings.HasPrefix(lines[0], "#!") {
			buf.WriteString(lines[0])
			buf.WriteByte('\n')
			buf.WriteString(marker)
			buf.WriteByte('\n')
			buf.WriteString(sourceLine)
			buf.WriteByte('\n')
			if len(lines) > 1 {
				buf.WriteString(lines[1])
			}
		} else {
			buf.WriteString(marker)
			buf.WriteByte('\n')
			buf.WriteString(sourceLine)
			buf.WriteByte('\n')
			buf.Write(existing)
		}
		_ = os.WriteFile(shimPath, buf.Bytes(), 0o750) //nolint:gosec // G306: hook must be executable
		return
	}

	shim := "#!/bin/sh\n" + marker + "\n" + sourceLine + "\n"
	_ = os.MkdirAll(filepath.Dir(shimPath), 0o750)
	_ = os.WriteFile(shimPath, []byte(shim), 0o750) //nolint:gosec // G306: hook must be executable
}

// resolveInitSkillsRoot is init's variant of resolveSkillsRoot. Init
// already knows the project dir, so it doesn't need vault discovery.
// Returns user-scope when any bundled skill is already installed
// there; otherwise project-scope.
func resolveInitSkillsRoot(projectDir string) (root, scope string) {
	if uRoot, err := userSkillsRoot(); err == nil && hasAnyInstalled(uRoot) {
		return uRoot, scopeUser
	}
	return filepath.Join(projectDir, skillsRelPath), scopeProject
}

// initSkillStep handles the --with-skill flag for every bundled skill.
// Returns one stat per skill in Bundled order. Resolves scope the same
// way 'pql skill install' does: user-scope wins if anything's already
// installed there, otherwise falls back to project-scope. mode==prompt
// is TTY-aware: prompt if stdin is a terminal, otherwise behave as
// mode==no (silent skip).
func initSkillStep(dir, withFlag string, in io.Reader, prompt io.Writer) ([]initSkillStat, error) {
	root, scope := resolveInitSkillsRoot(dir)
	statuses, err := skill.InspectAll(root)
	if err != nil {
		return nil, err
	}
	for _, st := range statuses {
		st.Scope = scope
	}

	out := make([]initSkillStat, 0, len(statuses))
	for _, st := range statuses {
		stat := initSkillStat{
			Name:  st.Name,
			Scope: scope,
			Mode:  withFlag,
			State: string(st.State),
			Path:  st.Path,
		}
		if st.OnDisk != nil {
			stat.Hash = st.OnDisk.Hash
		}
		out = append(out, stat)
	}

	switch withFlag {
	case "no":
		for i := range out {
			out[i].Note = "--with-skill=no; skill install untouched"
		}
		return out, nil

	case "yes":
		// Force per-skill except for Modified/Unknown — those still
		// require an explicit `pql skill install --force`. Yes here
		// means "install missing/stale and refresh current"; we do
		// not silently overwrite hand-edits.
		for i, st := range statuses {
			if st.State == skill.StateModified || st.State == skill.StateUnknown {
				out[i].Mode = modePreserved
				out[i].Note = "skill is " + string(st.State) + "; preserve user content. Use `pql skill install --force` to overwrite."
				continue
			}
			s := skill.ByName(st.Name)
			updated, err := s.Install(root, false)
			if err != nil {
				return out, err
			}
			out[i].State = string(updated.State)
			out[i].Hash = updated.OnDisk.Hash
			out[i].Note = "installed (--with-skill=yes)"
		}
		return out, nil

	case "prompt":
		if !isTerminal(in) {
			for i := range out {
				out[i].Mode = "prompt-skipped-no-tty"
				out[i].Note = "stdin is not a TTY; --with-skill=prompt deferred"
			}
			return out, nil
		}

		// Aggregate prompt: count skills that need work. If none,
		// say so once and exit. Otherwise, one prompt covers the
		// suite — per-skill prompts are noisy and the answer is
		// usually the same.
		var pending []string
		for _, st := range statuses {
			switch st.State {
			case skill.StateMissing, skill.StateStale:
				pending = append(pending, st.Name)
			}
		}
		if len(pending) == 0 {
			for i, st := range statuses {
				switch st.State {
				case skill.StateCurrent:
					out[i].Mode = modePreserved
					out[i].Note = "skill is already current; no action needed"
				case skill.StateModified, skill.StateUnknown:
					out[i].Mode = modePreserved
					out[i].Note = "skill is " + string(st.State) + "; preserve user content. Use `pql skill install --force` to overwrite."
				}
			}
			return out, nil
		}

		question := fmt.Sprintf("Install/update %d pql skill(s) at %s? (%s)",
			len(pending), root, strings.Join(pending, ", "))
		yes, err := promptYesNo(in, prompt, question, true)
		if err != nil {
			return out, err
		}
		if !yes {
			for i := range out {
				out[i].Mode = "prompt-declined"
				out[i].Note = "user declined skill install"
			}
			return out, nil
		}
		for i, st := range statuses {
			switch st.State {
			case skill.StateCurrent:
				out[i].Mode = modePreserved
				out[i].Note = "already current"
			case skill.StateModified, skill.StateUnknown:
				out[i].Mode = modePreserved
				out[i].Note = "skill is " + string(st.State) + "; preserve user content. Use `pql skill install --force` to overwrite."
			case skill.StateMissing, skill.StateStale:
				s := skill.ByName(st.Name)
				updated, err := s.Install(root, st.State == skill.StateStale)
				if err != nil {
					return out, err
				}
				out[i].Mode = "prompt-accepted"
				out[i].State = string(updated.State)
				out[i].Hash = updated.OnDisk.Hash
			}
		}
		return out, nil

	default:
		return out, fmt.Errorf("--with-skill: invalid value %q (want yes|no|prompt)", withFlag)
	}
}

// isTerminal reports whether r is a *os.File that backs a terminal. Used
// to gate interactive prompts so non-TTY invocations (CI, pipes) skip
// silently rather than hanging on stdin.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// promptYesNo writes question to w, reads a y/n line from r, and returns
// the parsed answer. Empty input returns defaultYes. Uses bufio so
// multi-character input is consumed cleanly.
func promptYesNo(r io.Reader, w io.Writer, question string, defaultYes bool) (bool, error) {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	if _, err := fmt.Fprintf(w, "%s %s ", question, hint); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		// EOF on empty input; honour default.
		return defaultYes, nil
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	switch answer {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	}
	return false, fmt.Errorf("unrecognised answer %q (expected y or n)", answer)
}

var requiredPermissions = []string{"Bash(pql)", "Bash(pql *)"}

func ensurePqlPermissions(dir string) initPermissions {
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	stat := initPermissions{Path: settingsPath}

	data, err := os.ReadFile(settingsPath) //nolint:gosec // G304: project settings file
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return stat
		}
		data = []byte("{}")
	}
	stat.Existed = !errors.Is(err, os.ErrNotExist)

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return stat
	}

	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = make(map[string]any)
		settings["permissions"] = perms
	}

	allowRaw, _ := perms["allow"].([]any)
	existing := make(map[string]bool)
	for _, v := range allowRaw {
		if s, ok := v.(string); ok {
			existing[s] = true
		}
	}

	var added []string
	for _, perm := range requiredPermissions {
		if !existing[perm] {
			allowRaw = append(allowRaw, perm)
			added = append(added, perm)
		}
	}

	if len(added) == 0 {
		return stat
	}

	perms["allow"] = allowRaw
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return stat
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil { //nolint:gosec // G301: .claude/ is committed
		return stat
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil { //nolint:gosec // G306: settings file is committed
		return stat
	}
	stat.Added = added
	return stat
}

// ensureChangelogDirs creates .pql/changelog/<table>/ for every
// replicated planning table and seeds each with a 0000-schema.sql
// containing the full pql.db schema (CREATE TABLE IF NOT EXISTS).
// The schema file is the same in every directory because CREATE
// statements are idempotent and a self-describing replay (any
// SQLite tool can replay one directory standalone) doesn't pay if
// it can't re-create the table set on its own. Per D-15.
func ensureChangelogDirs(dir string) initChangelogStat {
	root := filepath.Join(dir, ".pql", "changelog")
	tables := []string{"tickets", "ticket_deps", "ticket_labels", "ticket_history"}
	stat := initChangelogStat{Root: root}
	for _, table := range tables {
		tableDir := filepath.Join(root, table)
		if err := os.MkdirAll(tableDir, 0o755); err != nil { //nolint:gosec // G301: changelog dirs are committed
			stat.Skipped = "mkdir " + tableDir + ": " + err.Error()
			return stat
		}
		schemaPath := filepath.Join(tableDir, "0000-schema.sql")
		if _, err := os.Stat(schemaPath); err == nil {
			continue
		}
		body := "-- Auto-generated by pql init. CREATE TABLE statements\n" +
			"-- for the planning schema; per-table dir keeps the changelog\n" +
			"-- self-describing per D-15. CREATE TABLE IF NOT EXISTS is\n" +
			"-- idempotent so running schema files from each directory in\n" +
			"-- replay order is harmless.\n" +
			"--\n" +
			"-- Importer parses the markers below to detect schema drift\n" +
			"-- between the producing pql version and the local one — a\n" +
			"-- bumped canonical_version means projection rules changed\n" +
			"-- and replay must refuse rather than silently corrupt state.\n" +
			"-- pql:created_by: " + version.Version + "\n" +
			"-- pql:canonical_version: " + strconv.Itoa(planning.CanonicalVersion) + "\n\n" +
			planning.Schema()
		if err := os.WriteFile(schemaPath, []byte(body), 0o644); err != nil { //nolint:gosec // G306: schema file is committed to git
			stat.Skipped = "write " + schemaPath + ": " + err.Error()
			return stat
		}
		stat.TablesSeeded = append(stat.TablesSeeded, table)
	}
	return stat
}

// ensureGitAttributes appends `.pql/changelog/*.sql merge=union` to
// .gitattributes if not already present. Per D-18: same-line
// conflicts on changelog files (rare; updated_at distinguishes lines)
// resolve as union — both sides land, the inline LWW guard does the
// actual conflict resolution at replay time.
func ensureGitAttributes(dir string) initGitAttribute {
	path := filepath.Join(dir, ".gitattributes")
	stat := initGitAttribute{Path: path}
	const line = ".pql/changelog/*.sql merge=union"

	existing, err := os.ReadFile(path) //nolint:gosec // G304: known file
	if err == nil {
		stat.Existed = true
		if strings.Contains(string(existing), line) {
			return stat
		}
		var buf bytes.Buffer
		buf.Write(existing)
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			buf.WriteByte('\n')
		}
		buf.WriteString(line + "\n")
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil { //nolint:gosec // G306: committed file
			stat.Skipped = "write: " + err.Error()
			return stat
		}
		stat.Appended = true
		return stat
	}
	if !os.IsNotExist(err) {
		stat.Skipped = "stat: " + err.Error()
		return stat
	}
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil { //nolint:gosec // G306: committed file
		stat.Skipped = "write: " + err.Error()
		return stat
	}
	stat.Appended = true
	return stat
}

// ensureDecisionsSynced parses decisions/*.md and upserts every
// record into pql.db's decisions + decision_refs tables. Without
// this step pql init leaves the markdown-sourced half of the schema
// (D-8) empty until the user runs `pql decisions sync` themselves.
//
// resolveDQRDir reads cfg.DQRDir from the vault's .pql/config.yaml (if
// present) and joins it onto dir. Falls back to the legacy `decisions/`
// when neither a config file nor the new default `governance/` directory
// exist but a legacy `decisions/` tree does. Returns the configured
// (or default) path even if it doesn't exist on disk yet — caller stats
// for existence and decides what to do.
func resolveDQRDir(dir string) string {
	dqr := "governance"
	cfgPath := filepath.Join(dir, ".pql", "config.yaml")
	if body, err := os.ReadFile(cfgPath); err == nil { //nolint:gosec // G304: known config path
		// Lightweight yaml peek — full config.Load isn't worth the import
		// cycle from cli/init.go's tight loop.
		for _, line := range strings.Split(string(body), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "dqr_dir:") {
				v := strings.TrimSpace(strings.TrimPrefix(t, "dqr_dir:"))
				v = strings.Trim(v, `"'`)
				if v != "" {
					dqr = v
				}
				break
			}
		}
	}
	primary := filepath.Join(dir, dqr)
	if _, err := os.Stat(primary); err == nil { //nolint:gosec // G703: dqr_dir is config, not external user input
		return primary
	}
	legacy := filepath.Join(dir, "decisions")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return primary
}

// Skipped silently when the repo has no DQR directory — not every pql
// vault carries decisions; tickets-only setups are valid.
//
// DQR root resolution: honours the configured cfg.DQRDir (default
// `governance`, per D-21) and falls back to the legacy `decisions/`
// if only the legacy layout exists.
func ensureDecisionsSynced(ctx context.Context, dir string) initDecisionsSync {
	dqrDir := resolveDQRDir(dir)
	info, err := os.Stat(dqrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return initDecisionsSync{Skipped: "no DQR directory"}
		}
		return initDecisionsSync{Skipped: "stat DQR root: " + err.Error()}
	}
	if !info.IsDir() {
		return initDecisionsSync{Skipped: "DQR root is not a directory"}
	}

	pdb, err := planning.Open(ctx, dir)
	if err != nil {
		return initDecisionsSync{Skipped: "open pql.db: " + err.Error()}
	}
	defer func() { _ = pdb.Close() }()

	res, err := repo.SyncDecisions(ctx, pdb.SQL(), dqrDir, dir)
	if err != nil {
		return initDecisionsSync{Skipped: "sync: " + err.Error()}
	}
	return initDecisionsSync{
		Synced: res.Synced,
		Refs:   res.Refs,
		Broken: res.Broken,
	}
}

// autoImportPlan populates pql.db from whatever bootstrap artefact
// the repo carries. Preference order, per T-23:
//
//  1. .pql/changelog/ — the canonical replication format (D-15).
//  2. .pql/pql-plan.json — pre-T-19 snapshot. Imported via the
//     legacy repo.Import path; pql.db ends up populated, then a
//     `pql plan export` run will produce the changelog and the
//     legacy file can be removed manually.
//  3. legacy root-level pql-plan.json (very early form).
//
// Returns an empty initPlanImport with Skipped set when nothing
// matches, so a fresh repo with no state in any form opens cleanly.
func autoImportPlan(ctx context.Context, dir string) initPlanImport {
	changelogRoot := filepath.Join(dir, ".pql", "changelog")
	if hasChangelogData(changelogRoot) {
		return autoReplayChangelog(ctx, dir, changelogRoot)
	}

	snapPath := filepath.Join(dir, defaultSnapshotFile)
	data, err := os.ReadFile(snapPath) //nolint:gosec // G304: known snapshot file
	if err != nil {
		legacyPath := filepath.Join(dir, "pql-plan.json")
		data, err = os.ReadFile(legacyPath) //nolint:gosec // G304: legacy snapshot file
		if err != nil {
			return initPlanImport{Skipped: "no changelog or pql-plan.json found"}
		}
		snapPath = legacyPath
	}
	return autoImportLegacySnapshot(ctx, dir, snapPath, data)
}

// hasChangelogData reports whether the changelog directory holds at
// least one data file (a *.sql that isn't a schema fixture). Empty
// directories or directories with only schema files don't count —
// they look like a freshly-init'd repo with no committed state yet.
func hasChangelogData(root string) bool {
	tables, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, t := range tables {
		if !t.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, t.Name()))
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".sql") {
				continue
			}
			if strings.HasSuffix(name, "-schema.sql") || strings.HasSuffix(name, "schema.sql") {
				continue
			}
			return true
		}
	}
	return false
}

func autoReplayChangelog(ctx context.Context, dir, root string) initPlanImport {
	pdb, err := planning.Open(ctx, dir)
	if err != nil {
		return initPlanImport{File: root, Skipped: "open pql.db: " + err.Error()}
	}
	defer func() { _ = pdb.Close() }()

	res, err := changelog.Import(ctx, pdb.SQL(), dir)
	if err != nil {
		return initPlanImport{File: root, Skipped: "changelog replay: " + err.Error()}
	}
	return initPlanImport{
		File:     root,
		Imported: true,
		Count:    res.StatementsRun,
	}
}

func autoImportLegacySnapshot(ctx context.Context, dir, snapPath string, data []byte) initPlanImport {
	var snap repo.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return initPlanImport{File: snapPath, Skipped: "parse error: " + err.Error()}
	}
	if len(snap.Decisions) == 0 && len(snap.Tickets) == 0 {
		return initPlanImport{File: snapPath, Skipped: "snapshot is empty"}
	}
	pdb, err := planning.Open(ctx, dir)
	if err != nil {
		return initPlanImport{File: snapPath, Skipped: "open pql.db: " + err.Error()}
	}
	defer func() { _ = pdb.Close() }()
	if err := repo.Import(ctx, pdb.SQL(), &snap); err != nil {
		return initPlanImport{File: snapPath, Skipped: "import: " + err.Error()}
	}
	return initPlanImport{
		File:     snapPath,
		Imported: true,
		Count:    len(snap.Decisions) + len(snap.Tickets),
	}
}
