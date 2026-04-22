package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestShell_QueriesAndQuit(t *testing.T) {
	vault := filepath.Join("..", "..", "testdata", "council-snapshot")
	db := filepath.Join(t.TempDir(), "test.db")

	input := strings.Join([]string{
		"-- this is a comment",
		"",
		"SELECT name WHERE name = 'README'",
		"quit",
	}, "\n") + "\n"

	var stdout bytes.Buffer
	code := runCLI(t, &stdout, strings.NewReader(input),
		"--vault", vault, "--db", db, "shell")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Should have exactly one result array containing README.
	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, stdout.String())
	}
	if len(rows) != 1 || rows[0]["name"] != "README" {
		t.Errorf("rows = %v, want [{name:README}]", rows)
	}
}

func TestShell_ParseErrorContinues(t *testing.T) {
	vault := filepath.Join("..", "..", "testdata", "council-snapshot")
	db := filepath.Join(t.TempDir(), "test.db")

	input := "not valid pql\nSELECT name WHERE name = 'README'\nquit\n"

	var stdout bytes.Buffer
	code := runCLI(t, &stdout, strings.NewReader(input),
		"--vault", vault, "--db", db, "shell")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (parse errors are per-query)", code)
	}
	if !strings.Contains(stdout.String(), "README") {
		t.Errorf("second query should have produced output; got: %s", stdout.String())
	}
}

func TestShell_EOF(t *testing.T) {
	vault := filepath.Join("..", "..", "testdata", "council-snapshot")
	db := filepath.Join(t.TempDir(), "test.db")

	code := runCLI(t, &bytes.Buffer{}, strings.NewReader(""),
		"--vault", vault, "--db", db, "shell")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 on empty stdin/EOF", code)
	}
}

// runCLI builds a root command, wires the given stdout and stdin, and
// returns the exit code. Stderr goes to /dev/null so prompt output and
// diagnostics don't interfere with assertions on stdout.
func runCLI(t *testing.T, stdout *bytes.Buffer, stdin *strings.Reader, args ...string) int {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetIn(stdin)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exitError); ok {
		return ee.code
	}
	t.Fatalf("unexpected non-exitError: %v", err)
	return -1
}
