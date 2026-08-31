package parser

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ResolveOutcome reports what ResolveQuestion changed on disk. Every field is
// something a caller has to be able to tell the user, because this verb edits
// the user's own prose and a receipt that only said "ok" would not be checkable.
type ResolveOutcome struct {
	QuestionFile string `json:"question_file"`
	DecisionFile string `json:"decision_file"`
	// StatusLine is the line written into the question record, verbatim.
	StatusLine string `json:"status_line"`
	// BackLink is the line written into the decision record, empty when none
	// was written. BackLinkSkipped then says why.
	BackLink        string `json:"back_link,omitempty"`
	BackLinkSkipped string `json:"back_link_skipped,omitempty"`
}

// ResolveQuestion records a question's resolution into a decision, in the
// markdown, which is the source of truth for the DQR tree (D-8). Writing only
// to pql.db would be undone by the next `decisions sync`.
//
// The question's `**Status:**` line is rewritten to the resolved form. That one
// edit is sufficient for the link to appear on BOTH records: refs are stored
// with a source and a target and looked up on either, so `decisions refs Q-N`
// and `decisions show D-N --with-refs` both surface it from this single line.
//
// The decision record additionally gets a human-readable `**Raised by:**` line,
// but only when it has none. That field is free prose carrying a record's
// provenance — "user feedback during planning usage" and similar — and
// overwriting it to insert a cross-reference the database already holds would
// destroy something to duplicate something. When a line is already there, it is
// left exactly as written and the outcome says so.
func ResolveQuestion(repoRoot string, q, d Record) (ResolveOutcome, error) {
	out := ResolveOutcome{QuestionFile: q.FilePath, DecisionFile: d.FilePath}

	out.StatusLine = fmt.Sprintf("- **Status:** Resolved → %s", markdownLink(q.FilePath, d))
	qAbs := filepath.Join(repoRoot, filepath.FromSlash(q.FilePath))
	if err := editRecord(qAbs, q.ID, func(lines []string) []string {
		for i, line := range lines {
			if statusRe.MatchString(line) {
				lines[i] = out.StatusLine
				return lines
			}
		}
		// No Status line at all — a question written without one parses as
		// open by default, so add the line rather than failing. Position it
		// directly under the heading, where every other record carries it.
		return insertAfterHeading(lines, out.StatusLine)
	}); err != nil {
		return out, err
	}

	backLink := fmt.Sprintf("- **Raised by:** Resolved %s.", markdownLink(d.FilePath, q))
	dAbs := filepath.Join(repoRoot, filepath.FromSlash(d.FilePath))
	err := editRecord(dAbs, d.ID, func(lines []string) []string {
		for _, line := range lines {
			if !raisedByRe.MatchString(line) {
				continue
			}
			if strings.Contains(line, q.ID) {
				out.BackLinkSkipped = "already references " + q.ID
			} else {
				out.BackLinkSkipped = "existing **Raised by:** line left as written"
			}
			return nil // nil means "no change"
		}
		out.BackLink = backLink
		return appendToMetadata(lines, backLink)
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// markdownLink renders a link to target as written from a file at fromPath.
// Both are repo-relative record paths; the link is computed between their
// directories so it works from any domain file to any other, including across
// the questions/ and decisions/ subdirectories.
//
// Record.FilePath comes from filepath.Rel and so carries OS separators, while
// a markdown link must use slashes. The conversions are explicit in both
// directions rather than assumed, because on Windows the unconverted form
// produces a link that renders as literal text.
func markdownLink(fromPath string, target Record) string {
	from := filepath.Dir(filepath.FromSlash(fromPath))
	rel, err := filepath.Rel(from, filepath.FromSlash(target.FilePath))
	if err != nil {
		rel = target.FilePath
	}
	return fmt.Sprintf("[%s](%s#%s)", target.ID, path.Clean(filepath.ToSlash(rel)), Anchor(target))
}

// Anchor returns the heading anchor for a record, matching the slug a
// GitHub-flavored renderer derives from `### <ID>: <Title>`.
func Anchor(r Record) string {
	return slugify(r.ID + ": " + r.Title)
}

// editRecord applies fn to the lines of one record inside a DQR file and
// writes the file back when fn returns a changed slice. fn receives only the
// lines belonging to that record — heading included, up to the line before the
// next `### ` heading — so a rewrite cannot leak into a neighbouring record.
// Returning nil from fn means "leave the file alone".
func editRecord(absPath, id string, fn func(lines []string) []string) error {
	data, err := os.ReadFile(absPath) //nolint:gosec // G304: path derived from a parsed record's own FilePath
	if err != nil {
		return fmt.Errorf("parser: read %s: %w", absPath, err)
	}
	// Preserve the file's trailing-newline shape: splitting on "\n" turns a
	// trailing newline into a final empty element, and rejoining restores it.
	lines := strings.Split(string(data), "\n")

	start := -1
	for i, line := range lines {
		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == id {
			start = i
			continue
		}
		if start >= 0 {
			// Next record's heading ends the previous one.
			return applyAndWrite(absPath, lines, start, i, fn)
		}
	}
	if start < 0 {
		return fmt.Errorf("parser: %s not found in %s", id, absPath)
	}
	return applyAndWrite(absPath, lines, start, len(lines), fn)
}

func applyAndWrite(absPath string, lines []string, start, end int, fn func([]string) []string) error {
	// Copy so fn cannot mutate the surrounding file through the slice it is
	// handed; the record range aliases the same backing array otherwise.
	section := make([]string, end-start)
	copy(section, lines[start:end])

	updated := fn(section)
	if updated == nil {
		return nil
	}

	out := make([]string, 0, len(lines)-(end-start)+len(updated))
	out = append(out, lines[:start]...)
	out = append(out, updated...)
	out = append(out, lines[end:]...)

	//nolint:gosec // G306: DQR markdown is meant to be world-readable, and
	// this matches the mode the rest of the tree already carries.
	if err := os.WriteFile(absPath, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return fmt.Errorf("parser: write %s: %w", absPath, err)
	}
	return nil
}

// insertAfterHeading puts line directly below the record's `### ` heading.
func insertAfterHeading(lines []string, line string) []string {
	if len(lines) == 0 {
		return []string{line}
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[0], line)
	return append(out, lines[1:]...)
}

// appendToMetadata puts line after the record's last `- **Field:**` line,
// keeping the metadata block contiguous rather than stranding the new line
// below prose or a nested list.
func appendToMetadata(lines []string, line string) []string {
	last := 0
	for i, l := range lines {
		if metaFieldRe.MatchString(l) {
			last = i
		}
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:last+1]...)
	out = append(out, line)
	return append(out, lines[last+1:]...)
}
