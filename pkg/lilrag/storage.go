package lilrag

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3" // Register SQLite3 driver
)

type SQLiteStorage struct {
	db         *sql.DB
	path       string
	vectorSize int
	dataDir    string
}

func NewSQLiteStorage(path string, vectorSize int, dataDir string) (*SQLiteStorage, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
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
			source_path TEXT,
			doc_type TEXT,
			namespace TEXT,
			chunking_method TEXT DEFAULT 'auto',
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
			content_start INTEGER DEFAULT 0,
			content_end INTEGER DEFAULT 0,
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
	if err != nil {
		return err
	}

	// Migration: Add namespace column to existing databases
	_, err = s.db.Exec(`ALTER TABLE documents ADD COLUMN namespace TEXT`)
	if err != nil {
		// Ignore error if column already exists (expected for new databases)
		if !strings.Contains(err.Error(), "duplicate column name") &&
			!strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add namespace column: %w", err)
		}
	}

	// Migration: Add chunking_method column to existing databases
	_, err = s.db.Exec(`ALTER TABLE documents ADD COLUMN chunking_method TEXT DEFAULT 'auto'`)
	if err != nil {
		// Ignore error if column already exists (expected for new databases)
		if !strings.Contains(err.Error(), "duplicate column name") &&
			!strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add chunking_method column: %w", err)
		}
	}

	// Migration: Add content_start column to existing chunks tables
	_, err = s.db.Exec(`ALTER TABLE chunks ADD COLUMN content_start INTEGER DEFAULT 0`)
	if err != nil {
		// Ignore error if column already exists (expected for new databases)
		if !strings.Contains(err.Error(), "duplicate column name") &&
			!strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add content_start column: %w", err)
		}
	}

	// Migration: Add content_end column to existing chunks tables
	_, err = s.db.Exec(`ALTER TABLE chunks ADD COLUMN content_end INTEGER DEFAULT 0`)
	if err != nil {
		// Ignore error if column already exists (expected for new databases)
		if !strings.Contains(err.Error(), "duplicate column name") &&
			!strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to add content_end column: %w", err)
		}
	}

	return nil
}

// IndexWithNamespace indexes a single document with namespace
func (s *SQLiteStorage) IndexWithNamespace(
	ctx context.Context,
	id, text string,
	embedding []float32,
	namespace string,
) error {
	if s.db == nil {
		return fmt.Errorf("storage not initialized - call Initialize() first")
	}

	contentHash := s.generateContentHash(text)
	filePath, err := s.storeContent(id, text, contentHash)
	if err != nil {
		return fmt.Errorf("failed to store content: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("Failed to rollback transaction: %v", rollbackErr)
			}
		}
	}()

	// Insert the document with namespace
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents
		(id, original_text_compressed, content_hash, file_path, doc_type, namespace, chunking_method, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'text', ?, 'auto', 1, datetime('now'), datetime('now'))
	`, id, []byte{}, contentHash, filePath, namespace)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Generate a chunk ID for the main content
	chunkID := fmt.Sprintf("%s_chunk_0", id)

	// Compress text for storage
	compressedText, err := CompressText(text)
	if err != nil {
		return fmt.Errorf("failed to compress text: %w", err)
	}

	// Insert the chunk
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO chunks
		(chunk_id, document_id, chunk_index, chunk_text_compressed, start_pos, end_pos, content_start, content_end, token_count, chunk_type, created_at)
		VALUES (?, ?, 0, ?, 0, ?, 0, ?, ?, 'text', datetime('now'))
	`, chunkID, id, compressedText, len(text), len(text), len(text))
	if err != nil {
		return fmt.Errorf("failed to insert chunk: %w", err)
	}

	// Insert the embedding
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO embeddings (chunk_id, embedding)
		VALUES (?, ?)
	`, chunkID, string(embeddingJSON))
	if err != nil {
		return fmt.Errorf("failed to insert embedding: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	return nil
}

// IndexChunksWithNamespace indexes a document with chunks and namespace
func (s *SQLiteStorage) IndexChunksWithNamespace(
	ctx context.Context,
	documentID, text string,
	chunks []Chunk,
	embeddings [][]float32,
	originalFilePath, docType, namespace string,
	chunkingMethod ...string,
) error {
	if s.db == nil {
		return fmt.Errorf("storage not initialized - call Initialize() first")
	}

	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunk count (%d) doesn't match embedding count (%d)", len(chunks), len(embeddings))
	}

	// Check if document already exists and delete it if it does
	_, err := s.GetDocumentByID(ctx, documentID)
	if err == nil {
		// Document exists, delete it first to remove all chunks and embeddings
		if deleteErr := s.DeleteDocument(ctx, documentID); deleteErr != nil {
			return fmt.Errorf("failed to delete existing document before re-indexing: %w", deleteErr)
		}
	} else if !strings.Contains(err.Error(), "document not found") {
		// If error is not "document not found", it's a real error
		return fmt.Errorf("failed to check if document exists: %w", err)
	}
	// If document doesn't exist (err contains "document not found"), continue with normal indexing

	contentHash := s.generateContentHash(text)
	filePath, err := s.storeContent(documentID, text, contentHash)
	if err != nil {
		return fmt.Errorf("failed to store content: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("Failed to rollback transaction: %v", rollbackErr)
			}
		}
	}()

	// Insert document (document should not exist at this point since we deleted it if it existed)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO documents 
		(id, original_text_compressed, content_hash, file_path, source_path, 
		 doc_type, namespace, chunk_count, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, documentID, []byte{}, contentHash, filePath, originalFilePath, docType, namespace, len(chunks))
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Insert new chunks and embeddings
	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s_chunk_%d", documentID, i)

		// Extract original text segment to preserve formatting
		originalChunkText := text[chunk.StartPos:chunk.EndPos]

		// Compress original chunk text for storage
		compressedChunkText, err := CompressText(originalChunkText)
		if err != nil {
			return fmt.Errorf("failed to compress chunk %d text: %w", i, err)
		}

		// Insert chunk
		_, err = tx.ExecContext(ctx, `
			INSERT INTO chunks
			(chunk_id, document_id, chunk_index, chunk_text_compressed,
			 start_pos, end_pos, content_start, content_end, token_count, page_number, chunk_type, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		`, chunkID, documentID, chunk.Index, compressedChunkText,
			chunk.StartPos, chunk.EndPos, chunk.ContentStart, chunk.ContentEnd, chunk.TokenCount,
			chunk.PageNumber, chunk.ChunkType)
		if err != nil {
			return fmt.Errorf("failed to insert chunk %d: %w", i, err)
		}

		// Insert embedding
		embeddingJSON, err := json.Marshal(embeddings[i])
		if err != nil {
			return fmt.Errorf("failed to marshal embedding for chunk %d: %w", i, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO embeddings (chunk_id, embedding)
			VALUES (?, ?)
		`, chunkID, string(embeddingJSON))
		if err != nil {
			return fmt.Errorf("failed to insert embedding for chunk %d: %w", i, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	return nil
}

// IndexChunks indexes a document with its chunks and embeddings
func (s *SQLiteStorage) IndexChunks(ctx context.Context, documentID, text string,
	chunks []Chunk, embeddings [][]float32) error {
	return s.IndexChunksWithMetadata(ctx, documentID, text, chunks, embeddings, "", "")
}

// IndexChunksWithMethod indexes a document with chunks and chunking method
func (s *SQLiteStorage) IndexChunksWithMethod(ctx context.Context, documentID, text string,
	chunks []Chunk, embeddings [][]float32, originalFilePath, docType, chunkingMethod string) error {
	return s.IndexChunksWithNamespace(ctx, documentID, text, chunks, embeddings, originalFilePath, docType, "", chunkingMethod)
}

// IndexChunksWithNamespaceAndMethod indexes a document with chunks, namespace, and chunking method
func (s *SQLiteStorage) IndexChunksWithNamespaceAndMethod(ctx context.Context, documentID, text string,
	chunks []Chunk, embeddings [][]float32, originalFilePath, docType, namespace, chunkingMethod string) error {
	return s.IndexChunksWithNamespace(ctx, documentID, text, chunks, embeddings, originalFilePath, docType, namespace, chunkingMethod)
}

// IndexChunksWithMetadata indexes a document with metadata including original file path
func (s *SQLiteStorage) IndexChunksWithMetadata(ctx context.Context, documentID, text string,
	chunks []Chunk, embeddings [][]float32, originalFilePath, docType string) error {
	if s.db == nil {
		return fmt.Errorf("storage not initialized - call Initialize() first")
	}

	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunk count (%d) doesn't match embedding count (%d)", len(chunks), len(embeddings))
	}

	// Check if document already exists and delete it if it does
	_, err := s.GetDocumentByID(ctx, documentID)
	if err == nil {
		// Document exists, delete it first to remove all chunks and embeddings
		if deleteErr := s.DeleteDocument(ctx, documentID); deleteErr != nil {
			return fmt.Errorf("failed to delete existing document before re-indexing: %w", deleteErr)
		}
	} else if !strings.Contains(err.Error(), "document not found") {
		// If error is not "document not found", it's a real error
		return fmt.Errorf("failed to check if document exists: %w", err)
	}
	// If document doesn't exist (err contains "document not found"), continue with normal indexing

	contentHash := s.generateContentHash(text)
	filePath, err := s.storeContent(documentID, text, contentHash)
	if err != nil {
		return fmt.Errorf("failed to store content: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false

	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Log rollback error if needed, but don't override the main error
				fmt.Printf("Warning: failed to rollback transaction: %v\n", rbErr)
			}
		}
	}()

	// Compress original text for storage
	compressedText, err := CompressText(text)
	if err != nil {
		return fmt.Errorf("failed to compress document text: %w", err)
	}

	// Insert document (document should not exist at this point since we deleted it if it existed)
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (
			id, original_text_compressed, content_hash, file_path, source_path, doc_type, chunking_method, chunk_count, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, documentID, compressedText, contentHash, filePath, originalFilePath, docType, "auto", len(chunks), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Insert new chunks and embeddings
	for i, chunk := range chunks {
		chunkID := GetChunkID(documentID, chunk.Index)

		// Insert chunk with page metadata
		pageNumber := sql.NullInt32{}
		if chunk.PageNumber != nil {
			if *chunk.PageNumber > 2147483647 { // Max int32 value
				return fmt.Errorf("page number %d exceeds maximum allowed value", *chunk.PageNumber)
			}
			// #nosec G115 - Page number range already validated above
			pageNumber.Int32 = int32(*chunk.PageNumber)
			pageNumber.Valid = true
		}

		chunkType := chunk.ChunkType
		if chunkType == "" {
			chunkType = ChunkTypeText
		}

		// Extract original text segment to preserve formatting
		originalChunkText := text[chunk.StartPos:chunk.EndPos]

		// Compress original chunk text for storage
		compressedChunkText, err := CompressText(originalChunkText)
		if err != nil {
			return fmt.Errorf("failed to compress chunk %d text: %w", i, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO chunks (chunk_id, document_id, chunk_index, chunk_text_compressed,
			                   start_pos, end_pos, content_start, content_end, token_count, page_number, chunk_type)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, chunkID, documentID, chunk.Index, compressedChunkText, chunk.StartPos, chunk.EndPos,
			chunk.ContentStart, chunk.ContentEnd, chunk.TokenCount, pageNumber, chunkType)
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true
	return nil
}

// Index maintains backward compatibility for single-text indexing
func (s *SQLiteStorage) Index(ctx context.Context, id, text string, embedding []float32) error {
	// Create a single chunk for backward compatibility
	chunk := Chunk{
		Text:       text,
		Index:      0,
		StartPos:   0,
		EndPos:     len(text),
		TokenCount: len(strings.Fields(text)), // Simple token estimation
		ChunkType:  ChunkTypeText,
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

	if err := os.WriteFile(filePath, compressedText, 0o600); err != nil {
		return "", fmt.Errorf("failed to write compressed content file: %w", err)
	}

	return filePath, nil
}

func (s *SQLiteStorage) Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error) {
	if s.db == nil {
		return nil, fmt.Errorf("storage not initialized - call Initialize() first")
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding cannot be empty")
	}

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
			c.content_start,
			c.content_end,
			d.original_text_compressed,
			d.file_path,
			d.source_path,
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
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	// Process all chunks individually (no deduplication by document)
	var results []SearchResult

	for rows.Next() {
		var result SearchResult
		var distance float64
		var chunkIndex int
		var compressedChunkText []byte
		var compressedOriginalText []byte
		var pageNumber sql.NullInt32
		var chunkType string
		var contentStart, contentEnd int
		var filePath sql.NullString
		var sourcePath sql.NullString

		if err := rows.Scan(&result.ID, &compressedChunkText, &chunkIndex, &pageNumber,
			&chunkType, &contentStart, &contentEnd, &compressedOriginalText, &filePath, &sourcePath, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		score := 1.0 - distance

		// Decompress chunk text (this is the actual matching content)
		chunkText, err := DecompressText(compressedChunkText)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress chunk text: %w", err)
		}

		// Decompress original text (for metadata reference)
		originalText, err := DecompressText(compressedOriginalText)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress original text: %w", err)
		}

		// Set the result text to the specific matching chunk (not full document)
		result.Text = chunkText
		result.Score = score

		// Calculate ContentText (non-overlapping content)
		if contentStart >= 0 && contentEnd > contentStart && contentEnd <= len(chunkText) {
			result.ContentText = chunkText[contentStart:contentEnd]
		} else {
			// Fall back to full text if content boundaries are invalid
			result.ContentText = chunkText
		}

		// Store original document ID before modifying result.ID
		originalDocumentID := result.ID

		// Create a unique ID that includes chunk information for all chunks
		result.ID = fmt.Sprintf("%s-chunk-%d", result.ID, chunkIndex)

		// Add metadata about the matching chunk and document
		metadata := map[string]interface{}{
			"chunk_index":    chunkIndex,
			"chunk_type":     chunkType,
			"is_chunk":       true,
			"document_id":    originalDocumentID, // Original document ID
			"original_text":  originalText,
			"chunk_content":  chunkText,
		}

		// Add page number if available
		if pageNumber.Valid {
			metadata["page_number"] = int(pageNumber.Int32)
		}

		// Add file path if available
		if filePath.Valid && filePath.String != "" {
			metadata["file_path"] = filePath.String
		}

		// Add source path if available
		if sourcePath.Valid && sourcePath.String != "" {
			metadata["source_path"] = sourcePath.String
		}

		result.Metadata = metadata
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

func (s *SQLiteStorage) ListDocuments(ctx context.Context) ([]DocumentInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, original_text_compressed, chunk_count, source_path, doc_type, namespace, chunking_method, created_at, updated_at
		FROM documents
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	var documents []DocumentInfo
	for rows.Next() {
		var doc DocumentInfo
		var compressedText []byte
		var sourcePath sql.NullString
		var docType sql.NullString
		var namespace sql.NullString
		var chunkingMethod sql.NullString
		var updatedAtStr string
		var createdAtStr string

		err := rows.Scan(
			&doc.ID, &compressedText, &doc.ChunkCount,
			&sourcePath, &docType, &namespace, &chunkingMethod,
			&createdAtStr, &updatedAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document row: %w", err)
		}

		// Set metadata fields
		doc.SourcePath = sourcePath.String
		doc.DocType = docType.String
		doc.Namespace = namespace.String
		doc.ChunkingMethod = chunkingMethod.String
		if doc.ChunkingMethod == "" {
			doc.ChunkingMethod = "auto" // Default for old documents
		}
		doc.IsImage = docType.String == "image"

		// Decompress the text
		doc.Text, err = DecompressText(compressedText)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress document text for %s: %w", doc.ID, err)
		}

		// Parse the timestamps
		doc.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			// Try alternative format if RFC3339 fails
			doc.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAtStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse created_at timestamp for %s: %w", doc.ID, err)
			}
		}

		doc.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			// Try alternative format if RFC3339 fails
			doc.UpdatedAt, err = time.Parse("2006-01-02 15:04:05", updatedAtStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse updated_at timestamp for %s: %w", doc.ID, err)
			}
		}

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during document iteration: %w", err)
	}

	return documents, nil
}

// GetDocumentByID retrieves document information by ID
func (s *SQLiteStorage) GetDocumentByID(ctx context.Context, documentID string) (*DocumentInfo, error) {
	if s.db == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, source_path, doc_type, chunk_count, created_at, updated_at
		FROM documents 
		WHERE id = ?
	`, documentID)

	var doc DocumentInfo
	var sourcePath sql.NullString
	var docType sql.NullString

	err := row.Scan(&doc.ID, &sourcePath, &docType, &doc.ChunkCount, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", documentID)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	doc.SourcePath = sourcePath.String
	doc.DocType = docType.String
	doc.IsImage = docType.String == "image"

	return &doc, nil
}

// GetDocumentChunks retrieves all chunks for a document
func (s *SQLiteStorage) GetDocumentChunks(ctx context.Context, documentID string) ([]Chunk, error) {
	if s.db == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT chunk_index, chunk_text_compressed, start_pos, end_pos, token_count, page_number, chunk_type
		FROM chunks 
		WHERE document_id = ?
		ORDER BY chunk_index
	`, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	var chunks []Chunk
	for rows.Next() {
		var chunk Chunk
		var compressedText []byte
		var pageNumber sql.NullInt32

		err := rows.Scan(&chunk.Index, &compressedText, &chunk.StartPos, &chunk.EndPos,
			&chunk.TokenCount, &pageNumber, &chunk.ChunkType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk row: %w", err)
		}

		// Decompress chunk text
		chunk.Text, err = DecompressText(compressedText)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress chunk text: %w", err)
		}

		// Set page number if available
		if pageNumber.Valid {
			pageNum := int(pageNumber.Int32)
			chunk.PageNumber = &pageNum
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during chunk iteration: %w", err)
	}

	return chunks, nil
}

// DeleteDocument removes a document and all its associated data
func (s *SQLiteStorage) DeleteDocument(ctx context.Context, documentID string) error {
	if s.db == nil {
		return fmt.Errorf("storage not initialized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	var committed bool
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				fmt.Printf("Warning: failed to rollback transaction: %v\n", rbErr)
			}
		}
	}()

	// Get document info before deletion to clean up files
	var filePath sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT file_path FROM documents WHERE id = ?", documentID).Scan(&filePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("document not found: %s", documentID)
		}
		return fmt.Errorf("failed to get document info: %w", err)
	}

	// Delete embeddings first (foreign key constraints)
	_, err = tx.ExecContext(ctx, "DELETE FROM embeddings WHERE chunk_id LIKE ?", documentID+"%")
	if err != nil {
		return fmt.Errorf("failed to delete embeddings: %w", err)
	}

	// Delete chunks
	_, err = tx.ExecContext(ctx, "DELETE FROM chunks WHERE document_id = ?", documentID)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// Delete document
	result, err := tx.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("document not found: %s", documentID)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit deletion: %w", err)
	}
	committed = true

	// Clean up file after successful deletion
	if filePath.Valid && filePath.String != "" {
		if err := os.Remove(filePath.String); err != nil {
			// Log but don't fail - file cleanup is not critical
			fmt.Printf("Warning: failed to delete file %s: %v\n", filePath.String, err)
		}
	}

	return nil
}

// UpdateChunk updates a specific chunk's text and embedding
func (s *SQLiteStorage) UpdateChunk(ctx context.Context, chunkID, newText string, newEmbedding []float32) error {
	if s.db == nil {
		return fmt.Errorf("storage not initialized")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction: %v", rollbackErr)
		}
	}()

	// Update chunk text and token count
	tokenCount := len(strings.Fields(newText)) // Simple token estimation
	_, err = tx.ExecContext(ctx, `
		UPDATE chunks 
		SET chunk_text = ?, token_count = ?
		WHERE chunk_id = ?
	`, newText, tokenCount, chunkID)
	if err != nil {
		return fmt.Errorf("failed to update chunk: %w", err)
	}

	// Update embedding
	embeddingJSON, err := json.Marshal(newEmbedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE embeddings 
		SET embedding = ?
		WHERE chunk_id = ?
	`, string(embeddingJSON), chunkID)
	if err != nil {
		return fmt.Errorf("failed to update embedding: %w", err)
	}

	return tx.Commit()
}

// GetChunk retrieves a specific chunk by ID
func (s *SQLiteStorage) GetChunk(ctx context.Context, chunkID string) (*ChunkInfo, error) {
	if s.db == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	var chunk ChunkInfo
	row := s.db.QueryRowContext(ctx, `
		SELECT chunk_id, document_id, chunk_text, chunk_index, start_pos, end_pos, token_count, chunk_type, page_number
		FROM chunks 
		WHERE chunk_id = ?
	`, chunkID)

	var pageNumber sql.NullInt64
	var chunkText sql.NullString
	err := row.Scan(&chunk.ID, &chunk.DocumentID, &chunkText, &chunk.Index,
		&chunk.StartPos, &chunk.EndPos, &chunk.TokenCount, &chunk.ChunkType, &pageNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chunk not found: %s", chunkID)
		}
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	// Handle compressed text (chunk_text might be null if using compressed storage)
	if chunkText.Valid {
		chunk.Text = chunkText.String
	} else {
		// Try to get from compressed field if regular text is null
		var compressedText []byte
		row = s.db.QueryRowContext(ctx, `
			SELECT chunk_text_compressed FROM chunks WHERE chunk_id = ?
		`, chunkID)
		if err := row.Scan(&compressedText); err == nil && len(compressedText) > 0 {
			if decompressed, err := DecompressText(compressedText); err == nil {
				chunk.Text = decompressed
			}
		}
	}

	if pageNumber.Valid {
		pageNum := int(pageNumber.Int64)
		chunk.PageNumber = &pageNum
	}

	return &chunk, nil
}

// GetDocumentChunksWithInfo retrieves all chunks for a document with full metadata including IDs
func (s *SQLiteStorage) GetDocumentChunksWithInfo(ctx context.Context, documentID string) ([]ChunkInfo, error) {
	if s.db == nil {
		return nil, fmt.Errorf("storage not initialized")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT chunk_id, document_id, chunk_text, chunk_text_compressed, chunk_index, 
		       start_pos, end_pos, token_count, page_number, chunk_type
		FROM chunks 
		WHERE document_id = ?
		ORDER BY chunk_index
	`, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	var chunks []ChunkInfo
	for rows.Next() {
		var chunk ChunkInfo
		var compressedText []byte
		var chunkText sql.NullString
		var pageNumber sql.NullInt64

		err := rows.Scan(&chunk.ID, &chunk.DocumentID, &chunkText, &compressedText, &chunk.Index,
			&chunk.StartPos, &chunk.EndPos, &chunk.TokenCount, &pageNumber, &chunk.ChunkType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk row: %w", err)
		}

		// Handle text (prefer uncompressed if available)
		if chunkText.Valid && chunkText.String != "" {
			chunk.Text = chunkText.String
		} else if len(compressedText) > 0 {
			decompressed, err := DecompressText(compressedText)
			if err != nil {
				return nil, fmt.Errorf("failed to decompress chunk text: %w", err)
			}
			chunk.Text = decompressed
		}

		if pageNumber.Valid {
			pageNum := int(pageNumber.Int64)
			chunk.PageNumber = &pageNum
		}

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during chunk iteration: %w", err)
	}

	return chunks, nil
}

func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// UpdateDocumentSourcePath updates the source path for a document
func (s *SQLiteStorage) UpdateDocumentSourcePath(ctx context.Context, documentID, sourcePath string) error {
	if s.db == nil {
		return fmt.Errorf("storage not initialized")
	}

	_, err := s.db.ExecContext(ctx,
		"UPDATE documents SET source_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		sourcePath, documentID)
	if err != nil {
		return fmt.Errorf("failed to update document source path: %w", err)
	}

	return nil
}
