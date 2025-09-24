# Lil-RAG

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/streed/lil-rag)

A comprehensive and professional RAG (Retrieval Augmented Generation) system built with Go, SQLite, and Ollama. Lil-RAG provides three unified interfaces (CLI, HTTP API, and MCP server) with consistent parameters and comprehensive help documentation for indexing documents, semantic similarity searches, and AI-powered chat with advanced chunking strategies.

## ✨ Features

### Core Capabilities
- 🔍 **Semantic Vector Search** - Advanced similarity search using SQLite with sqlite-vec extension
- 💬 **Interactive Chat** - RAG-powered chat with persistent sessions and source citations
- 📄 **Multi-Format Support** - Native parsing for PDF, DOCX, XLSX, HTML, CSV, and text files
- 📚 **Document Management** - Complete CRUD operations for indexed documents
- 🗜️ **Smart Storage** - Automatic gzip compression and intelligent deduplication
- 🔄 **Complete Documents** - Returns full document content, not just chunks
- 🧩 **Advanced Chunking** - Three chunking strategies: recursive, semantic, and simple

### Multiple Unified Interfaces
- 💻 **CLI Application** - Full-featured command-line interface with comprehensive help documentation
- 🌐 **HTTP API Server** - RESTful API with interactive web interface and authentication
- 🔌 **MCP Server** - Model Context Protocol for AI assistant integration
- 📖 **Built-in Documentation** - Comprehensive docs accessible via `/docs` route
- 🔧 **Consistent Parameters** - All interfaces support identical parameters and options

### Professional Features
- ⚡ **High Performance** - Optimized Go implementation with efficient SQLite storage
- 🤖 **Ollama Integration** - Configurable embedding, chat, and vision models via Ollama
- 🎛️ **Profile Configuration** - User-friendly configuration management with comprehensive options
- 💾 **Persistent Storage** - Reliable SQLite database with WAL mode
- 🔧 **Health Monitoring** - Built-in health checks and metrics endpoints
- 🔐 **Authentication System** - Optional password protection with secure session management
- 📝 **Comprehensive Help** - Detailed help documentation for all commands and parameters

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

### Quick Install (Recommended)

**One-line install from GitHub releases:**

```bash
# Install to ~/.local/bin (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/streed/lil-rag/main/install.sh | bash

# Or download and run manually
curl -fsSL -O https://raw.githubusercontent.com/streed/lil-rag/main/install.sh
chmod +x install.sh
./install.sh

# Install to custom directory
./install.sh --dir /usr/local/bin

# Windows users can use Git Bash or WSL
```

The install script will:
- 🔍 **Auto-detect** your OS and architecture
- ⬇️ **Download** the latest release from GitHub
- 📦 **Extract** and install binaries
- ✅ **Verify** installation
- 📋 **Show** quick start instructions

### From Source

```bash
# Clone the repository
git clone https://github.com/streed/lil-rag.git
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
go install github.com/streed/lil-rag/cmd/lil-rag@latest

# Install server directly  
go install github.com/streed/lil-rag/cmd/lil-rag-server@latest

# Install MCP server directly
go install github.com/streed/lil-rag/cmd/lil-rag-mcp@latest
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
lil-rag config init

# View current settings
lil-rag config show
```

### 3. Index Documents

```bash
# Index direct text (ID auto-generated if not provided)
lil-rag index "This is about machine learning and neural networks."

# Index with explicit ID
lil-rag index doc1 "This is about machine learning and neural networks."

# Index from a file with chunking strategy
lil-rag index --chunking=semantic document.txt

# Index a PDF file with specific ID and chunking
lil-rag index --chunking=recursive doc3 research_paper.pdf

# Index from stdin with chunking strategy
echo "Content about artificial intelligence" | lil-rag index --chunking=simple -
```

### 4. Search Content

```bash
# Search with default settings
lil-rag search "machine learning"

# Search with custom limit
lil-rag search --limit=5 "neural networks"

# Search returning only matching chunks (no full documents)
lil-rag search --chunks-only "AI concepts"

# Search with both limit and chunks-only
lil-rag search --limit=3 --chunks-only "machine learning algorithms"

# Get help for all search options
lil-rag search --help
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

- `index [OPTIONS] [id] <text|file|->` - Index content with advanced chunking strategies
  - `--chunking=STRATEGY` - Choose chunking strategy: recursive, semantic, simple (default: recursive)
- `search [OPTIONS] <query>` - Search for similar content with flexible options
  - `--limit=N` - Maximum number of results to return (default: 10)
  - `--chunks-only` - Return only matching chunks without full document context
- `chat [OPTIONS] <message> [limit]` - Interactive chat with RAG context
  - `--session-id <id>` - Resume existing chat session
  - `--new-session` - Start new chat session
  - `--list-sessions` - List all chat sessions
  - `--show-sources` - Display detailed source information
- `documents` - List all indexed documents with metadata
- `delete <id> [--force]` - Delete a document by ID
- `reindex [OPTIONS]` - Reprocess all documents with new chunking strategy
  - `--chunking=STRATEGY` - Chunking strategy for reprocessing
  - `--force` - Skip confirmation prompt
- `health` - Check system health status
- `config <init|show|set>` - Manage configuration
- `auth <add|list|delete|reset-password>` - Manage authentication users
- `reset [--force]` - Delete database and all data

**💡 Tip**: All commands support `--help` or `-h` for detailed usage information and examples.

### Document Management

```bash
# Index with auto-generated IDs and chunking strategies
lil-rag index "Hello world"                                    # Direct text, auto ID, default chunking
lil-rag index --chunking=semantic document.pdf                 # PDF with semantic chunking
lil-rag index --chunking=recursive document.docx               # Word document with recursive chunking
echo "Hello world" | lil-rag index --chunking=simple -        # From stdin with simple chunking

# Index with explicit IDs and chunking strategies
lil-rag index --chunking=semantic doc1 "Hello world"           # Text with ID and semantic chunking
lil-rag index --chunking=recursive doc2 document.pdf           # PDF with ID and recursive chunking
echo "Hello world" | lil-rag index --chunking=simple doc3 -   # Stdin with ID and simple chunking

# Advanced document operations
lil-rag documents                                               # List all documents with metadata
lil-rag delete doc1                                             # Delete with confirmation
lil-rag delete doc2 --force                                     # Delete without confirmation
lil-rag reindex --chunking=semantic                             # Reprocess all documents with semantic chunking
lil-rag reindex --chunking=recursive --force                    # Reprocess without confirmation

# Get help for any command
lil-rag index --help                                            # Detailed index help with examples
lil-rag reindex --help                                          # Detailed reindex help
```

### Search & Chat

```bash
# Search examples with new options
lil-rag search "machine learning"                          # Default search (limit=10, full documents)
lil-rag search --limit=5 "machine learning"                # Search with custom limit
lil-rag search --chunks-only "AI concepts"                 # Return only matching chunks
lil-rag search --limit=3 --chunks-only "neural networks"   # Combined options

# Chat examples with session management and source control
lil-rag chat "What is machine learning?"                   # Basic chat with default sources
lil-rag chat --show-sources "Explain neural networks"      # Chat with explicit source display
lil-rag chat --new-session "Start a new conversation"      # Force new session
lil-rag chat --session-id abc123 "Continue our discussion" # Resume specific session
lil-rag chat --list-sessions                               # List all chat sessions

# Get help for detailed options
lil-rag search --help                                      # All search options and examples
lil-rag chat --help                                        # All chat options and examples
```

### Persistent Chat Sessions

LilRag supports persistent chat sessions that allow you to continue conversations across multiple CLI invocations. Each session gets a unique ID that you can use to resume conversations later.

```bash
# Create a new chat session
lil-rag chat --new-session "Hello, I want to start a conversation"
# Output: 🆕 Created new chat session: abc123-def456-ghi789
#         💡 Use --session-id abc123-def456-ghi789 to resume this conversation later

# Resume an existing chat session  
lil-rag chat --session-id abc123-def456-ghi789 "Continue our discussion"
# Output: 📝 Resuming chat session: abc123-def456-ghi789 (Title: New Chat)

# List all chat sessions
lil-rag chat --list-sessions
# Shows: ID, Title, Message count, Created/Updated timestamps

# You can also create sessions without the --new-session flag
lil-rag chat "This creates a session automatically" 
# When no session is specified, a new one is created automatically

# Combine with context limit
lil-rag chat --session-id abc123-def456-ghi789 "Follow up question" 3
```

**Session Features:**
- 💾 **Persistent Storage** - Messages are saved even if AI response fails
- 🔄 **Resume Conversations** - Use session ID to continue discussions  
- 📊 **Session Management** - List, track, and organize your conversations
- 🕐 **Automatic Timestamps** - Track when sessions were created and updated
- 📝 **Message Counting** - See how many messages are in each session
- 🆔 **Auto-generated IDs** - Unique session identifiers for easy reference

### System Operations

```bash
# Configuration management with help
lil-rag config init                                 # Initialize profile config
lil-rag config show                                 # Show current config
lil-rag config set ollama.model nomic-embed-text   # Update embedding model
lil-rag config set ollama.chat-model llama3.2      # Update chat model
lil-rag config --help                               # Get detailed config help

# Authentication management
lil-rag auth add username password                  # Add new user
lil-rag auth list                                   # List all users
lil-rag auth delete username                        # Delete user
lil-rag auth reset-password username newpass        # Reset user password
lil-rag auth --help                                 # Get detailed auth help

# System management with comprehensive help
lil-rag health                                      # Check system health
lil-rag health --help                               # Get health check details
lil-rag reset                                       # Reset database (with confirmation)
lil-rag reset --force                               # Reset database (skip confirmation)
lil-rag reset --help                                # Get detailed reset information
```

### Flags

```bash
-db string             Database path (overrides profile config)
-data-dir string       Data directory (overrides profile config)
-ollama string         Ollama URL (overrides profile config)  
-model string          Embedding model (overrides profile config)
-chat-model string     Chat model (overrides profile config)
-vision-model string   Vision model for image processing (overrides profile config)
-timeout int           Ollama timeout in seconds (overrides profile config)
-vector-size int       Vector size (overrides profile config)
-help                 Show help
-version              Show version
```

## 🌐 HTTP API

### Start the Server

```bash
# Start with default settings (localhost:12121)
lil-rag-server

# Start with custom host/port  
lil-rag-server --host 0.0.0.0 --port 9000

# Start with authentication disabled for development
lil-rag-server --no-secure

# Start with custom HTTP timeouts
lil-rag-server --read-timeout 120 --write-timeout 120 --idle-timeout 300
```

Visit http://localhost:12121 for the web interface with API documentation and interactive chat.

### 🔐 Authentication Setup

```bash
# Create first user (enables authentication)
lil-rag auth add admin mySecurePassword123

# List users
lil-rag auth list

# Disable authentication for development
lil-rag config set server.secure false
```

### 💬 Interactive Chat Interface

The HTTP server includes a modern, responsive chat interface for conversing with your indexed documents:

![Chat Interface](https://img.shields.io/badge/UI-Chat%20Interface-blue)

**Features:**
- 🎨 Modern, responsive design with JetBrains Mono font
- 📄 Document browser sidebar with click-to-view functionality  
- 💬 Real-time chat with RAG-powered responses
- 📚 Source citations with relevance scores
- 🔍 Full document display when clicking on sidebar items
- 📱 Mobile-friendly responsive layout

**Access the Chat:**
1. Start the server: `lil-rag-server`
2. Open your browser to: http://localhost:12121/chat
3. Browse indexed documents in the sidebar
4. Ask questions about your documents in the chat

**Chat Interface Capabilities:**
- Ask questions about indexed content
- View source documents with relevance scores
- Browse and preview all indexed documents
- See full document content by clicking sidebar items
- Markdown rendering for formatted responses

### API Endpoints

#### POST /api/index
Index content with a unique document ID and advanced chunking strategies.

**JSON Request with Chunking Strategy:**
```bash
curl -X POST http://localhost:8080/api/index \
  -H "Content-Type: application/json" \
  -d '{
    "id": "doc1",
    "text": "This document discusses machine learning algorithms and their applications in modern AI systems.",
    "chunking_strategy": "semantic"
  }'
```

**File Upload with Chunking Strategy:**
```bash
curl -X POST http://localhost:8080/api/index \
  -F "id=doc2" \
  -F "chunking_strategy=recursive" \
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
Search using query parameters or JSON body with chunks-only option.

```bash
# GET request with query parameters
curl "http://localhost:8080/api/search?query=machine%20learning&limit=5&chunks_only=false"

# POST request with all options (recommended)
curl -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "artificial intelligence applications",
    "limit": 3,
    "chunks_only": true
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
Interactive chat with RAG context, session management, and source control.

```bash
# Basic chat request
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is machine learning?",
    "limit": 5
  }'

# Advanced chat with session management and source control
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Continue our discussion about neural networks",
    "session_id": "existing-session-123",
    "new_session": false,
    "show_sources": true,
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
lil-rag-mcp

# The server uses the same profile configuration as CLI/HTTP server
# Or falls back to environment variables:
LILRAG_DB_PATH=/path/to/database.db \
LILRAG_OLLAMA_URL=http://localhost:11434 \
LILRAG_MODEL=nomic-embed-text \
lil-rag-mcp
```

### Available Tools

All MCP tools now support the same parameters as the CLI and HTTP interfaces for complete consistency.

#### lilrag_index
Index text content into the RAG system with advanced chunking strategies.

**Parameters:**
- `text` (required): Text content to index
- `id` (optional): Document ID (auto-generated if not provided)
- `chunking_strategy` (optional): Chunking strategy: recursive, semantic, simple (default: recursive)

#### lilrag_index_file
Index files (PDF, DOCX, XLSX, HTML, CSV, text) with advanced chunking strategies.

**Parameters:**
- `file_path` (required): Path to file to index
- `id` (optional): Document ID (defaults to filename)
- `chunking_strategy` (optional): Chunking strategy: recursive, semantic, simple (default: recursive)

#### lilrag_search
Semantic similarity search with flexible result options.

**Parameters:**
- `query` (required): Search query
- `limit` (optional): Max results (default: 10, max: 50)
- `chunks_only` (optional): Return only matching chunks without full document context (default: false)

#### lilrag_chat
Interactive chat with RAG context, session management, and source control.

**Parameters:**
- `message` (required): Question or message
- `limit` (optional): Max context documents (default: 5, max: 20)
- `session_id` (optional): Session ID to maintain conversation context
- `new_session` (optional): Start a new chat session (default: false)
- `show_sources` (optional): Display detailed source information (default: true)

#### lilrag_list_documents
List all indexed documents with metadata.

**Parameters:** None

#### lilrag_delete_document
Delete a document and all its chunks with optional force mode.

**Parameters:**
- `document_id` (required): ID of document to delete
- `force` (optional): Skip confirmation prompt (default: false, note: no effect in MCP as operations are programmatic)

### Integration Examples

The MCP server can be integrated with various AI tools and assistants that support the Model Context Protocol. The server provides a standard interface for document indexing, searching, and chat functionality.

## 🧩 Chunking Strategies

Lil-RAG supports three advanced chunking strategies across all interfaces (CLI, HTTP, and MCP):

### Recursive Chunking (Default)
- **Best for**: General-purpose text processing and most documents
- **Approach**: Hierarchical text splitting with semantic boundaries
- **Features**:
  - Respects paragraph and sentence boundaries
  - Maintains logical document structure
  - Optimal balance between context and precision
- **Use when**: You want reliable, consistent chunking for mixed content types

### Semantic Chunking
- **Best for**: Documents where topic coherence is critical
- **Approach**: Adaptive chunking focused on semantic similarity between sentences
- **Features**:
  - Groups semantically related content together
  - Dynamically adjusts chunk boundaries based on content similarity
  - Preserves topical coherence within chunks
- **Use when**: Working with research papers, technical documentation, or content where maintaining topic boundaries is important

### Simple Chunking
- **Best for**: Quick processing and straightforward text splitting
- **Approach**: Basic character-based chunking with word boundaries
- **Features**:
  - Fast processing with minimal computational overhead
  - Predictable chunk sizes
  - Good for simple text extraction scenarios
- **Use when**: You need fast processing or working with simple, homogeneous text

### Choosing the Right Strategy

```bash
# For general documents and mixed content (recommended default)
lil-rag index --chunking=recursive document.pdf

# For academic papers and technical documents where topic coherence matters
lil-rag index --chunking=semantic research_paper.pdf

# For quick processing of simple text
lil-rag index --chunking=simple plain_text.txt

# Reprocess existing documents with a different strategy
lil-rag reindex --chunking=semantic --force
```

**💡 Performance Notes:**
- **Recursive**: Balanced performance and quality
- **Semantic**: Higher computational cost due to similarity calculations, but better topic coherence
- **Simple**: Fastest processing, minimal memory usage

All chunking strategies respect the configured `max_chars` and `overlap` settings from your profile configuration.

## Configuration

LilRag uses a profile-based configuration system that stores settings in a JSON file in your user profile directory (`~/.lilrag/config.json`).

### Initial Setup

```bash
# Initialize profile configuration with defaults
lil-rag config init

# View current configuration
lil-rag config show
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
    "vision_model": "llama3.2-vision",
    "timeout_seconds": 30,
    "vector_size": 768
  },
  "storage_path": "/home/user/.lilrag/data/lilrag.db",
  "data_dir": "/home/user/.lilrag/data",
  "server": {
    "host": "localhost",
    "port": 8080
  },
  "chunking": {
    "max_chars": 2000,
    "overlap": 200
  }
}
```

### Advanced Configuration

#### Vision Model Configuration
LilRag supports image processing with configurable vision models for OCR and image analysis:

- **vision_model**: Vision model for image processing (default: "llama3.2-vision")
- Supports any Ollama vision model (llama3.2-vision, llava, bakllava, etc.)
- Automatically handles image files (JPG, PNG, PDF with images, etc.)

#### Timeout Configuration
Configure HTTP timeouts for Ollama API calls:

- **timeout_seconds**: Base timeout for API calls (default: 30 seconds)
- **Embeddings**: Uses the exact timeout value
- **Chat operations**: Uses 4x timeout (120s default) for longer responses  
- **Vision/Image processing**: Uses 10x timeout (300s default) for complex OCR

#### Chunking Configuration
Optimize text chunking for your use case with character-based chunking:

- **max_chars**: Maximum characters per chunk (default: 2000, optimized for modern RAG practices)
- **overlap**: Character overlap between chunks (default: 200, 10% overlap ratio)
- **chunking strategy**: Choose between recursive, semantic, or simple chunking
- Smaller chunks provide more precise search results
- Larger chunks preserve more context per result

```bash
# Optimize for precision (smaller chunks)
lil-rag config set chunking.max-chars 1000
lil-rag config set chunking.overlap 100

# Optimize for context (larger chunks)
lil-rag config set chunking.max-chars 4000
lil-rag config set chunking.overlap 400

# Use minimal chunking for simple text
lil-rag config set chunking.max-chars 500
lil-rag config set chunking.overlap 50
```

**Note**: The system has migrated from token-based to character-based chunking for more predictable and consistent results across different text types and languages.

### Updating Configuration

```bash
# Set Ollama endpoint
lil-rag config set ollama.endpoint http://192.168.1.100:11434

# Change embedding model
lil-rag config set ollama.model all-MiniLM-L6-v2

# Change chat model
lil-rag config set ollama.chat-model llama3.2

# Change vision model for image processing
lil-rag config set ollama.vision-model llama3.2-vision

# Update Ollama timeout (in seconds)
lil-rag config set ollama.timeout-seconds 60

# Update vector size (must match embedding model)
lil-rag config set ollama.vector-size 384

# Change data directory
lil-rag config set data.dir /path/to/my/data

# Update server settings
lil-rag config set server.port 9000
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
        DatabasePath:   filepath.Join(dataDir, "test.db"),
        DataDir:        dataDir,
        OllamaURL:      "http://localhost:11434",
        Model:          "nomic-embed-text",
        ChatModel:      "gemma3:4b",
        VisionModel:    "llama3.2-vision",
        TimeoutSeconds: 30,
        VectorSize:     768,
        MaxChars:       2000,
        Overlap:        200,
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

The CI/CD system builds binaries using native platform runners to avoid CGO cross-compilation issues:
- **Linux**: AMD64, ARM64 (built on Ubuntu runners)
- **macOS**: AMD64 (Intel), ARM64 (Apple Silicon) (built on macOS runners)
- **Windows**: AMD64 (built on Windows runners)

This approach uses pre-compiled Go binaries on each platform for reliable builds with CGO dependencies.

All binaries include the version information and can be checked with:
```bash
lil-rag --version
lil-rag-server --version
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

### Vision Model Issues
- Ensure vision model is available: `ollama list | grep vision`
- Pull vision model if missing: `ollama pull llama3.2-vision`
- Change vision model: `lil-rag config set ollama.vision-model llava`
- Supported models: llama3.2-vision, llava, bakllava, moondream, etc.

### Timeout Issues
- Increase timeout for slow operations: `lil-rag config set ollama.timeout-seconds 120`
- Chat timeouts use 4x base timeout (default: 120s)
- Vision processing uses 10x base timeout (default: 300s)
- Monitor /api/metrics for average response times

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
