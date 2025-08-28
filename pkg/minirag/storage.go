package minirag

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStorage struct {
	db         *sql.DB
	path       string
	vectorSize int
	dataDir    string
}

func NewSQLiteStorage(path string, vectorSize int, dataDir string) (*SQLiteStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &SQLiteStorage{
		path:       path,
		vectorSize: vectorSize,
		dataDir:    dataDir,
	}, nil
}

func (s *SQLiteStorage) Initialize() error {
	// Register sqlite-vec extension before opening database
	sqlite_vec.Auto()

	db, err := sql.Open("sqlite3", s.path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.db = db

	if err := s.loadVecExtension(); err != nil {
		return fmt.Errorf("failed to load vec extension: %w", err)
	}

	if err := s.createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) loadVecExtension() error {
	// Test that sqlite-vec extension is available
	var sqliteVersion, vecVersion string
	err := s.db.QueryRow("SELECT sqlite_version(), vec_version()").Scan(&sqliteVersion, &vecVersion)
	if err != nil {
		return fmt.Errorf("sqlite-vec extension not available: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) createTables() error {
	schema := fmt.Sprintf(`
		-- Main documents table
		CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			original_text TEXT,
			original_text_compressed BLOB,
			content_hash TEXT NOT NULL,
			file_path TEXT,
			metadata TEXT,
			chunk_count INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Chunks table for document pieces
		CREATE TABLE IF NOT EXISTS chunks (
			chunk_id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			chunk_text TEXT,
			chunk_text_compressed BLOB,
			start_pos INTEGER,
			end_pos INTEGER,
			token_count INTEGER,
			page_number INTEGER,
			chunk_type TEXT DEFAULT 'text',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
		);

		-- Embeddings for chunks
		CREATE VIRTUAL TABLE IF NOT EXISTS embeddings USING vec0(
			chunk_id TEXT PRIMARY KEY,
			embedding FLOAT[%d]
		);

		-- Indexes
		CREATE INDEX IF NOT EXISTS idx_documents_content_hash ON documents(content_hash);
		CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at);
		CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
		CREATE INDEX IF NOT EXISTS idx_chunks_document_chunk ON chunks(document_id, chunk_index);
	`, s.vectorSize)

	_, err := s.db.Exec(schema)
	return err
}

// IndexChunks indexes a document with its chunks and embeddings
func (s *SQLiteStorage) IndexChunks(ctx context.Context, documentID string, text string, chunks []Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunk count (%d) doesn't match embedding count (%d)", len(chunks), len(embeddings))
	}

	contentHash := s.generateContentHash(text)
	filePath, err := s.storeContent(documentID, text, contentHash)
	if err != nil {
		return fmt.Errorf("failed to store content: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Compress original text for storage
	compressedText, err := CompressText(text)
	if err != nil {
		return fmt.Errorf("failed to compress document text: %w", err)
	}

	// Insert or update document
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (id, original_text_compressed, content_hash, file_path, chunk_count, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?)
	`, documentID, compressedText, contentHash, filePath, len(chunks), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Delete existing chunks and embeddings for this document
	_, err = tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete old chunks: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM embeddings WHERE chunk_id LIKE ?`, documentID+"%")
	if err != nil {
		return fmt.Errorf("failed to delete old embeddings: %w", err)
	}

	// Insert new chunks and embeddings
	for i, chunk := range chunks {
		chunkID := GetChunkID(documentID, chunk.Index)

		// Insert chunk with page metadata
		pageNumber := sql.NullInt32{}
		if chunk.PageNumber != nil {
			pageNumber.Int32 = int32(*chunk.PageNumber)
			pageNumber.Valid = true
		}

		chunkType := chunk.ChunkType
		if chunkType == "" {
			chunkType = "text"
		}

		// Compress chunk text for storage
		compressedChunkText, err := CompressText(chunk.Text)
		if err != nil {
			return fmt.Errorf("failed to compress chunk %d text: %w", i, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO chunks (chunk_id, document_id, chunk_index, chunk_text_compressed, start_pos, end_pos, token_count, page_number, chunk_type) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, chunkID, documentID, chunk.Index, compressedChunkText, chunk.StartPos, chunk.EndPos, chunk.TokenCount, pageNumber, chunkType)
		if err != nil {
			return fmt.Errorf("failed to insert chunk %d: %w", i, err)
		}

		// Insert embedding
		embeddingJSON, err := json.Marshal(embeddings[i])
		if err != nil {
			return fmt.Errorf("failed to marshal embedding for chunk %d: %w", i, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO embeddings (chunk_id, embedding) VALUES (?, ?)
		`, chunkID, string(embeddingJSON))
		if err != nil {
			return fmt.Errorf("failed to insert embedding for chunk %d: %w", i, err)
		}
	}

	return tx.Commit()
}

// Index maintains backward compatibility for single-text indexing
func (s *SQLiteStorage) Index(ctx context.Context, id string, text string, embedding []float32) error {
	// Create a single chunk for backward compatibility
	chunk := Chunk{
		Text:       text,
		Index:      0,
		StartPos:   0,
		EndPos:     len(text),
		TokenCount: len(strings.Fields(text)), // Simple token estimation
		ChunkType:  "text",
		PageNumber: nil,
	}

	return s.IndexChunks(ctx, id, text, []Chunk{chunk}, [][]float32{embedding})
}

func (s *SQLiteStorage) generateContentHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

func (s *SQLiteStorage) storeContent(id, text, contentHash string) (string, error) {
	filename := fmt.Sprintf("%s_%s.txt.gz", id, contentHash[:8])
	filePath := filepath.Join(s.dataDir, filename)

	// Compress text before storing
	compressedText, err := CompressText(text)
	if err != nil {
		return "", fmt.Errorf("failed to compress content: %w", err)
	}

	if err := os.WriteFile(filePath, compressedText, 0644); err != nil {
		return "", fmt.Errorf("failed to write compressed content file: %w", err)
	}

	return filePath, nil
}

func (s *SQLiteStorage) Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error) {
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query embedding: %w", err)
	}

	// Search through chunks and return best matches
	query := `
		SELECT 
			c.document_id,
			c.chunk_text_compressed,
			c.chunk_index,
			c.page_number,
			c.chunk_type,
			d.original_text_compressed,
			d.file_path,
			vec_distance_cosine(e.embedding, ?) as distance
		FROM chunks c
		JOIN documents d ON c.document_id = d.id
		JOIN embeddings e ON c.chunk_id = e.chunk_id
		ORDER BY distance
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, string(embeddingJSON), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	// Use a map to deduplicate results by document ID, keeping the best score per document
	documentResults := make(map[string]SearchResult)

	for rows.Next() {
		var result SearchResult
		var distance float64
		var chunkIndex int
		var compressedChunkText []byte
		var compressedOriginalText []byte
		var pageNumber sql.NullInt32
		var chunkType string
		var filePath sql.NullString

		if err := rows.Scan(&result.ID, &compressedChunkText, &chunkIndex, &pageNumber, &chunkType, &compressedOriginalText, &filePath, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		score := 1.0 - distance

		// Check if we already have a result for this document
		if existingResult, exists := documentResults[result.ID]; exists {
			// Keep the result with the better score
			if score <= existingResult.Score {
				continue // Skip this result as we have a better one
			}
		}

		// Decompress chunk text (for the matching chunk information)
		chunkText, err := DecompressText(compressedChunkText)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress chunk text: %w", err)
		}

		// Decompress original text (this is what we'll show to the user)
		originalText, err := DecompressText(compressedOriginalText)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress original text: %w", err)
		}

		// Set the result text to the full document content
		result.Text = originalText
		result.Score = score

		// Add metadata about the matching chunk and document
		metadata := map[string]interface{}{
			"chunk_index":    chunkIndex,
			"chunk_type":     chunkType,
			"is_chunk":       chunkIndex > 0 || s.hasMultipleChunks(ctx, result.ID),
			"original_text":  originalText,
			"matching_chunk": chunkText, // Keep the matching chunk for reference
		}

		// Add page number if available
		if pageNumber.Valid {
			metadata["page_number"] = int(pageNumber.Int32)
		}

		// Add file path if available
		if filePath.Valid && filePath.String != "" {
			metadata["file_path"] = filePath.String
		}

		result.Metadata = metadata
		documentResults[result.ID] = result
	}

	// Convert map back to slice and sort by score (highest first)
	var results []SearchResult
	for _, result := range documentResults {
		results = append(results, result)
	}

	// Sort results by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit results to requested number
	if len(results) > limit {
		results = results[:limit]
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}

// hasMultipleChunks checks if a document has multiple chunks
func (s *SQLiteStorage) hasMultipleChunks(ctx context.Context, documentID string) bool {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT chunk_count FROM documents WHERE id = ?`, documentID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 1
}

func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
