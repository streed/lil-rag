package lilrag

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"lil-rag/pkg/metrics"
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
	DatabasePath   string
	DataDir        string
	OllamaURL      string
	Model          string
	ChatModel      string
	VisionModel    string
	TimeoutSeconds int
	VectorSize     int
	MaxTokens      int
	Overlap        int
}

type Storage interface {
	Initialize() error
	Index(ctx context.Context, id string, text string, embedding []float32) error
	IndexChunks(ctx context.Context, documentID string, text string, chunks []Chunk, embeddings [][]float32) error
	IndexChunksWithMetadata(
		ctx context.Context, documentID, text string, chunks []Chunk, embeddings [][]float32,
		originalFilePath, docType string,
	) error
	Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error)
	ListDocuments(ctx context.Context) ([]DocumentInfo, error)
	GetDocumentByID(ctx context.Context, documentID string) (*DocumentInfo, error)
	GetDocumentChunks(ctx context.Context, documentID string) ([]Chunk, error)
	GetDocumentChunksWithInfo(ctx context.Context, documentID string) ([]ChunkInfo, error)
	UpdateChunk(ctx context.Context, chunkID, newText string, newEmbedding []float32) error
	GetChunk(ctx context.Context, chunkID string) (*ChunkInfo, error)
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
	IsImage    bool      `json:"is_image"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ChunkInfo represents a chunk with database metadata for API responses
type ChunkInfo struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	Text       string `json:"text"`
	Index      int    `json:"index"`
	StartPos   int    `json:"start_pos"`
	EndPos     int    `json:"end_pos"`
	TokenCount int    `json:"token_count"`
	ChunkType  string `json:"chunk_type"`
	PageNumber *int   `json:"page_number,omitempty"`
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

	embedder, err := NewOllamaEmbedderWithTimeout(m.config.OllamaURL, m.config.Model, m.config.TimeoutSeconds)
	if err != nil {
		return fmt.Errorf("failed to initialize embedder: %w", err)
	}
	m.embedder = embedder

	// Initialize text chunker
	m.chunker = NewTextChunker(m.config.MaxTokens, m.config.Overlap)

	// Initialize PDF parser (keep for backward compatibility)
	m.pdfParser = NewPDFParser()

	// Initialize document handler with all supported parsers including vision
	m.documentHandler = NewDocumentHandlerWithVisionAndTimeout(
		m.chunker,
		m.config.OllamaURL,
		m.config.VisionModel,
		m.config.TimeoutSeconds,
	)

	// Initialize chat client
	m.chatClient = NewOllamaChatClientWithTimeout(m.config.OllamaURL, m.config.ChatModel, m.config.TimeoutSeconds*4)

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

	// Record document tokens processed
	totalTokens := 0
	for _, chunk := range chunks {
		totalTokens += chunk.TokenCount
	}
	metrics.RecordDocumentTokens("text", totalTokens)

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

	// Record document tokens processed - determine document type from file path
	totalTokens := 0
	for _, chunk := range chunks {
		totalTokens += chunk.TokenCount
	}
	docType := m.documentHandler.DetectDocumentType(filePath)
	metrics.RecordDocumentTokens(string(docType), totalTokens)

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

	// Store document with chunks and metadata
	return m.storage.IndexChunksWithMetadata(ctx, id, combinedText.String(), chunks, embeddings, filePath, string(docType))
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

	// Primary vector search
	results, err := m.storage.Search(ctx, embedding, limit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// For better compatibility with image-derived content, check if we need to apply
	// text-based fallback search for improved accuracy
	if m.needsTextFallback(query) {
		// Always supplement business/contact queries with text-based search for better recall
		textResults, err := m.performTextFallbackSearch(ctx, query, limit)
		if err == nil && len(textResults) > 0 {
			// Merge results, prioritizing exact text matches
			results = m.mergeSearchResults(results, textResults)
		}
	}

	return results, nil
}

// needsTextFallback determines if a query would benefit from text-based fallback search
func (m *LilRag) needsTextFallback(query string) bool {
	// Apply text fallback for queries that are likely to have exact matches in image content
	words := strings.Fields(query)
	if len(words) == 0 {
		return false
	}

	// Check for patterns that suggest exact matching would be valuable
	hasBusinessTerms := false
	hasPhoneNumber := false
	hasProperNoun := false

	// Compile phone regex once for performance
	phoneRegex := regexp.MustCompile(`\d{3}[-.]?\d{3}[-.]?\d{4}`)

	for _, word := range words {
		originalWord := word // Keep original for proper noun check
		word = strings.ToLower(word)
		if word == "painting" || word == "services" || word == "cleaning" ||
			word == "construction" || word == "repair" || word == "company" {
			hasBusinessTerms = true
		}

		// Check for phone number patterns
		if phoneRegex.MatchString(word) {
			hasPhoneNumber = true
		}

		// Check for proper nouns (capitalized words that might be names or companies)
		if len(originalWord) > 1 && strings.ToUpper(originalWord[:1]) == originalWord[:1] &&
			strings.ToLower(originalWord[1:]) == originalWord[1:] {
			hasProperNoun = true
		}
	}

	return hasBusinessTerms || hasPhoneNumber || hasProperNoun
}

// performTextFallbackSearch performs text-based search as a fallback
func (m *LilRag) performTextFallbackSearch(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// This is a simplified text search implementation
	// In a production system, this could use FTS (Full Text Search) or similar
	documents, err := m.storage.ListDocuments(ctx)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	queryLower := strings.ToLower(query)
	queryTerms := strings.Fields(queryLower)

	for _, doc := range documents {
		textLower := strings.ToLower(doc.Text)
		score := m.calculateTextMatchScore(textLower, queryTerms)

		if score > 0.1 { // Minimum relevance threshold
			result := SearchResult{
				ID:    doc.ID,
				Text:  doc.Text,
				Score: score,
				Metadata: map[string]interface{}{
					"search_type": "text_fallback",
					"doc_type":    doc.DocType,
					"is_image":    doc.IsImage,
				},
			}
			results = append(results, result)
		}
	}

	// Sort by score (highest first)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// calculateTextMatchScore calculates a simple text matching score
func (m *LilRag) calculateTextMatchScore(text string, queryTerms []string) float64 {
	if len(queryTerms) == 0 {
		return 0
	}

	matches := 0
	totalTerms := len(queryTerms)

	for _, term := range queryTerms {
		if strings.Contains(text, term) {
			matches++
		}
	}

	// Basic scoring: ratio of matched terms with bonus for exact phrase matches
	baseScore := float64(matches) / float64(totalTerms)

	// Bonus for exact phrase match
	phrase := strings.Join(queryTerms, " ")
	if strings.Contains(text, phrase) {
		baseScore += 0.3
	}

	return baseScore
}

// mergeSearchResults combines vector and text search results, avoiding duplicates
func (m *LilRag) mergeSearchResults(vectorResults, textResults []SearchResult) []SearchResult {
	seen := make(map[string]bool)
	var merged []SearchResult

	// Add vector results first (higher priority)
	for _, result := range vectorResults {
		if !seen[result.ID] {
			seen[result.ID] = true
			merged = append(merged, result)
		}
	}

	// Add text results that aren't already present
	for _, result := range textResults {
		if !seen[result.ID] {
			seen[result.ID] = true
			// Mark text results with lower confidence to indicate fallback
			if result.Metadata == nil {
				result.Metadata = make(map[string]interface{})
			}
			result.Metadata["fallback_search"] = true
			merged = append(merged, result)
		}
	}

	return merged
}

// UpdateChunk updates a chunk's text and regenerates its embedding
func (m *LilRag) UpdateChunk(ctx context.Context, chunkID, newText string) error {
	if strings.TrimSpace(newText) == "" {
		return fmt.Errorf("chunk text cannot be empty")
	}

	// Generate new embedding for the updated text
	embedding, err := m.embedder.Embed(ctx, newText)
	if err != nil {
		return fmt.Errorf("failed to generate embedding for updated chunk: %w", err)
	}

	// Update the chunk and embedding in storage
	return m.storage.UpdateChunk(ctx, chunkID, newText, embedding)
}

// GetChunk retrieves a specific chunk by ID
func (m *LilRag) GetChunk(ctx context.Context, chunkID string) (*ChunkInfo, error) {
	return m.storage.GetChunk(ctx, chunkID)
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

// GetDocumentChunksWithInfo retrieves all chunks for a document with IDs for editing
func (m *LilRag) GetDocumentChunksWithInfo(ctx context.Context, documentID string) ([]ChunkInfo, error) {
	if m.storage == nil {
		return nil, fmt.Errorf("storage not initialized")
	}
	return m.storage.GetDocumentChunksWithInfo(ctx, documentID)
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

// GetStorage returns the storage instance (for internal use)
func (m *LilRag) GetStorage() Storage {
	return m.storage
}
