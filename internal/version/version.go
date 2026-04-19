// Package version exposes build-info stamped via -ldflags at build time.
package version

import "runtime"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

const SchemaVersion = 1

type BuildInfo struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	Date          string `json:"date"`
	GoVersion     string `json:"go_version"`
	SchemaVersion int    `json:"schema_version"`
}

func Info() BuildInfo {
	return BuildInfo{
		Version:       Version,
		Commit:        Commit,
		Date:          Date,
		GoVersion:     runtime.Version(),
		SchemaVersion: SchemaVersion,
	}
}
