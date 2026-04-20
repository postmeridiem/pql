// Package config resolves the vault root, the on-disk index path, and the
// optional .pql/config.yaml that tunes indexer behaviour. The CLI calls Load(opts)
// once per invocation; everything downstream (indexer, query, render) reads
// the resulting Config.
//
// Resolution order is documented in docs/structure/initial-plan.md.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Frontmatter dialects.
const (
	FrontmatterYAML = "yaml"
	FrontmatterTOML = "toml"
)

// Wikilink dialects.
const (
	WikilinksObsidian = "obsidian"
	WikilinksPandoc   = "pandoc"
	WikilinksMarkdown = "markdown"
)

// Tag sources.
const (
	TagSourceInline      = "inline"
	TagSourceFrontmatter = "frontmatter"
)

// TagsConfig holds the tag-extraction policy.
type TagsConfig struct {
	Sources []string `yaml:"sources"`
}

// Config is the resolved view that the rest of the binary reads from.
// Source fields (Vault, DBPath, ConfigPath) record where things came from;
// the rest are loaded from .pql/config.yaml with defaults applied.
type Config struct {
	// Resolution metadata — populated by Load, not loaded from YAML.
	Vault      VaultDiscovery `yaml:"-"`
	DBPath     string         `yaml:"-"`
	ConfigPath string         `yaml:"-"` // empty if no file was loaded

	// User-tunable.
	DB          string            `yaml:"db"` // optional override of the default index path; vault-relative if not absolute
	Frontmatter string            `yaml:"frontmatter"`
	Wikilinks   string            `yaml:"wikilinks"`
	Tags        TagsConfig        `yaml:"tags"`
	Exclude     []string          `yaml:"exclude"`
	Aliases     map[string]string `yaml:"aliases"`
	// IgnoreFiles names the per-vault ignore files the indexer consults
	// when walking. Each entry is a filename (gitignore syntax) the walker
	// looks for at the vault root.
	//
	// Default is [".gitignore"] because most projects already keep their
	// exclusions there. Users add more files when they want pql-specific
	// rules without polluting .gitignore — typically [.gitignore,
	// .pqlignore] so a tiny .pqlignore can carry only the pql-only
	// deviations (e.g. drafts/) instead of duplicating .gitignore's
	// contents. Order matters: later files win on per-pattern conflicts.
	// Set to [] to disable file-based exclusions entirely.
	IgnoreFiles []string `yaml:"ignore_files"`
	GitMetadata bool     `yaml:"git_metadata"`
	FTS         bool     `yaml:"fts"`
}

// LoadOpts feeds Load. All Flag/Env fields can be empty; Load applies the
// documented precedence chain itself.
type LoadOpts struct {
	VaultFlag  string // --vault
	VaultEnv   string // $PQL_VAULT
	DBFlag     string // --db
	DBEnv      string // $PQL_DB
	ConfigFlag string // --config
	ConfigEnv  string // $PQL_CONFIG

	// Test injection. If empty, Load uses runtime values.
	StartDir string // for vault discovery (cwd if empty)
	HomeDir  string // for global config + cache lookups (os.UserHomeDir if empty)
	CacheDir string // for DB path (XDG cache lookup if empty)
}

// Load resolves the vault root, locates and loads the matching .pql/config.yaml (if
// any), applies defaults, validates, and computes the index DB path. Returns
// a fully-populated Config ready for the indexer/query layers.
func Load(opts LoadOpts) (*Config, error) {
	vd, err := DiscoverVault(VaultOpts{
		Flag:     opts.VaultFlag,
		Env:      opts.VaultEnv,
		StartDir: opts.StartDir,
	})
	if err != nil {
		return nil, err
	}

	cfg := defaults()
	cfg.Vault = vd

	cfgPath, err := resolveConfigPath(opts, vd.Path)
	if err != nil {
		return nil, err
	}
	if cfgPath != "" {
		if err := loadFile(cfgPath, cfg); err != nil {
			return nil, fmt.Errorf("config: load %q: %w", cfgPath, err)
		}
		cfg.ConfigPath = cfgPath
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	dbPath, err := resolveDBPath(opts, vd.Path, cfg.DB)
	if err != nil {
		return nil, err
	}
	cfg.DBPath = dbPath

	return cfg, nil
}

// Hash returns a stable SHA256 hex digest of the user-tunable fields. The
// indexer records this in index_meta.config_hash; if it changes between
// invocations a full reindex is the safe response.
func (c *Config) Hash() (string, error) {
	// DB is intentionally excluded — it determines WHERE the index lives,
	// not WHAT gets indexed, so changing it shouldn't trigger a reindex.
	view := struct {
		Frontmatter string            `json:"frontmatter"`
		Wikilinks   string            `json:"wikilinks"`
		Tags        TagsConfig        `json:"tags"`
		Exclude     []string          `json:"exclude"`
		Aliases     map[string]string `json:"aliases"`
		IgnoreFiles []string          `json:"ignore_files"`
		GitMetadata bool              `json:"git_metadata"`
		FTS         bool              `json:"fts"`
	}{
		Frontmatter: c.Frontmatter,
		Wikilinks:   c.Wikilinks,
		Tags:        c.Tags,
		Exclude:     c.Exclude,
		Aliases:     c.Aliases,
		IgnoreFiles: c.IgnoreFiles,
		GitMetadata: c.GitMetadata,
		FTS:         c.FTS,
	}
	b, err := json.Marshal(view)
	if err != nil {
		return "", fmt.Errorf("config: hash marshal: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func defaults() *Config {
	return &Config{
		Frontmatter: FrontmatterYAML,
		Wikilinks:   WikilinksObsidian,
		Tags:        TagsConfig{Sources: []string{TagSourceInline, TagSourceFrontmatter}},
		Exclude: []string{
			"**/.obsidian/**",
			"**/.git/**",
			"**/node_modules/**",
		},
		Aliases:     map[string]string{},
		IgnoreFiles: []string{".gitignore"},
		GitMetadata: false,
		FTS:         false,
	}
}

func loadFile(path string, into *Config) error {
	f, err := os.Open(path) //nolint:gosec // G304: caller-supplied path; pql config is by design read from a path the user picks
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // strict — unknown fields are typos worth catching
	if err := dec.Decode(into); err != nil {
		return err
	}
	return nil
}

func (c *Config) validate() error {
	if !slices.Contains([]string{FrontmatterYAML, FrontmatterTOML}, c.Frontmatter) {
		return fmt.Errorf("config: invalid frontmatter %q (want yaml|toml)", c.Frontmatter)
	}
	if !slices.Contains([]string{WikilinksObsidian, WikilinksPandoc, WikilinksMarkdown}, c.Wikilinks) {
		return fmt.Errorf("config: invalid wikilinks %q (want obsidian|pandoc|markdown)", c.Wikilinks)
	}
	for _, src := range c.Tags.Sources {
		if !slices.Contains([]string{TagSourceInline, TagSourceFrontmatter}, src) {
			return fmt.Errorf("config: invalid tag source %q (want inline|frontmatter)", src)
		}
	}
	return nil
}

// resolveConfigPath returns the path to load, or "" if no file applies.
//
// Precedence:
//  1. --config flag
//  2. PQL_CONFIG env var
//  3. <vault>/.pql/config.yaml (if it exists) — git-style "everything pql in one dir"
//  4. <home>/.config/pql/config.yaml (if it exists)
//  5. nothing — defaults only.
func resolveConfigPath(opts LoadOpts, vaultPath string) (string, error) {
	if opts.ConfigFlag != "" {
		if _, err := os.Stat(opts.ConfigFlag); err != nil {
			return "", fmt.Errorf("config: --config %q: %w", opts.ConfigFlag, err)
		}
		return filepath.Clean(opts.ConfigFlag), nil
	}
	if opts.ConfigEnv != "" {
		if _, err := os.Stat(opts.ConfigEnv); err != nil {
			return "", fmt.Errorf("config: PQL_CONFIG %q: %w", opts.ConfigEnv, err)
		}
		return filepath.Clean(opts.ConfigEnv), nil
	}
	local := filepath.Join(vaultPath, VaultStateDir, "config.yaml")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("config: stat %q: %w", local, err)
	}
	home, err := homeDir(opts.HomeDir)
	if err != nil {
		// Couldn't find home — fine, just no global config.
		return "", nil
	}
	global := filepath.Join(home, ".config", "pql", "config.yaml")
	if _, err := os.Stat(global); err == nil {
		return global, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("config: stat %q: %w", global, err)
	}
	return "", nil
}

func homeDir(injected string) (string, error) {
	if injected != "" {
		return injected, nil
	}
	return os.UserHomeDir()
}
