#!/usr/bin/env bash
# CI lint entry. Invoked by .github/workflows/* and runs identically locally.
set -euo pipefail

cd "$(dirname "$0")/.."

# Fail on a missing tool with something actionable. CI installs both in the
# workflow; locally they come from brew. Without this the script dies on a bare
# "command not found", which reads like a lint finding and sent one release
# hunting for a code problem that did not exist.
for tool in golangci-lint goreleaser; do
  command -v "$tool" >/dev/null || {
    echo "ci/lint.sh: $tool not on PATH (brew install $tool)" >&2
    exit 1
  }
done

echo "==> golangci-lint"
golangci-lint run

echo "==> goreleaser check"
goreleaser check

echo "==> govulncheck"
# Same pinned go-run invocation as the Makefile's vuln target — no
# local install needed, identical locally and in CI.
go run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...
