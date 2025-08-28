package minirag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mock implementations for testing

type MockStorage struct {
	documents   map[string]string
	embeddings  map[string][]float32
	chunks      map[string][]Chunk
	initialized bool
	closed      bool
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		documents:  make(map[string]string),
		embeddings: make(map[string][]float32),
		chunks:     make(map[string][]Chunk),
	}
}

func (m *MockStorage) Initialize() error {
	m.initialized = true
	return nil
}

func (m *MockStorage) Index(ctx context.Context, id string, text string, embedding []float32) error {
	if !m.initialized {
		return fmt.Errorf("storage not initialized")
	}
	m.documents[id] = text
	m.embeddings[id] = embedding
	return nil
}

func (m *MockStorage) IndexChunks(ctx context.Context, documentID string, text string, chunks []Chunk, embeddings [][]float32) error {
	if !m.initialized {
		return fmt.Errorf("storage not initialized")
	}
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunks and embeddings count mismatch")
	}
	m.documents[documentID] = text
	m.chunks[documentID] = chunks
	// Store first embedding for search compatibility
	if len(embeddings) > 0 {
		m.embeddings[documentID] = embeddings[0]
	}
	return nil
}

func (m *MockStorage) Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error) {
	if !m.initialized {
		return nil, fmt.Errorf("storage not initialized")
	}

	var results []SearchResult
	for id, text := range m.documents {
		// Simple mock scoring - return all documents with decreasing score
		score := 1.0 - float64(len(results))*0.1
		if score < 0 {
			score = 0.1
		}
		
		result := SearchResult{
			ID:    id,
			Text:  text,
			Score: score,
			Metadata: map[string]interface{}{
				"mock": true,
			},
		}
		results = append(results, result)
		
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (m *MockStorage) Close() error {
	m.closed = true
	return nil
}

type MockEmbedder struct {
	embeddings map[string][]float32
}

func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{
		embeddings: make(map[string][]float32),
	}
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}
	
	// Return a mock embedding based on text length
	length := len(text)
	return []float32{
		float32(length%100) / 100.0,
		float32(length%200) / 200.0,
		float32(length%300) / 300.0,
	}, nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		checkConfig func(*Config) error
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name:        "empty config with defaults",
			config:      &Config{},
			expectError: false,
			checkConfig: func(c *Config) error {
				if c.DatabasePath != "minirag.db" {
					return fmt.Errorf("expected default database path 'minirag.db', got %q", c.DatabasePath)
				}
				if c.OllamaURL != "http://localhost:11434" {
					return fmt.Errorf("expected default Ollama URL 'http://localhost:11434', got %q", c.OllamaURL)
				}
				if c.Model != "nomic-embed-text" {
					return fmt.Errorf("expected default model 'nomic-embed-text', got %q", c.Model)
				}
				if c.VectorSize != 768 {
					return fmt.Errorf("expected default vector size 768, got %d", c.VectorSize)
				}
				if c.MaxTokens != 1800 {
					return fmt.Errorf("expected default max tokens 1800, got %d", c.MaxTokens)
				}
				if c.Overlap != 200 {
					return fmt.Errorf("expected default overlap 200, got %d", c.Overlap)
				}
				return nil
			},
		},
		{
			name: "custom config values preserved",
			config: &Config{
				DatabasePath: "custom.db",
				DataDir:      "/custom/data",
				OllamaURL:    "http://custom:11434",
				Model:        "custom-model",
				VectorSize:   384,
				MaxTokens:    1000,
				Overlap:      100,
			},
			expectError: false,
			checkConfig: func(c *Config) error {
				if c.DatabasePath != "custom.db" {
					return fmt.Errorf("expected custom database path 'custom.db', got %q", c.DatabasePath)
				}
				if c.DataDir != "/custom/data" {
					return fmt.Errorf("expected custom data dir '/custom/data', got %q", c.DataDir)
				}
				if c.OllamaURL != "http://custom:11434" {
					return fmt.Errorf("expected custom Ollama URL 'http://custom:11434', got %q", c.OllamaURL)
				}
				if c.VectorSize != 384 {
					return fmt.Errorf("expected custom vector size 384, got %d", c.VectorSize)
				}
				return nil
			},
		},
		{
			name: "partial config with defaults",
			config: &Config{
				DatabasePath: "partial.db",
				VectorSize:   512,
			},
			expectError: false,
			checkConfig: func(c *Config) error {
				if c.DatabasePath != "partial.db" {
					return fmt.Errorf("expected database path 'partial.db', got %q", c.DatabasePath)
				}
				if c.VectorSize != 512 {
					return fmt.Errorf("expected vector size 512, got %d", c.VectorSize)
				}
				if c.OllamaURL != "http://localhost:11434" {
					return fmt.Errorf("expected default Ollama URL, got %q", c.OllamaURL)
				}
				if c.Model != "nomic-embed-text" {
					return fmt.Errorf("expected default model, got %q", c.Model)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			miniRag, err := New(tt.config)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if miniRag == nil {
					t.Error("Expected non-nil MiniRag instance")
				} else {
					if miniRag.config == nil {
						t.Error("Expected non-nil config")
					} else if tt.checkConfig != nil {
						if err := tt.checkConfig(miniRag.config); err != nil {
							t.Errorf("Config validation failed: %v", err)
						}
					}

					// Verify components are not initialized yet
					if miniRag.storage != nil {
						t.Error("Expected storage to be nil before initialization")
					}
					if miniRag.embedder != nil {
						t.Error("Expected embedder to be nil before initialization")
					}
					if miniRag.chunker != nil {
						t.Error("Expected chunker to be nil before initialization")
					}
				}
			}
		})
	}
}

func TestMiniRag_Initialize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "minirag_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid initialization",
			config: &Config{
				DatabasePath: filepath.Join(tempDir, "test.db"),
				DataDir:      filepath.Join(tempDir, "data"),
				VectorSize:   3, // Small vector size for testing
			},
			expectError: false,
		},
		{
			name: "initialization with default data dir",
			config: &Config{
				DatabasePath: filepath.Join(tempDir, "test2.db"),
				VectorSize:   3,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			miniRag, err := New(tt.config)
			if err != nil {
				t.Fatalf("Failed to create MiniRag: %v", err)
			}

			err = miniRag.Initialize()

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				// Skip if sqlite-vec is not available
				if strings.Contains(err.Error(), "sqlite-vec extension not available") {
					t.Skip("Skipping test: sqlite-vec extension not available")
				}
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && err == nil {
				// Verify components were initialized
				if miniRag.storage == nil {
					t.Error("Expected storage to be initialized")
				}
				if miniRag.embedder == nil {
					t.Error("Expected embedder to be initialized")
				}
				if miniRag.chunker == nil {
					t.Error("Expected chunker to be initialized")
				}
				if miniRag.pdfParser == nil {
					t.Error("Expected PDF parser to be initialized")
				}

				// Verify default data directory was set
				if tt.config.DataDir == "" && miniRag.config.DataDir != "data" {
					t.Errorf("Expected default data dir 'data', got %q", miniRag.config.DataDir)
				}

				// Clean up
				miniRag.Close()
			}
		})
	}
}

func TestMiniRag_Index_WithMocks(t *testing.T) {
	miniRag := &MiniRag{
		storage:  NewMockStorage(),
		embedder: NewMockEmbedder(),
		chunker:  NewTextChunker(100, 20), // Small limits for testing
		config:   &Config{MaxTokens: 100},
	}

	err := miniRag.storage.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize mock storage: %v", err)
	}

	tests := []struct {
		name        string
		text        string
		id          string
		expectError bool
	}{
		{
			name:        "empty text",
			text:        "",
			id:          "doc1",
			expectError: true,
		},
		{
			name:        "empty id",
			text:        "Some text",
			id:          "",
			expectError: true,
		},
		{
			name:        "valid short text",
			text:        "Short text",
			id:          "doc2",
			expectError: false,
		},
		{
			name:        "long text requiring chunking",
			text:        strings.Repeat("This is a long text that will require chunking. ", 20),
			id:          "doc3",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := miniRag.Index(ctx, tt.text, tt.id)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify document was stored
				mockStorage := miniRag.storage.(*MockStorage)
				if _, exists := mockStorage.documents[tt.id]; !exists {
					t.Errorf("Document %s was not stored", tt.id)
				}
				if _, exists := mockStorage.embeddings[tt.id]; !exists {
					t.Errorf("Embedding for %s was not stored", tt.id)
				}
			}
		})
	}
}

func TestMiniRag_IndexFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "minirag_file_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	miniRag := &MiniRag{
		storage:   NewMockStorage(),
		embedder:  NewMockEmbedder(),
		chunker:   NewTextChunker(1000, 200),
		pdfParser: NewPDFParser(),
		config:    &Config{},
	}

	err = miniRag.storage.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize mock storage: %v", err)
	}

	// Create test files
	textFile := filepath.Join(tempDir, "test.txt")
	textContent := "This is a test text file content for indexing."
	err = os.WriteFile(textFile, []byte(textContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test text file: %v", err)
	}

	tests := []struct {
		name        string
		filePath    string
		id          string
		expectError bool
	}{
		{
			name:        "valid text file",
			filePath:    textFile,
			id:          "text-doc",
			expectError: false,
		},
		{
			name:        "nonexistent file",
			filePath:    filepath.Join(tempDir, "nonexistent.txt"),
			id:          "missing-doc",
			expectError: true,
		},
		{
			name:        "empty file path",
			filePath:    "",
			id:          "empty-path-doc",
			expectError: true,
		},
		{
			name:        "empty id",
			filePath:    textFile,
			id:          "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := miniRag.IndexFile(ctx, tt.filePath, tt.id)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify document was stored
				mockStorage := miniRag.storage.(*MockStorage)
				if storedText, exists := mockStorage.documents[tt.id]; !exists {
					t.Errorf("Document %s was not stored", tt.id)
				} else if storedText != textContent {
					t.Errorf("Stored text doesn't match file content:\nExpected: %q\nGot: %q", textContent, storedText)
				}
			}
		})
	}
}

func TestMiniRag_Search(t *testing.T) {
	miniRag := &MiniRag{
		storage:  NewMockStorage(),
		embedder: NewMockEmbedder(),
		config:   &Config{},
	}

	err := miniRag.storage.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize mock storage: %v", err)
	}

	ctx := context.Background()

	// Index some test documents
	testDocs := map[string]string{
		"doc1": "Machine learning and artificial intelligence",
		"doc2": "Natural language processing techniques",
		"doc3": "Computer vision and image recognition",
	}

	for id, text := range testDocs {
		err := miniRag.Index(ctx, text, id)
		if err != nil {
			t.Fatalf("Failed to index document %s: %v", id, err)
		}
	}

	tests := []struct {
		name            string
		query           string
		limit           int
		expectError     bool
		expectedResults int
	}{
		{
			name:            "valid query",
			query:           "machine learning",
			limit:           2,
			expectError:     false,
			expectedResults: 2,
		},
		{
			name:            "empty query",
			query:           "",
			limit:           5,
			expectError:     true,
			expectedResults: 0,
		},
		{
			name:            "zero limit defaults to 10",
			query:           "artificial intelligence",
			limit:           0,
			expectError:     false,
			expectedResults: 3, // All documents
		},
		{
			name:            "negative limit defaults to 10",
			query:           "computer vision",
			limit:           -1,
			expectError:     false,
			expectedResults: 3, // All documents
		},
		{
			name:            "limit larger than available documents",
			query:           "processing",
			limit:           10,
			expectError:     false,
			expectedResults: 3, // All documents
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := miniRag.Search(ctx, tt.query, tt.limit)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if len(results) != tt.expectedResults {
					t.Errorf("Expected %d results, got %d", tt.expectedResults, len(results))
				}

				// Verify result structure
				for i, result := range results {
					if result.ID == "" {
						t.Errorf("Result %d has empty ID", i)
					}
					if result.Text == "" {
						t.Errorf("Result %d has empty text", i)
					}
					if result.Score < 0 || result.Score > 1 {
						t.Errorf("Result %d has invalid score: %f", i, result.Score)
					}
					if result.Metadata == nil {
						t.Errorf("Result %d has nil metadata", i)
					}
				}
			}
		})
	}
}

func TestMiniRag_Close(t *testing.T) {
	tests := []struct {
		name        string
		setupRag    func() *MiniRag
		expectError bool
	}{
		{
			name: "close with initialized storage",
			setupRag: func() *MiniRag {
				mockStorage := NewMockStorage()
				mockStorage.Initialize()
				return &MiniRag{storage: mockStorage}
			},
			expectError: false,
		},
		{
			name: "close with nil storage",
			setupRag: func() *MiniRag {
				return &MiniRag{storage: nil}
			},
			expectError: false,
		},
		{
			name: "close uninitialized MiniRag",
			setupRag: func() *MiniRag {
				return &MiniRag{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			miniRag := tt.setupRag()
			err := miniRag.Close()

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify mock storage was closed if it existed
			if mockStorage, ok := miniRag.storage.(*MockStorage); ok {
				if !mockStorage.closed {
					t.Error("Expected mock storage to be closed")
				}
			}
		})
	}
}

// Integration-style test using real components (but mocked external dependencies)
func TestMiniRag_Integration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "minirag_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := &Config{
		DatabasePath: filepath.Join(tempDir, "test.db"),
		DataDir:      filepath.Join(tempDir, "data"),
		VectorSize:   3, // Small vector size for testing
		MaxTokens:    50,
		Overlap:      10,
	}

	miniRag, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create MiniRag: %v", err)
	}

	// Replace embedder with mock for predictable testing
	miniRag.embedder = NewMockEmbedder()

	// Initialize with real storage but mock embedder
	mockStorage := NewMockStorage()
	miniRag.storage = mockStorage
	
	err = miniRag.storage.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize other components
	miniRag.chunker = NewTextChunker(config.MaxTokens, config.Overlap)
	miniRag.pdfParser = NewPDFParser()

	defer miniRag.Close()

	ctx := context.Background()

	// Test indexing various types of content
	t.Run("index short text", func(t *testing.T) {
		err := miniRag.Index(ctx, "Short document about AI", "short-doc")
		if err != nil {
			t.Errorf("Failed to index short text: %v", err)
		}
	})

	t.Run("index long text requiring chunking", func(t *testing.T) {
		longText := strings.Repeat("This is a sentence about machine learning. ", 20)
		err := miniRag.Index(ctx, longText, "long-doc")
		if err != nil {
			t.Errorf("Failed to index long text: %v", err)
		}
	})

	t.Run("search indexed content", func(t *testing.T) {
		results, err := miniRag.Search(ctx, "machine learning", 5)
		if err != nil {
			t.Errorf("Failed to search: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected at least one search result")
		}

		// Verify we can find our indexed documents
		foundDocs := make(map[string]bool)
		for _, result := range results {
			foundDocs[result.ID] = true
		}

		expectedDocs := []string{"short-doc", "long-doc"}
		for _, expectedDoc := range expectedDocs {
			if !foundDocs[expectedDoc] {
				t.Errorf("Expected to find document %s in search results", expectedDoc)
			}
		}
	})

	t.Run("index text file", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")
		fileContent := "This is test file content for integration testing."
		err := os.WriteFile(testFile, []byte(fileContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err = miniRag.IndexFile(ctx, testFile, "file-doc")
		if err != nil {
			t.Errorf("Failed to index file: %v", err)
		}

		// Verify file was indexed
		results, err := miniRag.Search(ctx, "test file", 1)
		if err != nil {
			t.Errorf("Failed to search for file content: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected to find indexed file in search results")
		} else if results[0].ID != "file-doc" {
			t.Errorf("Expected file-doc in results, got %s", results[0].ID)
		}
	})
}

// Test error handling scenarios
func TestMiniRag_ErrorHandling(t *testing.T) {
	// Test with uninitialized MiniRag
	uninitializedRag := &MiniRag{}
	ctx := context.Background()

	t.Run("index without initialization", func(t *testing.T) {
		err := uninitializedRag.Index(ctx, "test", "doc1")
		if err == nil {
			t.Error("Expected error when indexing with uninitialized MiniRag")
		}
	})

	t.Run("search without initialization", func(t *testing.T) {
		_, err := uninitializedRag.Search(ctx, "query", 1)
		if err == nil {
			t.Error("Expected error when searching with uninitialized MiniRag")
		}
	})

	t.Run("index file without initialization", func(t *testing.T) {
		err := uninitializedRag.IndexFile(ctx, "test.txt", "doc1")
		if err == nil {
			t.Error("Expected error when indexing file with uninitialized MiniRag")
		}
	})

	t.Run("close without initialization", func(t *testing.T) {
		err := uninitializedRag.Close()
		if err != nil {
			t.Errorf("Close should not error on uninitialized MiniRag: %v", err)
		}
	})
}

// Benchmark tests
func BenchmarkMiniRag_Index_Short(b *testing.B) {
	miniRag := &MiniRag{
		storage:  NewMockStorage(),
		embedder: NewMockEmbedder(),
		chunker:  NewTextChunker(1000, 200),
		config:   &Config{},
	}
	miniRag.storage.Initialize()

	ctx := context.Background()
	text := "This is a short text for benchmarking"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("bench-doc-%d", i)
		err := miniRag.Index(ctx, text, id)
		if err != nil {
			b.Fatalf("Failed to index: %v", err)
		}
	}
}

func BenchmarkMiniRag_Index_Long(b *testing.B) {
	miniRag := &MiniRag{
		storage:  NewMockStorage(),
		embedder: NewMockEmbedder(),
		chunker:  NewTextChunker(100, 20), // Force chunking
		config:   &Config{},
	}
	miniRag.storage.Initialize()

	ctx := context.Background()
	text := strings.Repeat("This is a long text that will be chunked for benchmarking. ", 50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("bench-long-doc-%d", i)
		err := miniRag.Index(ctx, text, id)
		if err != nil {
			b.Fatalf("Failed to index: %v", err)
		}
	}
}

func BenchmarkMiniRag_Search(b *testing.B) {
	miniRag := &MiniRag{
		storage:  NewMockStorage(),
		embedder: NewMockEmbedder(),
		config:   &Config{},
	}
	miniRag.storage.Initialize()

	ctx := context.Background()

	// Pre-populate with test data
	for i := 0; i < 100; i++ {
		text := fmt.Sprintf("Document %d about various topics including machine learning", i)
		id := fmt.Sprintf("doc-%d", i)
		miniRag.Index(ctx, text, id)
	}

	query := "machine learning"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := miniRag.Search(ctx, query, 10)
		if err != nil {
			b.Fatalf("Failed to search: %v", err)
		}
	}
}