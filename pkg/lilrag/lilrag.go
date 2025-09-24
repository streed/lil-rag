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

	// New service-oriented architecture
	services *Services
}

type Config struct {
	DatabasePath             string
	DataDir                  string
	OllamaURL                string
	Model                    string
	ChatModel                string
	VisionModel              string
	TimeoutSeconds           int
	VectorSize               int
	MaxTokens                int
	Overlap                  int
	ImageMaxSize             int
	DefaultStrategy          string
	SemanticThreshold        float32
	ThresholdType            string
	DefaultLimit             int
	DefaultChatLimit         int
	TruncateDocuments        bool
	MaxDocumentLength        int
	EnableQueryOptimization  bool
	ReturnMatchingChunksOnly bool
}

type Storage interface {
	Initialize() error
	Index(ctx context.Context, id string, text string, embedding []float32) error
	IndexWithNamespace(ctx context.Context, id string, text string, embedding []float32, namespace string) error
	IndexChunks(ctx context.Context, documentID string, text string, chunks []Chunk, embeddings [][]float32) error
	IndexChunksWithMetadata(
		ctx context.Context, documentID, text string, chunks []Chunk, embeddings [][]float32,
		originalFilePath, docType string,
	) error
	IndexChunksWithNamespace(
		ctx context.Context, documentID, text string, chunks []Chunk, embeddings [][]float32,
		originalFilePath, docType, namespace string,
	) error
	Search(ctx context.Context, embedding []float32, limit int) ([]SearchResult, error)
	SearchWithOptions(ctx context.Context, embedding []float32, limit int, returnIndividualChunks bool) ([]SearchResult, error)
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
	Namespace  string    `json:"namespace"`
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
		config.ChatModel = DefaultChatModel
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
		m.config.DataDir = DefaultDataDir
	}

	storage, err := NewSQLiteStorage(m.config.DatabasePath, m.config.VectorSize, m.config.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	m.storage = storage

	embedder, err := NewOllamaEmbedderWithTimeout(m.config.OllamaURL, m.config.Model, m.config.TimeoutSeconds*4)
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
		m.config.TimeoutSeconds*4,
		m.config.ImageMaxSize,
	)

	// Initialize chat client with configuration
	m.chatClient = NewOllamaChatClientWithConfig(
		m.config.OllamaURL,
		m.config.ChatModel,
		m.config.TimeoutSeconds*4,
		m.config.TruncateDocuments,
		m.config.MaxDocumentLength,
	)

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
		err = m.storage.Index(ctx, id, text, embedding)
		if err != nil {
			return err
		}
		fmt.Printf("Successfully indexed document '%s' with 1 chunk\n", id)
		return nil
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

// IndexWithStrategy indexes text using a specific chunking strategy
func (m *LilRag) IndexWithStrategy(ctx context.Context, text, id, strategy string) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if strategy == "" {
		strategy = m.config.DefaultStrategy // use config default
	}
	if m.chunker == nil || m.embedder == nil || m.storage == nil {
		return fmt.Errorf("LilRag not properly initialized")
	}

	// Always apply the specified chunking strategy, regardless of text length
	var chunks []Chunk
	if strategy == ChunkingStrategySemantic {
		chunks = m.chunker.ChunkTextWithSemanticEmbeddingAndThreshold(text, strategy, m.embedder, m.config.ThresholdType, m.config.SemanticThreshold)
	} else {
		chunks = m.chunker.ChunkTextWithStrategy(text, strategy)
	}
	if len(chunks) == 0 {
		return fmt.Errorf("failed to create chunks from text using %s strategy", strategy)
	}

	fmt.Printf("Splitting text into %d chunks for document '%s' using %s chunking\n", len(chunks), id, strategy)

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
	err := m.storage.IndexChunks(ctx, id, text, chunks, embeddings)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully indexed document '%s' with %d chunks\n", id, len(chunks))
	return nil
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

		// Build combined text for storage with preserved formatting
		if i > 0 {
			combinedText.WriteString("\n\n" + strings.Repeat("=", 50) + "\n")
			combinedText.WriteString(fmt.Sprintf("CHUNK %d OF %d", i+1, len(chunks)))
			combinedText.WriteString("\n" + strings.Repeat("=", 50) + "\n\n")
		}
		combinedText.WriteString(chunk.Text)
	}

	// Store document with chunks and metadata
	return m.storage.IndexChunksWithMetadata(ctx, id, combinedText.String(), chunks, embeddings, filePath, string(docType))
}

// IndexFileWithStrategy indexes a file using a specific chunking strategy for text-based content
func (m *LilRag) IndexFileWithStrategy(ctx context.Context, filePath, id, strategy string) error {
	if strategy == "" {
		strategy = m.config.DefaultStrategy // use config default
	}

	if m.documentHandler == nil {
		// Fallback to legacy behavior if document handler not initialized
		if IsPDFFile(filePath) {
			return m.IndexPDF(ctx, filePath, id)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		return m.IndexWithStrategy(ctx, string(content), id, strategy)
	}

	// For supported document formats, check if it's a text-based format that can benefit from custom chunking
	if !m.documentHandler.IsSupported(filePath) {
		return fmt.Errorf("unsupported file format: %s", filePath)
	}

	docType := m.documentHandler.DetectDocumentType(filePath)

	// For text-based documents, apply custom chunking strategy
	if docType == "text" || docType == "markdown" || docType == "csv" {
		// Parse the document to get the text content
		content, err := m.documentHandler.ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}

		if content == "" {
			return fmt.Errorf("no content found in file %s", filePath)
		}

		// Use custom chunking strategy for text content
		return m.IndexWithStrategy(ctx, content, id, strategy)
	}

	// For other document types (PDF, DOCX, etc.), extract text and apply chunking strategy
	content, err := m.documentHandler.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	if content == "" {
		return fmt.Errorf("no content found in document")
	}

	// Apply the specified chunking strategy to the extracted content
	var chunks []Chunk
	if strategy == ChunkingStrategySemantic {
		chunks = m.chunker.ChunkTextWithSemanticEmbeddingAndThreshold(content, strategy, m.embedder, m.config.ThresholdType, m.config.SemanticThreshold)
	} else {
		chunks = m.chunker.ChunkTextWithStrategy(content, strategy)
	}

	if len(chunks) == 0 {
		return fmt.Errorf("no chunks created from document")
	}

	fmt.Printf("Indexing %s document '%s' with %d chunks (using %s chunking)\n", docType, id, len(chunks), strategy)

	// Record document tokens processed
	totalTokens := 0
	for _, chunk := range chunks {
		totalTokens += chunk.TokenCount
	}
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

		// Build combined text for storage with preserved formatting
		if i > 0 {
			combinedText.WriteString("\n\n" + strings.Repeat("=", 50) + "\n")
			combinedText.WriteString(fmt.Sprintf("CHUNK %d OF %d", i+1, len(chunks)))
			combinedText.WriteString("\n" + strings.Repeat("=", 50) + "\n\n")
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
		limit = m.config.DefaultLimit
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

	// Primary vector search - use individual chunks mode if chunks-only is enabled
	results, err := m.storage.SearchWithOptions(ctx, embedding, limit, m.config.ReturnMatchingChunksOnly)
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

	// Filter to only matching chunks if enabled
	if m.config.ReturnMatchingChunksOnly {
		results = m.filterToMatchingChunksOnly(results)
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

// filterToMatchingChunksOnly filters search results to return only the chunk text content
func (m *LilRag) filterToMatchingChunksOnly(results []SearchResult) []SearchResult {
	for i := range results {
		if results[i].Metadata == nil {
			results[i].Metadata = make(map[string]interface{})
		}

		// Replace the full document text with just the matching chunk
		if matchingChunk, exists := results[i].Metadata["matching_chunk"]; exists {
			if chunkText, ok := matchingChunk.(string); ok {
				// Store original full text in metadata for reference
				results[i].Metadata["original_full_text"] = results[i].Text
				// Replace with just the matching chunk text
				results[i].Text = chunkText
			}
		}

		results[i].Metadata["chunks_only"] = true
	}
	return results
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
		limit = m.config.DefaultChatLimit
	}
	if m.chatClient == nil {
		return "", nil, fmt.Errorf("chat client not initialized")
	}

	// Optimize query for better search (only if enabled)
	optimizedQuery := userMessage
	if m.config.EnableQueryOptimization {
		var err error
		optimizedQuery, err = m.chatClient.OptimizeQuery(ctx, userMessage)
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

// ChatStreaming performs a conversational query using retrieved context with streaming response
func (m *LilRag) ChatStreaming(ctx context.Context, userMessage string, limit int,
	handler StreamingChatHandler) ([]SearchResult, error) {
	if userMessage == "" {
		return nil, fmt.Errorf("user message cannot be empty")
	}
	if limit <= 0 {
		limit = m.config.DefaultChatLimit
	}
	if m.chatClient == nil {
		return nil, fmt.Errorf("chat client not initialized")
	}

	// Optimize query for better search (only if enabled)
	optimizedQuery := userMessage
	if m.config.EnableQueryOptimization {
		var err error
		optimizedQuery, err = m.chatClient.OptimizeQuery(ctx, userMessage)
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
	}

	// Search for relevant documents using the optimized query
	searchResults, err := m.Search(ctx, optimizedQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	// Generate streaming chat response using the original user message and search results as context
	err = m.chatClient.GenerateResponseStreaming(ctx, userMessage, searchResults, handler)
	if err != nil {
		return searchResults, fmt.Errorf("failed to generate streaming chat response: %w", err)
	}

	return searchResults, nil
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

// ReindexAllDocuments reprocesses all documents with the current chunking configuration
func (m *LilRag) ReindexAllDocuments(ctx context.Context) error {
	if m.storage == nil || m.chunker == nil || m.embedder == nil || m.documentHandler == nil {
		return fmt.Errorf("LilRag not properly initialized")
	}

	// Get all documents
	documents, err := m.storage.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	if len(documents) == 0 {
		fmt.Println("No documents found to reindex")
		return nil
	}

	fmt.Printf("Starting reindex of %d documents with recursive chunking...\n", len(documents))

	processed := 0
	failed := 0

	for i, doc := range documents {
		fmt.Printf("Reindexing document %d/%d: %s\n", i+1, len(documents), doc.ID)

		err := m.reindexDocument(ctx, &doc)
		if err != nil {
			fmt.Printf("Failed to reindex document %s: %v\n", doc.ID, err)
			failed++
			continue
		}

		processed++
		if processed%10 == 0 {
			fmt.Printf("Progress: %d/%d documents processed\n", processed, len(documents))
		}
	}

	fmt.Printf("Reindex completed: %d processed, %d failed\n", processed, failed)
	if failed > 0 {
		return fmt.Errorf("reindex completed with %d failures", failed)
	}

	return nil
}

// ReindexAllDocumentsWithStrategy reprocesses all documents with the specified chunking strategy
func (m *LilRag) ReindexAllDocumentsWithStrategy(ctx context.Context, strategy string) error {
	if m.storage == nil || m.chunker == nil || m.embedder == nil || m.documentHandler == nil {
		return fmt.Errorf("LilRag not properly initialized")
	}

	// Get all documents
	documents, err := m.storage.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	if len(documents) == 0 {
		fmt.Println("No documents found to reindex")
		return nil
	}

	fmt.Printf("Starting reindex of %d documents with %s chunking...\n", len(documents), strategy)

	processed := 0
	failed := 0

	for i, doc := range documents {
		fmt.Printf("Reindexing document %d/%d: %s\n", i+1, len(documents), doc.ID)

		err := m.reindexDocumentWithStrategy(ctx, &doc, strategy)
		if err != nil {
			fmt.Printf("Failed to reindex document %s: %v\n", doc.ID, err)
			failed++
			continue
		}

		processed++
		if processed%10 == 0 {
			fmt.Printf("Progress: %d/%d documents processed\n", processed, len(documents))
		}
	}

	fmt.Printf("Reindex completed: %d processed, %d failed\n", processed, failed)
	if failed > 0 {
		return fmt.Errorf("reindex completed with %d failures", failed)
	}

	return nil
}

// reindexDocument reprocesses a single document with current chunking settings
func (m *LilRag) reindexDocument(ctx context.Context, doc *DocumentInfo) error {
	// If document has a source path, try to reprocess from the original file
	if doc.SourcePath != "" {
		// Check if the source file still exists
		if _, err := os.Stat(doc.SourcePath); err == nil {
			// File exists, reprocess from original
			return m.reindexFromFile(ctx, doc)
		}
		// File doesn't exist, fall back to reprocessing stored text
		fmt.Printf("Source file %s not found, reprocessing from stored text\n", doc.SourcePath)
	}

	// Reprocess from stored text content
	return m.reindexFromText(ctx, doc)
}

// reindexFromFile reprocesses a document from its original file
func (m *LilRag) reindexFromFile(ctx context.Context, doc *DocumentInfo) error {
	if !m.documentHandler.IsSupported(doc.SourcePath) {
		return fmt.Errorf("unsupported file format: %s", doc.SourcePath)
	}

	// Parse the file to get new chunks
	chunks, err := m.documentHandler.ParseFileWithChunks(doc.SourcePath, doc.ID)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", doc.SourcePath, err)
	}

	if len(chunks) == 0 {
		return fmt.Errorf("no content found in file %s", doc.SourcePath)
	}

	return m.reindexWithChunks(ctx, doc, chunks)
}

// reindexFromText reprocesses a document from its stored text
func (m *LilRag) reindexFromText(ctx context.Context, doc *DocumentInfo) error {
	if doc.Text == "" {
		return fmt.Errorf("no text content available for document %s", doc.ID)
	}

	// Re-chunk the text with current settings
	chunks := m.chunker.ChunkText(doc.Text)
	if len(chunks) == 0 {
		return fmt.Errorf("failed to create chunks from text for document %s", doc.ID)
	}

	return m.reindexWithChunks(ctx, doc, chunks)
}

// reindexWithChunks reindexes a document with the given chunks
func (m *LilRag) reindexWithChunks(ctx context.Context, doc *DocumentInfo, chunks []Chunk) error {
	fmt.Printf("Re-chunking document %s: %d chunks -> %d chunks\n", doc.ID, doc.ChunkCount, len(chunks))

	// Generate embeddings for all chunks
	embeddings := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		embedding, err := m.embedder.Embed(ctx, chunk.Text)
		if err != nil {
			return fmt.Errorf("failed to create embedding for chunk %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	// Build combined text for the document
	var combinedText strings.Builder
	for i, chunk := range chunks {
		if i > 0 {
			combinedText.WriteString("\n\n")
		}
		combinedText.WriteString(chunk.Text)
	}

	// Update the document with new chunks
	if doc.SourcePath != "" {
		// Use metadata version to preserve source path and doc type
		return m.storage.IndexChunksWithMetadata(
			ctx, doc.ID, combinedText.String(), chunks, embeddings,
			doc.SourcePath, doc.DocType,
		)
	}
	// Use basic version for text-only documents
	return m.storage.IndexChunks(ctx, doc.ID, combinedText.String(), chunks, embeddings)
}

// reindexDocumentWithStrategy reprocesses a single document with the specified chunking strategy
func (m *LilRag) reindexDocumentWithStrategy(ctx context.Context, doc *DocumentInfo, strategy string) error {
	// If document has a source path, try to reprocess from the original file
	if doc.SourcePath != "" {
		// Check if the source file still exists
		if _, err := os.Stat(doc.SourcePath); err == nil {
			// File exists, reprocess from original with strategy
			return m.reindexFromFileWithStrategy(ctx, doc, strategy)
		}
		// File doesn't exist, fall back to reprocessing stored text
		fmt.Printf("Source file %s not found, reprocessing from stored text\n", doc.SourcePath)
	}

	// Reprocess from stored text content with strategy
	return m.reindexFromTextWithStrategy(ctx, doc, strategy)
}

// reindexFromFileWithStrategy reprocesses a document from its original file with the specified strategy
func (m *LilRag) reindexFromFileWithStrategy(ctx context.Context, doc *DocumentInfo, strategy string) error {
	if !m.documentHandler.IsSupported(doc.SourcePath) {
		return fmt.Errorf("unsupported file format: %s", doc.SourcePath)
	}

	// Parse the file to get text content
	text, err := m.documentHandler.ParseFile(doc.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", doc.SourcePath, err)
	}

	if text == "" {
		return fmt.Errorf("no content found in file %s", doc.SourcePath)
	}

	// Apply the specified chunking strategy
	var chunks []Chunk
	if strategy == ChunkingStrategySemantic {
		chunks = m.chunker.ChunkTextWithSemanticEmbeddingAndThreshold(text, strategy, m.embedder, m.config.ThresholdType, m.config.SemanticThreshold)
	} else {
		chunks = m.chunker.ChunkTextWithStrategy(text, strategy)
	}
	if len(chunks) == 0 {
		return fmt.Errorf("failed to create chunks from file %s", doc.SourcePath)
	}

	return m.reindexWithChunks(ctx, doc, chunks)
}

// reindexFromTextWithStrategy reprocesses a document from its stored text with the specified strategy
func (m *LilRag) reindexFromTextWithStrategy(ctx context.Context, doc *DocumentInfo, strategy string) error {
	if doc.Text == "" {
		return fmt.Errorf("no text content available for document %s", doc.ID)
	}

	// Apply the specified chunking strategy
	var chunks []Chunk
	if strategy == ChunkingStrategySemantic {
		chunks = m.chunker.ChunkTextWithSemanticEmbeddingAndThreshold(doc.Text, strategy, m.embedder, m.config.ThresholdType, m.config.SemanticThreshold)
	} else {
		chunks = m.chunker.ChunkTextWithStrategy(doc.Text, strategy)
	}
	if len(chunks) == 0 {
		return fmt.Errorf("failed to create chunks from text for document %s", doc.ID)
	}

	return m.reindexWithChunks(ctx, doc, chunks)
}

// Services returns the modern service interfaces
func (m *LilRag) Services() *Services {
	return m.services
}

// initializeLegacyComponents initializes the legacy components for backward compatibility
func (m *LilRag) initializeLegacyComponents() error {
	if m.config.DataDir == "" {
		m.config.DataDir = "data"
	}

	storage, err := NewSQLiteStorage(m.config.DatabasePath, m.config.VectorSize, m.config.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	m.storage = storage

	embedder, err := NewOllamaEmbedderWithTimeout(m.config.OllamaURL, m.config.Model, m.config.TimeoutSeconds*4)
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
		m.config.TimeoutSeconds*4,
		m.config.ImageMaxSize,
	)

	// Initialize chat client with configuration
	m.chatClient = NewOllamaChatClientWithConfig(
		m.config.OllamaURL,
		m.config.ChatModel,
		m.config.TimeoutSeconds*4,
		m.config.TruncateDocuments,
		m.config.MaxDocumentLength,
	)

	return m.storage.Initialize()
}
