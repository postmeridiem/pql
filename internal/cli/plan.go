package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/planning"
	"github.com/postmeridiem/pql/internal/planning/changelog"
	"github.com/postmeridiem/pql/internal/planning/repo"
)

const defaultSnapshotFile = ".pql/pql-plan.json"

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Cross-cutting planning dashboard and search",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.AddCommand(newPlanStatusCmd())
	cmd.AddCommand(newPlanWhatsNextCmd())
	cmd.AddCommand(newPlanReviewCmd())
	cmd.AddCommand(newPlanExportCmd())
	cmd.AddCommand(newPlanRebuildCmd())
	cmd.AddCommand(newPlanImportCmd())
	cmd.AddCommand(newPlanUpgradeCmd())
	return cmd
}

// --- upgrade ---

func newPlanUpgradeCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Migrate .pql/changelog/ forward to the format this pql writes",
		Long: `Bring a vault's replication artefacts up to the versions this binary
speaks. Today that means the changelog file format.

The changelog is the log of record — unlike pql.db it cannot be
regenerated from anything — so an older format is migrated forward in
place rather than dropped and rebuilt. The rows are staged through
SQLite, which is what parses them correctly, and re-emitted through the
same renderer a normal export uses, so the output is byte-identical to
what this pql would have written. Row data is untouched; what changes is
the inline conflict guard each line carries.

  pql plan upgrade              # migrate and stamp the new format
  pql plan upgrade --dry-run    # list the files that would change

This also runs from the post-merge hook installed by pql init, because a
pull is where a working-tree change to tracked files is expected. It
rewrites files under .pql/changelog/, so the result belongs in a commit.
Every clone should land the same upgrade commit before writing new rows:
a union merge between an upgraded and a non-upgraded clone doubles each
row until the next upgrade collapses it. See D-28.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			res, err := changelog.Upgrade(ctx, cfg.Vault.Path, dryRun)
			if err != nil {
				return &exitError{code: diag.DataErr, msg: err.Error()}
			}

			// The pql.db axis reports alongside, so one verb answers "where
			// does this vault sit on every axis" rather than one per artefact.
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()
			schemaAxis, err := planning.SchemaStatus(ctx, pdb.SQL())
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			out := &struct {
				*changelog.UpgradeResult
				Schema planning.SchemaAxisStatus `json:"schema"`
			}{res, schemaAxis}
			return renderOne(cmd, out)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// warnChangelogFormat surfaces a changelog whose format differs from what this
// binary writes. An older format replays under superseded rules — survivable,
// but never silently; a newer one is a hard stop, because a format this binary
// does not know may encode rows it would misread.
func warnChangelogFormat(vaultPath string) error {
	_, warning, err := changelog.CheckFormat(vaultPath)
	if err != nil {
		return &exitError{code: diag.DataErr, msg: err.Error()}
	}
	if warning != "" {
		diag.Warn("pql.plan.format_stale", warning)
	}
	return nil
}

// --- status ---

func newPlanStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Planning dashboard: decision counts, open Qs, ticket summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			db := pdb.SQL()
			dash, err := buildDashboard(ctx, db)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(dash, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

type dashboard struct {
	Decisions decisionSummary `json:"decisions"`
	Tickets   ticketSummary   `json:"tickets"`
}

type decisionSummary struct {
	Total     int `json:"total"`
	Confirmed int `json:"confirmed"`
	Questions int `json:"questions"`
	Rejected  int `json:"rejected"`
	OpenQs    int `json:"open_questions"`
}

type ticketSummary struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

func buildDashboard(ctx context.Context, db *sql.DB) (*dashboard, error) {
	var d dashboard

	rows, err := db.QueryContext(ctx, `
		SELECT type, status, COUNT(*) FROM decisions GROUP BY type, status
	`)
	if err != nil {
		return nil, err
	}
	d.Tickets.ByStatus = make(map[string]int)

	for rows.Next() {
		var typ, status string
		var count int
		if err := rows.Scan(&typ, &status, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		d.Decisions.Total += count
		switch typ {
		case "confirmed":
			d.Decisions.Confirmed += count
		case "question":
			d.Decisions.Questions += count
			if status == "open" {
				d.Decisions.OpenQs += count
			}
		case "rejected":
			d.Decisions.Rejected += count
		}
	}
	_ = rows.Close()

	trows, err := db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM tickets GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		var status string
		var count int
		if err := trows.Scan(&status, &count); err != nil {
			_ = trows.Close()
			return nil, err
		}
		d.Tickets.Total += count
		d.Tickets.ByStatus[status] = count
	}
	_ = trows.Close()

	return &d, nil
}

// --- whatsnext ---

func newPlanWhatsNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whatsnext",
		Short: "Surface the next ticket to work on",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			db := pdb.SQL()
			t, err := repo.WhatNext(ctx, db, statusSetFromConfig(cfg))
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			out, err := buildShowTree(ctx, db, t, fullTree)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if t == nil {
				out.Message = "no actionable ticket — nothing active or ready with all blockers cleared; check the board, unblock work, or advance a ticket into the ready lane"
			}

			return renderOne(cmd, out)
		},
	}
}

// --- review ---

func newPlanReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review",
		Short: "Surface the next ticket awaiting review",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			db := pdb.SQL()
			t, err := repo.NextReview(ctx, db, statusSetFromConfig(cfg))
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			out, err := buildShowTree(ctx, db, t, fullTree)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if t == nil {
				out.Message = "no tickets awaiting review"
			}

			return renderOne(cmd, out)
		},
	}
}

// renderOne is a small render helper for the planning verbs that emit
// a single object. Mirrors the inline pattern used elsewhere; pulled
// out so plan whatsnext / plan review aren't 12 lines of boilerplate.
func renderOne[T any](cmd *cobra.Command, v *T) error {
	rOpts, err := renderOptsFromFlags(cmd)
	if err != nil {
		return &exitError{code: diag.Usage, msg: err.Error()}
	}
	rOpts.Out = cmd.OutOrStdout()
	if _, err := render.One(v, rOpts); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	return nil
}

// --- export ---

func newPlanExportCmd() *cobra.Command {
	var stage bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Append planning state changes to .pql/changelog/<table>/<YYYY-MM>.sql",
		Long: `Append per-table monthly SQL upsert files under .pql/changelog/
for every replicated planning row that has been modified since the
last export. The files are meant to be committed to git; replicas
pull them and replay via pql plan import.

Replicated tables: tickets, ticket_deps, ticket_labels,
ticket_history. Decisions and decision_refs are markdown-sourced
(D-8) and travel with their .md files instead.

  pql plan export            # append to .pql/changelog/<table>/<YYYY-MM>.sql
  pql plan export --stage    # also git-add the touched files

--stage runs ` + "`git add`" + ` over the files written in this run;
idempotent across untracked / tracked / unchanged states. Used by
the pre-commit hook installed by pql init.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			res, err := changelog.Export(ctx, pdb.SQL(), cfg.Vault.Path)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(res, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			if stage {
				if err := stageChangelog(ctx, res.FilesWritten); err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&stage, "stage", false, "git-add the changelog files written in this run")
	return cmd
}

// stageChangelog runs `git add` over each file written by the
// exporter. `git add` is idempotent across untracked / tracked /
// unchanged states, so the pre-commit hook can call this without
// inspecting per-file git state. Skips silently when there is nothing
// to stage (no rows changed since the last marker).
func stageChangelog(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: paths come from the exporter's resolved file list under .pql/changelog/
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add changelog files: %v: %s", err, out)
	}
	return nil
}

// --- rebuild ---

func newPlanRebuildCmd() *cobra.Command {
	var verify bool
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Drop replicated tables and replay .pql/changelog/ from scratch",
		Long: `Truncate the replicated planning tables (tickets, ticket_deps,
ticket_labels, ticket_history) and replay every changelog file
under .pql/changelog/. Used by the post-checkout and post-rewrite
hooks (D-18) because LWW-guarded incremental replay can't remove
rows that existed on the previous branch but not the new one.

Decisions and decision_refs are NOT touched — they are
markdown-sourced (D-8) and refreshed via ` + "`pql decisions sync`" + `.
After a branch switch that changed decisions/*.md, follow rebuild
with that command.

  pql plan rebuild            # power-user / disaster recovery
  pql plan rebuild --verify   # same, plus a before/after row comparison
  # or invoked from the post-checkout / post-rewrite hooks

--verify snapshots every replicated row's canonical hash before the
replay and compares afterwards, reporting rows that came back with
different content or did not come back at all. Counts alone would miss
the dangerous case — a row that is present but wrong. A divergence is a
warning, not a failure; only rows that disappear outright exit non-zero.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			if err := warnChangelogFormat(cfg.Vault.Path); err != nil {
				return err
			}

			if !verify {
				res, err := changelog.Rebuild(ctx, pdb.SQL(), cfg.Vault.Path)
				if err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				warnTicketCollisions(res.Collisions)
				if _, err := render.One(res, rOpts); err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				return nil
			}

			res, report, err := changelog.VerifiedRebuild(ctx, pdb.SQL(), cfg.Vault.Path)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			warnTicketCollisions(res.Collisions)
			warnRebuildDivergences(report)

			verified := &struct {
				*changelog.RebuildResult
				Verify *changelog.VerifyReport `json:"verify"`
			}{res, report}
			if _, err := render.One(verified, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			if report.RowsLost > 0 {
				return &exitError{code: diag.DataErr, msg: fmt.Sprintf(
					"%d row(s) did not survive the replay; see verify.divergences", report.RowsLost)}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&verify, "verify", false, "compare row hashes before and after the replay and report divergences")
	return cmd
}

// warnRebuildDivergences emits one stderr warning per row that did not survive
// a verified rebuild intact, so a divergence is visible without reading the
// result JSON.
func warnRebuildDivergences(report *changelog.VerifyReport) {
	for _, d := range report.Divergences {
		diag.Warn("pql.plan.rebuild_divergence", fmt.Sprintf(
			"%s: %s.%s did not survive replay unchanged", d.Kind, d.Table, d.Key))
	}
}

// warnTicketCollisions emits one stderr warning per duplicate-label
// collision surfaced by replay (D-26): a friendly ticket_id claimed by more
// than one record. Non-fatal — the collisions also ride along in the result
// JSON on stdout. Identity is collision-proof (record_id), so the graph is
// intact; this is the signal to reconcile the label with `pql ticket relabel`.
func warnTicketCollisions(collisions []changelog.TicketCollision) {
	for _, c := range collisions {
		parts := make([]string, 0, len(c.Records))
		for _, r := range c.Records {
			parts = append(parts, fmt.Sprintf("%s %q", r.RecordID, r.Title))
		}
		diag.Warn("changelog.ticket_id_collision", fmt.Sprintf(
			"ticket id %s is claimed by %d records — identity is intact (record_id), but the label is ambiguous; run `pql ticket relabel %s` to reconcile: %s",
			c.TicketID, len(c.Records), c.TicketID, strings.Join(parts, " vs "),
		))
	}
}

// --- import ---

func newPlanImportCmd() *cobra.Command {
	var legacyFile string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Replay .pql/changelog/ into pql.db",
		Long: `Replay every changelog file under .pql/changelog/ that has been
modified since the last import. Inline LWW guards on each line make
replay idempotent and order-free — the same file can be replayed any
number of times against any starting state and converge to the same
result (D-16). Used after git pull / merge to bring pql.db up to
date with incoming changelog edits, and to populate a fresh pql.db
on first open.

  pql plan import                          # replay .pql/changelog/
  pql plan import --legacy pql-plan.json   # one-time migration from
                                            # the pre-T-19 JSON snapshot

The --legacy path is for upgrading a repo that still has only the
old pql-plan.json; T-23 polishes this into an automatic migration.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			pdb, err := openPlanningDB(ctx, cfg)
			if err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}
			defer func() { _ = pdb.Close() }()

			if legacyFile != "" {
				return importLegacySnapshot(ctx, pdb.SQL(), legacyFile, cmd.OutOrStdout())
			}

			// Detect only — never rewrite from a read path. A fresh clone has
			// no merge to hook, so this is where a stale format most often
			// surfaces first.
			if err := warnChangelogFormat(cfg.Vault.Path); err != nil {
				return err
			}

			res, err := changelog.Import(ctx, pdb.SQL(), cfg.Vault.Path)
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			warnTicketCollisions(res.Collisions)

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()
			if _, err := render.One(res, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&legacyFile, "legacy", "", "import from a pre-T-19 pql-plan.json snapshot instead of replaying the changelog")
	return cmd
}

func importLegacySnapshot(ctx context.Context, db *sql.DB, path string, out io.Writer) error {
	data, err := os.ReadFile(path) //nolint:gosec // G304: user-specified --legacy path
	if err != nil {
		return &exitError{code: diag.NoInput, msg: fmt.Sprintf("read %s: %v", path, err)}
	}
	var snap repo.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return &exitError{code: diag.DataErr, msg: fmt.Sprintf("parse %s: %v", path, err)}
	}
	if err := repo.Import(ctx, db, &snap); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	_, _ = fmt.Fprintf(out, "imported from %s (legacy: %d decisions, %d tickets)\n",
		path, len(snap.Decisions), len(snap.Tickets))
	return nil
}
