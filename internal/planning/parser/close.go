package parser

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Relation is what closing one record into another means. It is derived from
// the pair of record types rather than chosen by the caller: a question closed
// by a decision was answered, a decision closed by a decision was replaced, and
// the caller should not have to know which word applies.
type Relation string

const (
	// RelResolved is a question answered, by a decision or by a rejection.
	RelResolved Relation = "resolved"
	// RelSuperseded is a decision replaced by a later one.
	RelSuperseded Relation = "superseded"
	// RelObsolete is a record that stopped mattering and is no longer relevant.
	RelObsolete Relation = "obsolete"
)

// CloseOutcome reports what Close changed on disk. Every field is something a
// caller must be able to show the user: this verb edits prose a human wrote,
// so a receipt saying only "ok" would not be checkable.
type CloseOutcome struct {
	Relation Relation `json:"relation"`
	// SubjectFile is the record being closed; TargetFile is what closed it,
	// empty when nothing did.
	SubjectFile string `json:"subject_file"`
	TargetFile  string `json:"target_file,omitempty"`
	// StatusLine is the line written into the subject, verbatim.
	StatusLine string `json:"status_line"`
	// BackLink is the line written into the target, empty when none was.
	// BackLinkSkipped then says why.
	BackLink        string `json:"back_link,omitempty"`
	BackLinkSkipped string `json:"back_link_skipped,omitempty"`
}

// RelationFor derives the relation for a subject/target pair, or explains why
// the pair is not a closure. The error text is user-facing: a caller reaching
// for an unsupported pair has a real disposition in mind and needs to be told
// which supported one it is, not merely that this one is wrong.
func RelationFor(subject, target Record) (Relation, error) {
	switch {
	case subject.ID == target.ID:
		// D -> D is a legal pair, so without this a decision could supersede
		// itself: status and Supersedes line both pointing at one record,
		// which reparses as superseded-by-nothing.
		return "", fmt.Errorf("%s cannot close into itself", subject.ID)

	case subject.Type == typeQuestion && (target.Type == typeConfirmed || target.Type == typeRejected):
		return RelResolved, nil
	case subject.Type == typeConfirmed && target.Type == typeConfirmed:
		return RelSuperseded, nil

	case subject.Type == typeQuestion && target.Type == typeQuestion:
		// The case that produced this verb's original design mistake, so the
		// error is where the next reader is stopped from repeating it.
		return "", fmt.Errorf(
			"%s is a question; a question cannot close into another question\n"+
				"  a question is closed by what ANSWERS it, or not at all:\n"+
				"    --into D-N    a decision that answers it\n"+
				"    --into R-N    a rejection that answers it \"no\"\n"+
				"    --obsolete    it stopped mattering and is no longer relevant\n"+
				"  if %s merely develops or narrows %s, that is a cross-reference\n"+
				"  and is already recorded — leave %s open",
			target.ID, target.ID, subject.ID, subject.ID)

	case subject.Type == typeConfirmed && target.Type == typeQuestion:
		return "", fmt.Errorf(
			"a decision cannot close into a question (%s is a question)\n"+
				"  a decision is closed by the decision that replaces it: --into D-N",
			target.ID)

	case subject.Type == typeRejected:
		return "", fmt.Errorf(
			"%s is a rejection; a rejection records an option already turned down\n"+
				"  and has no further state to close into",
			subject.ID)
	}
	return "", fmt.Errorf("%s (%s) cannot close into %s (%s)",
		subject.ID, subject.Type, target.ID, target.Type)
}

// Close records a record's terminal disposition in the markdown, which is the
// source of truth for the DQR tree (D-8) — writing only to pql.db would be
// undone by the next `decisions sync`.
//
// A nil target closes the subject as obsolete: it stopped mattering and is no
// longer relevant — the subsystem it asked about went away, the constraint that
// raised it lifted, the framing turned out to be wrong. That is distinct from resolution, and
// inventing a record to absorb it would be manufacturing a decision nobody
// made.
//
// The status is written so the parser reads it back. For resolution and
// obsolescence that means a line beginning "Resolved", which is the only
// terminal bucket the status vocabulary has; for supersession, one beginning
// "Superseded by". Neither may hedge — "in part" and "partially" keep a record
// live on purpose (T-119), so this verb never writes them.
//
// The target's cross-reference line is written only when the field is absent.
// An existing one carries prose a human wrote about where a record came from,
// and overwriting it to insert a link the database already holds would destroy
// something in order to duplicate something.
func Close(repoRoot string, subject Record, target *Record) (CloseOutcome, error) {
	out := CloseOutcome{SubjectFile: subject.FilePath}

	var backLinkField, backLinkLine string
	if target == nil {
		out.Relation = RelObsolete
		out.StatusLine = "- **Status:** Resolved as obsolete — it stopped mattering"
	} else {
		rel, err := RelationFor(subject, *target)
		if err != nil {
			return out, err
		}
		out.Relation = rel
		out.TargetFile = target.FilePath

		if rel == RelSuperseded {
			out.StatusLine = "- **Status:** Superseded by " + markdownLink(subject.FilePath, *target)
			backLinkField = "Supersedes"
			backLinkLine = "- **Supersedes:** " + markdownLink(target.FilePath, subject)
		} else {
			out.StatusLine = "- **Status:** Resolved → " + markdownLink(subject.FilePath, *target)
			backLinkField = "Raised by"
			backLinkLine = "- **Raised by:** Resolved " + markdownLink(target.FilePath, subject) + "."
		}
	}

	subjectAbs := filepath.Join(repoRoot, filepath.FromSlash(subject.FilePath))
	if err := editRecord(subjectAbs, subject.ID, func(lines []string) []string {
		for i, line := range lines {
			if statusRe.MatchString(line) {
				lines[i] = out.StatusLine
				return lines
			}
		}
		// No Status line at all: a record without one falls back to its
		// type's default, so the line has to be added rather than replaced.
		return insertAfterHeading(lines, out.StatusLine)
	}); err != nil {
		return out, err
	}

	if target == nil {
		return out, nil
	}

	targetAbs := filepath.Join(repoRoot, filepath.FromSlash(target.FilePath))
	err := editRecord(targetAbs, target.ID, func(lines []string) []string {
		for _, line := range lines {
			if !hasMetaField(line, backLinkField) {
				continue
			}
			if strings.Contains(line, subject.ID) {
				out.BackLinkSkipped = "already references " + subject.ID
			} else {
				out.BackLinkSkipped = fmt.Sprintf("existing **%s:** line left as written", backLinkField)
			}
			return nil // nil means "no change"
		}
		out.BackLink = backLinkLine
		return appendToMetadata(lines, backLinkLine)
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// hasMetaField reports whether line is the named `- **Field:**` metadata line,
// case-insensitively.
func hasMetaField(line, field string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(trimmed, "- **"+strings.ToLower(field)+":**")
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
	// Same file — a bare anchor, which is what every hand-written
	// cross-reference in the tree already uses. Supersession is usually
	// within one domain file, so this is the common case rather than an edge.
	if fromPath == target.FilePath {
		return fmt.Sprintf("[%s](#%s)", target.ID, Anchor(target))
	}
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
