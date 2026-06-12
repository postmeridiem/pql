#!/usr/bin/env bash
# CI lint entry. Invoked by .github/workflows/* and runs identically locally.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> golangci-lint"
golangci-lint run

echo "==> goreleaser check"
goreleaser check

echo "==> govulncheck"
# Same pinned go-run invocation as the Makefile's vuln target — no
# local install needed, identical locally and in CI.
go run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...
