package markdown

import (
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Tag-source identifiers — match config.TagSourceInline / TagSourceFrontmatter.
const (
	tagSourceInline      = "inline"
	tagSourceFrontmatter = "frontmatter"
)

// inlineTagRE matches `#tag` preceded by start-of-line or whitespace.
// Tag bodies start with a letter and accept letters, digits, underscore,
// hyphen, and `/` for nested tags (Obsidian convention).
//
// Tags following punctuation like `(#foo)` are not extracted in v1 — Go's
// regex engine lacks lookbehind, and the whitespace-prefixed rule covers
// the overwhelming majority of vault content. Documented limitation.
var inlineTagRE = regexp.MustCompile(`(?:^|\s)#([A-Za-z][\w/\-]*)`)

// ExtractTags returns the deduplicated, sorted list of tags attached to a
// file via the configured sources. Tag strings are returned without the
// leading `#`, matching the storage format in the tags table.
//
// fm is the parsed frontmatter map (values typed). Frontmatter tags can
// appear as a string scalar (`tags: foo`), a YAML list (`tags: [a, b, c]`),
// or as the singular `tag:` form. We accept both `tag:` and `tags:` keys.
func ExtractTags(body []byte, fm map[string]Value, sources []string) []string {
	useInline := slices.Contains(sources, tagSourceInline)
	useFM := slices.Contains(sources, tagSourceFrontmatter)
	if !useInline && !useFM {
		return nil
	}
	seen := map[string]struct{}{}

	if useFM {
		for _, key := range []string{"tags", "tag"} {
			v, ok := fm[key]
			if !ok {
				continue
			}
			for _, t := range tagsFromValue(v) {
				if t != "" {
					seen[t] = struct{}{}
				}
			}
		}
	}

	if useInline && len(body) > 0 {
		var fence fenceState
		for _, line := range splitLines(string(body)) {
			if fence.step(line) {
				continue
			}
			for _, m := range inlineTagRE.FindAllStringSubmatch(line, -1) {
				if len(m) >= 2 && m[1] != "" {
					seen[m[1]] = struct{}{}
				}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// tagsFromValue interprets a single frontmatter value as one or more tags.
// Strings split on whitespace + comma so users can write `tags: foo bar` or
// `tags: foo, bar` and get the expected two tags. Lists return their string
// elements verbatim. Other types (numbers, maps) are ignored — a numeric
// tag has no useful meaning.
func tagsFromValue(v Value) []string {
	if v.HasText {
		return splitTagString(v.Text)
	}
	if v.JSON == "" {
		return nil
	}
	// For arrays we re-parse the JSON. This is cheap given typical vault
	// frontmatter sizes; the Value abstraction deliberately doesn't carry
	// the raw any to keep the type small.
	var arr []any
	if err := json.Unmarshal([]byte(v.JSON), &arr); err != nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, splitTagString(s)...)
		}
	}
	return out
}

// splitTagString cleans a raw `tags:` string. Strips a leading `#` if the
// user kept it, splits on whitespace and commas, drops empties.
func splitTagString(s string) []string {
	rep := strings.NewReplacer(",", " ", ";", " ")
	parts := strings.Fields(rep.Replace(s))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimPrefix(p, "#")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
