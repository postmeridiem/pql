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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/repo"
	"github.com/postmeridiem/pql/internal/skill"
)

// initResult is the JSON shape `pql init` emits on stdout. Each sub-stat
// describes one of the project-state fixers init runs through.
type initResult struct {
	Directory   string           `json:"directory"`
	Config      initConfigStat   `json:"config"`
	Gitignore   initGitignore    `json:"gitignore"`
	Skill       initSkillStat    `json:"skill"`
	Permissions initPermissions  `json:"permissions"`
	PlanImport  initPlanImport   `json:"plan_import"`
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

type initSkillStat struct {
	Mode  string `json:"mode"`            // "yes" | "no" | "prompt-declined" | "prompt-accepted" | "prompt-skipped-no-tty" | "preserved"
	State string `json:"state"`           // post-action state per internal/skill
	Path  string `json:"path,omitempty"`  // SKILL.md path
	Hash  string `json:"hash,omitempty"`  // SHA-256 of installed content (when present)
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
			giStat, err := ensureGitignoreEntry(giPath, ".pql/")
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			skillStat, err := initSkillStep(dir, withSkill, cmd.InOrStdin(), cmd.OutOrStderr())
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			permStat := ensurePqlPermissions(dir)
			planStat := autoImportPlan(cmd.Context(), dir)

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			result := &initResult{
				Directory:   dir,
				Config:      cfgStat,
				Gitignore:   giStat,
				Skill:       skillStat,
				Permissions: permStat,
				PlanImport:  planStat,
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

// initSkillStep handles the --with-skill flag. Returns a stat describing
// what happened. mode==prompt is TTY-aware: prompt if stdin is a
// terminal, otherwise behave as mode==no (silent skip).
func initSkillStep(dir, withFlag string, in io.Reader, prompt io.Writer) (initSkillStat, error) {
	skillDir := filepath.Join(dir, skillRelPath)
	current, err := skill.Inspect(skillDir)
	if err != nil {
		return initSkillStat{}, err
	}

	stat := initSkillStat{
		Mode:  withFlag,
		State: string(current.State),
		Path:  current.Path,
	}
	if current.OnDisk != nil {
		stat.Hash = current.OnDisk.Hash
	}

	switch withFlag {
	case "no":
		stat.Note = "--with-skill=no; skill install untouched"
		return stat, nil

	case "yes":
		// Force=true so stale + modified are both updated. The user said
		// "yes" explicitly; respect that.
		updated, err := skill.Install(skillDir, true)
		if err != nil {
			return stat, err
		}
		stat.State = string(updated.State)
		stat.Hash = updated.OnDisk.Hash
		stat.Note = "installed (--with-skill=yes)"
		return stat, nil

	case "prompt":
		// Skip silently if not a TTY — a no-prompt environment shouldn't
		// hang waiting for input.
		if !isTerminal(in) {
			stat.Mode = "prompt-skipped-no-tty"
			stat.Note = "stdin is not a TTY; --with-skill=prompt deferred"
			return stat, nil
		}
		// Decide what to ask based on current state.
		var question string
		var defaultYes bool
		switch current.State {
		case skill.StateMissing:
			question = "Install the pql Claude Code skill at " + skillDir + "?"
			defaultYes = true
		case skill.StateStale:
			question = "Skill on disk is older than the binary's. Update it at " + skillDir + "?"
			defaultYes = true
		case skill.StateCurrent:
			stat.Mode = "preserved"
			stat.Note = "skill is already current; no action needed"
			return stat, nil
		case skill.StateModified, skill.StateUnknown:
			stat.Mode = "preserved"
			stat.Note = "skill is " + string(current.State) + "; preserve user content. Use `pql skill install --force` to overwrite."
			return stat, nil
		}
		yes, err := promptYesNo(in, prompt, question, defaultYes)
		if err != nil {
			return stat, err
		}
		if !yes {
			stat.Mode = "prompt-declined"
			stat.Note = "user declined skill install"
			return stat, nil
		}
		updated, err := skill.Install(skillDir, current.State == skill.StateStale)
		if err != nil {
			return stat, err
		}
		stat.Mode = "prompt-accepted"
		stat.State = string(updated.State)
		stat.Hash = updated.OnDisk.Hash
		return stat, nil

	default:
		return stat, fmt.Errorf("--with-skill: invalid value %q (want yes|no|prompt)", withFlag)
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

func autoImportPlan(ctx context.Context, dir string) initPlanImport {
	snapPath := filepath.Join(dir, defaultSnapshotFile)
	data, err := os.ReadFile(snapPath) //nolint:gosec // G304: known snapshot file in vault root
	if err != nil {
		return initPlanImport{Skipped: "no " + defaultSnapshotFile + " found"}
	}

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
