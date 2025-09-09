package lilrag

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Parser defines the interface for document parsers - aliased to existing DocumentParser
type Parser = DocumentParser

// VisionParser extends Parser for parsers that use vision models
type VisionParser interface {
	Parser

	// SetVisionConfig configures the vision model settings
	SetVisionConfig(ollamaURL, visionModel string, timeoutSeconds, imageMaxSize int) error
}

// ParserRegistry manages available document parsers
type ParserRegistry struct {
	parsers []Parser
}

// NewParserRegistry creates a new parser registry with default parsers
func NewParserRegistry() *ParserRegistry {
	registry := &ParserRegistry{}

	// Register default parsers using existing constructors
	registry.Register(&TextParser{})
	registry.Register(NewPDFParser())
	registry.Register(&HTMLParser{})
	registry.Register(&CSVParser{})
	registry.Register(&DOCXParser{})
	registry.Register(&XLSXParser{})

	return registry
}

// NewParserRegistryWithVision creates a parser registry including vision-enabled parsers
func NewParserRegistryWithVision(
	ollamaURL, visionModel string,
	timeoutSeconds, imageMaxSize int,
) (*ParserRegistry, error) {
	registry := NewParserRegistry()

	// Add vision parser using existing constructor with default chunker
	chunker := NewTextChunker(256, 38)
	imageParser := NewImageParserWithTimeout(ollamaURL, visionModel, chunker, timeoutSeconds, imageMaxSize)
	registry.Register(imageParser)

	return registry, nil
}

// Register adds a parser to the registry
func (r *ParserRegistry) Register(parser Parser) {
	r.parsers = append(r.parsers, parser)
}

// GetParser returns the appropriate parser for a file
func (r *ParserRegistry) GetParser(filePath string) Parser {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, parser := range r.parsers {
		for _, supportedExt := range parser.SupportedExtensions() {
			if strings.EqualFold(ext, supportedExt) {
				return parser
			}
		}
	}
	return nil
}

// GetSupportedExtensions returns all supported file extensions
func (r *ParserRegistry) GetSupportedExtensions() []string {
	var extensions []string
	extensionMap := make(map[string]bool)

	for _, parser := range r.parsers {
		// Use the SupportedExtensions method from the DocumentParser interface
		for _, ext := range parser.SupportedExtensions() {
			if !extensionMap[ext] {
				extensions = append(extensions, ext)
				extensionMap[ext] = true
			}
		}
	}

	return extensions
}

// IsSupported checks if a file type is supported
func (r *ParserRegistry) IsSupported(filePath string) bool {
	return r.GetParser(filePath) != nil
}

// DetectDocumentType determines the document type from file path
func DetectDocumentType(filePath string) DocumentType {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ExtPDF:
		return DocumentTypePDF
	case ".docx":
		return DocumentTypeDOCX
	case ".xlsx", ".xls":
		return DocumentTypeXLSX
	case ".html", ".htm":
		return DocumentTypeHTML
	case ".csv":
		return DocumentTypeCSV
	case ExtJPG, ExtJPEG, ExtPNG, ExtGIF, ExtBMP, ExtWEBP:
		return DocumentTypeImage
	default:
		return DocumentTypeTXT
	}
}

// ParseResult contains the result of parsing a document
type ParseResult struct {
	Content      string
	Chunks       []Chunk
	DocumentType DocumentType
	OriginalPath string
	Metadata     map[string]interface{}
}

// DocumentParsingService provides high-level document parsing operations
type DocumentParsingService struct {
	registry *ParserRegistry
	chunker  *TextChunker
}

// NewDocumentParsingService creates a new document parsing service
func NewDocumentParsingService(chunker *TextChunker) *DocumentParsingService {
	return &DocumentParsingService{
		registry: NewParserRegistry(),
		chunker:  chunker,
	}
}

// NewDocumentParsingServiceWithVision creates a document parsing service with vision support
func NewDocumentParsingServiceWithVision(
	chunker *TextChunker,
	ollamaURL, visionModel string,
	timeoutSeconds, imageMaxSize int,
) (*DocumentParsingService, error) {
	registry, err := NewParserRegistryWithVision(ollamaURL, visionModel, timeoutSeconds, imageMaxSize)
	if err != nil {
		return nil, err
	}

	return &DocumentParsingService{
		registry: registry,
		chunker:  chunker,
	}, nil
}

// ParseDocument parses a document and returns structured results
func (s *DocumentParsingService) ParseDocument(_ context.Context, filePath, documentID string) (*ParseResult, error) {
	parser := s.registry.GetParser(filePath)
	if parser == nil {
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}

	// Parse content
	content, err := parser.Parse(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	// Create chunks
	chunks, err := parser.ParseWithChunks(filePath, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create chunks: %w", err)
	}

	return &ParseResult{
		Content:      content,
		Chunks:       chunks,
		DocumentType: parser.GetDocumentType(),
		OriginalPath: filePath,
		Metadata: map[string]interface{}{
			"parser_type": string(parser.GetDocumentType()),
			"chunk_count": len(chunks),
		},
	}, nil
}

// GetSupportedExtensions returns supported file extensions
func (s *DocumentParsingService) GetSupportedExtensions() []string {
	return s.registry.GetSupportedExtensions()
}

// IsSupported checks if a file is supported
func (s *DocumentParsingService) IsSupported(filePath string) bool {
	return s.registry.IsSupported(filePath)
}
