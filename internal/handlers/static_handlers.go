package handlers

import (
	"fmt"
	"log"
	"net/http"

	"lil-rag/internal/theme"
)

// Static serves the home page at /
func (h *Handler) Static() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Use new template system if available
		if h.renderer != nil {
			data := &theme.TemplateData{
				Title:       "Home",
				Version:     h.version,
				PageName:    "home",
				PageContent: "home.html",
			}
			
			w.Header().Set("Content-Type", "text/html")
			if err := h.renderer.RenderPage(w, "base.html", data); err != nil {
				log.Printf("Template rendering error: %v", err)
				h.fallbackHomePage(w, r)
			}
			return
		}
		
		// Fallback to original HTML
		h.fallbackHomePage(w, r)
	}
}

// fallbackHomePage serves the original home page HTML as fallback
func (h *Handler) fallbackHomePage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lil-RAG - Simple RAG System</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 1000px; margin: 0 auto; padding: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Lil-RAG</h1>
        <p>A simple yet powerful RAG system built with Go, SQLite, and Ollama</p>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		fmt.Printf("Error writing response: %v\n", err)
	}
}

// DocumentsList serves a web page with a table view of all documents at /documents
func (h *Handler) DocumentsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Use new template system if available
		if h.renderer != nil {
			data := &theme.TemplateData{
				Title:       "Documents",
				Version:     h.version,
				PageName:    "documents",
				PageContent: "documents.html",
			}
			
			w.Header().Set("Content-Type", "text/html")
			if err := h.renderer.RenderPage(w, "base.html", data); err != nil {
				log.Printf("Template rendering error: %v", err)
				h.fallbackDocumentsPage(w, r)
			}
			return
		}
		
		// Fallback to original HTML
		h.fallbackDocumentsPage(w, r)
	}
}

// fallbackDocumentsPage serves the original documents page HTML as fallback  
func (h *Handler) fallbackDocumentsPage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Documents - LilRag</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 1000px; margin: 0 auto; padding: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📚 Documents</h1>
        <p>Document management interface</p>
        <div id="documents-container">Loading...</div>
    </div>
    <script>
        // Simple JavaScript to load documents
        async function loadDocuments() {
            try {
                const response = await fetch('/api/documents');
                const data = await response.json();
                const container = document.getElementById('documents-container');
                if (data.documents && data.documents.length > 0) {
                    container.innerHTML = '<ul>' + data.documents.map(doc => 
                        '<li>' + doc.id + ' (' + (doc.chunk_count || 0) + ' chunks)</li>'
                    ).join('') + '</ul>';
                } else {
                    container.innerHTML = '<p>No documents found.</p>';
                }
            } catch (error) {
                document.getElementById('documents-container').innerHTML = 
                    '<p>Error loading documents: ' + error.message + '</p>';
            }
        }
        loadDocuments();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		log.Printf("Failed to write HTML response: %v", err)
	}
}

// Documentation serves the documentation page at /docs
func (h *Handler) Documentation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET method is allowed")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lil-RAG Documentation</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 1000px; margin: 0 auto; padding: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Lil-RAG Documentation</h1>
        <p>Complete documentation for Lil-RAG - A simple yet powerful RAG system.</p>
        <p>Version: ` + h.version + `</p>
    </div>
</body>
</html>`

		if _, err := w.Write([]byte(html)); err != nil {
			h.writeError(w, http.StatusInternalServerError, "write_error", "Failed to write response")
			return
		}
	}
}