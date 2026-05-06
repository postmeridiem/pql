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

// Scope labels reported in JSON output and used internally to
// distinguish user-level (~/.claude/skills/) from project-level
// (<vault>/.claude/skills/) installs.
const (
	scopeUser    = "user"
	scopeProject = "project"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the Claude Code skills bundled with pql",
		Long: `Install, update, or inspect the Claude Code skills that ship embedded
in the pql binary. Each skill installs into its own subdirectory under
.claude/skills/.

Two scopes:
  user    — ~/.claude/skills/, applies across every project
  project — <vault>/.claude/skills/, scoped to one repo

Pass --user to target user-scope explicitly. Otherwise, scope is
auto-resolved: user-scope wins when any bundled skill already lives
there (you opted in once via 'pql skill install --user'); project-
scope is the default for fresh installs.

When auto-resolving to user-scope, an existing project-scope install
is tidied up — but only if it's pristine (current/stale/missing).
Hand-edited (modified) or unknown project-scope installs are left
alone; remove them yourself or pass --force to project-scope install.

Each install carries a .pql-install.json lock file with version +
bundle hash. Subsequent commands compare against the binary's
embedded bundle to detect:

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
			root, scope, err := resolveSkillsRoot(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			statuses, err := inspectAtScope(root, scope)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatuses(cmd, statuses)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/ explicitly (skip auto-resolve)")
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var (
		user  bool
		force bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install (or update) every bundled skill at the resolved scope",
		Long: `Write each bundled skill's files plus a lock file. Idempotent: if
the target already matches the embedded content, the files are untouched.

Without --user, install resolves to whichever scope is already in use:
user-scope if any bundled skill lives there, project-scope otherwise.
The first time you opt into user-scope must be done via --user.

When auto-resolving to user-scope, a pristine project-scope install
gets tidied up automatically. A hand-edited (modified) project-scope
install is preserved with a note — remove it yourself if you no
longer want it.

If a target skill is state=modified or state=unknown, install refuses
to overwrite unless --force is passed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, scope, err := resolveSkillsRoot(cmd, user)
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			rawStatuses, err := skill.InstallAll(root, force)
			statuses := tagScope(rawStatuses, scope)
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

			// When we auto-resolved to user-scope, tidy any safely-
			// removable project-scope leftovers. Skip when --user was
			// explicit (the user is just managing user-scope) or when
			// scope is project (no cross-scope cleanup applies).
			if !user && scope == scopeUser {
				cleanup, err := tidyProjectScope(cmd)
				if err == nil && len(cleanup) > 0 {
					statuses = append(statuses, cleanup...)
				}
			}
			return renderSkillStatuses(cmd, statuses)
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/ explicitly (skip auto-resolve)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite a modified or untracked skill")
	return cmd
}

func newSkillUninstallCmd() *cobra.Command {
	var user bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove every bundled skill from the target scope",
		Long: `Without --user, removes the project-scope install only — never the
user-scope install. To remove user-scope, pass --user explicitly.
This intentionally avoids surprise: 'pql skill uninstall' run from a
random shell shouldn't wipe out your global skill suite.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope := scopeProject
			var root string
			var err error
			if user {
				scope = scopeUser
				root, err = userSkillsRoot()
			} else {
				root, err = projectSkillsRoot(cmd)
			}
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			if err := skill.UninstallAll(root); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			rawStatuses, err := skill.InspectAll(root)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return renderSkillStatuses(cmd, tagScope(rawStatuses, scope))
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "target ~/.claude/skills/ instead of the current vault")
	return cmd
}

// resolveSkillsRoot returns the install root and its scope label.
// With user=true, always returns user-scope. Without, auto-resolves:
// user-scope wins if any bundled skill is already installed there;
// otherwise falls back to project-scope.
func resolveSkillsRoot(cmd *cobra.Command, user bool) (root, scope string, err error) {
	if user {
		root, err = userSkillsRoot()
		return root, scopeUser, err
	}

	uRoot, uErr := userSkillsRoot()
	if uErr == nil && hasAnyInstalled(uRoot) {
		return uRoot, scopeUser, nil
	}

	root, err = projectSkillsRoot(cmd)
	return root, scopeProject, err
}

// hasAnyInstalled reports whether any bundled skill is installed at
// root in any non-missing state.
func hasAnyInstalled(root string) bool {
	statuses, err := skill.InspectAll(root)
	if err != nil {
		return false
	}
	for _, st := range statuses {
		if st.State != skill.StateMissing {
			return true
		}
	}
	return false
}

func userSkillsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, skillsRelPath), nil
}

func projectSkillsRoot(cmd *cobra.Command) (string, error) {
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

// inspectAtScope is the standard "InspectAll + tag scope" sequence
// used wherever skill statuses are emitted to JSON.
func inspectAtScope(root, scope string) ([]*skill.Status, error) {
	statuses, err := skill.InspectAll(root)
	if err != nil {
		return nil, err
	}
	return tagScope(statuses, scope), nil
}

// tagScope sets Scope on every status in the slice. Returned for
// chaining; mutates in place.
func tagScope(statuses []*skill.Status, scope string) []*skill.Status {
	for _, st := range statuses {
		if st != nil {
			st.Scope = scope
		}
	}
	return statuses
}

// tidyProjectScope removes any safely-removable project-scope skill
// install (state in {current, stale, missing}). Modified or unknown
// installs are left alone; a status entry with a note is returned so
// the caller can surface what happened.
func tidyProjectScope(cmd *cobra.Command) ([]*skill.Status, error) {
	projRoot, err := projectSkillsRoot(cmd)
	if err != nil {
		// If we can't locate a project (no vault discovered, no
		// flag), there's nothing project-scoped to clean. Not an error.
		return nil, nil
	}
	rawStatuses, err := skill.InspectAll(projRoot)
	if err != nil {
		return nil, err
	}
	statuses := tagScope(rawStatuses, scopeProject)

	for _, st := range statuses {
		switch st.State {
		case skill.StateCurrent, skill.StateStale:
			s := skill.ByName(st.Name)
			if s == nil {
				continue
			}
			_ = s.Uninstall(projRoot)
		}
	}
	// Re-inspect after cleanup so the report reflects post-state.
	post, err := skill.InspectAll(projRoot)
	if err != nil {
		return nil, err
	}
	return tagScope(post, scopeProject), nil
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
