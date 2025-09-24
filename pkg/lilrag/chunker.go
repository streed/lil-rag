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
)

// Content type constants
const (
	ContentTypeCode       = "code"
	ContentTypeProse      = "prose"
	ContentTypeStructured = "structured"

	// Chunking strategy constants
	ChunkingStrategySemantic = "semantic"
	ThresholdTypeGradient    = "gradient"
)

// Chunk type constants
const (
	ChunkTypeText        = "text"
	ChunkTypeHTMLSection = "html_section"
)

type TextChunker struct {
	MaxChars   int
	Overlap    int
	TokenRegex *regexp.Regexp
}

type Chunk struct {
	Text       string
	Index      int
	StartPos   int
	EndPos     int
	CharCount  int
	TokenCount int   // Kept for backward compatibility
	PageNumber *int   // Optional page number for PDF chunks
	ChunkType  string // Type of chunk: "text", "pdf_page"
}

func NewTextChunker(maxChars, overlap int) *TextChunker {
	// Simple tokenization regex - splits on whitespace
	tokenRegex := regexp.MustCompile(`\S+`)

	return &TextChunker{
		MaxChars:   maxChars,
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

	// For very small texts (under 400 characters), return as single chunk unless MaxChars is very small
	if len(text) <= 400 && tc.MaxChars >= 400 {
		return []Chunk{
			{
				Text:       text,
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				CharCount:  len(text),
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
	if len(semanticChunks) == 1 && semanticChunks[0].TokenCount <= tc.MaxChars {
		return semanticChunks
	}

	// For larger documents or multiple semantic chunks, apply full processing
	return semanticChunks
}

// ChunkTextWithStrategy applies the specified chunking strategy
func (tc *TextChunker) ChunkTextWithStrategy(text, strategy string) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	contentType := tc.detectContentType(text)

	switch strategy {
	case "simple":
		return tc.fallbackChunk(text, contentType)
	case ChunkingStrategySemantic:
		// Semantic chunking focuses on content-aware boundaries
		return tc.adaptiveChunk(text, contentType)
	case "recursive":
		// Default recursive chunking (same as current ChunkText behavior)
		return tc.ChunkText(text)
	default:
		// Default recursive chunking (same as current ChunkText behavior)
		return tc.ChunkText(text)
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

// recursiveChunk implements LangChain-style hierarchical text splitting with split-and-merge approach
func (tc *TextChunker) recursiveChunk(text, contentType string, depth int) []Chunk {
	// Define hierarchical separators based on content type
	separators := tc.getSeparators(contentType)

	// Prevent infinite recursion
	maxDepth := len(separators)
	if depth >= maxDepth {
		return tc.fallbackChunk(text, contentType)
	}

	// If text is already within max characters, return as single chunk
	charCount := len(text)
	if charCount <= tc.MaxChars {
		return []Chunk{
			{
				Text:       strings.TrimSpace(text),
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				CharCount:  charCount,
				TokenCount: tc.EstimateTokenCount(text),
				ChunkType:  contentType,
			},
		}
	}

	// Try current separator
	separator := separators[depth]
	splits := tc.splitBySeparator(text, separator)

	// If no meaningful split occurred, try next separator
	if len(splits) <= 1 {
		return tc.recursiveChunk(text, contentType, depth+1)
	}

	// LangChain approach: try to combine splits to approach target size
	chunks := tc.combineSplitsIntoOptimalChunks(splits, separator, contentType, depth)

	// If we still have chunks that are too large, recurse on those
	var finalChunks []Chunk
	for _, chunk := range chunks {
		if chunk.CharCount > tc.MaxChars {
			// This chunk is still too large, recurse with next separator
			subChunks := tc.recursiveChunk(chunk.Text, contentType, depth+1)
			finalChunks = append(finalChunks, subChunks...)
		} else {
			finalChunks = append(finalChunks, chunk)
		}
	}

	return tc.reindexChunks(finalChunks, contentType)
}

// combineSplitsIntoOptimalChunks implements LangChain's core split-and-merge logic
func (tc *TextChunker) combineSplitsIntoOptimalChunks(splits []string, separator, contentType string, _ int) []Chunk {
	if len(splits) == 0 {
		return nil
	}

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentTokens int

	for _, split := range splits {
		split = strings.TrimSpace(split)
		if split == "" {
			continue
		}

		splitTokens := tc.EstimateTokenCount(split)

		// If this split alone exceeds max tokens, it will need further recursion
		if splitTokens > tc.MaxChars {
			// First, finalize current chunk if it has content
			if currentChunk.Len() > 0 {
				chunks = append(chunks, tc.createChunk(currentChunk.String(), currentTokens, contentType, len(chunks)))
				currentChunk.Reset()
				currentTokens = 0
			}

			// Add the oversized split as its own chunk (will be recursed later)
			chunks = append(chunks, tc.createChunk(split, splitTokens, contentType, len(chunks)))
			continue
		}

		// Calculate tokens if we add this split to current chunk
		var potentialText string
		var potentialTokens int

		if currentChunk.Len() == 0 {
			// First split in chunk
			potentialText = split
			potentialTokens = splitTokens
		} else {
			// Add separator back when combining splits
			if separator != "" && separator != " " {
				potentialText = currentChunk.String() + separator + split
			} else {
				potentialText = currentChunk.String() + " " + split
			}
			potentialTokens = tc.EstimateTokenCount(potentialText)
		}

		// Check if adding this split would exceed max tokens
		if potentialTokens > tc.MaxChars {
			// Finalize current chunk and start new one with overlap
			if currentChunk.Len() > 0 {
				chunks = append(chunks, tc.createChunk(currentChunk.String(), currentTokens, contentType, len(chunks)))

				// Start new chunk with overlap from previous chunk
				overlapText := tc.getOptimalOverlap(currentChunk.String(), contentType)
				if overlapText != "" && separator != "" && separator != " " {
					currentChunk.Reset()
					currentChunk.WriteString(overlapText + separator + split)
					currentTokens = tc.EstimateTokenCount(currentChunk.String())
				} else {
					currentChunk.Reset()
					if overlapText != "" {
						currentChunk.WriteString(overlapText + " " + split)
						currentTokens = tc.EstimateTokenCount(currentChunk.String())
					} else {
						currentChunk.WriteString(split)
						currentTokens = splitTokens
					}
				}
			} else {
				// Start new chunk with current split
				currentChunk.Reset()
				currentChunk.WriteString(split)
				currentTokens = splitTokens
			}
		} else {
			// Add split to current chunk
			if currentChunk.Len() == 0 {
				currentChunk.WriteString(split)
			} else {
				// Add separator back when combining
				if separator != "" && separator != " " {
					currentChunk.WriteString(separator)
				} else {
					currentChunk.WriteString(" ")
				}
				currentChunk.WriteString(split)
			}
			currentTokens = tc.EstimateTokenCount(currentChunk.String())
		}
	}

	// Add final chunk if it has content
	if currentChunk.Len() > 0 {
		chunks = append(chunks, tc.createChunk(currentChunk.String(), len(currentChunk.String()), contentType, len(chunks)))
	}

	return chunks
}

// createChunk helper function to create a chunk with proper structure
func (tc *TextChunker) createChunk(text string, _ int, contentType string, index int) Chunk {
	text = strings.TrimSpace(text)
	return Chunk{
		Text:       text,
		Index:      index,
		StartPos:   0,
		EndPos:     len(text),
		CharCount:  len(text), // Use actual length after trimming
		TokenCount: tc.EstimateTokenCount(text),
		ChunkType:  contentType,
	}
}

// reindexChunks reindexes chunks and applies proper indices
func (tc *TextChunker) reindexChunks(chunks []Chunk, contentType string) []Chunk {
	for i := range chunks {
		chunks[i].Index = i
		chunks[i].ChunkType = contentType
	}
	return chunks
}

// getOptimalOverlap extracts optimal overlap text from the end of a chunk
func (tc *TextChunker) getOptimalOverlap(chunkText, _ string) string {
	if tc.Overlap <= 0 {
		return ""
	}

	// For character-based overlap, take the last N characters
	if len(chunkText) <= tc.Overlap {
		return chunkText
	}

	// Try to break at word boundaries when possible
	overlapText := chunkText[len(chunkText)-tc.Overlap:]

	// Find the first word boundary to avoid splitting words
	if idx := strings.Index(overlapText, " "); idx != -1 {
		overlapText = overlapText[idx+1:]
	}

	return overlapText
}

// getSeparators returns hierarchical separators based on content type
func (tc *TextChunker) getSeparators(contentType string) []string {
	// Get base separators for content type
	baseSeparators := tc.getBaseSeparators(contentType)

	// Add language-specific separators if applicable
	langSeparators := tc.getLanguageSeparators()

	// Merge language separators into base separators (before word/character level)
	if len(langSeparators) > 0 {
		// Insert language separators before the last two elements (words and character fallback)
		if len(baseSeparators) >= 2 {
			merged := make([]string, 0, len(baseSeparators)+len(langSeparators))
			merged = append(merged, baseSeparators[:len(baseSeparators)-2]...)
			merged = append(merged, langSeparators...)
			merged = append(merged, baseSeparators[len(baseSeparators)-2:]...)
			return merged
		}
	}

	return baseSeparators
}

// getBaseSeparators returns base hierarchical separators based on content type
func (tc *TextChunker) getBaseSeparators(contentType string) []string {
	switch contentType {
	case ContentTypeCode:
		return []string{
			"\n\n\n", // Multiple blank lines (major sections)
			"\n\n",   // Double newlines (functions/classes)
			"\n",     // Single newlines (statements)
			"; ",     // Semicolons (statement separators)
			", ",     // Commas (parameter separators)
			" ",      // Words
			"",       // Character-level fallback
		}
	case ContentTypeStructured:
		return []string{
			"\n\n\n", // Major sections
			"\n\n",   // Paragraphs
			"\n# ",   // Headers
			"\n- ",   // List items
			"\n",     // Line breaks
			". ",     // Sentences
			"; ",     // Semicolons
			", ",     // Commas
			" ",      // Words
			"",       // Character-level fallback
		}
	case ContentTypeProse:
		return []string{
			"\n\n\n", // Chapter/section breaks
			"\n\n",   // Paragraph breaks
			"\n",     // Line breaks
			". ",     // Sentence boundaries
			"! ",     // Exclamations
			"? ",     // Questions
			"; ",     // Semicolons (clause boundaries)
			", ",     // Commas (phrase boundaries)
			" ",      // Words
			"",       // Character-level fallback
		}
	default:
		// LangChain-inspired default hierarchy with enhancements
		return []string{
			"\n\n", // Paragraphs (major semantic boundaries)
			"\n",   // Line breaks (minor semantic boundaries)
			". ",   // Sentences (strong semantic boundaries)
			"! ",   // Exclamations
			"? ",   // Questions
			"; ",   // Semicolons (clause boundaries)
			", ",   // Commas (phrase boundaries)
			" ",    // Words
			"",     // Character-level fallback (LangChain approach)
		}
	}
}

// getLanguageSeparators returns language-specific separators for international text
func (tc *TextChunker) getLanguageSeparators() []string {
	// LangChain-inspired international separators
	return []string{
		"。",     // Japanese/Chinese period
		"．",     // Fullwidth period
		"！",     // Fullwidth exclamation
		"？",     // Fullwidth question mark
		"；",     // Fullwidth semicolon
		"，",     // Fullwidth comma
		"、",     // Ideographic comma
		"\u200b", // Zero-width space (used in some languages)
		"·",      // Middle dot (used in Catalan, etc.)
		"¿",      // Inverted question mark (Spanish)
		"¡",      // Inverted exclamation mark (Spanish)
	}
}

// splitBySeparator splits text by a specific separator while preserving structure
func (tc *TextChunker) splitBySeparator(text, separator string) []string {
	if separator == " " {
		// Word-level splitting
		return strings.Fields(text)
	}

	if separator == "" {
		// Character-level splitting as final fallback (LangChain approach)
		return tc.splitIntoCharacterChunks(text)
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

// splitIntoCharacterChunks splits text into character-level chunks as final fallback
func (tc *TextChunker) splitIntoCharacterChunks(text string) []string {
	if text == "" {
		return nil
	}

	// Use the configured character limit directly
	maxChars := tc.MaxChars
	if maxChars < 1 {
		maxChars = 100 // Minimum chunk size
	}

	var chunks []string
	for i := 0; i < len(text); i += maxChars {
		end := i + maxChars
		if end > len(text) {
			end = len(text)
		}
		chunk := text[i:end]
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
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
				if newTokenCount <= tc.MaxChars {
					chunk.Text = newText
					chunk.TokenCount = newTokenCount
				}
				// If adding overlap would exceed max tokens, keep the chunk as-is
			}
		}

		chunk.Index = i
		chunk.StartPos = 0
		chunk.EndPos = len(chunk.Text)
		result = append(result, chunk)
	}

	return result
}

// getOverlapFromChunk extracts overlap text from a chunk with semantic awareness
func (tc *TextChunker) getOverlapFromChunk(chunk Chunk, overlapChars int, _ string) string {
	if overlapChars <= 0 {
		return ""
	}

	// Try sentence-aware overlap first for better semantic preservation
	sentences := tc.extractSentences(chunk.Text)
	if len(sentences) > 1 {
		return tc.getSemanticOverlap(sentences, overlapChars)
	}

	// For single sentence or when sentence parsing fails, be more careful
	// Prefer to keep the entire chunk if it's not much longer than overlap limit
	if len(chunk.Text) <= overlapChars*2 {
		return chunk.Text
	}

	// Try to find a sentence boundary within the overlap range
	text := chunk.Text
	if len(text) > overlapChars {
		// Look for sentence boundaries near the target overlap position
		searchStart := len(text) - overlapChars*2 // Look further back
		if searchStart < 0 {
			searchStart = 0
		}
		searchArea := text[searchStart:]

		// Look for sentence endings (., !, ?) followed by space or end of string
		sentenceEndPattern := []string{". ", "! ", "? ", ".\n", "!\n", "?\n"}
		lastSentenceEnd := -1

		for _, pattern := range sentenceEndPattern {
			if idx := strings.LastIndex(searchArea, pattern); idx != -1 && idx > lastSentenceEnd {
				lastSentenceEnd = idx
			}
		}

		if lastSentenceEnd != -1 {
			// Found a sentence boundary - use everything after it
			actualStart := searchStart + lastSentenceEnd + 2 // +2 to skip the ". " or similar
			if actualStart < len(text) {
				return strings.TrimSpace(text[actualStart:])
			}
		}
	}

	// Final fallback: use word boundaries
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	// Calculate how many words fit in the overlap
	wordsToTake := overlapChars / 5 // Rough estimate: average 5 chars per word
	if wordsToTake > len(words) {
		wordsToTake = len(words)
	}
	if wordsToTake == 0 {
		wordsToTake = 1 // At least one word
	}

	overlapWords := words[len(words)-wordsToTake:]
	result := strings.Join(overlapWords, " ")

	// If the word-based overlap is still too long, truncate at word boundary
	if len(result) > overlapChars && len(overlapWords) > 1 {
		for len(overlapWords) > 1 && len(strings.Join(overlapWords, " ")) > overlapChars {
			overlapWords = overlapWords[1:] // Remove first word
		}
		result = strings.Join(overlapWords, " ")
	}

	return result
}

// getSemanticOverlap extracts overlap using complete sentences when possible
func (tc *TextChunker) getSemanticOverlap(sentences []string, overlapChars int) string {
	if len(sentences) == 0 {
		return ""
	}

	// Start from the last sentence and work backwards
	var overlapSentences []string
	totalChars := 0

	for i := len(sentences) - 1; i >= 0; i-- {
		sentence := strings.TrimSpace(sentences[i])
		if sentence == "" {
			continue
		}
		sentenceChars := len(sentence)

		// If adding this sentence would exceed the overlap limit
		if totalChars+sentenceChars > overlapChars {
			// If we haven't added any sentences yet, we need to decide what to do
			if len(overlapSentences) == 0 {
				// If this single sentence is longer than our overlap limit,
				// prefer to skip overlap rather than break sentence boundaries
				// This preserves semantic meaning at the cost of some overlap
				if sentenceChars > overlapChars {
					return "" // No overlap rather than broken sentences
				}
				// If the sentence fits, include it
				return sentence
			}
			// We already have some sentences, so stop here to preserve boundaries
			break
		}

		// Add this sentence to the overlap (prepend to maintain order)
		overlapSentences = append([]string{sentence}, overlapSentences...)
		totalChars += sentenceChars
		if len(overlapSentences) > 1 {
			totalChars++ // Add space separator between sentences
		}
	}

	return strings.Join(overlapSentences, " ")
}

// fallbackChunk handles cases where recursive splitting fails
func (tc *TextChunker) fallbackChunk(text, contentType string) []Chunk {
	// Try sentence-aware chunking first
	sentences := tc.extractSentences(text)
	if len(sentences) > 1 {
		return tc.chunkBySentences(sentences, contentType)
	}

	// If no sentence boundaries found, use word-level splitting as last resort
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []Chunk
	var currentChunk strings.Builder
	var chunkIndex int

	for _, word := range words {
		// Check if adding this word would exceed character limit
		potentialText := currentChunk.String()
		if potentialText != "" {
			potentialText += " " + word
		} else {
			potentialText = word
		}

		if len(potentialText) > tc.MaxChars && currentChunk.Len() > 0 {
			// Create chunk with current content
			chunkText := currentChunk.String()
			chunk := Chunk{
				Text:       chunkText,
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     len(chunkText),
				CharCount:  len(chunkText),
				TokenCount: tc.EstimateTokenCount(chunkText),
				ChunkType:  contentType,
			}
			chunks = append(chunks, chunk)
			chunkIndex++

			// Start new chunk with current word
			currentChunk.Reset()
			currentChunk.WriteString(word)
		} else {
			// Add word to current chunk
			if currentChunk.Len() > 0 {
				currentChunk.WriteString(" ")
			}
			currentChunk.WriteString(word)
		}
	}

	// Add final chunk if there's content
	if currentChunk.Len() > 0 {
		chunkText := currentChunk.String()
		chunk := Chunk{
			Text:       chunkText,
			Index:      chunkIndex,
			StartPos:   0,
			EndPos:     len(chunkText),
			CharCount:  len(chunkText),
			TokenCount: tc.EstimateTokenCount(chunkText),
			ChunkType:  contentType,
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// extractSentences splits text into sentences using improved patterns
func (tc *TextChunker) extractSentences(text string) []string {
	// Handle different content types appropriately
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Enhanced sentence patterns that handle common abbreviations
	sentenceRegex := regexp.MustCompile(`[.!?]+(?:\s+|$)`)

	// Find all sentence boundaries
	matches := sentenceRegex.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return []string{text}
	}

	var sentences []string
	start := 0

	for _, match := range matches {
		// Include the punctuation in the sentence
		end := match[1]
		sentence := strings.TrimSpace(text[start:end])

		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		start = end
	}

	// Add any remaining text after the last sentence boundary
	if start < len(text) {
		remaining := strings.TrimSpace(text[start:])
		if remaining != "" {
			sentences = append(sentences, remaining)
		}
	}

	return sentences
}

// chunkBySentences groups sentences into chunks while respecting token limits
func (tc *TextChunker) chunkBySentences(sentences []string, contentType string) []Chunk {
	if len(sentences) == 0 {
		return nil
	}

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentChars int
	chunkIndex := 0

	for _, sentence := range sentences {
		sentenceChars := len(sentence)

		// If this single sentence exceeds max characters, we need to split it
		if sentenceChars > tc.MaxChars {
			// First, finish current chunk if it has content
			if currentChunk.Len() > 0 {
				chunkText := strings.TrimSpace(currentChunk.String())
				chunks = append(chunks, Chunk{
					Text:       chunkText,
					Index:      chunkIndex,
					StartPos:   0,
					EndPos:     len(chunkText),
					CharCount:  len(chunkText),
					TokenCount: tc.EstimateTokenCount(chunkText),
					ChunkType:  contentType,
				})
				chunkIndex++
				currentChunk.Reset()
				currentChars = 0
			}

			// Split the long sentence by words but preserve as much as possible
			longSentenceChunks := tc.splitLongSentenceByWords(sentence, contentType, chunkIndex)
			for _, chunk := range longSentenceChunks {
				chunk.Index = chunkIndex
				chunks = append(chunks, chunk)
				chunkIndex++
			}
			continue
		}

		// Check if adding this sentence would exceed the character limit
		if currentChars+sentenceChars > tc.MaxChars && currentChunk.Len() > 0 {
			// Create a chunk with current content
			chunkText := strings.TrimSpace(currentChunk.String())
			chunks = append(chunks, Chunk{
				Text:       chunkText,
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     len(chunkText),
				CharCount:  len(chunkText),
				TokenCount: tc.EstimateTokenCount(chunkText),
				ChunkType:  contentType,
			})
			chunkIndex++

			// Reset for next chunk
			currentChunk.Reset()
			currentChars = 0
		}

		// Add sentence to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
			currentChars++ // Add 1 for the space
		}
		currentChunk.WriteString(sentence)
		currentChars += sentenceChars
	}

	// Add final chunk if there's content
	if currentChunk.Len() > 0 {
		chunkText := strings.TrimSpace(currentChunk.String())
		chunks = append(chunks, Chunk{
			Text:       chunkText,
			Index:      chunkIndex,
			StartPos:   0,
			EndPos:     len(chunkText),
			CharCount:  len(chunkText),
			TokenCount: tc.EstimateTokenCount(chunkText),
			ChunkType:  contentType,
		})
	}

	return chunks
}

// splitLongSentenceByWords splits a single long sentence while preserving readability
func (tc *TextChunker) splitLongSentenceByWords(sentence, contentType string, startIndex int) []Chunk {
	words := strings.Fields(sentence)
	if len(words) <= tc.MaxChars {
		return []Chunk{{
			Text:       sentence,
			Index:      startIndex,
			StartPos:   0,
			EndPos:     len(sentence),
			CharCount:  len(sentence),
			TokenCount: len(words),
			ChunkType:  contentType,
		}}
	}

	var chunks []Chunk
	chunkIndex := 0

	for i := 0; i < len(words); i += tc.MaxChars {
		end := i + tc.MaxChars
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		chunkText := strings.Join(chunkWords, " ")

		// Add continuation indicators for readability
		if i > 0 {
			chunkText = "..." + chunkText
		}
		if end < len(words) {
			chunkText += "..."
		}

		chunks = append(chunks, Chunk{
			Text:       chunkText,
			Index:      startIndex + chunkIndex,
			StartPos:   0,
			EndPos:     len(chunkText),
			CharCount:  len(chunkText),
			TokenCount: len(chunkWords),
			ChunkType:  contentType,
		})
		chunkIndex++
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

	// For semantic chunking, preserve punctuation to maintain sentence boundaries
	// This is critical for overlap logic to work correctly
	var cleanParagraphs []string
	for _, para := range tempParagraphs {
		// Keep all punctuation intact for semantic chunking
		cleanParagraphs = append(cleanParagraphs, para)
	}

	return cleanParagraphs
}

// splitBySentences uses improved sentence detection patterns while preserving paragraph structure
func (tc *TextChunker) splitBySentences(text string) []string {
	// First, preserve paragraph structure by marking paragraph boundaries
	paragraphs := strings.Split(text, "\n\n")
	var sentences []string

	for i, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		// Add paragraph marker except for the first paragraph
		if i > 0 && len(sentences) > 0 {
			// Mark the start of a new paragraph by prefixing with special marker
			paragraph = "¶PARAGRAPH_BREAK¶" + paragraph
		}

		// Enhanced sentence patterns including abbreviations awareness
		sentenceRegex := regexp.MustCompile(`[.!?](?:\s+|$)`)

		// Find all sentence boundaries in this paragraph
		matches := sentenceRegex.FindAllStringIndex(paragraph, -1)
		if len(matches) == 0 {
			// No sentence boundaries found - treat whole paragraph as one sentence
			if paragraph != "" {
				sentences = append(sentences, paragraph)
			}
			continue
		}

		lastEnd := 0
		for j, match := range matches {
			// Extract sentence including the punctuation for the last sentence
			start := lastEnd
			var end int
			if j == len(matches)-1 {
				// Last sentence in paragraph - include the punctuation
				end = match[1]
			} else {
				// Middle sentences - exclude the punctuation
				end = match[0]
			}

			sentence := strings.TrimSpace(paragraph[start:end])
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			lastEnd = match[1]
		}
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
	// First try sentence-aware splitting
	sentences := tc.extractSentences(chunk.Text)
	if len(sentences) > 1 {
		sentenceChunks := tc.chunkBySentences(sentences, chunk.ChunkType)

		// Update chunk metadata to preserve original chunk info
		for i := range sentenceChunks {
			sentenceChunks[i].Index = 0 // Will be re-indexed by caller
			sentenceChunks[i].StartPos = chunk.StartPos
			sentenceChunks[i].EndPos = chunk.EndPos
		}
		return sentenceChunks
	}

	// Fallback to word-level splitting for content without sentence boundaries
	words := strings.Fields(chunk.Text)
	if len(words) <= tc.MaxChars {
		return []Chunk{chunk}
	}

	var chunks []Chunk

	for i := 0; i < len(words); i += tc.MaxChars - tc.Overlap {
		end := i + tc.MaxChars
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[i:end]
		chunkText := strings.Join(chunkWords, " ")

		// Add continuation indicators to show this is a split sentence/chunk
		if i > 0 {
			chunkText = "..." + chunkText
		}
		if end < len(words) {
			chunkText += "..."
		}

		chunks = append(chunks, Chunk{
			Text:       chunkText,
			Index:      0,                  // Will be re-indexed by the caller
			StartPos:   chunk.StartPos + i, // Approximate start position
			EndPos:     chunk.StartPos + i + len(chunkText),
			CharCount:  len(chunkText),
			TokenCount: len(chunkWords),
			ChunkType:  chunk.ChunkType,
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
	return tc.EstimateTokenCount(text) > tc.MaxChars
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

	// Create timestamp suffix for uniqueness (YYMMDD-HHMM format with milliseconds)
	timestamp := now.Format("060102-1504")
	milliseconds := now.Nanosecond() / 1000000 % 1000

	return fmt.Sprintf("%s-%s-%s%03d", adjective, noun, timestamp, milliseconds)
}

// ChunkTextWithSemanticEmbedding performs true semantic chunking using embeddings
func (tc *TextChunker) ChunkTextWithSemanticEmbedding(text, strategy string, embedder Embedder) []Chunk {
	return tc.ChunkTextWithSemanticEmbeddingAndThreshold(text, strategy, embedder, "percentile", 0.95)
}

// ChunkTextWithSemanticEmbeddingAndThreshold performs semantic chunking with configurable threshold
func (tc *TextChunker) ChunkTextWithSemanticEmbeddingAndThreshold(text, strategy string, embedder Embedder, thresholdType string, threshold float32) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Fallback to recursive chunking if embedder is not available
	if embedder == nil || strategy != ChunkingStrategySemantic {
		return tc.ChunkTextWithStrategy(text, strategy)
	}

	// Split text into sentences for semantic analysis
	sentences := tc.splitIntoSentences(text)
	if len(sentences) <= 1 {
		// Single sentence or no sentences - return as single chunk
		tokenCount := tc.EstimateTokenCount(text)
		return []Chunk{
			{
				Text:       text,
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				TokenCount: tokenCount,
				ChunkType:  tc.detectContentType(text),
			},
		}
	}

	// Apply LangChain-style sentence grouping for better semantic analysis
	sentenceGroups := tc.groupSentencesForSemanticAnalysis(sentences)

	// Generate embeddings for each sentence group
	ctx := context.Background()
	embeddings := make([][]float32, len(sentenceGroups))

	for i, group := range sentenceGroups {
		// Combine sentences in group for embedding
		groupText := strings.Join(group, " ")
		embedding, err := embedder.Embed(ctx, groupText)
		if err != nil {
			// Fallback to recursive chunking on embedding error
			return tc.ChunkTextWithStrategy(text, "recursive")
		}
		embeddings[i] = embedding
	}

	// Calculate similarities between consecutive sentences
	similarities := make([]float32, len(embeddings)-1)
	for i := 0; i < len(embeddings)-1; i++ {
		similarities[i] = cosineSimilarity(embeddings[i], embeddings[i+1])
	}

	// Find semantic breakpoints based on similarity threshold
	breakpoints := findSemanticBreakpoints(similarities, thresholdType, threshold)

	// Create chunks based on breakpoints and sentence groups
	chunks := tc.createSemanticChunksFromGroups(sentences, sentenceGroups, breakpoints)

	// Apply overlap and ensure token limits
	return tc.finalizeSemanticChunks(chunks)
}

// groupSentencesForSemanticAnalysis groups sentences using LangChain-style approach
func (tc *TextChunker) groupSentencesForSemanticAnalysis(sentences []string) [][]string {
	if len(sentences) == 0 {
		return nil
	}

	// For small numbers of sentences, use individual sentences for better granularity
	if len(sentences) <= 5 {
		var groups [][]string
		for _, sentence := range sentences {
			groups = append(groups, []string{sentence})
		}
		return groups
	}

	var groups [][]string
	groupSize := 3 // LangChain uses groups of 3 sentences for larger texts

	// Create overlapping groups for better semantic continuity
	for i := 0; i < len(sentences); i += groupSize {
		end := i + groupSize
		if end > len(sentences) {
			end = len(sentences)
		}

		group := sentences[i:end]

		// Skip very small trailing groups, merge them with previous group
		if len(group) == 1 && len(groups) > 0 {
			groups[len(groups)-1] = append(groups[len(groups)-1], group...)
		} else {
			groups = append(groups, group)
		}
	}

	return groups
}

// createSemanticChunksFromGroups creates chunks from sentence groups and breakpoints
func (tc *TextChunker) createSemanticChunksFromGroups(sentences []string, sentenceGroups [][]string, breakpoints []int) []Chunk {
	if len(sentenceGroups) == 0 {
		return nil
	}

	// If no breakpoints, return single chunk
	if len(breakpoints) == 0 {
		text := tc.joinSentencesPreservingPunctuation(sentences)
		chunk := Chunk{
			Text:       text,
			Index:      0,
			StartPos:   0,
			EndPos:     len(text),
			TokenCount: tc.EstimateTokenCount(text),
			ChunkType:  tc.detectContentType(text),
		}
		return []Chunk{chunk}
	}

	var chunks []Chunk
	currentStart := 0

	// Create chunks based on breakpoints
	for _, breakpoint := range breakpoints {
		if breakpoint > currentStart && breakpoint <= len(sentenceGroups) {
			// Collect sentences from currentStart to breakpoint
			var chunkSentences []string
			for gIdx := currentStart; gIdx < breakpoint; gIdx++ {
				chunkSentences = append(chunkSentences, sentenceGroups[gIdx]...)
			}

			// Create chunk from collected sentences
			if len(chunkSentences) > 0 {
				chunkText := tc.joinSentencesPreservingPunctuation(chunkSentences)
				if strings.TrimSpace(chunkText) != "" {
					chunk := Chunk{
						Text:       chunkText,
						Index:      len(chunks),
						StartPos:   0,
						EndPos:     len(chunkText),
						TokenCount: tc.EstimateTokenCount(chunkText),
						ChunkType:  tc.detectContentType(chunkText),
					}
					chunks = append(chunks, chunk)
				}
			}

			currentStart = breakpoint
		}
	}

	// Add remaining sentences as final chunk
	if currentStart < len(sentenceGroups) {
		var chunkSentences []string
		for gIdx := currentStart; gIdx < len(sentenceGroups); gIdx++ {
			chunkSentences = append(chunkSentences, sentenceGroups[gIdx]...)
		}

		if len(chunkSentences) > 0 {
			chunkText := tc.joinSentencesPreservingPunctuation(chunkSentences)
			if strings.TrimSpace(chunkText) != "" {
				chunk := Chunk{
					Text:       chunkText,
					Index:      len(chunks),
					StartPos:   0,
					EndPos:     len(chunkText),
					TokenCount: tc.EstimateTokenCount(chunkText),
					ChunkType:  tc.detectContentType(chunkText),
				}
				chunks = append(chunks, chunk)
			}
		}
	}

	// If no chunks were created, create a single chunk
	if len(chunks) == 0 {
		text := tc.joinSentencesPreservingPunctuation(sentences)
		chunk := Chunk{
			Text:       text,
			Index:      0,
			StartPos:   0,
			EndPos:     len(text),
			TokenCount: tc.EstimateTokenCount(text),
			ChunkType:  tc.detectContentType(text),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// joinSentencesPreservingPunctuation joins sentences while maintaining proper punctuation
func (tc *TextChunker) joinSentencesPreservingPunctuation(sentences []string) string {
	if len(sentences) == 0 {
		return ""
	}

	var result strings.Builder
	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		if i > 0 {
			// Add space between sentences
			result.WriteString(" ")
		}

		result.WriteString(sentence)
	}

	return result.String()
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		return 0.0
	}

	var dotProduct, norm1, norm2 float32
	for i := 0; i < len(vec1); i++ {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (float32(math.Sqrt(float64(norm1))) * float32(math.Sqrt(float64(norm2))))
}

// findSemanticBreakpoints identifies where to split chunks based on similarity threshold
func findSemanticBreakpoints(similarities []float32, thresholdType string, threshold float32) []int {
	if len(similarities) == 0 {
		return nil
	}

	var breakpoints []int

	switch thresholdType {
	case "percentile":
		breakpoints = findPercentileBreakpoints(similarities, threshold)
	case "standard_deviation":
		breakpoints = findStandardDeviationBreakpoints(similarities, threshold)
	case "interquartile":
		breakpoints = findInterquartileBreakpoints(similarities, threshold)
	case ThresholdTypeGradient:
		breakpoints = findGradientBreakpoints(similarities, threshold)
	case "fixed":
		breakpoints = findFixedThresholdBreakpoints(similarities, threshold)
	default:
		breakpoints = findPercentileBreakpoints(similarities, 0.95) // Default to 95th percentile
	}

	return breakpoints
}

// findPercentileBreakpoints finds breakpoints based on percentile threshold
func findPercentileBreakpoints(similarities []float32, percentile float32) []int {
	if len(similarities) == 0 {
		return nil
	}

	// Sort similarities to find percentile threshold
	sorted := make([]float32, len(similarities))
	copy(sorted, similarities)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// For small sets, use a more aggressive approach
	// The idea is to find significant drops in similarity
	if len(similarities) <= 3 {
		// For small similarity arrays, use an absolute threshold approach
		var breakpoints []int

		// For semantic chunking, similarities below 0.6 indicate topic changes
		// This is more aggressive than percentile methods for clear topic boundaries
		threshold := float32(0.6)

		for i, sim := range similarities {
			if sim < threshold {
				breakpoints = append(breakpoints, i+1)
			}
		}

		// If we found breakpoints using the absolute threshold, return them
		if len(breakpoints) > 0 {
			return breakpoints
		}

		// Fallback: if the range is large, split at the median
		if len(sorted) >= 2 && (sorted[len(sorted)-1]-sorted[0]) > 0.2 {
			median := sorted[len(sorted)/2]
			for i, sim := range similarities {
				if sim < median {
					breakpoints = append(breakpoints, i+1)
				}
			}
			return breakpoints
		}
	}

	// Calculate percentile threshold
	index := int((1.0 - percentile) * float32(len(sorted)))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}
	percentileThreshold := sorted[index]

	// Find breakpoints where similarity is below threshold
	var breakpoints []int
	for i, sim := range similarities {
		if sim < percentileThreshold {
			breakpoints = append(breakpoints, i+1) // Break after this sentence
		}
	}

	return breakpoints
}

// findStandardDeviationBreakpoints finds breakpoints based on standard deviation
func findStandardDeviationBreakpoints(similarities []float32, numStdDev float32) []int {
	if len(similarities) == 0 {
		return nil
	}

	// For small arrays, use gap detection like percentile method
	if len(similarities) <= 3 {
		return findPercentileBreakpoints(similarities, 0.8)
	}

	// Calculate mean and standard deviation
	var sum float32
	for _, sim := range similarities {
		sum += sim
	}
	mean := sum / float32(len(similarities))

	var variance float32
	for _, sim := range similarities {
		diff := sim - mean
		variance += diff * diff
	}
	stdDev := float32(math.Sqrt(float64(variance / float32(len(similarities)))))

	// Find breakpoints where similarity is below mean - numStdDev * stdDev
	threshold := mean - numStdDev*stdDev
	var breakpoints []int
	for i, sim := range similarities {
		if sim < threshold {
			breakpoints = append(breakpoints, i+1)
		}
	}

	return breakpoints
}

// findInterquartileBreakpoints finds breakpoints using interquartile range
func findInterquartileBreakpoints(similarities []float32, scalingFactor float32) []int {
	if len(similarities) == 0 {
		return nil
	}

	// For small arrays, use gap detection like percentile method
	if len(similarities) <= 3 {
		return findPercentileBreakpoints(similarities, 0.8)
	}

	// Sort similarities to find quartiles
	sorted := make([]float32, len(similarities))
	copy(sorted, similarities)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Calculate Q1 and Q3
	q1Index := len(sorted) / 4
	q3Index := 3 * len(sorted) / 4
	if q1Index >= len(sorted) {
		q1Index = len(sorted) - 1
	}
	if q3Index >= len(sorted) {
		q3Index = len(sorted) - 1
	}

	q1 := sorted[q1Index]
	q3 := sorted[q3Index]
	iqr := q3 - q1

	// Find breakpoints using IQR threshold
	threshold := q1 - scalingFactor*iqr
	var breakpoints []int
	for i, sim := range similarities {
		if sim < threshold {
			breakpoints = append(breakpoints, i+1)
		}
	}

	return breakpoints
}

// findGradientBreakpoints finds breakpoints using advanced gradient-based anomaly detection
func findGradientBreakpoints(similarities []float32, sensitivity float32) []int {
	if len(similarities) == 0 {
		return nil
	}

	// For small arrays, use the same logic as percentile method
	if len(similarities) <= 3 {
		return findPercentileBreakpoints(similarities, 0.95)
	}

	// Calculate first-order gradients (differences between consecutive similarities)
	gradients := make([]float32, len(similarities)-1)
	for i := 0; i < len(similarities)-1; i++ {
		gradients[i] = similarities[i+1] - similarities[i]
	}

	// Calculate second-order gradients (gradient of gradients) for anomaly detection
	secondOrderGradients := make([]float32, len(gradients)-1)
	for i := 0; i < len(gradients)-1; i++ {
		secondOrderGradients[i] = gradients[i+1] - gradients[i]
	}

	// Apply anomaly detection using statistical methods
	breakpoints := detectGradientAnomalies(gradients, secondOrderGradients, sensitivity)

	// Apply domain-specific optimization for highly correlated content
	if isHighlyCorrelatedContent(similarities) {
		breakpoints = optimizeForDomainSpecificContent(breakpoints, similarities, sensitivity)
	}

	return breakpoints
}

// detectGradientAnomalies uses statistical anomaly detection on gradients
func detectGradientAnomalies(gradients, secondOrderGradients []float32, sensitivity float32) []int {
	var breakpoints []int

	// Calculate statistics for first-order gradients
	mean, stdDev := calculateGradientStats(gradients)

	// Use both first and second-order gradients for anomaly detection
	for i, grad := range gradients {
		// Check for steep negative drops (semantic boundaries)
		if grad < mean-stdDev*2.0 && grad < -sensitivity {
			breakpoints = append(breakpoints, i+1)
		}

		// For second-order gradients, look for sudden changes in gradient direction
		if i < len(secondOrderGradients) {
			secondGrad := secondOrderGradients[i]
			// Large positive second-order gradient indicates sharp turn in similarity curve
			if secondGrad > stdDev*1.5 && grad < -sensitivity*0.5 {
				breakpoints = append(breakpoints, i+1)
			}
		}
	}

	// Remove duplicate breakpoints and ensure they're sorted
	return deduplicateBreakpoints(breakpoints)
}

// calculateGradientStats computes mean and standard deviation of gradients
func calculateGradientStats(gradients []float32) (float32, float32) {
	if len(gradients) == 0 {
		return 0, 0
	}

	// Calculate mean
	var sum float32
	for _, grad := range gradients {
		sum += grad
	}
	mean := sum / float32(len(gradients))

	// Calculate standard deviation
	var variance float32
	for _, grad := range gradients {
		diff := grad - mean
		variance += diff * diff
	}
	variance /= float32(len(gradients))

	// Return mean and standard deviation
	return mean, float32(math.Sqrt(float64(variance)))
}

// isHighlyCorrelatedContent detects if content is domain-specific (highly correlated)
func isHighlyCorrelatedContent(similarities []float32) bool {
	if len(similarities) < 3 {
		return false
	}

	// Calculate variance of similarities
	var sum, sumSquares float32
	for _, sim := range similarities {
		sum += sim
		sumSquares += sim * sim
	}

	n := float32(len(similarities))
	mean := sum / n
	variance := (sumSquares / n) - (mean * mean)

	// Low variance indicates highly correlated content (domain-specific)
	// High mean similarity also indicates domain-specific content
	return variance < 0.01 && mean > 0.7
}

// optimizeForDomainSpecificContent applies specialized processing for domain content
func optimizeForDomainSpecificContent(breakpoints []int, similarities []float32, sensitivity float32) []int {
	// For domain-specific content, make the distribution wider (LangChain approach)
	enhancedSensitivity := sensitivity * 0.7 // More sensitive to smaller changes

	// Re-analyze with enhanced sensitivity for subtle semantic boundaries
	var enhancedBreakpoints []int

	for i := 0; i < len(similarities)-1; i++ {
		sim := similarities[i]

		// Look for more subtle drops in highly correlated content
		if i > 0 && i < len(similarities)-1 {
			prevSim := similarities[i-1]
			nextSim := similarities[i+1]

			// Local minimum detection with enhanced sensitivity
			if sim < prevSim && sim < nextSim && sim < enhancedSensitivity {
				enhancedBreakpoints = append(enhancedBreakpoints, i+1)
			}
		}
	}

	// Combine original and enhanced breakpoints
	breakpoints = append(breakpoints, enhancedBreakpoints...)
	return deduplicateBreakpoints(breakpoints)
}

// deduplicateBreakpoints removes duplicates and sorts breakpoints
func deduplicateBreakpoints(breakpoints []int) []int {
	if len(breakpoints) == 0 {
		return breakpoints
	}

	// Use map to remove duplicates
	seen := make(map[int]bool)
	var unique []int

	for _, bp := range breakpoints {
		if !seen[bp] {
			seen[bp] = true
			unique = append(unique, bp)
		}
	}

	// Sort the breakpoints
	for i := 0; i < len(unique)-1; i++ {
		for j := i + 1; j < len(unique); j++ {
			if unique[i] > unique[j] {
				unique[i], unique[j] = unique[j], unique[i]
			}
		}
	}

	return unique
}

// findFixedThresholdBreakpoints finds breakpoints using a fixed similarity threshold
func findFixedThresholdBreakpoints(similarities []float32, threshold float32) []int {
	var breakpoints []int
	for i, sim := range similarities {
		if sim < threshold {
			breakpoints = append(breakpoints, i+1)
		}
	}
	return breakpoints
}

// createSemanticChunks creates chunks based on sentence breakpoints

// finalizeSemanticChunks applies token limits and overlap to semantic chunks
func (tc *TextChunker) finalizeSemanticChunks(chunks []Chunk) []Chunk {
	if len(chunks) == 0 {
		return chunks
	}

	var finalChunks []Chunk

	for _, chunk := range chunks {
		// If chunk exceeds token limit, split it further
		if chunk.TokenCount > tc.MaxChars {
			subChunks := tc.splitLongChunkByWords(chunk)
			finalChunks = append(finalChunks, subChunks...)
		} else {
			finalChunks = append(finalChunks, chunk)
		}
	}

	// Apply overlap and reindex
	return tc.applyOverlapAndReindex(finalChunks, finalChunks[0].ChunkType)
}
