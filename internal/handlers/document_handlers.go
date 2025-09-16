package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lil-rag/internal/theme"
	"lil-rag/pkg/lilrag"
)

// ViewDocument serves a document for web viewing at /view/{id}
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

// DocumentRouter routes between document content, chunks, and delete requests at /api/documents/*
func (h *Handler) DocumentRouter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			h.DeleteDocument().ServeHTTP(w, r)
		case strings.HasSuffix(r.URL.Path, "/chunks"):
			h.DocumentChunks().ServeHTTP(w, r)
		default:
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

		// Check if this is an image document to determine which chunks method to use
		docInfo, err := h.rag.GetDocumentByID(ctx, documentID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "document not found", err.Error())
			return
		}

		// For image documents, use ChunksWithInfo to include IDs for editing
		if docInfo.IsImage {
			chunksWithInfo, err := h.rag.GetDocumentChunksWithInfo(ctx, documentID)
			if err != nil {
				h.writeError(w, http.StatusNotFound, "chunks not found", err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(chunksWithInfo); err != nil {
				h.writeError(w, http.StatusInternalServerError, "failed to encode chunks", err.Error())
			}
		} else {
			// For regular documents, use the standard chunks method
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
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// ServeFile serves image files from document source paths at /file/{id}
func (h *Handler) ServeFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract document ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) != 2 || pathParts[0] != "file" {
			h.writeError(w, http.StatusBadRequest, "invalid file path", "Expected format: /file/{document_id}")
			return
		}

		documentID := pathParts[1]
		if documentID == "" {
			h.writeError(w, http.StatusBadRequest, "document ID required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Get document information
		docInfo, err := h.rag.GetDocumentByID(ctx, documentID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "document not found", err.Error())
			return
		}

		// Check if this is an image document with a valid source path
		if !docInfo.IsImage || docInfo.SourcePath == "" {
			h.writeError(w, http.StatusNotFound, "file not available", "Document is not an image or source path is missing")
			return
		}

		// Check if file exists
		if _, err := os.Stat(docInfo.SourcePath); os.IsNotExist(err) {
			h.writeError(w, http.StatusNotFound, "file not found", "Original image file is no longer available")
			return
		}

		// Determine content type based on file extension
		ext := strings.ToLower(filepath.Ext(docInfo.SourcePath))
		var contentType string
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".bmp":
			contentType = "image/bmp"
		case ".webp":
			contentType = "image/webp"
		case ".tiff", ".tif":
			contentType = "image/tiff"
		default:
			contentType = "application/octet-stream"
		}

		// Set headers
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

		// Serve the file
		http.ServeFile(w, r, docInfo.SourcePath)
	}
}

// UpdateChunk handles chunk update requests at /api/chunks/{id}
func (h *Handler) UpdateChunk() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract chunk ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) != 3 || pathParts[0] != "api" || pathParts[1] != "chunks" {
			h.writeError(w, http.StatusBadRequest, "invalid chunk path", "Expected format: /api/chunks/{chunk_id}")
			return
		}

		chunkID := pathParts[2]
		if chunkID == "" {
			h.writeError(w, http.StatusBadRequest, "chunk ID required", "")
			return
		}

		// Parse request body
		var updateReq struct {
			Text string `json:"text"`
		}

		if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid JSON", err.Error())
			return
		}

		if strings.TrimSpace(updateReq.Text) == "" {
			h.writeError(w, http.StatusBadRequest, "text cannot be empty", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Update the chunk
		if err := h.rag.UpdateChunk(ctx, chunkID, updateReq.Text); err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to update chunk", err.Error())
			return
		}

		// Return success response
		response := map[string]interface{}{
			"success": true,
			"message": "Chunk updated successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// GetChunk handles chunk retrieval requests at /api/chunks/{id}
func (h *Handler) GetChunk() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Extract chunk ID from URL path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) != 3 || pathParts[0] != "api" || pathParts[1] != "chunks" {
			h.writeError(w, http.StatusBadRequest, "invalid chunk path", "Expected format: /api/chunks/{chunk_id}")
			return
		}

		chunkID := pathParts[2]
		if chunkID == "" {
			h.writeError(w, http.StatusBadRequest, "chunk ID required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Get the chunk
		chunk, err := h.rag.GetChunk(ctx, chunkID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "chunk not found", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(chunk); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

// serveDocumentContent serves the document content in a web viewer
func (h *Handler) serveDocumentContent(
	w http.ResponseWriter, r *http.Request, docInfo *lilrag.DocumentInfo, highlightChunk int,
) {
	// Check if this is an image document
	if docInfo.IsImage && docInfo.SourcePath != "" {
		h.serveImageDocument(w, r, docInfo, highlightChunk)
		return
	}

	// Use new template system if available - currently disabled for document view fallback
	// TODO: Re-enable when document-view.html template is properly implemented
	if false && h.renderer != nil {
		// Create template data with document info in Data field
		data := &theme.TemplateData{
			Title:    fmt.Sprintf("Document: %s", docInfo.ID),
			Version:  h.version,
			PageName: "documents", // For navigation highlighting
			Data: map[string]interface{}{
				"DocumentInfo":   docInfo,
				"DocumentID":     docInfo.ID,
				"HighlightChunk": highlightChunk,
			},
		}

		w.Header().Set("Content-Type", "text/html")
		if err := h.renderer.RenderPage(w, "document-view.html", data); err != nil {
			log.Printf("Template rendering error: %v", err)
			h.fallbackDocumentView(w, r, docInfo, highlightChunk)
		}
		return
	}

	// Fallback to original HTML
	h.fallbackDocumentView(w, r, docInfo, highlightChunk)
}

// fallbackDocumentView serves the original document view HTML as fallback
func (h *Handler) fallbackDocumentView(
	w http.ResponseWriter,
	_ *http.Request,
	docInfo *lilrag.DocumentInfo,
	highlightChunk int,
) {
	// For non-image documents, serve a simple HTML viewer with the document content
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Document: %s</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        html {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100%%;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1000px;
            margin: 0 auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
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
        
        .document-header {
            border-bottom: 1px solid #e9ecef;
            margin-bottom: 20px;
            padding-bottom: 20px;
            position: relative;
        }
        .document-title {
            font-size: 1.5em;
            margin: 0 0 10px 0;
            color: #333;
            text-align: center;
        }
        .document-meta {
            color: #666;
            font-size: 0.9em;
            text-align: center;
        }
        .document-content {
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
        
        .container {
            background: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            padding: 40px;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="nav-header">
        <a href="/" class="logo">🚀 Lil-RAG</a>
        <div class="nav-links">
            <a href="/" class="nav-link">🏠 Home</a>
            <a href="/chat" class="nav-link">💬 Chat</a>
            <a href="/documents" class="nav-link">📚 Documents</a>
        </div>
    </div>
    <div class="container">
            <div class="document-header">
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
        
        // HTML escape function to prevent XSS and rendering issues
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        
        console.log('Document viewer script starting...');
        
        // Load document chunks for highlighting
        fetch('/api/documents/' + '%s' + '/chunks')
            .then(response => {
                if (!response.ok) {
                    throw new Error('Failed to fetch chunks: ' + response.status);
                }
                return response.json();
            })
            .then(chunks => {
                const contentDiv = document.getElementById('content');
                contentDiv.innerHTML = '';
                
                if (!chunks || chunks.length === 0) {
                    contentDiv.innerHTML = '<div class="chunk"><div class="chunk-text">No content chunks ' +
                        'available for this document.</div></div>';
                    return;
                }
                
                chunks.forEach((chunk, index) => {
                    const chunkDiv = document.createElement('div');
                    chunkDiv.className = 'chunk';
                    if (index === highlightChunk) {
                        chunkDiv.className += ' highlighted-chunk';
                    }
                    
                    // Handle different chunk formats (Text vs text property)
                    const chunkText = chunk.Text || chunk.text || '';
                    const escapedText = escapeHtml(chunkText).replace(/\n/g, '<br>');
                    chunkDiv.innerHTML = '<div class="chunk-text">' + escapedText + '</div>';
                    contentDiv.appendChild(chunkDiv);
                });
            })
            .catch(error => {
                console.error('Error loading chunks:', error);
                // Fallback to regular content loading
                fetch('/api/documents/' + '%s')
                    .then(response => {
                        if (!response.ok) {
                            throw new Error('Failed to fetch document: ' + response.status);
                        }
                        return response.text();
                    })
                    .then(content => {
                        const contentDiv = document.getElementById('content');
                        if (content && content.trim()) {
                            const escapedContent = escapeHtml(content).replace(/\n/g, '<br>');
                            contentDiv.innerHTML = '<div class="chunk"><div class="chunk-text">' + 
                                escapedContent + '</div></div>';
                        } else {
                            contentDiv.innerHTML = '<div class="chunk"><div class="chunk-text">No content ' +
                                'available for this document.</div></div>';
                        }
                    })
                    .catch(fallbackError => {
                        console.error('Error loading document content:', fallbackError);
                        document.getElementById('content').innerHTML = 
                            '<div class="chunk"><div class="chunk-text">Error loading document content: ' + 
                            fallbackError.message + '</div></div>';
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
		docInfo.ID,         // title
		docInfo.ID,         // delete button
		docInfo.ID,         // document title
		docInfo.DocType,    // type
		docInfo.ChunkCount, // chunks
		docInfo.SourcePath, // source
		docInfo.UpdatedAt.Format("2006-01-02 15:04:05"), // updated
		highlightChunk, // highlightChunk JS variable
		docInfo.ID,     // fetch chunks URL
		docInfo.ID,     // fetch document URL
	)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Failed to write HTML response: %v", err)
	}
}

// serveImageDocument serves the image document with its visual content and OCR text
func (h *Handler) serveImageDocument(
	w http.ResponseWriter, r *http.Request, docInfo *lilrag.DocumentInfo, highlightChunk int,
) {
	// Check if template renderer is available
	if h.renderer != nil {
		pageData := map[string]interface{}{
			"DocumentID":     docInfo.ID,
			"DocumentInfo":   docInfo,
			"HighlightChunk": highlightChunk,
		}

		templateData := &theme.TemplateData{
			Title:       fmt.Sprintf("Image Document: %s", docInfo.ID),
			PageName:    "documents",
			PageContent: "image-view.html",
			Data:        pageData,
		}

		if err := h.renderer.RenderPage(w, "base.html", templateData); err != nil {
			log.Printf("Failed to render image document template: %v", err)
			// Fallback to original HTML approach
			h.fallbackImageDocumentView(w, r, docInfo, highlightChunk)
			return
		}
		return
	}

	// Fallback to original HTML
	h.fallbackImageDocumentView(w, r, docInfo, highlightChunk)
}

// fallbackImageDocumentView serves the original image document view HTML as fallback
func (h *Handler) fallbackImageDocumentView(
	w http.ResponseWriter, _ *http.Request, docInfo *lilrag.DocumentInfo, highlightChunk int,
) {
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Image Document: %s</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        html {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100%%;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1000px;
            margin: 0 auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
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
        
        .document-header {
            border-bottom: 1px solid #e9ecef;
            margin-bottom: 20px;
            padding-bottom: 20px;
            position: relative;
        }
        
        .document-title {
            font-size: 1.8em;
            margin: 0 0 20px 0;
            color: #333;
        }
        
        .document-actions {
            position: absolute;
            top: 30px;
            right: 30px;
        }
        
        .delete-button {
            background: #dc3545;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9rem;
            transition: all 0.2s ease;
        }
        
        .delete-button:hover {
            background: #c82333;
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(220, 53, 69, 0.3);
        }
        
        .document-meta {
            color: #666;
            font-size: 0.9em;
        }
        
        .image-content {
            background: white;
            padding: 30px;
            display: flex;
            flex-direction: row;
            gap: 30px;
        }
        
        .image-viewer {
            flex: 1;
            max-width: 60%%;
        }
        
        .document-image {
            width: 100%%;
            max-height: 80vh;
            object-fit: contain;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
        }
        
        .ocr-content {
            flex: 1;
            max-width: 40%%;
        }
        
        .content-section-title {
            font-size: 1.2em;
            margin: 0 0 15px 0;
            color: #333;
            border-bottom: 2px solid #007bff;
            padding-bottom: 8px;
        }
        
        .ocr-text {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            white-space: pre-wrap;
            word-wrap: break-word;
            font-size: 0.9em;
            line-height: 1.5;
            max-height: 70vh;
            overflow-y: auto;
            border: 1px solid #e9ecef;
        }
        
        .chunk {
            margin-bottom: 15px;
            padding: 15px;
            background: white;
            border-radius: 6px;
            border-left: 3px solid #007bff;
            position: relative;
        }
        
        .chunk.highlighted-chunk {
            background: #fff3cd;
            border-left-color: #ffc107;
            box-shadow: 0 2px 8px rgba(255, 193, 7, 0.3);
        }
        
        .chunk-text {
            color: #333;
            min-height: 20px;
        }
        
        .chunk-actions {
            position: absolute;
            top: 10px;
            right: 10px;
            opacity: 0.3;
            transition: opacity 0.2s ease;
            z-index: 10;
        }
        
        .chunk:hover .chunk-actions {
            opacity: 1;
        }
        
        /* Make buttons visible on touch devices */
        @media (hover: none) {
            .chunk-actions {
                opacity: 0.7;
            }
        }
        
        .edit-btn {
            background: #007bff;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            margin-right: 5px;
            font-weight: 500;
            box-shadow: 0 2px 4px rgba(0, 123, 255, 0.2);
            transition: all 0.2s ease;
        }
        
        .edit-btn:hover {
            background: #0056b3;
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(0, 123, 255, 0.3);
        }
        
        .save-btn {
            background: #28a745;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            margin-right: 5px;
            font-weight: 500;
            box-shadow: 0 2px 4px rgba(40, 167, 69, 0.2);
            transition: all 0.2s ease;
        }
        
        .save-btn:hover {
            background: #1e7e34;
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(40, 167, 69, 0.3);
        }
        
        .cancel-btn {
            background: #6c757d;
            color: white;
            border: none;
            padding: 6px 12px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            font-weight: 500;
            box-shadow: 0 2px 4px rgba(108, 117, 125, 0.2);
            transition: all 0.2s ease;
        }
        
        .cancel-btn:hover {
            background: #545b62;
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(108, 117, 125, 0.3);
        }
        
        .chunk-editor {
            width: 100%%;
            min-height: 100px;
            padding: 10px;
            border: 1px solid #ccc;
            border-radius: 4px;
            font-family: inherit;
            font-size: 14px;
            resize: vertical;
        }
        
        .chunk-editor:focus {
            outline: none;
            border-color: #007bff;
            box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
        }
        
        .editing-indicator {
            color: #007bff;
            font-size: 12px;
            margin-top: 5px;
        }
        
        @media (max-width: 1024px) {
            .image-content {
                flex-direction: column;
            }
            
            .image-viewer, .ocr-content {
                max-width: 100%%;
            }
        }
    </style>
</head>
<body>
    <div class="nav-header">
        <a href="/" class="logo">🚀 Lil-RAG</a>
        <div class="nav-links">
            <a href="/" class="nav-link">🏠 Home</a>
            <a href="/chat" class="nav-link">💬 Chat</a>
            <a href="/documents" class="nav-link">📚 Documents</a>
        </div>
    </div>
    <div class="container">
        <div class="document-header">
            <div class="document-actions">
                <button class="delete-button" id="deleteBtn" onclick="deleteDocument('%s')">🗑️ Delete</button>
            </div>
            
            <h1 class="document-title">🖼️ %s</h1>
            
            <div class="document-meta">
                <strong>Type:</strong> %s (Image Document)<br>
                <strong>OCR Chunks:</strong> %d<br>
                <strong>Source:</strong> %s<br>
                <strong>Updated:</strong> %s
            </div>
        </div>
        
        <div class="image-content">
            <div class="image-viewer">
                <h3 class="content-section-title">📷 Original Image</h3>
                <img src="/file/%s" alt="Document Image" class="document-image" />
            </div>
            
            <div class="ocr-content">
                <h3 class="content-section-title">📝 Extracted Text (OCR)</h3>
                <div id="content">
                    Loading OCR content...
                </div>
            </div>
        </div>
    </div>

    <script>
        const highlightChunk = %d;
        
        // HTML escape function to prevent XSS and rendering issues
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        
        console.log('Image document viewer script starting...');
        
        // Load document chunks for OCR content display
        fetch('/api/documents/' + '%s' + '/chunks')
            .then(response => {
                if (!response.ok) {
                    throw new Error('Failed to fetch chunks: ' + response.status);
                }
                return response.json();
            })
            .then(chunks => {
                const contentDiv = document.getElementById('content');
                contentDiv.innerHTML = '';
                
                if (!chunks || chunks.length === 0) {
                    contentDiv.innerHTML = '<div class="ocr-text">No OCR content available for this ' +
                        'image document.</div>';
                    return;
                }
                
                chunks.forEach((chunk, index) => {
                    const chunkDiv = document.createElement('div');
                    chunkDiv.className = 'chunk';
                    
                    // Handle different chunk ID formats
                    const chunkId = chunk.id || chunk.ID || 'chunk-' + index;
                    const chunkText = chunk.text || chunk.Text || '';
                    
                    chunkDiv.dataset.chunkId = chunkId;
                    chunkDiv.dataset.originalText = chunkText;
                    
                    if (index === highlightChunk) {
                        chunkDiv.className += ' highlighted-chunk';
                    }
                    
                    // Create chunk content HTML with edit functionality
                    chunkDiv.innerHTML = 
                        '<div class="chunk-actions">' +
                            '<button class="edit-btn" onclick="editChunk(\'' + chunkId + '\')">' + 
                            '✏️ Edit</button>' +
                        '</div>' +
                        '<div class="chunk-text" id="text-' + chunkId + '">' + 
                        escapeHtml(chunkText).replace(/\n/g, '<br>') + '</div>';
                    
                    contentDiv.appendChild(chunkDiv);
                });
            })
            .catch(error => {
                console.error('Error loading OCR content:', error);
                document.getElementById('content').innerHTML = '<div class="ocr-text">Error loading OCR content: ' +
                    error.message + '</div>';
            });

        function editChunk(chunkId) {
            const chunkDiv = document.querySelector('[data-chunk-id="' + chunkId + '"]');
            const textDiv = document.getElementById('text-' + chunkId);
            const actionsDiv = chunkDiv.querySelector('.chunk-actions');
            const originalText = chunkDiv.dataset.originalText;
            
            // Create editor textarea
            const editor = document.createElement('textarea');
            editor.className = 'chunk-editor';
            editor.value = originalText;
            
            // Replace text content with editor
            textDiv.style.display = 'none';
            textDiv.parentNode.insertBefore(editor, textDiv.nextSibling);
            
            // Update action buttons
            actionsDiv.innerHTML = 
                '<button class="save-btn" onclick="saveChunk(\'' + chunkId + '\')">💾 Save</button>' +
                '<button class="cancel-btn" onclick="cancelEdit(\'' + chunkId + '\')">❌ Cancel</button>';
            
            // Add editing indicator
            const indicator = document.createElement('div');
            indicator.className = 'editing-indicator';
            indicator.textContent = '✏️ Editing chunk - changes will regenerate embeddings';
            chunkDiv.appendChild(indicator);
            
            // Focus the editor
            editor.focus();
            editor.select();
        }
        
        function saveChunk(chunkId) {
            const chunkDiv = document.querySelector('[data-chunk-id="' + chunkId + '"]');
            const editor = chunkDiv.querySelector('.chunk-editor');
            const newText = editor.value.trim();
            
            if (!newText) {
                alert('Chunk text cannot be empty');
                return;
            }
            
            // Show saving state
            const actionsDiv = chunkDiv.querySelector('.chunk-actions');
            actionsDiv.innerHTML = '<span style="color: #007bff;">💾 Saving...</span>';
            
            // Send update request
            fetch('/api/chunks/' + chunkId, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ text: newText })
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    // Update the chunk display
                    const textDiv = document.getElementById('text-' + chunkId);
                    textDiv.innerHTML = escapeHtml(newText).replace(/\n/g, '<br>');
                    textDiv.style.display = 'block';
                    
                    // Update stored original text
                    chunkDiv.dataset.originalText = newText;
                    
                    // Remove editor and indicator
                    editor.remove();
                    const indicator = chunkDiv.querySelector('.editing-indicator');
                    indicator && indicator.remove();
                    
                    // Restore edit button
                    actionsDiv.innerHTML = '<button class="edit-btn" onclick="editChunk(\'' + 
                        chunkId + '\')">✏️ Edit</button>';
                    
                    // Show success message briefly
                    const successMsg = document.createElement('div');
                    successMsg.style.cssText = 'position: fixed; top: 20px; right: 20px; ' + 
                        'background: #28a745; color: white; padding: 10px 20px; ' +
                        'border-radius: 4px; z-index: 1000;';
                    successMsg.textContent = '✅ Chunk updated and embeddings regenerated';
                    document.body.appendChild(successMsg);
                    setTimeout(() => successMsg.remove(), 3000);
                } else {
                    throw new Error(data.message || 'Failed to update chunk');
                }
            })
            .catch(error => {
                console.error('Error updating chunk:', error);
                alert('Error updating chunk: ' + error.message);
                
                // Restore edit buttons on error
                actionsDiv.innerHTML = 
                    '<button class="save-btn" onclick="saveChunk(\'' + chunkId + '\')">💾 Save</button>' +
                    '<button class="cancel-btn" onclick="cancelEdit(\'' + chunkId + '\')">❌ Cancel</button>';
            });
        }
        
        function cancelEdit(chunkId) {
            const chunkDiv = document.querySelector('[data-chunk-id="' + chunkId + '"]');
            const textDiv = document.getElementById('text-' + chunkId);
            const editor = chunkDiv.querySelector('.chunk-editor');
            const actionsDiv = chunkDiv.querySelector('.chunk-actions');
            
            // Remove editor and indicator
            editor.remove();
            chunkDiv.querySelector('.editing-indicator') && chunkDiv.querySelector('.editing-indicator').remove();
            
            // Show original text
            textDiv.style.display = 'block';
            
            // Restore edit button
            actionsDiv.innerHTML = '<button class="edit-btn" onclick="editChunk(\'' + chunkId + '\')">✏️ Edit</button>';
        }

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
		docInfo.ID,         // title
		docInfo.ID,         // delete button
		docInfo.ID,         // document title
		docInfo.DocType,    // type
		docInfo.ChunkCount, // chunks
		docInfo.SourcePath, // source
		docInfo.UpdatedAt.Format("2006-01-02 15:04:05"), // updated
		docInfo.ID,     // image src path
		highlightChunk, // highlightChunk JS variable
		docInfo.ID,     // fetch chunks URL
	)

	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Failed to write HTML response: %v", err)
	}
}
