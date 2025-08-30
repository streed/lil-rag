package minirag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSQLiteStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "storage_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		path        string
		vectorSize  int
		dataDir     string
		expectError bool
	}{
		{
			name:        "valid parameters",
			path:        filepath.Join(tempDir, "test.db"),
			vectorSize:  768,
			dataDir:     filepath.Join(tempDir, "data"),
			expectError: false,
		},
		{
			name:        "zero vector size",
			path:        filepath.Join(tempDir, "test2.db"),
			vectorSize:  0,
			dataDir:     filepath.Join(tempDir, "data2"),
			expectError: false, // Constructor doesn't validate vector size
		},
		{
			name:        "nested data directory",
			path:        filepath.Join(tempDir, "test3.db"),
			vectorSize:  384,
			dataDir:     filepath.Join(tempDir, "nested", "data"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewSQLiteStorage(tt.path, tt.vectorSize, tt.dataDir)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if storage == nil {
					t.Error("Expected non-nil storage")
				} else {
					if storage.path != tt.path {
						t.Errorf("Expected path %q, got %q", tt.path, storage.path)
					}
					if storage.vectorSize != tt.vectorSize {
						t.Errorf("Expected vector size %d, got %d", tt.vectorSize, storage.vectorSize)
					}
					if storage.dataDir != tt.dataDir {
						t.Errorf("Expected data dir %q, got %q", tt.dataDir, storage.dataDir)
					}
					if storage.db != nil {
						t.Error("Expected nil db before initialization")
					}

					// Verify data directory was created
					if _, err := os.Stat(tt.dataDir); os.IsNotExist(err) {
						t.Errorf("Expected data directory to be created: %s", tt.dataDir)
					}
				}
			}
		})
	}
}

func setupTestStorage(t *testing.T) (*SQLiteStorage, string) {
	tempDir, err := os.MkdirTemp("", "storage_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	dataDir := filepath.Join(tempDir, "data")

	storage, err := NewSQLiteStorage(dbPath, 3, dataDir) // Using small vector size for testing
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	return storage, tempDir
}

func TestSQLiteStorage_Initialize(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		// If sqlite-vec is not available, skip this test
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Verify database connection is established
	if storage.db == nil {
		t.Error("Expected db to be initialized")
	}

	// Verify tables were created by querying them
	tables := []string{"documents", "chunks", "embeddings"}
	for _, table := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		err := storage.db.QueryRow(query, table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		}
	}
}

func TestSQLiteStorage_generateContentHash(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name     string
		text     string
		expected string // We'll check length and format rather than exact value
	}{
		{
			name: "simple text",
			text: "Hello world",
		},
		{
			name: "empty text",
			text: "",
		},
		{
			name: "long text",
			text: strings.Repeat("a", 1000),
		},
		{
			name: "text with special characters",
			text: "Hello, 世界! 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := storage.generateContentHash(tt.text)

			// Check hash format (should be 64 character hex string)
			if len(hash) != 64 {
				t.Errorf("Expected hash length 64, got %d", len(hash))
			}

			// Check if it's valid hex
			for _, char := range hash {
				if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
					t.Errorf("Invalid hex character in hash: %c", char)
					break
				}
			}

			// Same input should produce same hash
			hash2 := storage.generateContentHash(tt.text)
			if hash != hash2 {
				t.Error("Same input should produce same hash")
			}
		})
	}

	// Different inputs should produce different hashes
	hash1 := storage.generateContentHash("text1")
	hash2 := storage.generateContentHash("text2")
	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestSQLiteStorage_storeContent(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		id          string
		text        string
		expectError bool
	}{
		{
			name:        "valid content",
			id:          "doc1",
			text:        "This is test content",
			expectError: false,
		},
		{
			name:        "empty content",
			id:          "doc2",
			text:        "",
			expectError: false,
		},
		{
			name:        "large content",
			id:          "doc3",
			text:        strings.Repeat("Large content ", 1000),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentHash := storage.generateContentHash(tt.text)
			filePath, err := storage.storeContent(tt.id, tt.text, contentHash)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify file was created
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("Expected file to be created: %s", filePath)
				}

				// Verify file path format
				expectedSuffix := ".txt.gz"
				if !strings.HasSuffix(filePath, expectedSuffix) {
					t.Errorf("Expected file path to end with %s, got %s", expectedSuffix, filePath)
				}

				// Verify file contains compressed data (should be different from original)
				data, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("Failed to read stored file: %v", err)
				} else {
					// For non-empty text, compressed data should be different from original
					if tt.text != "" && string(data) == tt.text {
						t.Error("File should contain compressed data, not original text")
					}

					// Verify we can decompress it back
					decompressed, err := DecompressText(data)
					if err != nil {
						t.Errorf("Failed to decompress stored content: %v", err)
					} else if decompressed != tt.text {
						t.Errorf("Decompressed content doesn't match original:\nOriginal: %q\nDecompressed: %q", tt.text, decompressed)
					}
				}
			}
		})
	}
}

func TestSQLiteStorage_Index_BackwardCompatibility(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	documentID := "test-doc"
	text := "This is a test document for backward compatibility"
	embedding := []float32{0.1, 0.2, 0.3}

	err = storage.Index(ctx, documentID, text, embedding)
	if err != nil {
		t.Fatalf("Failed to index document: %v", err)
	}

	// Verify document was stored
	var storedDocID string
	var chunkCount int
	err = storage.db.QueryRowContext(ctx, "SELECT id, chunk_count FROM documents WHERE id = ?", documentID).Scan(&storedDocID, &chunkCount)
	if err != nil {
		t.Errorf("Document not found in database: %v", err)
	} else {
		if storedDocID != documentID {
			t.Errorf("Expected document ID %s, got %s", documentID, storedDocID)
		}
		if chunkCount != 1 {
			t.Errorf("Expected chunk count 1, got %d", chunkCount)
		}
	}

	// Verify chunk was stored
	var chunkID string
	var docID string
	err = storage.db.QueryRowContext(ctx, "SELECT chunk_id, document_id FROM chunks WHERE document_id = ?", documentID).Scan(&chunkID, &docID)
	if err != nil {
		t.Errorf("Chunk not found in database: %v", err)
	} else {
		expectedChunkID := GetChunkID(documentID, 0)
		if chunkID != expectedChunkID {
			t.Errorf("Expected chunk ID %s, got %s", expectedChunkID, chunkID)
		}
	}

	// Verify embedding was stored
	var storedChunkID string
	err = storage.db.QueryRowContext(ctx, "SELECT chunk_id FROM embeddings WHERE chunk_id = ?", GetChunkID(documentID, 0)).Scan(&storedChunkID)
	if err != nil {
		t.Errorf("Embedding not found in database: %v", err)
	}
}

func TestSQLiteStorage_IndexChunks(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	documentID := "test-chunks-doc"
	text := "This is the original document text that was chunked"

	chunks := []Chunk{
		{
			Text:       "This is chunk 0",
			Index:      0,
			StartPos:   0,
			EndPos:     15,
			TokenCount: 4,
			ChunkType:  "text",
		},
		{
			Text:       "This is chunk 1",
			Index:      1,
			StartPos:   16,
			EndPos:     31,
			TokenCount: 4,
			ChunkType:  "text",
		},
	}

	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	err = storage.IndexChunks(ctx, documentID, text, chunks, embeddings)
	if err != nil {
		t.Fatalf("Failed to index chunks: %v", err)
	}

	// Verify document was stored with correct chunk count
	var storedDocID string
	var chunkCount int
	err = storage.db.QueryRowContext(ctx, "SELECT id, chunk_count FROM documents WHERE id = ?", documentID).Scan(&storedDocID, &chunkCount)
	if err != nil {
		t.Errorf("Document not found in database: %v", err)
	} else {
		if chunkCount != 2 {
			t.Errorf("Expected chunk count 2, got %d", chunkCount)
		}
	}

	// Verify chunks were stored
	rows, err := storage.db.QueryContext(ctx, "SELECT chunk_id, chunk_index FROM chunks WHERE document_id = ? ORDER BY chunk_index", documentID)
	if err != nil {
		t.Fatalf("Failed to query chunks: %v", err)
	}
	defer rows.Close()

	var storedChunks []struct {
		ID    string
		Index int
	}

	for rows.Next() {
		var chunkID string
		var index int
		err := rows.Scan(&chunkID, &index)
		if err != nil {
			t.Errorf("Failed to scan chunk row: %v", err)
		}
		storedChunks = append(storedChunks, struct {
			ID    string
			Index int
		}{chunkID, index})
	}

	if err := rows.Err(); err != nil {
		t.Errorf("Error iterating over rows: %v", err)
	}

	if len(storedChunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(storedChunks))
	}

	// Verify embeddings were stored
	for i := 0; i < 2; i++ {
		var count int
		chunkID := GetChunkID(documentID, i)
		err := storage.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM embeddings WHERE chunk_id = ?", chunkID).Scan(&count)
		if err != nil {
			t.Errorf("Failed to query embedding for chunk %d: %v", i, err)
		} else if count != 1 {
			t.Errorf("Expected 1 embedding for chunk %d, got %d", i, count)
		}
	}
}

func TestSQLiteStorage_IndexChunks_MismatchedCounts(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	documentID := "mismatch-doc"
	text := "Test document"

	chunks := []Chunk{
		{Text: "Chunk 1", Index: 0},
		{Text: "Chunk 2", Index: 1},
	}

	embeddings := [][]float32{
		{0.1, 0.2, 0.3}, // Only one embedding for two chunks
	}

	err = storage.IndexChunks(ctx, documentID, text, chunks, embeddings)
	if err == nil {
		t.Error("Expected error for mismatched chunk and embedding counts")
	}

	if !strings.Contains(err.Error(), "chunk count") || !strings.Contains(err.Error(), "embedding count") {
		t.Errorf("Expected error message about mismatched counts, got: %v", err)
	}
}

func TestSQLiteStorage_Search(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Index some test documents
	documents := []struct {
		id        string
		text      string
		embedding []float32
	}{
		{
			id:        "doc1",
			text:      "This is about machine learning and AI",
			embedding: []float32{0.8, 0.1, 0.1},
		},
		{
			id:        "doc2",
			text:      "This discusses natural language processing",
			embedding: []float32{0.1, 0.8, 0.1},
		},
		{
			id:        "doc3",
			text:      "This covers computer vision topics",
			embedding: []float32{0.1, 0.1, 0.8},
		},
	}

	for _, doc := range documents {
		err := storage.Index(ctx, doc.id, doc.text, doc.embedding)
		if err != nil {
			t.Fatalf("Failed to index document %s: %v", doc.id, err)
		}
	}

	// Test search
	tests := []struct {
		name            string
		queryEmbedding  []float32
		limit           int
		expectedResults int
		expectedFirstID string
	}{
		{
			name:            "search for ML content",
			queryEmbedding:  []float32{0.9, 0.05, 0.05}, // Close to doc1
			limit:           2,
			expectedResults: 2,
			expectedFirstID: "doc1",
		},
		{
			name:            "search for NLP content",
			queryEmbedding:  []float32{0.05, 0.9, 0.05}, // Close to doc2
			limit:           1,
			expectedResults: 1,
			expectedFirstID: "doc2",
		},
		{
			name:            "search all documents",
			queryEmbedding:  []float32{0.33, 0.33, 0.34}, // Equidistant
			limit:           5,
			expectedResults: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.Search(ctx, tt.queryEmbedding, tt.limit)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) != tt.expectedResults {
				t.Errorf("Expected %d results, got %d", tt.expectedResults, len(results))
			}

			if tt.expectedFirstID != "" && len(results) > 0 {
				if results[0].ID != tt.expectedFirstID {
					t.Errorf("Expected first result ID %s, got %s", tt.expectedFirstID, results[0].ID)
				}
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

				// Check that scores are in descending order
				if i > 0 && results[i-1].Score < result.Score {
					t.Errorf("Results not sorted by score: result %d score (%.3f) > result %d score (%.3f)",
						i-1, results[i-1].Score, i, result.Score)
				}
			}
		})
	}
}

func TestSQLiteStorage_hasMultipleChunks(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Index a single chunk document
	err = storage.Index(ctx, "single-chunk", "Single chunk document", []float32{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("Failed to index single chunk document: %v", err)
	}

	// Index a multi-chunk document
	chunks := []Chunk{
		{Text: "Chunk 1", Index: 0},
		{Text: "Chunk 2", Index: 1},
	}
	embeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	err = storage.IndexChunks(ctx, "multi-chunk", "Multi chunk document", chunks, embeddings)
	if err != nil {
		t.Fatalf("Failed to index multi chunk document: %v", err)
	}

	// Test hasMultipleChunks
	if storage.hasMultipleChunks(ctx, "single-chunk") {
		t.Error("Expected single-chunk document to have single chunk")
	}

	if !storage.hasMultipleChunks(ctx, "multi-chunk") {
		t.Error("Expected multi-chunk document to have multiple chunks")
	}

	if storage.hasMultipleChunks(ctx, "nonexistent") {
		t.Error("Expected nonexistent document to return false")
	}
}

func TestSQLiteStorage_Close(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	// Test close before initialization
	err := storage.Close()
	if err != nil {
		t.Errorf("Close should not error when db is nil: %v", err)
	}

	// Initialize and then close
	err = storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	err = storage.Close()
	if err != nil {
		t.Errorf("Failed to close storage: %v", err)
	}

	// Verify database is closed by trying to query
	var count int
	err = storage.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}

// Integration test - full workflow
func TestSQLiteStorage_FullWorkflow(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	err := storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Step 1: Index a document with multiple chunks
	documentID := "full-workflow-doc"
	originalText := "This is a comprehensive document about artificial intelligence. It covers machine learning algorithms and their applications in various domains."

	chunks := []Chunk{
		{
			Text:       "This is a comprehensive document about artificial intelligence.",
			Index:      0,
			StartPos:   0,
			EndPos:     62,
			TokenCount: 10,
			ChunkType:  "text",
		},
		{
			Text:       "It covers machine learning algorithms and their applications in various domains.",
			Index:      1,
			StartPos:   63,
			EndPos:     141,
			TokenCount: 12,
			ChunkType:  "text",
		},
	}

	embeddings := [][]float32{
		{0.8, 0.1, 0.1}, // AI focused
		{0.1, 0.8, 0.1}, // ML focused
	}

	err = storage.IndexChunks(ctx, documentID, originalText, chunks, embeddings)
	if err != nil {
		t.Fatalf("Failed to index document chunks: %v", err)
	}

	// Step 2: Search for AI-related content
	aiQuery := []float32{0.9, 0.05, 0.05}
	results, err := storage.Search(ctx, aiQuery, 5)
	if err != nil {
		t.Fatalf("Failed to search for AI content: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one search result")
	}

	result := results[0]
	if result.ID != documentID {
		t.Errorf("Expected result ID %s, got %s", documentID, result.ID)
	}

	if result.Text != originalText {
		t.Errorf("Expected full document text, got: %q", result.Text)
	}

	// Verify metadata
	metadata := result.Metadata
	if metadata == nil {
		t.Error("Expected metadata to be present")
	} else {
		if chunkIndex, ok := metadata["chunk_index"]; !ok {
			t.Error("Expected chunk_index in metadata")
		} else if chunkIndex.(int) != 0 { // Should match first chunk (AI content)
			t.Errorf("Expected chunk_index 0, got %v", chunkIndex)
		}

		if isChunk, ok := metadata["is_chunk"]; !ok {
			t.Error("Expected is_chunk in metadata")
		} else if !isChunk.(bool) {
			t.Error("Expected is_chunk to be true for multi-chunk document")
		}
	}

	// Step 3: Search for ML-related content
	mlQuery := []float32{0.05, 0.9, 0.05}
	results, err = storage.Search(ctx, mlQuery, 1)
	if err != nil {
		t.Fatalf("Failed to search for ML content: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one search result for ML query")
	}

	// Should still return the same document but highlight the ML chunk
	result = results[0]
	if result.ID != documentID {
		t.Errorf("Expected result ID %s, got %s", documentID, result.ID)
	}

	// Step 4: Update the document (re-index)
	updatedText := "This is an updated comprehensive document about artificial intelligence and machine learning."
	newChunks := []Chunk{
		{
			Text:       updatedText,
			Index:      0,
			StartPos:   0,
			EndPos:     len(updatedText),
			TokenCount: 15,
			ChunkType:  "text",
		},
	}
	newEmbeddings := [][]float32{
		{0.5, 0.5, 0.0}, // Balanced AI/ML
	}

	err = storage.IndexChunks(ctx, documentID, updatedText, newChunks, newEmbeddings)
	if err != nil {
		t.Fatalf("Failed to re-index document: %v", err)
	}

	// Step 5: Verify old chunks were replaced
	var chunkCount int
	err = storage.db.QueryRowContext(ctx, "SELECT chunk_count FROM documents WHERE id = ?", documentID).Scan(&chunkCount)
	if err != nil {
		t.Errorf("Failed to query updated document: %v", err)
	} else if chunkCount != 1 {
		t.Errorf("Expected chunk count 1 after update, got %d", chunkCount)
	}

	// Step 6: Search should return updated content
	results, err = storage.Search(ctx, aiQuery, 1)
	if err != nil {
		t.Fatalf("Failed to search updated content: %v", err)
	}

	if len(results) > 0 && results[0].Text != updatedText {
		t.Errorf("Expected updated text, got: %q", results[0].Text)
	}
}

// Test error conditions
func TestSQLiteStorage_ErrorHandling(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Test operations before initialization
	err := storage.Index(ctx, "test", "text", []float32{0.1, 0.2, 0.3})
	if err == nil {
		t.Error("Expected error when indexing before initialization")
	}

	_, err = storage.Search(ctx, []float32{0.1, 0.2, 0.3}, 1)
	if err == nil {
		t.Error("Expected error when searching before initialization")
	}

	// Initialize
	err = storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			t.Skip("Skipping test: sqlite-vec extension not available")
		}
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Test search with malformed embedding
	invalidEmbedding := []float32{} // Empty embedding
	_, err = storage.Search(ctx, invalidEmbedding, 1)
	if err == nil {
		t.Error("Expected error for empty embedding in search")
	}
}

// Performance benchmark
func BenchmarkSQLiteStorage_Index(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "storage_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "bench.db")
	dataDir := filepath.Join(tempDir, "data")

	storage, err := NewSQLiteStorage(dbPath, 3, dataDir)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}

	err = storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			b.Skip("Skipping benchmark: sqlite-vec extension not available")
		}
		b.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	text := "This is a benchmark test document for indexing performance"
	embedding := []float32{0.1, 0.2, 0.3}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		documentID := fmt.Sprintf("bench-doc-%d", i)
		err := storage.Index(ctx, documentID, text, embedding)
		if err != nil {
			b.Fatalf("Failed to index document: %v", err)
		}
	}
}

func BenchmarkSQLiteStorage_Search(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "storage_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "bench.db")
	dataDir := filepath.Join(tempDir, "data")

	storage, err := NewSQLiteStorage(dbPath, 3, dataDir)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}

	err = storage.Initialize()
	if err != nil {
		if strings.Contains(err.Error(), "sqlite-vec extension not available") {
			b.Skip("Skipping benchmark: sqlite-vec extension not available")
		}
		b.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Pre-populate with test data
	for i := 0; i < 100; i++ {
		documentID := fmt.Sprintf("search-bench-doc-%d", i)
		text := fmt.Sprintf("This is test document number %d for search benchmarking", i)
		embedding := []float32{float32(i) / 100.0, 0.2, 0.3}
		err := storage.Index(ctx, documentID, text, embedding)
		if err != nil {
			b.Fatalf("Failed to index document: %v", err)
		}
	}

	queryEmbedding := []float32{0.5, 0.2, 0.3}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := storage.Search(ctx, queryEmbedding, 10)
		if err != nil {
			b.Fatalf("Failed to search: %v", err)
		}
	}
}
