package planning

import "strings"

// Ticket status classes. A status' class is what the engine reasons
// about — the literal status names are a per-vault vocabulary (see
// .pql/config.yaml ticket_statuses), but every status maps to exactly
// one of these four roles, which is what the planning queries key off.
//
//   - initial:  not started; holds the default status ("to do").
//   - active:   work in flight; surfaced by `what-next`.
//   - review:   awaiting review; surfaced by `next-review`, and
//     deliberately excluded from `what-next` (don't review your own work).
//   - terminal: closed; clears blockers and is excluded from refine/unblocked.
//
// This is a vocabulary, not a state machine — transitions remain
// unrestricted per D-14.
const (
	StatusClassInitial  = "initial"
	StatusClassActive   = "active"
	StatusClassReview   = "review"
	StatusClassTerminal = "terminal"
)

// StatusClasses is the closed set of valid classes, in canonical order.
var StatusClasses = []string{
	StatusClassInitial, StatusClassActive, StatusClassReview, StatusClassTerminal,
}

// StatusDef describes one configured ticket status. Order and IsTerminal
// are derived by NewStatusSet (Order = position in the configured list;
// IsTerminal = class is terminal) and are not authored directly.
type StatusDef struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Class      string `json:"class"`
	Order      int    `json:"order"`
	IsDefault  bool   `json:"is_default"`
	IsTerminal bool   `json:"is_terminal"`
}

// StatusSet is the resolved, ordered ticket status vocabulary for a
// vault. Build it with NewStatusSet (from config) or DefaultStatusSet
// (the built-in six). The zero value is empty; IsZero reports it so
// callers can fall back to the default set.
type StatusSet struct {
	defs []StatusDef
}

// NewStatusSet resolves a configured status list into a StatusSet,
// stamping each def's Order (list position) and IsTerminal (class), and
// filling an empty Label from the name (e.g. "in_progress" → "In Progress").
func NewStatusSet(in []StatusDef) StatusSet {
	defs := make([]StatusDef, len(in))
	for i, d := range in {
		d.Order = i
		d.IsTerminal = d.Class == StatusClassTerminal
		if d.Label == "" {
			d.Label = titleize(d.Name)
		}
		defs[i] = d
	}
	return StatusSet{defs: defs}
}

// DefaultStatusSet is the built-in vocabulary used when a vault does not
// configure ticket_statuses. It reproduces pql's historical behaviour.
//
//nolint:goconst // the default status names are a one-off literal table here;
// hoisting them to constants would obscure the vocabulary, matching the
// inline-schema-enum convention in internal/planning/repo.
func DefaultStatusSet() StatusSet {
	return NewStatusSet([]StatusDef{
		{Name: "backlog", Class: StatusClassInitial, IsDefault: true},
		{Name: "ready", Class: StatusClassInitial},
		{Name: "in_progress", Class: StatusClassActive},
		{Name: "review", Class: StatusClassReview},
		{Name: "done", Class: StatusClassTerminal},
		{Name: "cancelled", Class: StatusClassTerminal},
	})
}

// IsZero reports whether the set carries no statuses (the zero value).
func (s StatusSet) IsZero() bool { return len(s.defs) == 0 }

// All returns the configured statuses in order (a copy — safe to mutate).
func (s StatusSet) All() []StatusDef {
	out := make([]StatusDef, len(s.defs))
	copy(out, s.defs)
	return out
}

// Names returns the configured status names in order.
func (s StatusSet) Names() []string {
	out := make([]string, len(s.defs))
	for i, d := range s.defs {
		out[i] = d.Name
	}
	return out
}

// Default returns the name of the default status — the one flagged
// is_default, else the first initial status, else the first status.
func (s StatusSet) Default() string {
	for _, d := range s.defs {
		if d.IsDefault {
			return d.Name
		}
	}
	for _, d := range s.defs {
		if d.Class == StatusClassInitial {
			return d.Name
		}
	}
	if len(s.defs) > 0 {
		return s.defs[0].Name
	}
	return ""
}

// IsValid reports whether name is a configured status.
func (s StatusSet) IsValid(name string) bool {
	for _, d := range s.defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

// OrderRank returns a status' position in the configured order; unknown
// statuses sort after all known ones.
func (s StatusSet) OrderRank(name string) int {
	for _, d := range s.defs {
		if d.Name == name {
			return d.Order
		}
	}
	return len(s.defs)
}

// Terminal returns the names of terminal (closed) statuses, in order.
func (s StatusSet) Terminal() []string { return s.byClass(StatusClassTerminal) }

// IsTerminal reports whether name is a terminal (closed) status. An
// unknown name is not terminal.
func (s StatusSet) IsTerminal(name string) bool {
	for _, d := range s.defs {
		if d.Name == name {
			return d.IsTerminal
		}
	}
	return false
}

// Active returns the names of active (in-flight) statuses, in order.
func (s StatusSet) Active() []string { return s.byClass(StatusClassActive) }

// Review returns the names of review statuses, in order.
func (s StatusSet) Review() []string { return s.byClass(StatusClassReview) }

// ReadyLane returns the single "ready" status — the most-advanced
// (last by order) initial status, which `what-next` treats as actionable.
// Earlier initial statuses (e.g. backlog) are not actionable. Returns ""
// when there are no initial statuses.
func (s StatusSet) ReadyLane() string {
	ready := ""
	for _, d := range s.defs {
		if d.Class == StatusClassInitial {
			ready = d.Name // last one wins
		}
	}
	return ready
}

func (s StatusSet) byClass(class string) []string {
	var out []string
	for _, d := range s.defs {
		if d.Class == class {
			out = append(out, d.Name)
		}
	}
	return out
}

// titleize renders a snake_case status name as a human label:
// "in_progress" → "In Progress".
func titleize(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
