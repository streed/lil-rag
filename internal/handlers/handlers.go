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

        .documents-sidebar {
            width: 300px;
            background: #f8f9fa;
            border-right: 1px solid #e9ecef;
            display: flex;
            flex-direction: column;
        }

        .sidebar-header {
            padding: 15px;
            border-bottom: 1px solid #e9ecef;
            background: #fff;
        }

        .sidebar-header h3 {
            margin: 0;
            color: #2c3e50;
            font-size: 1rem;
        }

        .documents-list {
            flex: 1;
            overflow-y: auto;
            padding: 10px;
        }

        .document-item {
            background: white;
            border: 1px solid #e9ecef;
            border-radius: 8px;
            margin-bottom: 8px;
            padding: 12px;
            cursor: pointer;
            transition: all 0.2s;
        }

        .document-item:hover {
            background: #f0f7ff;
            border-color: #007AFF;
        }

        .document-id {
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 4px;
        }

        .document-preview {
            font-size: 0.85em;
            color: #666;
            line-height: 1.4;
        }

        .document-meta {
            font-size: 0.75em;
            color: #999;
            margin-top: 6px;
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
        }

        .chat-header h1 {
            font-size: 1.5rem;
            margin-bottom: 5px;
        }

        .chat-header p {
            opacity: 0.8;
            font-size: 0.9rem;
        }

        .chat-messages {
            flex: 1;
            padding: 20px;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 15px;
        }

        .message {
            max-width: 80%;
            padding: 12px 18px;
            border-radius: 18px;
            word-wrap: break-word;
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
            width: 100%;
            font-size: 0.9rem;
            border-radius: 12px;
            margin-left: 0;
            margin-right: 0;
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

        .source-text {
            color: #666;
            font-size: 0.85rem;
            line-height: 1.4;
            white-space: pre-wrap;
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
            
            .chat-main {
                flex-direction: column;
            }
            
            .documents-sidebar {
                width: 100%;
                max-height: 200px;
                border-right: none;
                border-bottom: 1px solid #e9ecef;
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
            <h1>🤖 LilRag Chat</h1>
            <p>Ask questions about your indexed documents</p>
        </div>
        
        <div class="chat-main">
            <div class="documents-sidebar">
                <div class="sidebar-header">
                    <h3>📚 Documents</h3>
                </div>
                <div class="documents-list" id="documentsList">
                    <div style="padding: 20px; text-align: center; color: #666;">
                        Loading documents...
                    </div>
                </div>
            </div>
            
            <div class="chat-panel">
                <div class="chat-messages" id="messages">
                    <div class="message system">
                        Welcome! Ask me questions about your indexed documents. " +
                        "I'll search through them and provide relevant answers.
                    </div>
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

        function addMessage(content, type, sources = null) {
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message ' + type;
            
            // Render markdown for assistant messages, keep plain text for user messages
            let html = (type === 'assistant' && typeof marked !== 'undefined') ? marked.parse(content) : content;
            
            // Style document references in square brackets for assistant messages
            if (type === 'assistant') {
                html = html.replace(/\[([a-zA-Z0-9_-]+)\]/g, '<span class="doc-ref">[$1]</span>');
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

        // Documents functionality
        function loadDocuments() {
            fetch('/api/documents')
                .then(response => response.json())
                .then(data => {
                    displayDocuments(data.documents || []);
                })
                .catch(error => {
                    console.error('Error loading documents:', error);
                    document.getElementById('documentsList').innerHTML = 
                        "<div style='padding: 20px; text-align: center; color: #dc3545;'>" +
                        "Failed to load documents</div>";
                });
        }

        function displayDocuments(documents) {
            const container = document.getElementById('documentsList');
            
            if (documents.length === 0) {
                container.innerHTML = "<div style='padding: 20px; text-align: center; color: #666;'>" +
                    "No documents found</div>";
                return;
            }

            container.innerHTML = documents.map(doc => {
                const preview = doc.text.length > 150 ? doc.text.substring(0, 150) + '...' : doc.text;
                const updatedDate = new Date(doc.updated_at).toLocaleDateString();
                
                return '<div class="document-item" onclick="showDocumentDetail(\'' + doc.id + '\')">' +
                    '<div class="document-id">' + doc.id + '</div>' +
                    '<div class="document-preview">' + preview + '</div>' +
                    '<div class="document-meta">Updated: ' + updatedDate + ' • ' + doc.chunk_count + ' chunk(s)</div>' +
                    '</div>';
            }).join('');
        }

        function showDocumentDetail(docId) {
            // Find the document
            fetch('/api/documents')
                .then(response => response.json())
                .then(data => {
                    const doc = data.documents.find(d => d.id === docId);
                    if (doc) {
                        // Add a document message showing the document content
                        const messageDiv = document.createElement('div');
                        messageDiv.className = 'message document';
                        messageDiv.innerHTML = '<strong>📄 Document: ' + doc.id + '</strong><br><br>' + 
                            doc.text.replace(/\n/g, '<br>') +
                            '<br><br><em>Updated: ' + new Date(doc.updated_at).toLocaleString() + '</em>';
                        messagesContainer.appendChild(messageDiv);
                        messagesContainer.scrollTop = messagesContainer.scrollHeight;
                    }
                })
                .catch(error => {
                    console.error('Error loading document:', error);
                });
        }

        // Load documents on page load
        loadDocuments();

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
