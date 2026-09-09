package cli

import (
	"context"
	"fmt"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/changelog"
)

// openPlanningDBForMutation opens pql.db for a verb that is about to write
// to it, refusing in the one state where writing produces a silent label
// collision: an empty replica beside a committed changelog that holds
// ticket history (T-96). That state means the clone never replayed —
// typically because the replication hooks were never planted — and the
// next allocation would re-mint from T-1. Read verbs and the remedies
// (plan import, plan rebuild) open the DB directly.
// Errors are returned as *exitError with the right code and hint already
// attached — callers must return them as-is, not re-wrap, or the guard's
// remedy hint is lost.
func openPlanningDBForMutation(ctx context.Context, cfg *config.Config) (*planning.DB, error) {
	pdb, err := openPlanningDB(ctx, cfg)
	if err != nil {
		return nil, &exitError{code: diag.Unavail, msg: err.Error()}
	}
	if err := changelog.GuardReplicaCurrent(ctx, pdb.SQL(), cfg.Vault.Path); err != nil {
		_ = pdb.Close()
		return nil, &exitError{
			code: diag.DataErr,
			msg:  err.Error(),
			hint: "run `pql plan import` to replay the committed changelog into the replica (or `pql plan rebuild` for a full rebuild), then retry",
		}
	}
	return pdb, nil
}

// exportThrough flushes every replicated planning row mutated since the
// last export marker into <vault>/.pql/changelog/, making the changelog
// the synchronous log of record (write-through).
//
// Every ticket mutation calls this immediately after the write to pql.db
// succeeds. Because changelog.Export selects exactly the rows with
// updated_at > last_export_marker and advances the marker, this appends
// precisely the row(s) the mutation just touched — no extra bookkeeping.
//
// Why it matters: pql plan rebuild (run by the post-checkout and
// post-rewrite hooks) truncates the replicated tables and replays the
// changelog *from disk*. Without write-through, a ticket created but not
// yet committed lives only in pql.db and is silently destroyed by the
// next branch switch. Write-through guarantees the row is already in a
// changelog file on disk, which git carries across the checkout, so
// rebuild replays it back. See governance/decisions/architecture.md
// (D-15/D-16: write-through persistence).
//
// On error the mutation has already been committed to pql.db; the export
// marker is simply not advanced, so the next exportThrough (or a manual
// pql plan export) catches up. The caller surfaces the error so the user
// knows durability did not complete.
//
// Decisions are markdown-sourced (D-8) and travel with their .md files,
// so only ticket mutations route through here. Future writing consumers
// (e.g. a planning-surface MCP) must call this too.
func exportThrough(ctx context.Context, pdb *planning.DB, vaultPath string) error {
	if _, err := changelog.Export(ctx, pdb.SQL(), vaultPath); err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("write-through export to .pql/changelog/: %v", err)}
	}
	return nil
}
