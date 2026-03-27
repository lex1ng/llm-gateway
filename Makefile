# LLM Gateway Makefile

.PHONY: all build build-local build-linux build-linux-arm64 build-all \
        test lint clean run tidy fmt vet deps help \
        test-short test-coverage

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Binary
BINARY_NAME=llm-gateway
OUTPUT_DIR=output
MAIN_PACKAGE=./cmd/server

# Version (from git tag or "dev")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

all: tidy lint test build

# ─── Build ──────────────────────────────────────────────

## Build for current platform (macOS / local)
build: build-local

build-local:
	@echo "Building $(BINARY_NAME) $(VERSION) (local)..."
	@mkdir -p $(OUTPUT_DIR)/local
	$(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/local/$(BINARY_NAME) $(MAIN_PACKAGE)

## Build for Linux amd64
build-linux:
	@echo "Building $(BINARY_NAME) $(VERSION) (linux/amd64)..."
	@mkdir -p $(OUTPUT_DIR)/linux/amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/linux/amd64/$(BINARY_NAME) $(MAIN_PACKAGE)

## Build for Linux arm64
build-linux-arm64:
	@echo "Building $(BINARY_NAME) $(VERSION) (linux/arm64)..."
	@mkdir -p $(OUTPUT_DIR)/linux/arm64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(OUTPUT_DIR)/linux/arm64/$(BINARY_NAME) $(MAIN_PACKAGE)

## Build all platforms
build-all: build-local build-linux build-linux-arm64
	@echo ""
	@echo "All binaries:"
	@find $(OUTPUT_DIR) -name $(BINARY_NAME) -exec ls -lh {} \;

# ─── Run ────────────────────────────────────────────────

run: build-local
	./$(OUTPUT_DIR)/local/$(BINARY_NAME)

# ─── Test ───────────────────────────────────────────────

test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

test-short:
	@echo "Running short tests..."
	$(GOTEST) -v -short ./...

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# ─── Lint ───────────────────────────────────────────────

lint: vet
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

vet:
	@echo "Running go vet..."
	$(GOVET) ./...

fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# ─── Dependencies ──────────────────────────────────────

tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# ─── Clean ─────────────────────────────────────────────

clean:
	@echo "Cleaning..."
	@rm -rf $(OUTPUT_DIR)
	@rm -f coverage.out coverage.html

# ─── Help ──────────────────────────────────────────────

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  build             Build for current platform (default)"
	@echo "  build-linux       Build for Linux amd64"
	@echo "  build-linux-arm64 Build for Linux arm64"
	@echo "  build-all         Build all platforms"
	@echo ""
	@echo "Run:"
	@echo "  run               Build and run locally"
	@echo ""
	@echo "Test:"
	@echo "  test              Run all tests with race detector"
	@echo "  test-short        Run short tests only"
	@echo "  test-coverage     Run tests with HTML coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint              Run golangci-lint"
	@echo "  vet               Run go vet"
	@echo "  fmt               Format code"
	@echo ""
	@echo "Other:"
	@echo "  tidy              Tidy go modules"
	@echo "  deps              Download dependencies"
	@echo "  clean             Clean build artifacts"
	@echo ""
	@echo "Version: $(VERSION)"
