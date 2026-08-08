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
)

// Step moves an artefact from the version below it to To. Steps are the unit an
// axis grows by: a new format or schema change is one more entry in a slice.
type Step struct {
	// To is the version this step produces. The version it applies *from* is
	// implied by ordering — a step runs when the artefact sits below To.
	To int
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
	Current int
	// Found is the version detected on disk. Zero means "nothing detected",
	// which callers should resolve to their own implicit-first-version rule
	// before constructing the Axis.
	Found int
	// Steps may arrive in any order; Plan sorts them.
	Steps []Step
	// Recovery is appended to the error when no chain of steps reaches Current
	// — the axis's own escape hatch, e.g. "delete pql.db and run pql plan
	// rebuild". Optional.
	Recovery string
}

// UpToDate reports whether the artefact already speaks the binary's version.
func (a Axis) UpToDate() bool { return a.Found == a.Current }

// Ahead reports whether the artefact is newer than this binary understands.
func (a Axis) Ahead() bool { return a.Found > a.Current }

// Plan resolves the ordered steps that carry Found to Current.
//
// A nil slice with a nil error means there is nothing to do. Two situations are
// refusals rather than empty plans, both because proceeding would mean operating
// on an artefact under rules that do not describe it:
//
//   - Found > Current. The artefact was written by a newer pql. There is no
//     backward step and no way to know what a future version encoded, so this
//     refuses and says to upgrade — the same stance the changelog importer
//     already takes on a canonical_version it does not speak.
//   - A gap. Steps exist but do not chain all the way to Current, so applying
//     them would leave the artefact stranded at an intermediate version.
func Plan(a Axis) ([]Step, error) {
	if a.Ahead() {
		return nil, fmt.Errorf(
			"%s is at version %d but this pql speaks %d — refusing to proceed; "+
				"upgrade pql to one that understands version %d",
			a.Name, a.Found, a.Current, a.Found)
	}
	if a.UpToDate() {
		return nil, nil
	}

	steps := append([]Step(nil), a.Steps...)
	sort.Slice(steps, func(i, j int) bool { return steps[i].To < steps[j].To })

	var plan []Step
	at := a.Found
	for _, s := range steps {
		if s.To <= at {
			continue // already past this one
		}
		if s.To != at+1 {
			break // gap — reported below against the version actually reached
		}
		plan = append(plan, s)
		at = s.To
	}
	if at != a.Current {
		err := fmt.Errorf(
			"%s is at version %d and this pql speaks %d, but no migration step "+
				"reaches past version %d",
			a.Name, a.Found, a.Current, at)
		if a.Recovery != "" {
			return nil, fmt.Errorf("%w. Recovery: %s", err, a.Recovery)
		}
		return nil, err
	}
	return plan, nil
}

// Applied records one step that ran, for reporting back to the user.
type Applied struct {
	Axis string `json:"axis"`
	ID   string `json:"id"`
	To   int    `json:"to"`
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
			return done, fmt.Errorf("%s: step %s (→ version %d): %w", a.Name, s.ID, s.To, err)
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
