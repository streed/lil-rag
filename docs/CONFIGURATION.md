# Configuration Reference

LilRag uses a profile-based configuration system that stores settings in `~/.lilrag/config.json`. This document provides comprehensive reference for all configuration options.

## Configuration File Location

- **Default location**: `~/.lilrag/config.json`
- **Created automatically** when you run any LilRag command for the first time
- **User-specific**: Each user has their own configuration profile

## Complete Configuration Schema

```json
{
  "ollama": {
    "endpoint": "http://localhost:11434",
    "embedding_model": "nomic-embed-text",
    "vector_size": 768,
    "chat_model": "gemma3:4b",
    "vision_model": "llama3.2-vision",
    "timeout_seconds": 30
  },
  "storage_path": "/home/user/.lilrag/data/lilrag.db",
  "data_dir": "/home/user/.lilrag/data",
  "server": {
    "host": "localhost",
    "port": 8080
  },
  "chunking": {
    "max_tokens": 256,
    "overlap": 38
  }
}
```

## Configuration Sections

### Ollama Configuration (`ollama`)

Controls integration with Ollama for embeddings, chat, and vision processing.

#### `endpoint`
- **Type**: String
- **Default**: `"http://localhost:11434"`
- **Description**: URL of your Ollama server
- **Examples**:
  ```bash
  # Local Ollama instance
  ./bin/lil-rag config set ollama.endpoint http://localhost:11434
  
  # Remote Ollama server
  ./bin/lil-rag config set ollama.endpoint http://192.168.1.100:11434
  
  # Custom port
  ./bin/lil-rag config set ollama.endpoint http://localhost:11435
  ```

#### `embedding_model`
- **Type**: String  
- **Default**: `"nomic-embed-text"`
- **Description**: Model used for generating text embeddings
- **Common Options**:
  - `nomic-embed-text` (768 dimensions) - General purpose, good performance
  - `nomic-embed-text:v1.5` (768 dimensions) - Improved version
  - `all-MiniLM-L6-v2` (384 dimensions) - Smaller, faster
  - `mxbai-embed-large` (1024 dimensions) - High quality embeddings
- **Examples**:
  ```bash
  # Switch to smaller, faster model
  ./bin/lil-rag config set ollama.embedding-model all-MiniLM-L6-v2
  ./bin/lil-rag config set ollama.vector-size 384
  
  # Use high-quality large model
  ./bin/lil-rag config set ollama.embedding-model mxbai-embed-large
  ./bin/lil-rag config set ollama.vector-size 1024
  ```

#### `vector_size`
- **Type**: Integer
- **Default**: `768`
- **Description**: Vector dimension size (must match embedding model)
- **Common Sizes**:
  - `384` - all-MiniLM-L6-v2
  - `768` - nomic-embed-text, sentence-transformers
  - `1024` - mxbai-embed-large
  - `1536` - OpenAI text-embedding-ada-002

#### `chat_model`
- **Type**: String
- **Default**: `"gemma3:4b"`
- **Description**: Model used for chat/RAG responses
- **Recommended Options**:
  - `gemma3:4b` - Fast, good quality responses
  - `llama3.2:3b` - Very fast, good for simple queries
  - `llama3.2:8b` - Better quality, slower
  - `qwen2.5:7b` - Excellent reasoning capabilities
- **Examples**:
  ```bash
  # Use faster model for simple queries
  ./bin/lil-rag config set ollama.chat-model llama3.2:3b
  
  # Use higher quality model
  ./bin/lil-rag config set ollama.chat-model qwen2.5:7b
  ```

#### `vision_model`
- **Type**: String
- **Default**: `"llama3.2-vision"`
- **Description**: Model used for image processing and OCR
- **Supported Options**:
  - `llama3.2-vision` - Latest, best performance
  - `llava` - General purpose vision model
  - `llava:7b` - Smaller version of LLaVA
  - `llava:13b` - Larger, more capable version
  - `bakllava` - BakLLaVA vision model
  - `moondream` - Specialized for detailed descriptions
- **Examples**:
  ```bash
  # Use LLaVA for image processing
  ./bin/lil-rag config set ollama.vision-model llava:7b
  
  # Use specialized model for detailed image analysis
  ./bin/lil-rag config set ollama.vision-model moondream
  ```

#### `timeout_seconds`
- **Type**: Integer
- **Default**: `30`
- **Description**: Base timeout for Ollama API calls in seconds
- **Timeout Multipliers**:
  - **Embeddings**: Uses exact value (30s default)
  - **Chat operations**: Uses 4x value (120s default)
  - **Vision processing**: Uses 10x value (300s default)
- **Recommendations**:
  - `15-30` for fast local GPU setups
  - `60-120` for CPU inference or remote servers
  - `180+` for very large models or slow hardware
- **Examples**:
  ```bash
  # Fast local GPU
  ./bin/lil-rag config set ollama.timeout-seconds 15
  
  # Slow CPU inference
  ./bin/lil-rag config set ollama.timeout-seconds 120
  
  # Very large models
  ./bin/lil-rag config set ollama.timeout-seconds 300
  ```

### Storage Configuration

#### `storage_path`
- **Type**: String
- **Default**: `"/home/user/.lilrag/data/lilrag.db"`
- **Description**: Path to SQLite database file
- **Notes**: Directory will be created automatically if it doesn't exist

#### `data_dir`  
- **Type**: String
- **Default**: `"/home/user/.lilrag/data"`
- **Description**: Directory for storing file attachments and compressed documents
- **Examples**:
  ```bash
  # Custom data directory
  ./bin/lil-rag config set data-dir /path/to/my/rag/data
  
  # Network storage (ensure proper permissions)
  ./bin/lil-rag config set data-dir /mnt/shared/lilrag
  ```

### Server Configuration (`server`)

#### `host`
- **Type**: String
- **Default**: `"localhost"`
- **Description**: HTTP server bind address
- **Examples**:
  ```bash
  # Listen on all interfaces
  ./bin/lil-rag config set server.host 0.0.0.0
  
  # Listen on specific interface
  ./bin/lil-rag config set server.host 192.168.1.100
  ```

#### `port`
- **Type**: Integer  
- **Default**: `8080`
- **Description**: HTTP server port
- **Examples**:
  ```bash
  # Use alternative port
  ./bin/lil-rag config set server.port 9000
  
  # Use privileged port (requires root/admin)
  ./bin/lil-rag config set server.port 80
  ```

### Chunking Configuration (`chunking`)

Controls how documents are split into searchable chunks.

#### `max_tokens`
- **Type**: Integer
- **Default**: `256` (optimized for 2025 RAG best practices)
- **Description**: Maximum tokens per chunk
- **Recommendations**:
  - `128-256`: Precise search results, good for Q&A
  - `512-1024`: More context per result, good for summarization  
  - `1800+`: Legacy mode, preserves large context blocks
- **Examples**:
  ```bash
  # Optimize for precise search
  ./bin/lil-rag config set chunking.max-tokens 128
  
  # Optimize for context preservation
  ./bin/lil-rag config set chunking.max-tokens 512
  
  # Legacy chunking (pre-2025)
  ./bin/lil-rag config set chunking.max-tokens 1800
  ```

#### `overlap`
- **Type**: Integer
- **Default**: `38` (15% of max_tokens)
- **Description**: Token overlap between adjacent chunks
- **Purpose**: Prevents context loss at chunk boundaries
- **Recommendations**: 10-20% of max_tokens
- **Examples**:
  ```bash
  # Calculate overlap for different chunk sizes
  # For 128 tokens: 128 * 0.15 = 19
  ./bin/lil-rag config set chunking.overlap 19
  
  # For 512 tokens: 512 * 0.15 = 76  
  ./bin/lil-rag config set chunking.overlap 76
  
  # For 1800 tokens: 1800 * 0.11 = 200
  ./bin/lil-rag config set chunking.overlap 200
  ```

## Command Line Overrides

All configuration options can be overridden with command line flags:

```bash
# Override database location
./bin/lil-rag --db /tmp/test.db search "query"

# Override Ollama settings
./bin/lil-rag --ollama http://remote:11434 --model nomic-embed-text:v1.5 search "query"

# Override server settings  
./bin/lil-rag-server --host 0.0.0.0 --port 9000 --timeout 60

# Override vision model
./bin/lil-rag-server --vision-model llava --chat-model qwen2.5:7b
```

## Configuration Management Commands

```bash
# Initialize default configuration
./bin/lil-rag config init

# Show current configuration
./bin/lil-rag config show

# Set specific values
./bin/lil-rag config set section.key value

# Examples of setting values
./bin/lil-rag config set ollama.endpoint http://localhost:11434
./bin/lil-rag config set ollama.vision-model llava
./bin/lil-rag config set ollama.timeout-seconds 60  
./bin/lil-rag config set server.port 9000
./bin/lil-rag config set chunking.max-tokens 512
```

## Performance Optimization

### For Speed
```bash
# Fast embedding model
./bin/lil-rag config set ollama.embedding-model all-MiniLM-L6-v2
./bin/lil-rag config set ollama.vector-size 384

# Fast chat model  
./bin/lil-rag config set ollama.chat-model llama3.2:3b

# Small chunks for precise results
./bin/lil-rag config set chunking.max-tokens 128
./bin/lil-rag config set chunking.overlap 19

# Lower timeouts
./bin/lil-rag config set ollama.timeout-seconds 15
```

### For Quality
```bash
# High-quality embedding model
./bin/lil-rag config set ollama.embedding-model mxbai-embed-large  
./bin/lil-rag config set ollama.vector-size 1024

# High-quality chat model
./bin/lil-rag config set ollama.chat-model qwen2.5:7b

# Larger chunks for more context
./bin/lil-rag config set chunking.max-tokens 512
./bin/lil-rag config set chunking.overlap 76

# Higher timeouts for better results
./bin/lil-rag config set ollama.timeout-seconds 60
```

## Troubleshooting Configuration

### Reset to Defaults
```bash
# Remove current config and reinitialize
rm ~/.lilrag/config.json
./bin/lil-rag config init
```

### Validate Configuration
```bash
# Check if configuration is valid
./bin/lil-rag config show

# Test with health check
./bin/lil-rag health
./bin/lil-rag-server & curl http://localhost:8080/api/health
```

### Common Issues

#### Vector Size Mismatch
```bash
# Check your model's vector size first
ollama show nomic-embed-text | grep -i dimension

# Update vector size to match
./bin/lil-rag config set ollama.vector-size 768
```

#### Connection Issues
```bash  
# Verify Ollama is accessible
curl http://localhost:11434/api/version

# Update endpoint if needed
./bin/lil-rag config set ollama.endpoint http://your-ollama-server:11434
```

#### Permission Issues
```bash
# Check data directory permissions
ls -la ~/.lilrag/

# Fix permissions if needed
chmod -R 755 ~/.lilrag/
```

## Environment Variables

Configuration can also be set via environment variables (mainly for MCP server):

```bash
export LILRAG_DB_PATH="/path/to/database.db"
export LILRAG_DATA_DIR="/path/to/data"
export LILRAG_OLLAMA_URL="http://localhost:11434"
export LILRAG_EMBEDDING_MODEL="nomic-embed-text"
export LILRAG_CHAT_MODEL="gemma3:4b"  
export LILRAG_VISION_MODEL="llama3.2-vision"
export LILRAG_TIMEOUT_SECONDS="30"
export LILRAG_VECTOR_SIZE="768"
```

Environment variables take precedence over configuration file settings.