package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"lil-rag/pkg/config"
	"lil-rag/pkg/lilrag"
)

// version is set during build time via ldflags
var version = "dev"

type LilRagMCPServer struct {
	rag                *lilrag.LilRag
	defaultSearchLimit int
	defaultChatLimit   int
	maxSearchLimit     int
	maxChatLimit       int
}

// MCP Protocol types
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type MCPInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type MCPInitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Tools struct {
			ListChanged bool `json:"listChanged,omitempty"`
		} `json:"tools,omitempty"`
	} `json:"capabilities"`
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPCallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type MCPCallToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

func main() {
	server, err := NewLilRagMCPServer()
	if err != nil {
		log.Fatalf("Failed to create LilRag MCP server: %v", err)
	}
	defer server.Close()

	// Start MCP protocol handler
	server.HandleMCP()
}

func NewLilRagMCPServer() (*LilRagMCPServer, error) {
	// Try to load configuration from profile first
	profileConfig, err := config.LoadProfile()
	var ragConfig *lilrag.Config

	if err != nil {
		// If profile loading fails, use environment variables or defaults
		ragConfig = &lilrag.Config{
			DatabasePath:      getEnvOrDefault("LILRAG_DB_PATH", "lilrag.db"),
			DataDir:           getEnvOrDefault("LILRAG_DATA_DIR", "data"),
			OllamaURL:         getEnvOrDefault("LILRAG_OLLAMA_URL", "http://localhost:11434"),
			Model:             getEnvOrDefault("LILRAG_MODEL", "nomic-embed-text"),
			VectorSize:        getEnvIntOrDefault("LILRAG_VECTOR_SIZE", 768),
			MaxTokens:         getEnvIntOrDefault("LILRAG_MAX_TOKENS", 200),
			Overlap:           getEnvIntOrDefault("LILRAG_OVERLAP", 50),
			ImageMaxSize:      getEnvIntOrDefault("LILRAG_IMAGE_MAX_SIZE", 1120),
			DefaultLimit:      getEnvIntOrDefault("LILRAG_DEFAULT_LIMIT", 25),
			DefaultChatLimit:  getEnvIntOrDefault("LILRAG_DEFAULT_CHAT_LIMIT", 15),
			TruncateDocuments: getEnvBoolOrDefault("LILRAG_TRUNCATE_DOCUMENTS", false),
			MaxDocumentLength: getEnvIntOrDefault("LILRAG_MAX_DOCUMENT_LENGTH", 0),
		}
	} else {
		// Convert profile config to RAG config
		ragConfig = &lilrag.Config{
			DatabasePath:      profileConfig.StoragePath,
			DataDir:           profileConfig.DataDir,
			OllamaURL:         profileConfig.Ollama.Endpoint,
			Model:             profileConfig.Ollama.EmbeddingModel,
			ChatModel:         profileConfig.Ollama.ChatModel,
			VectorSize:        profileConfig.Ollama.VectorSize,
			MaxTokens:         profileConfig.Chunking.MaxTokens,
			Overlap:           profileConfig.Chunking.Overlap,
			ImageMaxSize:      profileConfig.Ollama.ImageMaxSize,
			DefaultLimit:      profileConfig.Search.DefaultLimit,
			DefaultChatLimit:  profileConfig.Search.DefaultChatLimit,
			TruncateDocuments: profileConfig.Search.TruncateDocuments,
			MaxDocumentLength: profileConfig.Search.MaxDocumentLength,
		}
	}

	// Create and initialize RAG instance
	rag, err := lilrag.New(ragConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LilRag instance: %w", err)
	}

	if err := rag.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize LilRag: %w", err)
	}

	// Determine max limits based on configuration source
	maxSearchLimit := 100 // Default MCP limit
	maxChatLimit := 50    // Default MCP limit

	if profileConfig != nil {
		maxSearchLimit = profileConfig.Search.MaxMCPSearchLimit
		maxChatLimit = profileConfig.Search.MaxMCPChatLimit
	}

	return &LilRagMCPServer{
		rag:                rag,
		defaultSearchLimit: ragConfig.DefaultLimit,
		defaultChatLimit:   ragConfig.DefaultChatLimit,
		maxSearchLimit:     maxSearchLimit,
		maxChatLimit:       maxChatLimit,
	}, nil
}

func (s *LilRagMCPServer) Close() {
	if s.rag != nil {
		_ = s.rag.Close() // Ignore close errors in cleanup
	}
}

func (s *LilRagMCPServer) HandleMCP() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var message MCPMessage
		if err := decoder.Decode(&message); err != nil {
			break // EOF or error, exit
		}

		response := s.processMessage(message)
		if response != nil {
			if err := encoder.Encode(response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return // Exit on encode error
			}
		}
	}
}

func (s *LilRagMCPServer) processMessage(message MCPMessage) *MCPMessage {
	switch message.Method {
	case "initialize":
		return s.handleInitialize(message)
	case "tools/list":
		return s.handleToolsList(message)
	case "tools/call":
		return s.handleToolsCall(message)
	default:
		return &MCPMessage{
			JSONRPC: "2.0",
			ID:      message.ID,
			Error: map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			},
		}
	}
}

func (s *LilRagMCPServer) handleInitialize(message MCPMessage) *MCPMessage {
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: MCPInitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: struct {
				Tools struct {
					ListChanged bool `json:"listChanged,omitempty"`
				} `json:"tools,omitempty"`
			}{
				Tools: struct {
					ListChanged bool `json:"listChanged,omitempty"`
				}{
					ListChanged: false,
				},
			},
			ServerInfo: struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}{
				Name:    "lil-rag",
				Version: version,
			},
		},
	}
}

func (s *LilRagMCPServer) handleToolsList(message MCPMessage) *MCPMessage {
	tools := []MCPTool{
		{
			Name:        "lilrag_index",
			Description: "Index text content into the RAG system for later retrieval",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The text content to index",
					},
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Optional document ID. If not provided, one will be auto-generated",
					},
					"chunking_strategy": map[string]interface{}{
						"type":        "string",
						"description": "Chunking strategy: recursive, semantic, simple (default: recursive)",
						"enum":        []string{"recursive", "semantic", "simple"},
						"default":     "recursive",
					},
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        "lilrag_index_file",
			Description: "Index a text or PDF file into the RAG system",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to index (supports multiple formats: .txt, .pdf, .docx, .xlsx, .html, .csv)",
					},
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Optional document ID. If not provided, filename will be used",
					},
					"chunking_strategy": map[string]interface{}{
						"type":        "string",
						"description": "Chunking strategy: recursive, semantic, simple (default: recursive)",
						"enum":        []string{"recursive", "semantic", "simple"},
						"default":     "recursive",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "lilrag_search",
			Description: "Search for relevant content in the RAG system using semantic similarity",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query to find relevant content",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": fmt.Sprintf("Maximum number of results to return "+
							"(default: %d, max: %d)", s.defaultSearchLimit, s.maxSearchLimit),
						"default":     s.defaultSearchLimit,
					},
					"chunks_only": map[string]interface{}{
						"type":        "boolean",
						"description": "Return only matching chunks without full document context (default: false)",
						"default":     false,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "lilrag_chat",
			Description: "Interactive chat with RAG context - ask questions and get responses with relevant sources",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "The question or message to ask",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": fmt.Sprintf("Maximum number of source documents to use for context "+
							"(default: %d, max: %d)", s.defaultChatLimit, s.maxChatLimit),
						"default":     s.defaultChatLimit,
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional session ID to maintain conversation context",
					},
					"new_session": map[string]interface{}{
						"type":        "boolean",
						"description": "Start a new chat session (default: false)",
						"default":     false,
					},
					"show_sources": map[string]interface{}{
						"type":        "boolean",
						"description": "Display detailed source information with response (default: false)",
						"default":     false,
					},
				},
				"required": []string{"message"},
			},
		},
		{
			Name:        "lilrag_list_documents",
			Description: "List all indexed documents with metadata",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
		{
			Name:        "lilrag_delete_document",
			Description: "Delete a document and all its chunks from the RAG system",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"document_id": map[string]interface{}{
						"type":        "string",
						"description": "The ID of the document to delete",
					},
					"force": map[string]interface{}{
						"type":        "boolean",
						"description": "Skip confirmation prompt (default: false)",
						"default":     false,
					},
				},
				"required": []string{"document_id"},
			},
		},
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *LilRagMCPServer) handleToolsCall(message MCPMessage) *MCPMessage {
	// Parse call parameters
	paramsBytes, err := json.Marshal(message.Params)
	if err != nil {
		return s.errorResponse(message.ID, -32602, "Invalid params")
	}

	var callParams MCPCallToolParams
	if err := json.Unmarshal(paramsBytes, &callParams); err != nil {
		return s.errorResponse(message.ID, -32602, "Invalid params")
	}

	// Call the appropriate tool handler
	switch callParams.Name {
	case "lilrag_index":
		return s.handleIndex(message.ID, callParams.Arguments)
	case "lilrag_index_file":
		return s.handleIndexFile(message.ID, callParams.Arguments)
	case "lilrag_search":
		return s.handleSearch(message.ID, callParams.Arguments)
	case "lilrag_chat":
		return s.handleChat(message.ID, callParams.Arguments)
	case "lilrag_list_documents":
		return s.handleListDocuments(message.ID, callParams.Arguments)
	case "lilrag_delete_document":
		return s.handleDeleteDocument(message.ID, callParams.Arguments)
	default:
		return s.errorResponse(message.ID, -32601, "Tool not found")
	}
}

func (s *LilRagMCPServer) handleIndex(id interface{}, args map[string]interface{}) *MCPMessage {
	// Extract parameters
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return s.errorResponse(id, -32602, "text parameter is required and must be a non-empty string")
	}

	docID, ok := args["id"].(string)
	if !ok || docID == "" {
		docID = lilrag.GenerateDocumentID()
	}

	strategy, ok := args["chunking_strategy"].(string)
	if !ok || strategy == "" {
		strategy = "recursive" // default
	}

	// Index the content with strategy
	ctx := context.Background()
	if err := s.rag.IndexWithStrategy(ctx, text, docID, strategy); err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Failed to index content: %v", err))
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPCallToolResult{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{
				Type: "text",
				Text: fmt.Sprintf("Successfully indexed content with ID: %s using %s chunking", docID, strategy),
			}},
		},
	}
}

func (s *LilRagMCPServer) handleIndexFile(id interface{}, args map[string]interface{}) *MCPMessage {
	// Extract parameters
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return s.errorResponse(id, -32602, "file_path parameter is required and must be a non-empty string")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return s.errorResponse(id, -32602, fmt.Sprintf("File does not exist: %s", filePath))
	}

	docID, ok := args["id"].(string)
	if !ok || docID == "" {
		docID = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}

	strategy, ok := args["chunking_strategy"].(string)
	if !ok || strategy == "" {
		strategy = "recursive" // default
	}

	// Index the file with strategy
	ctx := context.Background()
	if err := s.rag.IndexFileWithStrategy(ctx, filePath, docID, strategy); err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Failed to index file: %v", err))
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPCallToolResult{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{
				Type: "text",
				Text: fmt.Sprintf("Successfully indexed file '%s' with ID: %s using %s chunking", filePath, docID, strategy),
			}},
		},
	}
}

func (s *LilRagMCPServer) handleSearch(id interface{}, args map[string]interface{}) *MCPMessage {
	// Extract parameters
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return s.errorResponse(id, -32602, "query parameter is required and must be a non-empty string")
	}

	// Parse limit (default to configured value)
	limit := s.defaultSearchLimit
	if limitArg, exists := args["limit"]; exists {
		if limitFloat, ok := limitArg.(float64); ok {
			limit = int(limitFloat)
		}
	}

	// Cap limit at configured maximum
	if limit > s.maxSearchLimit {
		limit = s.maxSearchLimit
	}

	// Parse chunks_only flag (default: false)
	chunksOnly := false
	if chunksOnlyArg, exists := args["chunks_only"]; exists {
		if chunksOnlyBool, ok := chunksOnlyArg.(bool); ok {
			chunksOnly = chunksOnlyBool
		}
	}

	// Perform search
	ctx := context.Background()
	results, err := s.rag.Search(ctx, query, limit)
	if err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Search failed: %v", err))
	}

	// Apply chunks-only filtering if flag is set
	if chunksOnly {
		for i := range results {
			if results[i].Metadata == nil {
				results[i].Metadata = make(map[string]interface{})
			}
			results[i].Metadata["chunks_only"] = true
		}
	}

	if len(results) == 0 {
		return &MCPMessage{
			JSONRPC: "2.0",
			ID:      id,
			Result: MCPCallToolResult{
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{{
					Type: "text",
					Text: "No results found for the given query.",
				}},
			},
		}
	}

	// Format results
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Found %d results for query: %q\n\n", len(results), query))

	for i, result := range results {
		response.WriteString(fmt.Sprintf("## Result %d (Score: %.4f)\n", i+1, result.Score))
		response.WriteString(fmt.Sprintf("**Document ID:** %s\n", result.ID))
		response.WriteString(fmt.Sprintf("**Content:**\n%s\n\n", result.Text))
		response.WriteString("---\n\n")
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPCallToolResult{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{
				Type: "text",
				Text: response.String(),
			}},
		},
	}
}

func (s *LilRagMCPServer) errorResponse(id interface{}, code int, message string) *MCPMessage {
	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
}

func (s *LilRagMCPServer) handleChat(id interface{}, args map[string]interface{}) *MCPMessage {
	// Extract parameters
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return s.errorResponse(id, -32602, "message parameter is required and must be a non-empty string")
	}

	// Parse limit (default to configured value)
	limit := s.defaultChatLimit
	if limitArg, exists := args["limit"]; exists {
		if limitFloat, ok := limitArg.(float64); ok {
			limit = int(limitFloat)
		}
	}

	// Cap limit at configured maximum
	if limit > s.maxChatLimit {
		limit = s.maxChatLimit
	}

	// Parse session parameters (for future implementation)
	sessionID := ""
	if sid, ok := args["session_id"].(string); ok {
		sessionID = sid
	}
	newSession := false
	if newSessionArg, exists := args["new_session"]; exists {
		if newSessionBool, ok := newSessionArg.(bool); ok {
			newSession = newSessionBool
		}
	}

	// Parse show_sources flag (default: true for backward compatibility)
	showSources := true
	if showSourcesArg, exists := args["show_sources"]; exists {
		if showSourcesBool, ok := showSourcesArg.(bool); ok {
			showSources = showSourcesBool
		}
	}

	// TODO: Implement session management with sessionID and newSession
	_ = sessionID
	_ = newSession

	// Perform chat
	ctx := context.Background()
	response, sources, err := s.rag.Chat(ctx, message, limit)
	if err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Chat failed: %v", err))
	}

	// Format response
	var fullResponse strings.Builder
	fullResponse.WriteString(fmt.Sprintf("**Response:**\n%s", response))

	// Include sources if requested
	if showSources && len(sources) > 0 {
		fullResponse.WriteString(fmt.Sprintf("\n\n**Sources (%d):**\n", len(sources)))
		for i, source := range sources {
			fullResponse.WriteString(fmt.Sprintf("%d. **%s** (Score: %.4f)\n", i+1, source.ID, source.Score))
			// Truncate source content for readability
			sourceText := source.Text
			if len(sourceText) > 300 {
				sourceText = sourceText[:300] + "..."
			}
			fullResponse.WriteString(fmt.Sprintf("   %s\n\n", sourceText))
		}
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPCallToolResult{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{
				Type: "text",
				Text: fullResponse.String(),
			}},
		},
	}
}

func (s *LilRagMCPServer) handleListDocuments(id interface{}, _ map[string]interface{}) *MCPMessage {
	ctx := context.Background()
	documents, err := s.rag.ListDocuments(ctx)
	if err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Failed to list documents: %v", err))
	}

	if len(documents) == 0 {
		return &MCPMessage{
			JSONRPC: "2.0",
			ID:      id,
			Result: MCPCallToolResult{
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{{
					Type: "text",
					Text: "No documents found in the RAG system.",
				}},
			},
		}
	}

	// Format documents list
	var response strings.Builder
	response.WriteString(fmt.Sprintf("**Indexed Documents (%d):**\n\n", len(documents)))

	for i, doc := range documents {
		response.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, doc.ID))
		response.WriteString(fmt.Sprintf("   - Type: %s\n", doc.DocType))
		response.WriteString(fmt.Sprintf("   - Chunks: %d\n", doc.ChunkCount))
		if doc.SourcePath != "" {
			response.WriteString(fmt.Sprintf("   - Source: %s\n", doc.SourcePath))
		}
		response.WriteString(fmt.Sprintf("   - Created: %s\n", doc.CreatedAt.Format("2006-01-02 15:04:05")))
		response.WriteString(fmt.Sprintf("   - Updated: %s\n\n", doc.UpdatedAt.Format("2006-01-02 15:04:05")))
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPCallToolResult{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{
				Type: "text",
				Text: response.String(),
			}},
		},
	}
}

func (s *LilRagMCPServer) handleDeleteDocument(id interface{}, args map[string]interface{}) *MCPMessage {
	// Extract parameters
	documentID, ok := args["document_id"].(string)
	if !ok || documentID == "" {
		return s.errorResponse(id, -32602, "document_id parameter is required and must be a non-empty string")
	}

	// Parse force flag (for API compatibility - no confirmation needed in MCP)
	force := false
	if forceArg, exists := args["force"]; exists {
		if forceBool, ok := forceArg.(bool); ok {
			force = forceBool
		}
	}
	_ = force // Force parameter acknowledged but not needed for MCP operations

	// Delete the document
	ctx := context.Background()
	if err := s.rag.DeleteDocument(ctx, documentID); err != nil {
		return s.errorResponse(id, -32603, fmt.Sprintf("Failed to delete document: %v", err))
	}

	return &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result: MCPCallToolResult{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{
				Type: "text",
				Text: fmt.Sprintf("Successfully deleted document: %s", documentID),
			}},
		},
	}
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
