# LilRag Examples

This directory contains comprehensive examples demonstrating all features of the lil-rag system. The examples are organized by complexity and use case to help you understand and integrate lil-rag into your applications.

## Quick Start

Before running the examples, make sure you have:

1. **Ollama installed and running**:
   ```bash
   ollama serve
   ollama pull nomic-embed-text
   ollama pull llama3.2:3b
   ollama pull llama3.2-vision  # For image processing examples
   ```

2. **LilRag built**:
   ```bash
   make build
   ```

3. **Configuration initialized** (for profile examples):
   ```bash
   ./bin/lil-rag config init
   ```

## Example Structure

### 📁 Basic Examples

#### `curl_examples.sh`
**Comprehensive HTTP API demonstration**

Shows all available API endpoints including:
- Document indexing (text and file upload)
- Semantic search (GET and POST methods)
- File upload (PDF, images with OCR)
- Document management (CRUD operations)
- Chat functionality with RAG context
- System monitoring and health checks
- Authentication endpoints

```bash
# Start the server first
./bin/lil-rag-server &

# Run the examples
cd examples
./curl_examples.sh
```

#### `library/library_usage.go`
**Direct Go library integration**

Demonstrates:
- Manual configuration vs builder patterns
- Configuration templates (FastSearch, ContextualSearch)
- Document indexing and management
- Advanced search capabilities
- Chat functionality with RAG
- Error handling and best practices
- Resource management

```bash
cd examples/library
go run library_usage.go
```

#### `profile/profile_usage.go`
**Profile-based configuration management**

Shows how to:
- Load and use profile configurations
- Convert profiles to application configs
- Profile-optimized document processing
- File processing with vision models
- Configuration analysis and recommendations

```bash
cd examples/profile
go run profile_usage.go
```

### 📁 Advanced Examples

#### `advanced_examples/file_formats_example.go`
**Multi-format document processing**

Comprehensive demonstration of:
- PDF parsing and indexing
- Image OCR with vision models
- HTML content extraction
- CSV data processing
- Markdown document handling
- Format-specific search capabilities
- Adaptive chunking strategies

```bash
cd examples/advanced_examples
go run file_formats_example.go
```

#### `advanced_examples/chat_example.go`
**RAG-powered conversational AI**

Features:
- Knowledge base creation
- Interactive chat sessions
- Context preservation across messages
- Source citation in responses
- Different chat scenarios
- Search vs chat comparison

```bash
cd examples/advanced_examples
go run chat_example.go
```

#### `advanced_examples/configuration_example.go`
**Configuration management and optimization**

Covers:
- Profile configuration loading
- Builder pattern templates
- Runtime configuration overrides
- Performance optimization strategies
- Configuration validation
- Best practices for different use cases

```bash
cd examples/advanced_examples
go run configuration_example.go
```

#### `advanced_examples/mcp_example.go`
**Model Context Protocol integration**

Demonstrates:
- MCP server functionality
- AI assistant integration
- Tool definitions and capabilities
- Real-world use case scenarios
- Interactive testing

```bash
cd examples/advanced_examples
go run mcp_example.go
```

## Feature Coverage Matrix

| Feature | curl_examples.sh | library_usage.go | profile_usage.go | file_formats_example.go | chat_example.go | configuration_example.go | mcp_example.go |
|---------|:----------------:|:----------------:|:----------------:|:----------------------:|:---------------:|:------------------------:|:--------------:|
| **Core Features** |
| Text Indexing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| File Upload | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| PDF Processing | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| Image OCR | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| HTML/CSV/XLSX | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
| **Search & Retrieval** |
| Semantic Search | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Advanced Queries | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
| **Chat & RAG** |
| Chat Functionality | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ |
| Session Management | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| Context Preservation | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| **Document Management** |
| CRUD Operations | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |
| Chunk Management | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ |
| **Configuration** |
| Manual Config | ❌ | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
| Profile Config | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ |
| Builder Pattern | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ |
| Templates | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ |
| **Advanced Features** |
| MCP Integration | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Metrics/Health | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Authentication | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Error Handling | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ | ✅ |

## Use Case Examples

### 🏢 Enterprise Knowledge Management
**Recommended examples**: `curl_examples.sh`, `chat_example.go`, `mcp_example.go`
- Index company documents and policies
- Provide AI-powered search and chat
- Integrate with existing tools via MCP

### 🔬 Research and Analysis
**Recommended examples**: `file_formats_example.go`, `configuration_example.go`
- Process academic papers (PDF, HTML)
- Optimize for quality over speed
- Handle large document collections

### 💻 Developer Documentation
**Recommended examples**: `library_usage.go`, `profile_usage.go`
- Integrate into Go applications
- Manage multiple environments
- Programmatic document management

### 🚀 Real-time Applications
**Recommended examples**: `configuration_example.go`, `library_usage.go`
- Optimize for speed and low latency
- Use fast embedding models
- Implement efficient chunking

## Performance Tuning

### Speed Optimization
```go
// Use FastSearch template
builder := lilrag.ConfigurationTemplate{}.FastSearch()
builder.WithEmbeddingModel("all-MiniLM-L6-v2").
        WithVectorSize(384).
        WithChunking(128, 19)
```

### Quality Optimization
```go
// Use ContextualSearch template
builder := lilrag.ConfigurationTemplate{}.ContextualSearch()
builder.WithEmbeddingModel("mxbai-embed-large").
        WithVectorSize(1024).
        WithChunking(512, 77)
```

### Balanced Configuration
```go
// Standard setup
config := &lilrag.Config{
    Model:      "nomic-embed-text",
    VectorSize: 768,
    MaxTokens:  256,
    Overlap:    38,
}
```

## Testing the Examples

### Prerequisites Check
```bash
# Check Ollama
curl -s http://localhost:11434/api/tags | jq '.models[].name'

# Check lil-rag build
ls -la bin/

# Check configuration (for profile examples)
./bin/lil-rag config show
```

### Running All Examples
```bash
# HTTP API examples
./bin/lil-rag-server &
sleep 2
cd examples && ./curl_examples.sh

# Go library examples
cd examples/library && go run library_usage.go
cd ../profile && go run profile_usage.go

# Advanced examples
cd ../advanced_examples
go run file_formats_example.go
go run chat_example.go
go run configuration_example.go
go run mcp_example.go
```

### Troubleshooting

**Common Issues:**

1. **Ollama not running**:
   ```bash
   ollama serve
   # In another terminal:
   ollama pull nomic-embed-text
   ```

2. **Profile not initialized**:
   ```bash
   ./bin/lil-rag config init
   ```

3. **Server not starting**:
   ```bash
   # Check if port 8080 is available
   lsof -i :8080
   # Start with different port if needed
   ./bin/lil-rag-server --port 8081
   ```

4. **Missing test files**:
   ```bash
   # Test files should be in repository
   ls -la test_pdfs/ test_images/
   ```

## Next Steps

After exploring these examples:

1. **Read the documentation**: Check `docs/` directory for detailed guides
2. **Configure for your use case**: Use `lil-rag config` commands
3. **Deploy in production**: See deployment guides in documentation
4. **Integrate with your application**: Use the library examples as a starting point
5. **Explore MCP integration**: Connect with AI assistants

## Contributing

To add new examples:

1. Follow the existing example structure
2. Include comprehensive comments
3. Cover error handling
4. Add to the feature coverage matrix above
5. Update this README

## Support

- **Documentation**: `docs/` directory
- **Configuration Help**: `lil-rag config --help`
- **API Reference**: Start server and visit `/docs`
- **Issues**: GitHub repository issues section