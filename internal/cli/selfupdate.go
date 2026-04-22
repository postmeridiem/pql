package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/version"
)

const releasesAPI = "https://api.github.com/repos/postmeridiem/pql/releases/latest"

func newSelfUpdateCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update pql to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSelfUpdate(cmd, force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "update even if already on latest")
	return cmd
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func runSelfUpdate(cmd *cobra.Command, force bool) error {
	rel, err := fetchLatestRelease()
	if err != nil {
		return &exitError{code: diag.Unavail, msg: err.Error()}
	}

	latestVersion := strings.TrimPrefix(rel.TagName, "v")
	if !force && latestVersion == version.Version {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "already on %s\n", version.Version)
		return nil
	}

	assetName := archiveNameForPlatform()
	checksumName := "checksums.txt"

	var assetURL, checksumURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
		}
		if a.Name == checksumName {
			checksumURL = a.BrowserDownloadURL
		}
	}
	if assetURL == "" {
		return &exitError{code: diag.Unavail, msg: fmt.Sprintf("no asset %q in release %s", assetName, rel.TagName)}
	}

	archiveData, err := download(assetURL)
	if err != nil {
		return &exitError{code: diag.Unavail, msg: fmt.Sprintf("download: %v", err)}
	}

	if checksumURL != "" {
		if err := verifyChecksum(archiveData, assetName, checksumURL); err != nil {
			return &exitError{code: diag.Software, msg: fmt.Sprintf("checksum: %v", err)}
		}
	}

	binary, err := extractBinary(archiveData, assetName)
	if err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("extract: %v", err)}
	}

	execPath, err := os.Executable()
	if err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("find executable: %v", err)}
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("resolve symlink: %v", err)}
	}

	if err := atomicReplace(execPath, binary); err != nil {
		return &exitError{code: diag.Software, msg: fmt.Sprintf("replace: %v", err)}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated %s → %s\n", version.Version, latestVersion)
	return nil
}

func fetchLatestRelease() (*ghRelease, error) {
	resp, err := http.Get(releasesAPI) //nolint:gosec,noctx // URL is a constant
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func archiveNameForPlatform() string {
	goos := runtime.GOOS
	arch := runtime.GOARCH

	osName := strings.Title(goos) //nolint:staticcheck // simple case
	archName := arch
	if archName == "amd64" {
		archName = "x86_64"
	}

	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("pql_%s_%s.%s", osName, archName, ext)
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec,noctx // URL from GitHub API
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(data []byte, assetName, checksumURL string) error {
	checksumData, err := download(checksumURL)
	if err != nil {
		return fmt.Errorf("fetch checksums: %w", err)
	}
	actual := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actual[:])

	for _, line := range strings.Split(string(checksumData), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			if parts[0] != actualHex {
				return fmt.Errorf("SHA256 mismatch: expected %s, got %s", parts[0], actualHex)
			}
			return nil
		}
	}
	return fmt.Errorf("no checksum found for %s", assetName)
}

func extractBinary(archiveData []byte, assetName string) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZip(archiveData)
	}
	return extractFromTarGz(archiveData)
}

func extractFromTarGz(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		base := filepath.Base(hdr.Name)
		if base == "pql" || base == "pql.exe" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("pql binary not found in archive")
}

func extractFromZip(data []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base == "pql" || base == "pql.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			content, err := io.ReadAll(rc)
			_ = rc.Close()
			return content, err
		}
	}
	return nil, fmt.Errorf("pql binary not found in zip")
}

func atomicReplace(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, "pql-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil { //nolint:gosec // G302: binary must be executable
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
