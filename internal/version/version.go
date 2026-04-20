// Package version exposes build-info stamped via -ldflags at build time.
package version

import "runtime"

// Build-time identity stamped by the Makefile via -ldflags. Defaults are
// the unstamped sentinels — `make build` overwrites them with the
// project.yaml version, git short SHA, and ISO timestamp.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// SchemaVersion is the on-disk SQLite schema this binary expects. Bumped
// in lockstep with internal/store/schema/v*.sql; on mismatch the store
// drops and rebuilds the index (see internal/store.migrate).
const SchemaVersion = 1

// BuildInfo is the JSON shape returned by `pql version --build-info`.
// Mirrors the fields the skill reads to negotiate compatibility.
type BuildInfo struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	Date          string `json:"date"`
	GoVersion     string `json:"go_version"`
	SchemaVersion int    `json:"schema_version"`
}

// Info captures the current binary's stamped build metadata.
func Info() BuildInfo {
	return BuildInfo{
		Version:       Version,
		Commit:        Commit,
		Date:          Date,
		GoVersion:     runtime.Version(),
		SchemaVersion: SchemaVersion,
	}
}
