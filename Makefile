# Synth Makefile
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

BINARY_NAME := synth
BUILD_DIR := bin
MAIN_PKG := ./cmd/synth

# Version info injected at build time.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-s -w \
	-X main.version=$(VERSION)"

# Go settings.
GOFLAGS := -trimpath
CGO_ENABLED := 1

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Targets
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

.PHONY: build build-all test test-coverage lint lint-install install clean dev help

## build: Build the synth binary for the current platform.
build:
	@echo "→ Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)
	@echo "✓ Built $(BUILD_DIR)/$(BINARY_NAME)"

## build-all: Cross-compile for all supported platforms.
build-all:
	@echo "→ Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux  GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PKG)
	GOOS=linux  GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PKG)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PKG)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PKG)
	@echo "✓ Built all platforms"

## test: Run all tests.
test:
	@echo "→ Running tests..."
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -v -count=1
	@echo "✓ Tests passed"

## test-coverage: Run tests with coverage report.
test-coverage:
	@echo "→ Running tests with coverage..."
	CGO_ENABLED=$(CGO_ENABLED) go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "✓ Coverage report complete"

## lint: Run linters (requires golangci-lint).
lint:
	@echo "→ Running linters..."
	golangci-lint run ./...
	@echo "✓ Lint passed"

## lint-install: Install golangci-lint.
lint-install:
	@echo "→ Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.2.2
	@echo "✓ Installed golangci-lint"

## install: Build and install to GOPATH/bin.
install:
	@echo "→ Installing $(BINARY_NAME)..."
	CGO_ENABLED=$(CGO_ENABLED) go install $(GOFLAGS) $(LDFLAGS) $(MAIN_PKG)
	@echo "✓ Installed $(BINARY_NAME)"

## clean: Remove build artifacts.
clean:
	@echo "→ Cleaning..."
	@rm -rf $(BUILD_DIR)
	@echo "✓ Clean"

## dev: Build and run with --help (quick smoke test).
dev: build
	@echo ""
	@$(BUILD_DIR)/$(BINARY_NAME) --help

## help: Show this help.
help:
	@echo "Synth Makefile targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
