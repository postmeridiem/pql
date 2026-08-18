#!/usr/bin/env bash
# CI lint entry. Invoked by .github/workflows/* and runs identically locally.
set -euo pipefail

cd "$(dirname "$0")/.."

# Fail on a missing tool with something actionable. CI installs both in the
# workflow; locally they come from brew on macOS and, on a machine without it,
# from `go install`. The hint has to match the machine: it used to say "brew
# install" unconditionally, which is unactionable on a box that has no brew and
# is printed at exactly the moment someone is already stuck. Without any of
# this the script dies on a bare "command not found", which reads like a lint
# finding and sent one release hunting for a code problem that did not exist.
#
# A case rather than an associative array on purpose — macOS still ships bash
# 3.2, where `declare -A` is a syntax error, and this script has to run there.
# `go` is checked first and by itself, because every step below needs it and
# none of them say so. Without this, golangci-lint starts, fails to shell out to
# the toolchain, and prints four lines of internal error whose subject is
# go/packages — a symptom, several layers from the missing directory that caused
# it. One line naming the cause is the whole point of this block.
for tool in go golangci-lint goreleaser; do
  command -v "$tool" >/dev/null && continue
  if command -v brew >/dev/null 2>&1; then
    hint="brew install $tool"
  else
    case "$tool" in
      go)            hint="install the Go toolchain — https://go.dev/doc/install" ;;
      golangci-lint) hint="go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" ;;
      goreleaser)    hint="go install github.com/goreleaser/goreleaser/v2@latest" ;;
      *)             hint="install $tool" ;;
    esac
  fi
  echo "ci/lint.sh: $tool not on PATH ($hint)" >&2
  echo "  Already installed? A non-login shell — a git hook, an agent — skips" >&2
  echo "  /etc/profile.d and misses GOBIN. 'make lint' resolves the documented" >&2
  echo "  locations; running this script directly does not. Anywhere else, export" >&2
  echo "  it in the same shell invocation as the command — not a preceding one." >&2
  exit 1
done

echo "==> golangci-lint"
golangci-lint run

echo "==> goreleaser check"
goreleaser check

echo "==> govulncheck"
# Same pinned go-run invocation as the Makefile's vuln target — no
# local install needed, identical locally and in CI.
go run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...
