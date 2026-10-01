# Makefile for github.com/hamdiBouhani/leaktest

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

GO          ?= go
GOFLAGS     ?=
PKG         := github.com/hamdiBouhani/leaktest
COVERAGE    := coverage.out
COVERAGE_HTML := coverage.html

# Timeout for the test suite. Some leak tests intentionally wait for timeouts.
TEST_TIMEOUT := 60s

# Default target — run when you type just `make`.
.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Meta
# ---------------------------------------------------------------------------

.PHONY: help
help: ## Show this help.
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Build & install
# ---------------------------------------------------------------------------

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum.
	$(GO) mod tidy

.PHONY: download
download: ## Download module dependencies.
	$(GO) mod download

.PHONY: build
build: ## Compile all packages.
	$(GO) build $(GOFLAGS) ./...

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

.PHONY: test
test: ## Run the test suite.
	$(GO) test $(GOFLAGS) -count=1 -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output.
	$(GO) test $(GOFLAGS) -count=1 -timeout $(TEST_TIMEOUT) -v ./...

.PHONY: test-race
test-race: ## Run tests with the race detector enabled.
	$(GO) test $(GOFLAGS) -count=1 -timeout $(TEST_TIMEOUT) -race ./...

.PHONY: test-short
test-short: ## Run tests in short mode (skips long-running tests).
	$(GO) test $(GOFLAGS) -count=1 -timeout $(TEST_TIMEOUT) -short ./...

.PHONY: cover
cover: ## Generate a coverage profile and HTML report.
	$(GO) test $(GOFLAGS) -count=1 -timeout $(TEST_TIMEOUT) \
		-coverprofile=$(COVERAGE) -covermode=atomic ./...
	$(GO) tool cover -html=$(COVERAGE) -o $(COVERAGE_HTML)
	@echo "Coverage report written to $(COVERAGE_HTML)"

.PHONY: cover-func
cover-func: ## Print per-function coverage to stdout.
	$(GO) test $(GOFLAGS) -count=1 -timeout $(TEST_TIMEOUT) \
		-coverprofile=$(COVERAGE) -covermode=atomic ./...
	$(GO) tool cover -func=$(COVERAGE)

# ---------------------------------------------------------------------------
# Lint & vet
# ---------------------------------------------------------------------------

.PHONY: vet
vet: ## Run go vet on all packages.
	$(GO) vet ./...

.PHONY: lint
lint: ## Run staticcheck (requires: go install honnef.co/go/tools/cmd/staticcheck@latest).
	@command -v staticcheck >/dev/null 2>&1 || { \
		echo "staticcheck not found; install with:"; \
		echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	}
	staticcheck ./...

.PHONY: fmt
fmt: ## Format all Go source files.
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofmt-clean.
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt-clean:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

.PHONY: check
check: fmt-check vet test ## Run fmt-check, vet, and tests. The "CI in a box" target.

# ---------------------------------------------------------------------------
# Examples
# ---------------------------------------------------------------------------

.PHONY: examples
examples: ## Run all runnable examples.
	@echo ">>> examples/basic"
	$(GO) run ./examples/basic
	@echo ""
	@echo ">>> examples/http-server"
	$(GO) run ./examples/http-server

.PHONY: examples-basic
examples-basic: ## Run the basic example only.
	$(GO) run ./examples/basic

.PHONY: examples-http
examples-http: ## Run the http-server example only.
	$(GO) run ./examples/http-server

# ---------------------------------------------------------------------------
# Docs
# ---------------------------------------------------------------------------

.PHONY: doc
doc: ## Open local package documentation in a browser.
	$(GO) doc -all .

.PHONY: doc-server
doc-server: ## Serve pkg.go.dev-style docs at http://localhost:6060.
	@echo "Open http://localhost:6060/pkg/$(PKG)/"
	$(GO) run golang.org/x/tools/cmd/godoc@latest -http=:6060

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts and coverage files.
	$(GO) clean ./...
	rm -f $(COVERAGE) $(COVERAGE_HTML)

.PHONY: all
all: check examples ## Run all checks and examples. The full "green build".