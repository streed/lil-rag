# LilRag MCP Server

An MCP (Model Context Protocol) server implementation for LilRag, allowing AI assistants and other applications to interact with the comprehensive RAG (Retrieval Augmented Generation) system. The MCP server provides the same powerful features as the CLI and HTTP interfaces with complete parameter consistency.

## Features

The LilRag MCP server exposes six comprehensive tools with advanced features:

### 1. `lilrag_index`
Index text content into the RAG system with advanced chunking strategies.

**Parameters:**
- `text` (string, required): The text content to index
- `id` (string, optional): Document ID. Auto-generated if not provided
- `chunking_strategy` (string, optional): Chunking strategy: "recursive", "semantic", "simple" (default: "recursive")

**Example:**
```json
{
  "name": "lilrag_index",
  "arguments": {
    "text": "This is some important information about machine learning algorithms and their applications.",
    "id": "ml-doc-1",
    "chunking_strategy": "semantic"
  }
}
```

### 2. `lilrag_index_file`
Index files (PDF, DOCX, XLSX, HTML, CSV, text) with advanced chunking strategies.

**Parameters:**
- `file_path` (string, required): Path to the file to index (supports multiple formats)
- `id` (string, optional): Document ID. Uses filename if not provided
- `chunking_strategy` (string, optional): Chunking strategy: "recursive", "semantic", "simple" (default: "recursive")

**Example:**
```json
{
  "name": "lilrag_index_file",
  "arguments": {
    "file_path": "/path/to/research_paper.pdf",
    "id": "research-paper-2024",
    "chunking_strategy": "semantic"
  }
}
```

### 3. `lilrag_search`
Search for relevant content with flexible result options.

**Parameters:**
- `query` (string, required): The search query
- `limit` (integer, optional): Maximum results to return (default: 10, max: 50)
- `chunks_only` (boolean, optional): Return only matching chunks without full document context (default: false)

**Example:**
```json
{
  "name": "lilrag_search",
  "arguments": {
    "query": "machine learning algorithms",
    "limit": 5,
    "chunks_only": true
  }
}
```

### 4. `lilrag_chat`
Interactive chat with RAG context, session management, and source control.

**Parameters:**
- `message` (string, required): Question or message to the AI
- `limit` (integer, optional): Maximum context documents (default: 5, max: 20)
- `session_id` (string, optional): Session ID to maintain conversation context
- `new_session` (boolean, optional): Start a new chat session (default: false)
- `show_sources` (boolean, optional): Display detailed source information (default: true)

**Example:**
```json
{
  "name": "lilrag_chat",
  "arguments": {
    "message": "Explain the machine learning concepts mentioned in the research papers",
    "limit": 3,
    "show_sources": true
  }
}
```

### 5. `lilrag_list_documents`
List all indexed documents with comprehensive metadata.

**Parameters:** None

**Example:**
```json
{
  "name": "lilrag_list_documents",
  "arguments": {}
}
```

### 6. `lilrag_delete_document`
Delete a document and all its chunks from the RAG system.

**Parameters:**
- `document_id` (string, required): ID of document to delete
- `force` (boolean, optional): Skip confirmation prompt (default: false, no effect in MCP)

**Example:**
```json
{
  "name": "lilrag_delete_document",
  "arguments": {
    "document_id": "ml-doc-1",
    "force": true
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
- `LILRAG_CHAT_MODEL`: Chat model (default: "llama3.2")
- `LILRAG_VECTOR_SIZE`: Vector dimensions (default: 768)
- `LILRAG_MAX_CHARS`: Max characters per chunk (default: 2000)
- `LILRAG_OVERLAP`: Character overlap between chunks (default: 200)

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