package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/index/extractor/markdown"
	"github.com/postmeridiem/pql/internal/store"
)

// Indexer ties Walk + extractor + store together. One Indexer per pql
// invocation; not safe for concurrent Run calls (single writer at a time
// per the design philosophy on "one transaction per indexer invocation").
type Indexer struct {
	store *store.Store
	cfg   *config.Config
	now   func() time.Time // injectable for tests
}

// Stats summarises a Run for diagnostic output (`pql doctor`, --verbose).
type Stats struct {
	Walked     int   // files Walk enumerated
	Indexed    int   // files that were re-extracted and upserted
	Unchanged  int   // files skipped because mtime + hash matched
	Removed    int   // files deleted from index because they're gone from disk
	DurationMS int64 // wall time of the full Run
}

// New constructs an Indexer bound to a store and config. The caller owns
// both — Indexer.Run does not Close them.
func New(s *store.Store, cfg *config.Config) *Indexer {
	return &Indexer{store: s, cfg: cfg, now: time.Now}
}

// Run walks the vault, re-indexes changed files, and prunes rows for files
// that disappeared from disk. Everything happens inside a single transaction
// so a crash mid-run leaves the index in its pre-run state.
//
// Incremental update logic per file:
//
//   - row exists, mtime matches    → skip (Unchanged++)
//   - row exists, mtime differs but content hash unchanged → just touch mtime
//   - row absent OR content hash differs → DELETE row (cascade purges
//     dependent frontmatter/tags/links/headings) and INSERT fresh
//
// Pruning is the simple set difference: anything in `files` not seen by
// Walk is DELETEd (cascade does the rest).
func (idx *Indexer) Run(ctx context.Context) (Stats, error) {
	start := idx.now()
	stats := Stats{}

	files, err := Walk(WalkOpts{
		VaultPath:   idx.cfg.Vault.Path,
		Exclude:     idx.cfg.Exclude,
		IgnoreFiles: idx.cfg.IgnoreFiles,
	})
	if err != nil {
		return stats, fmt.Errorf("indexer: walk: %w", err)
	}
	stats.Walked = len(files)

	tx, err := idx.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("indexer: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // noop after a successful Commit

	seen := make(map[string]struct{}, len(files))
	extractOpts := markdown.ExtractOpts{TagSources: idx.cfg.Tags.Sources}
	scanTime := idx.now().Unix()

	for _, rel := range files {
		seen[rel] = struct{}{}
		ind, err := idx.indexOne(ctx, tx, rel, extractOpts, scanTime)
		if err != nil {
			return stats, err
		}
		switch ind {
		case actionIndexed:
			stats.Indexed++
		case actionUnchanged:
			stats.Unchanged++
		}
	}

	removed, err := pruneStale(ctx, tx, seen)
	if err != nil {
		return stats, err
	}
	stats.Removed = removed

	cfgHash, err := idx.cfg.Hash()
	if err != nil {
		return stats, fmt.Errorf("indexer: cfg hash: %w", err)
	}
	if err := upsertMeta(ctx, tx, "config_hash", cfgHash); err != nil {
		return stats, err
	}
	if err := upsertMeta(ctx, tx, "last_full_scan", strconv.FormatInt(scanTime, 10)); err != nil {
		return stats, err
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("indexer: commit: %w", err)
	}
	stats.DurationMS = idx.now().Sub(start).Milliseconds()
	return stats, nil
}

type indexAction int

const (
	actionUnchanged indexAction = iota
	actionIndexed
)

func (idx *Indexer) indexOne(ctx context.Context, tx *sql.Tx, rel string, opts markdown.ExtractOpts, scanTime int64) (indexAction, error) {
	full := filepath.Join(idx.cfg.Vault.Path, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		return actionUnchanged, fmt.Errorf("indexer: stat %q: %w", rel, err)
	}
	mtime := info.ModTime().Unix()
	size := info.Size()

	var existingMtime int64
	var existingHash string
	probeErr := tx.QueryRowContext(ctx,
		`SELECT mtime, content_hash FROM files WHERE path = ?`, rel,
	).Scan(&existingMtime, &existingHash)
	if probeErr != nil && !errors.Is(probeErr, sql.ErrNoRows) {
		return actionUnchanged, fmt.Errorf("indexer: probe %q: %w", rel, probeErr)
	}
	known := probeErr == nil

	if known && existingMtime == mtime {
		return actionUnchanged, nil
	}

	body, err := os.ReadFile(full)
	if err != nil {
		return actionUnchanged, fmt.Errorf("indexer: read %q: %w", rel, err)
	}
	hash := sha256Hex(body)

	if known && existingHash == hash {
		// Content unchanged — just refresh mtime + last_scanned.
		if _, err := tx.ExecContext(ctx,
			`UPDATE files SET mtime = ?, last_scanned = ? WHERE path = ?`,
			mtime, scanTime, rel,
		); err != nil {
			return actionUnchanged, fmt.Errorf("indexer: touch %q: %w", rel, err)
		}
		return actionUnchanged, nil
	}

	res, err := markdown.Extract(body, opts)
	if err != nil {
		return actionUnchanged, fmt.Errorf("indexer: extract %q: %w", rel, err)
	}

	if known {
		if _, err := tx.ExecContext(ctx, `DELETE FROM files WHERE path = ?`, rel); err != nil {
			return actionUnchanged, fmt.Errorf("indexer: delete %q: %w", rel, err)
		}
	}
	if err := insertFile(ctx, tx, rel, mtime, mtime, size, hash, scanTime); err != nil {
		return actionUnchanged, err
	}
	if err := insertFrontmatter(ctx, tx, rel, res.Frontmatter); err != nil {
		return actionUnchanged, err
	}
	if err := insertTags(ctx, tx, rel, res.Tags); err != nil {
		return actionUnchanged, err
	}
	if err := insertLinks(ctx, tx, rel, res.Links); err != nil {
		return actionUnchanged, err
	}
	if err := insertHeadings(ctx, tx, rel, res.Headings); err != nil {
		return actionUnchanged, err
	}
	return actionIndexed, nil
}

func pruneStale(ctx context.Context, tx *sql.Tx, seen map[string]struct{}) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT path FROM files`)
	if err != nil {
		return 0, fmt.Errorf("indexer: list existing: %w", err)
	}
	var stale []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("indexer: scan path: %w", err)
		}
		if _, ok := seen[p]; !ok {
			stale = append(stale, p)
		}
	}
	_ = rows.Close()

	for _, p := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM files WHERE path = ?`, p); err != nil {
			return 0, fmt.Errorf("indexer: remove stale %q: %w", p, err)
		}
	}
	return len(stale), nil
}

func insertFile(ctx context.Context, tx *sql.Tx, path string, mtime, ctime, size int64, hash string, scanTime int64) error {
	// ctime is approximated as mtime in v1 — true cross-platform creation
	// time isn't reliably available without per-OS Stat_t inspection.
	// Documented in initial-plan.md.
	_, err := tx.ExecContext(ctx,
		`INSERT INTO files (path, mtime, ctime, size, content_hash, last_scanned)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		path, mtime, ctime, size, hash, scanTime,
	)
	if err != nil {
		return fmt.Errorf("indexer: insert file %q: %w", path, err)
	}
	return nil
}

func insertFrontmatter(ctx context.Context, tx *sql.Tx, path string, fm map[string]markdown.Value) error {
	for k, v := range fm {
		var text sql.NullString
		if v.HasText {
			text = sql.NullString{String: v.Text, Valid: true}
		}
		var num sql.NullFloat64
		if v.HasNum {
			num = sql.NullFloat64{Float64: v.Num, Valid: true}
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO frontmatter (path, key, type, value_json, value_text, value_num)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			path, k, v.Type, v.JSON, text, num,
		)
		if err != nil {
			return fmt.Errorf("indexer: insert frontmatter %q[%q]: %w", path, k, err)
		}
	}
	return nil
}

func insertTags(ctx context.Context, tx *sql.Tx, path string, tags []string) error {
	for _, t := range tags {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO tags (path, tag) VALUES (?, ?)`,
			path, t,
		)
		if err != nil {
			return fmt.Errorf("indexer: insert tag %q[%q]: %w", path, t, err)
		}
	}
	return nil
}

func insertLinks(ctx context.Context, tx *sql.Tx, path string, links []markdown.Link) error {
	for _, l := range links {
		var alias sql.NullString
		if l.Alias != "" {
			alias = sql.NullString{String: l.Alias, Valid: true}
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO links (source_path, target_path, alias, link_type, line)
			 VALUES (?, ?, ?, ?, ?)`,
			path, l.Target, alias, l.Type, l.Line,
		)
		if err != nil {
			return fmt.Errorf("indexer: insert link %q→%q: %w", path, l.Target, err)
		}
	}
	return nil
}

func insertHeadings(ctx context.Context, tx *sql.Tx, path string, hs []markdown.Heading) error {
	for _, h := range hs {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO headings (path, depth, text, line_offset)
			 VALUES (?, ?, ?, ?)`,
			path, h.Depth, h.Text, h.LineOffset,
		)
		if err != nil {
			return fmt.Errorf("indexer: insert heading %q[%q]: %w", path, h.Text, err)
		}
	}
	return nil
}

func upsertMeta(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO index_meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("indexer: upsert meta %q: %w", key, err)
	}
	return nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
