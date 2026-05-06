package skill

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pql is the canonical single-file skill; clean-house has references.
// Tests below exercise both via Bundled where it matters; per-skill
// tests use ByName explicitly.

func TestBundled_HasExpectedSkills(t *testing.T) {
	if ByName("pql") == nil {
		t.Error("pql skill not bundled")
	}
	if ByName("clean-house") == nil {
		t.Error("clean-house skill not bundled")
	}
	if ByName("nonexistent") != nil {
		t.Error("ByName should return nil for unknown skill")
	}
}

func TestSkill_FilesContainsSkillMd(t *testing.T) {
	for _, s := range Bundled {
		t.Run(s.Name, func(t *testing.T) {
			if s.FileContent(SkillFile) == "" {
				t.Errorf("%s missing SKILL.md content", s.Name)
			}
			if !strings.Contains(s.FileContent(SkillFile), "---") {
				t.Errorf("%s SKILL.md missing frontmatter delimiter", s.Name)
			}
		})
	}
}

func TestCleanHouse_BundlesReferences(t *testing.T) {
	s := ByName("clean-house")
	if s == nil {
		t.Fatal("clean-house missing from Bundled")
	}
	rules := s.FileContent("references/rules.md")
	if rules == "" {
		t.Error("references/rules.md not bundled")
	}
	if !strings.Contains(rules, "RULE-ANCHOR-DRIFT") {
		t.Error("references/rules.md content looks wrong")
	}
}

func TestSkill_HashIsDeterministic(t *testing.T) {
	for _, s := range Bundled {
		t.Run(s.Name, func(t *testing.T) {
			a, b := s.Hash(), s.Hash()
			if a != b {
				t.Errorf("hash should be deterministic, got %q vs %q", a, b)
			}
			if !strings.HasPrefix(a, "sha256:") {
				t.Errorf("hash should be prefixed 'sha256:', got %q", a)
			}
		})
	}
}

func TestSkill_HashChangesWithContent(t *testing.T) {
	a := &Skill{Name: "x", files: map[string]string{"SKILL.md": "one"}}
	b := &Skill{Name: "x", files: map[string]string{"SKILL.md": "two"}}
	if a.Hash() == b.Hash() {
		t.Error("different content should produce different hash")
	}
}

// --- Inspect: state transitions ----------------------------------------

func TestInspect_MissingWhenNoDir(t *testing.T) {
	root := t.TempDir()
	for _, s := range Bundled {
		t.Run(s.Name, func(t *testing.T) {
			st, err := s.Inspect(root)
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if st.State != StateMissing {
				t.Errorf("State = %q, want %q", st.State, StateMissing)
			}
		})
	}
}

func TestInspect_CurrentAfterInstall(t *testing.T) {
	root := t.TempDir()
	for _, s := range Bundled {
		t.Run(s.Name, func(t *testing.T) {
			if _, err := s.Install(root, false); err != nil {
				t.Fatalf("Install: %v", err)
			}
			st, err := s.Inspect(root)
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
				t.Errorf("hashes diverge for fresh install: installed=%s onDisk=%s embedded=%s",
					st.Installed.Hash, st.OnDisk.Hash, st.Embedded.Hash)
			}
		})
	}
}

func TestInspect_ModifiedWhenSkillEdited(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	if _, err := s.Install(root, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.installDir(root), SkillFile), []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}
	st, err := s.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateModified {
		t.Errorf("State = %q, want %q", st.State, StateModified)
	}
}

func TestInspect_ModifiedWhenReferenceEdited(t *testing.T) {
	root := t.TempDir()
	s := ByName("clean-house")
	if _, err := s.Install(root, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	refPath := filepath.Join(s.installDir(root), "references/rules.md")
	if err := os.WriteFile(refPath, []byte("rewritten rules\n"), 0o644); err != nil {
		t.Fatalf("hand-edit reference: %v", err)
	}
	st, err := s.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateModified {
		t.Errorf("editing references/rules.md should mark bundle modified, got %q", st.State)
	}
}

func TestInspect_StaleWhenLockMatchesButEmbeddedDiffers(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	dir := s.installDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write old content + a lock matching that old content.
	old := map[string]string{SkillFile: "old skill content\n"}
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte(old[SkillFile]), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	lock := Snapshot{Version: "v0.0.1-old", Hash: hashBundle(old)}
	lockData, _ := json.MarshalIndent(lock, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, LockFile), lockData, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	st, err := s.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateStale {
		t.Errorf("State = %q, want %q", st.State, StateStale)
	}
}

func TestInspect_UnknownWhenLockAbsent(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	dir := s.installDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("manually placed\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	st, err := s.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateUnknown {
		t.Errorf("State = %q, want %q", st.State, StateUnknown)
	}
}

func TestInspect_UnknownWhenLockCorrupt(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	dir := s.installDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, LockFile), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	st, err := s.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if st.State != StateUnknown {
		t.Errorf("State = %q, want %q (corrupt lock treated as unknown)", st.State, StateUnknown)
	}
}

// --- Install: idempotency + safety -----------------------------------

func TestInstall_FreshSucceeds(t *testing.T) {
	root := t.TempDir()
	for _, s := range Bundled {
		t.Run(s.Name, func(t *testing.T) {
			st, err := s.Install(root, false)
			if err != nil {
				t.Fatalf("Install: %v", err)
			}
			if st.State != StateCurrent {
				t.Errorf("post-install state = %q", st.State)
			}
			for _, rel := range s.Files() {
				if _, err := os.Stat(filepath.Join(s.installDir(root), rel)); err != nil {
					t.Errorf("%s not written: %v", rel, err)
				}
			}
			if _, err := os.Stat(filepath.Join(s.installDir(root), LockFile)); err != nil {
				t.Errorf("lock not written: %v", err)
			}
		})
	}
}

func TestInstall_ReinstallIsIdempotent(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	first, err := s.Install(root, false)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	second, err := s.Install(root, false)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if first.OnDisk.Hash != second.OnDisk.Hash {
		t.Errorf("reinstall produced different hash: %s vs %s", first.OnDisk.Hash, second.OnDisk.Hash)
	}
}

func TestInstall_RefusesToOverwriteModified(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	if _, err := s.Install(root, false); err != nil {
		t.Fatalf("initial Install: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.installDir(root), SkillFile), []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}
	_, err := s.Install(root, false)
	var refused *ErrRefusedOverwrite
	if !errors.As(err, &refused) {
		t.Fatalf("expected ErrRefusedOverwrite, got %v", err)
	}
	if refused.State != StateModified || refused.Name != "pql" {
		t.Errorf("refused = %#v, want pql/modified", refused)
	}
	body, _ := os.ReadFile(filepath.Join(s.installDir(root), SkillFile))
	if string(body) != "edited\n" {
		t.Errorf("file modified despite refusal: %q", body)
	}
}

func TestInstall_ForceOverwritesModified(t *testing.T) {
	root := t.TempDir()
	s := ByName("clean-house")
	if _, err := s.Install(root, false); err != nil {
		t.Fatalf("initial Install: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.installDir(root), SkillFile), []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("hand-edit: %v", err)
	}
	st, err := s.Install(root, true)
	if err != nil {
		t.Fatalf("force Install: %v", err)
	}
	if st.State != StateCurrent {
		t.Errorf("state after force = %q", st.State)
	}
}

// --- Multi-skill operations ------------------------------------------

func TestInspectAll_ReportsAllBundled(t *testing.T) {
	root := t.TempDir()
	statuses, err := InspectAll(root)
	if err != nil {
		t.Fatalf("InspectAll: %v", err)
	}
	if len(statuses) != len(Bundled) {
		t.Fatalf("got %d statuses, want %d", len(statuses), len(Bundled))
	}
	for i, st := range statuses {
		if st.Name != Bundled[i].Name {
			t.Errorf("status[%d].Name = %q, want %q", i, st.Name, Bundled[i].Name)
		}
		if st.State != StateMissing {
			t.Errorf("%s state = %q, want missing", st.Name, st.State)
		}
	}
}

func TestInstallAll_InstallsEverySkill(t *testing.T) {
	root := t.TempDir()
	statuses, err := InstallAll(root, false)
	if err != nil {
		t.Fatalf("InstallAll: %v", err)
	}
	if len(statuses) != len(Bundled) {
		t.Fatalf("got %d statuses, want %d", len(statuses), len(Bundled))
	}
	for _, st := range statuses {
		if st.State != StateCurrent {
			t.Errorf("%s state = %q, want current", st.Name, st.State)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "pql", SkillFile)); err != nil {
		t.Errorf("pql SKILL.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "clean-house", "references", "rules.md")); err != nil {
		t.Errorf("clean-house references/rules.md missing: %v", err)
	}
}

func TestUninstallAll_RemovesAllSkills(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallAll(root, false); err != nil {
		t.Fatalf("InstallAll: %v", err)
	}
	if err := UninstallAll(root); err != nil {
		t.Fatalf("UninstallAll: %v", err)
	}
	for _, s := range Bundled {
		if _, err := os.Stat(filepath.Join(root, s.Name, SkillFile)); !os.IsNotExist(err) {
			t.Errorf("%s SKILL.md still exists: %v", s.Name, err)
		}
	}
}

func TestUninstall_LeavesUnrelatedFiles(t *testing.T) {
	root := t.TempDir()
	s := ByName("pql")
	if _, err := s.Install(root, false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	other := filepath.Join(s.installDir(root), "unrelated.md")
	if err := os.WriteFile(other, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("seed other file: %v", err)
	}
	if err := s.Uninstall(root); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated file should survive uninstall: %v", err)
	}
}

func TestUninstall_IdempotentOnMissing(t *testing.T) {
	root := t.TempDir()
	if err := ByName("pql").Uninstall(root); err != nil {
		t.Errorf("uninstall on missing should be a no-op, got %v", err)
	}
}

func TestInstall_LockCarriesVersionAndHash(t *testing.T) {
	root := t.TempDir()
	for _, s := range Bundled {
		t.Run(s.Name, func(t *testing.T) {
			if _, err := s.Install(root, false); err != nil {
				t.Fatalf("Install: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(s.installDir(root), LockFile))
			if err != nil {
				t.Fatalf("read lock: %v", err)
			}
			var lock Snapshot
			if err := json.Unmarshal(data, &lock); err != nil {
				t.Fatalf("parse lock: %v", err)
			}
			if lock.Hash == "" || lock.Version == "" || lock.InstalledAt == "" {
				t.Errorf("lock missing fields: %#v", lock)
			}
			if lock.Hash != s.Hash() {
				t.Errorf("lock hash mismatch: %s vs %s", lock.Hash, s.Hash())
			}
		})
	}
}
