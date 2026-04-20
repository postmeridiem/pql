// Package markdown extracts the structured pieces of a markdown file:
// frontmatter, wikilinks, tags, and headings.
//
// Each extractor runs as a free function for isolated testing; Extract is
// the convenience orchestrator that runs them all and returns a Result the
// indexer can upsert into the store. Code-fence state (which suppresses
// inline matching for wikilinks and tags) is shared across body extractors
// via the fenceState helper in this file.
//
// v1 scope is the Obsidian dialect — see docs/structure/initial-plan.md
// open questions for tag-syntax-in-code-fences and link-target-resolution.
package markdown

import "strings"

// Value type tags. Stored verbatim in frontmatter.type. New tags require a
// schema bump because the indexer's NOT NULL constraint accepts only these.
const (
	TypeString = "string"
	TypeNumber = "number"
	TypeBool   = "bool"
	TypeList   = "list"
	TypeObject = "object"
)

// Value is the typed view of a single frontmatter entry, ready to map onto
// the frontmatter table's type / value_json / value_text / value_num columns.
//
// Type is always set to one of the Type* constants — the explicit tag means
// `pql schema` can report value kinds without scanning value_*.
// JSON is always set (canonical JSON serialization).
// HasText/Text and HasNum/Num are set only when the underlying value can be
// represented as that scalar — strings get Text, numerics get Num, booleans
// get both Num (0/1) and JSON (true/false), lists/objects get only JSON.
type Value struct {
	Type    string
	JSON    string
	Text    string
	HasText bool
	Num     float64
	HasNum  bool
}

// Link is one outgoing reference from a file. Target is the raw wikilink
// target (or markdown URL); resolution to a real path happens in the indexer
// per the rules in initial-plan.md open question #6.
type Link struct {
	Target string
	Alias  string
	Type   string // "wiki", "embed", or "md"
	Line   int    // 1-based
}

// Heading is one ATX heading line. Setext-style headings (text underlined
// with === / ---) are deferred — they're rare and add lookahead complexity.
type Heading struct {
	Depth      int    // 1 for #, 2 for ##, etc.
	Text       string // text after the # markers, trimmed
	LineOffset int    // 0-based byte offset from body start
}

// Result is what one file produces. The indexer composes each field into
// the appropriate store table (files, frontmatter, links, tags, headings).
type Result struct {
	Frontmatter map[string]Value
	Body        []byte // content after the frontmatter delimiter
	Links       []Link
	Tags        []string
	Headings    []Heading
}

// ExtractOpts carries the config-driven knobs the extractor needs to
// honour. Right now only TagSources matters; future fields will join here
// rather than growing the package's exported surface.
type ExtractOpts struct {
	// TagSources is the subset of {"inline", "frontmatter"} the user wants
	// tag extraction to draw from. Empty slice means "neither" — tags
	// extraction is effectively disabled.
	TagSources []string
}

// Extract runs every v1 extractor against raw and returns a populated
// Result. Errors here mean the file's frontmatter was malformed; body
// extractors are best-effort and never error.
func Extract(raw []byte, opts ExtractOpts) (Result, error) {
	head, body, err := SplitFrontmatter(raw)
	if err != nil {
		return Result{}, err
	}
	fm, err := ParseFrontmatter(head)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Frontmatter: fm,
		Body:        body,
		Links:       ExtractLinks(body),
		Tags:        ExtractTags(body, fm, opts.TagSources),
		Headings:    ExtractHeadings(body),
	}, nil
}

// fenceState tracks whether the current line sits inside a fenced code
// block. Body extractors that care about prose-vs-code (links, tags) call
// step() once per line; isFenced() reports the state for that line.
//
// Recognises both ``` and ~~~ fences with optional infostrings. The opening
// marker's character and length must match exactly to close (per CommonMark).
type fenceState struct {
	open    bool
	marker  byte // '`' or '~'
	minRun  int  // length of the opening fence; closer must match or exceed
}

// step advances the state for a single body line. Returns whether the line
// itself is treated as inside the fence (so callers can suppress matching
// on fence-marker lines too).
func (f *fenceState) step(line string) (insideFence bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if marker, run, ok := parseFenceMarker(trimmed); ok {
		if !f.open {
			// Opening a new fence; the marker line itself is treated as fence.
			f.open = true
			f.marker = marker
			f.minRun = run
			return true
		}
		// Already in a fence. Closing requires same marker char and ≥ run.
		if marker == f.marker && run >= f.minRun {
			f.open = false
			f.marker = 0
			f.minRun = 0
			return true // the closing fence line is still "inside" for this step
		}
		// Different marker or shorter run inside an open fence: ignore.
	}
	return f.open
}

// parseFenceMarker reports whether trimmed begins with a CommonMark code
// fence (≥3 of the same char `\`` or `~`).
func parseFenceMarker(trimmed string) (marker byte, run int, ok bool) {
	if len(trimmed) < 3 {
		return 0, 0, false
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return 0, 0, false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return c, n, true
}
