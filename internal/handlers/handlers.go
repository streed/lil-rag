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

	"lil-rag/pkg/minirag"
)

type Handler struct {
	rag     *minirag.MiniRag
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

type SearchResponse struct {
	Results []minirag.SearchResult `json:"results"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func New(rag *minirag.MiniRag) *Handler {
	return &Handler{rag: rag, version: "dev"}
}

func NewWithVersion(rag *minirag.MiniRag, version string) *Handler {
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
			req.ID = minirag.GenerateDocumentID()
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
		if _, ok := interface{}(h.rag).(*minirag.MiniRag); ok {
			// Access the embedder (this would need to be exposed in MiniRag)
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

func (h *Handler) Static() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		html := `<!DOCTYPE html>
<html>
<head>
    <title>MiniRag API</title>
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
    <h1>MiniRag API</h1>
    <p>A simple RAG (Retrieval Augmented Generation) API using SQLite and Ollama</p>
    
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
		id = minirag.GenerateDocumentID()
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
	tempFile, err := os.CreateTemp(tempDir, "minirag_upload_*"+filepath.Ext(header.Filename))
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
