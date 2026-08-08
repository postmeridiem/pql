// Package migrate is the forward-migration vocabulary shared by pql's
// versioned artefacts.
//
// pql carries several independently-versioned things: the changelog file
// format, the pql.db schema, the canonical row projection. They were reached
// by different routes — one forced by a replication bug, one waiting on a
// distribution gate — but they need the same five answers: what version is on
// disk, what version does this binary speak, what ordered steps get from one to
// the other, what happens when there is no such path, and what happens when the
// artefact is *newer* than the binary.
//
// This package answers those once. An axis owns detection and its own steps;
// everything about sequencing, gap handling and refusal wording lives here, so
// adding a third versioned artefact is a registration rather than a
// reimplementation.
//
// It deliberately knows nothing about SQL, files or replication: the pql.db
// axis must not have to import a replication package to migrate a schema.
package migrate

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Version is the pql release an artefact's format was introduced in — "2.0.0",
// not "2".
//
// A bare counter answers "are these the same" and nothing else. Reading
// `changelog_format: 2` in a marker tells you nothing about when it changed or
// which binary you need; reading `2.0.0` tells you both, and it makes the
// version axes legible against the release history instead of requiring a
// lookup table to interpret. The cost is ordering that has to be computed
// rather than compared, which is Compare's job.
//
// Versions that are *stored per row* stay integers — the `canonical_version`
// column on every planning row, index.db's `schema_version`. Those are data,
// written once per row and compared for equality; widening them would be a
// data migration for no readability gain, since nobody reads a row's hash
// version for release archaeology.
type Version string

// Compare orders two versions the way releases order: -1 if a precedes b, 0 if
// equal, +1 if a follows b. The empty version sorts below every real one, which
// is what "an artefact that predates versioning" means.
//
// Deliberately not a general semver implementation: these values come from
// project.yaml and are always plain MAJOR.MINOR.PATCH. A non-numeric segment
// sorts as 0 rather than erroring, so a malformed marker degrades to "older"
// and gets migrated rather than crashing the caller.
func Compare(a, b Version) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	as, bs := strings.Split(string(a), "."), strings.Split(string(b), ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		if c := compareSegment(segment(as, i), segment(bs, i)); c != 0 {
			return c
		}
	}
	return 0
}

// IsRelease reports whether v looks like a pql release — at least
// MAJOR.MINOR, all numeric.
//
// A ledger or marker can hold a value this binary has no way to interpret: a
// bare counter from a scheme that predates release-versioned axes, a truncated
// write, a hand edit. Such a value must not be treated as a version to migrate
// *from*, because no step will ever chain to it and the artefact would be
// stranded — refused on every open, with a recovery hint telling the user to
// delete a database that is very likely fine. Callers use this to demote an
// uninterpretable stamp to "unversioned" and let the shape check decide, which
// heals rather than blocks.
func (v Version) IsRelease() bool {
	parts := strings.Split(string(v), ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func segment(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}
	return "0"
}

func compareSegment(a, b string) int {
	ai, _ := strconv.Atoi(a) // non-numeric degrades to 0; see Compare's docstring
	bi, _ := strconv.Atoi(b)
	switch {
	case ai < bi:
		return -1
	case ai > bi:
		return 1
	default:
		return 0
	}
}

// Step moves an artefact from one version to the next. Steps are the unit an
// axis grows by: a new format or schema change is one more entry in a slice.
type Step struct {
	// From is the version this step applies to, and To is what it produces.
	// Both are stated rather than implied by position: with app versions there
	// is no "next number", so the chain has to be explicit — which also makes a
	// missing link detectable instead of silently skipped.
	From Version
	To   Version
	// ID is a stable, human-readable identifier ("changelog-guard-by-position").
	// Axes that keep a ledger record it, so an applied history is legible
	// without decoding version numbers.
	ID string
	// Apply performs the migration. It must be safe to re-run: a step that is
	// interrupted after doing its work but before its version is recorded will
	// be applied again on the next pass.
	Apply func(ctx context.Context) error
}

// Axis is one versioned artefact.
type Axis struct {
	// Name appears in diagnostics: "changelog format", "pql.db schema".
	Name string
	// Current is the version this binary speaks.
	Current Version
	// Found is the version detected on disk. The empty version means the
	// artefact predates versioning entirely.
	Found Version
	// Steps may arrive in any order; Plan chains them.
	Steps []Step
	// Recovery is appended to the error when no chain of steps reaches Current
	// — the axis's own escape hatch, e.g. "delete pql.db and run pql plan
	// rebuild". Optional.
	Recovery string
}

// UpToDate reports whether the artefact already speaks the binary's version.
func (a Axis) UpToDate() bool { return Compare(a.Found, a.Current) == 0 }

// Ahead reports whether the artefact is newer than this binary understands.
func (a Axis) Ahead() bool { return Compare(a.Found, a.Current) > 0 }

// Plan resolves the ordered steps that carry Found to Current.
//
// A nil slice with a nil error means there is nothing to do. Two situations are
// refusals rather than empty plans, both because proceeding would mean operating
// on an artefact under rules that do not describe it:
//
//   - Found after Current. The artefact was written by a newer pql. There is no
//     backward step and no way to know what a future version encoded, so this
//     refuses and says to upgrade — the same stance the changelog importer
//     already takes on a canonical_version it does not speak.
//   - A broken chain. Steps exist but do not link all the way to Current, so
//     applying them would leave the artefact stranded at an intermediate
//     version.
func Plan(a Axis) ([]Step, error) {
	if a.Ahead() {
		return nil, fmt.Errorf(
			"%s is at version %s but this pql speaks %s — refusing to proceed; "+
				"upgrade pql to one that understands version %s",
			a.Name, a.Found, a.Current, a.Found)
	}
	if a.UpToDate() {
		return nil, nil
	}

	steps := append([]Step(nil), a.Steps...)
	sort.Slice(steps, func(i, j int) bool { return Compare(steps[i].To, steps[j].To) < 0 })

	var plan []Step
	at := a.Found
	for _, s := range steps {
		if Compare(s.To, at) <= 0 {
			continue // already past this one
		}
		if Compare(s.From, at) != 0 {
			break // the chain breaks here; reported below against the version reached
		}
		plan = append(plan, s)
		at = s.To
	}
	if Compare(at, a.Current) != 0 {
		reached := string(at)
		if reached == "" {
			reached = "(unversioned)"
		}
		err := fmt.Errorf(
			"%s is at version %s and this pql speaks %s, but no migration step "+
				"reaches past version %s",
			a.Name, displayVersion(a.Found), a.Current, reached)
		if a.Recovery != "" {
			return nil, fmt.Errorf("%w. Recovery: %s", err, a.Recovery)
		}
		return nil, err
	}
	return plan, nil
}

// displayVersion renders the empty version readably in diagnostics.
func displayVersion(v Version) string {
	if v == "" {
		return "(unversioned)"
	}
	return string(v)
}

// Applied records one step that ran, for reporting back to the user.
type Applied struct {
	Axis string  `json:"axis"`
	ID   string  `json:"id"`
	To   Version `json:"to"`
}

// Run plans and applies. onApplied, if non-nil, is called after each successful
// step so an axis with a ledger can record its progress inside the same pass —
// which is what makes an interrupted run resumable rather than ambiguous.
//
// A failing step stops the run and returns what had already been applied, so
// the caller can report partial progress instead of implying nothing happened.
func Run(ctx context.Context, a Axis, onApplied func(Step) error) ([]Applied, error) {
	plan, err := Plan(a)
	if err != nil {
		return nil, err
	}

	var done []Applied
	for _, s := range plan {
		if err := s.Apply(ctx); err != nil {
			return done, fmt.Errorf("%s: step %s (→ version %s): %w", a.Name, s.ID, s.To, err)
		}
		if onApplied != nil {
			if err := onApplied(s); err != nil {
				return done, fmt.Errorf("%s: recording step %s: %w", a.Name, s.ID, err)
			}
		}
		done = append(done, Applied{Axis: a.Name, ID: s.ID, To: s.To})
	}
	return done, nil
}
