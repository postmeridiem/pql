# pql — local dev targets. CI scripts in ci/ shell out to these where useful.

# Toolchain resolution. Non-interactive shells — git hooks, agent tool runners,
# anything not started from a login shell — skip /etc/profile.d, so a
# Homebrew-installed Go, golangci-lint or goreleaser is not on PATH there. That
# is why the pre-push hook failed on a missing golangci-lint while the same
# command worked in a terminal. Resolve it here instead of making every caller
# prefix PATH; no-op when Homebrew is absent.
HOMEBREW_BIN := $(firstword $(wildcard /home/linuxbrew/.linuxbrew/bin /opt/homebrew/bin /usr/local/homebrew/bin))
ifneq ($(HOMEBREW_BIN),)
  export PATH := $(HOMEBREW_BIN):$(PATH)
endif
# Same reasoning one directory over. gitleaks ships as a release tarball that
# conventionally lands in ~/.local/bin, which a non-login shell misses for
# exactly the reason above. Without this, ci/secrets.sh announces a skip on
# every hook-invoked push and the scan quietly never runs.
ifneq ($(wildcard $(HOME)/.local/bin),)
  export PATH := $(HOME)/.local/bin:$(PATH)
endif

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
install: build ## Install ./bin/pql into $(INSTALL_DIR) and refresh its skill.
	install -m 0755 $(BIN_DIR)/pql $(INSTALL_DIR)/pql
	@# The skill is //go:embed'd, so installing the binary without refreshing
	@# the skill leaves the installed copy describing the previous build —
	@# and `pql skill status` reports "current" throughout, because it
	@# compares against the binary's own embed. Keep the two together.
	@$(INSTALL_DIR)/pql skill install >/dev/null && echo "skill refreshed from the installed binary"

.PHONY: install-dev
install-dev: build ## Symlink ./bin/pql as $(INSTALL_DIR)/pql-dev (tracks rebuilds).
	ln -sf $(abspath $(BIN_DIR)/pql) $(INSTALL_DIR)/pql-dev

# Quiet by default: a per-package "ok" wall says nothing a count doesn't, and
# buries the one line that matters when something breaks. On failure the failing
# blocks are printed and the exit status propagates, so the pre-push gate is
# unaffected.
define run_tests
@out=$$($(GO) test $(1) $(GO_PACKAGES) 2>&1); st=$$?; \
if [ $$st -ne 0 ]; then echo "$$out" | grep -E -A 25 '^--- FAIL|^panic:|^# |^FAIL' || echo "$$out"; fi; \
echo "$$out" | awk '/^--- PASS/{p++} /^--- FAIL/{f++} /^--- SKIP/{s++} \
	END{printf "%d passed, %d failed, %d skipped\n", p+0, f+0, s+0}'; \
exit $$st
endef

.PHONY: test
test: ## Unit tests, fast. Failures and a count only.
	$(call run_tests,-v)

.PHONY: test-race
test-race: ## Unit tests with the race detector. Failures and a count only.
	$(call run_tests,-race -v)

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

.PHONY: secrets
secrets: ## Scan the outgoing commits for secrets and PII (ci/secrets.sh).
	./ci/secrets.sh

.PHONY: vuln
vuln: ## govulncheck alone. Also covered by `make lint`; kept for running it in isolation.
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...

.PHONY: fmt
fmt: ## gofmt + goimports.
	gofmt -w .
	@command -v goimports >/dev/null && goimports -w . || echo "(goimports not installed; skipping)"

.PHONY: fmt-check
fmt-check: ## List files gofmt would change, without writing. Read-only.
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "$$out"; else echo "all files formatted"; fi

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

.PHONY: pre-push
pre-push: ## Local pre-push gate: secrets + full lint gate + test + test-race. Wired by .githooks/pre-push.
	# First, and deliberately: it is the only check whose failure cannot be
	# undone by fixing it afterwards. A failed test costs another commit; a
	# pushed secret is cached and indexed whether or not it is deleted. The
	# repo already asked for this in CLAUDE.md ("keep the environment out of
	# it") as a grep to run by hand — which is how a home-directory path in
	# the T-25 changelog entry reached the remote and stayed.
	$(MAKE) secrets
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

.PHONY: binary-drift
binary-drift: ## Check the pql on PATH was built from HEAD. `--version` cannot tell you this.
	@./.claude/skills/pql-testing/scripts/check-binary-drift.sh

.PHONY: audit-ready
audit-ready: skill-drift binary-drift ## Both drift checks — run before a skill audit.
	@echo "Ready to audit."

# Disposable audit vault. Not /tmp: this box runs a tmpfs /tmp that is wiped on
# reboot and shared with every other process, and an audit that plants a
# malformed file then rebuilds wants somewhere durable and its own.
SCRATCH_VAULT ?= /var/mnt/data/projects/claude/scratch/pql-scratch-vault
.PHONY: scratch-vault
scratch-vault: build ## Build a disposable vault with both surfaces, safe to mutate.
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

.PHONY: scratch-poison
scratch-poison: ## Plant a file with malformed frontmatter in the scratch vault (audit shape D).
	@test -d "$(SCRATCH_VAULT)" || { echo "no scratch vault at $(SCRATCH_VAULT) — run: make scratch-vault"; exit 1; }
	@printf -- '---\ntitle: [unclosed\n  bad: : :\n---\n\n# Poisoned\n' > "$(SCRATCH_VAULT)/poisoned.md"
	@echo "Planted $(SCRATCH_VAULT)/poisoned.md — every vault command should now fail at exit 70."
	@echo "Reset with: make scratch-vault"

# The companion to scratch-poison. Planting the bad file was reachable; the
# documented recovery was not, because writing .pqlignore needs shell
# redirection. So an audit could break the vault and never test the fix.
.PHONY: scratch-ignore
scratch-ignore: ## Append PATTERN= to the scratch vault's .pqlignore (audit shape D recovery).
	@test -d "$(SCRATCH_VAULT)" || { echo "no scratch vault at $(SCRATCH_VAULT) — run: make scratch-vault"; exit 1; }
	@test -n "$(PATTERN)" || { echo "usage: make scratch-ignore PATTERN=poisoned.md"; exit 1; }
	@printf '%s\n' '$(PATTERN)' >> "$(SCRATCH_VAULT)/.pqlignore"
	@echo "Added '$(PATTERN)' to $(SCRATCH_VAULT)/.pqlignore"
	@echo "It takes effect on the next indexed command — .pqlignore is read by default."

# pql init writes into a directory and neither the skill nor --help pins which
# one, so an auditor cannot safely run it: aimed wrong it edits this repo.
# Pointing it at the scratch vault makes it exercisable.
.PHONY: scratch-init
scratch-init: build ## Run `pql init` against the scratch vault, never the repo.
	@test -d "$(SCRATCH_VAULT)" || { echo "no scratch vault at $(SCRATCH_VAULT) — run: make scratch-vault"; exit 1; }
	$(BIN_DIR)/pql --vault "$(SCRATCH_VAULT)" init
	@echo
	@echo "Ran against $(SCRATCH_VAULT). Reset with: make scratch-vault"

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
