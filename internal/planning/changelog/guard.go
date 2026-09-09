package changelog

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
)

// GuardReplicaCurrent refuses to proceed when the local replica is behind
// the committed changelog on the one axis a mutation cannot survive: label
// allocation. nextTicketID hands out max+1 over the replica's ticket_idmap,
// so any label in the changelog above that max is a collision waiting to be
// minted — silently, with the wrong label then cited in prose, commits and
// consuming repos that replay cannot reach (T-96, T-123).
//
// Two flavours of the same defect, told apart for the diagnostic's sake:
// an empty replica beside a populated changelog (the clone never replayed —
// typically the replication hooks were never planted), and a populated
// replica the changelog has moved past (pulled, but the post-merge import
// never fired). Both refuse; the remedy for both is `pql plan import`.
//
// The comparison reads the changelog the way replay does — staged through
// SQLite, not scraped with regexes (D-28) — and counts every label ever
// minted, including mappings later superseded by relabel, because "never
// re-mint a label that has ever been used" is exactly allocation's rule.
// A replica *ahead* of the changelog passes: that is just write-through
// with an export pending.
//
// Import and rebuild are the remedy for the guarded state, so they must
// not call this. Lives here rather than in the CLI so every writing
// consumer (a planning-surface MCP tomorrow) inherits the same refusal.
func GuardReplicaCurrent(ctx context.Context, db *sql.DB, vaultPath string) error {
	clMax, err := changelogMaxLabel(ctx, vaultPath)
	if err != nil {
		return err
	}
	if clMax == 0 {
		return nil // no ticket history in the changelog — nothing to be behind
	}

	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tickets`).Scan(&n); err != nil {
		return fmt.Errorf("guard: count tickets: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("planning replica is empty but %s holds ticket history (labels up to T-%d)",
			ChangelogDir, clMax)
	}

	var dbMax int
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(ticket_id, 3) AS INTEGER)), 0)
		FROM ticket_idmap
	`).Scan(&dbMax); err != nil {
		return fmt.Errorf("guard: max replica label: %w", err)
	}
	if clMax > dbMax {
		return fmt.Errorf("planning replica is behind the committed changelog: replica labels reach T-%d, changelog reaches T-%d",
			dbMax, clMax)
	}
	return nil
}

// changelogMaxLabel reports the highest T-NNN ever minted according to the
// committed changelog's ticket_idmap files, or 0 when there are none.
func changelogMaxLabel(ctx context.Context, vaultPath string) (int, error) {
	var idmapSpec tableSpec
	for _, s := range changelogTables {
		if s.Name == "ticket_idmap" {
			idmapSpec = s
		}
	}
	if idmapSpec.Name == "" {
		return 0, fmt.Errorf("guard: no ticket_idmap spec in changelogTables")
	}

	st, err := newStaging(ctx)
	if err != nil {
		return 0, err
	}
	defer st.close()

	dir := filepath.Join(vaultPath, ChangelogDir, idmapSpec.Name)
	if _, err := st.loadTable(ctx, idmapSpec, dir); err != nil {
		return 0, err
	}

	var maxLabel int
	err = st.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CAST(SUBSTR(ticket_id, 3) AS INTEGER)), 0)
		FROM ticket_idmap WHERE ticket_id LIKE 'T-%'
	`).Scan(&maxLabel)
	if err != nil {
		return 0, fmt.Errorf("guard: max changelog label: %w", err)
	}
	return maxLabel, nil
}
