package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDSLSource_Positional(t *testing.T) {
	got, err := readDSLSource([]string{"SELECT *"}, "", false, strings.NewReader(""))
	if err != nil {
		t.Fatalf("readDSLSource: %v", err)
	}
	if got != "SELECT *" {
		t.Errorf("got %q", got)
	}
}

func TestReadDSLSource_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "q.pql")
	if err := os.WriteFile(path, []byte("SELECT path"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := readDSLSource(nil, path, false, strings.NewReader(""))
	if err != nil {
		t.Fatalf("readDSLSource: %v", err)
	}
	if got != "SELECT path" {
		t.Errorf("got %q", got)
	}
}

func TestReadDSLSource_FromStdin(t *testing.T) {
	got, err := readDSLSource(nil, "", true, strings.NewReader("SELECT name"))
	if err != nil {
		t.Fatalf("readDSLSource: %v", err)
	}
	if got != "SELECT name" {
		t.Errorf("got %q", got)
	}
}

func TestReadDSLSource_NoInputErrors(t *testing.T) {
	_, err := readDSLSource(nil, "", false, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for no input mode")
	}
}

func TestReadDSLSource_MultipleModesError(t *testing.T) {
	_, err := readDSLSource([]string{"SELECT *"}, "/tmp/q", false, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error when positional + --file both set")
	}
	_, err = readDSLSource([]string{"SELECT *"}, "", true, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error when positional + --stdin both set")
	}
	_, err = readDSLSource(nil, "/tmp/q", true, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error when --file + --stdin both set")
	}
}

func TestReadDSLSource_MissingFileErrors(t *testing.T) {
	_, err := readDSLSource(nil, "/definitely/does/not/exist", false, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for missing --file target")
	}
}
