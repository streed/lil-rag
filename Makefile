# Mini-RAG - A simple RAG system with SQLite and Ollama

.PHONY: build test clean install fmt vet lint help dev examples deps coverage

# Build binary names
BINARY_CLI=mini-rag
BINARY_SERVER=mini-rag-server

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

# Default target
help: ## Show this help message
	@echo 'Mini-RAG Build System'
	@echo ''
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build CLI and server binaries
	@echo "Building $(BINARY_CLI) and $(BINARY_SERVER)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_CLI) ./cmd/mini-rag
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_SERVER) ./cmd/mini-rag-server
	@echo "Build complete!"

build-cli: ## Build only the CLI binary
	@echo "Building $(BINARY_CLI)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_CLI) ./cmd/mini-rag

build-server: ## Build only the server binary
	@echo "Building $(BINARY_SERVER)..."
	@mkdir -p bin
	$(GOBUILD) $(BUILDFLAGS) $(LDFLAGS) -o bin/$(BINARY_SERVER) ./cmd/mini-rag-server

test: ## Run tests
	$(GOTEST) -v ./pkg/... ./internal/... ./cmd/...

coverage: ## Run tests with coverage
	$(GOTEST) -cover -coverprofile=coverage.out ./pkg/... ./internal/... ./cmd/...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f bin/$(BINARY_CLI) bin/$(BINARY_SERVER)
	rm -f coverage.out coverage.html

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing binaries to $(GOPATH)/bin..."
	cp bin/$(BINARY_CLI) $(GOPATH)/bin/
	cp bin/$(BINARY_SERVER) $(GOPATH)/bin/
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

.DEFAULT_GOAL := help