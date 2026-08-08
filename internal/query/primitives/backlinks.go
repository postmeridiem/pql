package primitives

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// BacklinksOpts configures a Backlinks query.
type BacklinksOpts struct {
	// Path is the file we want backlinks for, vault-relative
	// (e.g. "members/vaasa/persona.md"). Required.
	Path string

	// Limit caps the result set. 0 means "no limit".
	Limit int
}

// Backlinks returns every link whose target plausibly resolves to opts.Path.
//
// The links table stores link targets verbatim — the text between the
// brackets, never a resolved file path — so one file is addressable by several
// spellings and the query has to try all of them. A link counts as a backlink
// to opts.Path when its target equals any of:
//
//   - the path as given                    "members/x/persona.md"
//   - the path without its extension       "members/x/persona"
//   - the bare basename                    "persona"
//
// or any of those followed by a '#' heading anchor.
//
// The extensionless *full path* is the form Obsidian writes for a wikilink
// outside the current folder, and omitting it was a silent-wrong-answer bug
// (T-72): querying the vault-relative path — the spelling every other command
// accepts and returns — matched nothing, so a caller obeying the "zero matches
// means nothing matched" contract reported that a linked file had no backlinks.
//
// Self-references (a file linking to itself) are excluded.
//
// This is still spelling comparison, not resolution: a target that reaches the
// file by a relative prefix ("../x/persona") or by Obsidian's shortest-unique
// path still misses. Normalising link targets at index time is Q-6.
func Backlinks(ctx context.Context, db *sql.DB, opts BacklinksOpts) ([]Backlink, error) {
	if opts.Path == "" {
		return nil, errors.New("primitives.Backlinks: Path is required")
	}

	// Spellings that address this file, deduped: for a path with no extension
	// the first two forms coincide, and for a top-level file all three do.
	forms := []string{opts.Path, strings.TrimSuffix(opts.Path, ".md"), nameFromPath(opts.Path)}
	seen := make(map[string]bool, len(forms))
	var exact []string
	for _, f := range forms {
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		exact = append(exact, f)
	}

	var (
		query strings.Builder
		args  []any
	)
	query.WriteString(`
		SELECT source_path, line, link_type
		FROM links
		WHERE source_path != ?
		  AND (`)
	args = append(args, opts.Path)
	for i, f := range exact {
		if i > 0 {
			query.WriteString(` OR`)
		}
		query.WriteString(` target_path = ? OR target_path LIKE ?`)
		args = append(args, f, f+"#%")
	}
	query.WriteString(`
		      )
		ORDER BY source_path, line`)
	if opts.Limit > 0 {
		query.WriteString(` LIMIT ?`)
		args = append(args, opts.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("primitives.Backlinks: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []Backlink{}
	for rows.Next() {
		var b Backlink
		if err := rows.Scan(&b.Path, &b.Line, &b.Via); err != nil {
			return nil, fmt.Errorf("primitives.Backlinks: scan: %w", err)
		}
		b.Name = nameFromPath(b.Path)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primitives.Backlinks: iterate: %w", err)
	}
	return out, nil
}

