# LLM Gateway Makefile

.PHONY: all build test lint clean run tidy fmt vet

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Binary name
BINARY_NAME=llm-gateway
BINARY_DIR=bin

# Main package
MAIN_PACKAGE=./cmd/server

# Build flags
LDFLAGS=-ldflags "-s -w"

all: tidy lint test build

## Build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

## Run
run: build
	./$(BINARY_DIR)/$(BINARY_NAME)

## Test
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

## Lint
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

## Dependencies
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## Clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html

## Help
help:
	@echo "Available targets:"
	@echo "  all          - tidy, lint, test, build"
	@echo "  build        - Build the binary"
	@echo "  run          - Build and run"
	@echo "  test         - Run all tests"
	@echo "  test-short   - Run short tests only"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  lint         - Run linters"
	@echo "  vet          - Run go vet"
	@echo "  fmt          - Format code"
	@echo "  tidy         - Tidy go modules"
	@echo "  deps         - Download dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  help         - Show this help"
