package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"lil-rag/internal/theme"
	"lil-rag/pkg/lilrag"
	"lil-rag/pkg/metrics"
)

// Handler is the main handler struct containing shared dependencies
type Handler struct {
	rag      *lilrag.LilRag
	version  string
	dataDir  string
	renderer *theme.Renderer
}

// Request and Response types
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

// Constructor functions
func New(rag *lilrag.LilRag) *Handler {
	renderer, err := theme.NewRenderer()
	if err != nil {
		log.Printf("Failed to create theme renderer: %v", err)
		renderer = nil
	}
	return &Handler{rag: rag, version: "dev", dataDir: "", renderer: renderer}
}

func NewWithVersion(rag *lilrag.LilRag, version string, dataDir string) *Handler {
	renderer, err := theme.NewRenderer()
	if err != nil {
		log.Printf("Failed to create theme renderer: %v", err)
		renderer = nil
	}
	return &Handler{rag: rag, version: version, dataDir: dataDir, renderer: renderer}
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