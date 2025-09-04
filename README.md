# Lil-RAG

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/your-username/lil-rag)

A simple yet powerful RAG (Retrieval Augmented Generation) system built with Go, SQLite, and Ollama. Lil-RAG provides CLI, HTTP API, and MCP server interfaces for indexing documents and performing semantic similarity searches with compression and deduplication.

## ✨ Features

### Core Capabilities
- 🔍 **Semantic Vector Search** - Advanced similarity search using SQLite with sqlite-vec extension
- 💬 **Interactive Chat** - RAG-powered chat with context and source citations
- 📄 **Multi-Format Support** - Native parsing for PDF, DOCX, XLSX, HTML, CSV, and text files
- 📚 **Document Management** - Complete CRUD operations for indexed documents
- 🗜️ **Smart Storage** - Automatic gzip compression and intelligent deduplication
- 🔄 **Complete Documents** - Returns full document content, not just chunks

### Multiple Interfaces
- 💻 **CLI Application** - Full-featured command-line interface with all operations
- 🌐 **HTTP API Server** - RESTful API with interactive web interface
- 🔌 **MCP Server** - Model Context Protocol for AI assistant integration
- 📖 **Built-in Documentation** - Comprehensive docs accessible via `/docs` route

### Performance & Reliability
- ⚡ **High Performance** - Optimized Go implementation with efficient SQLite storage
- 🤖 **Ollama Integration** - Configurable embedding and chat models via Ollama
- 🎛️ **Profile Configuration** - User-friendly configuration management
- 💾 **Persistent Storage** - Reliable SQLite database with WAL mode
- 🔧 **Health Monitoring** - Built-in health checks and metrics endpoints

## 📋 Prerequisites

- **Go 1.21+** with CGO support
- **Ollama** with an embedding model installed
- **SQLite** with sqlite-vec extension support

### Installing Dependencies

1. **Install Go**: Download from [golang.org](https://golang.org/dl/)

2. **Install Ollama**: Follow instructions at [ollama.ai](https://ollama.ai)
   ```bash
   # Start Ollama
   ollama serve
   
   # Pull an embedding model  
   ollama pull nomic-embed-text
   ```

3. **SQLite-vec Extension**: The Go bindings handle this automatically via CGO

## 🚀 Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/your-username/lil-rag.git
cd lil-rag

# Build both CLI and server
make build

# Or build individually
make build-cli      # builds bin/lil-rag
make build-server   # builds bin/lil-rag-server
make build-mcp      # builds bin/lil-rag-mcp

# Install to $GOPATH/bin (optional)
make install

# Note: Pre-built binaries are available for Linux and Windows
# macOS users should build from source using the commands above
```

### Using Go

```bash
# Install CLI directly
go install github.com/your-username/lil-rag/cmd/lil-rag@latest

# Install server directly  
go install github.com/your-username/lil-rag/cmd/lil-rag-server@latest

# Install MCP server directly
go install github.com/your-username/lil-rag/cmd/lil-rag-mcp@latest
```

## 🎯 Quick Start

### 1. Start Ollama & Pull Model

```bash
# Start Ollama (in a separate terminal)
ollama serve

# Pull an embedding model
ollama pull nomic-embed-text
```

### 2. Initialize Configuration

```bash
# Initialize user profile configuration
./bin/lil-rag config init

# View current settings
./bin/lil-rag config show
```

### 3. Index Documents

```bash
# Index direct text
./bin/lil-rag index doc1 "This is about machine learning and neural networks."

# Index from a file  
./bin/lil-rag index doc2 document.txt

# Index a PDF file
./bin/lil-rag index doc3 research_paper.pdf

# Index from stdin
echo "Content about artificial intelligence" | ./bin/lil-rag index doc4 -
```

### 4. Search Content

```bash
# Search with default limit (10)
./bin/lil-rag search "machine learning"

# Search with custom limit
./bin/lil-rag search "neural networks" 3

# Get full document content (limit=1 shows complete documents)
./bin/lil-rag search "AI concepts" 1
```

**Example Output:**
```
Found 2 results:

1. ID: doc1 [Best match: Chunk 1] (Score: 0.8542)
   This is about machine learning and neural networks. Neural networks are...
   [complete document content shown]

2. ID: doc3 [Best match: Page 1] (Score: 0.7891)
   Research Paper: Deep Learning Fundamentals...
   [complete document content shown]
```

## 💻 CLI Usage

### All Commands

- `index [id] <text|file|->` - Index content (ID optional, auto-generated if not provided)
- `search <query> [limit]` - Search for similar content  
- `chat <message> [limit]` - Interactive chat with RAG context
- `documents` - List all indexed documents
- `delete <id> [--force]` - Delete a document by ID
- `health` - Check system health status
- `config <init|show|set>` - Manage configuration
- `reset [--force]` - Delete database and all data

### Document Management

```bash
# Index with auto-generated IDs (like HTTP API)
lil-rag index "Hello world"                        # Direct text, auto ID
lil-rag index document.pdf                         # PDF file, auto ID
lil-rag index document.docx                        # Word document, auto ID
echo "Hello world" | lil-rag index -              # From stdin, auto ID

# Index with explicit IDs
lil-rag index doc1 "Hello world"                   # Direct text with ID
lil-rag index doc2 document.pdf                    # PDF file with ID
echo "Hello world" | lil-rag index doc3 -         # From stdin with ID

# List and manage documents
lil-rag documents                                   # List all documents
lil-rag delete doc1                                 # Delete with confirmation
lil-rag delete doc2 --force                        # Delete without confirmation
```

### Search & Chat

```bash
# Search examples  
lil-rag search "machine learning" 5                # Search with limit
lil-rag search "AI concepts"                       # Default limit (10)

# Chat examples
lil-rag chat "What is machine learning?" 3         # Chat with context limit
lil-rag chat "Explain neural networks"             # Default context (5 docs)
```

### System Operations

```bash
# Configuration
lil-rag config init                                 # Initialize profile config
lil-rag config show                                 # Show current config
lil-rag config set ollama.model nomic-embed-text   # Update embedding model
lil-rag config set ollama.chat-model llama3.2      # Update chat model

# System management
lil-rag health                                      # Check system health
lil-rag reset                                       # Reset database (with confirmation)
lil-rag reset --force                               # Reset database (skip confirmation)
```

### Flags

```bash
-db string           Database path (overrides profile config)
-data-dir string     Data directory (overrides profile config)
-ollama string       Ollama URL (overrides profile config)  
-model string        Embedding model (overrides profile config)
-chat-model string   Chat model (overrides profile config)
-vector-size int     Vector size (overrides profile config)
-help               Show help
-version            Show version
```

## 🌐 HTTP API

### Start the Server

```bash
# Start with default settings (localhost:8080)
./bin/lil-rag-server

# Start with custom host/port  
./bin/lil-rag-server --host 0.0.0.0 --port 9000
```

Visit http://localhost:8080 for the web interface with API documentation.

### API Endpoints

#### POST /api/index
Index content with a unique document ID.

**JSON Request:**
```bash
curl -X POST http://localhost:8080/api/index \
  -H "Content-Type: application/json" \
  -d '{
    "id": "doc1", 
    "text": "This document discusses machine learning algorithms and their applications in modern AI systems."
  }'
```

**File Upload:**
```bash
curl -X POST http://localhost:8080/api/index \
  -F "id=doc2" \
  -F "file=@document.pdf"
```

**Response:**
```json
{
  "success": true,
  "id": "doc1",
  "message": "Successfully indexed 123 characters"
}
```

#### GET /api/search & POST /api/search
Search using query parameters or JSON body.

```bash
# GET request
curl "http://localhost:8080/api/search?query=machine%20learning&limit=5"

# POST request (recommended)
curl -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "artificial intelligence applications", 
    "limit": 3
  }'
```

**Response:**
```json
{
  "results": [
    {
      "ID": "doc1",
      "Text": "This document discusses machine learning algorithms...",
      "Score": 0.8542,
      "Metadata": {
        "chunk_index": 1,
        "chunk_type": "text", 
        "is_chunk": true,
        "file_path": "/path/to/compressed/file.gz",
        "matching_chunk": "...algorithms and their applications..."
      }
    }
  ]
}
```

#### POST /api/chat
Interactive chat with RAG context and source citations.

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is machine learning?", 
    "limit": 5
  }'
```

**Response:**
```json
{
  "response": "Machine learning is a subset of artificial intelligence...",
  "sources": [
    {
      "ID": "doc1",
      "Text": "Machine learning algorithms...",
      "Score": 0.8542
    }
  ],
  "query": "What is machine learning?"
}
```

#### GET /api/documents
List all indexed documents with metadata.

```bash
curl http://localhost:8080/api/documents
```

**Response:**
```json
{
  "documents": [
    {
      "id": "doc1",
      "doc_type": "text",
      "chunk_count": 3,
      "source_path": "/path/to/file.txt",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

#### DELETE /api/documents/{id}
Delete a specific document and all its chunks.

```bash
curl -X DELETE http://localhost:8080/api/documents/doc1
```

#### GET /api/health
Health check endpoint for monitoring.

```bash
curl http://localhost:8080/api/health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0"
}
```

#### GET /api/metrics
Performance metrics and system information.

```bash
curl http://localhost:8080/api/metrics
```

### Web Interface

- **Home**: `http://localhost:8080/` - API overview and quick actions
- **Chat Interface**: `http://localhost:8080/chat` - Interactive chat with your documents
- **Document Library**: `http://localhost:8080/documents` - Browse and manage documents
- **Documentation**: `http://localhost:8080/docs` - Complete API reference and guides

## 🔌 MCP Server

The Model Context Protocol (MCP) server allows AI assistants and tools to interact with your RAG system seamlessly.

### Start the MCP Server

```bash
# Start with default settings
./bin/lil-rag-mcp

# The server uses the same profile configuration as CLI/HTTP server
# Or falls back to environment variables:
LILRAG_DB_PATH=/path/to/database.db \
LILRAG_OLLAMA_URL=http://localhost:11434 \
LILRAG_MODEL=nomic-embed-text \
./bin/lil-rag-mcp
```

### Available Tools

#### lilrag_index
Index text content into the RAG system.

**Parameters:**
- `text` (required): Text content to index
- `id` (optional): Document ID (auto-generated if not provided)

#### lilrag_index_file  
Index files (PDF, DOCX, XLSX, HTML, CSV, text).

**Parameters:**
- `file_path` (required): Path to file to index
- `id` (optional): Document ID (defaults to filename)

#### lilrag_search
Semantic similarity search.

**Parameters:**
- `query` (required): Search query
- `limit` (optional): Max results (default: 10, max: 50)

#### lilrag_chat
Interactive chat with RAG context.

**Parameters:**
- `message` (required): Question or message  
- `limit` (optional): Max context documents (default: 5, max: 20)

#### lilrag_list_documents
List all indexed documents with metadata.

**Parameters:** None

#### lilrag_delete_document
Delete a document and all its chunks.

**Parameters:**
- `document_id` (required): ID of document to delete

### Integration Examples

The MCP server can be integrated with various AI tools and assistants that support the Model Context Protocol. The server provides a standard interface for document indexing, searching, and chat functionality.

## Configuration

LilRag uses a profile-based configuration system that stores settings in a JSON file in your user profile directory (`~/.lilrag/config.json`).

### Initial Setup

```bash
# Initialize profile configuration with defaults
./bin/lil-rag config init

# View current configuration
./bin/lil-rag config show
```

### Configuration Options

The configuration includes:

- **Ollama Settings**: Endpoint URL, embedding model, and vector size
- **Storage**: Database path and data directory for indexed content
- **Server**: HTTP server host and port

Example profile configuration (`~/.lilrag/config.json`):

```json
{
  "ollama": {
    "endpoint": "http://localhost:11434",
    "embedding_model": "nomic-embed-text",
    "chat_model": "llama3.2",
    "vector_size": 768
  },
  "storage_path": "/home/user/.lilrag/data/lilrag.db",
  "data_dir": "/home/user/.lilrag/data",
  "server": {
    "host": "localhost",
    "port": 8080
  }
}
```

### Updating Configuration

```bash
# Set Ollama endpoint
./bin/lil-rag config set ollama.endpoint http://192.168.1.100:11434

# Change embedding model
./bin/lil-rag config set ollama.model all-MiniLM-L6-v2

# Change chat model
./bin/lil-rag config set ollama.chat-model llama3.2

# Update vector size (must match embedding model)
./bin/lil-rag config set ollama.vector-size 384

# Change data directory
./bin/lil-rag config set data.dir /path/to/my/data

# Update server settings
./bin/lil-rag config set server.port 9000
```

## Library Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "lil-rag/pkg/lilrag"
)

func main() {
    // Create configuration
    homeDir, _ := os.UserHomeDir()
    dataDir := filepath.Join(homeDir, ".lilrag", "data")
    
    config := &lilrag.Config{
        DatabasePath: filepath.Join(dataDir, "test.db"),
        DataDir:      dataDir,
        OllamaURL:    "http://localhost:11434",
        Model:        "nomic-embed-text", 
        VectorSize:   768,
    }

    // Initialize LilRag
    rag, err := lilrag.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer rag.Close()

    if err := rag.Initialize(); err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Index content - note the parameter order: text first, then id
    err = rag.Index(ctx, "This is a document about Go programming", "doc1")
    if err != nil {
        log.Fatal(err)
    }

    // Search for similar content
    results, err := rag.Search(ctx, "Go programming", 5)
    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results {
        fmt.Printf("ID: %s, Score: %.4f\n", result.ID, result.Score)
        fmt.Printf("Text: %s\n\n", result.Text)
    }
}
```

## Development

```bash
# Run tests
make test

# Build for current platform
make build

# Build for all platforms (Linux, macOS, Windows)
make build-cross

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean

# Install binaries to $GOPATH/bin
make install

# Show current version
make version
```

### Version Management

The project uses semantic versioning stored in the `VERSION` file. When code is merged to the main branch, the build system automatically:

1. **Increments the patch version** (e.g., 1.0.0 → 1.0.1)
2. **Builds cross-platform binaries** for Linux, macOS, and Windows
3. **Embeds the version** into the binaries at build time
4. **Creates release archives** with checksums
5. **Updates the VERSION file** in the repository

### Cross-Platform Builds

The CI/CD system builds binaries for:
- **Linux**: AMD64, ARM64
- **macOS**: AMD64 (Intel), ARM64 (Apple Silicon)
- **Windows**: AMD64

All binaries include the version information and can be checked with:
```bash
./lil-rag --version
./lil-rag-server --version
```

## 🏗️ Architecture

```
lil-rag/
├── cmd/                    # Main applications
│   ├── lil-rag/          # CLI application
│   └── lil-rag-server/   # HTTP API server
├── pkg/                    # Public library packages
│   ├── lilrag/           # Core RAG functionality
│   │   ├── storage.go     # SQLite + sqlite-vec storage
│   │   ├── embedder.go    # Ollama integration
│   │   ├── chunker.go     # Text chunking logic
│   │   ├── compression.go # Gzip compression
│   │   ├── pdf.go         # PDF parsing
│   │   └── lilrag.go     # Main library interface
│   └── config/            # Configuration management
├── internal/               # Private application code
│   └── handlers/          # HTTP request handlers
├── examples/               # Example programs
│   ├── library/           # Library usage example
│   └── profile/           # Profile config example
├── .github/               # GitHub templates and workflows
│   ├── workflows/         # CI/CD pipelines
│   └── ISSUE_TEMPLATE/    # Issue templates
└── docs/                  # Additional documentation
```

### Key Components

- **Storage Layer**: SQLite with sqlite-vec for efficient vector operations
- **Embedding Layer**: Ollama integration with configurable models
- **Processing Layer**: Text chunking, PDF parsing, and compression
- **API Layer**: REST endpoints and CLI interface
- **Configuration**: Profile-based user configuration system

## Troubleshooting

### Configuration Issues
- Profile config location: `~/.lilrag/config.json`
- Initialize config if missing: `lil-rag config init`
- Check config values: `lil-rag config show`
- Reset to defaults: Delete config file and run `lil-rag config init`

### sqlite-vec Extension Not Found
- Ensure sqlite-vec is installed and available in your SQLite
- The extension file should be accessible as `vec0`

### Ollama Connection Issues  
- Verify Ollama is running: `ollama list`
- Check the Ollama URL: `lil-rag config show`
- Update endpoint: `lil-rag config set ollama.endpoint http://localhost:11434`
- Ensure the embedding model is pulled: `ollama pull nomic-embed-text`

### Vector Size Mismatch
- Different models have different vector sizes
- Common sizes: 768 (nomic-embed-text), 384 (all-MiniLM-L6-v2), 1536 (text-embedding-ada-002)
- Update vector size: `lil-rag config set ollama.vector-size 768`

### Data Directory Issues
- Files are stored in the configured data directory
- Check location: `lil-rag config show`
- Change location: `lil-rag config set data.dir /path/to/data`
- Ensure write permissions to the directory

## License

MIT License