package minirag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOllamaEmbedder(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		model       string
		expectError bool
	}{
		{
			name:        "valid parameters",
			baseURL:     "http://localhost:11434",
			model:       "nomic-embed-text",
			expectError: false,
		},
		{
			name:        "empty baseURL",
			baseURL:     "",
			model:       "nomic-embed-text",
			expectError: false, // Constructor doesn't validate empty URL
		},
		{
			name:        "empty model",
			baseURL:     "http://localhost:11434",
			model:       "",
			expectError: false, // Constructor doesn't validate empty model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedder, err := NewOllamaEmbedder(tt.baseURL, tt.model)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if embedder == nil {
					t.Error("Expected non-nil embedder")
				} else {
					if embedder.baseURL != tt.baseURL {
						t.Errorf("Expected baseURL %q, got %q", tt.baseURL, embedder.baseURL)
					}
					if embedder.model != tt.model {
						t.Errorf("Expected model %q, got %q", tt.model, embedder.model)
					}
					if embedder.client == nil {
						t.Error("Expected non-nil HTTP client")
					}
					if embedder.cache == nil {
						t.Error("Expected non-nil cache")
					}
					if embedder.cacheMaxSize != 1000 {
						t.Errorf("Expected cache max size 1000, got %d", embedder.cacheMaxSize)
					}
					if embedder.preprocessor == nil {
						t.Error("Expected non-nil preprocessor")
					}
				}
			}
		})
	}
}

func TestTextPreprocessor_preprocess(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "empty string",
			text:     "",
			expected: "",
		},
		{
			name:     "simple text",
			text:     "Hello world",
			expected: "Hello world",
		},
		{
			name:     "text with extra whitespace",
			text:     "  Hello    world  ",
			expected: "Hello world",
		},
		{
			name:     "text with newlines",
			text:     "Hello\n\nworld\t\ttest",
			expected: "Hello world test",
		},
		{
			name:     "text with unicode",
			text:     "Hello 世界 🌍",
			expected: "Hello 世界 🌍",
		},
		{
			name:     "text exceeding max length",
			text:     strings.Repeat("a", 10000),
			expected: strings.Repeat("a", 8192),
		},
		{
			name:     "long text with word boundary",
			text:     strings.Repeat("word ", 2000),
			expected: strings.TrimSpace(strings.Repeat("word ", 1638)), // Cut at word boundary
		},
	}

	preprocessor := &TextPreprocessor{
		normalizeWhitespace: true,
		removeExtraSpaces:   true,
		maxLength:           8192,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessor.preprocess(tt.text)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTextPreprocessor_preprocess_ConfigOptions(t *testing.T) {
	tests := []struct {
		name                string
		normalizeWhitespace bool
		removeExtraSpaces   bool
		maxLength           int
		input               string
		expected            string
	}{
		{
			name:                "whitespace normalization disabled",
			normalizeWhitespace: false,
			removeExtraSpaces:   true,
			maxLength:           100,
			input:               "  Hello\n\nworld  ",
			expected:            "Hello\n\nworld",
		},
		{
			name:                "extra spaces removal disabled",
			normalizeWhitespace: true,
			removeExtraSpaces:   false,
			maxLength:           100,
			input:               "  Hello\n\nworld  ",
			expected:            "  Hello world  ",
		},
		{
			name:                "no max length limit",
			normalizeWhitespace: true,
			removeExtraSpaces:   true,
			maxLength:           0,
			input:               strings.Repeat("a", 10000),
			expected:            strings.Repeat("a", 10000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preprocessor := &TextPreprocessor{
				normalizeWhitespace: tt.normalizeWhitespace,
				removeExtraSpaces:   tt.removeExtraSpaces,
				maxLength:           tt.maxLength,
			}

			result := preprocessor.preprocess(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Mock HTTP server for testing Ollama API
func createMockOllamaServer(t *testing.T, responses map[string][]float32, errorResponse bool, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			if t != nil {
				t.Errorf("Unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != "POST" {
			if t != nil {
				t.Errorf("Unexpected method: %s", r.Method)
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req OllamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if t != nil {
				t.Errorf("Failed to decode request: %v", err)
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if errorResponse {
			w.WriteHeader(statusCode)
			w.Write([]byte("Mock error response"))
			return
		}

		embedding, exists := responses[req.Prompt]
		if !exists {
			embedding = []float32{0.1, 0.2, 0.3} // Default embedding
		}

		response := OllamaEmbedResponse{
			Embedding: embedding,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func TestOllamaEmbedder_Embed(t *testing.T) {
	// Create mock responses
	mockResponses := map[string][]float32{
		"Hello world": {0.1, 0.2, 0.3},
		"Test text":   {0.4, 0.5, 0.6},
	}

	server := createMockOllamaServer(t, mockResponses, false, http.StatusOK)
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	tests := []struct {
		name        string
		text        string
		expectError bool
		expectedLen int
	}{
		{
			name:        "valid text",
			text:        "Hello world",
			expectError: false,
			expectedLen: 3,
		},
		{
			name:        "empty text",
			text:        "",
			expectError: true,
			expectedLen: 0,
		},
		{
			name:        "another valid text",
			text:        "Test text",
			expectError: false,
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			embedding, err := embedder.Embed(ctx, tt.text)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(embedding) != tt.expectedLen {
				t.Errorf("Expected embedding length %d, got %d", tt.expectedLen, len(embedding))
			}
		})
	}
}

func TestOllamaEmbedder_Embed_Caching(t *testing.T) {
	mockResponses := map[string][]float32{
		"cached text": {0.7, 0.8, 0.9},
	}

	server := createMockOllamaServer(t, mockResponses, false, http.StatusOK)
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx := context.Background()
	text := "cached text"

	// First call - should hit the server
	embedding1, err := embedder.Embed(ctx, text)
	if err != nil {
		t.Fatalf("First embed failed: %v", err)
	}

	// Second call - should hit the cache
	embedding2, err := embedder.Embed(ctx, text)
	if err != nil {
		t.Fatalf("Second embed failed: %v", err)
	}

	// Verify embeddings are the same
	if len(embedding1) != len(embedding2) {
		t.Errorf("Embedding lengths don't match: %d vs %d", len(embedding1), len(embedding2))
	}

	for i, v1 := range embedding1 {
		if v1 != embedding2[i] {
			t.Errorf("Embedding values don't match at index %d: %f vs %f", i, v1, embedding2[i])
		}
	}

	// Verify cache stats
	stats := embedder.GetCacheStats()
	if stats["cache_size"].(int) != 1 {
		t.Errorf("Expected cache size 1, got %v", stats["cache_size"])
	}
}

func TestOllamaEmbedder_EmbedQuery(t *testing.T) {
	mockResponses := map[string][]float32{
		"Find information about: test query": {1.0, 2.0, 3.0},
		"What is machine learning?":          {4.0, 5.0, 6.0},
	}

	server := createMockOllamaServer(t, mockResponses, false, http.StatusOK)
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:        "short query with enhancement",
			query:       "test query",
			expectError: false,
		},
		{
			name:        "question query without enhancement",
			query:       "What is machine learning?",
			expectError: false,
		},
		{
			name:        "empty query",
			query:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			embedding, err := embedder.EmbedQuery(ctx, tt.query)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && len(embedding) == 0 {
				t.Error("Expected non-empty embedding")
			}
		})
	}
}

func TestOllamaEmbedder_preprocessQuery(t *testing.T) {
	embedder, _ := NewOllamaEmbedder("http://localhost:11434", "test-model")

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "short query gets enhanced",
			query:    "machine learning",
			expected: "Find information about: machine learning",
		},
		{
			name:     "question query stays unchanged",
			query:    "What is machine learning?",
			expected: "What is machine learning?",
		},
		{
			name:     "long query stays unchanged",
			query:    "This is a very long query with many words that should not be enhanced",
			expected: "This is a very long query with many words that should not be enhanced",
		},
		{
			name:     "query with extra whitespace",
			query:    "  machine    learning  ",
			expected: "Find information about: machine learning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := embedder.preprocessQuery(tt.query)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestOllamaEmbedder_Error_Handling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantError  bool
	}{
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantError:  true,
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createMockOllamaServer(t, nil, true, tt.statusCode)
			defer server.Close()

			embedder, err := NewOllamaEmbedder(server.URL, "test-model")
			if err != nil {
				t.Fatalf("Failed to create embedder: %v", err)
			}

			ctx := context.Background()
			_, err = embedder.Embed(ctx, "test text")

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestOllamaEmbedder_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		response := OllamaEmbedResponse{
			Embedding: []float32{0.1, 0.2, 0.3},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	// Set very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = embedder.Embed(ctx, "test text")
	if err == nil {
		t.Error("Expected timeout error but got none")
	}
}

func TestOllamaEmbedder_RetryLogic(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server error"))
			return
		}
		// Succeed on third attempt
		response := OllamaEmbedResponse{
			Embedding: []float32{0.1, 0.2, 0.3},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx := context.Background()
	embedding, err := embedder.Embed(ctx, "test text")

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}
	if len(embedding) != 3 {
		t.Errorf("Expected embedding length 3, got %d", len(embedding))
	}
	if callCount != 3 {
		t.Errorf("Expected 3 API calls, got %d", callCount)
	}
}

func TestOllamaEmbedder_CacheManagement(t *testing.T) {
	server := createMockOllamaServer(t, make(map[string][]float32), false, http.StatusOK)
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	// Test initial cache stats
	stats := embedder.GetCacheStats()
	if stats["cache_size"].(int) != 0 {
		t.Errorf("Expected initial cache size 0, got %v", stats["cache_size"])
	}

	ctx := context.Background()

	// Add some items to cache
	for i := 0; i < 5; i++ {
		text := fmt.Sprintf("text %d", i)
		_, err := embedder.Embed(ctx, text)
		if err != nil {
			t.Fatalf("Failed to embed text: %v", err)
		}
	}

	// Check cache size
	stats = embedder.GetCacheStats()
	if stats["cache_size"].(int) != 5 {
		t.Errorf("Expected cache size 5, got %v", stats["cache_size"])
	}

	// Clear cache
	embedder.ClearCache()

	// Check cache is empty
	stats = embedder.GetCacheStats()
	if stats["cache_size"].(int) != 0 {
		t.Errorf("Expected cache size 0 after clear, got %v", stats["cache_size"])
	}
}

func TestOllamaEmbedder_CacheLRU(t *testing.T) {
	server := createMockOllamaServer(t, make(map[string][]float32), false, http.StatusOK)
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	// Set small cache size for testing
	embedder.cacheMaxSize = 3

	ctx := context.Background()

	// Fill cache to capacity
	for i := 0; i < 3; i++ {
		text := fmt.Sprintf("text %d", i)
		_, err := embedder.Embed(ctx, text)
		if err != nil {
			t.Fatalf("Failed to embed text: %v", err)
		}
	}

	stats := embedder.GetCacheStats()
	if stats["cache_size"].(int) != 3 {
		t.Errorf("Expected cache size 3, got %v", stats["cache_size"])
	}

	// Add one more item - should trigger LRU eviction
	_, err = embedder.Embed(ctx, "text 3")
	if err != nil {
		t.Fatalf("Failed to embed text: %v", err)
	}

	stats = embedder.GetCacheStats()
	if stats["cache_size"].(int) != 3 {
		t.Errorf("Expected cache size to stay 3 after LRU, got %v", stats["cache_size"])
	}
}

func TestOllamaEmbedder_EmptyEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := OllamaEmbedResponse{
			Embedding: []float32{}, // Empty embedding
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx := context.Background()
	_, err = embedder.Embed(ctx, "test text")

	if err == nil {
		t.Error("Expected error for empty embedding but got none")
	}
	if !strings.Contains(err.Error(), "empty embedding") {
		t.Errorf("Expected 'empty embedding' error, got: %v", err)
	}
}

func TestOllamaEmbedder_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	embedder, err := NewOllamaEmbedder(server.URL, "test-model")
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx := context.Background()
	_, err = embedder.Embed(ctx, "test text")

	if err == nil {
		t.Error("Expected error for invalid JSON but got none")
	}
}

// Benchmark tests
func BenchmarkOllamaEmbedder_Embed_WithCache(b *testing.B) {
	server := createMockOllamaServer(nil, map[string][]float32{
		"benchmark text": {0.1, 0.2, 0.3},
	}, false, http.StatusOK)
	defer server.Close()

	embedder, _ := NewOllamaEmbedder(server.URL, "test-model")
	ctx := context.Background()

	// Prime the cache
	embedder.Embed(ctx, "benchmark text")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := embedder.Embed(ctx, "benchmark text")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTextPreprocessor_preprocess(b *testing.B) {
	preprocessor := &TextPreprocessor{
		normalizeWhitespace: true,
		removeExtraSpaces:   true,
		maxLength:           8192,
	}

	text := "  This  is  a  test  text  with  extra  whitespace  "
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		preprocessor.preprocess(text)
	}
}

func BenchmarkOllamaEmbedder_preprocessQuery(b *testing.B) {
	embedder, _ := NewOllamaEmbedder("http://localhost:11434", "test-model")
	query := "machine learning algorithms"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		embedder.preprocessQuery(query)
	}
}
