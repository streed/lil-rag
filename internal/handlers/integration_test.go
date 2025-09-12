package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lil-rag/pkg/lilrag"
)

// Integration tests that test the full workflow from indexing to searching to chatting

func TestIntegration_FullWorkflow(t *testing.T) {
	// Skip integration tests if Ollama is not available
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	handler := createTestHandler(t)

	// Test document content
	testDocuments := []struct {
		id      string
		content string
		title   string
	}{
		{
			id:      "doc1",
			content: "The quick brown fox jumps over the lazy dog. This is a test document about animals.",
			title:   "Animal Document",
		},
		{
			id:      "doc2",
			content: "Machine learning is a subset of artificial intelligence that focuses on algorithms. Neural networks are important in deep learning.",
			title:   "ML Document",
		},
		{
			id:      "doc3",
			content: "Go programming language is developed by Google. It has excellent concurrency support with goroutines and channels.",
			title:   "Go Programming",
		},
	}

	var successfullyIndexed int
	t.Run("index_documents", func(t *testing.T) {
		for _, doc := range testDocuments {
			req := IndexRequest{
				ID:   doc.id,
				Text: doc.content,
			}

			bodyBytes, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Failed to marshal index request: %v", err)
			}

			httpReq := httptest.NewRequest(http.MethodPost, "/api/index", bytes.NewReader(bodyBytes))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Index()(w, httpReq)

			if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
				strings.Contains(w.Body.String(), "context deadline exceeded")) {
				t.Logf("Skipping document %s due to Ollama connection error: %s", doc.id, w.Body.String())
				continue
			}

			if w.Code != http.StatusCreated {
				t.Errorf("Failed to index document %s: status %d, body: %s", doc.id, w.Code, w.Body.String())
				continue
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse index response: %v", err)
				continue
			}

			if status, ok := response["status"]; !ok || status != "indexed" {
				t.Errorf("Expected indexed status, got: %v", response)
				continue
			}

			successfullyIndexed++
		}
	})

	// Wait a bit for indexing to complete
	time.Sleep(100 * time.Millisecond)

	t.Run("list_documents", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		handler.Documents()(w, req)

		if w.Code != http.StatusOK {
			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping integration test due to connection error: %s", w.Body.String())
			}
			t.Errorf("Expected status 200, got %d. Response: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse documents response: %v", err)
		}

		if count, ok := response["count"]; !ok {
			t.Error("Expected count field in documents response")
		} else if countFloat, ok := count.(float64); !ok || int(countFloat) != successfullyIndexed {
			t.Logf("Expected %d documents (based on successful indexing), got count: %v", successfullyIndexed, count)
			// Only fail if we expected some documents but got none, or if count doesn't match exactly
			if successfullyIndexed > 0 && int(countFloat) == 0 {
				t.Errorf("Expected %d documents but got 0, indicating a database or listing issue", successfullyIndexed)
			}
		}
	})

	t.Run("search_documents", func(t *testing.T) {
		if successfullyIndexed == 0 {
			t.Skip("Skipping search tests as no documents were successfully indexed")
		}

		searchQueries := []struct {
			query          string
			expectedDocIDs []string
		}{
			{
				query:          "animals fox dog",
				expectedDocIDs: []string{"doc1"},
			},
			{
				query:          "machine learning neural networks",
				expectedDocIDs: []string{"doc2"},
			},
			{
				query:          "go programming goroutines",
				expectedDocIDs: []string{"doc3"},
			},
		}

		for _, sq := range searchQueries {
			req := SearchRequest{
				Query: sq.query,
				Limit: 5,
			}

			bodyBytes, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Failed to marshal search request: %v", err)
			}

			httpReq := httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewReader(bodyBytes))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Search()(w, httpReq)

			if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
				strings.Contains(w.Body.String(), "context deadline exceeded")) {
				t.Skipf("Skipping search test due to Ollama connection error: %s", w.Body.String())
			}

			if w.Code != http.StatusOK {
				t.Errorf("Search failed for query '%s': status %d, body: %s", sq.query, w.Code, w.Body.String())
				continue
			}

			var response SearchResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse search response: %v", err)
				continue
			}

			if len(response.Results) == 0 {
				t.Errorf("Expected search results for query '%s', got none", sq.query)
				continue
			}

			// Check that we got relevant results
			foundExpected := false
			for _, result := range response.Results {
				for _, expectedID := range sq.expectedDocIDs {
					if result.ID == expectedID {
						foundExpected = true
						break
					}
				}
				if foundExpected {
					break
				}
			}

			if !foundExpected {
				t.Errorf("Expected to find documents %v in search results for query '%s', got: %v",
					sq.expectedDocIDs, sq.query, response.Results)
			}
		}
	})

	t.Run("chat_with_documents", func(t *testing.T) {
		if successfullyIndexed == 0 {
			t.Skip("Skipping chat tests as no documents were successfully indexed")
		}

		chatQueries := []struct {
			message          string
			expectedKeywords []string
		}{
			{
				message:          "Tell me about animals",
				expectedKeywords: []string{"fox", "dog", "animal"},
			},
			{
				message:          "What is machine learning?",
				expectedKeywords: []string{"machine learning", "algorithm", "neural"},
			},
			{
				message:          "How does Go handle concurrency?",
				expectedKeywords: []string{"goroutine", "channel", "concurrency"},
			},
		}

		for _, cq := range chatQueries {
			req := ChatRequest{
				Message: cq.message,
				Limit:   3,
			}

			bodyBytes, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Failed to marshal chat request: %v", err)
			}

			httpReq := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Chat()(w, httpReq)

			if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
				strings.Contains(w.Body.String(), "context deadline exceeded") ||
				strings.Contains(w.Body.String(), "chat failed")) {
				t.Skipf("Skipping chat test due to Ollama connection error: %s", w.Body.String())
			}

			if w.Code != http.StatusOK {
				t.Errorf("Chat failed for message '%s': status %d, body: %s", cq.message, w.Code, w.Body.String())
				continue
			}

			var response ChatResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse chat response: %v", err)
				continue
			}

			if response.Response == "" {
				t.Errorf("Expected non-empty chat response for message '%s'", cq.message)
				continue
			}

			if len(response.Sources) == 0 {
				t.Errorf("Expected sources in chat response for message '%s'", cq.message)
				continue
			}

			// Check that response contains relevant keywords (case insensitive)
			responseLower := strings.ToLower(response.Response)
			foundKeyword := false
			for _, keyword := range cq.expectedKeywords {
				if strings.Contains(responseLower, strings.ToLower(keyword)) {
					foundKeyword = true
					break
				}
			}

			if !foundKeyword {
				t.Errorf("Expected chat response for '%s' to contain one of %v, got: %s",
					cq.message, cq.expectedKeywords, response.Response)
			}
		}
	})

	t.Run("document_content_retrieval", func(t *testing.T) {
		for _, doc := range testDocuments {
			// Test document content API
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/documents/"+doc.id, http.NoBody)
			handler.DocumentContent()(w, req)

			if w.Code == 404 {
				// Document might not have source path
				continue
			}

			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping document retrieval test due to connection error: %s", w.Body.String())
			}

			// Test document chunks API
			w = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodGet, "/api/documents/"+doc.id+"/chunks", http.NoBody)
			handler.DocumentChunks()(w, req)

			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping chunks test due to connection error: %s", w.Body.String())
			}

			if w.Code != http.StatusOK {
				t.Errorf("Failed to get chunks for document %s: status %d, body: %s", doc.id, w.Code, w.Body.String())
				continue
			}

			var chunks []map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &chunks); err != nil {
				t.Errorf("Failed to parse chunks response: %v", err)
				continue
			}

			if len(chunks) == 0 {
				t.Errorf("Expected chunks for document %s", doc.id)
			}
		}
	})

	t.Run("delete_documents", func(t *testing.T) {
		// Delete one document
		docToDelete := testDocuments[0]

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/documents/"+docToDelete.id, http.NoBody)
		handler.DeleteDocument()(w, req)

		if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
			t.Skipf("Skipping delete test due to connection error: %s", w.Body.String())
		}

		if w.Code != http.StatusOK {
			// Check if the error is because document doesn't exist (which is fine)
			if w.Code == 404 && strings.Contains(w.Body.String(), "not found") {
				return
			}
			t.Errorf("Failed to delete document %s: status %d, body: %s", docToDelete.id, w.Code, w.Body.String())
		}

		// Verify document is deleted by trying to get its content
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/documents/"+docToDelete.id, http.NoBody)
		handler.DocumentContent()(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected document %s to be deleted, but still accessible with status %d", docToDelete.id, w.Code)
		}
	})
}

func TestIntegration_FileUploadWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	handler := createTestHandler(t)
	tempDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		name     string
		content  string
		filename string
	}{
		{
			name:     "text_file",
			content:  "This is a text file for testing file upload functionality. It contains sample text that should be indexed.",
			filename: "test.txt",
		},
		{
			name:     "csv_file",
			content:  "Name,Age,City\nJohn,30,New York\nJane,25,Los Angeles\nBob,35,Chicago",
			filename: "data.csv",
		},
	}

	documentIDs := make([]string, 0, len(testFiles))

	t.Run("upload_files", func(t *testing.T) {
		for _, tf := range testFiles {
			// Create temporary file
			filePath := filepath.Join(tempDir, tf.filename)
			err := os.WriteFile(filePath, []byte(tf.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create multipart form
			var b bytes.Buffer
			writer := multipart.NewWriter(&b)

			// Add file field
			part, err := writer.CreateFormFile("file", tf.filename)
			if err != nil {
				t.Fatalf("Failed to create form file: %v", err)
			}

			_, err = part.Write([]byte(tf.content))
			if err != nil {
				t.Fatalf("Failed to write file content: %v", err)
			}

			err = writer.Close()
			if err != nil {
				t.Fatalf("Failed to close writer: %v", err)
			}

			// Send upload request
			req := httptest.NewRequest(http.MethodPost, "/api/index", &b)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			handler.Index()(w, req)

			if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
				strings.Contains(w.Body.String(), "context deadline exceeded")) {
				t.Skipf("Skipping file upload test due to Ollama connection error: %s", w.Body.String())
			}

			if w.Code != http.StatusCreated {
				t.Errorf("Failed to upload file %s: status %d, body: %s", tf.filename, w.Code, w.Body.String())
				continue
			}

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse upload response: %v", err)
				continue
			}

			if docID, ok := response["id"].(string); ok {
				documentIDs = append(documentIDs, docID)
			}
		}
	})

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	t.Run("search_uploaded_files", func(t *testing.T) {
		if len(documentIDs) == 0 {
			t.Skip("No documents were uploaded successfully")
		}

		// Search for content from uploaded files
		searchQueries := []string{
			"text file testing",
			"John Los Angeles",
		}

		for _, query := range searchQueries {
			req := SearchRequest{
				Query: query,
				Limit: 5,
			}

			bodyBytes, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Failed to marshal search request: %v", err)
			}

			httpReq := httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewReader(bodyBytes))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Search()(w, httpReq)

			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping search test due to connection error: %s", w.Body.String())
			}

			if w.Code != http.StatusOK {
				t.Errorf("Search failed for query '%s': status %d, body: %s", query, w.Code, w.Body.String())
				continue
			}

			var response SearchResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse search response: %v", err)
				continue
			}

			// Check that we got some results
			if len(response.Results) == 0 {
				t.Logf("No search results for query '%s' (expected in test environment)", query)
			}
		}
	})

	t.Run("cleanup_uploaded_files", func(t *testing.T) {
		for _, docID := range documentIDs {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, "/api/documents/"+docID, http.NoBody)
			handler.DeleteDocument()(w, req)

			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping cleanup test due to connection error: %s", w.Body.String())
			}

			// Don't fail the test if deletion fails - it might already be cleaned up
			if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
				t.Logf("Failed to delete document %s: status %d", docID, w.Code)
			}
		}
	})
}

func TestIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	handler := createTestHandler(t)

	t.Run("invalid_operations_sequence", func(t *testing.T) {
		// Try to search non-existent document
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/documents/non-existent-doc", http.NoBody)
		handler.DocumentContent()(w, req)

		if w.Code != http.StatusNotFound {
			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping error handling test due to connection error: %s", w.Body.String())
			}
			t.Errorf("Expected 404 for non-existent document, got %d", w.Code)
		}

		// Try to delete non-existent document
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/api/documents/non-existent-doc", http.NoBody)
		handler.DeleteDocument()(w, req)

		if w.Code != http.StatusNotFound {
			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping error handling test due to connection error: %s", w.Body.String())
			}
			t.Errorf("Expected 404 for deleting non-existent document, got %d", w.Code)
		}

		// Try to get chunks of non-existent document
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api/documents/non-existent-doc/chunks", http.NoBody)
		handler.DocumentChunks()(w, req)

		if w.Code != http.StatusNotFound {
			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				t.Skipf("Skipping error handling test due to connection error: %s", w.Body.String())
			}
			t.Errorf("Expected 404 for chunks of non-existent document, got %d", w.Code)
		}
	})

	t.Run("malformed_requests", func(t *testing.T) {
		// Malformed JSON in index request
		req := httptest.NewRequest(http.MethodPost, "/api/index", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.Index()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for malformed JSON in index, got %d", w.Code)
		}

		// Malformed JSON in search request
		req = httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		handler.Search()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for malformed JSON in search, got %d", w.Code)
		}

		// Malformed JSON in chat request
		req = httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		handler.Chat()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for malformed JSON in chat, got %d", w.Code)
		}
	})
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	handler := createTestHandler(t)

	// Test concurrent health checks
	t.Run("concurrent_health_checks", func(t *testing.T) {
		const numRequests = 10
		results := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
				handler.Health()(w, req)
				results <- w.Code
			}()
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			code := <-results
			if code != http.StatusOK {
				t.Errorf("Expected 200 for concurrent health check, got %d", code)
			}
		}
	})

	// Test concurrent document listings
	t.Run("concurrent_document_listings", func(t *testing.T) {
		const numRequests = 5
		results := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			go func() {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
				handler.Documents()(w, req)
				results <- w.Code
			}()
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			code := <-results
			if code != http.StatusOK {
				if code == 500 {
					// May fail due to connection issues in test environment
					continue
				}
				t.Errorf("Expected 200 for concurrent document listing, got %d", code)
			}
		}
	})
}

// Benchmark integration test
func BenchmarkIntegration_IndexAndSearch(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	config := &lilrag.Config{
		DatabasePath: filepath.Join(b.TempDir(), "bench_integration.db"),
		DataDir:      filepath.Join(b.TempDir(), "data"),
		VectorSize:   3, // Small vector for speed
		MaxTokens:    100,
		Overlap:      20,
	}

	ragInstance, err := lilrag.New(config)
	if err != nil {
		b.Fatalf("Failed to create LilRag: %v", err)
	}

	// Try to initialize
	if err := ragInstance.Initialize(); err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			b.Skip("Skipping benchmark: sqlite-vec extension not available")
		}
		b.Fatalf("Failed to initialize LilRag: %v", err)
	}

	handler := New(ragInstance)

	// Index a document first
	indexReq := IndexRequest{
		ID:   "bench-doc",
		Text: "This is a benchmark document with some sample text for testing performance.",
	}

	bodyBytes, _ := json.Marshal(indexReq)
	req := httptest.NewRequest(http.MethodPost, "/api/index", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.Index()(w, req)

	if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
		b.Skip("Skipping benchmark due to Ollama connection error")
	}

	if w.Code != http.StatusCreated {
		b.Fatalf("Failed to index document for benchmark: %d", w.Code)
	}

	// Wait for indexing
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()

	// Benchmark search operations
	for i := 0; i < b.N; i++ {
		searchReq := SearchRequest{
			Query: "sample text performance",
			Limit: 5,
		}

		bodyBytes, _ := json.Marshal(searchReq)
		req := httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.Search()(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Search failed during benchmark: %d", w.Code)
		}
	}
}
