package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/connect"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/index"
	"github.com/postmeridiem/pql/internal/intent/related"
	"github.com/postmeridiem/pql/internal/store"
	"github.com/postmeridiem/pql/internal/telemetry"
)

func newRelatedCmd() *cobra.Command {
	var proj *projection
	cmd := &cobra.Command{
		Use:   "related <path>",
		Short: "Find files structurally related to a file",
		Long: `Ranks the files that sit closest to this one in the vault graph —
what it links to, what links to it, what shares its tags — weighted mostly on
link overlap.

` + rankedProjectionHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIntent(cmd, args[0], proj, func(ctx context.Context, db *store.Store, cfg *config.Config) ([]connect.Enriched, error) {
				limit, _ := cmd.Flags().GetInt("limit")
				if limit == 0 {
					limit = 10
				}
				return related.Run(ctx, db.DB(), args[0], limit)
			})
		},
	}
	proj = addRankedProjectionFlags(cmd)
	return cmd
}

// rankedProjectionHelp is the projection paragraph shared by search, related
// and context — one wording for one behaviour, rather than three that drift.
const rankedProjectionHelp = `The default projection returns path and score only. The per-signal
provenance in signals[] and the neighborhood in connections[] are the bulk of
the payload and most callers want the ranking, not its derivation (T-74). Opt
back in with --full (or --fields '*'), pick an exact key set with
--fields path,score,signals, or use --oneline for a plain path<TAB>score index.`

// addRankedProjectionFlags registers the projection flags on a ranked verb.
func addRankedProjectionFlags(cmd *cobra.Command) *projection {
	return addProjectionFlags(cmd, projectionFlags{
		Example: "path,score",
		Oneline: "path<TAB>score",
		Full:    "emit whole results, including the signals[] and connections[] the default projection omits",
	})
}

func runIntent(
	cmd *cobra.Command,
	targetPath string,
	proj *projection,
	fn func(ctx context.Context, st *store.Store, cfg *config.Config) ([]connect.Enriched, error),
) error {
	ctx := cmd.Context()
	verbose, _ := cmd.Flags().GetBool("verbose")
	tm := telemetry.New(verbose)
	defer tm.Emit()

	stopCfg := tm.Start("config")
	cfg, err := config.Load(loadOptsFromFlags(cmd))
	stopCfg()
	if err != nil {
		return &exitError{code: diag.NoInput, msg: err.Error()}
	}

	stopStore := tm.Start("store_open")
	st, err := store.Open(ctx, cfg.DBPath)
	stopStore()
	if err != nil {
		return &exitError{code: diag.Unavail, msg: err.Error()}
	}
	defer func() { _ = st.Close() }()

	stopIndex := tm.Start("index")
	if _, err := index.New(st, cfg).Run(ctx); err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}
	stopIndex()

	// A path argument names a thing; an unknown one is an error, not an
	// empty result (D-29). Without this a typo returned `[]` at exit 0,
	// which reads as "nothing is related to this file" when the truth is
	// "there is no such file" — and `pql meta` on the same typo already
	// exits 66, so the identical mistake was loud on one verb and silent on
	// the neighbouring one. `search` passes an empty targetPath: its
	// argument is a query, which is a filter, and an unmatched filter is
	// correctly empty.
	if targetPath != "" {
		indexed, err := pathIsIndexed(ctx, st, targetPath)
		if err != nil {
			return &exitError{code: diag.Software, msg: err.Error()}
		}
		if !indexed {
			return &exitError{code: diag.NoInput, msg: fmt.Sprintf("file not indexed: %s", targetPath)}
		}
	}

	flatSearch, _ := cmd.Flags().GetBool("flat-search")
	if flatSearch {
		return runFlatFallback(cmd, st, cfg, targetPath, proj)
	}

	stopEnrich := tm.Start("enrich")
	results, err := fn(ctx, st, cfg)
	stopEnrich()
	if err != nil {
		return &exitError{code: diag.Software, msg: err.Error()}
	}

	// Default projection: drop the provenance (T-74). Measured on one file,
	// `related` spent 1438 bytes to deliver 88 bytes of paths — five signal
	// objects per result, most of them zeros. The provenance is why a result
	// is accountable and stays one flag away, but the answer is the default.
	if proj.wantsDefaultProjection() {
		for i := range results {
			results[i].Signals = nil
			results[i].Connections = nil
		}
	}
	return renderProjectedList(cmd, results, proj, func(e connect.Enriched) string {
		return e.Path + "\t" + strconv.FormatFloat(e.Score, 'f', 4, 64)
	})
}

// pathIsIndexed reports whether path is a row in files. Exact match only:
// the argument these verbs take is the vault-relative path every other
// command returns, so accepting spellings here would re-introduce the
// guessing that Q-6 is about.
func pathIsIndexed(ctx context.Context, st *store.Store, path string) (bool, error) {
	var n int
	err := st.DB().QueryRowContext(ctx, `SELECT count(*) FROM files WHERE path = ?`, path).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check path %s: %w", path, err)
	}
	return n > 0, nil
}

func runFlatFallback(cmd *cobra.Command, st *store.Store, cfg *config.Config, path string, proj *projection) error {
	ctx := cmd.Context()
	rows, err := st.DB().QueryContext(ctx,
		`SELECT path FROM files WHERE path != ? ORDER BY path`, path)
	if err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("flat fallback: %v", err)}
	}
	defer func() { _ = rows.Close() }()

	type row struct {
		Path string `json:"path"`
	}
	var results []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Path); err != nil {
			return &exitError{code: diag.Software, msg: err.Error()}
		}
		results = append(results, r)
	}

	// Flat rows are already the minimum, so the projection flags have little
	// left to trim — but they still apply, so --oneline and --fields behave
	// the same either side of --flat-search. There is no score to print:
	// nothing ranked these. `--fields score` fails here naming path as the
	// only valid key, which is the honest answer.
	return renderProjectedList(cmd, results, proj, func(r row) string { return r.Path })
}
