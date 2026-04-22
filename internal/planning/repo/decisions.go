// Package repo provides data-access helpers over pql.db for the
// planning subcommands. Consumer-agnostic: no cobra imports.
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/postmeridiem/pql/internal/planning/parser"
)

// SyncResult summarises a decisions sync operation.
type SyncResult struct {
	Synced   int      `json:"synced"`
	Refs     int      `json:"refs"`
	Broken   int      `json:"broken"`
	Warnings []string `json:"warnings,omitempty"`
}

// SyncDecisions upserts parsed records into pql.db and rebuilds
// the decision_refs table. Idempotent.
func SyncDecisions(ctx context.Context, db *sql.DB, decisionsDir, repoRoot string) (*SyncResult, error) {
	records, warnings, err := parser.ParseAll(decisionsDir, repoRoot)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("repo: begin sync: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM decision_refs"); err != nil {
		return nil, fmt.Errorf("repo: clear refs: %w", err)
	}

	upserted := 0
	for _, r := range records {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO decisions (id, type, domain, title, status, date, file_path, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
			ON CONFLICT(id) DO UPDATE SET
				type      = excluded.type,
				domain    = excluded.domain,
				title     = excluded.title,
				status    = excluded.status,
				date      = excluded.date,
				file_path = excluded.file_path,
				synced_at = datetime('now')
		`, r.ID, r.Type, r.Domain, r.Title, r.Status, nullIfEmpty(r.Date), r.FilePath); err != nil {
			return nil, fmt.Errorf("repo: upsert %s: %w", r.ID, err)
		}
		upserted++
	}

	knownIDs := make(map[string]bool)
	for _, r := range records {
		knownIDs[r.ID] = true
	}

	refsCreated := 0
	broken := 0
	for _, r := range records {
		for _, ref := range r.Refs {
			if !knownIDs[ref.TargetID] {
				broken++
				warnings = append(warnings, fmt.Sprintf("broken ref: %s -> %s (%s)", r.ID, ref.TargetID, ref.RefType))
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO decision_refs (source_id, target_id, ref_type, note)
				VALUES (?, ?, ?, ?)
			`, r.ID, ref.TargetID, ref.RefType, ref.Note); err != nil {
				return nil, fmt.Errorf("repo: insert ref %s->%s: %w", r.ID, ref.TargetID, err)
			}
			refsCreated++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("repo: commit sync: %w", err)
	}
	return &SyncResult{
		Synced:   upserted,
		Refs:     refsCreated,
		Broken:   broken,
		Warnings: warnings,
	}, nil
}

// Decision is a row from the decisions table.
type Decision struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	Domain   string  `json:"domain"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	Date     *string `json:"date,omitempty"`
	FilePath string  `json:"file_path"`
	SyncedAt string  `json:"synced_at"`
}

// DecisionFilter constrains ListDecisions results.
type DecisionFilter struct {
	Type   string
	Domain string
	Status string
}

// ListDecisions returns decisions matching the filter.
func ListDecisions(ctx context.Context, db *sql.DB, f DecisionFilter) ([]Decision, error) {
	query := `SELECT id, type, domain, title, status, date, file_path, synced_at
		FROM decisions WHERE 1=1`
	var params []any
	if f.Type != "" {
		query += " AND type = ?"
		params = append(params, f.Type)
	}
	if f.Domain != "" {
		query += " AND domain = ?"
		params = append(params, f.Domain)
	}
	if f.Status != "" {
		query += " AND status = ?"
		params = append(params, f.Status)
	}
	query += " ORDER BY id"

	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("repo: list decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(&d.ID, &d.Type, &d.Domain, &d.Title, &d.Status,
			&d.Date, &d.FilePath, &d.SyncedAt); err != nil {
			return nil, fmt.Errorf("repo: scan decision: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// GetDecision returns a single decision by ID, or nil if not found.
func GetDecision(ctx context.Context, db *sql.DB, id string) (*Decision, error) {
	var d Decision
	err := db.QueryRowContext(ctx,
		`SELECT id, type, domain, title, status, date, file_path, synced_at
		 FROM decisions WHERE id = ?`, id,
	).Scan(&d.ID, &d.Type, &d.Domain, &d.Title, &d.Status,
		&d.Date, &d.FilePath, &d.SyncedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("repo: get decision %s: %w", id, err)
	}
	return &d, nil
}

// DecisionRef is a row from the decision_refs table.
type DecisionRef struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	RefType  string `json:"ref_type"`
	Note     string `json:"note,omitempty"`
}

// RefsOf returns all cross-references involving the given decision ID
// (in either direction).
func RefsOf(ctx context.Context, db *sql.DB, id string) ([]DecisionRef, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT source_id, target_id, ref_type, COALESCE(note, '')
		FROM decision_refs
		WHERE source_id = ? OR target_id = ?
		ORDER BY ref_type, source_id, target_id
	`, id, id)
	if err != nil {
		return nil, fmt.Errorf("repo: refs of %s: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var result []DecisionRef
	for rows.Next() {
		var r DecisionRef
		if err := rows.Scan(&r.SourceID, &r.TargetID, &r.RefType, &r.Note); err != nil {
			return nil, fmt.Errorf("repo: scan ref: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// TicketSummary is a minimal ticket view for decision joins.
type TicketSummary struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// TicketsForDecision returns tickets linked to a decision via decision_ref.
func TicketsForDecision(ctx context.Context, db *sql.DB, decisionID string) ([]TicketSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, title, status, priority
		FROM tickets WHERE decision_ref = ? ORDER BY id
	`, decisionID)
	if err != nil {
		return nil, fmt.Errorf("repo: tickets for %s: %w", decisionID, err)
	}
	defer func() { _ = rows.Close() }()

	var result []TicketSummary
	for rows.Next() {
		var t TicketSummary
		if err := rows.Scan(&t.ID, &t.Type, &t.Title, &t.Status, &t.Priority); err != nil {
			return nil, fmt.Errorf("repo: scan ticket: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// CoverageGap is a D-record without any implementing ticket.
type CoverageGap struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Title  string `json:"title"`
}

// Coverage returns confirmed decisions that have no linked tickets.
func Coverage(ctx context.Context, db *sql.DB) ([]CoverageGap, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT d.id, d.domain, d.title
		FROM decisions d
		LEFT JOIN tickets t ON t.decision_ref = d.id
		WHERE d.type = 'confirmed' AND t.id IS NULL
		ORDER BY d.id
	`)
	if err != nil {
		return nil, fmt.Errorf("repo: coverage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []CoverageGap
	for rows.Next() {
		var g CoverageGap
		if err := rows.Scan(&g.ID, &g.Domain, &g.Title); err != nil {
			return nil, fmt.Errorf("repo: scan coverage: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
