package lilrag

import (
	"os"
)

// TextParser handles plain text and markdown files
type TextParser struct {
	chunker *TextChunker
}

// NewTextParser creates a new text parser
func NewTextParser() *TextParser {
	return &TextParser{}
}

// Parse extracts text content from a text file
func (tp *TextParser) Parse(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ParseWithChunks extracts and chunks content from a text file
func (tp *TextParser) ParseWithChunks(filePath, _ string) ([]Chunk, error) {
	content, err := tp.Parse(filePath)
	if err != nil {
		return nil, err
	}

	// Use a default chunker if none provided
	if tp.chunker == nil {
		tp.chunker = NewTextChunker(256, 38) // Use optimized defaults
	}

	return tp.chunker.ChunkText(content), nil
}

// SupportedExtensions returns the file extensions this parser supports
func (tp *TextParser) SupportedExtensions() []string {
	return []string{".txt", ".md", ".text"}
}

// GetDocumentType returns the type of documents this parser handles
func (tp *TextParser) GetDocumentType() DocumentType {
	return DocumentTypeTXT
}
