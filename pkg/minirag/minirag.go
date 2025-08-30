package minirag

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type MiniRag struct {
	storage   Storage
	embedder  Embedder
	chunker   *TextChunker
	pdfParser *PDFParser
	config    *Config
}

type Config struct {
	DatabasePath string
	DataDir      string
	OllamaURL    string
	Model        string
	VectorSize   int
	MaxTokens    int
	Overlap      int
}

type Storage interface {
	Initialize() error
	Index(ctx context.Context, id string, text string, embedding []float32) error
	IndexChunks(ctx context.Context, documentID string, text string, chunks []Chunk, embeddings [][]float32) error
	Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error)
	Close() error
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type SearchResult struct {
	ID       string
	Text     string
	Score    float64
	Metadata map[string]interface{}
}

func New(config *Config) (*MiniRag, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.DatabasePath == "" {
		config.DatabasePath = "minirag.db"
	}
	if config.OllamaURL == "" {
		config.OllamaURL = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "nomic-embed-text"
	}
	if config.VectorSize == 0 {
		config.VectorSize = 768
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 1800
	}
	if config.Overlap == 0 {
		config.Overlap = 200
	}

	return &MiniRag{
		config: config,
	}, nil
}

func (m *MiniRag) Initialize() error {
	if m.config.DataDir == "" {
		m.config.DataDir = "data"
	}

	storage, err := NewSQLiteStorage(m.config.DatabasePath, m.config.VectorSize, m.config.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	m.storage = storage

	embedder, err := NewOllamaEmbedder(m.config.OllamaURL, m.config.Model)
	if err != nil {
		return fmt.Errorf("failed to initialize embedder: %w", err)
	}
	m.embedder = embedder

	// Initialize text chunker
	m.chunker = NewTextChunker(m.config.MaxTokens, m.config.Overlap)

	// Initialize PDF parser
	m.pdfParser = NewPDFParser()

	return m.storage.Initialize()
}

func (m *MiniRag) Index(ctx context.Context, text, id string) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if m.chunker == nil || m.embedder == nil || m.storage == nil {
		return fmt.Errorf("MiniRag not properly initialized")
	}

	// Check if text needs chunking
	if !m.chunker.IsLongText(text) {
		// Simple case: text fits in one chunk
		embedding, err := m.embedder.Embed(ctx, text)
		if err != nil {
			return fmt.Errorf("failed to create embedding: %w", err)
		}
		return m.storage.Index(ctx, id, text, embedding)
	}

	// Complex case: text needs to be chunked
	chunks := m.chunker.ChunkText(text)
	if len(chunks) == 0 {
		return fmt.Errorf("failed to create chunks from text")
	}

	fmt.Printf("Splitting text into %d chunks for document '%s'\n", len(chunks), id)

	// Create embeddings for each chunk
	embeddings := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		fmt.Printf("Creating embedding for chunk %d/%d (tokens: %d)\n", i+1, len(chunks), chunk.TokenCount)
		embedding, err := m.embedder.Embed(ctx, chunk.Text)
		if err != nil {
			return fmt.Errorf("failed to create embedding for chunk %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	// Store document with chunks
	return m.storage.IndexChunks(ctx, id, text, chunks, embeddings)
}

// IndexPDF indexes a PDF file with page-based chunking
func (m *MiniRag) IndexPDF(ctx context.Context, filePath, id string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	// Check if it's a PDF file
	if !IsPDFFile(filePath) {
		return fmt.Errorf("file %s is not a PDF file", filePath)
	}

	// Parse PDF into page-based chunks
	chunks, err := m.pdfParser.ParsePDFWithPageChunks(filePath, id)
	if err != nil {
		return fmt.Errorf("failed to parse PDF: %w", err)
	}

	if len(chunks) == 0 {
		return fmt.Errorf("no readable content found in PDF")
	}

	fmt.Printf("Parsing PDF into %d page chunks for document '%s'\n", len(chunks), id)

	// Create embeddings for each page chunk
	embeddings := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		pageInfo := ""
		if chunk.PageNumber != nil {
			pageInfo = fmt.Sprintf(" (page %d)", *chunk.PageNumber)
		}
		fmt.Printf("Creating embedding for chunk %d/%d%s (tokens: %d)\n",
			i+1, len(chunks), pageInfo, chunk.TokenCount)

		embedding, err := m.embedder.Embed(ctx, chunk.Text)
		if err != nil {
			return fmt.Errorf("failed to create embedding for page chunk %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	// Create a combined text for the document record (first 1000 chars from each page)
	var combinedText strings.Builder
	for _, chunk := range chunks {
		text := chunk.Text
		if len(text) > 1000 {
			text = text[:1000] + "..."
		}
		if chunk.PageNumber != nil {
			combinedText.WriteString(fmt.Sprintf("[Page %d] ", *chunk.PageNumber))
		}
		combinedText.WriteString(text)
		combinedText.WriteString("\n\n")
	}

	// Store document with page chunks
	return m.storage.IndexChunks(ctx, id, combinedText.String(), chunks, embeddings)
}

// IndexFile indexes a file, automatically detecting if it's a PDF or text file
func (m *MiniRag) IndexFile(ctx context.Context, filePath, id string) error {
	if IsPDFFile(filePath) {
		return m.IndexPDF(ctx, filePath, id)
	}

	// For non-PDF files, read as text and use regular indexing
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	return m.Index(ctx, string(content), id)
}

func (m *MiniRag) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}
	if m.embedder == nil || m.storage == nil {
		return nil, fmt.Errorf("MiniRag not properly initialized")
	}

	var embedding []float32
	var err error

	// Use enhanced query processing if available
	if ollamaEmbedder, ok := m.embedder.(*OllamaEmbedder); ok {
		embedding, err = ollamaEmbedder.EmbedQuery(ctx, query)
	} else {
		embedding, err = m.embedder.Embed(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create query embedding: %w", err)
	}

	return m.storage.Search(ctx, embedding, limit)
}

func (m *MiniRag) Close() error {
	if m.storage != nil {
		return m.storage.Close()
	}
	return nil
}
