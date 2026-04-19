#!/usr/bin/env bash
# CI lint entry. Invoked by .github/workflows/* and runs identically locally.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> golangci-lint"
golangci-lint run

echo "==> goreleaser check"
goreleaser check

echo "==> govulncheck"
govulncheck ./...
