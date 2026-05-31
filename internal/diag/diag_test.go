package diag

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestEmit_WritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	Emit(&buf, Diagnostic{Level: LevelWarn, Code: "x.y", Msg: "hi"})
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("Emit output should end with newline: %q", out)
	}
	if !strings.Contains(out, `"code":"x.y"`) || !strings.Contains(out, `"msg":"hi"`) {
		t.Errorf("Emit output missing fields: %q", out)
	}
}

// TestWarn_QuietSuppresses captures os.Stderr to confirm --quiet (via
// SetQuiet) silences warnings while leaving them on by default.
func TestWarn_QuietSuppresses(t *testing.T) {
	t.Cleanup(func() { SetQuiet(false) })

	capture := func(fn func()) string {
		orig := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		fn()
		_ = w.Close()
		os.Stderr = orig
		b, _ := io.ReadAll(r)
		return string(b)
	}

	SetQuiet(false)
	if got := capture(func() { Warn("a.b", "loud") }); !strings.Contains(got, "loud") {
		t.Errorf("warning should emit when not quiet, got %q", got)
	}

	SetQuiet(true)
	if got := capture(func() { Warn("a.b", "loud") }); got != "" {
		t.Errorf("warning should be suppressed when quiet, got %q", got)
	}
}
