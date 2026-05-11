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

// Canonical record-type values. Repeated across inferStatus,
// SubdirRecordType, recordTypeSubdir, and the prefix map.
const (
	typeConfirmed = "confirmed"
	typeQuestion  = "question"
	typeRejected  = "rejected"
)

var typeFromPrefix = map[byte]string{
	'D': typeConfirmed,
	'Q': typeQuestion,
	'R': typeRejected,
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

// ParseFile reads a single DQR markdown file and returns all records
// found. Domain comes from the filename stem; for the D-21 layout the
// record type also comes from the immediate parent directory name
// (decisions/, questions/, or rejected/). For the legacy flat layout
// the parent-directory inference is skipped and the heading prefix
// (D|Q|R) alone determines record type — same behaviour the parser
// already had before D-21. filePath is recorded relative to repoRoot.
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

// domainFromFilename strips the file extension and the legacy
// `questions-` prefix. Under the D-21 layout the prefix never appears
// (record type is encoded in the subdirectory, not the filename), so
// this is effectively just "drop the extension." The prefix strip is
// retained as a no-cost backstop for repos that still carry legacy
// filenames.
func domainFromFilename(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.HasPrefix(stem, "questions-") {
		return stem[len("questions-"):]
	}
	return stem
}

// SubdirRecordType maps the immediate parent directory name to the
// record type the parser expects in that file under the D-21 layout.
// Empty string means "no expectation" — caller skips the consistency
// check (legacy flat layout, or a file outside the recognised
// subdirectories).
func SubdirRecordType(parentDir string) string {
	switch parentDir {
	case "decisions":
		return typeConfirmed
	case "questions":
		return typeQuestion
	case "rejected":
		return typeRejected
	default:
		return ""
	}
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

// Decision/question record status values, in their canonical form.
// Repeated by inferStatus across record-type branches.
const (
	statusActive     = "active"
	statusOpen       = "open"
	statusResolved   = "resolved"
	statusSuperseded = "superseded"
)

func inferStatus(recType string, body []string) string {
	if recType == typeRejected {
		return statusActive
	}
	for _, line := range body {
		if supersededRe.MatchString(line) {
			return statusSuperseded
		}
	}
	if recType == typeQuestion {
		for _, line := range body {
			m := statusRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			s := strings.ToLower(strings.TrimSpace(m[1]))
			if strings.Contains(s, "partial") || strings.Contains(s, "remaining") {
				return statusOpen
			}
			if strings.HasPrefix(s, "resolved") {
				return statusResolved
			}
			return statusOpen
		}
		return statusOpen
	}
	return statusActive
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

// ParseAll parses every *.md file under dqrRoot. Under the D-21
// layout the walker descends into the recognised type subdirectories
// (decisions/, questions/, rejected/) and infers the expected record
// type from the parent dir; a heading-prefix mismatch surfaces as a
// warning so authors notice mis-filed records.
//
// Under the legacy flat layout (no subdirectories — files live
// directly at dqrRoot), the parser falls back to the pre-D-21
// behaviour: domain from filename, type from heading prefix, no
// subdir-consistency check. README.md at any level is skipped.
// repoRoot is the project root used to compute relative file paths.
func ParseAll(dqrRoot, repoRoot string) ([]Record, []string, error) {
	entries, err := os.ReadDir(dqrRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("parser: read %s: %w", dqrRoot, err)
	}

	// Detect layout: any of decisions/, questions/, rejected/ as a
	// direct subdirectory indicates the D-21 layout. Otherwise stay
	// in the legacy flat mode.
	type pathInfo struct {
		path       string
		expectType string // "" for legacy flat
	}
	var paths []pathInfo
	var newLayout bool
	for _, e := range entries {
		if e.IsDir() && SubdirRecordType(e.Name()) != "" {
			newLayout = true
			break
		}
	}

	if newLayout {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			expect := SubdirRecordType(e.Name())
			if expect == "" {
				continue
			}
			subPath := filepath.Join(dqrRoot, e.Name())
			subEntries, err := os.ReadDir(subPath)
			if err != nil {
				return nil, nil, fmt.Errorf("parser: read %s: %w", subPath, err)
			}
			for _, se := range subEntries {
				if se.IsDir() || !strings.HasSuffix(se.Name(), ".md") {
					continue
				}
				if strings.EqualFold(se.Name(), "readme.md") {
					continue
				}
				paths = append(paths, pathInfo{
					path:       filepath.Join(subPath, se.Name()),
					expectType: expect,
				})
			}
		}
	} else {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if strings.EqualFold(e.Name(), "readme.md") {
				continue
			}
			paths = append(paths, pathInfo{
				path: filepath.Join(dqrRoot, e.Name()),
			})
		}
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].path < paths[j].path })

	var records []Record
	var warnings []string
	for _, p := range paths {
		// Filename style check: lowercase, hyphenated, .md extension.
		// Surfaces on every layout (legacy and D-21) because the
		// convention applies regardless.
		if w := filenameStyleWarning(p.path, repoRoot); w != "" {
			warnings = append(warnings, w)
		}
		recs, err := ParseFile(p.path, repoRoot)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("error parsing %s: %v", filepath.Base(p.path), err))
			continue
		}
		if p.expectType != "" {
			for _, rec := range recs {
				if rec.Type != p.expectType {
					rel, _ := filepath.Rel(repoRoot, p.path)
					if rel == "" {
						rel = p.path
					}
					warnings = append(warnings, fmt.Sprintf(
						"subdir-type mismatch: %s is in a %s/ subdirectory but contains a %s record (%s); "+
							"move the record to the matching subdirectory",
						rel, recordTypeSubdir(p.expectType), rec.Type, rec.ID))
				}
			}
		}
		records = append(records, recs...)
	}
	return records, warnings, nil
}

// filenameStyleRe matches lowercase, hyphenated, .md-extension names.
// Stems may contain digits but must start with a letter.
var filenameStyleRe = regexp.MustCompile(`^[a-z][a-z0-9-]*\.md$`)

// filenameStyleWarning checks a markdown filename against D-21's
// style convention: lowercase letters, digits, hyphens; .md
// extension. Returns the warning string or "" if the name is OK.
func filenameStyleWarning(path, repoRoot string) string {
	base := filepath.Base(path)
	if filenameStyleRe.MatchString(base) {
		return ""
	}
	rel, _ := filepath.Rel(repoRoot, path)
	if rel == "" {
		rel = path
	}
	return fmt.Sprintf(
		"filename style: %s should be lowercase + hyphenated (e.g. %s)",
		rel, strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(base, "_", "-"), " ", "-")))
}

// recordTypeSubdir is the inverse of SubdirRecordType for the
// warning message.
func recordTypeSubdir(recType string) string {
	switch recType {
	case typeConfirmed:
		return "decisions"
	case typeQuestion:
		return "questions"
	case typeRejected:
		return "rejected"
	default:
		return recType
	}
}

// Validate runs a dry parse and returns (ok, errors, warnings).
// Errors are structural — duplicate IDs, empty titles, broken refs,
// and parse failures — and make the result non-ok. Warnings are
// style-class — filename convention, subdir/heading mismatch — and
// do not affect ok. CLI callers print both; `--no-style` suppresses
// the warning stream without touching errors.
func Validate(decisionsDir, repoRoot string) (ok bool, errs, warnings []string) {
	records, parseWarnings, err := ParseAll(decisionsDir, repoRoot)
	if err != nil {
		return false, []string{err.Error()}, nil
	}
	warnings = append(warnings, parseWarnings...)

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
	return len(errs) == 0, errs, warnings
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
