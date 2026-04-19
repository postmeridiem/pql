#!/usr/bin/env bash
# CI release entry. Invoked on tag push by .github/workflows/release.yaml.
# Requires GITHUB_TOKEN in env so goreleaser can publish to GitHub Releases.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> goreleaser release"
goreleaser release --clean
