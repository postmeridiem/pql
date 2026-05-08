package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

// ImportResult summarises a replay run.
type ImportResult struct {
	FilesReplayed []string `json:"files_replayed"`
	StatementsRun int      `json:"statements_run"`
}

// Import replays every changelog file under <vault>/.pql/changelog/
// whose mtime is newer than the last_import_marker. Files are
// processed table-by-table, lexicographically within each table
// directory — zero-prefixed schema files (e.g. 0000-schema.sql) run
// before year-prefixed data files by name ordering alone.
//
// Each line in a data file is an INSERT … ON CONFLICT … WHERE … with
// the inline LWW guard from T-19, which makes replay idempotent and
// order-free: the same file can be replayed any number of times
// against any starting state and the database converges to the same
// result. That property is what lets post-merge hooks (D-18) call
// Import without coordination and what lets the marker be a
// best-effort optimisation rather than a correctness invariant.
//
// Empty changelog directory is not an error — Import returns an
// empty result so first-run scenarios (fresh clone, new project)
// behave cleanly.
func Import(ctx context.Context, db *sql.DB, vaultPath string) (*ImportResult, error) {
	root := filepath.Join(vaultPath, ChangelogDir)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return &ImportResult{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("changelog: stat %s: %w", root, err)
	}

	marker, err := repo.ReadMeta(ctx, db, repo.MetaLastImportMarker)
	if err != nil {
		return nil, err
	}
	cutoff := parseMarker(marker)

	tables, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("changelog: read %s: %w", root, err)
	}
	tableNames := make([]string, 0, len(tables))
	for _, t := range tables {
		if t.IsDir() {
			tableNames = append(tableNames, t.Name())
		}
	}
	sort.Strings(tableNames)

	res := &ImportResult{}
	for _, table := range tableNames {
		dir := filepath.Join(root, table)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("changelog: read %s: %w", dir, err)
		}
		fileNames := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				fileNames = append(fileNames, e.Name())
			}
		}
		sort.Strings(fileNames)

		for _, name := range fileNames {
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				return nil, fmt.Errorf("changelog: stat %s: %w", path, err)
			}
			if !cutoff.IsZero() && !info.ModTime().After(cutoff) {
				continue
			}
			content, err := os.ReadFile(path) //nolint:gosec // G304: path is constructed from a directory we just listed
			if err != nil {
				return nil, fmt.Errorf("changelog: read %s: %w", path, err)
			}
			if _, err := db.ExecContext(ctx, string(content)); err != nil {
				return nil, fmt.Errorf("changelog: replay %s: %w", path, err)
			}
			res.FilesReplayed = append(res.FilesReplayed, path)
			res.StatementsRun += countStatements(content)
		}
	}

	if err := repo.WriteMeta(ctx, db, repo.MetaLastImportMarker,
		time.Now().UTC().Format(markerFormat)); err != nil {
		return nil, err
	}
	return res, nil
}

// parseMarker turns the stored marker string into a time.Time. An
// empty or unparseable marker yields the zero time so every file
// looks "newer".
func parseMarker(m string) time.Time {
	if m == "" {
		return time.Time{}
	}
	t, err := time.Parse(markerFormat, m)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// countStatements approximates the row count for a replayed file by
// counting INSERT lines. Matches the exporter's one-line-per-row
// shape.
func countStatements(content []byte) int {
	n := 0
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "INSERT") {
			n++
		}
	}
	return n
}
