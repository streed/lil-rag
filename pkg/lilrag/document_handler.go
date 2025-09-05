package lilrag

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DocumentType represents the type of document being processed
type DocumentType string

const (
	DocumentTypePDF     DocumentType = "pdf"
	DocumentTypeDOCX    DocumentType = "docx"
	DocumentTypeXLSX    DocumentType = "xlsx"
	DocumentTypePPTX    DocumentType = "pptx"
	DocumentTypeHTML    DocumentType = "html"
	DocumentTypeCSV     DocumentType = "csv"
	DocumentTypeTXT     DocumentType = "txt"
	DocumentTypeODT     DocumentType = "odt"
	DocumentTypeImage   DocumentType = "image"
	DocumentTypeUnknown DocumentType = "unknown"
)

// DocumentParser is the interface that all document parsers must implement
type DocumentParser interface {
	// Parse extracts text content from a document file
	Parse(filePath string) (string, error)

	// ParseWithChunks extracts and chunks content optimally for the document type
	ParseWithChunks(filePath, documentID string) ([]Chunk, error)

	// SupportedExtensions returns the file extensions this parser supports
	SupportedExtensions() []string

	// DocumentType returns the type of documents this parser handles
	GetDocumentType() DocumentType
}

// DocumentHandler manages all document parsers and routes files to appropriate handlers
type DocumentHandler struct {
	parsers        map[DocumentType]DocumentParser
	chunker        *TextChunker
	ollamaURL      string
	visionModel    string
	timeoutSeconds int
}

// NewDocumentHandler creates a new document handler with all supported parsers
func NewDocumentHandler(chunker *TextChunker) *DocumentHandler {
	return NewDocumentHandlerWithVision(chunker, DefaultOllamaURL, "llama3.2-vision")
}

// NewDocumentHandlerWithVision creates a document handler with custom vision model settings
func NewDocumentHandlerWithVision(chunker *TextChunker, ollamaURL, visionModel string) *DocumentHandler {
	return NewDocumentHandlerWithVisionAndTimeout(chunker, ollamaURL, visionModel, 300)
}

// NewDocumentHandlerWithVisionAndTimeout creates a document handler with custom vision model and timeout settings
func NewDocumentHandlerWithVisionAndTimeout(chunker *TextChunker, ollamaURL, visionModel string, timeoutSeconds int) *DocumentHandler {
	dh := &DocumentHandler{
		parsers:        make(map[DocumentType]DocumentParser),
		chunker:        chunker,
		ollamaURL:      ollamaURL,
		visionModel:    visionModel,
		timeoutSeconds: timeoutSeconds,
	}

	// Register default parsers
	dh.registerDefaultParsers()

	return dh
}

// registerDefaultParsers registers all built-in document parsers
func (dh *DocumentHandler) registerDefaultParsers() {
	// PDF parser (already exists)
	dh.RegisterParser(DocumentTypePDF, NewPDFParser())

	// Text parser (already exists as fallback)
	dh.RegisterParser(DocumentTypeTXT, NewTextParser())

	// Microsoft Office document parsers
	dh.RegisterParser(DocumentTypeDOCX, NewDOCXParser())
	dh.RegisterParser(DocumentTypeXLSX, NewXLSXParser())
	// dh.RegisterParser(DocumentTypePPTX, NewPPTXParser()) // TODO: Implement PPTX parser

	// Web and data format parsers
	dh.RegisterParser(DocumentTypeHTML, NewHTMLParser())
	dh.RegisterParser(DocumentTypeCSV, NewCSVParser())

	// Image parser with OCR capabilities
	dh.RegisterParser(DocumentTypeImage, NewImageParserWithTimeout(dh.ollamaURL, dh.visionModel, dh.chunker, dh.timeoutSeconds*10))

	// Future parsers will be added here
	// dh.RegisterParser(DocumentTypeODT, NewODTParser())
}

// RegisterParser registers a new document parser
func (dh *DocumentHandler) RegisterParser(docType DocumentType, parser DocumentParser) {
	dh.parsers[docType] = parser
}

// DetectDocumentType determines the document type from file extension
func (dh *DocumentHandler) DetectDocumentType(filePath string) DocumentType {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".pdf":
		return DocumentTypePDF
	case ".docx":
		return DocumentTypeDOCX
	case ".xlsx":
		return DocumentTypeXLSX
	case ".pptx":
		return DocumentTypePPTX
	case ".html", ".htm":
		return DocumentTypeHTML
	case ".csv":
		return DocumentTypeCSV
	case ".txt", ".md":
		return DocumentTypeTXT
	case ".odt":
		return DocumentTypeODT
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif":
		return DocumentTypeImage
	default:
		return DocumentTypeUnknown
	}
}

// ParseFile parses any supported document file
func (dh *DocumentHandler) ParseFile(filePath string) (string, error) {
	docType := dh.DetectDocumentType(filePath)

	parser, exists := dh.parsers[docType]
	if !exists {
		// Fallback to text parser for unknown types
		parser = dh.parsers[DocumentTypeTXT]
		if parser == nil {
			return "", fmt.Errorf("no parser available for document type: %s", docType)
		}
	}

	return parser.Parse(filePath)
}

// ParseFileWithChunks parses and chunks any supported document file
func (dh *DocumentHandler) ParseFileWithChunks(filePath, documentID string) ([]Chunk, error) {
	docType := dh.DetectDocumentType(filePath)

	parser, exists := dh.parsers[docType]
	if !exists {
		// Fallback to text parser for unknown types
		parser = dh.parsers[DocumentTypeTXT]
		if parser == nil {
			return nil, fmt.Errorf("no parser available for document type: %s", docType)
		}
	}

	return parser.ParseWithChunks(filePath, documentID)
}

// GetSupportedFormats returns all supported document formats
func (dh *DocumentHandler) GetSupportedFormats() map[DocumentType][]string {
	formats := make(map[DocumentType][]string)

	for docType, parser := range dh.parsers {
		formats[docType] = parser.SupportedExtensions()
	}

	return formats
}

// IsSupported checks if a file type is supported
func (dh *DocumentHandler) IsSupported(filePath string) bool {
	docType := dh.DetectDocumentType(filePath)
	_, exists := dh.parsers[docType]
	return exists || dh.parsers[DocumentTypeTXT] != nil // Text parser as fallback
}
