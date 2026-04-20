package markdown

import (
	"regexp"
	"strings"
)

// Link types — see schema's links.link_type column.
const (
	LinkWiki  = "wiki"
	LinkEmbed = "embed"
	LinkMD    = "md"
)

var (
	// [[Target]] or [[Target|alias]] or [[Target#heading]] or [[Target#heading|alias]].
	// The optional leading `!` is captured so we can distinguish embeds from
	// regular wikilinks in one pass.
	wikiLinkRE = regexp.MustCompile(`(!)?\[\[([^\]]+?)\]\]`)

	// Standard markdown link [text](url). We don't try to distinguish image
	// embeds from text links here — both look the same to the index, and
	// callers rarely care for query purposes. Multiline alt text is rare in
	// vault notes; v1 keeps the regex single-line.
	mdLinkRE = regexp.MustCompile(`!?\[([^\]]*)\]\(([^)]+)\)`)
)

// ExtractLinks returns every wikilink, embed, and standard markdown link in
// body. Lines inside fenced code blocks are skipped — code samples that
// happen to contain `[[brackets]]` aren't real references.
//
// Inline-code matching (single backticks) is intentionally not handled in
// v1; it requires per-character state tracking and the false-positive rate
// for a typical vault is low. Documented as a known limitation.
func ExtractLinks(body []byte) []Link {
	if len(body) == 0 {
		return nil
	}
	var links []Link
	var fence fenceState
	src := string(body)
	lineNum := 0
	for _, line := range splitLines(src) {
		lineNum++
		if fence.step(line) {
			continue
		}
		// Wikilinks (and embeds) take precedence over markdown links because
		// the embed prefix `!` would also match the leading `!` of an image.
		for _, m := range wikiLinkRE.FindAllStringSubmatchIndex(line, -1) {
			bang, body := m[2], m[5]
			rawTarget := line[m[4]:m[5]]
			t := LinkWiki
			if bang != -1 && line[m[2]:m[3]] == "!" {
				t = LinkEmbed
			}
			target, alias := splitTargetAlias(rawTarget)
			links = append(links, Link{
				Target: target, Alias: alias, Type: t, Line: lineNum,
			})
			_ = body
		}
		for _, m := range mdLinkRE.FindAllStringSubmatchIndex(line, -1) {
			// Skip if this is actually a wikilink that already matched —
			// `![[X]]` matches mdLinkRE too if we're not careful, but the
			// inner brackets contain `[[` which the (`[^\]]*`) text capture
			// rejects. So we only need to filter pure-bracket wikilink hits.
			textStart, textEnd := m[2], m[3]
			text := line[textStart:textEnd]
			if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
				continue
			}
			urlStart, urlEnd := m[4], m[5]
			target := line[urlStart:urlEnd]
			alias := text
			if alias == "" {
				alias = target
			}
			links = append(links, Link{
				Target: target, Alias: alias, Type: LinkMD, Line: lineNum,
			})
		}
	}
	return links
}

// splitTargetAlias separates `Target|alias` into its two parts. A `#heading`
// suffix stays with the target (resolution is the indexer's problem).
func splitTargetAlias(raw string) (target, alias string) {
	if i := strings.Index(raw, "|"); i >= 0 {
		return strings.TrimSpace(raw[:i]), strings.TrimSpace(raw[i+1:])
	}
	return strings.TrimSpace(raw), ""
}

// splitLines yields each line of s without the trailing newline. CRLF and
// LF are both handled. An empty trailing line (file ending in newline) is
// suppressed.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			end := i
			if end > start && s[end-1] == '\r' {
				end--
			}
			out = append(out, s[start:end])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
