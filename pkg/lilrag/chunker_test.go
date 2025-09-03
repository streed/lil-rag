package lilrag

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewTextChunker(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	if chunker.MaxTokens != 100 {
		t.Errorf("Expected MaxTokens to be 100, got %d", chunker.MaxTokens)
	}
	if chunker.Overlap != 20 {
		t.Errorf("Expected Overlap to be 20, got %d", chunker.Overlap)
	}
	if chunker.TokenRegex == nil {
		t.Error("Expected TokenRegex to be initialized")
	}
}

func TestTextChunker_EstimateTokenCount(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "single word",
			text:     "hello",
			expected: 1,
		},
		{
			name:     "multiple words",
			text:     "hello world test",
			expected: 3,
		},
		{
			name:     "with punctuation",
			text:     "Hello, world! How are you?",
			expected: 5,
		},
		{
			name:     "with extra spaces",
			text:     "  hello   world   ",
			expected: 2,
		},
		{
			name:     "with newlines",
			text:     "hello\nworld\ntest",
			expected: 3,
		},
		{
			name:     "complex text",
			text:     "This is a test. It has multiple sentences! And punctuation?",
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunker.EstimateTokenCount(tt.text)
			if result != tt.expected {
				t.Errorf("EstimateTokenCount(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTextChunker_IsLongText(t *testing.T) {
	chunker := NewTextChunker(5, 1)

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "empty text",
			text:     "",
			expected: false,
		},
		{
			name:     "short text",
			text:     "hello world",
			expected: false,
		},
		{
			name:     "exact max tokens",
			text:     "one two three four five",
			expected: false,
		},
		{
			name:     "over max tokens",
			text:     "one two three four five six",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunker.IsLongText(tt.text)
			if result != tt.expected {
				t.Errorf("IsLongText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTextChunker_ChunkText_SingleChunk(t *testing.T) {
	chunker := NewTextChunker(10, 2)

	text := "This is a short text that fits in one chunk"
	chunks := chunker.ChunkText(text)

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if chunk.Text != text {
		t.Errorf("Expected chunk text to be %q, got %q", text, chunk.Text)
	}
	if chunk.Index != 0 {
		t.Errorf("Expected chunk index to be 0, got %d", chunk.Index)
	}
	if chunk.StartPos != 0 {
		t.Errorf("Expected start position to be 0, got %d", chunk.StartPos)
	}
	if chunk.EndPos != len(text) {
		t.Errorf("Expected end position to be %d, got %d", len(text), chunk.EndPos)
	}
}

func TestTextChunker_ChunkText_MultipleChunks(t *testing.T) {
	chunker := NewTextChunker(5, 1) // Very small chunks for testing

	text := "This is the first sentence. This is the second sentence. This is the third sentence."
	chunks := chunker.ChunkText(text)

	if len(chunks) == 0 {
		t.Fatal("Expected multiple chunks, got 0")
	}

	// Verify chunks have sequential indices
	for i, chunk := range chunks {
		if chunk.Index != i {
			t.Errorf("Expected chunk %d to have index %d, got %d", i, i, chunk.Index)
		}
		if chunk.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
		if chunk.TokenCount == 0 {
			t.Errorf("Chunk %d has zero token count", i)
		}
	}
}

func TestTextChunker_ChunkText_WithOverlap(t *testing.T) {
	chunker := NewTextChunker(3, 1)

	text := "First sentence. Second sentence. Third sentence."
	chunks := chunker.ChunkText(text)

	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks with overlap, got %d", len(chunks))
	}

	// Should have some overlap between consecutive chunks
	// This is a basic check - the exact overlap depends on sentence structure
	for i := 1; i < len(chunks); i++ {
		if chunks[i].Text == "" {
			t.Errorf("Chunk %d should not be empty", i)
		}
	}
}

func TestTextChunker_ChunkText_EmptyText(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	tests := []string{"", "   ", "\n\n", "\t\t"}

	for _, text := range tests {
		chunks := chunker.ChunkText(text)
		if chunks != nil {
			t.Errorf("Expected nil chunks for empty text %q, got %v", text, chunks)
		}
	}
}

func TestTextChunker_splitIntoSentences(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "single sentence",
			text:     "This is one sentence",
			expected: []string{"This is one sentence"},
		},
		{
			name:     "multiple sentences",
			text:     "First sentence. Second sentence! Third sentence?",
			expected: []string{"First sentence", "Second sentence", "Third sentence?"},
		},
		{
			name:     "with extra spaces",
			text:     "First sentence.   Second sentence!   Third sentence?",
			expected: []string{"First sentence", "Second sentence", "Third sentence?"},
		},
		{
			name:     "no sentence boundaries",
			text:     "This is all one long sentence without proper punctuation",
			expected: []string{"This is all one long sentence without proper punctuation"},
		},
		{
			name:     "paragraph splits",
			text:     "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.",
			expected: []string{"First paragraph", "Second paragraph", "Third paragraph."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunker.splitIntoSentences(tt.text)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitIntoSentences(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTextChunker_getOverlapText(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	sentences := []string{"First sentence", "Second sentence", "Third sentence", "Fourth sentence"}

	tests := []struct {
		name          string
		currentIndex  int
		overlapTokens int
		expected      string
	}{
		{
			name:          "no overlap at start",
			currentIndex:  0,
			overlapTokens: 2,
			expected:      "",
		},
		{
			name:          "zero overlap tokens",
			currentIndex:  2,
			overlapTokens: 0,
			expected:      "",
		},
		{
			name:          "single sentence overlap",
			currentIndex:  2,
			overlapTokens: 2,
			expected:      "Second sentence",
		},
		{
			name:          "multiple sentence overlap",
			currentIndex:  3,
			overlapTokens: 4,
			expected:      "Second sentence Third sentence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunker.getOverlapText(sentences, tt.currentIndex, tt.overlapTokens)
			if result != tt.expected {
				t.Errorf("getOverlapText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTextChunker_findStartPosition(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	text := "This is a test sentence. This is another sentence."

	tests := []struct {
		name     string
		sentence string
		expected int
	}{
		{
			name:     "sentence at start",
			sentence: "This is a test sentence",
			expected: 0,
		},
		{
			name:     "sentence in middle",
			sentence: "This is another sentence",
			expected: 25,
		},
		{
			name:     "sentence not found",
			sentence: "Not in text",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunker.findStartPosition(text, tt.sentence)
			if result != tt.expected {
				t.Errorf("findStartPosition(%q, %q) = %d, want %d", text, tt.sentence, result, tt.expected)
			}
		})
	}
}

func TestTextChunker_splitLongChunkByWords(t *testing.T) {
	chunker := NewTextChunker(5, 1)

	chunk := Chunk{
		Text:       "This is a very long chunk that needs to be split into smaller pieces",
		Index:      0,
		StartPos:   0,
		EndPos:     100,
		TokenCount: 14,
	}

	chunks := chunker.splitLongChunkByWords(chunk)

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify each chunk respects max tokens
	for i, c := range chunks {
		if c.TokenCount > chunker.MaxTokens {
			t.Errorf("Chunk %d has %d tokens, exceeds max %d", i, c.TokenCount, chunker.MaxTokens)
		}
		if c.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
	}

	// Verify the full text is preserved (approximately)
	var allText strings.Builder
	for i, c := range chunks {
		if i > 0 {
			allText.WriteString(" ")
		}
		allText.WriteString(c.Text)
	}

	// The reconstructed text should contain most of the original words
	originalWords := strings.Fields(chunk.Text)
	reconstructedWords := strings.Fields(allText.String())

	if len(reconstructedWords) < len(originalWords)-2 { // Allow some variation due to overlap
		t.Errorf("Lost too many words during splitting: original %d, reconstructed %d",
			len(originalWords), len(reconstructedWords))
	}
}

func TestTextChunker_splitLongChunkByWords_ShortChunk(t *testing.T) {
	chunker := NewTextChunker(10, 2)

	chunk := Chunk{
		Text:       "Short chunk",
		Index:      0,
		StartPos:   0,
		EndPos:     11,
		TokenCount: 2,
	}

	chunks := chunker.splitLongChunkByWords(chunk)

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for short text, got %d", len(chunks))
	}

	if chunks[0].Text != chunk.Text {
		t.Errorf("Expected chunk text to remain unchanged: got %q, want %q", chunks[0].Text, chunk.Text)
	}
}

func TestGetChunkID(t *testing.T) {
	tests := []struct {
		name       string
		documentID string
		chunkIndex int
		expected   string
	}{
		{
			name:       "first chunk",
			documentID: "doc1",
			chunkIndex: 0,
			expected:   "doc1",
		},
		{
			name:       "second chunk",
			documentID: "doc1",
			chunkIndex: 1,
			expected:   "doc1_chunk_1",
		},
		{
			name:       "high index",
			documentID: "test-doc",
			chunkIndex: 15,
			expected:   "test-doc_chunk_15",
		},
		{
			name:       "complex doc id",
			documentID: "user_123_document",
			chunkIndex: 3,
			expected:   "user_123_document_chunk_3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetChunkID(tt.documentID, tt.chunkIndex)
			if result != tt.expected {
				t.Errorf("GetChunkID(%q, %d) = %q, want %q", tt.documentID, tt.chunkIndex, result, tt.expected)
			}
		})
	}
}

func TestGenerateDocumentID(t *testing.T) {
	// Test that the function generates valid IDs
	for i := 0; i < 10; i++ {
		id := GenerateDocumentID()

		// Check format: should be adjective-noun-YYMMDD-HHMM (4 parts)
		parts := strings.Split(id, "-")
		if len(parts) != 4 {
			t.Errorf("Expected ID to have 4 parts separated by hyphens, got %d parts: %s", len(parts), id)
		}

		// Check that it's not empty
		if id == "" {
			t.Error("Generated ID should not be empty")
		}

		// Check that it contains only valid characters (alphanumeric and hyphens)
		if matched, _ := regexp.MatchString("^[a-z0-9-]+$", id); !matched {
			t.Errorf("Generated ID contains invalid characters: %s", id)
		}

		// Check length is reasonable (should be under 30 characters for readability)
		if len(id) > 30 {
			t.Errorf("Generated ID is too long (%d chars): %s", len(id), id)
		}

		// Check that first part is from adjectives list
		adjective := parts[0]
		adjectives := []string{
			"happy", "bright", "swift", "clever", "gentle", "bold", "calm", "wise",
			"brave", "quick", "sharp", "smart", "clean", "fresh", "light", "clear",
		}
		found := false
		for _, adj := range adjectives {
			if adjective == adj {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Generated ID adjective '%s' not in expected list", adjective)
		}
	}

	// Test uniqueness: generate multiple IDs and ensure they are different
	// (given timestamp precision and randomness, they should be unique)
	ids := make(map[string]bool)
	for i := 0; i < 5; i++ {
		id := GenerateDocumentID()
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true

		// Sleep a tiny bit to ensure timestamp differences
		time.Sleep(time.Millisecond)
	}
}

func TestChunk_Struct(t *testing.T) {
	// Test that Chunk struct works as expected
	pageNum := 1
	chunk := Chunk{
		Text:       "Test chunk",
		Index:      0,
		StartPos:   10,
		EndPos:     20,
		TokenCount: 2,
		PageNumber: &pageNum,
		ChunkType:  "pdf_page",
	}

	if chunk.Text != "Test chunk" {
		t.Errorf("Expected Text to be 'Test chunk', got %q", chunk.Text)
	}
	if chunk.PageNumber == nil || *chunk.PageNumber != 1 {
		t.Errorf("Expected PageNumber to be 1, got %v", chunk.PageNumber)
	}
	if chunk.ChunkType != "pdf_page" {
		t.Errorf("Expected ChunkType to be 'pdf_page', got %q", chunk.ChunkType)
	}

	// Test with nil page number
	chunk2 := Chunk{
		Text:       "Test chunk 2",
		PageNumber: nil,
		ChunkType:  "text",
	}

	if chunk2.PageNumber != nil {
		t.Errorf("Expected PageNumber to be nil, got %v", chunk2.PageNumber)
	}
}

// Benchmark tests for performance
func BenchmarkTextChunker_EstimateTokenCount(b *testing.B) {
	chunker := NewTextChunker(1000, 200)
	text := strings.Repeat("This is a test sentence. ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.EstimateTokenCount(text)
	}
}

func BenchmarkTextChunker_ChunkText_Small(b *testing.B) {
	chunker := NewTextChunker(100, 20)
	text := strings.Repeat("This is a test sentence. ", 20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.ChunkText(text)
	}
}

func BenchmarkTextChunker_ChunkText_Large(b *testing.B) {
	chunker := NewTextChunker(500, 100)
	text := strings.Repeat("This is a test sentence with multiple words that will be chunked. ", 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunker.ChunkText(text)
	}
}

// Integration test with realistic text
func TestTextChunker_RealWorldExample(t *testing.T) {
	chunker := NewTextChunker(50, 10)

	text := `
	Artificial intelligence (AI) is intelligence demonstrated by machines, in contrast to the natural intelligence displayed by humans and animals.
	Leading AI textbooks define the field as the study of "intelligent agents": any device that perceives its environment and takes actions that maximize its chance of successfully achieving its goals.
	Colloquially, the term "artificial intelligence" is often used to describe machines that mimic "cognitive" functions that humans associate with the human mind, such as "learning" and "problem solving".

	As machines become increasingly capable, tasks considered to require "intelligence" are often removed from the definition of AI, a phenomenon known as the AI effect.
	A quip in Tesler's Theorem says "AI is whatever hasn't been done yet." For instance, optical character recognition is frequently excluded from things considered to be AI, having become a routine technology.
	Modern machine learning techniques are a core part of AI. Machine learning algorithms build a model based on sample data, known as "training data", in order to make predictions or decisions without being explicitly programmed to do so.
	`

	chunks := chunker.ChunkText(text)

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify basic properties
	for i, chunk := range chunks {
		if chunk.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has wrong index %d", i, chunk.Index)
		}
		if chunk.TokenCount > chunker.MaxTokens {
			t.Errorf("Chunk %d exceeds max tokens: %d > %d", i, chunk.TokenCount, chunker.MaxTokens)
		}
	}

	// Verify that we didn't lose significant content
	var allChunkText strings.Builder
	for _, chunk := range chunks {
		allChunkText.WriteString(chunk.Text)
		allChunkText.WriteString(" ")
	}

	originalWords := strings.Fields(text)
	chunkWords := strings.Fields(allChunkText.String())

	// Should preserve most words (allowing for some duplication due to overlap)
	if len(chunkWords) < len(originalWords) {
		t.Errorf("Lost words during chunking: original %d, chunks %d", len(originalWords), len(chunkWords))
	}
}
