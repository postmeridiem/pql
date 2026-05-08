package changelog

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/postmeridiem/pql/internal/planning/repo"
)

// RebuildResult summarises a rebuild run.
type RebuildResult struct {
	TablesCleared []string `json:"tables_cleared"`
	FilesReplayed []string `json:"files_replayed"`
	StatementsRun int      `json:"statements_run"`
}

// replicatedTables lists, in foreign-key-safe order for DELETE, the
// tables that participate in changelog replication. Children come
// first so DELETE doesn't trip parent FK constraints.
var replicatedTables = []string{
	"ticket_history",
	"ticket_labels",
	"ticket_deps",
	"tickets",
}

// Rebuild truncates the replicated planning tables, resets the
// import marker, and runs Import to repopulate from .pql/changelog/.
// Used by the post-checkout and post-rewrite hooks (D-18) because
// incremental replay can only INSERT/UPDATE — it can't remove rows
// that existed on the previous branch but not the new one.
//
// Decisions and decision_refs are NOT touched: they are
// markdown-sourced (D-8) and refreshed via `pql decisions sync`,
// not via the changelog. Callers that switch markdown content
// (i.e. branch checkout) should follow Rebuild with `pql decisions
// sync` if the decisions/ tree changed.
func Rebuild(ctx context.Context, db *sql.DB, vaultPath string) (*RebuildResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("changelog: rebuild begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	cleared := make([]string, 0, len(replicatedTables))
	for _, table := range replicatedTables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil { //nolint:gosec // G202: closed-set table whitelist
			return nil, fmt.Errorf("changelog: clear %s: %w", table, err)
		}
		cleared = append(cleared, table)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM meta WHERE key = ?`, repo.MetaLastImportMarker); err != nil {
		return nil, fmt.Errorf("changelog: reset import marker: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("changelog: rebuild commit: %w", err)
	}

	imp, err := Import(ctx, db, vaultPath)
	if err != nil {
		return nil, err
	}
	return &RebuildResult{
		TablesCleared: cleared,
		FilesReplayed: imp.FilesReplayed,
		StatementsRun: imp.StatementsRun,
	}, nil
}
