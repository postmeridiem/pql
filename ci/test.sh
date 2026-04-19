#!/usr/bin/env bash
# CI test entry. Unit + race + integration. Under 5 min budget on PR.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> unit tests"
go test ./...

echo "==> race detector"
go test -race ./...

echo "==> integration tests"
go test -tags=integration ./internal/cli/...
