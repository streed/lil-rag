# LilRag MCP Server

An MCP (Model Context Protocol) server implementation for LilRag, allowing AI assistants and other applications to interact with the RAG (Retrieval Augmented Generation) system.

## Features

The LilRag MCP server exposes three main tools:

### 1. `lilrag_index`
Index text content into the RAG system for later retrieval.

**Parameters:**
- `text` (string, required): The text content to index
- `id` (string, optional): Document ID. Auto-generated if not provided

**Example:**
```json
{
  "name": "lilrag_index",
  "arguments": {
    "text": "This is some important information about machine learning.",
    "id": "ml-doc-1"
  }
}
```

### 2. `lilrag_index_file`
Index a text or PDF file into the RAG system.

**Parameters:**
- `file_path` (string, required): Path to the file to index (supports .txt and .pdf files)
- `id` (string, optional): Document ID. Uses filename if not provided

**Example:**
```json
{
  "name": "lilrag_index_file", 
  "arguments": {
    "file_path": "/path/to/document.pdf",
    "id": "important-doc"
  }
}
```

### 3. `lilrag_search`
Search for relevant content using semantic similarity.

**Parameters:**
- `query` (string, required): The search query
- `limit` (integer, optional): Maximum results to return (default: 10, max: 50)

**Example:**
```json
{
  "name": "lilrag_search",
  "arguments": {
    "query": "machine learning algorithms",
    "limit": 5
  }
}
```

## Installation

Build the MCP server:

```bash
go build -o lil-rag-mcp ./cmd/lil-rag-mcp
```

## Configuration

The server can be configured in two ways:

### 1. Profile Configuration (Recommended)
Use the same profile configuration as other LilRag tools. Create or update your profile using:

```bash
lil-rag configure
```

### 2. Environment Variables
If no profile is found, the server uses environment variables:

- `LILRAG_DB_PATH`: Database file path (default: "lilrag.db")
- `LILRAG_DATA_DIR`: Data directory (default: "data")  
- `LILRAG_OLLAMA_URL`: Ollama server URL (default: "http://localhost:11434")
- `LILRAG_MODEL`: Embedding model (default: "nomic-embed-text")
- `LILRAG_VECTOR_SIZE`: Vector dimensions (default: 768)
- `LILRAG_MAX_TOKENS`: Max tokens per chunk (default: 200)
- `LILRAG_OVERLAP`: Chunk overlap tokens (default: 50)

## Usage

### As an MCP Server
Start the server to listen for MCP connections:

```bash
./lil-rag-mcp
```

The server communicates via stdin/stdout using the MCP protocol.

### Integration with AI Assistants

#### Claude Desktop
Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "lilrag": {
      "command": "/path/to/lil-rag-mcp",
      "args": []
    }
  }
}
```

#### Other MCP Clients
Any MCP-compatible client can connect to the server using the standard MCP protocol over stdio.

## Requirements

- Go 1.23 or higher
- Ollama server running (for embeddings)
- SQLite with vector extension support

## Troubleshooting

### Common Issues

1. **"Failed to create LilRag instance"**
   - Check that Ollama is running and accessible
   - Verify configuration settings
   - Ensure database directory is writable

2. **"Search failed"** 
   - Verify content has been indexed first
   - Check that the embedding model is available in Ollama
   - Ensure vector dimensions match your model

3. **"Failed to index file"**
   - Verify file exists and is readable
   - For PDFs, ensure the file is not corrupted
   - Check available disk space

### Logs
The server logs to stderr, which can be captured when running as an MCP server.

## Development

To modify or extend the MCP server:

1. The main server logic is in `main.go`
2. Add new tools by implementing them in the `RegisterTools` method
3. Update the README when adding new functionality
4. Run tests: `go test ./...`

## License

Same as the main LilRag project.