package lilrag

import (
	"fmt"
	"strings"

	"github.com/nguyenthenguyen/docx"
)

// DOCXParser handles Microsoft Word .docx files
type DOCXParser struct {
	chunker *TextChunker
}

// NewDOCXParser creates a new DOCX parser
func NewDOCXParser() *DOCXParser {
	return &DOCXParser{}
}

// Parse extracts text content from a DOCX file
func (dp *DOCXParser) Parse(filePath string) (string, error) {
	r, err := docx.ReadDocxFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open DOCX file: %w", err)
	}
	defer r.Close()

	docData := r.Editable()
	return docData.GetContent(), nil
}

// ParseWithChunks extracts and chunks content from a DOCX file
func (dp *DOCXParser) ParseWithChunks(filePath, documentID string) ([]Chunk, error) {
	content, err := dp.Parse(filePath)
	if err != nil {
		return nil, err
	}

	// Use a default chunker if none provided
	if dp.chunker == nil {
		dp.chunker = NewTextChunker(320, 48) // Slightly larger chunks for prose content
	}

	// Clean up the content
	content = dp.cleanContent(content)
	
	// Detect content type for better chunking
	contentType := dp.detectContentType(content)
	
	// Chunk the content based on type
	chunks := dp.chunkByContentType(content, contentType)
	
	// Update chunk metadata
	for i, chunk := range chunks {
		chunk.Index = i
		chunk.ChunkType = fmt.Sprintf("docx_%s", contentType)
		chunks[i] = chunk
	}

	return chunks, nil
}

// cleanContent removes excessive whitespace and normalizes text
func (dp *DOCXParser) cleanContent(content string) string {
	// Replace multiple whitespace with single spaces
	lines := strings.Split(content, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	
	return strings.Join(cleanLines, "\n")
}

// detectContentType analyzes content to determine optimal chunking strategy
func (dp *DOCXParser) detectContentType(content string) string {
	// Count indicators of different content types
	codePatterns := []string{"function", "class", "def ", "import ", "var ", "const ", "let ", "public ", "private "}
	structuredPatterns := []string{"# ", "## ", "### ", "- ", "* ", "1. ", "2. ", "3. "}
	
	codeScore := 0
	structuredScore := 0
	
	for _, pattern := range codePatterns {
		codeScore += strings.Count(strings.ToLower(content), pattern)
	}
	
	for _, pattern := range structuredPatterns {
		structuredScore += strings.Count(content, pattern)
	}
	
	if codeScore > 3 {
		return "code"
	} else if structuredScore > 3 {
		return "structured"
	} else {
		return "prose"
	}
}

// chunkByContentType applies content-type specific chunking
func (dp *DOCXParser) chunkByContentType(content string, contentType string) []Chunk {
	switch contentType {
	case "code":
		return dp.chunkCodeContent(content)
	case "structured":
		return dp.chunkStructuredContent(content)
	default:
		return dp.chunkProseContent(content)
	}
}

// chunkCodeContent chunks content that appears to be code or technical documentation
func (dp *DOCXParser) chunkCodeContent(content string) []Chunk {
	// Look for code blocks or function definitions
	paragraphs := strings.Split(content, "\n\n")
	var chunks []Chunk
	var currentChunk strings.Builder
	currentTokens := 0
	targetSize := int(float64(dp.chunker.MaxTokens) * 1.5) // Larger chunks for code

	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			continue
		}

		paraTokens := dp.chunker.EstimateTokenCount(paragraph)
		
		if currentTokens+paraTokens > targetSize && currentChunk.Len() > 0 {
			// Create chunk
			chunk := Chunk{
				Text:       strings.TrimSpace(currentChunk.String()),
				StartPos:   0,
				EndPos:     currentChunk.Len(),
				TokenCount: currentTokens,
			}
			chunks = append(chunks, chunk)
			
			// Reset
			currentChunk.Reset()
			currentTokens = 0
		}
		
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(paragraph)
		currentTokens += paraTokens
	}
	
	// Add final chunk
	if currentChunk.Len() > 0 {
		chunk := Chunk{
			Text:       strings.TrimSpace(currentChunk.String()),
			StartPos:   0,
			EndPos:     currentChunk.Len(),
			TokenCount: currentTokens,
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks
}

// chunkStructuredContent chunks content with headers and lists
func (dp *DOCXParser) chunkStructuredContent(content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var currentChunk strings.Builder
	currentTokens := 0
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		lineTokens := dp.chunker.EstimateTokenCount(line)
		
		// Check if this is a header (new section)
		if dp.isHeader(line) && currentChunk.Len() > 0 {
			// Save current chunk
			chunk := Chunk{
				Text:       strings.TrimSpace(currentChunk.String()),
				StartPos:   0,
				EndPos:     currentChunk.Len(),
				TokenCount: currentTokens,
			}
			chunks = append(chunks, chunk)
			
			// Reset for new section
			currentChunk.Reset()
			currentTokens = 0
		}
		
		// Add line to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
		currentTokens += lineTokens
		
		// Check if chunk is getting too large
		if currentTokens > dp.chunker.MaxTokens {
			chunk := Chunk{
				Text:       strings.TrimSpace(currentChunk.String()),
				StartPos:   0,
				EndPos:     currentChunk.Len(),
				TokenCount: currentTokens,
			}
			chunks = append(chunks, chunk)
			
			currentChunk.Reset()
			currentTokens = 0
		}
	}
	
	// Add final chunk
	if currentChunk.Len() > 0 {
		chunk := Chunk{
			Text:       strings.TrimSpace(currentChunk.String()),
			StartPos:   0,
			EndPos:     currentChunk.Len(),
			TokenCount: currentTokens,
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks
}

// chunkProseContent chunks regular prose/narrative content
func (dp *DOCXParser) chunkProseContent(content string) []Chunk {
	// Use paragraph-based chunking for prose
	return dp.chunker.ChunkText(content)
}

// isHeader checks if a line is likely a header
func (dp *DOCXParser) isHeader(line string) bool {
	// Simple header detection
	if len(line) < 3 {
		return false
	}
	
	// Check for markdown-style headers
	if strings.HasPrefix(line, "#") {
		return true
	}
	
	// Check for numbered headers
	if strings.Contains(line, ".") && len(strings.Fields(line)) <= 8 {
		firstWord := strings.Fields(line)[0]
		if strings.Contains(firstWord, ".") && len(firstWord) < 6 {
			return true
		}
	}
	
	// Check for ALL CAPS headers (short lines)
	if line == strings.ToUpper(line) && len(line) < 50 && !strings.Contains(line, " ") {
		return true
	}
	
	return false
}

// SupportedExtensions returns the file extensions this parser supports
func (dp *DOCXParser) SupportedExtensions() []string {
	return []string{".docx"}
}

// GetDocumentType returns the type of documents this parser handles
func (dp *DOCXParser) GetDocumentType() DocumentType {
	return DocumentTypeDOCX
}