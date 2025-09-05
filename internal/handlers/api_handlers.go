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

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"lil-rag/pkg/lilrag"
	"lil-rag/pkg/metrics"
)

// Index handles document indexing requests at /api/index
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
		// Token tracking for embeddings is handled within the embedder itself

		log.Printf("Successfully indexed document %s", req.ID)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "indexed", "id": req.ID}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// Search handles search requests at /api/search (supports both GET and POST)
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
	log.Printf("Search POST request - query: '%s', limit: %d", req.Query, req.Limit)
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
	// Token tracking for search embeddings is handled within the embedder itself

	log.Printf("Search completed - found %d results for query '%s'", len(results), query)
	response := SearchResponse{Results: results}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding search response: %v", err)
	}
}

// Health handles health check requests at /api/health
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

// Metrics handles metrics requests at /api/metrics
func (h *Handler) Metrics() http.HandlerFunc {
	// Return the Prometheus metrics handler
	return promhttp.Handler().ServeHTTP
}

// UpdateSystemMetrics updates system-wide metrics like document count
func (h *Handler) UpdateSystemMetrics(ctx context.Context) {
	// Get document count and update metrics
	if documents, err := h.rag.ListDocuments(ctx); err == nil {
		metrics.UpdateDocumentCount(len(documents))
	} else {
		log.Printf("Failed to update document count metric: %v", err)
	}
}

// Documents handles document listing requests at /api/documents
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

// handleFileUpload processes file upload requests
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

	// Check if this is an image file and preserve it in the data directory
	var permanentPath string
	if h.dataDir != "" && lilrag.IsImageFile(tempFile.Name()) {
		// Create images directory
		imagesDir := filepath.Join(h.dataDir, "images")
		if err := os.MkdirAll(imagesDir, 0o755); err != nil {
			log.Printf("Warning: Failed to create images directory: %v", err)
		} else {
			// Generate a unique filename
			ext := filepath.Ext(header.Filename)
			permanentPath = filepath.Join(imagesDir, id+ext)

			// Copy the temporary file to the permanent location
			if err := copyFile(tempFile.Name(), permanentPath); err != nil {
				log.Printf("Warning: Failed to preserve image file: %v", err)
				permanentPath = "" // Reset if copy failed
			} else {
				log.Printf("Preserved image file at: %s", permanentPath)
			}
		}
	}

	// Index the file using document handler
	if err := h.rag.IndexFile(ctx, tempFile.Name(), id); err != nil {
		log.Printf("Failed to index file %s: %v", header.Filename, err)

		// Check if this is a client error (bad input) vs server error
		errorMessage := err.Error()
		if strings.Contains(errorMessage, "no content found") ||
			strings.Contains(errorMessage, "unsupported file format") ||
			strings.Contains(errorMessage, "failed to parse") {
			// Client error - bad file content or format
			h.writeError(w, http.StatusBadRequest, "failed to index file", errorMessage)
		} else {
			// Server error - embedding service, storage, etc.
			h.writeError(w, http.StatusInternalServerError, "failed to index file", errorMessage)
		}
		return
	}

	// If we preserved an image file, update the document's source path
	if permanentPath != "" {
		if err := h.updateDocumentSourcePath(ctx, id, permanentPath); err != nil {
			log.Printf("Warning: Failed to update document source path: %v", err)
		}
	}

	log.Printf("Successfully indexed file %s as document %s", header.Filename, id)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "indexed",
		"id":     id,
	}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// updateDocumentSourcePath updates the source path for a document
func (h *Handler) updateDocumentSourcePath(ctx context.Context, documentID, sourcePath string) error {
	// Get the storage instance from rag and update directly
	// This is a temporary solution until we add this to the Storage interface
	if storage := h.rag.GetStorage(); storage != nil {
		if sqliteStorage, ok := storage.(*lilrag.SQLiteStorage); ok {
			return sqliteStorage.UpdateDocumentSourcePath(ctx, documentID, sourcePath)
		}
	}
	log.Printf("Could not update document source path - storage not accessible")
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
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
			if _, writeErr := w.Write([]byte(content)); writeErr != nil {
				log.Printf("Failed to write response: %v", writeErr)
			}
			return
		}
		log.Printf("Failed to parse document file %s: %v", docInfo.SourcePath, err)
	}

	h.writeError(w, http.StatusNotFound, "document content not available",
		"Original source file not found or not readable")
}
