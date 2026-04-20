// Package index walks the vault, parses each markdown file's structured
// pieces (frontmatter, wikilinks, tags, headings), and upserts the result
// into the SQLite store. The walker is the entry point.
//
// The walker prunes excluded directories rather than walking-then-filtering
// — `.git/` with thousands of internal files costs zero scanning time. See
// docs/vault-layout.md and docs/pqlignore.md for the exclusion model;
// .pqlignore integration lands in its own commit.
package index

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// markdownExt is the only file extension the v1 walker considers. .markdown
// support is a candidate for a later config flag if anyone asks.
const markdownExt = ".md"

// builtinExcludes are non-overridable. We deliberately match git's own
// built-in exactly: just `.git/` and nothing else. Git knows about its
// state dir without anyone listing it in `.gitignore`; pql does the
// same so the starting point is identical and the user's level of
// control matches what they already expect from git.
//
// Everything else — pql's own `.pql/`, sqlite sidecars, node_modules/,
// build outputs — is the user-config layer's responsibility via
// `ignore_files` (defaults to .gitignore, which `pql init` appends
// `.pql/` to) and `exclude:`. If a user wants to index node_modules
// for searchability, that's their call to make.
//
// The walker only enumerates; per-extractor filters (markdown today,
// tree-sitter later) decide what each file actually means, so we don't
// need to defensively block file types here.
var builtinExcludes = []string{
	"**/.git",
	"**/.git/**",
}

// WalkOpts configures a single Walk invocation.
type WalkOpts struct {
	// VaultPath is the absolute root to walk. Required.
	VaultPath string
	// Exclude is the user's exclusion list (typically Config.Exclude from
	// .pql/config.yaml). Doublestar patterns matched against vault-relative paths
	// using forward slashes. Built-in excludes are always applied on top.
	Exclude []string
}

// Walk enumerates indexable .md files under opts.VaultPath. Returns paths
// relative to the vault root, using forward slashes (so they're stable
// across platforms and ready to use as the primary key in `files.path`).
// Sorted lexicographically for deterministic output.
//
// Directories matching exclude patterns are pruned — never descended into.
// Patterns are matched against vault-relative paths; e.g. `**/.obsidian/**`
// matches both `.obsidian` at the root and `members/foo/.obsidian`.
func Walk(opts WalkOpts) ([]string, error) {
	if opts.VaultPath == "" {
		return nil, errors.New("index: VaultPath required")
	}
	root, err := filepath.Abs(opts.VaultPath)
	if err != nil {
		return nil, fmt.Errorf("index: resolve vault path: %w", err)
	}

	patterns := append(append([]string(nil), builtinExcludes...), opts.Exclude...)

	var files []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Surface stat / permission errors so callers can decide; don't
			// silently skip — that masks real problems.
			return fmt.Errorf("index: walk %q: %w", path, err)
		}
		rel, err := vaultRel(root, path)
		if err != nil {
			return err
		}
		// Root itself is "."; never match against it.
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if matchesAny(patterns, rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), markdownExt) {
			return nil
		}
		if matchesAny(patterns, rel) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return files, nil
}

// vaultRel produces a vault-relative path with forward slashes regardless
// of the host OS, so paths are stable for storage and comparison.
func vaultRel(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("index: relative path: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

// matchesAny reports whether rel matches any doublestar pattern. Returns
// false on the first malformed pattern (caller-supplied patterns are not
// validated upfront — invalid ones simply fail to match anything).
func matchesAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		ok, err := doublestar.Match(p, rel)
		if err != nil {
			continue
		}
		if ok {
			return true
		}
	}
	return false
}

// Stat is a small struct returned alongside Walk results in the future when
// the indexer needs mtime/size/etc. without re-stat'ing. Reserved name.
type Stat = os.FileInfo
