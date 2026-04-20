package render

import (
	"bytes"
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestRender_JSONDefault(t *testing.T) {
	var buf bytes.Buffer
	rows := []sample{{"a", 1}, {"b", 2}}
	n, err := Render(rows, Opts{Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	want := `[{"name":"a","age":1},{"name":"b","age":2}]` + "\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRender_EmptySliceIsEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	n, err := Render([]sample{}, Opts{Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
	if buf.String() != "[]\n" {
		t.Errorf("got %q, want []", buf.String())
	}
}

func TestRender_NilSliceIsEmptyArray(t *testing.T) {
	var buf bytes.Buffer
	n, err := Render[sample](nil, Opts{Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 0 || buf.String() != "[]\n" {
		t.Errorf("got count=%d output=%q, want 0 []", n, buf.String())
	}
}

func TestRender_Pretty(t *testing.T) {
	var buf bytes.Buffer
	_, err := Render([]sample{{"a", 1}}, Opts{Format: FormatPretty, Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "a"`) {
		t.Errorf("pretty output missing indented name field: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "\n") {
		t.Errorf("pretty output should be multi-line: %q", buf.String())
	}
}

func TestRender_JSONL(t *testing.T) {
	var buf bytes.Buffer
	rows := []sample{{"a", 1}, {"b", 2}}
	n, err := Render(rows, Opts{Format: FormatJSONL, Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	want := `{"name":"a","age":1}` + "\n" + `{"name":"b","age":2}` + "\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRender_JSONLEmptyEmitsNothing(t *testing.T) {
	var buf bytes.Buffer
	n, err := Render([]sample{}, Opts{Format: FormatJSONL, Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 0 || buf.Len() != 0 {
		t.Errorf("got count=%d len=%d, want 0 0", n, buf.Len())
	}
}

func TestRender_LimitTruncates(t *testing.T) {
	var buf bytes.Buffer
	rows := []sample{{"a", 1}, {"b", 2}, {"c", 3}}
	n, err := Render(rows, Opts{Limit: 2, Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	if !strings.Contains(buf.String(), `"a"`) || !strings.Contains(buf.String(), `"b"`) {
		t.Errorf("missing first two rows: %q", buf.String())
	}
	if strings.Contains(buf.String(), `"c"`) {
		t.Errorf("third row should be truncated: %q", buf.String())
	}
}

func TestRender_UnknownFormatErrors(t *testing.T) {
	var buf bytes.Buffer
	_, err := Render([]sample{{"a", 1}}, Opts{Format: "csv", Out: &buf})
	if err == nil {
		t.Fatal("expected unknown-format error, got nil")
	}
}

func TestRender_JSONLEndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	_, err := Render([]sample{{"a", 1}}, Opts{Format: FormatJSONL, Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("jsonl output should end with newline: %q", buf.String())
	}
}

func TestRender_EscapeHTMLOff(t *testing.T) {
	// Wikilink targets and tag values can contain & and < — verify they
	// pass through unescaped.
	var buf bytes.Buffer
	rows := []sample{{Name: "a&b<c", Age: 1}}
	_, err := Render(rows, Opts{Out: &buf})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "a&b<c") {
		t.Errorf("html escaped unexpectedly: %q", buf.String())
	}
}

func TestRenderOne_Object(t *testing.T) {
	var buf bytes.Buffer
	v := &sample{Name: "a", Age: 1}
	wrote, err := RenderOne(v, Opts{Out: &buf})
	if err != nil {
		t.Fatalf("RenderOne: %v", err)
	}
	if !wrote {
		t.Error("expected wrote=true for non-nil value")
	}
	if buf.String() != `{"name":"a","age":1}`+"\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderOne_NilEmitsNull(t *testing.T) {
	var buf bytes.Buffer
	wrote, err := RenderOne[sample](nil, Opts{Out: &buf})
	if err != nil {
		t.Fatalf("RenderOne: %v", err)
	}
	if wrote {
		t.Error("expected wrote=false for nil value")
	}
	if buf.String() != "null\n" {
		t.Errorf("got %q, want null\\n", buf.String())
	}
}

func TestRenderOne_PrettyIndents(t *testing.T) {
	var buf bytes.Buffer
	_, err := RenderOne(&sample{Name: "a", Age: 1}, Opts{Format: FormatPretty, Out: &buf})
	if err != nil {
		t.Fatalf("RenderOne: %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "a"`) {
		t.Errorf("pretty output missing indent: %q", buf.String())
	}
}
