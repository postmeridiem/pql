// Package diag defines the exit-code contract and emits structured diagnostics
// to stderr as line-delimited JSON. See docs/output-contract.md.
package diag

import (
	"encoding/json"
	"io"
	"os"
)

// Exit codes per docs/output-contract.md.
const (
	OK       = 0  // success — including zero results (empty [] on stdout)
	Usage    = 64 // EX_USAGE — bad CLI flag
	DataErr  = 65 // EX_DATAERR — PQL parse or evaluation error
	NoInput  = 66 // EX_NOINPUT — vault root not found / unreadable
	Unavail  = 69 // EX_UNAVAILABLE — index corruption / migration failure
	Software = 70 // EX_SOFTWARE — internal error
)

// Level is the severity tag on a stderr Diagnostic. "warn" is informational,
// "error" precedes a non-zero exit code.
type Level string

// The level set is intentionally tiny — this isn't syslog. See
// docs/output-contract.md.
const (
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Diagnostic is one entry in the stderr JSON-per-line stream.
type Diagnostic struct {
	Level Level  `json:"level"`
	Code  string `json:"code"`
	Msg   string `json:"msg"`
	Hint  string `json:"hint,omitempty"`
}

// Emit writes a diagnostic as one JSON line to w.
func Emit(w io.Writer, d Diagnostic) {
	b, err := json.Marshal(d)
	if err != nil {
		// Should not happen for the small Diagnostic struct; degrade gracefully.
		_, _ = io.WriteString(w, `{"level":"error","code":"diag.marshal","msg":"failed to marshal diagnostic"}`+"\n")
		return
	}
	_, _ = w.Write(append(b, '\n'))
}

// quiet, when set, suppresses warning diagnostics (errors still emit).
// Toggled once from the global --quiet flag in the root pre-run. The CLI
// is single-threaded at this layer, so a package-level toggle is safe and
// matches diag's existing process-wide os.Stderr target.
var quiet bool

// SetQuiet enables or disables suppression of warning diagnostics.
func SetQuiet(q bool) { quiet = q }

// Warn emits a warning diagnostic to stderr, unless --quiet is set.
func Warn(code, msg string) {
	if quiet {
		return
	}
	Emit(os.Stderr, Diagnostic{Level: LevelWarn, Code: code, Msg: msg})
}

// Error emits an error diagnostic to stderr.
func Error(code, msg, hint string) {
	Emit(os.Stderr, Diagnostic{Level: LevelError, Code: code, Msg: msg, Hint: hint})
}
