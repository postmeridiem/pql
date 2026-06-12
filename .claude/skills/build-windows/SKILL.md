---
name: build-windows
description: Build and install the pql binary on a Windows dev machine (Git Bash/MSYS). Use whenever asked to build, rebuild, or install pql on Windows, or when `make build` fails with an unsupported GOOS/GOARCH pair like `mingw64_nt-…/amd64`. Wraps the build skill — same judgment steps, Windows-specific execution.
---

# Build & install pql on Windows

This is the Windows execution of the `build` skill. **Steps 0–1 there (doc
sync, version bump) are judgment calls that apply unchanged — run them first.**
Only the mechanical steps below differ, because the Makefile and install path
assume a Unix host.

## Prerequisites (once per machine)

- Go per `project.yaml` `go_version`.
- `golangci-lint` v2 (the repo's `.golangci.yaml` uses the v2 schema):

```
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

It lands in `$(go env GOPATH)/bin`, which is typically not on PATH in Git
Bash — prefix it explicitly when linting (below).

## Gate

```
go test ./... && PATH="$(go env GOPATH)/bin:$PATH" make lint
```

Never build over red. Tests that simulate read-only directories via chmod are
skipped on Windows (`skipOnWindows` in `internal/config/config_test.go`) —
chmod cannot make an NTFS directory read-only. If new tests fail only on
Windows with path-separator or permission-simulation symptoms, fix the test
for portability (assert via `filepath.Clean`/`filepath.Join`, or skip the
un-simulatable case) rather than touching product code.

## Build

```
make build GOOS=windows
```

The Makefile derives `GOOS` from `uname -s`, which MSYS reports as
`mingw64_nt-<version>` — an invalid GOOS — so the override is required. Output
is `./bin/pql`: a PE executable without the `.exe` extension. Confirm the
stamped version is clean: `./bin/pql version` prints just the number.

## Install

```
cp ./bin/pql ~/.local/bin/pql.exe && pql version
```

The PATH binary on Windows is `pql.exe` — the extension is what makes it
executable outside MSYS shells. Do **not** use `make install`: it copies to an
extensionless `pql`, leaving the stale `pql.exe` shadowing it for PowerShell
and cmd callers.

The `build` skill's "When NOT to install" gate (mid-flight pql.db schema
changes) applies here unchanged.
