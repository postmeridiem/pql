// Package skill embeds the Claude Code skill that ships alongside the
// binary, hashes it for drift detection, and installs/uninstalls it at
// a target directory (either project-level <vault>/.claude/skills/pql/
// or user-level ~/.claude/skills/pql/).
//
// The install target holds two files:
//
//   SKILL.md            — the skill itself (content varies per release)
//   .pql-install.json   — a small lock file recording which version was
//                         installed and its content hash at install time
//
// The pairing lets us tell pristine-but-stale files apart from
// hand-edited ones, which is the only reason the State machine has four
// live states (missing / current / stale / modified) instead of two.
package skill

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/postmeridiem/pql/internal/version"
)

// Filename constants — centralised so tests and callers agree.
const (
	SkillFile = "SKILL.md"
	LockFile  = ".pql-install.json"
)

//go:embed SKILL.md
var embedded string

// State is the install-location's current status relative to the binary
// we're running. Always one of the constants below.
type State string

// Drift states. The five-way split lets the CLI tell the user exactly
// what's wrong (missing/stale/modified/unknown) instead of a binary
// installed/not-installed flag. See docs/skill.md for the state machine.
const (
	StateMissing  State = "missing"  // no SKILL.md at the target
	StateCurrent  State = "current"  // matches the binary's embedded skill
	StateStale    State = "stale"    // pristine install, just older binary wrote it
	StateModified State = "modified" // hand-edited since install — preserve
	StateUnknown  State = "unknown"  // SKILL.md present but no lock file
)

// Snapshot captures a skill at a point in time. Hash is "sha256:<hex>"
// so the algorithm is explicit on disk and future bumps stay obvious.
type Snapshot struct {
	Version     string `json:"version"`
	Hash        string `json:"hash"`
	InstalledAt string `json:"installed_at,omitempty"`
}

// Status is the full report returned by Inspect. Installed is the lock
// file's record (nil if no lock present); OnDisk is the actual content
// hash right now; Embedded is what this binary would install.
type Status struct {
	State     State     `json:"state"`
	Path      string    `json:"path"`
	Installed *Snapshot `json:"installed,omitempty"`
	OnDisk    *Snapshot `json:"on_disk,omitempty"`
	Embedded  Snapshot  `json:"embedded"`
}

// Content returns the embedded SKILL.md content. Exposed for the rare
// caller that wants to render or diff it; Install is the usual entry.
func Content() string { return embedded }

// Embedded returns the snapshot for the skill baked into this binary.
func Embedded() Snapshot {
	return Snapshot{
		Version: version.Version,
		Hash:    hashBytes([]byte(embedded)),
	}
}

// Inspect reports the state at dir. dir is the directory that would
// contain SKILL.md and .pql-install.json (typically
// <vault>/.claude/skills/pql/ or <home>/.claude/skills/pql/). Non-
// existence of dir is the same as StateMissing.
func Inspect(dir string) (*Status, error) {
	skillPath := filepath.Join(dir, SkillFile)
	lockPath := filepath.Join(dir, LockFile)

	st := &Status{
		Path:     skillPath,
		Embedded: Embedded(),
	}

	contents, err := os.ReadFile(skillPath) //nolint:gosec // G304: skillPath is derived from caller-controlled dir; reading the on-disk skill is the function's purpose
	if errors.Is(err, os.ErrNotExist) {
		st.State = StateMissing
		return st, nil
	}
	if err != nil {
		return nil, fmt.Errorf("skill: read %s: %w", skillPath, err)
	}
	st.OnDisk = &Snapshot{Hash: hashBytes(contents)}

	lockData, err := os.ReadFile(lockPath) //nolint:gosec // G304: lockPath is derived from caller-controlled dir; reading the lock file is the function's purpose
	if err == nil {
		var lock Snapshot
		if err := json.Unmarshal(lockData, &lock); err != nil {
			// Corrupt lock file — treat as unknown, preserve the user's
			// file. Don't error: the skill itself is usable even without
			// a readable lock.
			st.State = StateUnknown
			return st, nil
		}
		st.Installed = &lock
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("skill: read lock: %w", err)
	}

	switch {
	case st.Installed == nil:
		st.State = StateUnknown
	case st.Installed.Hash != st.OnDisk.Hash:
		st.State = StateModified
	case st.OnDisk.Hash != st.Embedded.Hash:
		st.State = StateStale
	default:
		st.State = StateCurrent
	}
	return st, nil
}

// Install writes the embedded skill + a fresh lock file to dir,
// creating the directory as needed. State=modified and state=unknown
// require force=true — otherwise Install refuses to preserve hand-edits.
// The returned Status reflects post-install state.
func Install(dir string, force bool) (*Status, error) {
	current, err := Inspect(dir)
	if err != nil {
		return nil, err
	}
	if !force {
		switch current.State {
		case StateModified:
			return current, &ErrRefusedOverwrite{State: current.State,
				Reason: "skill has been hand-edited since install"}
		case StateUnknown:
			return current, &ErrRefusedOverwrite{State: current.State,
				Reason: "skill present but wasn't installed by pql"}
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: committed dir needs group/other read
		return nil, fmt.Errorf("skill: mkdir %s: %w", dir, err)
	}

	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte(embedded), 0o644); err != nil { //nolint:gosec // G306: committed file needs group/other read
		return nil, fmt.Errorf("skill: write SKILL.md: %w", err)
	}

	lock := Snapshot{
		Version:     version.Version,
		Hash:        hashBytes([]byte(embedded)),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	lockData, _ := json.MarshalIndent(lock, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, LockFile), append(lockData, '\n'), 0o644); err != nil { //nolint:gosec // G306: committed file needs group/other read
		return nil, fmt.Errorf("skill: write lock: %w", err)
	}

	return Inspect(dir)
}

// Uninstall removes SKILL.md and the lock file. Missing files are not
// errors (idempotent). If dir ends up empty we also remove it; non-
// empty dirs are left alone.
func Uninstall(dir string) error {
	var errs []error
	for _, name := range []string{SkillFile, LockFile} {
		p := filepath.Join(dir, name)
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("skill: remove %s: %w", p, err))
		}
	}
	_ = os.Remove(dir) // removes only if empty; ignore error otherwise
	return errors.Join(errs...)
}

// ErrRefusedOverwrite is returned by Install when it would clobber
// user work without force=true. Callers can errors.As it to decide
// whether to prompt or surface a specific exit code.
type ErrRefusedOverwrite struct {
	State  State
	Reason string
}

func (e *ErrRefusedOverwrite) Error() string {
	return fmt.Sprintf("skill: refusing to overwrite (%s): %s; pass force=true to overwrite",
		e.State, e.Reason)
}

// hashBytes produces the "sha256:<hex>" form used on disk and in
// Status.Hash. Single-source so tests and production agree byte-for-byte.
func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}
