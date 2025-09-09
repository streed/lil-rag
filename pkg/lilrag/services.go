package lilrag

import (
	"context"
	"fmt"
	"time"
)

// ServiceConfig contains configuration for all services
type ServiceConfig struct {
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
	ImageMaxSize   int
}

// IndexingService handles document indexing operations
type IndexingService interface {
	IndexText(ctx context.Context, text, documentID string) error
	IndexFile(ctx context.Context, filePath, documentID string) error
	UpdateChunk(ctx context.Context, chunkID, newText string) error
}

// SearchService handles search operations
type SearchService interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	SearchWithContext(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]SearchResult, error)
}

// ChatService handles conversational operations
type ChatService interface {
	Chat(ctx context.Context, message string, limit int) (string, []SearchResult, error)
	OptimizeQuery(ctx context.Context, userMessage string) (string, error)
}

// DocumentService handles document management operations
type DocumentService interface {
	ListDocuments(ctx context.Context) ([]DocumentInfo, error)
	GetDocument(ctx context.Context, documentID string) (*DocumentInfo, error)
	GetDocumentChunks(ctx context.Context, documentID string) ([]ChunkInfo, error)
	DeleteDocument(ctx context.Context, documentID string) error
}

// HealthService handles health check operations
type HealthService interface {
	CheckHealth(ctx context.Context) error
	GetMetrics(ctx context.Context) (map[string]interface{}, error)
}

// Services aggregates all service interfaces
type Services struct {
	Indexing IndexingService
	Search   SearchService
	Chat     ChatService
	Document DocumentService
	Health   HealthService
}

// DefaultIndexingService implements IndexingService
type DefaultIndexingService struct {
	storage        Storage
	embedder       Embedder
	parsingService *DocumentParsingService
}

// NewIndexingService creates a new indexing service
func NewIndexingService(storage Storage, embedder Embedder, parsingService *DocumentParsingService) IndexingService {
	return &DefaultIndexingService{
		storage:        storage,
		embedder:       embedder,
		parsingService: parsingService,
	}
}

// IndexText indexes text content
func (s *DefaultIndexingService) IndexText(ctx context.Context, text, documentID string) error {
	if err := ValidateText(text); err != nil {
		return err
	}
	if err := ValidateDocumentID(documentID); err != nil {
		return err
	}

	// Generate embedding
	embedding, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return NewEmbeddingError("failed to generate embedding", err).WithOperation("IndexText")
	}

	// Store in database
	if err := s.storage.Index(ctx, documentID, text, embedding); err != nil {
		return NewStorageError("failed to store document", err).WithOperation("IndexText")
	}

	return nil
}

// IndexFile indexes a file
func (s *DefaultIndexingService) IndexFile(ctx context.Context, filePath, documentID string) error {
	if err := ValidateDocumentID(documentID); err != nil {
		return err
	}

	// Parse document
	result, err := s.parsingService.ParseDocument(ctx, filePath, documentID)
	if err != nil {
		return NewParsingError("failed to parse document", err).WithOperation("IndexFile")
	}

	if len(result.Chunks) == 0 {
		return NewParsingError("no content found in document", nil).WithOperation("IndexFile")
	}

	// Generate embeddings for all chunks
	embeddings := make([][]float32, len(result.Chunks))
	for i, chunk := range result.Chunks {
		embedding, err := s.embedder.Embed(ctx, chunk.Text)
		if err != nil {
			return NewEmbeddingError(fmt.Sprintf("failed to generate embedding for chunk %d", i), err).
				WithOperation("IndexFile").
				WithContext("chunk_index", i)
		}
		embeddings[i] = embedding
	}

	// Store document with chunks and metadata
	if err := s.storage.IndexChunksWithMetadata(ctx, documentID, result.Content, result.Chunks, embeddings,
		result.OriginalPath, string(result.DocumentType)); err != nil {
		return NewStorageError("failed to store document chunks", err).WithOperation("IndexFile")
	}

	return nil
}

// UpdateChunk updates an existing chunk
func (s *DefaultIndexingService) UpdateChunk(ctx context.Context, chunkID, newText string) error {
	if err := ValidateText(newText); err != nil {
		return err
	}

	// Generate new embedding
	embedding, err := s.embedder.Embed(ctx, newText)
	if err != nil {
		return NewEmbeddingError("failed to generate embedding for updated chunk", err).WithOperation("UpdateChunk")
	}

	// Update in storage
	if err := s.storage.UpdateChunk(ctx, chunkID, newText, embedding); err != nil {
		return NewStorageError("failed to update chunk", err).WithOperation("UpdateChunk")
	}

	return nil
}

// DefaultSearchService implements SearchService
type DefaultSearchService struct {
	storage  Storage
	embedder Embedder
}

// NewSearchService creates a new search service
func NewSearchService(storage Storage, embedder Embedder) SearchService {
	return &DefaultSearchService{
		storage:  storage,
		embedder: embedder,
	}
}

// Search performs vector similarity search
func (s *DefaultSearchService) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if err := ValidateSearchQuery(query); err != nil {
		return nil, err
	}
	if err := ValidateSearchLimit(limit); err != nil {
		return nil, err
	}

	// Generate query embedding
	var embedding []float32
	var err error

	// Use enhanced query processing if available
	if ollamaEmbedder, ok := s.embedder.(*OllamaEmbedder); ok {
		embedding, err = ollamaEmbedder.EmbedQuery(ctx, query)
	} else {
		embedding, err = s.embedder.Embed(ctx, query)
	}

	if err != nil {
		return nil, NewEmbeddingError("failed to generate query embedding", err).WithOperation("Search")
	}

	// Perform vector search
	results, err := s.storage.Search(ctx, embedding, limit)
	if err != nil {
		return nil, NewSearchError("vector search failed", err).WithOperation("Search")
	}

	return results, nil
}

// SearchWithContext performs search with additional filters
func (s *DefaultSearchService) SearchWithContext(
	ctx context.Context,
	query string,
	limit int,
	_ map[string]interface{},
) ([]SearchResult, error) {
	// For now, delegate to basic search
	// Future implementation could use filters for refined searching
	return s.Search(ctx, query, limit)
}

// DefaultChatService implements ChatService
type DefaultChatService struct {
	searchService SearchService
	chatClient    *OllamaChatClient
}

// NewChatService creates a new chat service
func NewChatService(searchService SearchService, chatClient *OllamaChatClient) ChatService {
	return &DefaultChatService{
		searchService: searchService,
		chatClient:    chatClient,
	}
}

// Chat performs conversational query with RAG context
func (s *DefaultChatService) Chat(ctx context.Context, message string, limit int) (string, []SearchResult, error) {
	if message == "" {
		return "", nil, NewValidationError("message cannot be empty", nil)
	}
	if limit <= 0 {
		limit = 5 // Default context limit
	}

	// Optimize query for better search
	optimizedQuery, err := s.chatClient.OptimizeQuery(ctx, message)
	if err != nil {
		// Log warning but continue with original message
		optimizedQuery = message
	}

	// Search for relevant context
	searchResults, err := s.searchService.Search(ctx, optimizedQuery, limit)
	if err != nil {
		return "", nil, NewSearchError("failed to search for context", err).WithOperation("Chat")
	}

	// Generate chat response
	response, err := s.chatClient.GenerateResponse(ctx, message, searchResults)
	if err != nil {
		return "", searchResults, NewChatError("failed to generate response", err).WithOperation("Chat")
	}

	return response, searchResults, nil
}

// OptimizeQuery optimizes a user query for better search
func (s *DefaultChatService) OptimizeQuery(ctx context.Context, userMessage string) (string, error) {
	return s.chatClient.OptimizeQuery(ctx, userMessage)
}

// DefaultDocumentService implements DocumentService
type DefaultDocumentService struct {
	storage Storage
}

// NewDocumentService creates a new document service
func NewDocumentService(storage Storage) DocumentService {
	return &DefaultDocumentService{storage: storage}
}

// ListDocuments returns all documents
func (s *DefaultDocumentService) ListDocuments(ctx context.Context) ([]DocumentInfo, error) {
	documents, err := s.storage.ListDocuments(ctx)
	if err != nil {
		return nil, NewStorageError("failed to list documents", err).WithOperation("ListDocuments")
	}
	return documents, nil
}

// GetDocument returns a specific document
func (s *DefaultDocumentService) GetDocument(ctx context.Context, documentID string) (*DocumentInfo, error) {
	if err := ValidateDocumentID(documentID); err != nil {
		return nil, err
	}

	document, err := s.storage.GetDocumentByID(ctx, documentID)
	if err != nil {
		return nil, NewStorageError("failed to get document", err).WithOperation("GetDocument")
	}
	return document, nil
}

// GetDocumentChunks returns chunks for a document
func (s *DefaultDocumentService) GetDocumentChunks(ctx context.Context, documentID string) ([]ChunkInfo, error) {
	if err := ValidateDocumentID(documentID); err != nil {
		return nil, err
	}

	chunks, err := s.storage.GetDocumentChunksWithInfo(ctx, documentID)
	if err != nil {
		return nil, NewStorageError("failed to get document chunks", err).WithOperation("GetDocumentChunks")
	}
	return chunks, nil
}

// DeleteDocument removes a document
func (s *DefaultDocumentService) DeleteDocument(ctx context.Context, documentID string) error {
	if err := ValidateDocumentID(documentID); err != nil {
		return err
	}

	if err := s.storage.DeleteDocument(ctx, documentID); err != nil {
		return NewStorageError("failed to delete document", err).WithOperation("DeleteDocument")
	}
	return nil
}

// DefaultHealthService implements HealthService
type DefaultHealthService struct {
	storage  Storage
	embedder Embedder
}

// NewHealthService creates a new health service
func NewHealthService(storage Storage, embedder Embedder) HealthService {
	return &DefaultHealthService{
		storage:  storage,
		embedder: embedder,
	}
}

// CheckHealth performs system health checks
func (s *DefaultHealthService) CheckHealth(ctx context.Context) error {
	// Check storage connectivity
	if s.storage == nil {
		return NewStorageError("storage not initialized", nil).WithOperation("CheckHealth")
	}

	// Check embedding service
	if s.embedder == nil {
		return NewEmbeddingError("embedder not initialized", nil).WithOperation("CheckHealth")
	}

	// Test embedding generation
	_, err := s.embedder.Embed(ctx, "health check test")
	if err != nil {
		return NewEmbeddingError("embedding service unhealthy", err).WithOperation("CheckHealth")
	}

	return nil
}

// GetMetrics returns system metrics
func (s *DefaultHealthService) GetMetrics(_ context.Context) (map[string]interface{}, error) {
	metrics := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"status":    "healthy",
	}

	// Add basic embedder info if available
	if ollamaEmbedder, ok := s.embedder.(*OllamaEmbedder); ok {
		metrics["embedder"] = map[string]interface{}{
			"type":      "ollama",
			"available": true,
		}
		_ = ollamaEmbedder // Use the variable to avoid linting errors
	}

	return metrics, nil
}

// ServiceFactory creates and configures all services
type ServiceFactory struct {
	config *ServiceConfig
}

// NewServiceFactory creates a new service factory
func NewServiceFactory(config *ServiceConfig) *ServiceFactory {
	return &ServiceFactory{config: config}
}

// CreateServices creates and configures all services
func (f *ServiceFactory) CreateServices(_ context.Context) (*Services, error) {
	// Initialize storage
	storage, err := NewSQLiteStorage(f.config.DatabasePath, f.config.VectorSize, f.config.DataDir)
	if err != nil {
		return nil, NewStorageError("failed to create storage", err)
	}

	if initErr := storage.Initialize(); initErr != nil {
		return nil, NewStorageError("failed to initialize storage", initErr)
	}

	// Initialize embedder
	embedder, err := NewOllamaEmbedderWithTimeout(f.config.OllamaURL, f.config.Model, f.config.TimeoutSeconds)
	if err != nil {
		return nil, NewEmbeddingError("failed to create embedder", err)
	}

	// Initialize text chunker
	chunker := NewTextChunker(f.config.MaxTokens, f.config.Overlap)

	// Initialize parsing service
	parsingService, err := NewDocumentParsingServiceWithVision(chunker, f.config.OllamaURL,
		f.config.VisionModel, f.config.TimeoutSeconds, f.config.ImageMaxSize)
	if err != nil {
		return nil, NewParsingError("failed to create parsing service", err)
	}

	// Initialize chat client
	chatClient := NewOllamaChatClientWithTimeout(f.config.OllamaURL, f.config.ChatModel, f.config.TimeoutSeconds*4)

	// Create services
	indexingService := NewIndexingService(storage, embedder, parsingService)
	searchService := NewSearchService(storage, embedder)
	chatService := NewChatService(searchService, chatClient)
	documentService := NewDocumentService(storage)
	healthService := NewHealthService(storage, embedder)

	return &Services{
		Indexing: indexingService,
		Search:   searchService,
		Chat:     chatService,
		Document: documentService,
		Health:   healthService,
	}, nil
}
