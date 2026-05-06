// Package skill embeds the Claude Code skills that ship alongside the
// binary, hashes them for drift detection, and installs/uninstalls
// them at a target root directory (typically <vault>/.claude/skills/
// or ~/.claude/skills/).
//
// Each bundled skill installs into its own subdirectory under the
// root: <root>/<skill-name>/. The subdirectory holds the SKILL.md,
// any reference files (references/*), and a small lock file:
//
//   SKILL.md            — primary skill content
//   references/...      — optional bundled reference docs
//   .pql-install.json   — lock file recording version + bundle hash
//
// The lock pairs with the on-disk hash to distinguish pristine-but-
// stale installs from hand-edited ones — the only reason the State
// machine has four live states (missing / current / stale / modified)
// instead of two.
package skill

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/postmeridiem/pql/internal/version"
)

// Filename constants — centralised so tests and callers agree.
const (
	SkillFile = "SKILL.md"
	LockFile  = ".pql-install.json"
)

//go:embed SKILL.md
var pqlSkillMd string

//go:embed all:clean-house
var cleanHouseFS embed.FS

// Bundled is the deterministic, ordered list of skills shipped with
// this binary. Order affects output ordering only — functionally the
// slice is treated as a set.
var Bundled = buildBundled()

func buildBundled() []*Skill {
	var skills []*Skill

	// pql — single-file skill, embedded directly.
	skills = append(skills, &Skill{
		Name:  "pql",
		files: map[string]string{SkillFile: pqlSkillMd},
	})

	// clean-house — directory bundle, walk the embed.FS.
	cleanHouse := &Skill{
		Name:  "clean-house",
		files: map[string]string{},
	}
	if err := fs.WalkDir(cleanHouseFS, "clean-house", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := cleanHouseFS.ReadFile(path)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "clean-house/")
		cleanHouse.files[rel] = string(data)
		return nil
	}); err != nil {
		panic(fmt.Sprintf("skill: walk clean-house bundle: %v", err))
	}
	skills = append(skills, cleanHouse)

	return skills
}

// ByName returns the bundled skill with the given name, or nil.
func ByName(name string) *Skill {
	for _, s := range Bundled {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Skill is a bundled skill with the operations to inspect, install,
// and uninstall it relative to a root directory.
type Skill struct {
	Name  string
	files map[string]string // relative path → content; always contains SKILL.md
}

// Files returns the bundle's relative paths in deterministic order.
func (s *Skill) Files() []string {
	paths := make([]string, 0, len(s.files))
	for p := range s.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// FileContent returns the embedded content of a file in the bundle.
// Returns "" if the path is not part of the bundle.
func (s *Skill) FileContent(rel string) string { return s.files[rel] }

// Hash returns the bundle hash — sha256 over the sorted (path,
// content) pairs. Used to detect drift between embedded and on-disk.
func (s *Skill) Hash() string { return hashBundle(s.files) }

// State is the install-location's status relative to this binary.
type State string

const (
	StateMissing  State = "missing"  // no SKILL.md at the target
	StateCurrent  State = "current"  // matches the binary's embedded skill
	StateStale    State = "stale"    // pristine install, just older binary wrote it
	StateModified State = "modified" // hand-edited since install — preserve
	StateUnknown  State = "unknown"  // SKILL.md present but no lock file
)

// Snapshot captures a skill at a point in time. Hash is "sha256:<hex>".
type Snapshot struct {
	Version     string `json:"version"`
	Hash        string `json:"hash"`
	InstalledAt string `json:"installed_at,omitempty"`
}

// Status is the full report for a single skill.
type Status struct {
	Name      string    `json:"name"`
	State     State     `json:"state"`
	Path      string    `json:"path"` // install directory
	Installed *Snapshot `json:"installed,omitempty"`
	OnDisk    *Snapshot `json:"on_disk,omitempty"`
	Embedded  Snapshot  `json:"embedded"`
}

// Embedded returns the snapshot for this skill as bundled in the
// running binary.
func (s *Skill) Embedded() Snapshot {
	return Snapshot{
		Version: version.Version,
		Hash:    s.Hash(),
	}
}

// installDir returns the directory this skill installs into beneath
// the given root. Creating it is Install's job.
func (s *Skill) installDir(root string) string {
	return filepath.Join(root, s.Name)
}

// Inspect reports the state of this skill at <root>/<name>/. Missing
// directory or SKILL.md is StateMissing (not an error).
func (s *Skill) Inspect(root string) (*Status, error) {
	dir := s.installDir(root)
	st := &Status{
		Name:     s.Name,
		Path:     dir,
		Embedded: s.Embedded(),
	}

	skillPath := filepath.Join(dir, SkillFile)
	if _, err := os.Stat(skillPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			st.State = StateMissing
			return st, nil
		}
		return nil, fmt.Errorf("skill: stat %s: %w", skillPath, err)
	}

	onDisk, err := s.readOnDisk(dir)
	if err != nil {
		return nil, err
	}
	st.OnDisk = &Snapshot{Hash: hashBundle(onDisk)}

	lockData, err := os.ReadFile(filepath.Join(dir, LockFile)) //nolint:gosec // G304: lock path is derived from caller-controlled root
	switch {
	case errors.Is(err, os.ErrNotExist):
		st.State = StateUnknown
		return st, nil
	case err != nil:
		return nil, fmt.Errorf("skill: read lock: %w", err)
	}

	var lock Snapshot
	if err := json.Unmarshal(lockData, &lock); err != nil {
		st.State = StateUnknown
		return st, nil
	}
	st.Installed = &lock

	switch {
	case st.Installed.Hash != st.OnDisk.Hash:
		st.State = StateModified
	case st.OnDisk.Hash != st.Embedded.Hash:
		st.State = StateStale
	default:
		st.State = StateCurrent
	}
	return st, nil
}

// readOnDisk loads only the files this bundle expects to find at dir.
// Missing files are treated as empty (their content contributes the
// empty hash slot) — that way a partial install reads as Modified.
func (s *Skill) readOnDisk(dir string) (map[string]string, error) {
	out := make(map[string]string, len(s.files))
	for rel := range s.files {
		data, err := os.ReadFile(filepath.Join(dir, rel)) //nolint:gosec // G304: rel comes from the embedded bundle, not user input
		if errors.Is(err, os.ErrNotExist) {
			out[rel] = ""
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("skill: read %s: %w", rel, err)
		}
		out[rel] = string(data)
	}
	return out, nil
}

// Install writes the bundle and a fresh lock file to <root>/<name>/.
// State=modified and state=unknown require force=true — otherwise
// Install refuses to clobber hand-edits.
func (s *Skill) Install(root string, force bool) (*Status, error) {
	current, err := s.Inspect(root)
	if err != nil {
		return nil, err
	}
	if !force {
		switch current.State {
		case StateModified:
			return current, &ErrRefusedOverwrite{Name: s.Name, State: current.State,
				Reason: "skill has been hand-edited since install"}
		case StateUnknown:
			return current, &ErrRefusedOverwrite{Name: s.Name, State: current.State,
				Reason: "skill present but wasn't installed by pql"}
		}
	}

	dir := s.installDir(root)
	for _, rel := range s.Files() {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { //nolint:gosec // G301: committed dir needs group/other read
			return nil, fmt.Errorf("skill: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(s.files[rel]), 0o644); err != nil { //nolint:gosec // G306: committed file needs group/other read
			return nil, fmt.Errorf("skill: write %s: %w", rel, err)
		}
	}

	lock := Snapshot{
		Version:     version.Version,
		Hash:        s.Hash(),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	lockData, _ := json.MarshalIndent(lock, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, LockFile), append(lockData, '\n'), 0o644); err != nil { //nolint:gosec // G306: committed file needs group/other read
		return nil, fmt.Errorf("skill: write lock: %w", err)
	}

	return s.Inspect(root)
}

// Uninstall removes this skill's files and lock from <root>/<name>/.
// Missing files are not errors. The directory is removed if empty
// after; non-empty dirs (user dropped unrelated files in) are left
// alone.
func (s *Skill) Uninstall(root string) error {
	dir := s.installDir(root)
	var errs []error
	// Remove every file we ship plus the lock.
	for _, rel := range append(s.Files(), LockFile) {
		p := filepath.Join(dir, rel)
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("skill: remove %s: %w", p, err))
		}
	}
	// Best-effort empty-dir cleanup, walking up subdirs we created
	// (e.g. references/) before the install root.
	cleanupEmptyDirs(dir)
	return errors.Join(errs...)
}

// cleanupEmptyDirs removes dir and any of its empty subdirs, walking
// bottom-up. Stops at the first non-empty directory; never deletes
// anything outside dir.
func cleanupEmptyDirs(dir string) {
	// Walk children first.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			cleanupEmptyDirs(filepath.Join(dir, e.Name()))
		}
	}
	_ = os.Remove(dir) // removes only if empty
}

// InspectAll reports state for every bundled skill.
func InspectAll(root string) ([]*Status, error) {
	out := make([]*Status, 0, len(Bundled))
	for _, s := range Bundled {
		st, err := s.Inspect(root)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// InstallAll installs every bundled skill. force applies to all of
// them. Returns the post-install statuses; on first refusal, returns
// the partial slice and the ErrRefusedOverwrite (callers decide
// whether to retry with force).
func InstallAll(root string, force bool) ([]*Status, error) {
	out := make([]*Status, 0, len(Bundled))
	for _, s := range Bundled {
		st, err := s.Install(root, force)
		out = append(out, st)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// UninstallAll removes every bundled skill from root. Errors are
// joined; idempotent.
func UninstallAll(root string) error {
	var errs []error
	for _, s := range Bundled {
		if err := s.Uninstall(root); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ErrRefusedOverwrite is returned by Install when it would clobber
// user work without force=true.
type ErrRefusedOverwrite struct {
	Name   string
	State  State
	Reason string
}

func (e *ErrRefusedOverwrite) Error() string {
	return fmt.Sprintf("skill %q: refusing to overwrite (%s): %s; pass force=true to overwrite",
		e.Name, e.State, e.Reason)
}

// hashBytes produces the "sha256:<hex>" form used on disk.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// hashBundle hashes the bundle as a deterministic concatenation of
// sorted "path\x00content\x00" pairs. Stable across Go versions; not
// affected by map iteration order.
func hashBundle(files map[string]string) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(files[p]))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
