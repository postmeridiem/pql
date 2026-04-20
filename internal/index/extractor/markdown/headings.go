package markdown

import (
	"regexp"
	"strings"
)

// atxHeadingRE matches an ATX heading line: 1–6 leading hashes, at least
// one space, then heading text. Trailing hashes (e.g. `## Heading ##`) are
// stripped by the caller.
//
// Setext-style headings (text underlined with === / ---) are intentionally
// out of v1 scope — they're rare in vault prose and add lookahead.
var atxHeadingRE = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

// ExtractHeadings returns every ATX heading in body in document order.
// LineOffset is the 0-based byte offset of the heading's first character
// from the start of body — useful for "jump to heading" UX without
// re-scanning the file later.
//
// Headings inside fenced code blocks are skipped (a code sample starting
// with `# ` isn't a real heading).
func ExtractHeadings(body []byte) []Heading {
	if len(body) == 0 {
		return nil
	}
	var out []Heading
	var fence fenceState
	src := string(body)
	offset := 0
	for _, line := range splitLines(src) {
		lineLen := len(line) + 1 // include the newline we split on
		if fence.step(line) {
			offset += lineLen
			continue
		}
		if m := atxHeadingRE.FindStringSubmatch(line); m != nil {
			out = append(out, Heading{
				Depth:      len(m[1]),
				Text:       trimTrailingHashes(m[2]),
				LineOffset: offset,
			})
		}
		offset += lineLen
	}
	return out
}

// trimTrailingHashes removes a closing run of `#` from a heading title
// (e.g. `Heading ##` → `Heading`). Per CommonMark, the run is only stripped
// when preceded by whitespace.
func trimTrailingHashes(s string) string {
	t := strings.TrimRight(s, "#")
	if t == s {
		return s
	}
	t = strings.TrimRight(t, " \t")
	return t
}
