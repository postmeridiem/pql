package primitives

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// MetaOpts configures a Meta query.
type MetaOpts struct {
	// Path is the file we want metadata for, vault-relative. Required.
	Path string
}

// MetaOne returns the per-file aggregate for opts.Path: filesystem metadata,
// frontmatter (raw JSON values keyed by frontmatter key), tags, outlinks,
// and headings. Returns (nil, nil) when the file isn't indexed — the CLI
// surface (runQueryOne) maps that to exit code 66 (NoInput).
//
// Each piece is fetched in its own SELECT for clarity; a single JOIN
// would interleave rows awkwardly because each child table has different
// cardinality per parent. Five small queries against a primary-key parent
// are cheap and don't touch the network.
func MetaOne(ctx context.Context, db *sql.DB, opts MetaOpts) (*Meta, error) {
	if opts.Path == "" {
		return nil, errors.New("primitives.Meta: Path is required")
	}

	m := &Meta{
		Path:        opts.Path,
		Name:        nameFromPath(opts.Path),
		Frontmatter: map[string]json.RawMessage{},
		Tags:        []string{},
		Outlinks:    []Outlink{},
		Headings:    []Heading{},
	}

	// File row — also serves as the existence check.
	row := db.QueryRowContext(ctx,
		`SELECT size, mtime FROM files WHERE path = ?`, opts.Path)
	if err := row.Scan(&m.Size, &m.Mtime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("primitives.Meta: file row: %w", err)
	}

	// Frontmatter — raw JSON values straight from the value_json column.
	frows, err := db.QueryContext(ctx,
		`SELECT key, value_json FROM frontmatter WHERE path = ? ORDER BY key`,
		opts.Path,
	)
	if err != nil {
		return nil, fmt.Errorf("primitives.Meta: frontmatter query: %w", err)
	}
	for frows.Next() {
		var key string
		var raw []byte
		if err := frows.Scan(&key, &raw); err != nil {
			_ = frows.Close()
			return nil, fmt.Errorf("primitives.Meta: scan frontmatter: %w", err)
		}
		// Copy because []byte from Scan is reused across iterations.
		buf := make([]byte, len(raw))
		copy(buf, raw)
		m.Frontmatter[key] = buf
	}
	if err := frows.Err(); err != nil {
		_ = frows.Close()
		return nil, fmt.Errorf("primitives.Meta: iterate frontmatter: %w", err)
	}
	_ = frows.Close()

	// Tags — sorted for stable output.
	trows, err := db.QueryContext(ctx,
		`SELECT tag FROM tags WHERE path = ? ORDER BY tag`, opts.Path)
	if err != nil {
		return nil, fmt.Errorf("primitives.Meta: tags query: %w", err)
	}
	for trows.Next() {
		var t string
		if err := trows.Scan(&t); err != nil {
			_ = trows.Close()
			return nil, fmt.Errorf("primitives.Meta: scan tag: %w", err)
		}
		m.Tags = append(m.Tags, t)
	}
	if err := trows.Err(); err != nil {
		_ = trows.Close()
		return nil, fmt.Errorf("primitives.Meta: iterate tags: %w", err)
	}
	_ = trows.Close()

	// Outlinks — document order.
	lrows, err := db.QueryContext(ctx,
		`SELECT target_path, alias, line, link_type
		 FROM links WHERE source_path = ? ORDER BY line, target_path`,
		opts.Path,
	)
	if err != nil {
		return nil, fmt.Errorf("primitives.Meta: links query: %w", err)
	}
	for lrows.Next() {
		var (
			o     Outlink
			alias sql.NullString
		)
		if err := lrows.Scan(&o.Target, &alias, &o.Line, &o.Via); err != nil {
			_ = lrows.Close()
			return nil, fmt.Errorf("primitives.Meta: scan link: %w", err)
		}
		if alias.Valid {
			o.Alias = alias.String
		}
		m.Outlinks = append(m.Outlinks, o)
	}
	if err := lrows.Err(); err != nil {
		_ = lrows.Close()
		return nil, fmt.Errorf("primitives.Meta: iterate links: %w", err)
	}
	_ = lrows.Close()

	// Headings — document order via line_offset.
	hrows, err := db.QueryContext(ctx,
		`SELECT depth, text, line_offset FROM headings WHERE path = ?
		 ORDER BY line_offset`,
		opts.Path,
	)
	if err != nil {
		return nil, fmt.Errorf("primitives.Meta: headings query: %w", err)
	}
	for hrows.Next() {
		var h Heading
		if err := hrows.Scan(&h.Depth, &h.Text, &h.LineOffset); err != nil {
			_ = hrows.Close()
			return nil, fmt.Errorf("primitives.Meta: scan heading: %w", err)
		}
		m.Headings = append(m.Headings, h)
	}
	if err := hrows.Err(); err != nil {
		_ = hrows.Close()
		return nil, fmt.Errorf("primitives.Meta: iterate headings: %w", err)
	}
	_ = hrows.Close()

	return m, nil
}
