SHELL := /bin/bash

.PHONY: fmt vet test lint build sign build-signed check

BINARY_NAME := daedalus
BINARY_DIR := bin
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)
SIGN_IDENTITY ?= -

fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not found; skipping import formatting"; \
		echo "Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

vet:
	@echo "==> vet"
	@go vet ./...

test:
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Verifying dependencies..."
	@go mod verify
	@echo "Running tests..."
	@if [ "$$(go env CGO_ENABLED)" = "1" ]; then \
		go test -v -race ./...; \
	else \
		echo "CGO disabled; running tests without -race"; \
		go test -v ./...; \
	fi

lint:
	@echo "Linting code..."
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@golangci-lint run ./...

build:
	@echo "==> build"
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_PATH) ./cmd/daedalus

sign: build
	@echo "Signing $(BINARY_PATH) (macOS only)..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign -s "$(SIGN_IDENTITY)" -f "$(BINARY_PATH)"; \
		echo "✓ Signed: $(BINARY_PATH)"; \
	else \
		echo "ℹ️  Skipping sign (non-macOS)"; \
	fi

build-signed: build sign

check:
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
