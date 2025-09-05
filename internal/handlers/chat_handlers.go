package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"lil-rag/internal/theme"
	"lil-rag/pkg/metrics"
)

// Chat handles both chat interface requests (GET) and chat API requests (POST) at /chat and /api/chat
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

// serveChatInterface serves the chat web interface
func (h *Handler) serveChatInterface(w http.ResponseWriter, r *http.Request) {
	// Use new template system if available
	if h.renderer != nil {
		data := &theme.TemplateData{
			Title:       "Chat",
			Version:     h.version,
			PageName:    "chat",
			PageContent: "chat.html",
		}

		w.Header().Set("Content-Type", "text/html")
		if err := h.renderer.RenderPage(w, "base.html", data); err != nil {
			log.Printf("Template rendering error: %v", err)
			h.fallbackChatPage(w, r)
		}
		return
	}

	// Fallback to original HTML
	h.fallbackChatPage(w, r)
}

// fallbackChatPage serves the original chat page HTML as fallback
func (h *Handler) fallbackChatPage(w http.ResponseWriter, _ *http.Request) {
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
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1000px;
            margin: 0 auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }

        .chat-container {
            background: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            padding: 40px;
            margin-bottom: 20px;
            height: 80vh;
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
            background: transparent;
            color: #333;
            padding: 20px 0;
            text-align: center;
            border-radius: 0;
            position: relative;
        }

        .chat-header h1 {
            font-size: 1.5rem;
            margin-bottom: 5px;
            color: #2c3e50;
        }

        .chat-header p {
            opacity: 0.8;
            font-size: 0.9rem;
            color: #666;
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
            width: 100%;
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
        
        .nav-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
            background: rgba(255,255,255,0.1);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 15px 25px;
            box-shadow: 0 4px 15px rgba(0,0,0,0.1);
        }

        .nav-links {
            display: flex;
            gap: 15px;
        }

        .nav-link {
            color: white;
            text-decoration: none;
            padding: 8px 16px;
            border-radius: 8px;
            font-weight: 500;
            transition: all 0.2s ease;
            background: rgba(255,255,255,0.1);
            border: 1px solid rgba(255,255,255,0.2);
        }

        .nav-link:hover {
            background: rgba(255,255,255,0.2);
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(255,255,255,0.2);
        }

        .nav-link.active {
            background: rgba(255,255,255,0.3);
            border-color: rgba(255,255,255,0.4);
        }

        .logo {
            color: white;
            font-size: 1.2em;
            font-weight: 700;
            text-decoration: none;
        }

        .container {
            background: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            padding: 40px;
            margin-bottom: 20px;
        }
    </style>
    <script src="https://cdn.jsdelivr.net/npm/marked@12.0.0/marked.min.js"></script>
</head>
<body>
    <div class="nav-header">
        <a href="/" class="logo">🚀 Lil-RAG</a>
        <div class="nav-links">
            <a href="/" class="nav-link">🏠 Home</a>
            <a href="/chat" class="nav-link active">💬 Chat</a>
            <a href="/documents" class="nav-link">📚 Documents</a>
        </div>
    </div>
    <div class="chat-container">
        <div class="chat-header">
            <h1>🤖 LilRag Chat</h1>
            <p>Ask questions about your indexed documents</p>
            <button class="clear-chat-button" onclick="clearChatHistory()" title="Clear chat history">
                🗑️ Clear Chat
            </button>
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
                html = html.replace(/\\[([a-zA-Z0-9_-]+)\\]/g, function(match, docId) {
                    // Find the source with this document ID to get chunk information
                    let chunkParam = '';
                    if (sources) {
                        const source = sources.find(s => s.ID === docId);
                        if (source && source.Metadata && source.Metadata.chunk_index !== undefined) {
                            chunkParam = '?highlight=' + source.Metadata.chunk_index;
                        }
                    }
                    return '<a href="/view/' + docId + chunkParam + 
                        '" class="doc-ref-link" target="_blank">[' + docId + ']</a>';
                });
            }
            
            if (sources && sources.length > 0) {
                html += '<div class="sources-section">';
                html += '<div class="sources-header">📚 Sources (' + sources.length + '):</div>';
                html += '<div class="sources-compact">';
                sources.forEach((source, index) => {
                    const sourceId = 'source-' + Date.now() + '-' + index;
                    html += '<button class="source-button" onclick="toggleSource(\\'' + sourceId + '\\'))">';
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
                    const chunkParam = source.Metadata && source.Metadata.chunk_index !== undefined ? 
                        '?highlight=' + source.Metadata.chunk_index : '';
                    html += '<a href="/view/' + source.ID + chunkParam + 
                        '" class="view-document-link" target="_blank">📄 View Document</a>';
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
    </div>
</body>
</html>`

	if _, err := w.Write([]byte(html)); err != nil {
		fmt.Printf("Error writing response: %v\n", err)
	}
}

// handleChatMessage processes chat API requests
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
	// Token tracking is now handled within the chat client itself

	log.Printf("Chat completed successfully - found %d sources, response length: %d", len(searchResults), len(response))
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(chatResp); err != nil {
		log.Printf("Error encoding chat response: %v", err)
	}
}
