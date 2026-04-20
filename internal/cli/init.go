package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
)

// initResult is the JSON shape `pql init` emits on stdout.
type initResult struct {
	Directory string         `json:"directory"`
	Config    initConfigStat `json:"config"`
	Gitignore initGitignore  `json:"gitignore"`
}

type initConfigStat struct {
	Path        string `json:"path"`
	Created     bool   `json:"created"`
	Overwritten bool   `json:"overwritten"`
}

type initGitignore struct {
	Path     string `json:"path,omitempty"`
	Exists   bool   `json:"exists"`
	Appended bool   `json:"appended"`
	Entry    string `json:"entry,omitempty"`
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

# Honor .gitignore files in addition to .pqlignore.
respect_gitignore: false

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
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Seed .pql.yaml in the current directory (or --vault target)",
		Long: `Write a default .pql.yaml in the current directory so pql can index this
vault with sensible defaults. If a .gitignore exists in the same directory
and doesn't already mention the .pql/ state directory, append it so the
SQLite index doesn't land in version control.

  pql init                       # write .pql.yaml here; do nothing if it exists
  pql init --force               # overwrite an existing .pql.yaml
  pql init --vault path/to/vault # initialise a different directory

Output is one JSON object describing what was created or modified. Exit 64
if .pql.yaml already exists and --force wasn't passed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := initTargetDir(cmd)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			cfgPath := filepath.Join(dir, ".pql.yaml")
			cfgStat, err := writeDefaultConfig(cfgPath, force)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}

			giPath := filepath.Join(dir, ".gitignore")
			giStat, err := ensureGitignoreEntry(giPath, ".pql/")
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			result := &initResult{Directory: dir, Config: cfgStat, Gitignore: giStat}
			if _, err := render.RenderOne(result, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing .pql.yaml")
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

func writeDefaultConfig(path string, force bool) (initConfigStat, error) {
	stat := initConfigStat{Path: path}
	_, err := os.Stat(path)
	exists := err == nil
	if exists && !force {
		return stat, fmt.Errorf("%s exists; pass --force to overwrite", path)
	}
	if err := os.WriteFile(path, []byte(defaultConfigBody), 0o644); err != nil {
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
	body, err := os.ReadFile(path)
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
			return stat, nil // already present, nothing to do
		}
	}
	if err := scanner.Err(); err != nil {
		return stat, fmt.Errorf("scan %s: %w", path, err)
	}

	// Append. Preserve a trailing newline before our entry if the file
	// doesn't end with one; emit one after.
	var buf bytes.Buffer
	buf.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(entry)
	buf.WriteByte('\n')

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return stat, fmt.Errorf("write %s: %w", path, err)
	}
	stat.Appended = true
	stat.Entry = entry
	return stat, nil
}
