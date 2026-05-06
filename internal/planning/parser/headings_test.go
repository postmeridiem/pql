package parser

import (
	"reflect"
	"testing"
)

func TestExtractHeadings_Basic(t *testing.T) {
	body := `Some prelude text.

# Top
Body paragraph.

## A subsection
More body.

### Even deeper
Nested.

Closing prose.`
	got := ExtractHeadings(body)
	want := []Heading{
		{Level: 1, Text: "Top", Slug: "top"},
		{Level: 2, Text: "A subsection", Slug: "a-subsection"},
		{Level: 3, Text: "Even deeper", Slug: "even-deeper"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestExtractHeadings_DuplicateSlugs(t *testing.T) {
	body := `## Goals
First.

## Goals
Second.

## Goals
Third.`
	got := ExtractHeadings(body)
	wantSlugs := []string{"goals", "goals-1", "goals-2"}
	if len(got) != 3 {
		t.Fatalf("got %d headings, want 3", len(got))
	}
	for i, h := range got {
		if h.Slug != wantSlugs[i] {
			t.Errorf("[%d] slug = %q, want %q", i, h.Slug, wantSlugs[i])
		}
	}
}

func TestExtractHeadings_SkipsCodeFences(t *testing.T) {
	body := "## Real heading\n\n```sh\n# not a heading — inside a fence\n```\n\n## Also real"
	got := ExtractHeadings(body)
	if len(got) != 2 {
		t.Fatalf("got %d headings, want 2 — fence content leaked through:\n%#v", len(got), got)
	}
	if got[0].Text != "Real heading" || got[1].Text != "Also real" {
		t.Errorf("got %#v", got)
	}
}

func TestExtractHeadings_StripsTrailingHashes(t *testing.T) {
	body := "## Closed-style heading ##\n## Another one ###"
	got := ExtractHeadings(body)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	if got[0].Text != "Closed-style heading" || got[1].Text != "Another one" {
		t.Errorf("trailing hashes not stripped: %#v", got)
	}
}

func TestExtractHeadings_IgnoresNonATX(t *testing.T) {
	body := `Setext-underline style is not supported intentionally.

Heading
=======

#NoSpace should be ignored`
	got := ExtractHeadings(body)
	if len(got) != 0 {
		t.Errorf("expected zero headings, got %#v", got)
	}
}

func TestExtractHeadings_MaxLevelSix(t *testing.T) {
	body := "###### Six\n####### Seven"
	got := ExtractHeadings(body)
	// '#######' is 7 hashes; the loop caps at 6 then sees '#' at line[6],
	// not a space, so it's rejected as a non-heading.
	if len(got) != 1 || got[0].Level != 6 || got[0].Text != "Six" {
		t.Errorf("got %#v, want one h6", got)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"already-hyphen", "already-hyphen"},
		{"under_score_kept", "under_score_kept"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"Punctuation: dropped! (mostly).", "punctuation-dropped-mostly"},
		{"Multiple   spaces", "multiple-spaces"},
		{"Mixed--many--dashes", "mixed-many-dashes"},
		{"123 numeric", "123-numeric"},
		{"emoji 🎯 dropped", "emoji-dropped"},
		{"---", ""},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractHeadings_NilForEmpty(t *testing.T) {
	if got := ExtractHeadings(""); got != nil {
		t.Errorf("empty body should return nil, got %#v", got)
	}
}
