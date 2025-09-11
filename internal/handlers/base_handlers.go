package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"lil-rag/internal/theme"
	"lil-rag/pkg/auth"
	"lil-rag/pkg/chathistory"
	"lil-rag/pkg/lilrag"
	"lil-rag/pkg/metrics"
)

// Handler is the main handler struct containing shared dependencies
type Handler struct {
	rag         *lilrag.LilRag
	version     string
	dataDir     string
	renderer    *theme.Renderer
	chatHistory *chathistory.ChatHistory
	summarizer  chathistory.ChatSummarizer
	auth        *auth.Auth
	secure      bool // Whether authentication is required
}

// Request and Response types
type IndexRequest struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Namespace string `json:"namespace,omitempty"`
}

type SearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type ChatResponse struct {
	Response        string                `json:"response"`
	Sources         []lilrag.SearchResult `json:"sources"`
	Query           string                `json:"query"`
	SessionID       string                `json:"session_id"`
	RemainingTokens int                   `json:"remaining_tokens"`
	HasCompacted    bool                  `json:"has_compacted"`
}

type ChatSessionsResponse struct {
	Sessions []chathistory.ChatSession `json:"sessions"`
}

type CreateSessionResponse struct {
	Session *chathistory.ChatSession `json:"session"`
}

type ChatHistoryResponse struct {
	Messages []chathistory.ChatMessage `json:"messages"`
}

type SearchResponse struct {
	Results []lilrag.SearchResult `json:"results"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Constructor functions
func New(rag *lilrag.LilRag) *Handler {
	renderer, err := theme.NewRenderer()
	if err != nil {
		log.Printf("Failed to create theme renderer: %v", err)
		renderer = nil
	}
	return &Handler{rag: rag, version: "dev", dataDir: "", renderer: renderer}
}

func NewWithVersion(rag *lilrag.LilRag, version, dataDir string, secure bool) *Handler {
	renderer, err := theme.NewRenderer()
	if err != nil {
		log.Printf("Failed to create theme renderer: %v", err)
		renderer = nil
	}

	// Initialize chat history database
	chatHistoryPath := dataDir + "/chat_history.db"
	chatHistory, err := chathistory.New(chatHistoryPath)
	if err != nil {
		log.Printf("Failed to create chat history: %v", err)
		chatHistory = nil
	}

	// Create summarizer with LLM adapter
	var summarizer chathistory.ChatSummarizer
	if chatHistory != nil {
		summarizer = chathistory.NewLLMSummarizer(&LLMAdapter{rag: rag})
	}

	// Initialize authentication system
	authPath := dataDir + "/auth.db"
	authSystem, err := auth.New(authPath)
	if err != nil {
		log.Printf("Failed to create auth system: %v", err)
		authSystem = nil
	}

	return &Handler{
		rag:         rag,
		version:     version,
		dataDir:     dataDir,
		renderer:    renderer,
		chatHistory: chatHistory,
		summarizer:  summarizer,
		auth:        authSystem,
		secure:      secure,
	}
}

// LLMAdapter adapts the lilrag to the chathistory.LLMClient interface
type LLMAdapter struct {
	rag *lilrag.LilRag
}

func (a *LLMAdapter) GenerateResponse(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// For now, we'll combine system and user prompts since the current Chat method
	// doesn't accept separate system prompts. This could be enhanced later.
	combinedPrompt := fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, userPrompt)
	response, _, err := a.rag.Chat(ctx, combinedPrompt, 0) // Use 0 sources for summary generation
	return response, err
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

// Utility functions
func (h *Handler) writeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := ErrorResponse{
		Error:   errType,
		Message: message,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}
