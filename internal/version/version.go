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

// pql versions four things independently, and a consumer diagnosing an upgrade
// needs to compare what it holds on disk against what a binary speaks. All four
// are declared here and mirrored in project.yaml, which is the file a human
// reads; version_test.go parses project.yaml and fails on drift, so the mirror
// cannot rot the way a comment asking someone to remember would.
//
// Each is bumped on its own schedule — a release that changes none of them is
// the common case. docs/versions.md maps app version to axis version over time.
const (
	// SchemaVersion is the on-disk index.db schema this binary expects. Bumped
	// in lockstep with internal/store/schema/v*.sql; on mismatch the store
	// drops and rebuilds the index (see internal/store.migrate), which is safe
	// because index.db is a pure cache.
	SchemaVersion = 1

	// CanonicalVersion identifies the row-canonicalisation rules behind every
	// planning row's content hash. Aliased by planning.CanonicalVersion, which
	// is where the rules themselves live.
	CanonicalVersion = 2

	// PlanningSchemaVersion is the pql.db schema generation this binary
	// expects. Bumped alongside each forward migration step added to
	// planning.schemaSteps; the schema_migrations ledger in a database records
	// how far it has been migrated.
	PlanningSchemaVersion = 1

	// ChangelogFormat is the .pql/changelog/ file format this binary writes.
	// Unlike the other two this one is not regenerable — the changelog is the
	// log of record — so a version below this is migrated forward in place
	// rather than dropped and rebuilt (D-28).
	ChangelogFormat = 2
)

// BuildInfo is the JSON shape returned by `pql version --build-info`.
// Mirrors the fields the skill reads to negotiate compatibility.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	// The version axes, so a consumer can diff what it has against what this
	// binary emits without reading source.
	SchemaVersion         int `json:"schema_version"`
	PlanningSchemaVersion int `json:"planning_schema_version"`
	CanonicalVersion      int `json:"canonical_version"`
	ChangelogFormat       int `json:"changelog_format"`
}

// Info captures the current binary's stamped build metadata.
func Info() BuildInfo {
	return BuildInfo{
		Version:               Version,
		Commit:                Commit,
		Date:                  Date,
		GoVersion:             runtime.Version(),
		SchemaVersion:         SchemaVersion,
		PlanningSchemaVersion: PlanningSchemaVersion,
		CanonicalVersion:      CanonicalVersion,
		ChangelogFormat:       ChangelogFormat,
	}
}
