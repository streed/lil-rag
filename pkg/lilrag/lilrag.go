package lilrag

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	// Default configuration values
	DefaultOllamaURL = "http://localhost:11434"
	DefaultModel     = "nomic-embed-text"
)

type LilRag struct {
	storage         Storage
	embedder        Embedder
	chatClient      *OllamaChatClient
	chunker         *TextChunker
	pdfParser       *PDFParser // Keep for backward compatibility
	documentHandler *DocumentHandler
	config          *Config
}

type Config struct {
	DatabasePath string
	DataDir      string
	OllamaURL    string
	Model        string
	ChatModel    string
	VectorSize   int
	MaxTokens    int
	Overlap      int
}

type Storage interface {
	Initialize() error
	Index(ctx context.Context, id string, text string, embedding []float32) error
	IndexChunks(ctx context.Context, documentID string, text string, chunks []Chunk, embeddings [][]float32) error
	IndexChunksWithMetadata(ctx context.Context, documentID, text string, chunks []Chunk, embeddings [][]float32, originalFilePath, docType string) error
	Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error)
	ListDocuments(ctx context.Context) ([]DocumentInfo, error)
	GetDocumentByID(ctx context.Context, documentID string) (*DocumentInfo, error)
	GetDocumentChunks(ctx context.Context, documentID string) ([]Chunk, error)
	DeleteDocument(ctx context.Context, documentID string) error
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

type DocumentInfo struct {
	ID         string    `json:"id"`
	Text       string    `json:"text"`
	ChunkCount int       `json:"chunk_count"`
	SourcePath string    `json:"source_path"`
	DocType    string    `json:"doc_type"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func New(config *Config) (*LilRag, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.DatabasePath == "" {
		config.DatabasePath = "lilrag.db"
	}
	if config.OllamaURL == "" {
		config.OllamaURL = DefaultOllamaURL
	}
	if config.Model == "" {
		config.Model = DefaultModel
	}
	if config.ChatModel == "" {
		config.ChatModel = "gemma3:4b"
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

	return &LilRag{
		config: config,
	}, nil
}

func (m *LilRag) Initialize() error {
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

	// Initialize PDF parser (keep for backward compatibility)
	m.pdfParser = NewPDFParser()

	// Initialize document handler with all supported parsers
	m.documentHandler = NewDocumentHandler(m.chunker)

	// Initialize chat client
	m.chatClient = NewOllamaChatClient(m.config.OllamaURL, m.config.ChatModel)

	return m.storage.Initialize()
}

func (m *LilRag) Index(ctx context.Context, text, id string) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if m.chunker == nil || m.embedder == nil || m.storage == nil {
		return fmt.Errorf("LilRag not properly initialized")
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
func (m *LilRag) IndexPDF(ctx context.Context, filePath, id string) error {
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

// IndexFile indexes a file, automatically detecting the format and using appropriate parser
func (m *LilRag) IndexFile(ctx context.Context, filePath, id string) error {
	if m.documentHandler == nil {
		// Fallback to legacy behavior if document handler not initialized
		if IsPDFFile(filePath) {
			return m.IndexPDF(ctx, filePath, id)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		return m.Index(ctx, string(content), id)
	}

	// Use document handler for all supported formats
	if !m.documentHandler.IsSupported(filePath) {
		return fmt.Errorf("unsupported file format: %s", filePath)
	}

	// Parse and chunk the document
	chunks, err := m.documentHandler.ParseFileWithChunks(filePath, id)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	if len(chunks) == 0 {
		return fmt.Errorf("no content found in document")
	}

	// Generate embeddings for all chunks
	embeddings := make([][]float32, len(chunks))
	var combinedText strings.Builder
	
	for i, chunk := range chunks {
		embedding, err := m.embedder.Embed(ctx, chunk.Text)
		if err != nil {
			return fmt.Errorf("failed to create embedding for chunk %d: %w", i, err)
		}
		embeddings[i] = embedding
		
		// Build combined text for storage
		if i > 0 {
			combinedText.WriteString("\n\n")
		}
		combinedText.WriteString(chunk.Text)
	}

	// Detect document type for metadata
	docType := string(m.documentHandler.DetectDocumentType(filePath))
	
	// Store document with chunks and metadata
	return m.storage.IndexChunksWithMetadata(ctx, id, combinedText.String(), chunks, embeddings, filePath, docType)
}

func (m *LilRag) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 10
	}
	if m.embedder == nil || m.storage == nil {
		return nil, fmt.Errorf("LilRag not properly initialized")
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

// Chat performs a conversational query using retrieved context
func (m *LilRag) Chat(ctx context.Context, userMessage string, limit int) (string, []SearchResult, error) {
	if userMessage == "" {
		return "", nil, fmt.Errorf("user message cannot be empty")
	}
	if limit <= 0 {
		limit = 5 // Default limit for chat context
	}
	if m.chatClient == nil {
		return "", nil, fmt.Errorf("chat client not initialized")
	}

	// First, optimize the query using the LLM for better semantic search
	optimizedQuery, err := m.chatClient.OptimizeQuery(ctx, userMessage)
	if err != nil {
		// Log the error but continue with the original query
		fmt.Printf("Warning: Query optimization failed, using original query: %v\n", err)
		optimizedQuery = userMessage
	}

	// Log the query transformation for visibility
	if optimizedQuery != userMessage {
		fmt.Printf("Query optimization: '%s' → '%s'\n", userMessage, optimizedQuery)
	} else {
		fmt.Printf("Query optimization: No change needed for '%s'\n", userMessage)
	}

	// Search for relevant documents using the optimized query
	searchResults, err := m.Search(ctx, optimizedQuery, limit)
	if err != nil {
		return "", nil, fmt.Errorf("failed to search documents: %w", err)
	}

	// Generate chat response using the original user message and search results as context
	response, err := m.chatClient.GenerateResponse(ctx, userMessage, searchResults)
	if err != nil {
		return "", searchResults, fmt.Errorf("failed to generate chat response: %w", err)
	}

	return response, searchResults, nil
}

func (m *LilRag) ListDocuments(ctx context.Context) ([]DocumentInfo, error) {
	return m.storage.ListDocuments(ctx)
}

func (m *LilRag) GetDocumentByID(ctx context.Context, documentID string) (*DocumentInfo, error) {
	if m.storage == nil {
		return nil, fmt.Errorf("storage not initialized")
	}
	return m.storage.GetDocumentByID(ctx, documentID)
}

func (m *LilRag) GetDocumentChunks(ctx context.Context, documentID string) ([]Chunk, error) {
	if m.storage == nil {
		return nil, fmt.Errorf("storage not initialized")
	}
	return m.storage.GetDocumentChunks(ctx, documentID)
}

func (m *LilRag) DeleteDocument(ctx context.Context, documentID string) error {
	if m.storage == nil {
		return fmt.Errorf("storage not initialized")
	}
	return m.storage.DeleteDocument(ctx, documentID)
}

func (m *LilRag) ParseDocumentFile(filePath string) (string, error) {
	if m.documentHandler == nil {
		return "", fmt.Errorf("document handler not initialized")
	}
	if !m.documentHandler.IsSupported(filePath) {
		return "", fmt.Errorf("unsupported file type: %s", filePath)
	}
	return m.documentHandler.ParseFile(filePath)
}

func (m *LilRag) Close() error {
	if m.storage != nil {
		return m.storage.Close()
	}
	return nil
}
