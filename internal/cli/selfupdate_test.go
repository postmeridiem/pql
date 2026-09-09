package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

// T-125: self-update constructed an asset name without the version while
// goreleaser published one with it, so no released binary could ever update
// itself. The name lived as a template in .goreleaser.yaml and as a Sprintf
// here, in different languages in different files, which is why the drift
// was silent. This test closes that gap at the source: it renders the real
// name_template for every platform the release builds, and asserts the
// suffix matcher self-update actually uses picks exactly one asset per
// platform out of the full published list.

type goreleaserConfig struct {
	ProjectName string `yaml:"project_name"`
	Builds      []struct {
		Goos   []string `yaml:"goos"`
		Goarch []string `yaml:"goarch"`
		Ignore []struct {
			Goos   string `yaml:"goos"`
			Goarch string `yaml:"goarch"`
		} `yaml:"ignore"`
	} `yaml:"builds"`
	Archives []struct {
		NameTemplate    string `yaml:"name_template"`
		FormatOverrides []struct {
			Goos    string   `yaml:"goos"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
}

// renderedAssets renders .goreleaser.yaml's archive names for every
// platform the build matrix produces, with the archive extension applied
// the way goreleaser applies it (tar.gz default, format_overrides per OS).
func renderedAssets(t *testing.T, version string) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yaml: %v", err)
	}
	var cfg goreleaserConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yaml: %v", err)
	}
	if len(cfg.Builds) != 1 || len(cfg.Archives) != 1 {
		t.Fatalf("expected 1 build and 1 archive in .goreleaser.yaml, got %d and %d — update this test's assumptions",
			len(cfg.Builds), len(cfg.Archives))
	}

	tmpl, err := template.New("name").Funcs(template.FuncMap{
		// goreleaser's title func; ASCII OS names only, same caveat as
		// platformSuffixFor.
		"title": strings.Title, //nolint:staticcheck // mirrors goreleaser
	}).Parse(cfg.Archives[0].NameTemplate)
	if err != nil {
		t.Fatalf("parse name_template: %v", err)
	}

	ignored := make(map[string]bool)
	for _, ig := range cfg.Builds[0].Ignore {
		ignored[ig.Goos+"/"+ig.Goarch] = true
	}
	extFor := func(goos string) string {
		for _, fo := range cfg.Archives[0].FormatOverrides {
			if fo.Goos == goos && len(fo.Formats) > 0 {
				return fo.Formats[0]
			}
		}
		return "tar.gz"
	}

	assets := make(map[string]string) // asset name -> "goos/goarch"
	for _, goos := range cfg.Builds[0].Goos {
		for _, arch := range cfg.Builds[0].Goarch {
			if ignored[goos+"/"+arch] {
				continue
			}
			var sb strings.Builder
			err := tmpl.Execute(&sb, map[string]string{
				"ProjectName": cfg.ProjectName,
				"Version":     version,
				"Os":          goos,
				"Arch":        arch,
			})
			if err != nil {
				t.Fatalf("render name_template for %s/%s: %v", goos, arch, err)
			}
			name := strings.TrimSpace(sb.String()) + "." + extFor(goos)
			assets[name] = goos + "/" + arch
		}
	}
	return assets
}

func TestPlatformSuffixMatchesGoreleaserAssets(t *testing.T) {
	assets := renderedAssets(t, "9.9.9")
	if len(assets) == 0 {
		t.Fatal("no assets rendered from .goreleaser.yaml build matrix")
	}

	// The full published list a release carries, as self-update sees it.
	names := make([]string, 0, len(assets)+1)
	for name := range assets {
		names = append(names, name)
	}
	names = append(names, "checksums.txt")

	for name, platform := range assets {
		goos, arch, _ := strings.Cut(platform, "/")
		suffix := platformSuffixFor(goos, arch)

		var matches []string
		for _, candidate := range names {
			if strings.HasSuffix(candidate, suffix) {
				matches = append(matches, candidate)
			}
		}
		if len(matches) != 1 {
			t.Errorf("%s/%s: suffix %q matched %d assets %v, want exactly 1",
				goos, arch, suffix, len(matches), matches)
			continue
		}
		if matches[0] != name {
			t.Errorf("%s/%s: suffix %q matched %q, want %q", goos, arch, suffix, matches[0], name)
		}
	}
}

// The defect itself: the version must play no part in asset resolution.
// Rendering the same matrix at two versions must yield suffix matches for
// both — a matcher that encodes the version would fail one of them.
func TestPlatformSuffixIsVersionAgnostic(t *testing.T) {
	for _, version := range []string{"2.3.0", "10.20.30"} {
		for name, platform := range renderedAssets(t, version) {
			goos, arch, _ := strings.Cut(platform, "/")
			if !strings.HasSuffix(name, platformSuffixFor(goos, arch)) {
				t.Errorf("version %s: asset %q does not end in suffix %q",
					version, name, platformSuffixFor(goos, arch))
			}
		}
	}
}
