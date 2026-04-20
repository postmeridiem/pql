package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/skill"
)

// skillRelPath is the trailing path appended to either a vault root or
// the user's home to reach the install location. Mirrors the Claude
// Code skill convention documented in SKILL.md.
var skillRelPath = filepath.Join(".claude", "skills", "pql")

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the Claude Code skill bundled with pql",
		Long: `Install, update, or inspect the Claude Code skill that ships embedded
in the pql binary.

By default the skill is installed to the current vault at
<vault>/.claude/skills/pql/. Pass --user to target the per-user location
at ~/.claude/skills/pql/ instead, which applies across every project.

Install writes SKILL.md plus a .pql-install.json lock file that records
the version and SHA-256 hash of the content at install time. Subsequent
status/install calls compare that lock against the binary's embedded
content to detect four states:

  missing   — no SKILL.md at the target
  current   — matches this binary's embedded skill (no-op to reinstall)
  stale     — pristine install, just from an older binary (update freely)
  modified  — hand-edited since install (preserved unless --force)

`,
	}
	cmd.AddCommand(newSkillStatusCmd())
	cmd.AddCommand(newSkillInstallCmd())
	cmd.AddCommand(newSkillUninstallCmd())
	return cmd
}

func newSkillStatusCmd() *cobra.Command {
	var user bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report the install state of the pql skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := skillTargetDir(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			st, err := skill.Inspect(dir)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatus(cmd, st)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/pql/ instead of the current vault")
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var (
		user  bool
		force bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install (or update) the pql skill at the target directory",
		Long: `Write the embedded SKILL.md + lock file. Idempotent: if the target
already matches the embedded content, the files are untouched.

If the installed skill has been hand-edited (state=modified) or exists
but wasn't tracked by pql (state=unknown), install refuses to overwrite
unless --force is passed. That default preserves user customisations.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := skillTargetDir(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			st, err := skill.Install(dir, force)
			if err != nil {
				var refused *skill.ErrRefusedOverwrite
				if errors.As(err, &refused) {
					return &exitError{
						code: diag.Usage,
						msg:  err.Error(),
						hint: "pass --force to overwrite",
					}
				}
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatus(cmd, st)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/pql/ instead of the current vault")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite a modified or untracked SKILL.md")
	return cmd
}

func newSkillUninstallCmd() *cobra.Command {
	var user bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the pql skill from the target directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := skillTargetDir(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			if err := skill.Uninstall(dir); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			st, err := skill.Inspect(dir)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatus(cmd, st)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/pql/ instead of the current vault")
	return cmd
}

// skillTargetDir resolves the directory the subcommand operates on.
// With --user, it's <home>/.claude/skills/pql/. Without, it's
// <vault>/.claude/skills/pql/ — using vault discovery directly
// (no store/index overhead needed for this pure-filesystem operation).
func skillTargetDir(cmd *cobra.Command, user bool) (string, error) {
	if user {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("--user: locate home dir: %w", err)
		}
		return filepath.Join(home, skillRelPath), nil
	}
	vaultFlag, _ := cmd.Flags().GetString("vault")
	d, err := config.DiscoverVault(config.VaultOpts{
		Flag: vaultFlag,
		Env:  os.Getenv("PQL_VAULT"),
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(d.Path, skillRelPath), nil
}

// renderSkillStatus is the shared JSON emission path for every
// skill-subcommand success path. Returns errNoMatch when nothing is
// installed, so `pql skill status` against a never-installed vault
// cleanly maps to exit code 2.
func renderSkillStatus(cmd *cobra.Command, st *skill.Status) error {
	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	if _, err := render.RenderOne(st, rOpts); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	if st.State == skill.StateMissing {
		return errNoMatch
	}
	return nil
}
