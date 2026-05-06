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

// skillsRelPath is the trailing path appended to either a vault root
// or the user's home to reach the install root. Each bundled skill
// installs into its own subdirectory beneath this root.
var skillsRelPath = filepath.Join(".claude", "skills")

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the Claude Code skills bundled with pql",
		Long: `Install, update, or inspect the Claude Code skills that ship embedded
in the pql binary. Each skill installs into its own subdirectory under
.claude/skills/.

By default skills install to the current vault at <vault>/.claude/skills/.
Pass --user to target the per-user location at ~/.claude/skills/ instead,
which applies across every project.

Each skill ships with a .pql-install.json lock file recording its
version + bundle hash at install time. Subsequent status/install calls
compare that against the binary's embedded bundle to detect:

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
		Short: "Report the install state of every bundled skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := skillsRoot(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			statuses, err := skill.InspectAll(root)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatuses(cmd, statuses)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/ instead of the current vault")
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var (
		user  bool
		force bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install (or update) every bundled skill at the target directory",
		Long: `Write each bundled skill's files plus a lock file. Idempotent: if
the target already matches the embedded content, the files are untouched.

If an installed skill has been hand-edited (state=modified) or exists
but wasn't tracked by pql (state=unknown), install refuses to overwrite
unless --force is passed. That default preserves user customisations.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := skillsRoot(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			statuses, err := skill.InstallAll(root, force)
			if err != nil {
				var refused *skill.ErrRefusedOverwrite
				if errors.As(err, &refused) {
					_ = renderSkillStatuses(cmd, statuses)
					return &exitError{
						code: diag.Usage,
						msg:  err.Error(),
						hint: "pass --force to overwrite",
					}
				}
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatuses(cmd, statuses)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/ instead of the current vault")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite a modified or untracked skill")
	return cmd
}

func newSkillUninstallCmd() *cobra.Command {
	var user bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove every bundled skill from the target directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := skillsRoot(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			if err := skill.UninstallAll(root); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			statuses, err := skill.InspectAll(root)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatuses(cmd, statuses)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/ instead of the current vault")
	return cmd
}

// skillsRoot resolves the install root. With --user, it's
// <home>/.claude/skills/. Without, it's <vault>/.claude/skills/.
func skillsRoot(cmd *cobra.Command, user bool) (string, error) {
	if user {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("--user: locate home dir: %w", err)
		}
		return filepath.Join(home, skillsRelPath), nil
	}
	vaultFlag, _ := cmd.Flags().GetString("vault")
	d, err := config.DiscoverVault(config.VaultOpts{
		Flag: vaultFlag,
		Env:  os.Getenv("PQL_VAULT"),
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(d.Path, skillsRelPath), nil
}

// renderSkillStatuses emits the shared JSON for every skill subcommand.
// Returns errNoMatch only when *every* skill is missing — partial
// installs aren't no-match.
func renderSkillStatuses(cmd *cobra.Command, statuses []*skill.Status) error {
	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	if _, err := render.Render(statuses, rOpts); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	allMissing := len(statuses) > 0
	for _, st := range statuses {
		if st.State != skill.StateMissing {
			allMissing = false
			break
		}
	}
	if allMissing {
		return errNoMatch
	}
	return nil
}
