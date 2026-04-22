// Package telemetry records per-phase timings and emits them on
// --verbose as stderr JSON diagnostics.
package telemetry

import (
	"time"

	"github.com/postmeridiem/pql/internal/diag"
)

// Timer accumulates named phase durations.
type Timer struct {
	phases  []phase
	enabled bool
}

type phase struct {
	name  string
	start time.Time
	dur   time.Duration
}

// New creates a Timer. If enabled is false, all operations are no-ops.
func New(enabled bool) *Timer {
	return &Timer{enabled: enabled}
}

// Start begins timing a named phase and returns a function to stop it.
func (t *Timer) Start(name string) func() {
	if !t.enabled {
		return func() {}
	}
	p := phase{name: name, start: time.Now()}
	return func() {
		p.dur = time.Since(p.start)
		t.phases = append(t.phases, p)
	}
}

// Emit writes all recorded phases as stderr diagnostics.
func (t *Timer) Emit() {
	if !t.enabled {
		return
	}
	for _, p := range t.phases {
		diag.Warn("timing."+p.name,
			p.dur.Round(time.Microsecond).String())
	}
}
