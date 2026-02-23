SHELL := /bin/bash

.PHONY: fmt vet test lint build check

BINARY_NAME := daedalus
BINARY_DIR := bin
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)

fmt:
	@echo "==> formatting"
	@gofmt -w $$(find . -type f -name '*.go' -not -path './vendor/*')

vet:
	@echo "==> vet"
	@go vet ./...

test:
	@echo "==> test"
	@go test ./...

lint:
	@echo "==> lint"
	@golangci-lint run ./...

build:
	@echo "==> build"
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_PATH) ./cmd/daedalus

check: test vet lint
