// Package parser reads decisions/*.md files and extracts structured
// records. The format matches the convention documented in clide's
// decisions/README.md: ### D|Q|R-NNN: Title headings, field lines,
// inline amendments, and cross-references.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	headingRe     = regexp.MustCompile(`^###\s+([DQR]-\d+):\s+(.+)$`)
	dateRe        = regexp.MustCompile(`^\s*-\s+\*\*(?:Date|Rejected):\*\*\s+(\d{4}-\d{2}-\d{2})`)
	statusRe      = regexp.MustCompile(`^\s*-\s+\*\*Status:\*\*\s+(.+)`)
	supersedesRe  = regexp.MustCompile(`(?i)^\s*-\s+\*\*Supersedes:\*\*`)
	supersededRe  = regexp.MustCompile(`(?i)^\s*-\s+\*\*Superseded\s+by:\*\*`)
	resolvesRe    = regexp.MustCompile(`(?i)^\s*-\s+\*\*Resolves:\*\*`)
	dependsRe     = regexp.MustCompile(`(?i)^\s*-\s+\*\*Depends\s+on:\*\*`)
	amendsRe      = regexp.MustCompile(`(?i)^\s*\*\*Amendment\s*\(`)
	refIDRe       = regexp.MustCompile(`\b[DQRT]-\d+\b`)
)

var typeFromPrefix = map[byte]string{
	'D': "confirmed",
	'Q': "question",
	'R': "rejected",
}

// Record is one parsed decision/question/rejected record.
type Record struct {
	ID       string
	Type     string // confirmed | question | rejected
	Domain   string
	Title    string
	Status   string // active | superseded | resolved | open
	Date     string // YYYY-MM-DD or empty
	FilePath string // relative to repo root
	Refs     []Ref
	Body     string // raw markdown between this heading and the next
}

// Ref is a cross-reference extracted from a record's body.
type Ref struct {
	TargetID string
	RefType  string // supersedes | references | resolves | depends_on | amends
	Note     string
}

// ParseFile reads a single decisions/*.md file and returns all records found.
// Domain is inferred from the filename (stripped of "questions-" prefix).
// filePath is recorded as the path relative to repoRoot.
func ParseFile(path, repoRoot string) ([]Record, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path discovered by caller
	if err != nil {
		return nil, fmt.Errorf("parser: read %s: %w", path, err)
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		rel = path
	}
	domain := domainFromFilename(filepath.Base(path))
	return parseText(string(data), domain, rel), nil
}

func domainFromFilename(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.HasPrefix(stem, "questions-") {
		return stem[len("questions-"):]
	}
	return stem
}

func parseText(text, domain, filePath string) []Record {
	var records []Record
	var curID, curTitle string
	var curBody []string

	flush := func() {
		if curID == "" {
			return
		}
		recType := typeFromPrefix[curID[0]]
		rec := Record{
			ID:       curID,
			Type:     recType,
			Domain:   domain,
			Title:    strings.TrimSpace(curTitle),
			Status:   inferStatus(recType, curBody),
			Date:     extractDate(curBody),
			FilePath: filePath,
			Refs:     extractRefs(curID, curBody),
			Body:     trimBody(stripMetaLines(curBody)),
		}
		records = append(records, rec)
		curID = ""
		curTitle = ""
		curBody = nil
	}

	for _, line := range strings.Split(text, "\n") {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			flush()
			curID = m[1]
			curTitle = m[2]
			curBody = nil
		} else if curID != "" {
			if strings.TrimSpace(line) == "---" {
				flush()
			} else {
				curBody = append(curBody, line)
			}
		}
	}
	flush()
	return records
}

func inferStatus(recType string, body []string) string {
	if recType == "rejected" {
		return "active"
	}
	for _, line := range body {
		if supersededRe.MatchString(line) {
			return "superseded"
		}
	}
	if recType == "question" {
		for _, line := range body {
			m := statusRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			s := strings.ToLower(strings.TrimSpace(m[1]))
			if strings.Contains(s, "partial") || strings.Contains(s, "remaining") {
				return "open"
			}
			if strings.HasPrefix(s, "resolved") {
				return "resolved"
			}
			return "open"
		}
		return "open"
	}
	return "active"
}

func extractDate(body []string) string {
	for _, line := range body {
		if m := dateRe.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

func extractRefs(recID string, body []string) []Ref {
	var refs []Ref
	seen := make(map[string]bool)

	for _, line := range body {
		refType := classifyRefLine(line)
		for _, target := range refIDRe.FindAllString(line, -1) {
			if target == recID || target[0] == 'T' {
				continue
			}
			key := target + "|" + refType
			if seen[key] {
				continue
			}
			seen[key] = true
			note := strings.TrimSpace(line)
			note = strings.TrimLeft(note, "- ")
			if len(note) > 200 {
				note = note[:197] + "..."
			}
			refs = append(refs, Ref{
				TargetID: target,
				RefType:  refType,
				Note:     note,
			})
		}
	}
	return refs
}

func classifyRefLine(line string) string {
	switch {
	case supersedesRe.MatchString(line):
		return "supersedes"
	case supersededRe.MatchString(line):
		return "references"
	case resolvesRe.MatchString(line):
		return "resolves"
	case dependsRe.MatchString(line):
		return "depends_on"
	case amendsRe.MatchString(line):
		return "amends"
	default:
		return "references"
	}
}

// ParseAll parses every *.md file in decisionsDir (skipping README.md),
// returning all records and any warnings. repoRoot is the project root
// used to compute relative file paths.
func ParseAll(decisionsDir, repoRoot string) ([]Record, []string, error) {
	entries, err := os.ReadDir(decisionsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("parser: read %s: %w", decisionsDir, err)
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if strings.EqualFold(e.Name(), "readme.md") {
			continue
		}
		paths = append(paths, filepath.Join(decisionsDir, e.Name()))
	}
	sort.Strings(paths)

	var records []Record
	var warnings []string
	for _, p := range paths {
		recs, err := ParseFile(p, repoRoot)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("error parsing %s: %v", filepath.Base(p), err))
			continue
		}
		records = append(records, recs...)
	}
	return records, warnings, nil
}

// Validate runs a dry parse and returns (ok, errors). Non-zero errors
// fail push-check.
func Validate(decisionsDir, repoRoot string) (ok bool, errs []string) {
	records, warnings, err := ParseAll(decisionsDir, repoRoot)
	if err != nil {
		return false, []string{err.Error()}
	}
	errs = append(errs, warnings...)

	idSeen := make(map[string]string)
	for _, rec := range records {
		if prev, ok := idSeen[rec.ID]; ok && prev != rec.FilePath {
			errs = append(errs, fmt.Sprintf(
				"duplicate id %s: %s and %s", rec.ID, prev, rec.FilePath))
		}
		idSeen[rec.ID] = rec.FilePath
		if rec.Title == "" {
			errs = append(errs, fmt.Sprintf("%s: empty title", rec.ID))
		}
	}

	known := make(map[string]bool)
	for id := range idSeen {
		known[id] = true
	}
	for _, rec := range records {
		for _, ref := range rec.Refs {
			if !known[ref.TargetID] {
				errs = append(errs, fmt.Sprintf(
					"%s -> %s (%s): target not found", rec.ID, ref.TargetID, ref.RefType))
			}
		}
	}
	return len(errs) == 0, errs
}

func stripMetaLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if dateRe.MatchString(line) || statusRe.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return out
}

func trimBody(lines []string) string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

// FindRecord parses a single file and returns the record with the given ID.
func FindRecord(path, repoRoot, id string) (*Record, error) {
	records, err := ParseFile(path, repoRoot)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].ID == id {
			return &records[i], nil
		}
	}
	return nil, nil
}

// NextID returns the next available ID for the given prefix (D, Q, or R)
// by scanning all records and incrementing the highest number found.
func NextID(records []Record, prefix string) string {
	prefix = strings.ToUpper(prefix)
	highest := 0
	pfx := prefix + "-"
	for _, rec := range records {
		if !strings.HasPrefix(rec.ID, pfx) {
			continue
		}
		numStr := rec.ID[len(pfx):]
		n := 0
		for _, c := range numStr {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		if n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("%s-%d", prefix, highest+1)
}
