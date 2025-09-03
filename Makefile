# Lil-RAG - A simple RAG system with SQLite and Ollama

.PHONY: build test clean install fmt vet lint help dev examples deps coverage build-cross clean-dist version

# Build binary names
BINARY_CLI=lil-rag
BINARY_SERVER=lil-rag-server
BINARY_MCP=lil-rag-mcp

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags="-s -w"
BUILDFLAGS=-trimpath

# Get version from VERSION file if it exists, otherwise use "dev"
VERSION=$(shell if [ -f VERSION ]; then cat VERSION; else echo "dev"; fi)
LDFLAGS_WITH_VERSION=-ldflags="-s -w -X main.version=$(VERSION)"

# Default target
help: ## Show this help message
	@echo 'Lil-RAG Build System'
	@echo ''
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build all binaries (CLI, server, MCP)
	@echo "Building $(BINARY_CLI), $(BINARY_SERVER), and $(BINARY_MCP) version $(VERSION)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o bin/$(BINARY_CLI) ./cmd/lil-rag
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o bin/$(BINARY_SERVER) ./cmd/lil-rag-server
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o bin/$(BINARY_MCP) ./cmd/lil-rag-mcp
	@echo "Build complete!"

build-cli: ## Build only the CLI binary
	@echo "Building $(BINARY_CLI) version $(VERSION)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o bin/$(BINARY_CLI) ./cmd/lil-rag

build-server: ## Build only the server binary
	@echo "Building $(BINARY_SERVER) version $(VERSION)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o bin/$(BINARY_SERVER) ./cmd/lil-rag-server

build-mcp: ## Build only the MCP server binary
	@echo "Building $(BINARY_MCP) version $(VERSION)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o bin/$(BINARY_MCP) ./cmd/lil-rag-mcp

test: ## Run tests
	$(GOTEST) -v ./pkg/... ./internal/... ./cmd/...

coverage: ## Run tests with coverage
	$(GOTEST) -cover -coverprofile=coverage.out ./pkg/... ./internal/... ./cmd/...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f bin/$(BINARY_CLI) bin/$(BINARY_SERVER) bin/$(BINARY_MCP)
	rm -f coverage.out coverage.html
	rm -rf dist/

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing binaries to $(GOPATH)/bin..."
	cp bin/$(BINARY_CLI) $(GOPATH)/bin/
	cp bin/$(BINARY_SERVER) $(GOPATH)/bin/
	cp bin/$(BINARY_MCP) $(GOPATH)/bin/
	@echo "Installation complete!"

fmt: ## Format Go code
	$(GOFMT) -s -w .

vet: ## Run go vet
	$(GOVET) ./...

lint: fmt vet ## Run linting tools
	@echo "Code formatted and vetted successfully!"

deps: ## Download and tidy dependencies
	$(GOCMD) mod download
	$(GOCMD) mod tidy
	@echo "Dependencies updated!"

dev: deps lint build ## Development build with all checks
	@echo "Development build complete!"

examples: build ## Build and validate example programs
	@echo "Validating example programs..."
	@cd examples/library && $(GOCMD) build -o /dev/null .
	@cd examples/profile && $(GOCMD) build -o /dev/null .
	@echo "Examples validated successfully!"

all: clean deps lint test build examples ## Run all checks and build everything
	@echo "All tasks completed successfully!"

build-cross: ## Build binaries for all platforms
	@echo "Building cross-platform binaries version $(VERSION)..."
	@mkdir -p dist
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_CLI)-linux-amd64 ./cmd/lil-rag
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_SERVER)-linux-amd64 ./cmd/lil-rag-server
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_CLI)-linux-arm64 ./cmd/lil-rag
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_SERVER)-linux-arm64 ./cmd/lil-rag-server
	@echo "Building for macOS AMD64..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_CLI)-darwin-amd64 ./cmd/lil-rag
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_SERVER)-darwin-amd64 ./cmd/lil-rag-server
	@echo "Building for macOS ARM64..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_CLI)-darwin-arm64 ./cmd/lil-rag
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_SERVER)-darwin-arm64 ./cmd/lil-rag-server
	@echo "Building for Windows AMD64..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_CLI)-windows-amd64.exe ./cmd/lil-rag
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc $(GOBUILD) $(BUILDFLAGS) $(LDFLAGS_WITH_VERSION) -o dist/$(BINARY_SERVER)-windows-amd64.exe ./cmd/lil-rag-server
	@echo "Cross-platform build complete!"

clean-dist: ## Clean distribution artifacts
	rm -rf dist/

version: ## Show current version
	@echo "$(VERSION)"

.DEFAULT_GOAL := help