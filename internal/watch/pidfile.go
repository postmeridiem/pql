package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/version"
)

const pidFileName = "watch.pid"

// PIDInfo is the JSON content of <vault>/.pql/watch.pid.
type PIDInfo struct {
	PID        int    `json:"pid"`
	Scope      string `json:"scope"`
	StartedAt  string `json:"started_at"`
	PQLVersion string `json:"pql_version"`
}

// PIDFilePath returns the path to watch.pid for a vault.
func PIDFilePath(vaultPath string) string {
	return filepath.Join(vaultPath, config.VaultStateDir, pidFileName)
}

// WritePIDFile creates the watch.pid file.
func WritePIDFile(vaultPath, scope string) error {
	info := PIDInfo{
		PID:        os.Getpid(),
		Scope:      scope,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
		PQLVersion: version.Version,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("watch: marshal pid file: %w", err)
	}
	path := PIDFilePath(vaultPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("watch: create .pql dir: %w", err)
	}
	return os.WriteFile(path, data, 0o644) //nolint:gosec // G306: pid file is not sensitive
}

// RemovePIDFile removes the watch.pid file.
func RemovePIDFile(vaultPath string) {
	_ = os.Remove(PIDFilePath(vaultPath))
}

// ReadPIDFile reads and returns the PID info. Returns nil if the file
// doesn't exist or the recorded PID is no longer alive.
func ReadPIDFile(vaultPath string) *PIDInfo {
	data, err := os.ReadFile(PIDFilePath(vaultPath))
	if err != nil {
		return nil
	}
	var info PIDInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	if !processAlive(info.PID) {
		return nil
	}
	return &info
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
