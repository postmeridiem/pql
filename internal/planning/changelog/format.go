package changelog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/postmeridiem/pql/internal/planning/migrate"
	"github.com/postmeridiem/pql/internal/version"
)

// formatMarkerFile records which format a changelog is written in. It lives at
// the changelog root rather than inside a table directory, so Import's walk —
// which only descends into directories — never executes it.
const formatMarkerFile = "0000-format.sql"

// formatAxisName is the axis label that appears in every diagnostic.
const formatAxisName = "changelog format"

// UpgradeResult reports what an upgrade did.
type UpgradeResult struct {
	// FoundFormat is the version detected on disk before any step ran.
	FoundFormat migrate.Version `json:"found_format"`
	// CurrentFormat is the version this binary writes.
	CurrentFormat migrate.Version `json:"current_format"`
	// Steps applied, in order. Empty when the changelog was already current.
	Steps []migrate.Applied `json:"steps,omitempty"`
	// FilesRewritten lists the changelog files whose contents changed,
	// relative to the changelog root.
	FilesRewritten []string `json:"files_rewritten,omitempty"`
	// DryRun reports whether anything was actually written.
	DryRun bool `json:"dry_run"`
}

// UpToDate reports whether there was nothing to do.
func (r *UpgradeResult) UpToDate() bool { return r.FoundFormat == r.CurrentFormat }

// DetectFormat reads the format version a changelog declares.
//
// A changelog with no marker reports the pre-versioned format: every changelog
// written before this mechanism existed is, by definition, the shape that
// existed then. An absent changelog directory reports the current format —
// there is nothing to migrate, and a fresh vault should not be told it is stale.
func DetectFormat(vaultPath string) (migrate.Version, error) {
	root := filepath.Join(vaultPath, ChangelogDir)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return version.ChangelogFormat, nil
	} else if err != nil {
		return "", fmt.Errorf("changelog: stat %s: %w", root, err)
	}

	body, err := os.ReadFile(filepath.Join(root, formatMarkerFile)) //nolint:gosec // G304: fixed filename under the vault's changelog dir
	if err != nil {
		if os.IsNotExist(err) {
			return version.PreVersionedChangelog, nil
		}
		return "", fmt.Errorf("changelog: read format marker: %w", err)
	}

	raw := readMarker(body, "pql:changelog_format")
	if raw == "" {
		return version.PreVersionedChangelog, nil
	}
	return migrate.Version(raw), nil
}

// formatAxis describes the changelog format axis for a given vault.
func formatAxis(vaultPath string, found migrate.Version) migrate.Axis {
	return migrate.Axis{
		Name:    formatAxisName,
		Current: version.ChangelogFormat,
		Found:   found,
		Steps: []migrate.Step{
			{
				From: version.PreVersionedChangelog,
				To:   "2.0.0",
				ID:   "changelog-guard-by-position",
				Apply: func(ctx context.Context) error {
					// Format 2 changes no row data at all: the entire delta is
					// the inline conflict clause, which moved from a
					// content-hash tiebreak to append position (T-59, D-16).
					// Rows are staged and re-emitted rather than edited in
					// place so the new clause comes from the same renderer a
					// normal export uses.
					_, err := restageAll(ctx, vaultPath, false)
					return err
				},
			},
		},
		Recovery: "re-clone the repository, or regenerate .pql/changelog/ with `pql plan export` " +
			"after rebuilding pql.db from a replica that is on a format this binary understands",
	}
}

// Upgrade brings a vault's changelog up to the format this binary writes.
//
// It is idempotent: an already-current changelog is a no-op, and re-running
// after an interrupted pass re-derives the same output, because every step is a
// pure function of the files it reads. That is what makes an interrupted
// upgrade safe — there is no half-migrated state to reason about, only files
// that have or have not been rewritten yet, and a second run finishes the job.
//
// dryRun stages and renders everything but writes nothing, so a caller can see
// exactly which files an upgrade would touch first.
func Upgrade(ctx context.Context, vaultPath string, dryRun bool) (*UpgradeResult, error) {
	found, err := DetectFormat(vaultPath)
	if err != nil {
		return nil, err
	}

	res := &UpgradeResult{
		FoundFormat:   found,
		CurrentFormat: version.ChangelogFormat,
		DryRun:        dryRun,
	}
	axis := formatAxis(vaultPath, found)

	if dryRun {
		if _, err := migrate.Plan(axis); err != nil {
			return nil, err
		}
		if res.UpToDate() {
			return res, nil
		}
		changed, err := restageAll(ctx, vaultPath, true)
		if err != nil {
			return nil, err
		}
		res.FilesRewritten = changed
		return res, nil
	}

	// Capture the rewritten files from the step itself: the step is what knows
	// which files it touched, and reporting them is most of this verb's value.
	var rewritten []string
	axis.Steps[0].Apply = func(ctx context.Context) error {
		changed, err := restageAll(ctx, vaultPath, false)
		rewritten = changed
		return err
	}

	steps, err := migrate.Run(ctx, axis, nil)
	if err != nil {
		return nil, err
	}
	res.Steps = steps
	res.FilesRewritten = rewritten

	if len(steps) > 0 {
		if err := WriteFormatMarker(vaultPath); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// restageAll stages every table's files and re-emits them through the current
// renderer. Returns the files whose content changed, relative to the changelog
// root. With preview set, nothing is written.
//
// Ordering note: staging reads every file before anything is written, so a
// failure part-way through leaves the changelog untouched. Once writing starts,
// each file is replaced atomically via a temp file and rename, so no file is
// ever observed half-written — the worst an interruption leaves is a mix of
// upgraded and not-yet-upgraded files, which the next run resolves.
func restageAll(ctx context.Context, vaultPath string, preview bool) ([]string, error) {
	root := filepath.Join(vaultPath, ChangelogDir)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}

	stage, err := newStaging(ctx)
	if err != nil {
		return nil, err
	}
	defer stage.close()

	type pending struct {
		path  string
		rel   string
		lines []string
	}
	var writes []pending

	for _, spec := range changelogTables {
		tableDir := filepath.Join(root, spec.Name)
		files, err := stage.loadTable(ctx, spec, tableDir)
		if err != nil {
			return nil, err
		}
		for _, name := range files {
			lines, err := stage.renderFile(ctx, spec, name)
			if err != nil {
				return nil, err
			}
			writes = append(writes, pending{
				path:  filepath.Join(tableDir, name),
				rel:   filepath.Join(spec.Name, name),
				lines: lines,
			})
		}
	}

	var changed []string
	for _, w := range writes {
		body := ""
		if len(w.lines) > 0 {
			body = strings.Join(w.lines, "\n") + "\n"
		}
		existing, err := os.ReadFile(w.path) //nolint:gosec // G304: path built from a directory just listed
		if err != nil {
			return nil, fmt.Errorf("changelog: read %s: %w", w.path, err)
		}
		if string(existing) == body {
			continue // already in the current format, byte for byte
		}
		changed = append(changed, w.rel)
		if preview {
			continue
		}
		if err := writeFileAtomic(w.path, body); err != nil {
			return nil, err
		}
	}
	return changed, nil
}

// writeFileAtomic replaces a file via a temp file in the same directory plus a
// rename, so a reader never sees a partially-written changelog and an
// interrupted upgrade cannot corrupt one.
func writeFileAtomic(path, body string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".pql-upgrade-*")
	if err != nil {
		return fmt.Errorf("changelog: temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("changelog: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("changelog: close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil { //nolint:gosec // G302: changelog files are committed to git
		return fmt.Errorf("changelog: chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("changelog: replace %s: %w", path, err)
	}
	return nil
}

// WriteFormatMarker stamps the changelog with the format it is now in. Written
// last by an upgrade, so a marker is never ahead of the files it describes: an
// interrupted upgrade leaves the old marker, and the next run redoes the work
// rather than believing a lie.
//
// Exported for `pql init`, which stamps a newly-created changelog so it is
// never mistaken for the unversioned shape that predates the marker.
func WriteFormatMarker(vaultPath string) error {
	root := filepath.Join(vaultPath, ChangelogDir)
	if err := os.MkdirAll(root, 0o755); err != nil { //nolint:gosec // G301: changelog dir is committed
		return fmt.Errorf("changelog: mkdir %s: %w", root, err)
	}
	body := "-- Changelog format marker, written by pql. Comments only: this file\n" +
		"-- is never executed — Import descends into the per-table directories\n" +
		"-- and does not read the changelog root.\n" +
		"--\n" +
		"-- A changelog carrying no marker is format 1, the shape that existed\n" +
		"-- before formats were versioned. An older format is migrated forward\n" +
		"-- by `pql plan upgrade` (and automatically from the post-merge hook);\n" +
		"-- a newer one is refused rather than replayed under rules this binary\n" +
		"-- does not know. See D-28 and docs/versions.md.\n" +
		"-- pql:changelog_format: " + version.ChangelogFormat + "\n" +
		"-- pql:written_by: " + version.Version + "\n"
	path := filepath.Join(root, formatMarkerFile)
	return writeFileAtomic(path, body)
}

// CheckFormat reports whether a vault's changelog can be replayed by this
// binary, without changing anything.
//
// Older than current is survivable — the rows are readable, they just carry a
// superseded conflict guard — so it returns a warning for the caller to surface
// and lets replay proceed. Newer than current is refused: a format this binary
// does not know may encode rows in ways it would misread, and there is no
// backward step. That asymmetry mirrors the canonical_version guard the
// importer has always applied to schema fixtures.
func CheckFormat(vaultPath string) (found migrate.Version, warning string, err error) {
	found, err = DetectFormat(vaultPath)
	if err != nil {
		return "", "", err
	}
	current := migrate.Version(version.ChangelogFormat)
	switch cmp := migrate.Compare(found, current); {
	case cmp == 0:
		return found, "", nil
	case cmp > 0:
		return found, "", fmt.Errorf(
			"%s is version %s but this pql writes version %s — refusing to replay; "+
				"upgrade pql to one that understands version %s",
			formatAxisName, found, current, found)
	default:
		return found, fmt.Sprintf(
			"%s is version %s, this pql writes version %s — replaying under the older "+
				"rules; run `pql plan upgrade` to migrate the changelog forward",
			formatAxisName, found, current), nil
	}
}
