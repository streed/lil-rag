package minirag

import (
	"fmt"
	"regexp"
	"strings"
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
	// Simple tokenization regex - splits on whitespace and punctuation
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
	if tokenCount <= tc.MaxTokens {
		return []Chunk{
			{
				Text:       text,
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				TokenCount: tokenCount,
			},
		}
	}

	// Split text into sentences for better chunk boundaries
	sentences := tc.splitIntoSentences(text)
	if len(sentences) == 0 {
		return nil
	}

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentTokenCount int
	chunkIndex := 0
	startPos := 0

	for i, sentence := range sentences {
		sentenceTokens := tc.EstimateTokenCount(sentence)

		// If adding this sentence would exceed max tokens, finalize current chunk
		if currentTokenCount > 0 && currentTokenCount+sentenceTokens > tc.MaxTokens {
			chunkText := strings.TrimSpace(currentChunk.String())
			if chunkText != "" {
				chunks = append(chunks, Chunk{
					Text:       chunkText,
					Index:      chunkIndex,
					StartPos:   startPos,
					EndPos:     startPos + len(chunkText),
					TokenCount: currentTokenCount,
				})
				chunkIndex++
			}

			// Start new chunk with overlap from previous chunk
			currentChunk.Reset()
			currentTokenCount = 0

			// Add overlap from previous sentences if available
			if tc.Overlap > 0 && len(chunks) > 0 {
				overlapText := tc.getOverlapText(sentences, i, tc.Overlap)
				if overlapText != "" {
					currentChunk.WriteString(overlapText)
					currentChunk.WriteString(" ")
					currentTokenCount = tc.EstimateTokenCount(overlapText)
				}
			}

			startPos = tc.findStartPosition(text, sentence)
		}

		// Add current sentence to chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)
		currentTokenCount += sentenceTokens
	}

	// Add final chunk if it has content
	if currentChunk.Len() > 0 {
		chunkText := strings.TrimSpace(currentChunk.String())
		if chunkText != "" {
			chunks = append(chunks, Chunk{
				Text:       chunkText,
				Index:      chunkIndex,
				StartPos:   startPos,
				EndPos:     startPos + len(chunkText),
				TokenCount: currentTokenCount,
			})
		}
	}

	// Handle case where we have very long sentences that exceed max tokens
	var finalChunks []Chunk
	for _, chunk := range chunks {
		if chunk.TokenCount <= tc.MaxTokens {
			finalChunks = append(finalChunks, chunk)
		} else {
			// Split very long chunks by words
			wordChunks := tc.splitLongChunkByWords(chunk)
			finalChunks = append(finalChunks, wordChunks...)
		}
	}

	// Re-index all chunks to ensure unique, sequential indices
	for i := range finalChunks {
		finalChunks[i].Index = i
	}

	return finalChunks
}

func (tc *TextChunker) splitIntoSentences(text string) []string {
	// Simple sentence splitting on periods, exclamation marks, and question marks
	// This is basic - for production you might want a more sophisticated sentence splitter
	sentenceRegex := regexp.MustCompile(`[.!?]+\s+`)

	sentences := sentenceRegex.Split(text, -1)

	// Clean up sentences and filter out empty ones
	var cleanSentences []string
	for _, sentence := range sentences {
		cleaned := strings.TrimSpace(sentence)
		if cleaned != "" {
			cleanSentences = append(cleanSentences, cleaned)
		}
	}

	// If no sentence boundaries found, split by paragraphs or large whitespace
	if len(cleanSentences) <= 1 && len(text) > tc.MaxTokens*4 {
		paragraphs := strings.Split(text, "\n\n")
		if len(paragraphs) > 1 {
			return paragraphs
		}

		// As last resort, split by double spaces or long whitespace
		parts := regexp.MustCompile(`\s{2,}`).Split(text, -1)
		if len(parts) > 1 {
			return parts
		}
	}

	return cleanSentences
}

func (tc *TextChunker) getOverlapText(sentences []string, currentIndex, overlapTokens int) string {
	if currentIndex == 0 || overlapTokens == 0 {
		return ""
	}

	var overlapBuilder strings.Builder
	tokenCount := 0

	// Go backwards from current index to build overlap
	for i := currentIndex - 1; i >= 0 && tokenCount < overlapTokens; i-- {
		sentenceTokens := tc.EstimateTokenCount(sentences[i])
		if tokenCount+sentenceTokens <= overlapTokens {
			if overlapBuilder.Len() > 0 {
				overlapBuilder.WriteString(" " + sentences[i])
			} else {
				overlapBuilder.WriteString(sentences[i])
			}
			tokenCount += sentenceTokens
		} else {
			break
		}
	}

	return overlapBuilder.String()
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
