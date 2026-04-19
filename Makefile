# pql — local dev targets. CI scripts in ci/ shell out to these where useful.

GO       ?= go
BIN_DIR  ?= bin
INSTALL_DIR ?= $(HOME)/.local/bin

# Version stamping: VERSION from git tag (or "dev"), COMMIT short SHA, DATE RFC3339.
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

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
vuln: ## govulncheck on all packages.
	govulncheck ./...

.PHONY: fmt
fmt: ## gofmt + goimports.
	gofmt -w .
	@command -v goimports >/dev/null && goimports -w . || echo "(goimports not installed; skipping)"

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

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

.DEFAULT_GOAL := help
