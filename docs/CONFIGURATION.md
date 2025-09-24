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
    "timeout_seconds": 30,
    "image_max_size": 1120
  },
  "storage_path": "/home/user/.lilrag/data/lilrag.db",
  "data_dir": "/home/user/.lilrag/data",
  "server": {
    "host": "localhost",
    "port": 12121,
    "secure": true,
    "read_timeout": 30,
    "write_timeout": 30,
    "idle_timeout": 120,
    "max_header_bytes": 1048576,
    "enable_cors": false,
    "trusted_proxies": []
  },
  "chunking": {
    "max_chars": 2000,
    "overlap": 200
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
  lil-rag config set ollama.endpoint http://localhost:11434
  
  # Remote Ollama server
  lil-rag config set ollama.endpoint http://192.168.1.100:11434
  
  # Custom port
  lil-rag config set ollama.endpoint http://localhost:11435
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
  lil-rag config set ollama.embedding-model all-MiniLM-L6-v2
  lil-rag config set ollama.vector-size 384
  
  # Use high-quality large model
  lil-rag config set ollama.embedding-model mxbai-embed-large
  lil-rag config set ollama.vector-size 1024
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
  lil-rag config set ollama.chat-model llama3.2:3b
  
  # Use higher quality model
  lil-rag config set ollama.chat-model qwen2.5:7b
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
  lil-rag config set ollama.vision-model llava:7b
  
  # Use specialized model for detailed image analysis
  lil-rag config set ollama.vision-model moondream
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
  lil-rag config set ollama.timeout-seconds 15
  
  # Slow CPU inference
  lil-rag config set ollama.timeout-seconds 120
  
  # Very large models
  lil-rag config set ollama.timeout-seconds 300
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
  lil-rag config set data-dir /path/to/my/rag/data
  
  # Network storage (ensure proper permissions)
  lil-rag config set data-dir /mnt/shared/lilrag
  ```

### Server Configuration (`server`)

Controls HTTP server behavior, authentication, timeouts, and security settings.

#### `host`
- **Type**: String
- **Default**: `"localhost"`
- **Description**: HTTP server bind address
- **Examples**:
  ```bash
  # Listen on all interfaces
  lil-rag config set server.host 0.0.0.0
  
  # Listen on specific interface
  lil-rag config set server.host 192.168.1.100
  ```

#### `port`
- **Type**: Integer  
- **Default**: `12121`
- **Description**: HTTP server port
- **Examples**:
  ```bash
  # Use alternative port
  lil-rag config set server.port 9000
  
  # Use privileged port (requires root/admin)
  lil-rag config set server.port 80
  ```

#### `secure`
- **Type**: Boolean
- **Default**: `true`
- **Description**: Enable authentication and password protection
- **Notes**: 
  - When `true`: Users must log in to access the UI
  - When `false`: Authentication is completely disabled, all endpoints accessible
  - Users are managed via `lil-rag auth add <username> <password>`
- **Examples**:
  ```bash
  # Enable authentication (requires user login)
  lil-rag config set server.secure true
  
  # Disable authentication (open access)
  lil-rag config set server.secure false
  ```

#### `read_timeout`
- **Type**: Integer
- **Default**: `30`
- **Description**: HTTP read timeout in seconds
- **Purpose**: Maximum time to read the entire request, including body
- **Recommendations**:
  - `15-30` for fast networks and small files
  - `60-120` for slow networks or large file uploads
  - `300+` for very large document uploads
- **Examples**:
  ```bash
  # Fast local network
  lil-rag config set server.read-timeout 15
  
  # Large file uploads
  lil-rag config set server.read-timeout 300
  ```

#### `write_timeout`
- **Type**: Integer
- **Default**: `30`
- **Description**: HTTP write timeout in seconds
- **Purpose**: Maximum time to write the response to the client
- **Recommendations**:
  - `15-30` for fast responses and small data
  - `60-120` for slow clients or large responses
  - `300+` for bulk export operations
- **Examples**:
  ```bash
  # Standard web responses
  lil-rag config set server.write-timeout 30
  
  # Large data exports
  lil-rag config set server.write-timeout 300
  ```

#### `idle_timeout`
- **Type**: Integer
- **Default**: `120`
- **Description**: HTTP keep-alive idle timeout in seconds
- **Purpose**: Maximum time to keep connections open between requests
- **Recommendations**:
  - `60-120` for standard web applications
  - `300+` for long-running operations or slow clients
- **Examples**:
  ```bash
  # Standard web app
  lil-rag config set server.idle-timeout 60
  
  # Long-running operations
  lil-rag config set server.idle-timeout 300
  ```

#### `max_header_bytes`
- **Type**: Integer
- **Default**: `1048576` (1 MB)
- **Description**: Maximum size of HTTP headers in bytes
- **Purpose**: Prevents oversized header attacks
- **Recommendations**:
  - `1048576` (1 MB) for standard applications
  - `2097152` (2 MB) for applications with large cookies/tokens
- **Examples**:
  ```bash
  # Standard header size
  lil-rag config set server.max-header-bytes 1048576
  
  # Large authentication tokens
  lil-rag config set server.max-header-bytes 2097152
  ```

#### `enable_cors`
- **Type**: Boolean
- **Default**: `false`
- **Description**: Enable Cross-Origin Resource Sharing (CORS) headers
- **Purpose**: Allow web applications from other domains to access the API
- **When to Enable**:
  - Embedding LilRag in web applications
  - API access from different domains
  - Development with frontend frameworks
- **Examples**:
  ```bash
  # Enable for web app integration
  lil-rag config set server.enable-cors true
  
  # Disable for security (default)
  lil-rag config set server.enable-cors false
  ```

#### `trusted_proxies`
- **Type**: Array of Strings
- **Default**: `[]`
- **Description**: List of trusted proxy IP addresses
- **Purpose**: For deployments behind reverse proxies (nginx, Apache, etc.)
- **Format**: Array of IP addresses or CIDR ranges
- **Examples**:
  ```bash
  # Single proxy IP
  lil-rag config set server.trusted-proxies '["192.168.1.1"]'
  
  # Multiple proxies
  lil-rag config set server.trusted-proxies '["192.168.1.1", "10.0.0.1"]'
  
  # CIDR range
  lil-rag config set server.trusted-proxies '["192.168.0.0/16"]'
  ```

### Chunking Configuration (`chunking`)

Controls how documents are split into searchable chunks.

#### `max_chars`
- **Type**: Integer
- **Default**: `2000`
- **Description**: Maximum characters per chunk
- **Recommendations**:
  - `800-1500`: Precise search results, good for Q&A
  - `2000-4000`: More context per result, good for summarization
  - `6000+`: Very large context blocks for comprehensive results
- **Examples**:
  ```bash
  # Optimize for precise search
  lil-rag config set chunking.max-chars 1000

  # Optimize for context preservation
  lil-rag config set chunking.max-chars 3000

  # Large context chunks
  lil-rag config set chunking.max-chars 6000
  ```

#### `overlap`
- **Type**: Integer
- **Default**: `200` (10% of max_chars)
- **Description**: Character overlap between adjacent chunks
- **Purpose**: Prevents context loss at chunk boundaries
- **Recommendations**: 10-20% of max_chars
- **Examples**:
  ```bash
  # Calculate overlap for different chunk sizes
  # For 1000 chars: 1000 * 0.15 = 150
  lil-rag config set chunking.overlap 150

  # For 2000 chars: 2000 * 0.10 = 200
  lil-rag config set chunking.overlap 200

  # For 3000 chars: 3000 * 0.15 = 450
  lil-rag config set chunking.overlap 450
  ```

## Command Line Overrides

All configuration options can be overridden with command line flags.

### LilRag Client Command Line Options

```bash
# Override database and storage
lil-rag --db /tmp/test.db --data-dir /tmp/data search "query"

# Override Ollama settings
lil-rag --ollama http://remote:11434 --model nomic-embed-text:v1.5 search "query"

# Override embedding and chat models
lil-rag --model mxbai-embed-large --vector-size 1024 --chat-model qwen2.5:7b search "query"
```

### LilRag Server Command Line Options

#### Basic Server Settings
```bash
# Server host and port
lil-rag-server --host 0.0.0.0 --port 9000

# Authentication control
lil-rag-server --secure              # Enable authentication
lil-rag-server --no-secure           # Disable authentication
```

#### HTTP Timeout Settings
```bash
# Configure HTTP timeouts
lil-rag-server --read-timeout 60 --write-timeout 120 --idle-timeout 300

# Large file upload configuration
lil-rag-server --read-timeout 300 --max-header-bytes 2097152
```

#### CORS and Security
```bash
# Enable CORS for web applications
lil-rag-server --enable-cors

# Disable CORS (default)
lil-rag-server --no-cors
```

#### Complete Example
```bash
# Production server with custom settings
lil-rag-server \
  --host 0.0.0.0 \
  --port 8080 \
  --secure \
  --read-timeout 120 \
  --write-timeout 120 \
  --idle-timeout 300 \
  --max-header-bytes 2097152 \
  --enable-cors \
  --chat-model qwen2.5:7b \
  --ollama http://gpu-server:11434
```

### Complete CLI Reference

#### LilRag Client (`lil-rag`)
- `--db <path>` - Database path
- `--data-dir <path>` - Data directory  
- `--ollama <url>` - Ollama server URL
- `--model <name>` - Embedding model
- `--chat-model <name>` - Chat model
- `--vector-size <int>` - Vector dimensions

#### LilRag Server (`lil-rag-server`)
- `--host <address>` - Server bind address
- `--port <number>` - Server port
- `--secure` - Enable authentication
- `--no-secure` - Disable authentication
- `--read-timeout <seconds>` - HTTP read timeout
- `--write-timeout <seconds>` - HTTP write timeout  
- `--idle-timeout <seconds>` - HTTP idle timeout
- `--max-header-bytes <bytes>` - Maximum header size
- `--enable-cors` - Enable CORS headers
- `--no-cors` - Disable CORS headers
- `--db <path>` - Database path override
- `--data-dir <path>` - Data directory override
- `--ollama <url>` - Ollama server URL override
- `--model <name>` - Embedding model override
- `--chat-model <name>` - Chat model override
- `--vector-size <int>` - Vector size override
- `--version` - Show version information

## Authentication System

LilRag includes a built-in authentication system to protect your data and control access to the application.

### Authentication Commands

```bash
# Add a new user
lil-rag auth add <username> <password>

# List all users
lil-rag auth list

# Example: Create admin user
lil-rag auth add admin mySecurePassword123
```

### Authentication Behavior

- **When `server.secure = true`**: 
  - All UI endpoints require authentication
  - Users must log in via the `/login` page
  - Session cookies are HTTP-only and secure
  - Sessions expire after 30 days of inactivity

- **When `server.secure = false`**:
  - Authentication is completely bypassed
  - All endpoints are publicly accessible
  - Useful for development or trusted environments

### Security Features

- **Password Security**: Uses bcrypt with salt for secure password storage
- **Session Management**: Cryptographically secure session tokens
- **HTTP-Only Cookies**: Session tokens not accessible via JavaScript
- **Database Protection**: User credentials and sessions stored in SQLite database
- **CLI-Only User Management**: Users can only be added via command line interface

### Examples

```bash
# Enable authentication and create users
lil-rag config set server.secure true
lil-rag auth add admin AdminPass123
lil-rag auth add user UserPass456

# Disable authentication for development
lil-rag config set server.secure false

# Enable authentication via CLI override
lil-rag-server --secure

# Disable authentication via CLI override
lil-rag-server --no-secure
```

## Configuration Management Commands

```bash
# Initialize default configuration
lil-rag config init

# Show current configuration
lil-rag config show

# Set specific values
lil-rag config set section.key value

# Examples of setting values
lil-rag config set ollama.endpoint http://localhost:11434
lil-rag config set ollama.vision-model llava
lil-rag config set ollama.timeout-seconds 60  
lil-rag config set server.port 12121
lil-rag config set server.secure true
lil-rag config set server.read-timeout 60
lil-rag config set chunking.max-chars 2000
```

## Performance Optimization

### For Speed
```bash
# Fast embedding model
lil-rag config set ollama.embedding-model all-MiniLM-L6-v2
lil-rag config set ollama.vector-size 384

# Fast chat model  
lil-rag config set ollama.chat-model llama3.2:3b

# Small chunks for precise results
lil-rag config set chunking.max-chars 1000
lil-rag config set chunking.overlap 100

# Lower timeouts
lil-rag config set ollama.timeout-seconds 15
```

### For Quality
```bash
# High-quality embedding model
lil-rag config set ollama.embedding-model mxbai-embed-large  
lil-rag config set ollama.vector-size 1024

# High-quality chat model
lil-rag config set ollama.chat-model qwen2.5:7b

# Larger chunks for more context
lil-rag config set chunking.max-chars 3000
lil-rag config set chunking.overlap 300

# Higher timeouts for better results
lil-rag config set ollama.timeout-seconds 60
```

## Troubleshooting Configuration

### Reset to Defaults
```bash
# Remove current config and reinitialize
rm ~/.lilrag/config.json
lil-rag config init
```

### Validate Configuration
```bash
# Check if configuration is valid
lil-rag config show

# Test with health check
lil-rag health
lil-rag-server & curl http://localhost:12121/api/health
```

### Common Issues

#### Vector Size Mismatch
```bash
# Check your model's vector size first
ollama show nomic-embed-text | grep -i dimension

# Update vector size to match
lil-rag config set ollama.vector-size 768
```

#### Connection Issues
```bash  
# Verify Ollama is accessible
curl http://localhost:11434/api/version

# Update endpoint if needed
lil-rag config set ollama.endpoint http://your-ollama-server:11434
```

#### Permission Issues
```bash
# Check data directory permissions
ls -la ~/.lilrag/

# Fix permissions if needed
chmod -R 755 ~/.lilrag/
```

#### Authentication Issues
```bash
# Check authentication status
curl http://localhost:12121/api/auth/status

# Create first user if none exist
lil-rag auth add admin myPassword123

# Disable authentication for debugging
lil-rag config set server.secure false

# List existing users
lil-rag auth list

# Test login functionality
curl -X POST http://localhost:12121/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"myPassword123"}'
```

#### HTTP Timeout Issues
```bash
# Increase timeouts for slow networks
lil-rag config set server.read-timeout 120
lil-rag config set server.write-timeout 120
lil-rag config set server.idle-timeout 300

# Or use CLI overrides for testing
lil-rag-server --read-timeout 300 --write-timeout 300
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