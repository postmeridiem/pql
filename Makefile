# pql — local dev targets. CI scripts in ci/ shell out to these where useful.

GO       ?= go
BIN_DIR  ?= bin

# OS / arch detection.
OS   := $(strip $(shell uname -s | tr A-Z a-z))
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
  ARCH := amd64
endif
ifeq ($(ARCH),aarch64)
  ARCH := arm64
endif

GOOS   ?= $(OS)
GOARCH ?= $(ARCH)

# Default install directory. Override with INSTALL_DIR=.
INSTALL_DIR ?= $(HOME)/.local/bin

# Version stamping. Source of truth: project.yaml `version:` field.
# Tagged releases are handled by goreleaser, which uses the git tag.
VERSION      ?= $(shell awk -F': *' '/^version:/ {gsub(/[" ]/,"",$$2); print $$2; exit}' project.yaml)
COMMIT       ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
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
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/pql ./cmd/pql

.PHONY: install
install: build ## Install ./bin/pql into $(INSTALL_DIR).
	install -m 0755 $(BIN_DIR)/pql $(INSTALL_DIR)/pql

.PHONY: install-dev
install-dev: build ## Symlink ./bin/pql as $(INSTALL_DIR)/pql-dev (tracks rebuilds).
	ln -sf $(abspath $(BIN_DIR)/pql) $(INSTALL_DIR)/pql-dev

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
eval: ## Ranking-quality eval against the golden set (ci/eval.sh). Tag: eval.
	./ci/eval.sh

.PHONY: eval-baseline
eval-baseline: ## Record current eval as baseline for diffing future runs.
	@echo "TODO: tools/eval-report writes current eval to baseline.json"

.PHONY: fuzz-dsl
fuzz-dsl: ## Fuzz the PQL DSL lexer + parser for 10m.
	$(GO) test -fuzz=. -fuzztime=10m ./internal/query/dsl/lex/
	$(GO) test -fuzz=. -fuzztime=10m ./internal/query/dsl/parse/

# Targets that delegate to ci/ rather than restating it. The scripts are the
# definition of what CI runs; a Makefile target that re-lists their steps drifts
# the moment one side gains a step, and the local gate then passes on a check
# CI does not agree with. That is not hypothetical — `make lint` was
# golangci-lint alone while ci/lint.sh had grown two more stages, so a workflow
# verified with `make lint` failed on a tool it never installed.
.PHONY: lint
lint: ## Full lint gate: golangci-lint + goreleaser check + govulncheck (ci/lint.sh).
	./ci/lint.sh

.PHONY: ci-test
ci-test: ## Exactly what CI runs for tests: unit + race + integration (ci/test.sh).
	./ci/test.sh

.PHONY: vuln
vuln: ## govulncheck alone. Also covered by `make lint`; kept for running it in isolation.
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...

.PHONY: fmt
fmt: ## gofmt + goimports.
	gofmt -w .
	@command -v goimports >/dev/null && goimports -w . || echo "(goimports not installed; skipping)"

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

.PHONY: pre-push
pre-push: ## Local pre-push gate: full lint gate + test + test-race. Wired by .githooks/pre-push.
	$(MAKE) lint
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

.PHONY: skill-drift
skill-drift: ## Compare the repo, shipped and installed copies of the pql skill.
	@./.claude/skills/pql-testing/scripts/check-drift.sh

SCRATCH_VAULT ?= /tmp/pql-scratch-vault
.PHONY: scratch-vault
scratch-vault: build ## Build a disposable vault under /tmp with both surfaces, safe to mutate.
	@rm -rf "$(SCRATCH_VAULT)"
	@mkdir -p "$(SCRATCH_VAULT)"
	@cp -r testdata/council-snapshot/. "$(SCRATCH_VAULT)/"
	@cp -r governance "$(SCRATCH_VAULT)/"
	@mkdir -p "$(SCRATCH_VAULT)/.pql"
	@cp -r .pql/changelog "$(SCRATCH_VAULT)/.pql/"
	@$(BIN_DIR)/pql --vault "$(SCRATCH_VAULT)" plan import >/dev/null
	@$(BIN_DIR)/pql --vault "$(SCRATCH_VAULT)" decisions sync >/dev/null
	@echo "Scratch vault ready: $(SCRATCH_VAULT)"
	@echo "  Surface 1 (vault):    markdown + .base files from testdata/council-snapshot"
	@echo "  Surface 2 (planning): this repo's tickets and decisions, replayed"
	@echo "  Safe to mutate — it is a copy. Re-run this target to reset it."
	@echo "  Use with: pql --vault $(SCRATCH_VAULT) <command>"

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
