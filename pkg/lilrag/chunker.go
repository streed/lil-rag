package lilrag

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// Content type constants
const (
	ContentTypeCode       = "code"
	ContentTypeProse      = "prose"
	ContentTypeStructured = "structured"
)

type TextChunker struct {
	MaxTokens  int
	Overlap    int
	TokenRegex *regexp.Regexp
}

type Chunk struct {
	Text       string
	Index      int
	StartPos   int
	EndPos     int
	TokenCount int
	PageNumber *int   // Optional page number for PDF chunks
	ChunkType  string // Type of chunk: "text", "pdf_page"
}

func NewTextChunker(maxTokens, overlap int) *TextChunker {
	// Simple tokenization regex - splits on whitespace
	tokenRegex := regexp.MustCompile(`\S+`)

	return &TextChunker{
		MaxTokens:  maxTokens,
		Overlap:    overlap,
		TokenRegex: tokenRegex,
	}
}

func (tc *TextChunker) EstimateTokenCount(text string) int {
	return len(tc.TokenRegex.FindAllString(text, -1))
}

func (tc *TextChunker) ChunkText(text string) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	tokenCount := tc.EstimateTokenCount(text)
	contentType := tc.detectContentType(text)

	// For very small texts (under 100 tokens), return as single chunk unless MaxTokens is very small
	if tokenCount <= 100 && tc.MaxTokens >= 100 {
		return []Chunk{
			{
				Text:       text,
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				TokenCount: tokenCount,
				ChunkType:  contentType,
			},
		}
	}

	// Apply semantic chunking for ALL documents to get optimal boundaries
	// Even small documents benefit from content-type aware processing
	semanticChunks := tc.adaptiveChunk(text, contentType)

	// If semantic chunking produces only one chunk that fits in token limit,
	// we still benefit from the content-type detection and boundary analysis
	if len(semanticChunks) == 1 && semanticChunks[0].TokenCount <= tc.MaxTokens {
		return semanticChunks
	}

	// For larger documents or multiple semantic chunks, apply full processing
	return semanticChunks
}

// detectContentType analyzes text to determine optimal chunking strategy
func (tc *TextChunker) detectContentType(text string) string {
	codeIndicators := []string{"function", "class", "def ", "```", "import ", "#include", "var ", "let ", "const "}
	structuredIndicators := []string{"# ", "## ", "### ", "- ", "* ", "1. ", "2. "}

	for _, indicator := range codeIndicators {
		if strings.Contains(text, indicator) {
			return ContentTypeCode
		}
	}

	structureCount := 0
	for _, indicator := range structuredIndicators {
		if strings.Contains(text, indicator) {
			structureCount++
		}
	}

	if structureCount > 2 {
		return ContentTypeStructured
	}

	if strings.Count(text, "\n\n") > len(text)/500 {
		return ContentTypeProse
	}

	return "text"
}

// adaptiveChunk applies content-aware chunking strategies with optimal sizing
func (tc *TextChunker) adaptiveChunk(text, contentType string) []Chunk {
	var sentences []string
	var targetChunkSize int

	// Research-based optimal chunk sizes for different content types
	switch contentType {
	case ContentTypeCode:
		sentences = tc.splitByCodeBlocks(text)
		targetChunkSize = int(float64(tc.MaxTokens) * 1.5) // 384 tokens - larger for code context
	case ContentTypeStructured:
		sentences = tc.splitIntoSentences(text)
		targetChunkSize = tc.MaxTokens // 256 tokens - optimal for structured docs
	case ContentTypeProse:
		sentences = tc.splitByParagraphs(text)
		targetChunkSize = int(float64(tc.MaxTokens) * 1.25) // 320 tokens - medium for narratives
	default:
		sentences = tc.splitIntoSentences(text)
		targetChunkSize = tc.MaxTokens // 256 tokens - default optimal size
	}

	if len(sentences) == 0 {
		return nil
	}

	return tc.buildChunksWithSmartOverlap(sentences, targetChunkSize, contentType)
}

// buildChunksWithSmartOverlap creates chunks with context-aware overlap
func (tc *TextChunker) buildChunksWithSmartOverlap(sentences []string, targetSize int, contentType string) []Chunk {
	var chunks []Chunk
	var currentChunk strings.Builder
	var currentTokenCount int
	chunkIndex := 0

	// Adaptive overlap based on content type
	overlapRatio := tc.getOverlapRatio(contentType)
	dynamicOverlap := int(float64(tc.Overlap) * overlapRatio)

	for i, sentence := range sentences {
		sentenceTokens := tc.EstimateTokenCount(sentence)

		// Check if we need to start a new chunk
		if tc.shouldStartNewChunk(currentTokenCount, sentenceTokens, targetSize) {
			if currentChunk.Len() > 0 {
				// Finalize current chunk
				chunkText := strings.TrimSpace(currentChunk.String())
				if chunkText != "" {
					chunks = append(chunks, Chunk{
						Text:       chunkText,
						Index:      chunkIndex,
						StartPos:   0,
						EndPos:     len(chunkText),
						TokenCount: currentTokenCount,
						ChunkType:  contentType,
					})
					chunkIndex++
				}

				// Start new chunk with smart overlap
				currentChunk.Reset()
				currentTokenCount = 0

				// Add contextual overlap
				if dynamicOverlap > 0 && len(chunks) > 0 {
					overlapText := tc.getContextualOverlap(sentences, i, dynamicOverlap, contentType)
					if overlapText != "" {
						currentChunk.WriteString(overlapText)
						currentChunk.WriteString("\n")
						currentTokenCount = tc.EstimateTokenCount(overlapText)
					}
				}
			}
		}

		// Add current sentence to chunk
		if currentChunk.Len() > 0 {
			separator := tc.getSeparator(contentType)
			currentChunk.WriteString(separator)
		}
		currentChunk.WriteString(sentence)
		currentTokenCount += sentenceTokens
	}

	// Add final chunk
	if currentChunk.Len() > 0 {
		chunkText := strings.TrimSpace(currentChunk.String())
		if chunkText != "" {
			chunks = append(chunks, Chunk{
				Text:       chunkText,
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     len(chunkText),
				TokenCount: currentTokenCount,
				ChunkType:  contentType,
			})
		}
	}

	// Post-process oversized chunks
	return tc.handleOversizedChunks(chunks)
}

// Helper functions for adaptive chunking
func (tc *TextChunker) shouldStartNewChunk(currentTokens, newTokens, targetSize int) bool {
	return currentTokens > 0 && currentTokens+newTokens > targetSize
}

func (tc *TextChunker) getOverlapRatio(contentType string) float64 {
	switch contentType {
	case ContentTypeCode:
		return 1.2 // More overlap for code context
	case ContentTypeStructured:
		return 0.8 // Less overlap for lists/headers
	case ContentTypeProse:
		return 1.0 // Standard overlap for narrative text
	default:
		return 1.0
	}
}

func (tc *TextChunker) getSeparator(contentType string) string {
	switch contentType {
	case ContentTypeCode:
		return "\n"
	case ContentTypeStructured:
		return "\n"
	default:
		return " "
	}
}

func (tc *TextChunker) getContextualOverlap(
	sentences []string, currentIndex, overlapTokens int, contentType string,
) string {
	if currentIndex == 0 || overlapTokens == 0 {
		return ""
	}

	var overlapSentences []string
	tokenCount := 0

	// For code, prioritize function/class definitions in overlap
	if contentType == ContentTypeCode {
		for i := currentIndex - 1; i >= 0 && tokenCount < overlapTokens; i-- {
			sentence := sentences[i]
			sentenceTokens := tc.EstimateTokenCount(sentence)

			// Prioritize important code constructs
			if tc.isImportantCodeConstruct(sentence) || tokenCount+sentenceTokens <= overlapTokens {
				overlapSentences = append(overlapSentences, sentence)
				tokenCount += sentenceTokens
			}
		}
	} else {
		// Standard overlap for other content types
		for i := currentIndex - 1; i >= 0 && tokenCount < overlapTokens; i-- {
			sentence := sentences[i]
			sentenceTokens := tc.EstimateTokenCount(sentence)
			if tokenCount+sentenceTokens <= overlapTokens {
				overlapSentences = append(overlapSentences, sentence)
				tokenCount += sentenceTokens
			} else {
				break
			}
		}
	}

	// Reverse to maintain order
	for i, j := 0, len(overlapSentences)-1; i < j; i, j = i+1, j-1 {
		overlapSentences[i], overlapSentences[j] = overlapSentences[j], overlapSentences[i]
	}

	separator := tc.getSeparator(contentType)
	return strings.Join(overlapSentences, separator)
}

func (tc *TextChunker) isImportantCodeConstruct(text string) bool {
	importantPatterns := []string{"function", "class", "def ", "interface", "type ", "struct"}
	text = strings.ToLower(text)
	for _, pattern := range importantPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func (tc *TextChunker) handleOversizedChunks(chunks []Chunk) []Chunk {
	var finalChunks []Chunk

	for _, chunk := range chunks {
		if chunk.TokenCount <= tc.MaxTokens {
			finalChunks = append(finalChunks, chunk)
		} else {
			// Split oversized chunks with preserved semantics
			subChunks := tc.splitOversizedChunk(chunk)
			finalChunks = append(finalChunks, subChunks...)
		}
	}

	// Re-index all chunks
	for i := range finalChunks {
		finalChunks[i].Index = i
	}

	return finalChunks
}

func (tc *TextChunker) splitOversizedChunk(chunk Chunk) []Chunk {
	if chunk.ChunkType == ContentTypeCode {
		return tc.splitCodeChunk(chunk)
	}
	return tc.splitLongChunkByWords(chunk)
}

func (tc *TextChunker) splitCodeChunk(chunk Chunk) []Chunk {
	lines := strings.Split(chunk.Text, "\n")
	var chunks []Chunk
	var currentChunk strings.Builder
	var currentTokens int

	for _, line := range lines {
		lineTokens := tc.EstimateTokenCount(line)

		if currentTokens > 0 && currentTokens+lineTokens > tc.MaxTokens {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, Chunk{
					Text:       strings.TrimSpace(currentChunk.String()),
					TokenCount: currentTokens,
					ChunkType:  "code",
				})
			}
			currentChunk.Reset()
			currentTokens = 0
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
		currentTokens += lineTokens
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, Chunk{
			Text:       strings.TrimSpace(currentChunk.String()),
			TokenCount: currentTokens,
			ChunkType:  "code",
		})
	}

	return chunks
}

func (tc *TextChunker) splitIntoSentences(text string) []string {
	// Enhanced semantic boundary detection for optimal chunking

	// First try semantic boundaries (paragraphs) - highest priority for semantic coherence
	paragraphs := tc.splitByParagraphs(text)
	if len(paragraphs) > 1 {
		return paragraphs
	}

	// Then try sentence boundaries with improved patterns
	sentences := tc.splitBySentences(text)
	if len(sentences) > 1 {
		return sentences
	}

	// For code or structured content, try logical separators
	codeBlocks := tc.splitByCodeBlocks(text)
	if len(codeBlocks) > 1 {
		return codeBlocks
	}

	// Fallback to whitespace-based splitting for dense text
	parts := tc.splitByWhitespace(text)
	return parts
}

// splitByParagraphs prioritizes paragraph boundaries for semantic coherence
func (tc *TextChunker) splitByParagraphs(text string) []string {
	// Split on double newlines (paragraph breaks)
	paragraphs := strings.Split(text, "\n\n")

	// First pass: collect non-empty paragraphs
	var tempParagraphs []string
	for _, para := range paragraphs {
		cleaned := strings.TrimSpace(para)
		if cleaned != "" {
			tempParagraphs = append(tempParagraphs, cleaned)
		}
	}

	// Second pass: apply punctuation rules
	var cleanParagraphs []string
	for i, para := range tempParagraphs {
		// Remove trailing punctuation from all paragraphs except the last one
		if i < len(tempParagraphs)-1 && strings.HasSuffix(para, ".") {
			para = strings.TrimSuffix(para, ".")
			para = strings.TrimSpace(para)
		}
		cleanParagraphs = append(cleanParagraphs, para)
	}

	return cleanParagraphs
}

// splitBySentences uses improved sentence detection patterns
func (tc *TextChunker) splitBySentences(text string) []string {
	// Enhanced sentence patterns including abbreviations awareness
	sentenceRegex := regexp.MustCompile(`[.!?](?:\s+|$)`)

	// Find all sentence boundaries
	matches := sentenceRegex.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		// No sentence boundaries found
		cleaned := strings.TrimSpace(text)
		if cleaned != "" {
			return []string{cleaned}
		}
		return []string{}
	}

	var sentences []string
	lastEnd := 0

	for i, match := range matches {
		// Extract sentence including the punctuation for the last sentence
		start := lastEnd
		var end int
		if i == len(matches)-1 {
			// Last sentence - include the punctuation
			end = match[1]
		} else {
			// Middle sentences - exclude the punctuation
			end = match[0]
		}

		sentence := strings.TrimSpace(text[start:end])
		if sentence != "" { // Allow shorter sentences for tests
			sentences = append(sentences, sentence)
		}
		lastEnd = match[1]
	}

	return sentences
}

// splitByCodeBlocks handles code snippets and structured content
func (tc *TextChunker) splitByCodeBlocks(text string) []string {
	// Split on code block patterns, function definitions, or structured separators
	codePattern := regexp.MustCompile("(?:\\n```|\\n---|\\nfunction |\\nclass |\\n#+ |\\n\\* )")

	blocks := codePattern.Split(text, -1)

	var cleanBlocks []string
	for _, block := range blocks {
		cleaned := strings.TrimSpace(block)
		if cleaned != "" && len(cleaned) > 20 {
			cleanBlocks = append(cleanBlocks, cleaned)
		}
	}

	return cleanBlocks
}

// splitByWhitespace falls back to whitespace-based chunking
func (tc *TextChunker) splitByWhitespace(text string) []string {
	// Split by significant whitespace gaps
	parts := regexp.MustCompile(`\s{3,}|\n\s*\n`).Split(text, -1)

	var cleanParts []string
	for _, part := range parts {
		cleaned := strings.TrimSpace(part)
		if cleaned != "" {
			cleanParts = append(cleanParts, cleaned)
		}
	}

	// If still no good splits, return as single part
	if len(cleanParts) <= 1 {
		return []string{strings.TrimSpace(text)}
	}

	return cleanParts
}

func (tc *TextChunker) getOverlapText(sentences []string, currentIndex, overlapTokens int) string {
	// Delegate to contextual overlap with default content type
	return tc.getContextualOverlap(sentences, currentIndex, overlapTokens, "text")
}

func (tc *TextChunker) findStartPosition(text, sentence string) int {
	index := strings.Index(text, sentence)
	if index != -1 {
		return index
	}
	return 0
}

func (tc *TextChunker) splitLongChunkByWords(chunk Chunk) []Chunk {
	words := strings.Fields(chunk.Text)
	if len(words) <= tc.MaxTokens {
		return []Chunk{chunk}
	}

	var chunks []Chunk

	for i := 0; i < len(words); i += tc.MaxTokens - tc.Overlap {
		end := i + tc.MaxTokens
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		chunkText := strings.Join(chunkWords, " ")

		chunks = append(chunks, Chunk{
			Text:       chunkText,
			Index:      0,                  // Will be re-indexed by the caller
			StartPos:   chunk.StartPos + i, // Approximate start position
			EndPos:     chunk.StartPos + i + len(chunkText),
			TokenCount: len(chunkWords),
		})

		// Break if we've covered all words
		if end >= len(words) {
			break
		}
	}

	return chunks
}

// IsLongText checks if text needs chunking
func (tc *TextChunker) IsLongText(text string) bool {
	return tc.EstimateTokenCount(text) > tc.MaxTokens
}

// GetChunkID generates a unique ID for a chunk
func GetChunkID(documentID string, chunkIndex int) string {
	if chunkIndex == 0 {
		return documentID
	}
	return fmt.Sprintf("%s_chunk_%d", documentID, chunkIndex)
}

// GenerateDocumentID generates a human-readable document ID when none is provided
func GenerateDocumentID() string {
	// List of friendly adjectives and nouns for human-readable IDs
	adjectives := []string{
		"happy", "bright", "swift", "clever", "gentle", "bold", "calm", "wise",
		"brave", "quick", "sharp", "smart", "clean", "fresh", "light", "clear",
	}

	nouns := []string{
		"doc", "file", "text", "note", "page", "item", "data", "content",
		"record", "entry", "memo", "paper", "sheet", "digest", "brief", "piece",
	}

	// Use current time for uniqueness and randomness for variety
	now := time.Now()
	r := rand.New(rand.NewSource(now.UnixNano()))

	adjective := adjectives[r.Intn(len(adjectives))]
	noun := nouns[r.Intn(len(nouns))]

	// Create timestamp suffix for uniqueness (YYMMDD-HHMM format for brevity)
	timestamp := now.Format("060102-1504")

	return fmt.Sprintf("%s-%s-%s", adjective, noun, timestamp)
}
