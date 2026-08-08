package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// staged holds every changelog row of one table, in the order the files
// present them, keyed back to the file it came from.
//
// The staging database is doing one job a regex cannot: parsing. A committed
// changelog line is SQL — doubled quotes, embedded newlines, bare NULLs — and
// the only thing guaranteed to read it the way replay will is SQLite itself.
// Executing the INSERT *is* the parse; the row lands as typed columns, and
// re-emitting from those columns produces a line that agrees with what a normal
// export would have written.
type staged struct {
	db *sql.DB
}

// stagingSchema builds the relaxed mirror of a replicated table: the canonical
// columns with no primary key, no UNIQUE, no foreign keys, plus seq and
// src_file.
//
// Same table *name* as the real thing, so committed `INSERT INTO tickets (…)`
// statements execute unchanged. Relaxed *shape*, so every row is kept rather
// than the last writer winning — which is the whole point. Staging under the
// canonical schema would let the old guard resolve the rows on the way in and
// bake the loss being repaired into the output.
func stagingSchema(spec tableSpec) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE " + spec.Name + " (\n")
	b.WriteString("  seq INTEGER PRIMARY KEY AUTOINCREMENT,\n")
	b.WriteString("  src_file TEXT,\n")
	for i, c := range spec.Columns {
		b.WriteString("  " + c + " TEXT")
		if i < len(spec.Columns)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(");")
	return b.String()
}

// newStaging opens an in-memory database carrying a relaxed mirror of every
// replicated table.
func newStaging(ctx context.Context) (*staged, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("changelog: open staging db: %w", err)
	}
	for _, spec := range changelogTables {
		if _, err := db.ExecContext(ctx, stagingSchema(spec)); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("changelog: staging schema for %s: %w", spec.Name, err)
		}
	}
	return &staged{db: db}, nil
}

func (s *staged) close() { _ = s.db.Close() }

// stripConflictClause removes the trailing ON CONFLICT action from a changelog
// statement, leaving a plain INSERT.
//
// This is a structural edit — dropping a clause whose boundary is unambiguous —
// not a rewrite of row contents, which stays SQLite's job. It has to happen:
// the clause carries whichever guard the file was written under, and executing
// it during staging would let the *old* rule pick a winner before the new rule
// ever gets a chance.
//
// The scan is quote-aware because a value may legitimately contain the text
// "ON CONFLICT".
func stripConflictClause(stmt string) string {
	const marker = ") ON CONFLICT("
	inStr := false
	for i := 0; i < len(stmt); i++ {
		if stmt[i] == '\'' {
			if inStr && i+1 < len(stmt) && stmt[i+1] == '\'' {
				i++ // escaped quote, stays inside the literal
				continue
			}
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if strings.HasPrefix(stmt[i:], marker) {
			return stmt[:i+1] + ";"
		}
	}
	return stmt
}

// loadTable stages every data file of one table directory, in replay order.
// Schema fixtures are skipped: they carry CREATE TABLE statements that would
// fight the relaxed staging shape, and they are not data.
func (s *staged) loadTable(ctx context.Context, spec tableSpec, tableDir string) ([]string, error) {
	entries, err := os.ReadDir(tableDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("changelog: read %s: %w", tableDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") && !isSchemaFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	// Sorted for the same reason Import sorts: file order is replay order, and
	// replay order is what decides an equal-timestamp tie.
	sort.Strings(names)

	loaded := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(tableDir, name)
		body, err := os.ReadFile(path) //nolint:gosec // G304: path built from a directory just listed
		if err != nil {
			return nil, fmt.Errorf("changelog: read %s: %w", path, err)
		}
		for _, stmt := range splitStatements(string(body)) {
			if !strings.HasPrefix(stmt, "INSERT INTO ") {
				continue // comments and anything else are not rows
			}
			if _, err := s.db.ExecContext(ctx, stripConflictClause(stmt)); err != nil {
				return nil, fmt.Errorf("changelog: stage %s: %w", path, err)
			}
		}
		// Attribute everything staged so far to this file. Rows keep their
		// original file on re-emit rather than being re-bucketed by month, so
		// an upgrade never moves a row between files.
		if _, err := s.db.ExecContext(ctx,
			"UPDATE "+spec.Name+" SET src_file = ? WHERE src_file IS NULL", name); err != nil { //nolint:gosec // G202: closed-set table name from the spec list
			return nil, fmt.Errorf("changelog: attribute rows of %s: %w", path, err)
		}
		loaded = append(loaded, name)
	}
	return loaded, nil
}

// renderFile re-emits one source file's rows, in staging order, through the
// current statement renderer.
//
// Rows identical across every canonical column collapse to one line. On a
// healthy changelog that is a no-op — fileSink already refuses to append a
// byte-identical line. It matters after a union merge between an upgraded and a
// non-upgraded clone, where the same row arrives twice differing only in the
// guard clause it carried; without this the doubling would persist forever.
func (s *staged) renderFile(ctx context.Context, spec tableSpec, srcFile string) ([]string, error) {
	//nolint:gosec // G202: table and column names come from the closed spec list, never from input
	query := "SELECT " + strings.Join(spec.Columns, ", ") +
		" FROM " + spec.Name + " WHERE src_file = ? ORDER BY seq"
	rows, err := s.db.QueryContext(ctx, query, srcFile)
	if err != nil {
		return nil, fmt.Errorf("changelog: read staged %s: %w", spec.Name, err)
	}
	defer func() { _ = rows.Close() }()

	var lines []string
	seen := make(map[string]bool)
	for rows.Next() {
		vals := make([]sql.NullString, len(spec.Columns))
		dest := make([]any, len(vals))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("changelog: scan staged %s: %w", spec.Name, err)
		}
		literals := make([]string, len(vals))
		for i, v := range vals {
			literals[i] = stagedLiteral(spec.Columns[i], v)
		}
		line, err := spec.render(literals)
		if err != nil {
			return nil, err
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

// stagedLiteral turns a staged column value back into the SQL literal the
// exporter would have written for it.
//
// canonical_version is the one numeric column in the set: the exporter emits it
// bare (sqlNullInt), and re-quoting it as a string would change the bytes and
// break the byte-for-byte agreement two clones depend on.
func stagedLiteral(column string, v sql.NullString) string {
	if !v.Valid {
		return sqlNull
	}
	if column == colCanonicalVersion {
		return v.String
	}
	return sqlStr(v.String)
}
