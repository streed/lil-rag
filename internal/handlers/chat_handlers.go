package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"lil-rag/internal/theme"
	"lil-rag/pkg/chathistory"
	"lil-rag/pkg/lilrag"
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
            gap: 20px;
        }

        .sidebar {
            width: 280px;
            border-right: 1px solid #e0e0e0;
            display: flex;
            flex-direction: column;
            background: #f8f9fa;
            border-radius: 8px;
            padding: 15px;
        }

        .sidebar-header {
            margin-bottom: 15px;
            padding-bottom: 10px;
            border-bottom: 1px solid #e0e0e0;
        }

        .sidebar-header h3 {
            font-size: 1rem;
            margin: 0;
            color: #333;
        }

        .new-chat-btn {
            background: #007bff;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9rem;
            margin-top: 8px;
            transition: background-color 0.2s ease;
        }

        .new-chat-btn:hover {
            background: #0056b3;
        }

        .chat-sessions {
            flex: 1;
            overflow-y: auto;
            margin-top: 10px;
        }

        .chat-session {
            padding: 10px;
            border-radius: 6px;
            cursor: pointer;
            margin-bottom: 5px;
            border: 1px solid transparent;
            transition: all 0.2s ease;
        }

        .chat-session:hover {
            background: #e9ecef;
            border-color: #dee2e6;
        }

        .chat-session.active {
            background: #007bff;
            color: white;
        }

        .chat-session-title {
            font-size: 0.9rem;
            font-weight: 500;
            margin-bottom: 2px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .chat-session-time {
            font-size: 0.75rem;
            opacity: 0.7;
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

        .token-counter {
            font-size: 0.8rem;
            color: #666;
            text-align: right;
            margin-bottom: 8px;
            opacity: 0.8;
        }

        .token-counter.warning {
            color: #ff6b35;
        }

        .token-counter.danger {
            color: #dc3545;
            font-weight: 500;
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
            <div class="sidebar">
                <div class="sidebar-header">
                    <h3>Chat History</h3>
                    <button class="new-chat-btn" onclick="startNewChat()">
                        ➕ New Chat
                    </button>
                </div>
                <div class="chat-sessions" id="chatSessions">
                    <!-- Chat sessions will be loaded here -->
                </div>
            </div>
            
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
                    <div class="token-counter" id="tokenCounter">
                        Tokens remaining: <span id="remainingTokens">1000</span>
                    </div>
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	log.Printf("Chat request - message: '%s', session: '%s', limit: %d, stream: %v", req.Message, req.SessionID, req.Limit, req.Stream)

	// Handle chat history integration
	var sessionID string
	var chatContext *chathistory.ChatContext
	var hasCompacted bool

	if h.chatHistory != nil {
		// Create or get session
		if req.SessionID == "" {
			// Create new session
			session, err := h.chatHistory.CreateSession(ctx)
			if err != nil {
				log.Printf("Failed to create chat session: %v", err)
				// Continue without history if creation fails
				sessionID = ""
			} else {
				sessionID = session.ID
			}
		} else {
			sessionID = req.SessionID
		}

		// Get chat context if we have a session
		if sessionID != "" {
			var err error
			chatContext, err = h.chatHistory.GetChatContext(ctx, sessionID)
			if err != nil {
				log.Printf("Failed to get chat context: %v", err)
				// Continue without context if retrieval fails
				chatContext = nil
			} else {
				hasCompacted = chatContext.HasCompactedData

				// Check if we need to compact the history
				if chatContext.TotalTokens > chathistory.MaxContextTokens {
					err = h.compactChatHistory(ctx, sessionID, chatContext)
					if err != nil {
						log.Printf("Failed to compact chat history: %v", err)
					} else {
						// Refresh context after compaction
						if refreshedContext, err := h.chatHistory.GetChatContext(ctx, sessionID); err == nil {
							chatContext = refreshedContext
						}
						hasCompacted = true
					}
				}
			}
		}
	}

	// Store user message in history
	if h.chatHistory != nil && sessionID != "" {
		_, err := h.chatHistory.AddMessage(ctx, sessionID, "user", req.Message)
		if err != nil {
			log.Printf("Failed to store user message: %v", err)
		}
	}

	// Generate enhanced message with context
	enhancedMessage := h.buildContextualMessage(req.Message, chatContext)

	// Check if streaming is requested
	if req.Stream {
		h.handleStreamingChat(w, ctx, enhancedMessage, req, sessionID, chatContext, hasCompacted)
		return
	}

	// Generate LLM response using retrieved documents as context
	chatStart := time.Now()
	response, searchResults, err := h.rag.Chat(ctx, enhancedMessage, req.Limit)
	chatDuration := time.Since(chatStart)

	if err != nil {
		log.Printf("Chat failed for message '%s': %v", req.Message, err)
		metrics.RecordChatRequest(chatDuration, false, 0, 0)
		h.writeError(w, http.StatusInternalServerError, "chat failed", err.Error())
		return
	}

	// Store assistant response in history
	if h.chatHistory != nil && sessionID != "" {
		_, err := h.chatHistory.AddMessage(ctx, sessionID, "assistant", response)
		if err != nil {
			log.Printf("Failed to store assistant response: %v", err)
		}

		// Generate title after each response to keep it updated
		go h.generateSessionTitle(sessionID, req.Message)
	}

	// Calculate remaining tokens
	remainingTokens := chathistory.MaxContextTokens
	if chatContext != nil {
		remainingTokens = chatContext.RemainingTokens
	}

	chatResp := ChatResponse{
		Response:        response,
		Sources:         searchResults,
		Query:           req.Message,
		SessionID:       sessionID,
		RemainingTokens: remainingTokens,
		HasCompacted:    hasCompacted,
	}

	metrics.RecordChatRequest(chatDuration, true, len(searchResults), len(response))

	log.Printf("Chat completed successfully - session: %s, sources: %d, response length: %d, remaining tokens: %d",
		sessionID, len(searchResults), len(response), remainingTokens)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(chatResp); err != nil {
		log.Printf("Error encoding chat response: %v", err)
	}
}

// buildContextualMessage combines the current message with previous chat context
func (h *Handler) buildContextualMessage(currentMessage string, chatContext *chathistory.ChatContext) string {
	if chatContext == nil || len(chatContext.Messages) == 0 {
		return currentMessage
	}

	var contextParts []string

	// Add compacted history if available
	if chatContext.CompactedHistory != nil {
		contextParts = append(contextParts, "Previous conversation summary: "+chatContext.CompactedHistory.CompactedContent)
	}

	// Add recent message history
	recentMessages := chatContext.Messages
	if len(recentMessages) > 6 { // Limit to last 6 messages (3 exchanges)
		recentMessages = recentMessages[len(recentMessages)-6:]
	}

	for _, msg := range recentMessages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		contextParts = append(contextParts, fmt.Sprintf("%s: %s", role, msg.Content))
	}

	// Add current message
	contextParts = append(contextParts, "User: "+currentMessage)

	return strings.Join(contextParts, "\n\n")
}

// compactChatHistory compacts chat history when it gets too long
func (h *Handler) compactChatHistory(ctx context.Context, sessionID string, chatContext *chathistory.ChatContext) error {
	if h.summarizer == nil {
		return fmt.Errorf("summarizer not available")
	}

	// Generate summary of the messages
	summary, err := h.summarizer.CompactHistory(ctx, chatContext.Messages)
	if err != nil {
		return fmt.Errorf("failed to generate chat summary: %w", err)
	}

	// Store the compaction
	return h.chatHistory.CompactChatHistory(ctx, sessionID, summary)
}

// generateSessionTitle asynchronously generates a title for a new chat session
func (h *Handler) generateSessionTitle(sessionID, _ string) {
	if h.summarizer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First check if the session already has a custom title
	session, err := h.chatHistory.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("Failed to get session for title check: %v", err)
		return
	}

	// Don't generate title if it already has a custom title (not "New Chat" and not empty)
	if session.Title != "" && session.Title != "New Chat" {
		return
	}

	// Get the first few messages to generate a title
	chatContext, err := h.chatHistory.GetChatContext(ctx, sessionID)
	if err != nil {
		log.Printf("Failed to get chat context for title generation: %v", err)
		return
	}

	title, err := h.summarizer.GenerateTitle(ctx, chatContext.Messages)
	if err != nil {
		log.Printf("Failed to generate chat title: %v", err)
		// Fallback to simple title
		title = chathistory.GenerateChatSummary(chatContext.Messages)
	}

	// Update the session title
	err = h.chatHistory.UpdateSessionTitle(ctx, sessionID, title)
	if err != nil {
		log.Printf("Failed to update session title: %v", err)
	}
}

// handleStreamingChat processes streaming chat requests using Server-Sent Events
func (h *Handler) handleStreamingChat(w http.ResponseWriter, ctx context.Context, enhancedMessage string, req ChatRequest, sessionID string, chatContext *chathistory.ChatContext, hasCompacted bool) {
	// Set headers for Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Send initial metadata with session info
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming not supported", "")
		return
	}

	chatStart := time.Now()
	var completeResponse strings.Builder
	var searchResults []lilrag.SearchResult

	// Create a streaming handler
	streamHandler := func(chunk string, done bool) error {
		completeResponse.WriteString(chunk)
		
		// Send the chunk as SSE
		chunkData := ChatStreamChunk{
			Type:    "chunk",
			Content: chunk,
			Done:    done,
		}
		
		data, err := json.Marshal(chunkData)
		if err != nil {
			return fmt.Errorf("failed to marshal chunk: %w", err)
		}
		
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
		if err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
		
		flusher.Flush()
		return nil
	}

	// Generate streaming response
	var err error
	searchResults, err = h.rag.ChatStreaming(ctx, enhancedMessage, req.Limit, streamHandler)
	chatDuration := time.Since(chatStart)
	
	if err != nil {
		log.Printf("Streaming chat failed for message '%s': %v", req.Message, err)
		metrics.RecordChatRequest(chatDuration, false, 0, 0)
		
		// Send error as SSE
		errorChunk := ChatStreamChunk{
			Type:    "error",
			Content: "Failed to generate response",
			Done:    true,
		}
		data, _ := json.Marshal(errorChunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	finalResponse := completeResponse.String()

	// Store assistant response in history
	if h.chatHistory != nil && sessionID != "" {
		_, err := h.chatHistory.AddMessage(ctx, sessionID, "assistant", finalResponse)
		if err != nil {
			log.Printf("Failed to store assistant response: %v", err)
		}

		// Generate title after each response to keep it updated
		go h.generateSessionTitle(sessionID, req.Message)
	}

	// Calculate remaining tokens
	remainingTokens := chathistory.MaxContextTokens
	if chatContext != nil {
		remainingTokens = chatContext.RemainingTokens
	}

	// Send sources and metadata
	sourcesChunk := ChatStreamChunk{
		Type:            "sources",
		Sources:         searchResults,
		Query:           req.Message,
		SessionID:       sessionID,
		RemainingTokens: remainingTokens,
		HasCompacted:    hasCompacted,
	}
	
	data, err := json.Marshal(sourcesChunk)
	if err != nil {
		log.Printf("Failed to marshal sources chunk: %v", err)
	} else {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Send final done message
	doneChunk := ChatStreamChunk{
		Type: "done",
		Done: true,
	}
	
	data, err = json.Marshal(doneChunk)
	if err != nil {
		log.Printf("Failed to marshal done chunk: %v", err)
	} else {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	metrics.RecordChatRequest(chatDuration, true, len(searchResults), len(finalResponse))

	log.Printf("Streaming chat completed successfully - session: %s, sources: %d, response length: %d, remaining tokens: %d",
		sessionID, len(searchResults), len(finalResponse), remainingTokens)
}
