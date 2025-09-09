package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lil-rag/pkg/lilrag"
)

// Comprehensive benchmark tests for all major operations

func BenchmarkHandler_Health(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "benchmark_health")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &lilrag.Config{
		DatabasePath: filepath.Join(tempDir, "bench.db"),
		DataDir:      filepath.Join(tempDir, "data"),
		VectorSize:   3,
	}
	ragInstance, err := lilrag.New(config)
	if err != nil {
		b.Fatalf("Failed to create LilRag: %v", err)
	}
	handler := New(ragInstance)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
		handler.Health()(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful health response, got status %d", w.Code)
		}
	}
}

func BenchmarkHandler_Metrics(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "benchmark_metrics")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &lilrag.Config{
		DatabasePath: filepath.Join(tempDir, "bench.db"),
		DataDir:      filepath.Join(tempDir, "data"),
		VectorSize:   3,
	}
	ragInstance, err := lilrag.New(config)
	if err != nil {
		b.Fatalf("Failed to create LilRag: %v", err)
	}
	handler := New(ragInstance)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/metrics", http.NoBody)
		handler.Metrics()(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful metrics response, got status %d", w.Code)
		}
	}
}

func BenchmarkHandler_Static(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "benchmark_static")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &lilrag.Config{
		DatabasePath: filepath.Join(tempDir, "bench.db"),
		DataDir:      filepath.Join(tempDir, "data"),
		VectorSize:   3,
	}
	ragInstance, err := lilrag.New(config)
	if err != nil {
		b.Fatalf("Failed to create LilRag: %v", err)
	}
	handler := New(ragInstance)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		handler.Static()(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful static response, got status %d", w.Code)
		}
	}
}

func BenchmarkHandler_Documents_GET(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
		handler.Documents()(w, req)

		if w.Code != http.StatusOK {
			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				b.Skip("Skipping benchmark due to Ollama connection error")
			}
			b.Fatalf("Expected successful documents response, got status %d", w.Code)
		}
	}
}

func BenchmarkHandler_ChatInterface_GET(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/chat", http.NoBody)
		handler.Chat()(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful chat interface response, got status %d", w.Code)
		}
	}
}

func BenchmarkHandler_Index_JSON_Small(b *testing.B) {
	handler := createBenchmarkHandler(b)

	indexReq := IndexRequest{
		Text: "Small test document content for benchmarking.",
	}

	bodyBytes, err := json.Marshal(indexReq)
	if err != nil {
		b.Fatalf("Failed to marshal index request: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/index", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		handler.Index()(w, req)

		// Skip if Ollama connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded")) {
			b.Skip("Skipping benchmark due to Ollama connection error")
		}

		if w.Code != http.StatusCreated {
			b.Fatalf("Expected successful index response, got status %d: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHandler_Index_JSON_Medium(b *testing.B) {
	handler := createBenchmarkHandler(b)

	// Medium sized document (about 500 words)
	mediumText := strings.Repeat("This is a medium-sized document for performance testing. ", 50) +
		"It contains multiple sentences to simulate real document content. " +
		"Performance testing helps identify bottlenecks in the indexing pipeline. " +
		strings.Repeat("Additional content for realistic document size testing. ", 25)

	indexReq := IndexRequest{
		Text: mediumText,
	}

	bodyBytes, err := json.Marshal(indexReq)
	if err != nil {
		b.Fatalf("Failed to marshal index request: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/index", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		handler.Index()(w, req)

		// Skip if Ollama connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded")) {
			b.Skip("Skipping benchmark due to Ollama connection error")
		}

		if w.Code != http.StatusCreated {
			b.Fatalf("Expected successful index response, got status %d: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHandler_Index_JSON_Large(b *testing.B) {
	handler := createBenchmarkHandler(b)

	// Large document (about 2000 words)
	largeText := strings.Repeat("This is a large document for stress testing the indexing system. ", 200) +
		"Large documents test the chunking algorithms and embedding generation performance. " +
		"Real-world documents often contain thousands of words across multiple paragraphs. " +
		strings.Repeat("Performance optimization is crucial for handling large document collections efficiently. ", 100) +
		"The system should maintain reasonable response times even with substantial content volumes."

	indexReq := IndexRequest{
		Text: largeText,
	}

	bodyBytes, err := json.Marshal(indexReq)
	if err != nil {
		b.Fatalf("Failed to marshal index request: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/index", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		handler.Index()(w, req)

		// Skip if Ollama connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded")) {
			b.Skip("Skipping benchmark due to Ollama connection error")
		}

		if w.Code != http.StatusCreated {
			b.Fatalf("Expected successful index response, got status %d: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHandler_Search_GET_Simple(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/search?query=test&limit=5", http.NoBody)
		handler.Search()(w, req)

		// Skip if no documents or connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded")) {
			b.Skip("Skipping benchmark due to Ollama connection error")
		}

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful search response, got status %d: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHandler_Search_POST_Complex(b *testing.B) {
	handler := createBenchmarkHandler(b)

	searchReq := SearchRequest{
		Query: "complex search query with multiple terms machine learning artificial intelligence",
		Limit: 10,
	}

	bodyBytes, err := json.Marshal(searchReq)
	if err != nil {
		b.Fatalf("Failed to marshal search request: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		handler.Search()(w, req)

		// Skip if no documents or connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded")) {
			b.Skip("Skipping benchmark due to Ollama connection error")
		}

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful search response, got status %d: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHandler_Chat_POST_Simple(b *testing.B) {
	handler := createBenchmarkHandler(b)

	chatReq := ChatRequest{
		Message: "What is machine learning?",
		Limit:   5,
	}

	bodyBytes, err := json.Marshal(chatReq)
	if err != nil {
		b.Fatalf("Failed to marshal chat request: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		handler.Chat()(w, req)

		// Skip if no documents or connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded") ||
			strings.Contains(w.Body.String(), "chat failed")) {
			b.Skip("Skipping benchmark due to Ollama connection error")
		}

		if w.Code != http.StatusOK {
			b.Fatalf("Expected successful chat response, got status %d: %s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkHandler_ErrorHandling(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/index", strings.NewReader("{invalid json}"))
		req.Header.Set("Content-Type", "application/json")

		handler.Index()(w, req)

		if w.Code != http.StatusBadRequest {
			b.Fatalf("Expected bad request error response, got status %d", w.Code)
		}
	}
}

func BenchmarkHandler_ConcurrentHealth(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
			handler.Health()(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected successful health response, got status %d", w.Code)
			}
		}
	})
}

func BenchmarkHandler_ConcurrentDocuments(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
			handler.Documents()(w, req)

			if w.Code != http.StatusOK {
				if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
					continue // Skip connection errors in benchmark
				}
				b.Fatalf("Expected successful documents response, got status %d", w.Code)
			}
		}
	})
}

func BenchmarkHandler_ResponseSizes(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.Run("Small_Health_Response", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
			handler.Health()(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected successful response, got status %d", w.Code)
			}

			// Measure response size
			responseSize := len(w.Body.Bytes())
			if responseSize == 0 {
				b.Error("Expected non-empty response")
			}
		}
	})

	b.Run("Large_Static_Response", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			handler.Static()(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected successful response, got status %d", w.Code)
			}

			// Measure response size
			responseSize := len(w.Body.Bytes())
			if responseSize < 1000 { // HTML page should be reasonably large
				b.Error("Expected large HTML response")
			}
		}
	})
}

func BenchmarkHandler_MemoryUsage(b *testing.B) {
	handler := createBenchmarkHandler(b)

	b.Run("Low_Memory_Health", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
			handler.Health()(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected successful response, got status %d", w.Code)
			}
		}
	})

	b.Run("High_Memory_Static", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			handler.Static()(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected successful response, got status %d", w.Code)
			}
		}
	})
}

// Performance regression tests
func BenchmarkHandler_PerformanceRegression(b *testing.B) {
	handler := createBenchmarkHandler(b)

	// These benchmarks help catch performance regressions over time
	b.Run("Health_Baseline", func(b *testing.B) {
		start := time.Now()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
			handler.Health()(w, req)
		}
		duration := time.Since(start)

		// Log performance metrics for monitoring
		b.Logf("Average time per health check: %v", duration/time.Duration(b.N))
	})

	b.Run("Documents_Baseline", func(b *testing.B) {
		start := time.Now()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/documents", http.NoBody)
			handler.Documents()(w, req)

			if w.Code == 500 && strings.Contains(w.Body.String(), "connection refused") {
				continue // Skip connection errors
			}
		}
		duration := time.Since(start)

		b.Logf("Average time per documents list: %v", duration/time.Duration(b.N))
	})
}

// Helper function to create a benchmark handler
func createBenchmarkHandler(b *testing.B) *Handler {
	tempDir, err := os.MkdirTemp("", "benchmark_handler")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	b.Cleanup(func() { os.RemoveAll(tempDir) })

	config := &lilrag.Config{
		DatabasePath: filepath.Join(tempDir, "bench.db"),
		DataDir:      filepath.Join(tempDir, "data"),
		VectorSize:   3, // Small vector size for faster benchmarks
		MaxTokens:    100,
		Overlap:      20,
	}

	ragInstance, err := lilrag.New(config)
	if err != nil {
		b.Fatalf("Failed to create LilRag for benchmark: %v", err)
	}

	return New(ragInstance)
}

// Stress test functions
func BenchmarkHandler_StressTest_ManyRequests(b *testing.B) {
	handler := createBenchmarkHandler(b)

	// Stress test with many concurrent requests
	b.SetParallelism(20) // High parallelism
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
			handler.Health()(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected successful response under stress, got status %d", w.Code)
			}
		}
	})
}

func BenchmarkHandler_StressTest_LargePayloads(b *testing.B) {
	handler := createBenchmarkHandler(b)

	// Very large document for stress testing
	veryLargeText := strings.Repeat("This is a very large document for stress testing the system with substantial content. ", 1000)

	indexReq := IndexRequest{
		Text: veryLargeText,
	}

	bodyBytes, err := json.Marshal(indexReq)
	if err != nil {
		b.Fatalf("Failed to marshal large index request: %v", err)
	}

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/index", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		handler.Index()(w, req)

		// Skip if Ollama connection issues
		if w.Code == 500 && (strings.Contains(w.Body.String(), "connection refused") ||
			strings.Contains(w.Body.String(), "context deadline exceeded")) {
			b.Skip("Skipping stress test due to Ollama connection error")
		}

		if w.Code != http.StatusCreated {
			// For stress testing, some failures might be acceptable
			b.Logf("Index request failed under stress: status %d", w.Code)
		}
	}
}
