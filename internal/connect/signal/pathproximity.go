package signal

import (
	"path/filepath"
	"strings"
)

// PathProximity scores candidates by how close they are in the
// directory tree to the target path. Same directory = highest score.
type PathProximity struct{}

// Name implements Signal.
func (PathProximity) Name() string { return "path_proximity" }

// Score implements Signal.
func (PathProximity) Score(ctx *Context, candidatePath string) (float64, error) {
	if ctx.TargetPath == "" {
		return 0, nil
	}
	targetDir := filepath.Dir(ctx.TargetPath)
	candidateDir := filepath.Dir(candidatePath)

	if targetDir == candidateDir {
		return 1.0, nil
	}

	targetParts := strings.Split(targetDir, "/")
	candidateParts := strings.Split(candidateDir, "/")

	shared := 0
	for i := range targetParts {
		if i >= len(candidateParts) || targetParts[i] != candidateParts[i] {
			break
		}
		shared++
	}

	maxLen := len(targetParts)
	if len(candidateParts) > maxLen {
		maxLen = len(candidateParts)
	}
	if maxLen == 0 {
		return 1.0, nil
	}
	return float64(shared) / float64(maxLen), nil
}
