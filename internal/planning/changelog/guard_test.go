package changelog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuard_FreshVaultPasses(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	if err := GuardReplicaCurrent(ctx, db, vault); err != nil {
		t.Errorf("guard on fresh vault: %v, want nil", err)
	}
}

func TestGuard_PopulatedReplicaPasses(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)
	seedTicket(t, db, "T-1", "2025-05-08 11:00:00")

	if err := GuardReplicaCurrent(ctx, db, vault); err != nil {
		t.Errorf("guard on populated replica: %v, want nil", err)
	}
}

// The T-96 state: a clone whose changelog holds history but whose replica
// never replayed it. A mutation here would re-mint labels from T-1.
func TestGuard_EmptyReplicaWithHistoryRefuses(t *testing.T) {
	ctx := context.Background()

	srcVault, srcDB := setupVault(t)
	seedTicket(t, srcDB, "T-1", "2025-05-08 11:00:00")
	if _, err := Export(ctx, srcDB, srcVault); err != nil {
		t.Fatalf("Export: %v", err)
	}

	dstVault, dstDB := setupVault(t)
	copyTree(t,
		filepath.Join(srcVault, ".pql", "changelog"),
		filepath.Join(dstVault, ".pql", "changelog"),
	)

	err := GuardReplicaCurrent(ctx, dstDB, dstVault)
	if err == nil {
		t.Fatal("guard on empty replica with changelog history: nil, want refusal")
	}
	if !strings.Contains(err.Error(), "replica is empty") {
		t.Errorf("refusal should name the condition, got: %v", err)
	}

	// And the remedy clears it: import, then the same guard passes.
	if _, err := Import(ctx, dstDB, dstVault); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if err := GuardReplicaCurrent(ctx, dstDB, dstVault); err != nil {
		t.Errorf("guard after import: %v, want nil", err)
	}
}

// Empty .sql files carry no history — a replica beside them is genuinely
// fresh, not stale.
func TestGuard_EmptyChangelogFilesPass(t *testing.T) {
	ctx := context.Background()
	vault, db := setupVault(t)

	dir := filepath.Join(vault, ".pql", "changelog", "tickets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2025-05.sql"), nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := GuardReplicaCurrent(ctx, db, vault); err != nil {
		t.Errorf("guard beside empty changelog files: %v, want nil", err)
	}
}
