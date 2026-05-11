package cli

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

// initDQRStruct describes what ensureDQRStructure did. Embedded in
// initResult.
type initDQRStruct struct {
	Root          string   `json:"root,omitempty"`
	Subdirs       []string `json:"subdirs,omitempty"`
	ReadmeWritten bool     `json:"readme_written"`
	ReadmeUpdated bool     `json:"readme_updated"`
	Skipped       string   `json:"skipped,omitempty"`
}

const dqrReadmeRecordsMarker = "<!-- pql:records (auto-generated; do not edit manually) -->"

// dqrReadmePrologue is what `pql init` plants in
// <dqr_dir>/README.md when no README exists yet. The prologue is
// human-edited content; subsequent regeneration only overwrites the
// records section below the marker.
const dqrReadmePrologue = `# Decisions, Questions, Rejected

This directory holds structured planning records that pql parses
into pql.db. Each record is a ` + "`### [DQR]-N: Title`" + ` heading inside
a markdown file. Files live in three per-type subdirectories:

- ` + "`decisions/<domain>.md`" + ` — confirmed design decisions
- ` + "`questions/<domain>.md`" + ` — open questions that may resolve into
  decisions or rejected proposals
- ` + "`rejected/<domain>.md`" + ` — rejected proposals (kept for the audit
  trail)

The parser infers domain from the filename stem and record type
from the parent subdirectory.

D-records that propose implementation work link to ` + "`initiative`" + `-type
tickets via ` + "`decision_ref`" + `. Run ` + "`pql decisions show <id>" + `
` + "--with-tickets`" + ` to inspect implementation status.

## Recommended domains

Start with this canonical set; create files as records land in
each domain:

- **architecture** — structural commitments (storage, layering,
  languages, libraries)
- **process** — team workflow (commits, branches, releases, reviews)
- **design** — user-facing surface (UX, UI, public APIs)
- **coding-conventions** — team-internal code shape (style, lint,
  file layout)
- **testing** — quality strategy (coverage, layers, gates)

You might also want, project-permitting:

- ` + "`accessibility`" + ` — if you ship user-facing software
- ` + "`security`" + ` — if you handle user data or network surfaces
- ` + "`licensing`" + ` — if you release open-source or commercial
- ` + "`documentation`" + ` — if user-docs are non-trivial
- ` + "`deployment`" + ` — if shipping is non-trivial
- ` + "`performance`" + ` — if you have perf budgets / SLOs

` + dqrReadmeRecordsMarker + `
`

// ensureDQRStructure creates the configured DQR root and its three
// type subdirectories, and plants the README prologue if absent.
// Skips when a legacy `decisions/` tree exists with content — T-37's
// migration handles those repos.
func ensureDQRStructure(dir string) initDQRStruct {
	dqr := readDQRDirFromConfig(dir)
	stat := initDQRStruct{Root: filepath.Join(dir, dqr)}

	// Refuse to plant when a legacy decisions/ tree carries content
	// and the configured root would be a different directory. Keeps
	// us from silently bifurcating state across two layouts.
	legacy := filepath.Join(dir, "decisions")
	if dqr != "decisions" && hasLegacyDQRContent(legacy) {
		stat.Skipped = "legacy decisions/ tree present; migrate to " + dqr +
			"/ first (or set dqr_dir: decisions in .pql/config.yaml)"
		return stat
	}

	for _, sub := range []string{"decisions", "questions", "rejected"} {
		subPath := filepath.Join(stat.Root, sub)
		if err := os.MkdirAll(subPath, 0o755); err != nil { //nolint:gosec // G301: committed directory
			stat.Skipped = "mkdir " + subPath + ": " + err.Error()
			return stat
		}
		stat.Subdirs = append(stat.Subdirs, subPath)
	}

	readmePath := filepath.Join(stat.Root, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		if err := os.WriteFile(readmePath, []byte(dqrReadmePrologue), 0o644); err != nil { //nolint:gosec // G306: committed file
			stat.Skipped = "write README: " + err.Error()
			return stat
		}
		stat.ReadmeWritten = true
	}
	return stat
}

// readDQRDirFromConfig peeks at .pql/config.yaml for a dqr_dir setting
// without spinning up the full config.Load (which would create a
// cyclic dependency from init.go's bootstrap-time call site).
// Defaults to "governance" when the file is absent or the field is
// unset.
func readDQRDirFromConfig(dir string) string {
	cfgPath := filepath.Join(dir, ".pql", "config.yaml")
	body, err := os.ReadFile(cfgPath) //nolint:gosec // G304: known config path
	if err != nil {
		return "governance"
	}
	for _, line := range strings.Split(string(body), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "dqr_dir:") {
			v := strings.TrimSpace(strings.TrimPrefix(t, "dqr_dir:"))
			v = strings.Trim(v, `"'`)
			if v != "" {
				return v
			}
		}
	}
	return "governance"
}

// hasLegacyDQRContent returns true when a legacy `decisions/` tree
// exists with at least one .md file (excluding README.md). Used by
// ensureDQRStructure to avoid silently bifurcating state.
func hasLegacyDQRContent(legacy string) bool {
	info, err := os.Stat(legacy)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(legacy)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if strings.EqualFold(e.Name(), "readme.md") {
			continue
		}
		return true
	}
	return false
}

// regenerateDQRReadme rewrites the records section of
// <dqr_root>/README.md from pql.db's current decisions table.
// Idempotent — running it on a README with no marker appends one;
// running with an up-to-date README produces no diff. Prologue is
// preserved verbatim.
func regenerateDQRReadme(ctx context.Context, db *sql.DB, dqrRoot string) (bool, error) {
	readmePath := filepath.Join(dqrRoot, "README.md")
	existing, err := os.ReadFile(readmePath) //nolint:gosec // G304: planted by ensureDQRStructure
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read README: %w", err)
	}

	header := string(existing)
	idx := strings.Index(header, dqrReadmeRecordsMarker)
	if idx < 0 {
		header = strings.TrimRight(header, "\n") + "\n\n" + dqrReadmeRecordsMarker + "\n"
	} else {
		header = header[:idx+len(dqrReadmeRecordsMarker)] + "\n"
	}

	section, err := buildRecordsSection(ctx, db, dqrRoot)
	if err != nil {
		return false, err
	}

	updated := []byte(header + "\n" + section)
	if bytes.Equal(updated, existing) {
		return false, nil
	}
	if err := os.WriteFile(readmePath, updated, 0o644); err != nil { //nolint:gosec // G306: committed file
		return false, fmt.Errorf("write README: %w", err)
	}
	return true, nil
}

// buildRecordsSection groups every decisions-table row (D, Q, R) by
// type and emits a markdown list keyed on title, with anchor links
// relative to dqrRoot.
func buildRecordsSection(ctx context.Context, db *sql.DB, dqrRoot string) (string, error) {
	decisions, err := repo.ListDecisions(ctx, db, repo.DecisionFilter{})
	if err != nil {
		return "", fmt.Errorf("list decisions: %w", err)
	}
	byType := map[string][]repo.Decision{}
	for _, d := range decisions {
		byType[d.Type] = append(byType[d.Type], d)
	}

	var b strings.Builder
	b.WriteString("## Decisions\n\n")
	writeBucket(&b, byType["confirmed"], dqrRoot)
	b.WriteString("\n## Open questions\n\n")
	writeBucket(&b, byType["question"], dqrRoot)
	b.WriteString("\n## Rejected\n\n")
	writeBucket(&b, byType["rejected"], dqrRoot)
	return b.String(), nil
}

func writeBucket(b *strings.Builder, rows []repo.Decision, dqrRoot string) {
	if len(rows) == 0 {
		b.WriteString("- _(none)_\n")
		return
	}
	for _, r := range rows {
		rel := readmeRelativePath(r.FilePath, dqrRoot)
		anchor := slugify(r.ID + " " + r.Title)
		fmt.Fprintf(b, "- [%s: %s](%s#%s) — _%s_\n",
			r.ID, r.Title, rel, anchor, r.Domain)
	}
}

// regenerateReadmeStep opens pql.db and refreshes the records
// section of <dqr_root>/README.md. Errors are swallowed and recorded
// as stat.Skipped — the README staying stale isn't worth aborting
// the rest of init for.
func regenerateReadmeStep(ctx context.Context, dir string, stat *initDQRStruct) bool {
	if stat.Root == "" || stat.Skipped != "" {
		return false
	}
	dbPath := filepath.Join(dir, ".pql", "pql.db")
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		stat.Skipped = "open pql.db for README regen: " + err.Error()
		return false
	}
	defer func() { _ = db.Close() }()
	updated, err := regenerateDQRReadme(ctx, db, stat.Root)
	if err != nil {
		stat.Skipped = "regenerate README: " + err.Error()
		return false
	}
	stat.ReadmeUpdated = updated
	return updated
}

// readmeRelativePath converts a repo-root-relative file_path
// (e.g. "governance/decisions/architecture.md") into a path
// relative to the README's location (e.g. "decisions/architecture.md").
func readmeRelativePath(filePath, dqrRoot string) string {
	dqrName := filepath.Base(dqrRoot)
	prefix := dqrName + "/"
	if strings.HasPrefix(filePath, prefix) {
		return filePath[len(prefix):]
	}
	return filePath
}

// slugify mirrors GitHub's heading-anchor convention: lowercase,
// non-alphanumeric collapsed to hyphens, runs of hyphens preserved
// (GFM keeps them).
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
			// other characters dropped
		}
	}
	return b.String()
}
