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
// The two counters. These are written per row or per database and compared for
// equality, so they stay integers: widening them would be a data migration for
// no gain, since nobody reads a row's hash version to work out which release
// introduced it.
const (
	// SchemaVersion is the on-disk index.db schema this binary expects. Bumped
	// in lockstep with internal/store/schema/v*.sql; on mismatch the store
	// drops and rebuilds the index (see internal/store.migrate), which is safe
	// because index.db is a pure cache.
	SchemaVersion = 1

	// CanonicalVersion identifies the row-canonicalisation rules behind every
	// planning row's content hash, and is stored on every one of those rows.
	// Aliased by planning.CanonicalVersion, where the rules themselves live.
	CanonicalVersion = 2
)

// The two migrated axes. These carry the pql release their format was
// introduced in rather than a bare counter, because that is what someone
// holding an out-of-date artefact actually needs to know: `changelog_format:
// 2.0.0` says both "this is not what you have" and "here is the release that
// changed it", where `2` says only the first. Ordering is computed by
// migrate.Compare.
const (
	// PlanningSchemaVersion is the pql.db schema generation this binary
	// expects. Bumped alongside each forward migration step added to
	// planning.schemaSteps; the schema_migrations ledger in a database records
	// how far it has been migrated.
	PlanningSchemaVersion = "2.0.0"

	// ChangelogFormat is the .pql/changelog/ file format this binary writes.
	// Unlike the counters above this artefact is not regenerable — the
	// changelog is the log of record — so a version below this is migrated
	// forward in place rather than dropped and rebuilt (D-28).
	ChangelogFormat = "2.0.0"

	// PreVersionedChangelog is what a changelog carrying no format marker is
	// treated as: the last release before the format was versioned. Naming it
	// beats a bare empty string in diagnostics, where "your changelog is in the
	// 1.11.0-era format" is actionable and "" is not.
	PreVersionedChangelog = "1.11.0"
)

// BuildInfo is the JSON shape returned by `pql version --build-info`.
// Mirrors the fields the skill reads to negotiate compatibility.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	// The version axes, so a consumer can diff what it has against what this
	// binary emits without reading source. The two migrated axes report the
	// release their format was introduced in; the two per-row counters report
	// a number.
	SchemaVersion         int    `json:"schema_version"`
	CanonicalVersion      int    `json:"canonical_version"`
	PlanningSchemaVersion string `json:"planning_schema_version"`
	ChangelogFormat       string `json:"changelog_format"`
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
