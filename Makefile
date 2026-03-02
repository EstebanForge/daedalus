SHELL := /bin/bash

# Variables
BINARY_NAME := daedalus
BINARY_DIR := bin
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)
DIST_DIR := dist
VERSION ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
SIGN_IDENTITY ?= -

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOINSTALL := $(GOCMD) install

# Lint
GOLANGCI_LINT_VERSION ?= 2.10.1
GOLANGCI_LINT_BIN ?= golangci-lint
LINT_TIMEOUT ?= 5m

# Directories
CMD_DIR := cmd/daedalus

# Cross-platform output binaries
BINARY_LINUX := $(DIST_DIR)/$(BINARY_NAME)-linux-amd64
BINARY_LINUX_ARM := $(DIST_DIR)/$(BINARY_NAME)-linux-arm64
BINARY_MAC := $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64
BINARY_MAC_ARM := $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64
BINARY_WINDOWS := $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe

.PHONY: all build sign build-signed build-release build-all build-linux build-linux-arm \
	build-darwin build-darwin-arm build-windows clean test test-unit test-coverage \
	lint lint-ci lint-fix fmt vet check ci dev-check run run-dev install uninstall \
	deps tidy benchmark version release-preflight help

.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Daedalus - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'

all: test build ## Run tests and build

build: ## Build the binary for current platform
	@echo "==> build"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_PATH) ./$(CMD_DIR)
	@echo "✓ Built: $(BINARY_PATH)"

build-release: ## Build optimized release binary for current platform
	@echo "Building release binary..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_PATH) ./$(CMD_DIR)
	@echo "✓ Release binary built: $(BINARY_PATH)"

sign: build ## Sign local binary (macOS only)
	@echo "Signing $(BINARY_PATH) (macOS only)..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign -s "$(SIGN_IDENTITY)" -f "$(BINARY_PATH)"; \
		echo "✓ Signed: $(BINARY_PATH)"; \
	else \
		echo "ℹ️  Skipping sign (non-macOS)"; \
	fi

build-signed: build sign ## Build and sign local binary (macOS)

build-all: build-linux build-linux-arm build-darwin build-darwin-arm build-windows ## Build binaries for all platforms

build-linux: ## Build Linux AMD64 binary
	@echo "Building for Linux AMD64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_LINUX) ./$(CMD_DIR)
	@echo "✓ Built: $(BINARY_LINUX)"

build-linux-arm: ## Build Linux ARM64 binary
	@echo "Building for Linux ARM64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_LINUX_ARM) ./$(CMD_DIR)
	@echo "✓ Built: $(BINARY_LINUX_ARM)"

build-darwin: ## Build macOS AMD64 binary
	@echo "Building for macOS AMD64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_MAC) ./$(CMD_DIR)
	@echo "✓ Built: $(BINARY_MAC)"

build-darwin-arm: ## Build macOS ARM64 (Apple Silicon) binary
	@echo "Building for macOS ARM64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_MAC_ARM) ./$(CMD_DIR)
	@echo "✓ Built: $(BINARY_MAC_ARM)"

build-windows: ## Build Windows AMD64 binary
	@echo "Building for Windows AMD64..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -trimpath -o $(BINARY_WINDOWS) ./$(CMD_DIR)
	@echo "✓ Built: $(BINARY_WINDOWS)"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html
	@echo "✓ Clean complete"

test: ## Run all tests
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "Running tests..."
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		$(GOTEST) -v -race ./...; \
	else \
		echo "CGO disabled; running tests without -race"; \
		$(GOTEST) -v ./...; \
	fi

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		$(GOTEST) -v -race -short ./...; \
	else \
		echo "CGO disabled; running unit tests without -race"; \
		$(GOTEST) -v -short ./...; \
	fi

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...; \
	else \
		echo "CGO disabled; running coverage without -race"; \
		$(GOTEST) -v -coverprofile=coverage.out -covermode=atomic ./...; \
	fi
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not found; skipping import formatting"; \
		echo "Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi
	@echo "✓ Code formatted"

vet: ## Run go vet
	@echo "==> vet"
	@go vet ./...
	@echo "✓ Vet complete"

lint: ## Run linter
	@echo "Running linter..."
	@which $(GOLANGCI_LINT_BIN) > /dev/null || (echo "$(GOLANGCI_LINT_BIN) not installed." && exit 1)
	@actual_version="$$($(GOLANGCI_LINT_BIN) version | awk '{print $$4}' | sed 's/^v//')"; \
	if [ "$$actual_version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint version mismatch: required $(GOLANGCI_LINT_VERSION), found $$actual_version"; \
		exit 1; \
	fi
	@echo "Using golangci-lint $(GOLANGCI_LINT_VERSION)"
	$(GOLANGCI_LINT_BIN) run --timeout=$(LINT_TIMEOUT)

lint-ci: ## Run linter in CI parity mode (clears cache first)
	@echo "Running CI-parity linter..."
	$(GOLANGCI_LINT_BIN) cache clean
	@$(MAKE) --no-print-directory lint

lint-fix: ## Run linter with auto-fix
	@echo "Running linter with auto-fix..."
	@which $(GOLANGCI_LINT_BIN) > /dev/null || (echo "$(GOLANGCI_LINT_BIN) not installed." && exit 1)
	@actual_version="$$($(GOLANGCI_LINT_BIN) version | awk '{print $$4}' | sed 's/^v//')"; \
	if [ "$$actual_version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint version mismatch: required $(GOLANGCI_LINT_VERSION), found $$actual_version"; \
		exit 1; \
	fi
	@echo "Using golangci-lint $(GOLANGCI_LINT_VERSION)"
	$(GOLANGCI_LINT_BIN) run --timeout=$(LINT_TIMEOUT) --fix

check: ## Run all checks (fmt, vet, lint, test, build)
	@echo "==> make fmt"
	@$(MAKE) --no-print-directory fmt
	@echo "==> make vet"
	@$(MAKE) --no-print-directory vet
	@echo "==> make lint"
	@$(MAKE) --no-print-directory lint
	@echo "==> make test"
	@$(MAKE) --no-print-directory test
	@echo "==> make build"
	@$(MAKE) --no-print-directory build
	@echo "✓ Full checks complete"

ci: check ## Run CI checks (full pipeline)
	@echo "✓ CI checks passed"

dev-check: fmt vet test-unit ## Quick development check (fmt, vet, test-unit)
	@echo "✓ Development checks passed"

run: build ## Build and run the application (usage: make run ARGS="...")
	@echo "Running $(BINARY_NAME) $(ARGS)..."
	./$(BINARY_PATH) $(ARGS)

run-dev: ## Run with hot reload (requires air)
	@which air > /dev/null || (echo "Air not installed. Install: go install github.com/cosmtrek/air@latest" && exit 1)
	@echo "Starting hot reload..."
	air

install: ## Install binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOINSTALL) $(LDFLAGS) ./$(CMD_DIR)
	@echo "✓ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

uninstall: ## Remove installed binary
	@echo "Uninstalling $(BINARY_NAME)..."
	rm -f $(shell go env GOPATH)/bin/$(BINARY_NAME)
	@echo "✓ Uninstalled"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOGET) -v -t -d ./...
	@echo "✓ Dependencies downloaded"

tidy: ## Tidy go.mod and go.sum
	@echo "Tidying modules..."
	$(GOMOD) tidy
	@echo "✓ Modules tidied"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

version: ## Display version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"

release-preflight: ## Run release checks against a local tag (usage: make release-preflight TAG=1.0.0)
	@[ -n "$(TAG)" ] || (echo "TAG is required (example: make release-preflight TAG=1.0.0)" && exit 1)
	@git rev-parse --verify --quiet "refs/tags/$(TAG)" >/dev/null || (echo "Tag not found: $(TAG)" && exit 1)
	@git diff --quiet && git diff --cached --quiet || (echo "Working tree must be clean for release-preflight" && exit 1)
	@tmp_dir="$$(mktemp -d)"; \
	echo "Using temporary worktree: $$tmp_dir"; \
	trap 'git worktree remove --force "$$tmp_dir" >/dev/null 2>&1 || true' EXIT; \
	git worktree add --detach "$$tmp_dir" "refs/tags/$(TAG)" >/dev/null; \
	cd "$$tmp_dir"; \
	$(MAKE) --no-print-directory test; \
	$(MAKE) --no-print-directory lint-ci
