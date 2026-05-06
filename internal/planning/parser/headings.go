package parser

import (
	"fmt"
	"strings"
)

// Heading is a single markdown heading found in a record body, with
// the slug clean-house and other anchor-link tools use to resolve
// cross-references.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Slug  string `json:"slug"`
}

// ExtractHeadings parses ATX-style markdown headings (# .. ######)
// out of a record body and returns them in document order. Fenced
// code blocks are skipped so a `# foo` inside a code fence is not
// mistaken for a heading. Duplicate slugs are disambiguated by
// appending -1, -2, ... in occurrence order — the same convention
// GitHub-flavored renderers (and Obsidian) use.
func ExtractHeadings(body string) []Heading {
	var headings []Heading
	seen := map[string]int{}
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' && level < 6 {
			level++
		}
		if level == 0 || level == len(line) {
			continue
		}
		if line[level] != ' ' && line[level] != '\t' {
			continue
		}
		text := strings.TrimRight(strings.TrimSpace(line[level+1:]), "# \t")
		if text == "" {
			continue
		}
		base := slugify(text)
		if base == "" {
			continue
		}
		slug := base
		if n := seen[base]; n > 0 {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		seen[base]++
		headings = append(headings, Heading{Level: level, Text: text, Slug: slug})
	}
	return headings
}

// slugify converts heading text to the lowercase-hyphen anchor slug
// convention used by GitHub-flavored markdown and Obsidian. Letters,
// digits, and underscores survive; spaces and hyphens collapse to a
// single hyphen; everything else is dropped. Leading and trailing
// hyphens are trimmed.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-':
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
