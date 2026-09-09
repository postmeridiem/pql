package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GuardReplicaCurrent refuses to proceed when the local replica has plainly
// never replayed the committed changelog: zero rows in tickets while
// .pql/changelog/tickets/ holds history. Mutating in that state silently
// re-mints labels from T-1, and the collision surfaces only at the next
// replay — after the wrong label has been used in prose, commits and
// consuming repos that replay cannot reach (T-96).
//
// The empty-and-fresh vault passes: no tickets changelog, or only empty
// files, means an empty replica is the true state. Import and rebuild are
// the remedy for the guarded state, so they must not call this.
//
// Lives here rather than in the CLI so every writing consumer (a
// planning-surface MCP tomorrow) inherits the same refusal.
func GuardReplicaCurrent(ctx context.Context, db *sql.DB, vaultPath string) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tickets`).Scan(&n); err != nil {
		return fmt.Errorf("guard: count tickets: %w", err)
	}
	if n > 0 {
		return nil
	}

	dir := filepath.Join(vaultPath, ChangelogDir, "tickets")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no tickets changelog — an empty replica is genuinely fresh
		}
		return fmt.Errorf("guard: read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() > 0 {
			return fmt.Errorf("planning replica is empty but %s holds ticket history (%s)",
				filepath.Join(ChangelogDir, "tickets"), e.Name())
		}
	}
	return nil
}
