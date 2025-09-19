package lilrag

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tiktoken-go/tokenizer"
)

// Content type constants
const (
	ContentTypeCode       = "code"
	ContentTypeProse      = "prose"
	ContentTypeStructured = "structured"
)

// Chunking method constants
const (
	ChunkingMethodRecursive = "recursive" // Hierarchical splitting with semantic boundaries
	ChunkingMethodSimple    = "simple"    // Simple token-based chunking with overlap
	ChunkingMethodSemantic  = "semantic"  // Embedding-based semantic similarity chunking
	ChunkingMethodAuto      = "auto"      // Automatically choose best method based on content
)

// Chunk type constants
const (
	ChunkTypeText        = "text"
	ChunkTypeHTMLSection = "html_section"
)

type TextChunker struct {
	MaxTokens  int
	Overlap    int
	TokenRegex *regexp.Regexp
	Method     string                     // Chunking method to use
	Tokenizer  tokenizer.Codec           // tiktoken tokenizer for accurate token counting
	Embedder   Embedder                  // embedder for semantic chunking (optional)
}

type Chunk struct {
	Text         string
	Index        int
	StartPos     int // Start position in the original text (including overlap)
	EndPos       int // End position in the original text (including overlap)
	ContentStart int // Start position of non-overlapping content within this chunk
	ContentEnd   int // End position of non-overlapping content within this chunk
	TokenCount   int
	PageNumber   *int   // Optional page number for PDF chunks
	ChunkType    string // Type of chunk: "text", "pdf_page"
}

func NewTextChunker(maxTokens, overlap int) *TextChunker {
	// Simple tokenization regex - splits on whitespace (fallback)
	tokenRegex := regexp.MustCompile(`\S+`)

	// Initialize tiktoken tokenizer for accurate token counting
	// Using cl100k_base which is used by GPT-4 and GPT-3.5-turbo
	enc, err := tokenizer.Get(tokenizer.Cl100kBase)
	if err != nil {
		// If tokenizer fails to initialize, we'll fall back to regex estimation
		enc = nil
	}

	return &TextChunker{
		MaxTokens:  maxTokens,
		Overlap:    overlap,
		TokenRegex: tokenRegex,
		Method:     ChunkingMethodAuto, // Default to auto method selection
		Tokenizer:  enc,
		Embedder:   nil, // Embedder will be set when semantic chunking is needed
	}
}

// NewTextChunkerWithEmbedder creates a TextChunker with an embedder for semantic chunking
func NewTextChunkerWithEmbedder(maxTokens, overlap int, embedder Embedder) *TextChunker {
	chunker := NewTextChunker(maxTokens, overlap)
	chunker.Embedder = embedder
	return chunker
}

// NewTextChunkerWithMethod creates a TextChunker with specific chunking method
func NewTextChunkerWithMethod(maxTokens, overlap int, method string) *TextChunker {
	chunker := NewTextChunker(maxTokens, overlap)
	chunker.Method = method
	return chunker
}

// EstimateTokenCount provides accurate token estimation using tiktoken
func (tc *TextChunker) EstimateTokenCount(text string) int {
	if text == "" {
		return 0
	}

	// Use tiktoken tokenizer for accurate token counting if available
	if tc.Tokenizer != nil {
		tokens, _, err := tc.Tokenizer.Encode(text)
		if err == nil {
			return len(tokens)
		}
		// If encoding fails, fall back to regex estimation
	}

	// Fallback to regex-based estimation if tiktoken is not available
	text = strings.TrimSpace(text)
	words := tc.TokenRegex.FindAllString(text, -1)
	tokenCount := len(words)

	// Add tokens for punctuation and special characters
	punctuationCount := strings.Count(text, ".") + strings.Count(text, ",") +
		strings.Count(text, "!") + strings.Count(text, "?") + strings.Count(text, ";") +
		strings.Count(text, ":") + strings.Count(text, "'") + strings.Count(text, "\"") +
		strings.Count(text, "(") + strings.Count(text, ")") + strings.Count(text, "[") +
		strings.Count(text, "]") + strings.Count(text, "{") + strings.Count(text, "}")

	// Add tokens for newlines (each newline is roughly 1 token)
	newlineCount := strings.Count(text, "\n")

	// Apply multiplier for longer words (they often get split into multiple tokens)
	longWordMultiplier := 0
	for _, word := range words {
		if len(word) > 6 { // Words longer than 6 chars often become multiple tokens
			longWordMultiplier += (len(word) - 6) / 4 // Rough estimation
		}
	}

	// More accurate estimation: 1.3 tokens per word on average for natural text
	estimatedTokens := int(float64(tokenCount)*1.3) + punctuationCount + newlineCount + longWordMultiplier

	// Apply minimum of original count to avoid under-estimation
	if estimatedTokens < tokenCount {
		estimatedTokens = tokenCount
	}

	return estimatedTokens
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

	// Choose chunking method based on configuration
	switch tc.Method {
	case ChunkingMethodSimple:
		return tc.sentenceBoundaryChunk(text, contentType)
	case ChunkingMethodRecursive:
		return tc.adaptiveChunk(text, contentType)
	case ChunkingMethodSemantic:
		return tc.semanticChunk(text, contentType)
	case ChunkingMethodAuto:
		// Auto-select based on content and size
		return tc.autoChunk(text, contentType, tokenCount)
	default:
		// Default to recursive for backward compatibility
		return tc.adaptiveChunk(text, contentType)
	}
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

	return ChunkTypeText
}

// adaptiveChunk applies content-aware chunking strategies with optimal sizing
func (tc *TextChunker) adaptiveChunk(text, contentType string) []Chunk {
	// Use recursive chunking algorithm
	return tc.recursiveChunk(text, contentType, 0)
}

// recursiveChunk implements hierarchical text splitting with semantic boundaries
func (tc *TextChunker) recursiveChunk(text, contentType string, depth int) []Chunk {
	// Define hierarchical separators based on content type
	separators := tc.getSeparators(contentType)

	// Prevent infinite recursion
	maxDepth := 5
	if depth >= maxDepth {
		return tc.fallbackChunk(text, contentType)
	}

	// If text is already within max tokens, return as single chunk
	tokenCount := tc.EstimateTokenCount(text)

	if tokenCount <= tc.MaxTokens {
		return []Chunk{
			{
				Text:       strings.TrimSpace(text),
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				TokenCount: tokenCount,
				ChunkType:  contentType,
			},
		}
	}

	// Try splitting with current separator
	if depth < len(separators) {
		separator := separators[depth]
		parts := tc.splitBySeparator(text, separator)

		// If we got meaningful splits, process them recursively
		if len(parts) > 1 {
			var allChunks []Chunk

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				partTokens := tc.EstimateTokenCount(part)

				// If part is still too large, recurse with next separator
				if partTokens > tc.MaxTokens {
					subChunks := tc.recursiveChunk(part, contentType, depth+1)
					allChunks = append(allChunks, subChunks...)
				} else {
					// Part fits within target size
					chunk := Chunk{
						Text:       part,
						Index:      len(allChunks),
						StartPos:   0,
						EndPos:     len(part),
						TokenCount: partTokens,
						ChunkType:  contentType,
					}
					allChunks = append(allChunks, chunk)
				}
			}

			// Apply overlap and reindex
			return tc.applyOverlapAndReindex(allChunks, contentType)
		}
	}

	// If we reach here, try next separator level
	return tc.recursiveChunk(text, contentType, depth+1)
}

// getSeparators returns hierarchical separators based on content type
func (tc *TextChunker) getSeparators(contentType string) []string {
	switch contentType {
	case ContentTypeCode:
		return []string{
			"\n\n\n", // Multiple blank lines (major sections)
			"\n\n",   // Double newlines (functions/classes)
			"\n",     // Single newlines (statements)
			". ",     // Sentences
			" ",      // Words
		}
	case ContentTypeStructured:
		return []string{
			"\n\n\n", // Major sections
			"\n\n",   // Paragraphs
			"\n# ",   // Headers
			"\n- ",   // List items
			". ",     // Sentences
			" ",      // Words
		}
	case ContentTypeProse:
		return []string{
			"\n\n\n", // Chapter/section breaks
			"\n\n",   // Paragraph breaks
			". ",     // Sentence boundaries
			"! ",     // Exclamations
			"? ",     // Questions
			" ",      // Words
		}
	default:
		return []string{
			"\n\n", // Paragraphs
			". ",   // Sentences
			"! ",   // Exclamations
			"? ",   // Questions
			" ",    // Words
		}
	}
}

// splitBySeparator splits text by a specific separator while preserving structure
func (tc *TextChunker) splitBySeparator(text, separator string) []string {
	if separator == " " {
		// Word-level splitting
		return strings.Fields(text)
	}

	parts := strings.Split(text, separator)
	var cleanParts []string

	for i, part := range parts {
		if i == 0 {
			// First part - keep as is
			cleanParts = append(cleanParts, strings.TrimSpace(part))
		} else if strings.TrimSpace(part) != "" {
			// Subsequent parts - add separator back for context
			cleanParts = append(cleanParts, separator+strings.TrimSpace(part))
		}
	}

	// Filter out empty parts
	var result []string
	for _, part := range cleanParts {
		if strings.TrimSpace(part) != "" {
			result = append(result, part)
		}
	}

	return result
}

// applyOverlapAndReindex applies smart overlap and reindexes chunks
func (tc *TextChunker) applyOverlapAndReindex(chunks []Chunk, contentType string) []Chunk {
	if len(chunks) <= 1 {
		return chunks
	}

	overlapRatio := tc.getOverlapRatio(contentType)
	overlapTokens := int(float64(tc.Overlap) * overlapRatio)

	var result []Chunk

	for i, chunk := range chunks {
		if i == 0 {
			// First chunk - no overlap needed
			chunk.Index = 0
			result = append(result, chunk)
			continue
		}

		// Add overlap from previous chunk
		if overlapTokens > 0 && i > 0 {
			prevChunk := chunks[i-1]
			overlapText := tc.getOverlapFromChunk(prevChunk, overlapTokens, contentType)

			if overlapText != "" {
				separator := tc.getSeparator(contentType)
				newText := overlapText + separator + chunk.Text
				newTokenCount := tc.EstimateTokenCount(newText)

				// Only add overlap if it doesn't exceed max tokens
				if newTokenCount <= tc.MaxTokens {
					chunk.Text = newText
					chunk.TokenCount = newTokenCount
				}
				// If adding overlap would exceed max tokens, keep the chunk as-is
			}
		}

		chunk.Index = i
		// Don't overwrite position information - the recursive chunker should not be used
		// for storing chunks with original text positions. Remove these problematic lines:
		// chunk.StartPos = 0
		// chunk.EndPos = len(chunk.Text)
		result = append(result, chunk)
	}

	return result
}

// getOverlapFromChunk extracts overlap text from a chunk
func (tc *TextChunker) getOverlapFromChunk(chunk Chunk, overlapTokens int, _ string) string {
	if overlapTokens <= 0 {
		return ""
	}

	words := strings.Fields(chunk.Text)
	if len(words) <= overlapTokens {
		return chunk.Text
	}

	// Take the last N words for overlap
	overlapWords := words[len(words)-overlapTokens:]
	return strings.Join(overlapWords, " ")
}

// fallbackChunk handles cases where recursive splitting fails
func (tc *TextChunker) fallbackChunk(text, contentType string) []Chunk {
	// Use word-level splitting as last resort
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []Chunk

	for i := 0; i < len(words); i += tc.MaxTokens {
		end := i + tc.MaxTokens
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		chunkText := strings.Join(chunkWords, " ")

		chunk := Chunk{
			Text:       chunkText,
			Index:      len(chunks),
			StartPos:   0,
			EndPos:     len(chunkText),
			TokenCount: len(chunkWords),
			ChunkType:  contentType,
		}

		chunks = append(chunks, chunk)

		if end >= len(words) {
			break
		}
	}

	return chunks
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
	// Enhanced sentence boundary detection with better abbreviation handling

	// Common abbreviations that shouldn't end sentences
	abbreviations := []string{
		"Mr.", "Mrs.", "Ms.", "Dr.", "Prof.", "Sr.", "Jr.", "Inc.", "Corp.", "Ltd.", "Co.",
		"vs.", "etc.", "i.e.", "e.g.", "cf.", "Fig.", "fig.", "Table", "table", "Vol.", "vol.",
		"No.", "no.", "p.", "pp.", "ch.", "Ch.", "sec.", "Sec.", "govt.", "Govt.",
	}

	// Create abbreviation regex pattern
	abbrevPattern := strings.Join(abbreviations, "|")
	abbrevRegex := regexp.MustCompile(`(?i)\b(?:` + regexp.QuoteMeta(abbrevPattern) + `)\s`)

	// Replace abbreviations temporarily to avoid false sentence breaks
	tempText := text
	abbrevReplacements := make(map[string]string)
	matches := abbrevRegex.FindAllString(tempText, -1)
	for i, match := range matches {
		placeholder := fmt.Sprintf("__ABBREV_%d__", i)
		abbrevReplacements[placeholder] = match
		tempText = strings.Replace(tempText, match, placeholder, 1)
	}

	// Enhanced sentence patterns - more sophisticated boundary detection
	sentenceRegex := regexp.MustCompile(`[.!?]+(?:\s+|$)`)

	// Find all sentence boundaries in the temp text
	boundaries := sentenceRegex.FindAllStringIndex(tempText, -1)
	if len(boundaries) == 0 {
		// No sentence boundaries found, restore abbreviations and return as single sentence
		for placeholder, original := range abbrevReplacements {
			tempText = strings.Replace(tempText, placeholder, original, -1)
		}
		cleaned := strings.TrimSpace(tempText)
		if cleaned != "" {
			return []string{cleaned}
		}
		return []string{}
	}

	var sentences []string
	lastEnd := 0

	for _, boundary := range boundaries {
		// Extract sentence including the punctuation
		start := lastEnd
		end := boundary[1] // Include punctuation

		sentence := strings.TrimSpace(tempText[start:end])
		if sentence != "" {
			// Restore abbreviations in this sentence
			for placeholder, original := range abbrevReplacements {
				sentence = strings.Replace(sentence, placeholder, original, -1)
			}
			sentences = append(sentences, sentence)
		}
		lastEnd = boundary[1]
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

// GetContentText returns the non-overlapping content of the chunk
func (c *Chunk) GetContentText() string {
	if c.ContentStart < 0 || c.ContentEnd > len(c.Text) || c.ContentStart >= c.ContentEnd {
		// If content boundaries are invalid, return the full text
		return c.Text
	}
	return c.Text[c.ContentStart:c.ContentEnd]
}

// HasOverlap returns whether this chunk has overlapping content
func (c *Chunk) HasOverlap() bool {
	return c.ContentStart > 0 || c.ContentEnd < len(c.Text)
}

// sentenceBoundaryChunk creates chunks that respect sentence boundaries and calculate proper indices
func (tc *TextChunker) sentenceBoundaryChunk(text, contentType string) []Chunk {
	if text == "" {
		return nil
	}

	sentences := tc.splitIntoSentences(text)
	if len(sentences) <= 1 {
		// Single sentence or no sentences, return as one chunk
		tokenCount := tc.EstimateTokenCount(text)
		return []Chunk{
			{
				Text:         text,
				Index:        0,
				StartPos:     0,
				EndPos:       len(text),
				ContentStart: 0,
				ContentEnd:   len(text),
				TokenCount:   tokenCount,
				ChunkType:    contentType,
			},
		}
	}

	var chunks []Chunk
	var currentSentences []string
	var currentTokens int
	chunkIndex := 0

	// Build sentence position map
	sentencePositions := tc.buildSentencePositions(text, sentences)

	// Reserve space for overlap
	effectiveMaxTokens := tc.MaxTokens - tc.Overlap

	for i, sentence := range sentences {
		sentenceTokens := tc.EstimateTokenCount(sentence)

		// Check if adding this sentence would exceed our limit
		if len(currentSentences) > 0 && currentTokens+sentenceTokens > effectiveMaxTokens {
			// Finalize current chunk
			chunk := tc.buildChunkWithOverlap(text, currentSentences, sentencePositions, chunkIndex, contentType)
			chunks = append(chunks, chunk)
			chunkIndex++

			// Start new chunk with overlap from previous chunk
			overlapSentences := tc.getOverlapSentences(sentences, i, tc.Overlap)
			currentSentences = overlapSentences
			currentTokens = tc.EstimateTokenCount(strings.Join(overlapSentences, " "))
		}

		// Add current sentence
		currentSentences = append(currentSentences, sentence)
		currentTokens += sentenceTokens
	}

	// Add final chunk if any sentences remain
	if len(currentSentences) > 0 {
		chunk := tc.buildChunkWithOverlap(text, currentSentences, sentencePositions, chunkIndex, contentType)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// buildSentencePositions creates a map of sentence positions in the original text
func (tc *TextChunker) buildSentencePositions(text string, sentences []string) map[string][]int {
	positions := make(map[string][]int)
	searchOffset := 0

	for _, sentence := range sentences {
		// Find this sentence in the text starting from searchOffset
		index := strings.Index(text[searchOffset:], sentence)
		if index != -1 {
			actualIndex := searchOffset + index
			positions[sentence] = []int{actualIndex, actualIndex + len(sentence)}
			searchOffset = actualIndex + len(sentence)
		}
	}

	return positions
}

// buildChunkWithOverlap creates a chunk with proper overlap calculation
func (tc *TextChunker) buildChunkWithOverlap(originalText string, sentences []string, positions map[string][]int, chunkIndex int, contentType string) Chunk {
	if len(sentences) == 0 {
		return Chunk{}
	}

	// Find the start and end positions in the original text
	firstSentence := sentences[0]
	lastSentence := sentences[len(sentences)-1]

	var startPos, endPos int
	if pos, exists := positions[firstSentence]; exists {
		startPos = pos[0]
	}
	if pos, exists := positions[lastSentence]; exists {
		endPos = pos[1]
	}

	chunkText := originalText[startPos:endPos]

	// Calculate content boundaries (non-overlapping part)
	contentStart := 0
	contentEnd := len(chunkText)

	if chunkIndex > 0 {
		// For chunks after the first, overlap is at the beginning
		overlapTokens := tc.Overlap
		overlapChars := tc.estimateCharactersFromTokens(overlapTokens)
		if overlapChars < len(chunkText) {
			contentStart = overlapChars
		}
	}

	return Chunk{
		Text:         chunkText,
		Index:        chunkIndex,
		StartPos:     startPos,
		EndPos:       endPos,
		ContentStart: contentStart,
		ContentEnd:   contentEnd,
		TokenCount:   tc.EstimateTokenCount(chunkText),
		ChunkType:    contentType,
	}
}

// getOverlapSentences gets sentences for overlap from previous chunk
func (tc *TextChunker) getOverlapSentences(sentences []string, currentIndex, overlapTokens int) []string {
	if currentIndex == 0 || overlapTokens <= 0 {
		return []string{}
	}

	var overlapSentences []string
	var tokens int

	// Work backwards from current index to get overlap sentences
	for i := currentIndex - 1; i >= 0; i-- {
		sentenceTokens := tc.EstimateTokenCount(sentences[i])
		if tokens + sentenceTokens > overlapTokens {
			break
		}
		overlapSentences = append([]string{sentences[i]}, overlapSentences...)
		tokens += sentenceTokens
	}

	return overlapSentences
}

// estimateCharactersFromTokens estimates character count from token count
func (tc *TextChunker) estimateCharactersFromTokens(tokens int) int {
	// Rough estimate: 1 token ≈ 4 characters on average
	return tokens * 4
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

	// Create timestamp suffix for uniqueness (YYMMDD-HHMM format with milliseconds)
	timestamp := now.Format("060102-1504")
	milliseconds := now.Nanosecond() / 1000000 % 1000

	return fmt.Sprintf("%s-%s-%s%03d", adjective, noun, timestamp, milliseconds)
}

// simpleChunk implements straightforward token-based chunking with proper overlap
func (tc *TextChunker) simpleChunk(text, contentType string) []Chunk {
	if text == "" {
		return nil
	}

	// Split text into words to allow for precise token tracking
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentTokens int
	chunkIndex := 0

	// Reserve space for overlap
	effectiveMaxTokens := tc.MaxTokens - tc.Overlap

	for i, word := range words {
		// Estimate tokens for current word (including space)
		wordWithSpace := word
		if currentChunk.Len() > 0 {
			wordWithSpace = " " + word
		}
		wordTokens := tc.EstimateTokenCount(wordWithSpace)

		// Check if adding this word would exceed our limit
		if currentTokens > 0 && currentTokens+wordTokens > effectiveMaxTokens {
			// Finalize current chunk
			chunkText := strings.TrimSpace(currentChunk.String())
			if chunkText != "" {
				chunk := Chunk{
					Text:       chunkText,
					Index:      chunkIndex,
					StartPos:   0,
					EndPos:     len(chunkText),
					TokenCount: tc.EstimateTokenCount(chunkText), // Accurate count for final chunk
					ChunkType:  contentType,
				}
				chunks = append(chunks, chunk)
				chunkIndex++
			}

			// Start new chunk with overlap from previous chunk
			currentChunk.Reset()
			currentTokens = 0

			// Add overlap words from the end of the previous chunk
			if tc.Overlap > 0 && len(chunks) > 0 {
				overlapWords := tc.getOverlapWords(words, i, tc.Overlap)
				if len(overlapWords) > 0 {
					currentChunk.WriteString(strings.Join(overlapWords, " "))
					currentTokens = tc.EstimateTokenCount(currentChunk.String())
				}
			}
		}

		// Add current word to chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(word)
		currentTokens = tc.EstimateTokenCount(currentChunk.String())
	}

	// Add final chunk if there's content
	if currentChunk.Len() > 0 {
		chunkText := strings.TrimSpace(currentChunk.String())
		if chunkText != "" {
			chunk := Chunk{
				Text:       chunkText,
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     len(chunkText),
				TokenCount: tc.EstimateTokenCount(chunkText),
				ChunkType:  contentType,
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// getOverlapWords gets the last N tokens worth of words for overlap
func (tc *TextChunker) getOverlapWords(words []string, currentIndex, overlapTokens int) []string {
	if overlapTokens <= 0 || currentIndex <= 0 {
		return nil
	}

	var overlapWords []string
	tokensCollected := 0

	// Go backwards from current position to collect overlap words
	for i := currentIndex - 1; i >= 0 && tokensCollected < overlapTokens; i-- {
		word := words[i]
		wordTokens := tc.EstimateTokenCount(word + " ") // Include space

		if tokensCollected+wordTokens <= overlapTokens {
			overlapWords = append([]string{word}, overlapWords...) // Prepend to maintain order
			tokensCollected += wordTokens
		} else {
			break
		}
	}

	return overlapWords
}

// autoChunk automatically selects the best chunking method based on content analysis
func (tc *TextChunker) autoChunk(text, contentType string, tokenCount int) []Chunk {
	// Decision factors for choosing chunking method:
	// 1. Content type and structure
	// 2. Document size
	// 3. Presence of natural boundaries
	// 4. Availability of embedder for semantic chunking

	// For small documents (< 2x max tokens), use simple chunking
	if tokenCount < tc.MaxTokens*2 {
		return tc.simpleChunk(text, contentType)
	}

	// For semantic chunking if embedder is available
	if tc.Embedder != nil && contentType == ContentTypeProse && tokenCount > tc.MaxTokens*3 {
		return tc.semanticChunk(text, contentType)
	}

	// For all other cases, use sentence boundary chunking to preserve original text positions
	// This ensures that chunk.StartPos and chunk.EndPos work correctly with originalText[start:end]
	return tc.sentenceBoundaryChunk(text, contentType)
}

// semanticChunk implements embedding-based semantic similarity chunking
func (tc *TextChunker) semanticChunk(text, contentType string) []Chunk {
	if text == "" {
		return nil
	}

	// If embedder is not available, fall back to recursive chunking
	if tc.Embedder == nil {
		return tc.adaptiveChunk(text, contentType)
	}

	// Split text into sentences for semantic analysis
	sentences := tc.splitIntoSentences(text)
	if len(sentences) <= 1 {
		// Single sentence, return as one chunk
		tokenCount := tc.EstimateTokenCount(text)
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

	// Generate embeddings for each sentence
	ctx := context.Background()
	embeddings, err := tc.generateSentenceEmbeddings(ctx, sentences)
	if err != nil {
		// If embedding fails, fall back to recursive chunking
		return tc.adaptiveChunk(text, contentType)
	}

	// Calculate semantic boundaries using cosine similarity
	boundaries := tc.findSemanticBoundaries(embeddings, sentences)

	// Group sentences into chunks based on semantic boundaries
	chunks := tc.groupSentencesSemanically(sentences, boundaries, text, contentType)

	// Ensure chunks respect token limits and add overlaps
	return tc.optimizeSemanticChunks(chunks, contentType)
}

// generateSentenceEmbeddings creates embeddings for each sentence
func (tc *TextChunker) generateSentenceEmbeddings(ctx context.Context, sentences []string) ([][]float32, error) {
	embeddings := make([][]float32, len(sentences))

	for i, sentence := range sentences {
		if strings.TrimSpace(sentence) == "" {
			continue
		}

		embedding, err := tc.Embedder.Embed(ctx, sentence)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for sentence %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

// findSemanticBoundaries identifies chunk boundaries using cosine similarity
func (tc *TextChunker) findSemanticBoundaries(embeddings [][]float32, sentences []string) []int {
	if len(embeddings) <= 1 {
		return []int{0, len(sentences)}
	}

	// Calculate cosine distances between consecutive sentences
	distances := make([]float64, len(embeddings)-1)
	for i := 0; i < len(embeddings)-1; i++ {
		distances[i] = tc.cosineSimilarity(embeddings[i], embeddings[i+1])
	}

	// Find breakpoints using percentile threshold (95th percentile)
	sortedDistances := make([]float64, len(distances))
	copy(sortedDistances, distances)
	sort.Float64s(sortedDistances)

	// Use 95th percentile as threshold for semantic boundaries
	percentileIndex := int(float64(len(sortedDistances)) * 0.05) // 5th percentile (low similarity = boundary)
	if percentileIndex >= len(sortedDistances) {
		percentileIndex = len(sortedDistances) - 1
	}
	threshold := sortedDistances[percentileIndex]

	// Identify boundaries where similarity is below threshold
	boundaries := []int{0} // Always start with first sentence
	for i, distance := range distances {
		if distance <= threshold {
			boundaries = append(boundaries, i+1)
		}
	}
	boundaries = append(boundaries, len(sentences)) // Always end with last sentence

	// Remove duplicate boundaries and ensure proper ordering
	uniqueBoundaries := tc.removeDuplicateBoundaries(boundaries)

	return uniqueBoundaries
}

// cosineSimilarity calculates cosine similarity between two vectors
func (tc *TextChunker) cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// removeDuplicateBoundaries removes duplicate indices and ensures proper ordering
func (tc *TextChunker) removeDuplicateBoundaries(boundaries []int) []int {
	if len(boundaries) <= 1 {
		return boundaries
	}

	seen := make(map[int]bool)
	var unique []int

	for _, boundary := range boundaries {
		if !seen[boundary] {
			seen[boundary] = true
			unique = append(unique, boundary)
		}
	}

	sort.Ints(unique)
	return unique
}

// groupSentencesSemanically creates chunks from sentences based on boundaries
func (tc *TextChunker) groupSentencesSemanically(sentences []string, boundaries []int, originalText, contentType string) []Chunk {
	var chunks []Chunk

	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]

		if start >= end || start >= len(sentences) {
			continue
		}

		// Ensure we don't go beyond the sentences array
		if end > len(sentences) {
			end = len(sentences)
		}

		// Join sentences for this chunk
		chunkSentences := sentences[start:end]
		chunkText := strings.Join(chunkSentences, " ")
		chunkText = strings.TrimSpace(chunkText)

		if chunkText == "" {
			continue
		}

		// Calculate token count and positions
		tokenCount := tc.EstimateTokenCount(chunkText)
		startPos := tc.findStartPosition(originalText, chunkSentences[0])
		endPos := startPos + len(chunkText)

		chunks = append(chunks, Chunk{
			Text:       chunkText,
			Index:      len(chunks),
			StartPos:   startPos,
			EndPos:     endPos,
			TokenCount: tokenCount,
			ChunkType:  contentType,
		})
	}

	return chunks
}

// optimizeSemanticChunks ensures chunks respect token limits and adds appropriate overlaps
func (tc *TextChunker) optimizeSemanticChunks(chunks []Chunk, contentType string) []Chunk {
	if len(chunks) == 0 {
		return chunks
	}

	var optimized []Chunk

	for i, chunk := range chunks {
		// If chunk exceeds token limit, split it further
		if chunk.TokenCount > tc.MaxTokens {
			// Split oversized chunk using simple token-based splitting
			subChunks := tc.splitOversizedSemanticChunk(chunk)
			optimized = append(optimized, subChunks...)
		} else {
			// Add overlap with previous chunk if not the first chunk
			if i > 0 && tc.Overlap > 0 {
				chunk = tc.addSemanticOverlap(chunk, optimized[len(optimized)-1])
			}
			optimized = append(optimized, chunk)
		}
	}

	// Re-index chunks
	for i := range optimized {
		optimized[i].Index = i
	}

	return optimized
}

// splitOversizedSemanticChunk splits a chunk that exceeds token limits
func (tc *TextChunker) splitOversizedSemanticChunk(chunk Chunk) []Chunk {
	// Use simple word-based splitting for oversized semantic chunks
	words := strings.Fields(chunk.Text)
	if len(words) <= tc.MaxTokens {
		return []Chunk{chunk}
	}

	var subChunks []Chunk
	effectiveMaxTokens := tc.MaxTokens - tc.Overlap
	if effectiveMaxTokens <= 0 {
		effectiveMaxTokens = tc.MaxTokens
	}

	for i := 0; i < len(words); i += effectiveMaxTokens {
		end := i + tc.MaxTokens
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		chunkText := strings.Join(chunkWords, " ")
		tokenCount := tc.EstimateTokenCount(chunkText)

		subChunks = append(subChunks, Chunk{
			Text:       chunkText,
			Index:      len(subChunks),
			StartPos:   chunk.StartPos,
			EndPos:     chunk.StartPos + len(chunkText),
			TokenCount: tokenCount,
			ChunkType:  chunk.ChunkType,
		})
	}

	return subChunks
}

// addSemanticOverlap adds contextual overlap between semantic chunks
func (tc *TextChunker) addSemanticOverlap(currentChunk, previousChunk Chunk) Chunk {
	if tc.Overlap <= 0 {
		return currentChunk
	}

	// Take last few words from previous chunk as overlap
	prevWords := strings.Fields(previousChunk.Text)
	overlapWords := tc.Overlap / 4 // Estimate 4 tokens per word for overlap
	if overlapWords > len(prevWords) {
		overlapWords = len(prevWords)
	}
	if overlapWords <= 0 {
		return currentChunk
	}

	overlap := strings.Join(prevWords[len(prevWords)-overlapWords:], " ")
	newText := overlap + " " + currentChunk.Text
	newTokenCount := tc.EstimateTokenCount(newText)

	return Chunk{
		Text:       newText,
		Index:      currentChunk.Index,
		StartPos:   currentChunk.StartPos,
		EndPos:     currentChunk.EndPos,
		TokenCount: newTokenCount,
		ChunkType:  currentChunk.ChunkType,
	}
}
