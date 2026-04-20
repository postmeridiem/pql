// Package ignore loads per-vault gitignore-syntax exclusion files and
// produces a Matcher the walker consults for each candidate path.
//
// v1 reads each named file at the vault root only — no nested cascade.
// Order matters: lines from later files appear after earlier ones, so a
// later file can `!`-re-include something an earlier file excluded
// (standard gitignore precedence).
//
// Files that don't exist are silently skipped. The matcher returned by
// Load is always non-nil, including the case where every named file is
// missing — callers can call Matches without nil-checking.
package ignore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Matcher reports whether a vault-relative path is excluded by the
// composite ignore rules. Implementations are safe for concurrent reads
// after Load returns.
type Matcher interface {
	Matches(path string) bool
}

// Load reads each named file at <vaultPath>/<name> in order and produces
// a composite Matcher. Returns a no-op matcher when no files are named or
// none of the named files exist.
//
// The composition rule is "concatenate then compile" — every line from
// every file goes into one big gitignore document, in order. That keeps
// `!` re-inclusion semantics behaving the way users expect from .gitignore
// itself, including across files: a later file's `!path` can re-include
// what an earlier file excluded.
func Load(vaultPath string, files []string) (Matcher, error) {
	if len(files) == 0 {
		return noopMatcher{}, nil
	}
	var lines []string
	for _, name := range files {
		path := filepath.Join(vaultPath, name)
		body, err := os.ReadFile(path) //nolint:gosec // G304: path is vaultPath/<configured ignore filename>; reading user-named ignore files is the feature
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("ignore: read %s: %w", path, err)
		}
		lines = append(lines, splitLines(string(body))...)
	}
	if len(lines) == 0 {
		return noopMatcher{}, nil
	}
	return &compiledMatcher{gi: gitignore.CompileIgnoreLines(lines...)}, nil
}

// splitLines splits gitignore content on \n, normalising CRLF endings.
// Empty trailing lines are kept (the gitignore parser handles them
// harmlessly) so file bytes round-trip cleanly.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}

type compiledMatcher struct {
	gi *gitignore.GitIgnore
}

func (c *compiledMatcher) Matches(path string) bool {
	return c.gi.MatchesPath(path)
}

// noopMatcher is the zero-rules matcher. Returned when no ignore files
// are configured or all named files are missing.
type noopMatcher struct{}

func (noopMatcher) Matches(string) bool { return false }
