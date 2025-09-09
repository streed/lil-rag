package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"lil-rag/pkg/lilrag"
)

// ServiceHandler provides service-oriented HTTP handlers using the new abstraction layer
type ServiceHandler struct {
	services *lilrag.Services
	version  string
	dataDir  string
}

// NewServiceHandler creates a new service-based handler
func NewServiceHandler(services *lilrag.Services, version, dataDir string) *ServiceHandler {
	return &ServiceHandler{
		services: services,
		version:  version,
		dataDir:  dataDir,
	}
}

// IndexText handles text indexing requests
func (h *ServiceHandler) IndexText() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

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
		}

		if req.Text == "" {
			h.writeError(w, http.StatusBadRequest, "text is required", "")
			return
		}

		log.Printf("Indexing document %s with %d characters", req.ID, len(req.Text))
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()

		// Use modern service interface
		if err := h.services.Indexing.IndexText(ctx, req.Text, req.ID); err != nil {
			log.Printf("Failed to index document %s: %v", req.ID, err)
			h.writeServiceError(w, err)
			return
		}

		log.Printf("Successfully indexed document %s", req.ID)
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"id":      req.ID,
			"message": "Successfully indexed text document",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// Search handles search requests using the service layer
func (h *ServiceHandler) Search() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req SearchRequest
		var err error

		switch r.Method {
		case http.MethodGet:
			// Handle GET request
			req.Query = r.URL.Query().Get("query")
			if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
				limit := 10 // default
				if parsedLimit := parseIntOrDefault(limitStr, 10); parsedLimit > 0 && parsedLimit <= 100 {
					limit = parsedLimit
				}
				req.Limit = limit
			}
		case http.MethodPost:
			// Handle POST request
			if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
				h.writeError(w, http.StatusBadRequest, "invalid request body", decodeErr.Error())
				return
			}
		default:
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if req.Query == "" {
			h.writeError(w, http.StatusBadRequest, "query parameter is required", "")
			return
		}

		if req.Limit <= 0 {
			req.Limit = 10
		}

		log.Printf("Search request: query='%s', limit=%d", req.Query, req.Limit)
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Use modern service interface
		results, err := h.services.Search.Search(ctx, req.Query, req.Limit)
		if err != nil {
			log.Printf("Search failed: %v", err)
			h.writeServiceError(w, err)
			return
		}

		log.Printf("Search completed: found %d results", len(results))

		w.Header().Set("Content-Type", "application/json")
		response := SearchResponse{Results: results}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode search response: %v", err)
		}
	}
}

// Chat handles chat requests using the service layer
func (h *ServiceHandler) Chat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
			return
		}

		if req.Message == "" {
			h.writeError(w, http.StatusBadRequest, "message is required", "")
			return
		}

		if req.Limit <= 0 {
			req.Limit = 5
		}

		log.Printf("Chat request: message='%s', limit=%d", req.Message, req.Limit)
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()

		// Use modern service interface
		response, sources, err := h.services.Chat.Chat(ctx, req.Message, req.Limit)
		if err != nil {
			log.Printf("Chat failed: %v", err)
			h.writeServiceError(w, err)
			return
		}

		log.Printf("Chat completed: generated response with %d sources", len(sources))

		w.Header().Set("Content-Type", "application/json")
		chatResponse := ChatResponse{
			Response: response,
			Sources:  sources,
			Query:    req.Message,
		}

		if err := json.NewEncoder(w).Encode(chatResponse); err != nil {
			log.Printf("Failed to encode chat response: %v", err)
		}
	}
}

// ListDocuments handles document listing requests
func (h *ServiceHandler) ListDocuments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Use modern service interface
		documents, err := h.services.Document.ListDocuments(ctx)
		if err != nil {
			log.Printf("Failed to list documents: %v", err)
			h.writeServiceError(w, err)
			return
		}

		log.Printf("Listed %d documents", len(documents))

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"documents": documents,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode documents response: %v", err)
		}
	}
}

// DeleteDocument handles document deletion requests
func (h *ServiceHandler) DeleteDocument() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract document ID from URL path
		path := r.URL.Path
		segments := splitPath(path)
		if len(segments) < 3 || segments[len(segments)-1] == "" {
			h.writeError(w, http.StatusBadRequest, "document ID is required", "")
			return
		}

		documentID := segments[len(segments)-1]
		log.Printf("Delete request for document ID: %s", documentID)

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Use modern service interface
		if err := h.services.Document.DeleteDocument(ctx, documentID); err != nil {
			log.Printf("Failed to delete document %s: %v", documentID, err)
			h.writeServiceError(w, err)
			return
		}

		log.Printf("Successfully deleted document %s", documentID)

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"success": true,
			"message": "Document deleted successfully",
			"id":      documentID,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode delete response: %v", err)
		}
	}
}

// Health handles health check requests
func (h *ServiceHandler) Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Use modern service interface
		if err := h.services.Health.CheckHealth(ctx); err != nil {
			log.Printf("Health check failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			response := map[string]interface{}{
				"status":    "unhealthy",
				"timestamp": time.Now().UTC(),
				"error":     err.Error(),
				"version":   h.version,
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"version":   h.version,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode health response: %v", err)
		}
	}
}

// Metrics handles metrics requests
func (h *ServiceHandler) Metrics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Use modern service interface
		metrics, err := h.services.Health.GetMetrics(ctx)
		if err != nil {
			log.Printf("Failed to get metrics: %v", err)
			h.writeServiceError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			log.Printf("Failed to encode metrics response: %v", err)
		}
	}
}

// Helper methods

// writeError writes an error response
func (h *ServiceHandler) writeError(w http.ResponseWriter, status int, errType, message string) {
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

// writeServiceError writes an error response from a service error
func (h *ServiceHandler) writeServiceError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	// Convert service error to appropriate HTTP status
	status := http.StatusInternalServerError
	if lilragErr, ok := err.(*lilrag.Error); ok {
		switch lilragErr.Type {
		case lilrag.ErrorTypeValidation:
			status = http.StatusBadRequest
		case lilrag.ErrorTypeConfig:
			status = http.StatusInternalServerError
		case lilrag.ErrorTypeStorage:
			if lilragErr.Code == "DOCUMENT_NOT_FOUND" {
				status = http.StatusNotFound
			} else {
				status = http.StatusInternalServerError
			}
		case lilrag.ErrorTypeEmbedding:
			status = http.StatusServiceUnavailable
		case lilrag.ErrorTypeParsing:
			status = http.StatusBadRequest
		case lilrag.ErrorTypeSearch:
			status = http.StatusInternalServerError
		case lilrag.ErrorTypeChat:
			status = http.StatusServiceUnavailable
		case lilrag.ErrorTypeNetwork:
			status = http.StatusServiceUnavailable
		case lilrag.ErrorTypeInternal:
			status = http.StatusInternalServerError
		}
	}

	w.WriteHeader(status)
	response := lilrag.ToErrorResponse(err)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode service error response: %v", err)
	}
}

// Helper functions
func splitPath(path string) []string {
	parts := []string{}
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func parseIntOrDefault(s string, defaultVal int) int {
	result := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return defaultVal
		}
		result = result*10 + int(r-'0')
	}
	return result
}
