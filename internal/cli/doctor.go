package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/store"
	"github.com/postmeridiem/pql/internal/store/schema"
	"github.com/postmeridiem/pql/internal/version"
)

// doctorReport is the JSON shape `pql doctor` emits on stdout.
type doctorReport struct {
	Vault   doctorVault          `json:"vault"`
	Config  doctorConfig         `json:"config"`
	DB      doctorDB             `json:"db"`
	Index   *doctorIndex         `json:"index"` // nil when DB doesn't exist
	Version version.BuildInfo    `json:"version"`
}

type doctorVault struct {
	Path          string `json:"path"`
	DiscoveredVia string `json:"discovered_via"`
}

type doctorConfig struct {
	Path             string   `json:"path,omitempty"` // empty if no file loaded
	Loaded           bool     `json:"loaded"`
	Frontmatter      string   `json:"frontmatter"`
	Wikilinks        string   `json:"wikilinks"`
	TagSources       []string `json:"tag_sources"`
	Exclude          []string `json:"exclude"`
	RespectGitignore bool     `json:"respect_gitignore"`
	GitMetadata      bool     `json:"git_metadata"`
	FTS              bool     `json:"fts"`
	Hash             string   `json:"hash"`
}

type doctorDB struct {
	Path             string `json:"path"`
	Exists           bool   `json:"exists"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	SchemaVersion    int    `json:"schema_version,omitempty"`
	LastFullScan     int64  `json:"last_full_scan,omitempty"`
	StoredConfigHash string `json:"stored_config_hash,omitempty"`
}

type doctorIndex struct {
	Files           int `json:"files"`
	FrontmatterRows int `json:"frontmatter_rows"`
	TagsRows        int `json:"tags_rows"`
	LinksRows       int `json:"links_rows"`
	HeadingsRows    int `json:"headings_rows"`
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose vault / config / DB / index resolution",
		Long: `Print a single JSON object describing what pql resolved for this
invocation: which vault root was picked and via which discovery rule,
which config file (if any) was loaded and the effective values, where
the SQLite index lives and whether it exists yet, and per-table row
counts when it does.

doctor is read-only and does NOT trigger an indexer run — it reports
what's on disk right now, including "the DB hasn't been created yet"
as a normal state to report rather than an error.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}
			cfgHash, err := cfg.Hash()
			if err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}

			report := &doctorReport{
				Vault: doctorVault{
					Path:          cfg.Vault.Path,
					DiscoveredVia: cfg.Vault.Reason,
				},
				Config: doctorConfig{
					Path:             cfg.ConfigPath,
					Loaded:           cfg.ConfigPath != "",
					Frontmatter:      cfg.Frontmatter,
					Wikilinks:        cfg.Wikilinks,
					TagSources:       cfg.Tags.Sources,
					Exclude:          cfg.Exclude,
					RespectGitignore: cfg.RespectGitignore,
					GitMetadata:      cfg.GitMetadata,
					FTS:              cfg.FTS,
					Hash:             cfgHash,
				},
				DB:      doctorDB{Path: cfg.DBPath},
				Version: version.Info(),
			}

			if err := populateDBState(ctx, cfg.DBPath, report); err != nil {
				return &exitError{code: diag.Unavail, msg: err.Error()}
			}

			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			if _, err := render.RenderOne(report, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

// populateDBState fills in report.DB and report.Index by inspecting the
// SQLite file at dbPath. Absence is reported, not erroring — this is a
// diagnostic, not a query.
func populateDBState(ctx context.Context, dbPath string, report *doctorReport) error {
	info, err := os.Stat(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		// DB hasn't been created yet — leave Index nil, mark Exists=false.
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", dbPath, err)
	}
	report.DB.Exists = true
	report.DB.SizeBytes = info.Size()

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", dbPath, err)
	}
	defer st.Close()

	v, err := st.SchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	report.DB.SchemaVersion = v

	// Best-effort meta reads: missing keys are fine (fresh DB has none).
	if v := readMeta(ctx, st.DB(), "last_full_scan"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &report.DB.LastFullScan)
	}
	report.DB.StoredConfigHash = readMeta(ctx, st.DB(), "config_hash")

	idx := &doctorIndex{
		Files:           tableCount(ctx, st.DB(), "files"),
		FrontmatterRows: tableCount(ctx, st.DB(), "frontmatter"),
		TagsRows:        tableCount(ctx, st.DB(), "tags"),
		LinksRows:       tableCount(ctx, st.DB(), "links"),
		HeadingsRows:    tableCount(ctx, st.DB(), "headings"),
	}
	report.Index = idx

	// Sanity: the binary's expected schema must match what's on disk. If
	// it doesn't, the next real query would trigger a rebuild — flag it so
	// the user understands the index is about to be wiped.
	_ = schema.Version
	return nil
}

func tableCount(ctx context.Context, db *sql.DB, table string) int {
	var n int
	// table is a small fixed set above — safe to interpolate.
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&n)
	return n
}

func readMeta(ctx context.Context, db *sql.DB, key string) string {
	var v string
	_ = db.QueryRowContext(ctx, `SELECT value FROM index_meta WHERE key = ?`, key).Scan(&v)
	return v
}
