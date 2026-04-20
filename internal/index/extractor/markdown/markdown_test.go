package markdown

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// --- frontmatter ----------------------------------------------------------

func TestSplitFrontmatter_Basic(t *testing.T) {
	in := []byte("---\nfoo: bar\n---\nbody line\n")
	head, body, err := SplitFrontmatter(in)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if string(head) != "foo: bar\n" {
		t.Errorf("head = %q", head)
	}
	if string(body) != "body line\n" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontmatter_NoFrontmatter(t *testing.T) {
	in := []byte("# heading only\n\nno fm here\n")
	head, body, err := SplitFrontmatter(in)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if head != nil {
		t.Errorf("expected nil head, got %q", head)
	}
	if string(body) != string(in) {
		t.Errorf("body should equal input")
	}
}

func TestSplitFrontmatter_CRLF(t *testing.T) {
	in := []byte("---\r\nfoo: bar\r\n---\r\nbody\r\n")
	head, body, err := SplitFrontmatter(in)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if !strings.Contains(string(head), "foo: bar") {
		t.Errorf("head missing foo: bar — got %q", head)
	}
	if !strings.Contains(string(body), "body") {
		t.Errorf("body missing — got %q", body)
	}
}

func TestSplitFrontmatter_UnclosedTreatedAsBody(t *testing.T) {
	in := []byte("---\nfoo: bar\nno closer here\n")
	head, body, err := SplitFrontmatter(in)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if head != nil {
		t.Errorf("expected nil head when delimiter unclosed, got %q", head)
	}
	if string(body) != string(in) {
		t.Errorf("body should equal input when fm unclosed")
	}
}

func TestParseFrontmatter_TypingPerKind(t *testing.T) {
	head := []byte(`
name: Vaasa
voting: true
seat: 4
score: 3.14
prior_jobs: [analyst, lecturer]
nullified: null
`)
	fm, err := ParseFrontmatter(head)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}

	cases := []struct {
		key                  string
		wantText             string
		wantHasText          bool
		wantNum              float64
		wantHasNum           bool
		wantJSONHasSubstring string
	}{
		{"name", "Vaasa", true, 0, false, `"Vaasa"`},
		{"voting", "", false, 1, true, `true`},
		{"seat", "", false, 4, true, `4`},
		{"score", "", false, 3.14, true, `3.14`},
		{"prior_jobs", "", false, 0, false, `["analyst","lecturer"]`},
	}
	for _, c := range cases {
		v, ok := fm[c.key]
		if !ok {
			t.Errorf("key %q missing from parsed fm", c.key)
			continue
		}
		if v.HasText != c.wantHasText || v.Text != c.wantText {
			t.Errorf("%s: text=(%q,%v), want=(%q,%v)", c.key, v.Text, v.HasText, c.wantText, c.wantHasText)
		}
		if v.HasNum != c.wantHasNum || v.Num != c.wantNum {
			t.Errorf("%s: num=(%v,%v), want=(%v,%v)", c.key, v.Num, v.HasNum, c.wantNum, c.wantHasNum)
		}
		if !strings.Contains(v.JSON, c.wantJSONHasSubstring) {
			t.Errorf("%s: JSON %q missing substring %q", c.key, v.JSON, c.wantJSONHasSubstring)
		}
	}
	if _, ok := fm["nullified"]; ok {
		t.Errorf("null values should be skipped, but key 'nullified' is present")
	}
}

func TestParseFrontmatter_BadYAMLErrors(t *testing.T) {
	_, err := ParseFrontmatter([]byte("foo: : :"))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

// --- links ----------------------------------------------------------------

func TestExtractLinks_Wikilinks(t *testing.T) {
	body := []byte(`see [[NoteA]] and [[NoteB|alias]] and [[NoteC#heading]] and [[NoteD#heading|alias]]`)
	got := ExtractLinks(body)
	want := []Link{
		{Target: "NoteA", Type: LinkWiki, Line: 1},
		{Target: "NoteB", Alias: "alias", Type: LinkWiki, Line: 1},
		{Target: "NoteC#heading", Type: LinkWiki, Line: 1},
		{Target: "NoteD#heading", Alias: "alias", Type: LinkWiki, Line: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d links, want %d: %#v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("link[%d] = %#v, want %#v", i, got[i], w)
		}
	}
}

func TestExtractLinks_Embed(t *testing.T) {
	body := []byte(`embedded ![[image.png]]`)
	got := ExtractLinks(body)
	if len(got) != 1 || got[0].Type != LinkEmbed || got[0].Target != "image.png" {
		t.Errorf("expected one embed of image.png, got %#v", got)
	}
}

func TestExtractLinks_MarkdownLink(t *testing.T) {
	body := []byte(`see [the spec](docs/spec.md) for details`)
	got := ExtractLinks(body)
	if len(got) != 1 {
		t.Fatalf("expected 1 link, got %v", got)
	}
	if got[0].Type != LinkMD || got[0].Target != "docs/spec.md" || got[0].Alias != "the spec" {
		t.Errorf("got %#v", got[0])
	}
}

func TestExtractLinks_SkipsCodeFences(t *testing.T) {
	body := []byte("real [[NoteA]]\n```\ncode [[NoteB]] here\n```\nreal [[NoteC]]\n")
	got := ExtractLinks(body)
	gotTargets := make([]string, 0, len(got))
	for _, l := range got {
		gotTargets = append(gotTargets, l.Target)
	}
	if !slices.Equal(gotTargets, []string{"NoteA", "NoteC"}) {
		t.Errorf("links inside fence should be skipped; got %v", gotTargets)
	}
}

func TestExtractLinks_TildeFence(t *testing.T) {
	body := []byte("~~~\n[[Inside]]\n~~~\n[[Outside]]\n")
	got := ExtractLinks(body)
	if len(got) != 1 || got[0].Target != "Outside" {
		t.Errorf("expected one link 'Outside', got %#v", got)
	}
}

func TestExtractLinks_LineNumbersAreOneBased(t *testing.T) {
	body := []byte("first\n[[Second]]\n[[Third]]\n")
	got := ExtractLinks(body)
	if len(got) != 2 {
		t.Fatalf("expected 2 links, got %d", len(got))
	}
	if got[0].Line != 2 || got[1].Line != 3 {
		t.Errorf("line numbers wrong: got %v %v, want 2 3", got[0].Line, got[1].Line)
	}
}

// --- tags -----------------------------------------------------------------

func TestExtractTags_FrontmatterList(t *testing.T) {
	fm := mustParse(t, `tags: [council-member, voting]`)
	got := ExtractTags(nil, fm, []string{tagSourceFrontmatter})
	if !slices.Equal(got, []string{"council-member", "voting"}) {
		t.Errorf("got %v", got)
	}
}

func TestExtractTags_FrontmatterString(t *testing.T) {
	fm := mustParse(t, `tags: foo, bar baz`)
	got := ExtractTags(nil, fm, []string{tagSourceFrontmatter})
	if !slices.Equal(got, []string{"bar", "baz", "foo"}) {
		t.Errorf("got %v", got)
	}
}

func TestExtractTags_SingularTagKey(t *testing.T) {
	fm := mustParse(t, `tag: only-one`)
	got := ExtractTags(nil, fm, []string{tagSourceFrontmatter})
	if !slices.Equal(got, []string{"only-one"}) {
		t.Errorf("got %v", got)
	}
}

func TestExtractTags_InlineHashtags(t *testing.T) {
	body := []byte("intro\n#first and #second\n#third/nested at end\n")
	got := ExtractTags(body, nil, []string{tagSourceInline})
	if !slices.Equal(got, []string{"first", "second", "third/nested"}) {
		t.Errorf("got %v", got)
	}
}

func TestExtractTags_DedupAcrossSources(t *testing.T) {
	fm := mustParse(t, `tags: [shared, fmonly]`)
	body := []byte("inline #shared and #inlineonly")
	got := ExtractTags(body, fm, []string{tagSourceInline, tagSourceFrontmatter})
	if !slices.Equal(got, []string{"fmonly", "inlineonly", "shared"}) {
		t.Errorf("got %v", got)
	}
}

func TestExtractTags_SkipsCodeFences(t *testing.T) {
	body := []byte("real #yes\n```\nfake #no\n```\nreal #also\n")
	got := ExtractTags(body, nil, []string{tagSourceInline})
	if !slices.Equal(got, []string{"also", "yes"}) {
		t.Errorf("got %v", got)
	}
}

func TestExtractTags_SourceFiltering(t *testing.T) {
	fm := mustParse(t, `tags: [fmtag]`)
	body := []byte("body #inlinetag")

	if got := ExtractTags(body, fm, []string{tagSourceInline}); !slices.Equal(got, []string{"inlinetag"}) {
		t.Errorf("inline-only got %v", got)
	}
	if got := ExtractTags(body, fm, []string{tagSourceFrontmatter}); !slices.Equal(got, []string{"fmtag"}) {
		t.Errorf("fm-only got %v", got)
	}
	if got := ExtractTags(body, fm, nil); got != nil {
		t.Errorf("no sources should return nil, got %v", got)
	}
}

// --- headings -------------------------------------------------------------

func TestExtractHeadings_AllDepths(t *testing.T) {
	body := []byte("# h1\n## h2\n### h3\n#### h4\n##### h5\n###### h6\n####### too-deep\n")
	got := ExtractHeadings(body)
	if len(got) != 6 {
		t.Fatalf("expected 6 headings (>6 hashes is not a heading), got %d: %v", len(got), got)
	}
	for i, h := range got {
		if h.Depth != i+1 {
			t.Errorf("heading[%d] depth = %d, want %d", i, h.Depth, i+1)
		}
		if h.Text != "h"+itoa(i+1) {
			t.Errorf("heading[%d] text = %q, want h%d", i, h.Text, i+1)
		}
	}
}

func TestExtractHeadings_TrailingHashesStripped(t *testing.T) {
	body := []byte("## Heading ##\n")
	got := ExtractHeadings(body)
	if len(got) != 1 || got[0].Text != "Heading" {
		t.Errorf("got %#v", got)
	}
}

func TestExtractHeadings_LineOffsetsAreCorrect(t *testing.T) {
	body := []byte("# first\nsome prose\n## second\n")
	got := ExtractHeadings(body)
	if len(got) != 2 {
		t.Fatalf("expected 2 headings, got %d", len(got))
	}
	if got[0].LineOffset != 0 {
		t.Errorf("first heading offset = %d, want 0", got[0].LineOffset)
	}
	// "# first\n" = 8 bytes, "some prose\n" = 11 bytes, second heading starts at 19.
	if got[1].LineOffset != 19 {
		t.Errorf("second heading offset = %d, want 19", got[1].LineOffset)
	}
}

func TestExtractHeadings_SkipsCodeFences(t *testing.T) {
	body := []byte("# real\n```\n# fake\n```\n## also-real\n")
	got := ExtractHeadings(body)
	if len(got) != 2 || got[0].Text != "real" || got[1].Text != "also-real" {
		t.Errorf("got %#v", got)
	}
}

// --- Extract orchestrator -------------------------------------------------

func TestExtract_Smoke(t *testing.T) {
	raw := []byte(`---
type: council-member
name: Vaasa
voting: true
tags: [council-member]
---
# Vaasa

Sometimes [[Holt]] and [[Tintori|the analyst]] disagree. See ![[chart.png]].

#strategic
`)
	res, err := Extract(raw, ExtractOpts{TagSources: []string{tagSourceInline, tagSourceFrontmatter}})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Frontmatter["name"].Text != "Vaasa" {
		t.Errorf("fm name = %v", res.Frontmatter["name"])
	}
	if len(res.Links) != 3 {
		t.Errorf("expected 3 links, got %d: %#v", len(res.Links), res.Links)
	}
	if !slices.Equal(res.Tags, []string{"council-member", "strategic"}) {
		t.Errorf("tags = %v", res.Tags)
	}
	if len(res.Headings) != 1 || res.Headings[0].Text != "Vaasa" {
		t.Errorf("headings = %#v", res.Headings)
	}
}

func TestExtract_CouncilSnapshot_VaasaPersona(t *testing.T) {
	path := repoTestdataFile(t, "council-snapshot/members/vaasa/persona.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	res, err := Extract(raw, ExtractOpts{TagSources: []string{tagSourceInline, tagSourceFrontmatter}})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Frontmatter["type"].Text != "council-member" {
		t.Errorf("expected type=council-member, got %v", res.Frontmatter["type"])
	}
	if res.Frontmatter["name"].Text == "" {
		t.Errorf("expected name to be set")
	}
	// At least one heading expected in any persona file.
	if len(res.Headings) == 0 {
		t.Errorf("expected at least one heading in persona file")
	}
}

// --- helpers --------------------------------------------------------------

func mustParse(t *testing.T, yamlBody string) map[string]Value {
	t.Helper()
	fm, err := ParseFrontmatter([]byte(yamlBody))
	if err != nil {
		t.Fatalf("ParseFrontmatter %q: %v", yamlBody, err)
	}
	return fm
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func repoTestdataFile(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		candidate := filepath.Join(dir, "testdata", rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate testdata/%s starting from %s", rel, wd)
		}
		dir = parent
	}
}
