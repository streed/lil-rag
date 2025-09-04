package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lil-rag/pkg/lilrag"
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

func (h *Handler) Index() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		contentType := r.Header.Get("Content-Type")

		// Handle multipart form data (file uploads)
		if contentType != "" && strings.HasPrefix(contentType, "multipart/form-data") {
			h.handleFileUpload(w, r)
			return
		}

		// Handle JSON requests
		var req IndexRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
			return
		}

		// Generate ID if not provided
		if req.ID == "" {
			req.ID = lilrag.GenerateDocumentID()
		}

		if req.Text == "" {
			h.writeError(w, http.StatusBadRequest, "text is required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()

		if err := h.rag.Index(ctx, req.Text, req.ID); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to index", err.Error())
			return
		}

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

	results, err := h.rag.Search(ctx, query, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "search failed", err.Error())
		return
	}

	response := SearchResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but don't change response at this point
		fmt.Printf("Error encoding response: %v\n", err)
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
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		metrics := map[string]interface{}{
			"status": "metrics not available",
		}

		// Try to get cache stats from embedder if it's an OllamaEmbedder
		if _, ok := interface{}(h.rag).(*lilrag.LilRag); ok {
			// Access the embedder (this would need to be exposed in LilRag)
			metrics["message"] = "Cache statistics available in enhanced embedder"
			metrics["embedding_features"] = []string{"caching", "preprocessing", "query_enhancement", "retry_logic"}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			// Log error but don't change response at this point
			fmt.Printf("Error encoding response: %v\n", err)
		}
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
                width: 100%%;
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

	ctx := context.Background()

	// Generate LLM response using retrieved documents as context
	response, searchResults, err := h.rag.Chat(ctx, req.Message, req.Limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "chat failed", err.Error())
		return
	}

	chatResp := ChatResponse{
		Response: response,
		Sources:  searchResults,
		Query:    req.Message,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(chatResp); err != nil {
		fmt.Printf("Error encoding response: %v\n", err)
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
<html>
<head>
    <title>LilRag API</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        .endpoint { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .method { display: inline-block; padding: 2px 8px; border-radius: 3px; font-weight: bold; }
        .post { background-color: #49cc90; color: white; }
        .get { background-color: #61affe; color: white; }
        pre { background-color: #f5f5f5; padding: 10px; border-radius: 3px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>LilRag API</h1>
    <p>A simple RAG (Retrieval Augmented Generation) API using SQLite and Ollama</p>
    
    <div style="margin: 20px 0; padding: 15px; background: #e8f5e8; border: 1px solid #4caf50; 
         border-radius: 5px; text-align: center;">
        <h3 style="margin: 0 0 10px 0; color: #2e7d32;">💬 Try the Interactive Chat Interface!</h3>
        <p style="margin: 0 0 15px 0;">Ask questions about your documents in a user-friendly chat interface</p>
        <a href="/chat" style="background: #4caf50; color: white; padding: 10px 20px; 
           text-decoration: none; border-radius: 5px; font-weight: bold;">Open Chat Interface</a>
    </div>
    
    <div style="margin: 20px 0; padding: 15px; background: #e3f2fd; border: 1px solid #2196f3; 
         border-radius: 5px; text-align: center;">
        <h3 style="margin: 0 0 10px 0; color: #1565c0;">📚 Browse Your Documents</h3>
        <p style="margin: 0 0 15px 0;">View, manage, and organize all your indexed documents</p>
        <a href="/documents" style="background: #2196f3; color: white; padding: 10px 20px; 
           text-decoration: none; border-radius: 5px; font-weight: bold;">View Documents</a>
    </div>
    
    <div class="endpoint">
        <h3><span class="method post">POST</span> /api/index</h3>
        <p>Index text content with an optional ID (auto-generated if not provided)</p>
        <pre>{"id": "doc1", "text": "Your text content here"}
or
{"text": "Your text content here"}</pre>
    </div>
    
    <div class="endpoint">
        <h3><span class="method post">POST</span> /api/index (File Upload)</h3>
        <p>Upload and index files (text or PDF) with multipart/form-data</p>
        <pre>Form fields:
- id: Document ID (optional - auto-generated if not provided)
- file: File to upload (required)

Content-Type: multipart/form-data</pre>
    </div>
    
    <div class="endpoint">
        <h3><span class="method get">GET</span> /api/search?query=hello&limit=10</h3>
        <p>Search for similar content using query parameters</p>
    </div>
    
    <div class="endpoint">
        <h3><span class="method post">POST</span> /api/search</h3>
        <p>Search for similar content using JSON body</p>
        <pre>{"query": "your search query", "limit": 10}</pre>
    </div>
    
    <div class="endpoint">
        <h3><span class="method get">GET</span> /api/health</h3>
        <p>Health check endpoint</p>
    </div>
    
    <div class="endpoint">
        <h3><span class="method get">GET</span> /api/metrics</h3>
        <p>Performance metrics and cache statistics</p>
    </div>

    <h2>PDF Support</h2>
    <p>PDF files are automatically chunked by page. Search results will show page numbers like 
    <code>[Page 1]</code> to help you locate content within the document.</p>
    
    <h2>Performance Features</h2>
    <p>The system includes several performance optimizations:</p>
    <ul>
        <li><strong>Embedding Cache:</strong> Frequently requested embeddings are cached to reduce API calls</li>
        <li><strong>Text Preprocessing:</strong> Text is normalized and cleaned before embedding</li>
        <li><strong>Query Enhancement:</strong> Search queries are enhanced with context for better results</li>
        <li><strong>Retry Logic:</strong> Automatic retry with exponential backoff for API failures</li>
    </ul>
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
