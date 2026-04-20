# pql — local dev targets. CI scripts in ci/ shell out to these where useful.

GO       ?= go
BIN_DIR  ?= bin
INSTALL_DIR ?= $(HOME)/.local/bin

# Version stamping. Source of truth: project.yaml `version:` field. Local
# builds augment with git short SHA + dirty marker (semver build metadata).
# Tagged releases are handled by goreleaser, which uses the git tag instead.
VERSION_BASE ?= $(shell awk -F': *' '/^version:/ {gsub(/[" ]/,"",$$2); print $$2; exit}' project.yaml)
COMMIT       ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DIRTY        := $(shell git diff --quiet HEAD 2>/dev/null || echo .dirty)
VERSION      ?= $(VERSION_BASE)+$(COMMIT)$(DIRTY)
DATE         ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X 'github.com/postmeridiem/pql/internal/version.Version=$(VERSION)' \
	-X 'github.com/postmeridiem/pql/internal/version.Commit=$(COMMIT)' \
	-X 'github.com/postmeridiem/pql/internal/version.Date=$(DATE)'

GO_PACKAGES := ./...

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the pql binary into ./bin/pql with version stamped.
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/pql ./cmd/pql

.PHONY: install
install: build ## Install ./bin/pql into $(INSTALL_DIR).
	install -m 0755 $(BIN_DIR)/pql $(INSTALL_DIR)/pql

.PHONY: test
test: ## Unit tests, fast.
	$(GO) test $(GO_PACKAGES)

.PHONY: test-race
test-race: ## Unit tests with the race detector.
	$(GO) test -race $(GO_PACKAGES)

.PHONY: test-integration
test-integration: build ## Integration tests (binary + fixture vaults). Tag: integration.
	$(GO) test -tags=integration ./internal/cli/...

.PHONY: eval
eval: ## Ranking-quality eval against the golden set. Tag: eval.
	$(GO) test -tags=eval ./internal/connect/rank/...

.PHONY: eval-baseline
eval-baseline: ## Record current eval as baseline for diffing future runs.
	@echo "TODO: tools/eval-report writes current eval to baseline.json"

.PHONY: fuzz-dsl
fuzz-dsl: ## Fuzz the PQL DSL lexer + parser for 10m.
	$(GO) test -fuzz=. -fuzztime=10m ./internal/query/dsl/lex/
	$(GO) test -fuzz=. -fuzztime=10m ./internal/query/dsl/parse/

.PHONY: lint
lint: ## golangci-lint run.
	golangci-lint run

.PHONY: vuln
vuln: ## govulncheck on all packages. Uses `go run` with a pinned version so devs don't need a local install.
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...

.PHONY: fmt
fmt: ## gofmt + goimports.
	gofmt -w .
	@command -v goimports >/dev/null && goimports -w . || echo "(goimports not installed; skipping)"

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

.PHONY: pre-push
pre-push: ## Local pre-push gate: lint + vuln + test + test-race. Wired by .githooks/pre-push.
	@command -v golangci-lint >/dev/null || { echo "pre-push: golangci-lint not on PATH (brew install golangci-lint)"; exit 1; }
	$(MAKE) lint
	$(MAKE) vuln
	$(MAKE) test
	$(MAKE) test-race

.PHONY: snapshot
snapshot: ## GoReleaser snapshot build (dry-run, no publish).
	goreleaser release --snapshot --clean

.PHONY: profile-cpu
profile-cpu: ## CPU profile against the largest fixture vault.
	@echo "TODO: wire after first benchmark exists"

.PHONY: profile-mem
profile-mem: ## Memory profile against the largest fixture vault.
	@echo "TODO: wire after first benchmark exists"

.PHONY: clean
clean: ## Remove build artefacts.
	rm -rf $(BIN_DIR) dist/

COUNCIL_SRC ?= /var/mnt/data/projects/council
.PHONY: refresh-fixtures
refresh-fixtures: ## Re-copy the Council vault snapshot into testdata/. Manual; never runs in CI.
	@test -d "$(COUNCIL_SRC)" || { echo "COUNCIL_SRC=$(COUNCIL_SRC) not found"; exit 1; }
	rm -rf testdata/council-snapshot/{members,sessions}
	rm -f  testdata/council-snapshot/{council-members.base,council-sessions.base,README.md}
	cp -r "$(COUNCIL_SRC)/members" "$(COUNCIL_SRC)/sessions" testdata/council-snapshot/
	cp    "$(COUNCIL_SRC)/council-members.base" "$(COUNCIL_SRC)/council-sessions.base" testdata/council-snapshot/
	cp    "$(COUNCIL_SRC)/README.md" testdata/council-snapshot/
	@echo "Snapshot refreshed. Review with: git diff testdata/council-snapshot/"

.DEFAULT_GOAL := help
