package skill

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContent_NotEmpty(t *testing.T) {
	if strings.TrimSpace(Content()) == "" {
		t.Fatal("embedded skill content should not be empty")
	}
	if !strings.Contains(Content(), "---") {
		t.Error("embedded skill missing frontmatter delimiter")
	}
}

func TestEmbedded_HasDeterministicHash(t *testing.T) {
	a := Embedded()
	b := Embedded()
	if a.Hash != b.Hash {
		t.Errorf("hash should be deterministic, got %q vs %q", a.Hash, b.Hash)
	}
	if !strings.HasPrefix(a.Hash, "sha256:") {
		t.Errorf("hash should be prefixed 'sha256:', got %q", a.Hash)
	}
}

// --- Inspect: state transitions ----------------------------------------

func TestInspect_MissingWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	st, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateMissing {
		t.Errorf("State = %q, want %q", st.State, StateMissing)
	}
	if st.OnDisk != nil {
		t.Errorf("OnDisk should be nil, got %#v", st.OnDisk)
	}
}

func TestInspect_CurrentAfterInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	st, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateCurrent {
		t.Errorf("State = %q, want %q", st.State, StateCurrent)
	}
	if st.Installed == nil || st.OnDisk == nil {
		t.Errorf("expected both Installed and OnDisk, got %#v", st)
	}
	if st.Installed.Hash != st.OnDisk.Hash || st.OnDisk.Hash != st.Embedded.Hash {
		t.Errorf("all three hashes should match for a fresh install: installed=%s onDisk=%s embedded=%s",
			st.Installed.Hash, st.OnDisk.Hash, st.Embedded.Hash)
	}
}

func TestInspect_ModifiedWhenFileEditedAfterInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	// Simulate a hand-edit.
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}
	st, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateModified {
		t.Errorf("State = %q, want %q", st.State, StateModified)
	}
}

func TestInspect_StaleWhenLockMatchesButEmbeddedDiffers(t *testing.T) {
	dir := t.TempDir()
	// Write a SKILL.md that doesn't match the embedded one, plus a lock
	// file whose hash matches the on-disk file. Simulates "old version
	// installed cleanly; binary has since upgraded."
	contents := []byte("old skill content\n")
	if err := os.WriteFile(filepath.Join(dir, SkillFile), contents, 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	lock := Snapshot{
		Version: "v0.0.1-old",
		Hash:    hashBytes(contents),
	}
	lockData, _ := json.MarshalIndent(lock, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, LockFile), lockData, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	st, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateStale {
		t.Errorf("State = %q, want %q", st.State, StateStale)
	}
}

func TestInspect_UnknownWhenLockAbsent(t *testing.T) {
	dir := t.TempDir()
	// SKILL.md present, no lock file: user manually copied something in.
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("manually placed\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	st, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateUnknown {
		t.Errorf("State = %q, want %q", st.State, StateUnknown)
	}
	if st.Installed != nil {
		t.Errorf("Installed should be nil when lock missing, got %#v", st.Installed)
	}
}

func TestInspect_UnknownWhenLockCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, LockFile), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	st, err := Inspect(dir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateUnknown {
		t.Errorf("State = %q, want %q (corrupt lock treated as unknown)", st.State, StateUnknown)
	}
}

// --- Install: idempotency + safety -----------------------------------

func TestInstall_FreshSucceeds(t *testing.T) {
	dir := t.TempDir()
	st, err := Install(dir, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if st.State != StateCurrent {
		t.Errorf("post-install state = %q", st.State)
	}
	if _, err := os.Stat(filepath.Join(dir, SkillFile)); err != nil {
		t.Errorf("SKILL.md not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, LockFile)); err != nil {
		t.Errorf("lock file not written: %v", err)
	}
}

func TestInstall_ReinstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	first, err := Install(dir, false)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	second, err := Install(dir, false)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if first.OnDisk.Hash != second.OnDisk.Hash {
		t.Errorf("reinstall produced different hash: %s vs %s", first.OnDisk.Hash, second.OnDisk.Hash)
	}
	if second.State != StateCurrent {
		t.Errorf("second install state = %q", second.State)
	}
}

func TestInstall_RefusesToOverwriteModified(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("initial Install: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}

	_, err := Install(dir, false)
	var refused *ErrRefusedOverwrite
	if !errors.As(err, &refused) {
		t.Fatalf("expected ErrRefusedOverwrite, got %v", err)
	}
	if refused.State != StateModified {
		t.Errorf("refused state = %q, want %q", refused.State, StateModified)
	}

	body, _ := os.ReadFile(filepath.Join(dir, SkillFile))
	if string(body) != "edited\n" {
		t.Errorf("file was modified despite refusal: %q", body)
	}
}

func TestInstall_RefusesOverwriteUnknown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("manually placed\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := Install(dir, false)
	var refused *ErrRefusedOverwrite
	if !errors.As(err, &refused) {
		t.Fatalf("expected ErrRefusedOverwrite, got %v", err)
	}
	if refused.State != StateUnknown {
		t.Errorf("refused state = %q, want %q", refused.State, StateUnknown)
	}
}

func TestInstall_ForceOverwritesModified(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("initial Install: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}
	st, err := Install(dir, true)
	if err != nil {
		t.Fatalf("force Install: %v", err)
	}
	if st.State != StateCurrent {
		t.Errorf("state after force = %q", st.State)
	}
}

func TestInstall_CreatesParentDirs(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if _, err := Install(deep, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(deep, SkillFile)); err != nil {
		t.Errorf("SKILL.md not created in deep path: %v", err)
	}
}

func TestInstall_LockFileCarriesVersionAndHash(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, LockFile))
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	var lock Snapshot
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatalf("parse lock: %v", err)
	}
	if lock.Hash == "" || lock.Version == "" || lock.InstalledAt == "" {
		t.Errorf("lock file missing fields: %#v", lock)
	}
	if lock.Hash != hashBytes([]byte(Content())) {
		t.Errorf("lock hash doesn't match embedded content")
	}
}

// --- Uninstall ---------------------------------------------------------

func TestUninstall_RemovesFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := Uninstall(dir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, SkillFile)); !os.IsNotExist(err) {
		t.Errorf("SKILL.md still exists after uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, LockFile)); !os.IsNotExist(err) {
		t.Errorf("lock file still exists after uninstall: %v", err)
	}
}

func TestUninstall_IdempotentOnMissing(t *testing.T) {
	dir := t.TempDir()
	if err := Uninstall(dir); err != nil {
		t.Errorf("uninstall should be idempotent when nothing installed, got %v", err)
	}
}

func TestUninstall_RemovesEmptyDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pql")
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := Uninstall(dir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty dir should be removed, got %v", err)
	}
}

func TestUninstall_LeavesNonEmptyDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "pql")
	if _, err := Install(dir, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	// User dropped an unrelated file next to the skill. We shouldn't nuke it.
	otherPath := filepath.Join(dir, "unrelated.md")
	if err := os.WriteFile(otherPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("seed other file: %v", err)
	}
	if err := Uninstall(dir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(otherPath); err != nil {
		t.Errorf("unrelated file should survive uninstall, got %v", err)
	}
}
