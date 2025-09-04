package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lil-rag/pkg/lilrag"
	"lil-rag/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	rag     *lilrag.LilRag
	version string
}

type IndexRequest struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type SearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type ChatRequest struct {
	Message string `json:"message"`
	Limit   int    `json:"limit,omitempty"`
}

type ChatResponse struct {
	Response string                `json:"response"`
	Sources  []lilrag.SearchResult `json:"sources"`
	Query    string                `json:"query"`
}

type SearchResponse struct {
	Results []lilrag.SearchResult `json:"results"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func New(rag *lilrag.LilRag) *Handler {
	return &Handler{rag: rag, version: "dev"}
}

func NewWithVersion(rag *lilrag.LilRag, version string) *Handler {
	return &Handler{rag: rag, version: version}
}

// LoggingMiddleware logs HTTP requests with details and records Prometheus metrics
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer that captures status code
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Log the request
		log.Printf("[%s] %s %s - User-Agent: %s", 
			r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
		
		// Call the next handler
		next.ServeHTTP(wrappedWriter, r)
		
		// Record metrics and log response
		duration := time.Since(start)
		metrics.RecordHTTPRequest(r.Method, r.URL.Path, wrappedWriter.statusCode, duration)
		
		log.Printf("[%s] %s %s - %d - %v", 
			r.Method, r.URL.Path, r.RemoteAddr, wrappedWriter.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (h *Handler) Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		contentType := r.Header.Get("Content-Type")
		log.Printf("Index request - Content-Type: %s", contentType)

		// Handle multipart form data (file uploads)
		if contentType != "" && strings.HasPrefix(contentType, "multipart/form-data") {
			log.Printf("Processing file upload request")
			h.handleFileUpload(w, r)
			return
		}

		// Handle JSON requests
		var req IndexRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("Failed to decode index request: %v", err)
			h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
			return
		}

		// Generate ID if not provided
		if req.ID == "" {
			req.ID = lilrag.GenerateDocumentID()
			log.Printf("Auto-generated document ID: %s", req.ID)
		} else {
			log.Printf("Using provided document ID: %s", req.ID)
		}

		if req.Text == "" {
			h.writeError(w, http.StatusBadRequest, "text is required", "")
			return
		}

		log.Printf("Indexing document %s with %d characters", req.ID, len(req.Text))
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()

		// Record metrics for indexing
		indexStart := time.Now()
		err := h.rag.Index(ctx, req.Text, req.ID)
		indexDuration := time.Since(indexStart)
		
		if err != nil {
			log.Printf("Failed to index document %s: %v", req.ID, err)
			metrics.RecordIndexingRequest(indexDuration, false, len(req.Text))
			h.writeError(w, http.StatusInternalServerError, "failed to index", err.Error())
			return
		}

		metrics.RecordIndexingRequest(indexDuration, true, len(req.Text))
		// Estimate tokens for LLM embedding generation
		metrics.EstimateAndRecordTokens("embedding", "unknown", req.Text)
		
		log.Printf("Successfully indexed document %s", req.ID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"id":      req.ID,
			"message": fmt.Sprintf("Successfully indexed %d characters", len(req.Text)),
		}); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error encoding response: %v\n", err)
		}
	}
}

func (h *Handler) Search() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.handleSearchGET(w, r)
			return
		}

		if r.Method == http.MethodPost {
			h.handleSearchPOST(w, r)
			return
		}

		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
	}
}

func (h *Handler) handleSearchGET(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		h.writeError(w, http.StatusBadRequest, "query parameter is required", "")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	log.Printf("Search GET request - query: '%s', limit: %d", query, limit)

	h.performSearch(w, r, query, limit)
}

func (h *Handler) handleSearchPOST(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	if req.Query == "" {
		h.writeError(w, http.StatusBadRequest, "query is required", "")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	h.performSearch(w, r, req.Query, req.Limit)
}

func (h *Handler) performSearch(w http.ResponseWriter, r *http.Request, query string, limit int) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	log.Printf("Performing search - query: '%s', limit: %d", query, limit)
	searchStart := time.Now()
	results, err := h.rag.Search(ctx, query, limit)
	searchDuration := time.Since(searchStart)
	
	if err != nil {
		log.Printf("Search failed for query '%s': %v", query, err)
		metrics.RecordSearchRequest(searchDuration, false, 0)
		h.writeError(w, http.StatusInternalServerError, "search failed", err.Error())
		return
	}

	metrics.RecordSearchRequest(searchDuration, true, len(results))
	// Estimate tokens for embedding search
	metrics.EstimateAndRecordTokens("search", "unknown", query)
	
	log.Printf("Search completed - found %d results for query '%s'", len(results), query)
	response := SearchResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding search response: %v", err)
	}
}

func (h *Handler) Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"version":   h.version,
		}); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error encoding response: %v\n", err)
		}
	}
}

func (h *Handler) Metrics() http.HandlerFunc {
	// Return the Prometheus metrics handler
	return promhttp.Handler().ServeHTTP
}

// UpdateSystemMetrics updates system-wide metrics like document count
func (h *Handler) UpdateSystemMetrics(ctx context.Context) {
	// Get document count and update metrics
	if documents, err := h.rag.ListDocuments(ctx); err == nil {
		metrics.UpdateDocumentCount(len(documents))
		log.Printf("Updated system metrics - document count: %d", len(documents))
	} else {
		log.Printf("Failed to update document count metric: %v", err)
	}
}

func (h *Handler) Documents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Check if this is a request for a specific document
		path := strings.TrimPrefix(r.URL.Path, "/api/documents")
		if path != "" && path != "/" {
			// Handle individual document request
			documentID := strings.TrimPrefix(path, "/")
			h.serveDocumentText(w, r, documentID)
			return
		}

		// Handle list all documents request
		documents, err := h.rag.ListDocuments(ctx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to list documents", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"documents": documents,
			"count":     len(documents),
		}); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error encoding response: %v\n", err)
		}
	}
}

func (h *Handler) Chat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Serve the chat interface HTML
			h.serveChatInterface(w, r)
		case http.MethodPost:
			// Handle chat message
			h.handleChatMessage(w, r)
		default:
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		}
	}
}

func (h *Handler) serveChatInterface(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	html := `<!DOCTYPE html>
<html>
<head>
    <title>LilRag Chat</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'JetBrainsMono NL Nerd Font Propo', 'JetBrains Mono NL', 'JetBrains Mono', 
                         'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .chat-container {
            width: 95%;
            max-width: 1200px;
            height: 90vh;
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }

        .chat-main {
            display: flex;
            flex: 1;
            overflow: hidden;
        }








        .chat-panel {
            flex: 1;
            display: flex;
            flex-direction: column;
        }

        .chat-header {
            background: #2c3e50;
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 20px 20px 0 0;
            position: relative;
        }

        .chat-header h1 {
            font-size: 1.5rem;
            margin-bottom: 5px;
        }

        .chat-header p {
            opacity: 0.8;
            font-size: 0.9rem;
        }

        .clear-chat-button {
            position: absolute;
            top: 20px;
            right: 20px;
            background: rgba(231, 76, 60, 0.8);
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.8rem;
            transition: background-color 0.2s ease;
        }
        .nav-links {
            position: absolute;
            top: 20px;
            left: 20px;
            display: flex;
            gap: 15px;
        }
        .nav-link {
            background: rgba(52, 152, 219, 0.8);
            color: white;
            text-decoration: none;
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 0.8rem;
            transition: all 0.2s ease;
            font-weight: 500;
        }
        .nav-link:hover {
            background: rgba(52, 152, 219, 1);
            transform: translateY(-1px);
        }

        .clear-chat-button:hover {
            background: rgba(231, 76, 60, 1);
        }

        .chat-messages {
            flex: 1;
            padding: 20px;
            overflow-y: auto;
            overflow-x: hidden;
            display: flex;
            flex-direction: column;
            gap: 15px;
            min-width: 0;
        }

        .message {
            max-width: 80%;
            padding: 12px 18px;
            border-radius: 18px;
            word-wrap: break-word;
            overflow-wrap: break-word;
            hyphens: auto;
            line-height: 1.4;
        }

        .message.user {
            background: #007AFF;
            color: white;
            align-self: flex-end;
            margin-left: auto;
        }

        .message.assistant {
            background: #f1f3f5;
            color: #333;
            align-self: flex-start;
            border: 1px solid #e9ecef;
        }

        .message.assistant h1,
        .message.assistant h2,
        .message.assistant h3,
        .message.assistant h4 {
            margin: 0.5em 0 0.3em 0;
            color: #2c3e50;
        }

        .message.assistant h2 {
            font-size: 1.1em;
            font-weight: 600;
        }

        .message.assistant p {
            margin: 0.5em 0;
            line-height: 1.5;
        }

        .message.assistant strong {
            color: #2c3e50;
            font-weight: 600;
        }

        .message.assistant code {
            background: #e9ecef;
            padding: 2px 4px;
            border-radius: 3px;
            font-family: 'JetBrainsMono NL Nerd Font Propo', 'JetBrains Mono NL', 'JetBrains Mono', 
                         'Fira Code', 'Consolas', 'Monaco', monospace;
            font-size: 0.9em;
        }

        .message.assistant pre {
            background: #f8f9fa;
            border: 1px solid #e9ecef;
            border-radius: 4px;
            padding: 12px;
            margin: 8px 0;
            overflow-x: auto;
            max-width: 100%;
            box-sizing: border-box;
            white-space: pre;
        }

        .message.assistant pre code {
            background: none;
            padding: 0;
        }

        .message.assistant ul,
        .message.assistant ol {
            margin: 0.5em 0;
            padding-left: 1.5em;
            max-width: 100%;
            box-sizing: border-box;
            word-wrap: break-word;
            overflow-wrap: break-word;
        }

        .message.assistant li {
            margin: 0.3em 0;
            word-wrap: break-word;
            overflow-wrap: break-word;
            max-width: 100%;
            box-sizing: border-box;
        }

        .message.assistant li ul,
        .message.assistant li ol {
            margin: 0.3em 0;
            padding-left: 1.2em;
        }

        .message.assistant li p {
            margin: 0.2em 0;
            word-wrap: break-word;
            overflow-wrap: break-word;
        }

        /* Style document references in square brackets */
        .doc-ref {
            color: #888;
            font-weight: 500;
            font-size: 0.9em;
        }

        .doc-ref-link {
            color: #007bff;
            text-decoration: none;
            font-weight: 500;
            font-size: 0.9em;
            padding: 1px 3px;
            border-radius: 3px;
            transition: all 0.2s ease;
        }

        .doc-ref-link:hover {
            background-color: #007bff;
            color: white;
            text-decoration: none;
        }

        .message.system {
            background: #fff3cd;
            color: #856404;
            border: 1px solid #ffeaa7;
            align-self: center;
            font-size: 0.9rem;
        }

        .message.document {
            background: #f8f9fa;
            color: #2c3e50;
            border: 1px solid #e9ecef;
            align-self: flex-start;
            max-width: 100% !important;
            width: 100%%;
            font-size: 0.9rem;
            border-radius: 12px;
            margin-left: 0;
            margin-right: 0;
            word-wrap: break-word;
            overflow-wrap: break-word;
            box-sizing: border-box;
        }

        .message.document strong {
            color: #495057;
        }

        .sources-section {
            margin-top: 12px;
            padding-top: 10px;
            border-top: 1px solid #e0e0e0;
        }

        .sources-header {
            font-size: 0.85rem;
            color: #666;
            margin-bottom: 8px;
            font-weight: 500;
        }

        .sources-compact {
            display: flex;
            flex-wrap: wrap;
            gap: 6px;
            margin-bottom: 10px;
        }

        .source-button {
            background: #f5f5f5;
            border: 1px solid #ddd;
            border-radius: 16px;
            padding: 4px 12px;
            font-size: 0.8rem;
            color: #666;
            cursor: pointer;
            transition: all 0.2s ease;
            font-family: 'JetBrainsMono NL Nerd Font Propo', 'JetBrains Mono NL', 
                         'JetBrains Mono', 'Fira Code', 'Consolas', 'Monaco', monospace;
        }

        .source-button:hover {
            background: #e8e8e8;
            border-color: #bbb;
            color: #444;
        }

        .source-expandable {
            margin: 8px 0;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            background: #fafafa;
        }

        .source-content {
            padding: 12px;
        }

        .source-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 8px;
            padding-bottom: 6px;
            border-bottom: 1px solid #e8e8e8;
            gap: 12px;
        }

        .source-id {
            font-weight: 600;
            color: #555;
            font-size: 0.9rem;
        }

        .source-score {
            font-size: 0.8rem;
            color: #888;
            font-style: italic;
        }

        .view-document-link {
            font-size: 0.75rem;
            color: #007bff;
            text-decoration: none;
            padding: 2px 6px;
            border: 1px solid #007bff;
            border-radius: 3px;
            transition: all 0.2s ease;
        }

        .view-document-link:hover {
            background-color: #007bff;
            color: white;
        }

        .source-text {
            color: #666;
            font-size: 0.85rem;
            line-height: 1.4;
            white-space: pre-wrap;
            word-wrap: break-word;
            overflow-wrap: break-word;
            max-width: 100%;
            box-sizing: border-box;
        }

        .chat-input {
            padding: 20px;
            border-top: 1px solid #e9ecef;
            background: #f8f9fa;
        }

        .input-container {
            display: flex;
            gap: 10px;
            align-items: flex-end;
        }

        .message-input {
            flex: 1;
            padding: 12px 18px;
            border: 1px solid #dee2e6;
            border-radius: 25px;
            font-size: 1rem;
            outline: none;
            resize: none;
            max-height: 120px;
            min-height: 44px;
        }

        .message-input:focus {
            border-color: #007AFF;
            box-shadow: 0 0 0 3px rgba(0, 122, 255, 0.1);
        }

        .send-button {
            background: #007AFF;
            color: white;
            border: none;
            border-radius: 50%;
            width: 44px;
            height: 44px;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: background 0.2s;
        }

        .send-button:hover {
            background: #0056CC;
        }

        .send-button:disabled {
            background: #ccc;
            cursor: not-allowed;
        }

        .typing-indicator {
            display: none;
            align-self: flex-start;
            background: #f1f3f5;
            padding: 12px 18px;
            border-radius: 18px;
            margin-bottom: 15px;
        }

        .typing-dots {
            display: flex;
            gap: 4px;
        }

        .typing-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: #999;
            animation: typing 1.4s infinite ease-in-out;
        }

        .typing-dot:nth-child(1) { animation-delay: -0.32s; }
        .typing-dot:nth-child(2) { animation-delay: -0.16s; }

        @keyframes typing {
            0%, 80%, 100% { transform: scale(0); }
            40% { transform: scale(1); }
        }

        .error-message {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
            padding: 12px;
            border-radius: 8px;
            margin: 10px 0;
        }

        @media (max-width: 768px) {
            .chat-container {
                width: 100%;
                height: 100vh;
                border-radius: 0;
            }
            
            .chat-header {
                border-radius: 0;
            }
            
            
            
            .chat-panel {
                flex: 1;
            }
        }
    </style>
    <script src="https://cdn.jsdelivr.net/npm/marked@12.0.0/marked.min.js"></script>
</head>
<body>
    <div class="chat-container">
        <div class="chat-header">
            <div class="nav-links">
                <a href="/" class="nav-link">← Home</a>
                <a href="/documents" class="nav-link">📚 Documents</a>
            </div>
            <h1>🤖 LilRag Chat</h1>
            <p>Ask questions about your indexed documents</p>
            <button class="clear-chat-button" onclick="clearChatHistory()" title="Clear chat history">🗑️ Clear Chat</button>
        </div>
        
        <div class="chat-main">
            <div class="chat-panel">
                <div class="chat-messages" id="messages">
                </div>
                
                <div class="typing-indicator" id="typing">
                    <div class="typing-dots">
                        <div class="typing-dot"></div>
                        <div class="typing-dot"></div>
                        <div class="typing-dot"></div>
                    </div>
                </div>
                
                <div class="chat-input">
            <div class="input-container">
                <textarea 
                    class="message-input" 
                    id="messageInput" 
                    placeholder="Ask a question about your documents..."
                    rows="1"
                ></textarea>
                <button class="send-button" id="sendButton" onclick="sendMessage()">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/>
                    </svg>
                </button>
                </div>
            </div>
        </div>
    </div>

    <script>
        const messagesContainer = document.getElementById('messages');
        const messageInput = document.getElementById('messageInput');
        const sendButton = document.getElementById('sendButton');
        const typingIndicator = document.getElementById('typing');

        // Auto-resize textarea
        messageInput.addEventListener('input', function() {
            this.style.height = 'auto';
            this.style.height = Math.min(this.scrollHeight, 120) + 'px';
        });

        // Send message on Enter (but allow Shift+Enter for new lines)
        messageInput.addEventListener('keydown', function(e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });

        function addMessage(content, type, sources = null, skipSave = false) {
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + type;
            
            // Render markdown for assistant messages, keep plain text for user messages
            let html = (type === 'assistant' && typeof marked !== 'undefined') ? marked.parse(content) : content;
            
            // Style document references in square brackets for assistant messages
            if (type === 'assistant') {
                // Create clickable links to document viewer with chunk highlighting
                html = html.replace(/\[([a-zA-Z0-9_-]+)\]/g, function(match, docId) {
                    // Find the source with this document ID to get chunk information
                    let chunkParam = '';
                    if (sources) {
                        const source = sources.find(s => s.ID === docId);
                        if (source && source.Metadata && source.Metadata.chunk_index !== undefined) {
                            chunkParam = '?highlight=' + source.Metadata.chunk_index;
                        }
                    }
                    return '<a href="/view/' + docId + chunkParam + '" class="doc-ref-link" target="_blank">[' + docId + ']</a>';
                });
            }
            
            if (sources && sources.length > 0) {
                html += '<div class="sources-section">';
                html += '<div class="sources-header">📚 Sources (' + sources.length + '):</div>';
                html += '<div class="sources-compact">';
                sources.forEach((source, index) => {
                    const sourceId = 'source-' + Date.now() + '-' + index;
                    html += '<button class="source-button" onclick="toggleSource(\'' + sourceId + '\')">';
                    html += source.ID + ' (' + (source.Score * 100).toFixed(1) + '%)';
                    html += '</button>';
                });
                html += '</div>';
                
                sources.forEach((source, index) => {
                    const sourceId = 'source-' + Date.now() + '-' + index;
                    html += '<div id="' + sourceId + '" class="source-expandable" style="display: none;">';
                    html += '<div class="source-content">';
                    html += '<div class="source-header">';
                    html += '<span class="source-id">' + source.ID + '</span>';
                    html += '<span class="source-score">' + (source.Score * 100).toFixed(1) + '% relevance</span>';
                    const chunkParam = source.Metadata && source.Metadata.chunk_index !== undefined ? '?highlight=' + source.Metadata.chunk_index : '';
                    html += '<a href="/view/' + source.ID + chunkParam + '" class="view-document-link" target="_blank">📄 View Document</a>';
                    html += '</div>';
                    html += '<div class="source-text">' + source.Text + '</div>';
                    html += '</div>';
                    html += '</div>';
                });
                html += '</div>';
            }
            
            messageDiv.innerHTML = html;
            messagesContainer.appendChild(messageDiv);
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
            
            // Save to localStorage for persistence
            if (!skipSave) {
                saveChatHistory();
            }
        }

        function showTyping() {
            typingIndicator.style.display = 'block';
            messagesContainer.appendChild(typingIndicator);
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }

        function hideTyping() {
            typingIndicator.style.display = 'none';
        }

        function toggleSource(sourceId) {
            const element = document.getElementById(sourceId);
            if (element) {
                const isVisible = element.style.display !== 'none';
                element.style.display = isVisible ? 'none' : 'block';
            }
        }

        function showError(message) {
            const errorDiv = document.createElement('div');
            errorDiv.className = 'error-message';
            errorDiv.textContent = message;
            messagesContainer.appendChild(errorDiv);
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }

        async function sendMessage() {
            const message = messageInput.value.trim();
            if (!message || sendButton.disabled) return;

            // Add user message
            addMessage(message, 'user');
            
            // Clear input and disable send button
            messageInput.value = '';
            messageInput.style.height = 'auto';
            sendButton.disabled = true;
            
            // Show typing indicator
            showTyping();

            try {
                const response = await fetch('/api/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        message: message,
                        limit: 5
                    })
                });

                hideTyping();

                if (!response.ok) {
                    const errorData = await response.json();
                    throw new Error(errorData.error || 'Request failed');
                }

                const data = await response.json();
                addMessage(data.response, 'assistant', data.sources);
                
            } catch (error) {
                hideTyping();
                console.error('Error:', error);
                showError('Failed to get response: ' + error.message);
            } finally {
                sendButton.disabled = false;
                messageInput.focus();
            }
        }


        // Chat history persistence functions
        function saveChatHistory() {
            const messages = [];
            const messageElements = messagesContainer.querySelectorAll('.message');
            messageElements.forEach(element => {
                if (element.classList.contains('message')) {
                    const type = element.classList.contains('user') ? 'user' : 'assistant';
                    const content = element.textContent || element.innerText;
                    // Store the full HTML for assistant messages to preserve formatting and links
                    const html = type === 'assistant' ? element.innerHTML : content;
                    messages.push({ type, content, html });
                }
            });
            localStorage.setItem('lilrag-chat-history', JSON.stringify(messages));
        }
        
        function loadChatHistory() {
            try {
                const saved = localStorage.getItem('lilrag-chat-history');
                if (saved) {
                    const messages = JSON.parse(saved);
                    messages.forEach(msg => {
                        const messageDiv = document.createElement('div');
                        messageDiv.className = 'message ' + msg.type;
                        // Use saved HTML for assistant messages, plain content for user messages
                        messageDiv.innerHTML = msg.type === 'assistant' ? msg.html : msg.content;
                        messagesContainer.appendChild(messageDiv);
                    });
                    messagesContainer.scrollTop = messagesContainer.scrollHeight;
                }
            } catch (error) {
                console.error('Error loading chat history:', error);
                // Clear corrupted data
                localStorage.removeItem('lilrag-chat-history');
            }
        }
        
        function clearChatHistory() {
            localStorage.removeItem('lilrag-chat-history');
            messagesContainer.innerHTML = '';
        }
        
        // Load chat history on page load
        loadChatHistory();
        
        // Focus input on load
        messageInput.focus();
    </script>
</body>
</html>`

	if _, err := w.Write([]byte(html)); err != nil {
		fmt.Printf("Error writing response: %v\n", err)
	}
}

func (h *Handler) handleChatMessage(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode chat request: %v", err)
		h.writeError(w, http.StatusBadRequest, "invalid JSON", err.Error())
		return
	}

	if req.Message == "" {
		h.writeError(w, http.StatusBadRequest, "message is required", "")
		return
	}

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 5
	} else if req.Limit > 20 {
		req.Limit = 20 // Cap at 20 sources
	}

	log.Printf("Chat request - message: '%s', limit: %d", req.Message, req.Limit)
	ctx := context.Background()

	// Generate LLM response using retrieved documents as context with query optimization
	chatStart := time.Now()
	response, searchResults, err := h.rag.Chat(ctx, req.Message, req.Limit)
	chatDuration := time.Since(chatStart)
	
	if err != nil {
		log.Printf("Chat failed for message '%s': %v", req.Message, err)
		metrics.RecordChatRequest(chatDuration, false, 0, 0)
		h.writeError(w, http.StatusInternalServerError, "chat failed", err.Error())
		return
	}

	chatResp := ChatResponse{
		Response: response,
		Sources:  searchResults,
		Query:    req.Message,
	}

	metrics.RecordChatRequest(chatDuration, true, len(searchResults), len(response))
	// Estimate tokens for chat input and output
	metrics.EstimateAndRecordTokens("chat_input", "unknown", req.Message)
	metrics.EstimateAndRecordTokens("chat_output", "unknown", response)
	
	log.Printf("Chat completed successfully - found %d sources, response length: %d", len(searchResults), len(response))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(chatResp); err != nil {
		log.Printf("Error encoding chat response: %v", err)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *Handler) Static() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lil-RAG - Simple RAG System</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1000px;
            margin: 0 auto;
            padding: 20px;
            background: #f8f9fa;
        }

        .container {
            background: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            padding: 40px;
            margin-bottom: 20px;
        }

        h1 {
            color: #2c3e50;
            border-bottom: 3px solid #007AFF;
            padding-bottom: 10px;
            margin-bottom: 30px;
            text-align: center;
        }

        .subtitle {
            text-align: center;
            color: #666;
            font-size: 1.1em;
            margin-bottom: 40px;
        }

        .quick-actions {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin: 30px 0;
        }

        .action-card {
            padding: 25px;
            border-radius: 10px;
            text-align: center;
            transition: all 0.2s;
            text-decoration: none;
            color: inherit;
            display: block;
        }

        .action-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(0,0,0,0.1);
        }

        .action-card h3 {
            margin: 0 0 10px 0;
            font-size: 1.3em;
        }

        .action-card p {
            margin: 0 0 15px 0;
            color: #666;
        }

        .action-card.chat {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: 2px solid transparent;
        }

        .action-card.chat:hover {
            border: 2px solid #667eea;
        }

        .action-card.chat p {
            color: rgba(255,255,255,0.9);
        }

        .action-card.documents {
            background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
            color: white;
            border: 2px solid transparent;
        }

        .action-card.documents:hover {
            border: 2px solid #f093fb;
        }

        .action-card.documents p {
            color: rgba(255,255,255,0.9);
        }

        .action-card.docs {
            background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
            color: white;
            border: 2px solid transparent;
        }

        .action-card.docs:hover {
            border: 2px solid #4facfe;
        }

        .action-card.docs p {
            color: rgba(255,255,255,0.9);
        }

        .btn {
            display: inline-block;
            background: rgba(255,255,255,0.2);
            color: white;
            padding: 10px 20px;
            border-radius: 25px;
            text-decoration: none;
            font-weight: 500;
            transition: all 0.2s;
            border: 1px solid rgba(255,255,255,0.3);
        }

        .btn:hover {
            background: rgba(255,255,255,0.3);
            transform: scale(1.05);
        }

        .api-section {
            margin-top: 40px;
        }

        .api-section h2 {
            color: #007AFF;
            margin-bottom: 20px;
        }

        .endpoint {
            margin: 20px 0;
            padding: 20px;
            background: #f8f9fa;
            border-left: 4px solid #007AFF;
            border-radius: 0 8px 8px 0;
        }

        .method {
            display: inline-block;
            padding: 4px 10px;
            border-radius: 4px;
            font-weight: bold;
            font-size: 0.9em;
            margin-right: 10px;
        }

        .method.post { background: #28a745; color: white; }
        .method.get { background: #007bff; color: white; }
        .method.delete { background: #dc3545; color: white; }

        pre {
            background: #2d3748;
            color: #e2e8f0;
            padding: 15px;
            border-radius: 8px;
            overflow-x: auto;
            line-height: 1.4;
            margin: 10px 0;
        }

        .features {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 15px;
            margin: 20px 0;
        }

        .feature {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 8px;
            border-left: 4px solid #007AFF;
        }

        .feature h4 {
            margin: 0 0 8px 0;
            color: #007AFF;
        }

        .feature p {
            margin: 0;
            font-size: 0.9em;
            color: #666;
        }

        .footer {
            text-align: center;
            margin-top: 40px;
            padding: 20px;
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Lil-RAG</h1>
        <p class="subtitle">A simple yet powerful RAG (Retrieval Augmented Generation) system built with Go, SQLite, and Ollama</p>
        
        <div class="quick-actions">
            <a href="/chat" class="action-card chat">
                <h3>💬 Interactive Chat</h3>
                <p>Ask questions about your documents in a user-friendly chat interface</p>
                <span class="btn">Start Chatting</span>
            </a>
            
            <a href="/documents" class="action-card documents">
                <h3>📚 Document Library</h3>
                <p>View, manage, and organize all your indexed documents</p>
                <span class="btn">Browse Documents</span>
            </a>
            
            <a href="/docs" class="action-card docs">
                <h3>📖 Documentation</h3>
                <p>Complete API reference and usage guides for all interfaces</p>
                <span class="btn">View Documentation</span>
            </a>
        </div>

        <div class="api-section">
            <h2>🌟 Key Features</h2>
            <div class="features">
                <div class="feature">
                    <h4>🔍 Semantic Search</h4>
                    <p>Advanced similarity search using SQLite with sqlite-vec</p>
                </div>
                <div class="feature">
                    <h4>📄 Multi-Format</h4>
                    <p>Support for PDF, DOCX, XLSX, HTML, CSV, and text files</p>
                </div>
                <div class="feature">
                    <h4>💬 Chat Interface</h4>
                    <p>Interactive chat with RAG context and source citations</p>
                </div>
                <div class="feature">
                    <h4>🗜️ Smart Storage</h4>
                    <p>Automatic compression and deduplication</p>
                </div>
                <div class="feature">
                    <h4>🔧 Multiple APIs</h4>
                    <p>CLI, HTTP REST API, and MCP server interfaces</p>
                </div>
                <div class="feature">
                    <h4>⚡ High Performance</h4>
                    <p>Optimized Go implementation with efficient caching</p>
                </div>
            </div>
        </div>

        <div class="api-section">
            <h2>🌐 API Quick Reference</h2>
            
            <div class="endpoint">
                <h3><span class="method post">POST</span> /api/index</h3>
                <p>Index text content or upload files for processing</p>
                <pre>// JSON
{"id": "doc1", "text": "Your content here"}

// File Upload (multipart/form-data)
curl -F "id=doc2" -F "file=@document.pdf" /api/index</pre>
            </div>
            
            <div class="endpoint">
                <h3><span class="method get">GET</span> <span class="method post">POST</span> /api/search</h3>
                <p>Search for similar content using semantic similarity</p>
                <pre>// GET: /api/search?query=hello&limit=10
// POST: {"query": "your search query", "limit": 10}</pre>
            </div>
            
            <div class="endpoint">
                <h3><span class="method post">POST</span> /api/chat</h3>
                <p>Interactive chat with RAG context and source citations</p>
                <pre>{"message": "What is machine learning?", "limit": 5}</pre>
            </div>
            
            <div class="endpoint">
                <h3><span class="method get">GET</span> /api/documents</h3>
                <p>List all indexed documents with metadata</p>
            </div>
            
            <div class="endpoint">
                <h3><span class="method delete">DELETE</span> /api/documents/{id}</h3>
                <p>Delete a specific document and all its chunks</p>
            </div>
            
            <div class="endpoint">
                <h3><span class="method get">GET</span> /api/health</h3>
                <p>System health check and status</p>
            </div>
        </div>
    </div>

    <div class="footer">
        <p>📚 Lil-RAG v` + h.version + ` | Built with Go, SQLite, and Ollama | <a href="/docs" style="color: #007AFF;">Full Documentation</a></p>
    </div>
</body>
</html>`

		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error writing response: %v\n", err)
		}
	}
}

func (h *Handler) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form with max 50MB
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to parse form", err.Error())
		return
	}

	// Get the ID from form data, generate if not provided
	id := r.FormValue("id")
	if id == "" {
		id = lilrag.GenerateDocumentID()
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "file is required", err.Error())
		return
	}
	defer file.Close()

	// Create temporary file to save uploaded content
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "lilrag_upload_*"+filepath.Ext(header.Filename))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create temp file", err.Error())
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy uploaded file to temporary file
	if _, err := io.Copy(tempFile, file); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save file", err.Error())
		return
	}

	// Close temp file to ensure all data is written
	tempFile.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Check if it's a PDF file
	if isPDFFile(header.Filename) {
		// Index as PDF
		if err := h.rag.IndexPDF(ctx, tempFile.Name(), id); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to index PDF", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"id":       id,
			"type":     "pdf",
			"filename": header.Filename,
			"message":  fmt.Sprintf("Successfully indexed PDF file '%s'", header.Filename),
		}); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error encoding response: %v\n", err)
		}
	} else {
		// Read as text file
		content, err := os.ReadFile(tempFile.Name())
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to read file content", err.Error())
			return
		}

		text := string(content)
		if text == "" {
			h.writeError(w, http.StatusBadRequest, "file content is empty", "")
			return
		}

		if err := h.rag.Index(ctx, text, id); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to index text", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"id":       id,
			"type":     "text",
			"filename": header.Filename,
			"message":  fmt.Sprintf("Successfully indexed %d characters from '%s'", len(text), header.Filename),
		}); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error encoding response: %v\n", err)
		}
	}
}

func isPDFFile(filename string) bool {
	ext := filepath.Ext(filename)
	return ext == ".pdf" || ext == ".PDF"
}

// ViewDocument serves a document for web viewing
func (h *Handler) ViewDocument() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract document ID from URL path
		path := strings.TrimPrefix(r.URL.Path, "/view/")
		documentID := strings.TrimSuffix(path, "/")

		if documentID == "" {
			h.writeError(w, http.StatusBadRequest, "document ID required", "")
			return
		}

		// Get highlight chunk index from query parameter
		highlightChunk := -1
		if highlightParam := r.URL.Query().Get("highlight"); highlightParam != "" {
			if chunkIndex, err := strconv.Atoi(highlightParam); err == nil {
				highlightChunk = chunkIndex
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Get document information
		docInfo, err := h.rag.GetDocumentByID(ctx, documentID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "document not found", err.Error())
			return
		}

		// Serve the document based on its type
		h.serveDocumentContent(w, r, docInfo, highlightChunk)
	}
}

// DocumentContent handles API requests for document content at /api/documents/{id}
func (h *Handler) DocumentContent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract document ID from URL path
		path := strings.TrimPrefix(r.URL.Path, "/api/documents/")
		documentID := strings.TrimSuffix(path, "/")

		if documentID == "" {
			h.writeError(w, http.StatusBadRequest, "document ID required", "")
			return
		}

		h.serveDocumentText(w, r, documentID)
	}
}

// DocumentRouter routes between document content, chunks, and delete requests
func (h *Handler) DocumentRouter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			h.DeleteDocument().ServeHTTP(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/chunks") {
			h.DocumentChunks().ServeHTTP(w, r)
		} else {
			h.DocumentContent().ServeHTTP(w, r)
		}
	}
}

// DocumentChunks handles API requests for document chunks at /api/documents/{id}/chunks
func (h *Handler) DocumentChunks() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract document ID from URL path
		path := strings.TrimPrefix(r.URL.Path, "/api/documents/")
		path = strings.TrimSuffix(path, "/chunks")
		documentID := strings.TrimSuffix(path, "/")
		
		if documentID == "" {
			h.writeError(w, http.StatusBadRequest, "document ID required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Get document chunks
		chunks, err := h.rag.GetDocumentChunks(ctx, documentID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "chunks not found", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(chunks); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to encode chunks", err.Error())
		}
	}
}

// DeleteDocument handles DELETE requests for documents at /api/documents/{id}
func (h *Handler) DeleteDocument() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract document ID from URL path
		path := strings.TrimPrefix(r.URL.Path, "/api/documents/")
		documentID := strings.TrimSuffix(path, "/")
		
		if documentID == "" {
			h.writeError(w, http.StatusBadRequest, "document ID required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Delete the document
		err := h.rag.DeleteDocument(ctx, documentID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				h.writeError(w, http.StatusNotFound, "document not found", err.Error())
			} else {
				h.writeError(w, http.StatusInternalServerError, "failed to delete document", err.Error())
			}
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"status":  "success",
			"message": "Document deleted successfully",
		}
		json.NewEncoder(w).Encode(response)
	}
}

// serveDocumentContent serves the document content in a web viewer
func (h *Handler) serveDocumentContent(w http.ResponseWriter, r *http.Request, docInfo *lilrag.DocumentInfo, highlightChunk int) {
	// For now, serve a simple HTML viewer with the document content
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Document: %s</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: 'JetBrainsMono NL Nerd Font Propo', 'JetBrains Mono NL', 'JetBrains Mono', 
                         'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
            line-height: 1.6;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        
        .document-container {
            max-width: 1200px;
            width: 95%%;
            min-height: 80vh;
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
            overflow: hidden;
            display: flex;
            flex-direction: column;
        }
        .document-header {
            background: white;
            padding: 30px;
            border-bottom: 1px solid #e9ecef;
        }
        .document-title {
            font-size: 1.5em;
            margin: 0 180px 10px 180px;
            color: #333;
            text-align: center;
        }
        .document-meta {
            color: #666;
            font-size: 0.9em;
        }
        .document-content {
            background: white;
            padding: 30px;
            white-space: pre-wrap;
            font-family: inherit;
            flex: 1;
            overflow-y: auto;
            max-height: 70vh;
        }
        .chunk {
            margin-bottom: 24px;
            padding: 20px;
            border-left: 4px solid #e0e0e0;
            background: #fafafa;
            border-radius: 0 8px 8px 0;
            font-size: 1rem;
            line-height: 1.6;
        }
        .highlighted-chunk {
            background: #fff3cd;
            border-left-color: #ffc107;
            box-shadow: 0 0 12px rgba(255, 193, 7, 0.4);
            transform: translateX(4px);
            transition: all 0.3s ease;
        }
        .chunk-text {
            line-height: 1.5;
        }
        .back-to-chat-button {
            position: absolute;
            top: 30px;
            left: 30px;
            background: #007bff;
            color: white;
            text-decoration: none;
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 0.9rem;
            transition: all 0.2s ease;
            font-weight: 500;
        }
        .back-to-chat-button:hover {
            background: #0056b3;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(0, 123, 255, 0.3);
        }
        .document-header {
            position: relative;
        }
        .delete-button {
            position: absolute;
            top: 30px;
            right: 30px;
            background-color: #dc3545;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.9em;
            transition: background-color 0.2s ease;
        }
        .delete-button:hover {
            background-color: #c82333;
        }
        .delete-button:disabled {
            background-color: #6c757d;
            cursor: not-allowed;
        }

        @media (max-width: 768px) {
            .document-title {
                margin: 0 20px 10px 20px;
                text-align: left;
            }
            .back-to-chat-button {
                position: relative;
                top: 0;
                left: 0;
                margin-bottom: 15px;
                display: inline-block;
            }
            .delete-button {
                position: relative;
                top: 0;
                right: 0;
                float: right;
                margin-bottom: 15px;
            }
        }
    </style>
</head>
<body>
    <div class="document-container">
        <div class="document-header">
            <a href="/chat" class="back-to-chat-button">← Back to Chat</a>
            <button class="delete-button" onclick="deleteDocument('%s')" id="deleteBtn">🗑️ Delete</button>
            <h1 class="document-title">📄 %s</h1>
            <div class="document-meta">
                <strong>Type:</strong> %s<br>
                <strong>Chunks:</strong> %d<br>
                <strong>Source:</strong> %s<br>
                <strong>Updated:</strong> %s
            </div>
        </div>
        
        <div class="document-content" id="content">
            Loading document content...
        </div>
    </div>

    <script>
        const highlightChunk = %d;
        
        // Load document chunks for highlighting
        fetch('/api/documents/' + '%s' + '/chunks')
            .then(response => response.json())
            .then(chunks => {
                const contentDiv = document.getElementById('content');
                contentDiv.innerHTML = '';
                
                chunks.forEach((chunk, index) => {
                    const chunkDiv = document.createElement('div');
                    chunkDiv.className = 'chunk';
                    if (index === highlightChunk) {
                        chunkDiv.className += ' highlighted-chunk';
                    }
                    chunkDiv.innerHTML = '<div class="chunk-text">' + chunk.Text.replace(/\n/g, '<br>') + '</div>';
                    contentDiv.appendChild(chunkDiv);
                });
            })
            .catch(error => {
                // Fallback to regular content loading
                fetch('/api/documents/' + '%s')
                    .then(response => response.text())
                    .then(content => {
                        document.getElementById('content').textContent = content;
                    });
            });

        function deleteDocument(documentId) {
            if (!confirm('Are you sure you want to delete this document? This action cannot be undone.')) {
                return;
            }
            
            const deleteBtn = document.getElementById('deleteBtn');
            deleteBtn.disabled = true;
            deleteBtn.textContent = '⏳ Deleting...';
            
            fetch('/api/documents/' + documentId, {
                method: 'DELETE'
            })
            .then(response => {
                if (response.ok) {
                    alert('Document deleted successfully!');
                    window.location.href = '/chat';
                } else {
                    return response.json().then(error => {
                        throw new Error(error.message || 'Failed to delete document');
                    });
                }
            })
            .catch(error => {
                alert('Error deleting document: ' + error.message);
                deleteBtn.disabled = false;
                deleteBtn.textContent = '🗑️ Delete';
            });
        }
    </script>
</body>
</html>`,
		docInfo.ID,                                             // title
		docInfo.ID,                                             // delete button  
		docInfo.ID,                                             // document title
		docInfo.DocType,                                        // type
		docInfo.ChunkCount,                                     // chunks
		docInfo.SourcePath,                                     // source
		docInfo.UpdatedAt.Format("2006-01-02 15:04:05"),      // updated
		highlightChunk,                                         // highlightChunk JS variable
		docInfo.ID,                                             // fetch chunks URL
		docInfo.ID,                                             // fetch document URL
	)

	w.Write([]byte(html))
}

// serveDocumentText serves the raw text content of a document
func (h *Handler) serveDocumentText(w http.ResponseWriter, r *http.Request, documentID string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get document information
	docInfo, err := h.rag.GetDocumentByID(ctx, documentID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "document not found", err.Error())
		return
	}

	// For now, we'll serve the document content from the source file if available
	// In the future, we could reconstruct it from the compressed storage
	if docInfo.SourcePath != "" {
		// Try to parse the original file using document handlers
		content, err := h.rag.ParseDocumentFile(docInfo.SourcePath)
		if err == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(content))
			return
		}

		// Fallback to reading the raw file
		rawContent, err := os.ReadFile(docInfo.SourcePath)
		if err == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write(rawContent)
			return
		}
	}

	// If we can't read the source file, return an error
	h.writeError(w, http.StatusNotFound, "document content not available",
		"Original source file not found or not readable")
}

// DocumentsList serves a web page with a table view of all documents
func (h *Handler) DocumentsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		html := `<!DOCTYPE html>
<html>
<head>
    <title>Documents - LilRag</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: 'JetBrainsMono NL Nerd Font Propo', 'JetBrains Mono NL', 'JetBrains Mono', 
                         'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;
            line-height: 1.6;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        
        .documents-container {
            max-width: 1400px;
            width: 95%%;
            min-height: 80vh;
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
            overflow: hidden;
            display: flex;
            flex-direction: column;
        }
        
        .documents-header {
            background: white;
            padding: 30px;
            border-bottom: 1px solid #e9ecef;
            position: relative;
        }
        
        .documents-title {
            font-size: 1.8em;
            margin: 0 0 10px 0;
            color: #333;
            text-align: center;
        }
        
        .nav-buttons {
            position: absolute;
            top: 30px;
            left: 30px;
            display: flex;
            gap: 10px;
        }
        
        .nav-button {
            background: #007bff;
            color: white;
            text-decoration: none;
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 0.9rem;
            transition: all 0.2s ease;
            font-weight: 500;
        }
        
        .nav-button:hover {
            background: #0056b3;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(0, 123, 255, 0.3);
        }
        
        .documents-content {
            background: white;
            padding: 30px;
            flex: 1;
            overflow-y: auto;
        }
        
        .loading {
            text-align: center;
            color: #666;
            font-size: 1.1em;
            padding: 40px;
        }
        
        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 20px;
            border-radius: 8px;
            margin: 20px 0;
        }
        
        .documents-table {
            width: 100%%;
            border-collapse: collapse;
            background: white;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        
        .documents-table th {
            background: #f8f9fa;
            padding: 15px 12px;
            text-align: left;
            font-weight: 600;
            color: #333;
            border-bottom: 2px solid #dee2e6;
        }
        
        .documents-table td {
            padding: 12px;
            border-bottom: 1px solid #dee2e6;
            vertical-align: top;
        }
        
        .documents-table tr:hover {
            background: #f8f9fa;
        }
        
        .doc-id {
            font-family: 'Courier New', monospace;
            color: #007bff;
            font-weight: 500;
        }
        
        .doc-title {
            font-weight: 500;
            color: #333;
        }
        
        .doc-time {
            color: #666;
            font-size: 0.9em;
        }
        
        .action-button {
            padding: 6px 12px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            text-decoration: none;
            display: inline-block;
            font-size: 0.85em;
            font-weight: 500;
            transition: all 0.2s ease;
        }
        
        .view-button {
            background: #28a745;
            color: white;
            margin-right: 8px;
        }
        
        .view-button:hover {
            background: #218838;
            transform: translateY(-1px);
        }
        
        .delete-button {
            background: #dc3545;
            color: white;
        }
        
        .delete-button:hover {
            background: #c82333;
            transform: translateY(-1px);
        }
        
        .delete-button:disabled {
            background: #6c757d;
            cursor: not-allowed;
        }
        
        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: #666;
        }
        
        .empty-state h3 {
            margin: 0 0 15px 0;
            color: #333;
        }
        
        @media (max-width: 768px) {
            .documents-container {
                margin: 10px;
                width: calc(100%% - 20px);
                min-height: calc(100vh - 20px);
            }
            
            .nav-buttons {
                position: relative;
                top: 0;
                left: 0;
                margin-bottom: 20px;
            }
            
            .documents-title {
                text-align: left;
            }
            
            .documents-table {
                font-size: 0.9em;
            }
            
            .documents-table th,
            .documents-table td {
                padding: 8px 6px;
            }
        }
    </style>
</head>
<body>
    <div class="documents-container">
        <div class="documents-header">
            <div class="nav-buttons">
                <a href="/" class="nav-button">← Home</a>
                <a href="/chat" class="nav-button">💬 Chat</a>
            </div>
            <h1 class="documents-title">📚 Documents</h1>
        </div>
        
        <div class="documents-content">
            <div class="loading" id="loading">Loading documents...</div>
            <div class="error" id="error" style="display: none;"></div>
            <div id="documents-container" style="display: none;">
                <table class="documents-table">
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Title</th>
                            <th>Type</th>
                            <th>Chunks</th>
                            <th>Indexed</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="documents-body">
                    </tbody>
                </table>
            </div>
            <div class="empty-state" id="empty-state" style="display: none;">
                <h3>No documents found</h3>
                <p>Upload your first document to get started.</p>
            </div>
        </div>
    </div>

    <script>
        async function loadDocuments() {
            try {
                const response = await fetch('/api/documents');
                if (!response.ok) {
                    throw new Error('Failed to fetch documents');
                }
                
                const data = await response.json();
                displayDocuments(data.documents || data);
            } catch (error) {
                showError('Failed to load documents: ' + error.message);
            }
        }
        
        function displayDocuments(documents) {
            const loading = document.getElementById('loading');
            const errorDiv = document.getElementById('error');
            const container = document.getElementById('documents-container');
            const emptyState = document.getElementById('empty-state');
            const tbody = document.getElementById('documents-body');
            
            loading.style.display = 'none';
            errorDiv.style.display = 'none';
            
            if (!documents || documents.length === 0) {
                emptyState.style.display = 'block';
                return;
            }
            
            container.style.display = 'block';
            tbody.innerHTML = '';
            
            documents.forEach(doc => {
                const row = document.createElement('tr');
                row.innerHTML = ` + "`" + `
                    <td><span class="doc-id">${escapeHtml(doc.id)}</span></td>
                    <td><span class="doc-title">${escapeHtml(doc.id)}</span></td>
                    <td>${escapeHtml(doc.doc_type || 'text')}</td>
                    <td>${doc.chunk_count || 0}</td>
                    <td><span class="doc-time">${formatDate(doc.created_at)}</span></td>
                    <td>
                        <a href="/view/${escapeHtml(doc.id)}" class="action-button view-button">View</a>
                        <button class="action-button delete-button" onclick="deleteDocument('${escapeHtml(doc.id)}')">Delete</button>
                    </td>
                ` + "`" + `;
                tbody.appendChild(row);
            });
        }
        
        function showError(message) {
            const loading = document.getElementById('loading');
            const errorDiv = document.getElementById('error');
            
            loading.style.display = 'none';
            errorDiv.textContent = message;
            errorDiv.style.display = 'block';
        }
        
        function escapeHtml(unsafe) {
            return unsafe
                .replace(/&/g, "&amp;")
                .replace(/</g, "&lt;")
                .replace(/>/g, "&gt;")
                .replace(/"/g, "&quot;")
                .replace(/'/g, "&#039;");
        }
        
        function formatDate(dateString) {
            if (!dateString) return 'Unknown';
            const date = new Date(dateString);
            return date.toLocaleString();
        }
        
        async function deleteDocument(docId) {
            if (!confirm('Are you sure you want to delete this document? This action cannot be undone.')) {
                return;
            }
            
            const button = event.target;
            button.disabled = true;
            button.textContent = 'Deleting...';
            
            try {
                const response = await fetch('/api/documents/' + encodeURIComponent(docId), {
                    method: 'DELETE'
                });
                
                if (!response.ok) {
                    throw new Error('Failed to delete document');
                }
                
                // Reload the documents list
                loadDocuments();
            } catch (error) {
                alert('Failed to delete document: ' + error.message);
                button.disabled = false;
                button.textContent = 'Delete';
            }
        }
        
        // Load documents when page loads
        loadDocuments();
    </script>
</body>
</html>`

		w.Write([]byte(html))
	}
}

func (h *Handler) Documentation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is allowed")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lil-RAG Documentation</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f8f9fa;
        }

        .container {
            background: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            padding: 40px;
            margin-bottom: 20px;
        }

        h1, h2, h3 {
            color: #2c3e50;
            margin-top: 0;
        }

        h1 {
            border-bottom: 3px solid #007AFF;
            padding-bottom: 10px;
            margin-bottom: 30px;
        }

        h2 {
            color: #007AFF;
            margin-top: 30px;
            margin-bottom: 15px;
        }

        .nav {
            background: #f1f3f5;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
        }

        .nav ul {
            list-style: none;
            padding: 0;
            margin: 0;
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 10px;
        }

        .nav li {
            margin: 0;
        }

        .nav a {
            color: #007AFF;
            text-decoration: none;
            font-weight: 500;
            display: block;
            padding: 8px 12px;
            border-radius: 4px;
            transition: background-color 0.2s;
        }

        .nav a:hover {
            background: #e3f2fd;
        }

        .endpoint {
            background: #f8f9fa;
            border-left: 4px solid #007AFF;
            margin: 20px 0;
            padding: 15px;
            border-radius: 0 8px 8px 0;
        }

        .method {
            display: inline-block;
            padding: 4px 8px;
            border-radius: 4px;
            font-weight: bold;
            font-size: 0.9em;
            margin-right: 10px;
        }

        .method.post { background: #28a745; color: white; }
        .method.get { background: #007bff; color: white; }
        .method.delete { background: #dc3545; color: white; }

        pre {
            background: #2d3748;
            color: #e2e8f0;
            padding: 15px;
            border-radius: 8px;
            overflow-x: auto;
            line-height: 1.4;
        }

        code {
            background: #e3f2fd;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
            color: #1976d2;
        }

        pre code {
            background: none;
            padding: 0;
            color: inherit;
        }

        .feature {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 8px;
            margin: 15px 0;
        }

        .feature h3 {
            color: white;
            margin-top: 0;
        }

        .interface-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }

        .interface-card {
            background: white;
            border: 2px solid #e9ecef;
            border-radius: 8px;
            padding: 20px;
            transition: all 0.2s;
        }

        .interface-card:hover {
            border-color: #007AFF;
            box-shadow: 0 4px 12px rgba(0,122,255,0.15);
        }

        .interface-card h3 {
            color: #007AFF;
            margin-top: 0;
        }

        .command-list {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 8px;
            margin: 10px 0;
        }

        .command-list ul {
            margin: 0;
            padding-left: 20px;
        }

        .badge {
            display: inline-block;
            padding: 2px 8px;
            background: #007AFF;
            color: white;
            border-radius: 12px;
            font-size: 0.8em;
            font-weight: 500;
            margin-left: 8px;
        }

        .footer {
            text-align: center;
            margin-top: 40px;
            padding: 20px;
            color: #666;
            font-size: 0.9em;
        }

        .back-link {
            display: inline-block;
            margin-bottom: 20px;
            color: #007AFF;
            text-decoration: none;
            font-weight: 500;
        }

        .back-link:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back-link">← Back to Home</a>
        
        <h1>🚀 Lil-RAG Documentation</h1>
        
        <p>Complete documentation for Lil-RAG - A simple yet powerful RAG (Retrieval Augmented Generation) system built with Go, SQLite, and Ollama.</p>

        <div class="nav">
            <ul>
                <li><a href="#features">✨ Features</a></li>
                <li><a href="#interfaces">🔗 Interfaces</a></li>
                <li><a href="#http-api">🌐 HTTP API</a></li>
                <li><a href="#cli">💻 CLI Usage</a></li>
                <li><a href="#mcp">🔌 MCP Server</a></li>
                <li><a href="#config">⚙️ Configuration</a></li>
            </ul>
        </div>

        <section id="features">
            <h2>✨ Features</h2>
            
            <div class="interface-grid">
                <div class="feature">
                    <h3>🔍 Semantic Vector Search</h3>
                    <p>Advanced similarity search using SQLite with sqlite-vec extension for fast, accurate retrieval.</p>
                </div>
                
                <div class="feature">
                    <h3>📄 Multi-Format Support</h3>
                    <p>Native support for PDF, DOCX, XLSX, HTML, CSV, and text files with intelligent parsing.</p>
                </div>
                
                <div class="feature">
                    <h3>💬 Interactive Chat</h3>
                    <p>Chat functionality with RAG context, providing responses with relevant source citations.</p>
                </div>
                
                <div class="feature">
                    <h3>🗜️ Smart Compression</h3>
                    <p>Automatic gzip compression and deduplication for optimal storage efficiency.</p>
                </div>
            </div>
        </section>

        <section id="interfaces">
            <h2>🔗 Available Interfaces</h2>
            
            <div class="interface-grid">
                <div class="interface-card">
                    <h3>💻 Command Line Interface</h3>
                    <p>Full-featured CLI with commands for indexing, searching, chatting, and document management.</p>
                    <div class="command-list">
                        <strong>Commands:</strong>
                        <ul>
                            <li><code>index</code> - Index documents</li>
                            <li><code>search</code> - Search content</li>
                            <li><code>chat</code> - Interactive chat</li>
                            <li><code>documents</code> - List documents</li>
                            <li><code>delete</code> - Remove documents</li>
                            <li><code>health</code> - System status</li>
                        </ul>
                    </div>
                </div>

                <div class="interface-card">
                    <h3>🌐 HTTP API Server</h3>
                    <p>RESTful API with web interface for integration and interactive usage.</p>
                    <div class="command-list">
                        <strong>Endpoints:</strong>
                        <ul>
                            <li><code>/api/index</code> - Index content</li>
                            <li><code>/api/search</code> - Search documents</li>
                            <li><code>/api/chat</code> - Chat interface</li>
                            <li><code>/api/documents</code> - Document management</li>
                            <li><code>/api/health</code> - Health check</li>
                        </ul>
                    </div>
                </div>

                <div class="interface-card">
                    <h3>🔌 MCP Server</h3>
                    <p>Model Context Protocol server for integration with AI assistants and tools.</p>
                    <div class="command-list">
                        <strong>Tools:</strong>
                        <ul>
                            <li><code>lilrag_index</code> - Index content</li>
                            <li><code>lilrag_search</code> - Search documents</li>
                            <li><code>lilrag_chat</code> - Chat with context</li>
                            <li><code>lilrag_list_documents</code> - List documents</li>
                            <li><code>lilrag_delete_document</code> - Delete documents</li>
                        </ul>
                    </div>
                </div>
            </div>
        </section>

        <section id="http-api">
            <h2>🌐 HTTP API Reference</h2>
            
            <div class="endpoint">
                <h3><span class="method post">POST</span> /api/index</h3>
                <p>Index text content or upload files for processing.</p>
                <pre><code>curl -X POST http://localhost:8080/api/index \\
  -H "Content-Type: application/json" \\
  -d '{"id": "doc1", "text": "Your content here"}'

# File upload
curl -X POST http://localhost:8080/api/index \\
  -F "id=doc2" \\
  -F "file=@document.pdf"</code></pre>
            </div>

            <div class="endpoint">
                <h3><span class="method get">GET</span> <span class="method post">POST</span> /api/search</h3>
                <p>Search for similar content using semantic similarity.</p>
                <pre><code># GET request
curl "http://localhost:8080/api/search?query=machine%20learning&limit=5"

# POST request  
curl -X POST http://localhost:8080/api/search \\
  -H "Content-Type: application/json" \\
  -d '{"query": "artificial intelligence", "limit": 3}'</code></pre>
            </div>

            <div class="endpoint">
                <h3><span class="method post">POST</span> /api/chat</h3>
                <p>Interactive chat with RAG context and source citations.</p>
                <pre><code>curl -X POST http://localhost:8080/api/chat \\
  -H "Content-Type: application/json" \\
  -d '{"message": "What is machine learning?", "limit": 5}'</code></pre>
            </div>

            <div class="endpoint">
                <h3><span class="method get">GET</span> /api/documents</h3>
                <p>List all indexed documents with metadata.</p>
                <pre><code>curl http://localhost:8080/api/documents</code></pre>
            </div>

            <div class="endpoint">
                <h3><span class="method delete">DELETE</span> /api/documents/{id}</h3>
                <p>Delete a specific document and all its chunks.</p>
                <pre><code>curl -X DELETE http://localhost:8080/api/documents/doc1</code></pre>
            </div>

            <div class="endpoint">
                <h3><span class="method get">GET</span> /api/health</h3>
                <p>System health check endpoint.</p>
                <pre><code>curl http://localhost:8080/api/health</code></pre>
            </div>
        </section>

        <section id="cli">
            <h2>💻 CLI Usage</h2>
            
            <h3>Installation & Setup</h3>
            <pre><code># Build from source
make build

# Initialize configuration
./bin/lil-rag config init

# Check configuration
./bin/lil-rag config show</code></pre>

            <h3>Document Management</h3>
            <pre><code># Index text content (ID auto-generated)
./bin/lil-rag index "Your content here"

# Index with explicit ID
./bin/lil-rag index doc1 "Your content here"

# Index a PDF file (ID auto-generated)  
./bin/lil-rag index document.pdf

# Index a PDF file with explicit ID
./bin/lil-rag index doc2 document.pdf

# Index from stdin (ID auto-generated)
echo "Content" | ./bin/lil-rag index -

# Index from stdin with explicit ID
echo "Content" | ./bin/lil-rag index doc3 -

# List all documents
./bin/lil-rag documents

# Delete a document
./bin/lil-rag delete doc1 --force</code></pre>

            <h3>Search & Chat</h3>
            <pre><code># Search for content
./bin/lil-rag search "machine learning" 5

# Interactive chat
./bin/lil-rag chat "What is AI?" 3

# Check system health
./bin/lil-rag health</code></pre>

            <h3>Configuration</h3>
            <pre><code># Set Ollama endpoint
./bin/lil-rag config set ollama.endpoint http://localhost:11434

# Set embedding model
./bin/lil-rag config set ollama.model nomic-embed-text

# Set chat model
./bin/lil-rag config set ollama.chat-model llama3.2</code></pre>
        </section>

        <section id="mcp">
            <h2>🔌 MCP Server</h2>
            
            <p>The MCP server provides tools for AI assistants to interact with your RAG system.</p>

            <h3>Available Tools</h3>
            <div class="command-list">
                <ul>
                    <li><strong>lilrag_index</strong> - Index text content</li>
                    <li><strong>lilrag_index_file</strong> - Index files (PDF, DOCX, etc.)</li>
                    <li><strong>lilrag_search</strong> - Semantic search</li>
                    <li><strong>lilrag_chat</strong> - Chat with RAG context</li>
                    <li><strong>lilrag_list_documents</strong> - List all documents</li>
                    <li><strong>lilrag_delete_document</strong> - Delete documents</li>
                </ul>
            </div>

            <h3>Configuration</h3>
            <p>The MCP server uses the same profile configuration as the CLI and HTTP server, or falls back to environment variables:</p>
            <pre><code>LILRAG_DB_PATH=/path/to/database.db
LILRAG_OLLAMA_URL=http://localhost:11434
LILRAG_MODEL=nomic-embed-text</code></pre>
        </section>

        <section id="config">
            <h2>⚙️ Configuration</h2>
            
            <p>Lil-RAG uses profile-based configuration stored in <code>~/.lilrag/config.json</code>.</p>

            <h3>Configuration Keys</h3>
            <div class="command-list">
                <ul>
                    <li><code>ollama.endpoint</code> - Ollama server URL</li>
                    <li><code>ollama.model</code> - Embedding model name</li>
                    <li><code>ollama.chat-model</code> - Chat model name</li>
                    <li><code>ollama.vector-size</code> - Vector dimension size</li>
                    <li><code>storage.path</code> - Database file path</li>
                    <li><code>data.dir</code> - Data directory path</li>
                    <li><code>server.host</code> - HTTP server host</li>
                    <li><code>server.port</code> - HTTP server port</li>
                    <li><code>chunking.max-tokens</code> - Max tokens per chunk</li>
                    <li><code>chunking.overlap</code> - Token overlap between chunks</li>
                </ul>
            </div>

            <h3>Example Configuration</h3>
            <pre><code>{
  "ollama": {
    "endpoint": "http://localhost:11434",
    "embedding_model": "nomic-embed-text",
    "chat_model": "llama3.2",
    "vector_size": 768
  },
  "storage_path": "~/.lilrag/data/lilrag.db",
  "data_dir": "~/.lilrag/data",
  "server": {
    "host": "localhost",
    "port": 8080
  },
  "chunking": {
    "max_tokens": 200,
    "overlap": 50
  }
}</code></pre>
        </section>
    </div>

    <div class="footer">
        <p>📚 Lil-RAG v` + h.version + ` | <a href="https://github.com/your-username/lil-rag" style="color: #007AFF;">GitHub Repository</a></p>
    </div>
</body>
</html>`

		if _, err := w.Write([]byte(html)); err != nil {
			h.writeError(w, http.StatusInternalServerError, "write_error", "Failed to write response")
			return
		}
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := ErrorResponse{
		Error:   errType,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but don't change response at this point
		fmt.Printf("Error encoding error response: %v\n", err)
	}
}
