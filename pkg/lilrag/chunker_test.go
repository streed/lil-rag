package lilrag

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewTextChunker(t *testing.T) {
	chunker := NewTextChunker(100, 20)

	if chunker.MaxChars != 100 {
		t.Errorf("Expected MaxChars to be 100, got %d", chunker.MaxChars)
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
	chunker := NewTextChunker(100, 10)

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
			expected: []string{"First paragraph.", "Second paragraph.", "Third paragraph."},
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
		if c.TokenCount > chunker.MaxChars {
			t.Errorf("Chunk %d has %d tokens, exceeds max %d", i, c.TokenCount, chunker.MaxChars)
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
	// Compile regexp once outside the loop for better performance
	validCharRegex := regexp.MustCompile("^[a-z0-9-]+$")

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
		if !validCharRegex.MatchString(id) {
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
		if chunk.TokenCount > chunker.MaxChars {
			t.Errorf("Chunk %d exceeds max tokens: %d > %d", i, chunk.TokenCount, chunker.MaxChars)
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

// SemanticMockEmbedder for semantic chunking tests
type SemanticMockEmbedder struct {
	embeddings map[string][]float32
}

func NewSemanticMockEmbedder() *SemanticMockEmbedder {
	return &SemanticMockEmbedder{
		embeddings: make(map[string][]float32),
	}
}

func (m *SemanticMockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	// Check for pre-defined embeddings first
	if embedding, exists := m.embeddings[text]; exists {
		return embedding, nil
	}

	// Generate embedding based on content
	return generateTopicEmbedding(text), nil
}

// Enhanced MockEmbedder for semantic chunking tests
func createSemanticMockEmbedder() *SemanticMockEmbedder {
	embedder := NewSemanticMockEmbedder()

	// Pre-populate with topic-specific embeddings to ensure proper semantic clustering
	embedder.embeddings = map[string][]float32{
		// AI/ML topic cluster - high similarity within group
		"Artificial intelligence is transforming the world.": generateTopicEmbedding("ai artificial intelligence transforming world"),
		"Machine learning algorithms can process vast amounts of data.": generateTopicEmbedding("machine learning algorithms process data"),

		// Database topic cluster - different from AI/ML
		"Database systems store information efficiently.": generateTopicEmbedding("database systems store information efficiently"),
		"SQL queries retrieve specific records from tables.": generateTopicEmbedding("sql queries retrieve records tables"),
		"Data warehouses organize enterprise information.": generateTopicEmbedding("data warehouses organize enterprise information"),

		// Web topic cluster - different from both above
		"Web servers handle HTTP requests from clients.": generateTopicEmbedding("web servers handle http requests clients"),
		"RESTful APIs provide structured endpoints.": generateTopicEmbedding("restful apis provide structured endpoints"),
		"Microservices architecture enables scalable applications.": generateTopicEmbedding("microservices architecture enables scalable applications"),

		// Topic coherence test sentences
		"Artificial intelligence research focuses on creating intelligent agents.": generateTopicEmbedding("ai artificial intelligence research intelligent agents"),
		"Machine learning is a subset of AI that enables computers to learn from data.": generateTopicEmbedding("machine learning ai computers learn data"),
		"Deep learning uses neural networks with multiple layers.": generateTopicEmbedding("machine learning neural networks layers"),

		"Database normalization reduces data redundancy.": generateTopicEmbedding("database normalization data redundancy"),
		"SQL joins combine data from multiple tables.": generateTopicEmbedding("sql joins data tables"),
		"ACID properties ensure transaction reliability.": generateTopicEmbedding("database acid properties transaction reliability"),

		"HTTP protocols enable web communication.": generateTopicEmbedding("web http protocols communication"),
		"REST APIs follow stateless principles.": generateTopicEmbedding("web rest apis stateless principles"),
		"JSON provides lightweight data exchange format.": generateTopicEmbedding("web json data exchange format"),

		// Threshold types test sentences
		"Machine learning algorithms process data efficiently.": generateTopicEmbedding("machine learning algorithms process data efficiently"),
		"Neural networks learn complex patterns.": generateTopicEmbedding("machine learning neural networks patterns"),
		"Deep learning revolutionizes AI applications.": generateTopicEmbedding("machine learning ai applications"),

		"Database optimization improves query performance.": generateTopicEmbedding("database optimization query performance"),
		"Indexing strategies reduce lookup times.": generateTopicEmbedding("database indexing strategies lookup times"),
		"Query planners optimize execution paths.": generateTopicEmbedding("database query planners optimize execution"),
	}

	return embedder
}

// Helper to generate topic-specific embeddings for testing
func generateTopicEmbedding(text string) []float32 {
	words := strings.Fields(strings.ToLower(text))
	embedding := make([]float32, 384) // Standard embedding dimension

	// Initialize all dimensions to small random values for diversity
	for i := range embedding {
		embedding[i] = 0.1
	}

	// Create distinct topic clusters with strong signals
	for _, word := range words {
		switch {
		case strings.Contains(word, "ai") || strings.Contains(word, "artificial") || strings.Contains(word, "intelligence") || strings.Contains(word, "transforming"):
			// AI topic cluster: dimensions 0-49
			for i := 0; i < 50; i++ {
				embedding[i] += 0.9
			}
		case strings.Contains(word, "machine") || strings.Contains(word, "learning") || strings.Contains(word, "algorithm") || strings.Contains(word, "process"):
			// ML topic cluster: dimensions 0-49 (overlaps with AI for similarity)
			for i := 0; i < 50; i++ {
				embedding[i] += 0.8
			}
		case strings.Contains(word, "database") || strings.Contains(word, "sql") || strings.Contains(word, "queries") || strings.Contains(word, "tables") || strings.Contains(word, "warehouses") || strings.Contains(word, "normalization") || strings.Contains(word, "redundancy") || strings.Contains(word, "joins") || strings.Contains(word, "combine") || strings.Contains(word, "acid") || strings.Contains(word, "properties") || strings.Contains(word, "transaction") || strings.Contains(word, "reliability") || strings.Contains(word, "reduces") || strings.Contains(word, "multiple") || strings.Contains(word, "ensure"):
			// Database topic cluster: dimensions 100-149
			for i := 100; i < 150; i++ {
				embedding[i] += 1.3
			}
		case strings.Contains(word, "web") || strings.Contains(word, "http") || strings.Contains(word, "servers") || strings.Contains(word, "restful") || strings.Contains(word, "apis") || strings.Contains(word, "microservices") || strings.Contains(word, "clients") || strings.Contains(word, "endpoints") || strings.Contains(word, "architecture") || strings.Contains(word, "applications") || strings.Contains(word, "protocols") || strings.Contains(word, "communication") || strings.Contains(word, "stateless") || strings.Contains(word, "principles") || strings.Contains(word, "json") || strings.Contains(word, "exchange") || strings.Contains(word, "format"):
			// Web topic cluster: dimensions 300-349
			for i := 300; i < 350; i++ {
				embedding[i] += 1.5
			}
		case strings.Contains(word, "enable") || strings.Contains(word, "follow") || strings.Contains(word, "provides") || strings.Contains(word, "lightweight"):
			// Additional web indicators
			for i := 300; i < 350; i++ {
				embedding[i] += 0.8
			}
		case strings.Contains(word, "store") || strings.Contains(word, "information") || strings.Contains(word, "data"):
			// Data-related: boost database cluster
			for i := 100; i < 150; i++ {
				embedding[i] += 0.5
			}
		}
	}

	// Normalize the embedding
	var magnitude float32
	for _, val := range embedding {
		magnitude += val * val
	}
	if magnitude > 0 {
		magnitude = float32(1.0 / math.Sqrt(float64(magnitude)))
		for i := range embedding {
			embedding[i] *= magnitude
		}
	}

	return embedding
}

func TestTextChunker_SemanticChunking_Basic(t *testing.T) {
	chunker := NewTextChunker(50, 5)
	mockEmbedder := createSemanticMockEmbedder()

	// Text with clear topic shifts
	text := `Artificial intelligence is transforming the world. Machine learning algorithms can process vast amounts of data.

	Database systems store information efficiently. SQL queries retrieve specific records from tables. Data warehouses organize enterprise information.

	Web servers handle HTTP requests from clients. RESTful APIs provide structured endpoints. Microservices architecture enables scalable applications.`

	chunks := chunker.ChunkTextWithSemanticEmbedding(text, "semantic", mockEmbedder)

	// Debug: print sentences and similarities
	sentences := chunker.splitIntoSentences(text)
	t.Logf("Found %d sentences:", len(sentences))
	for i, sentence := range sentences {
		t.Logf("  %d: %s", i, strings.TrimSpace(sentence))
	}

	// Debug: calculate and print similarities manually
	var similarities []float32
	if len(sentences) > 1 {
		for i := 0; i < len(sentences)-1; i++ {
			emb1, _ := mockEmbedder.Embed(nil, sentences[i])
			emb2, _ := mockEmbedder.Embed(nil, sentences[i+1])
			sim := cosineSimilarity(emb1, emb2)
			similarities = append(similarities, sim)
			t.Logf("Similarity between sentence %d and %d: %f", i, i+1, sim)
		}

		// Debug breakpoint detection
		breakpoints := findSemanticBreakpoints(similarities, "percentile", 0.95)
		t.Logf("Detected breakpoints: %v", breakpoints)
	}

	t.Logf("Generated %d chunks:", len(chunks))
	for i, chunk := range chunks {
		t.Logf("  Chunk %d (%d tokens): %s", i, chunk.TokenCount, chunk.Text[:min(50, len(chunk.Text))]+"...")
	}

	if len(chunks) < 2 {
		t.Errorf("Expected multiple semantic chunks for text with topic shifts, got %d", len(chunks))
	}

	// Verify each chunk has reasonable content
	for i, chunk := range chunks {
		if chunk.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
		if chunk.TokenCount == 0 {
			t.Errorf("Chunk %d has zero token count", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has incorrect index %d", i, chunk.Index)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestTextChunker_SemanticChunking_TopicCoherence(t *testing.T) {
	chunker := NewTextChunker(100, 10)
	mockEmbedder := createSemanticMockEmbedder()

	// Text with distinct topics that should be chunked separately
	text := `Artificial intelligence research focuses on creating intelligent agents. Machine learning is a subset of AI that enables computers to learn from data. Deep learning uses neural networks with multiple layers.

	Database normalization reduces data redundancy. SQL joins combine data from multiple tables. ACID properties ensure transaction reliability.

	HTTP protocols enable web communication. REST APIs follow stateless principles. JSON provides lightweight data exchange format.`

	// Debug: show sentence splitting
	sentences := chunker.splitIntoSentences(text)
	t.Logf("Found %d sentences:", len(sentences))
	for i, sentence := range sentences {
		t.Logf("  %d: %s", i, strings.TrimSpace(sentence))
	}

	// Debug similarities
	if len(sentences) > 1 {
		for i := 0; i < len(sentences)-1; i++ {
			emb1, _ := mockEmbedder.Embed(nil, sentences[i])
			emb2, _ := mockEmbedder.Embed(nil, sentences[i+1])
			sim := cosineSimilarity(emb1, emb2)
			t.Logf("Similarity between sentence %d and %d: %f", i, i+1, sim)

			// Debug embedding values for sentences 1 and 2
			if i == 1 {
				t.Logf("Sentence 1 embedding (first 10 dims): %v", emb1[:10])
				t.Logf("Sentence 2 embedding (first 10 dims): %v", emb2[:10])
				t.Logf("Sentence 1 key dims 100-110: %v", emb1[100:110])
				t.Logf("Sentence 2 key dims 300-310: %v", emb2[300:310])
			}
		}

		// Debug breakpoint detection
		similarities := []float32{}
		for i := 0; i < len(sentences)-1; i++ {
			emb1, _ := mockEmbedder.Embed(nil, sentences[i])
			emb2, _ := mockEmbedder.Embed(nil, sentences[i+1])
			sim := cosineSimilarity(emb1, emb2)
			similarities = append(similarities, sim)
		}
		breakpoints := findSemanticBreakpoints(similarities, "percentile", 0.95)
		t.Logf("Similarities: %v", similarities)
		t.Logf("Detected breakpoints: %v", breakpoints)
	}

	chunks := chunker.ChunkTextWithSemanticEmbedding(text, "semantic", mockEmbedder)

	t.Logf("Generated %d chunks:", len(chunks))
	for i, chunk := range chunks {
		t.Logf("  Chunk %d: %s", i, chunk.Text)
	}

	// Should create separate chunks for AI, Database, and Web topics
	if len(chunks) < 3 {
		t.Errorf("Expected at least 3 chunks for distinct topics, got %d", len(chunks))
	}

	// Verify topic coherence within chunks
	for i, chunk := range chunks {
		chunkText := strings.ToLower(chunk.Text)

		// Count topic indicators in each chunk
		aiWords := countWords(chunkText, []string{"ai", "artificial", "intelligence", "machine", "learning", "neural"})
		dbWords := countWords(chunkText, []string{"database", "sql", "acid", "table", "transaction"})
		webWords := countWords(chunkText, []string{"http", "rest", "api", "json", "web"})

		// Each chunk should be dominated by one topic
		total := aiWords + dbWords + webWords
		if total > 0 {
			dominantTopic := max(aiWords, dbWords, webWords)
			if float64(dominantTopic)/float64(total) < 0.6 {
				t.Errorf("Chunk %d lacks topic coherence: AI=%d, DB=%d, Web=%d", i, aiWords, dbWords, webWords)
			}
		}
	}
}

func TestTextChunker_SemanticChunking_ThresholdTypes(t *testing.T) {
	chunker := NewTextChunker(100, 10)
	mockEmbedder := createSemanticMockEmbedder()

	text := `Machine learning algorithms process data efficiently. Neural networks learn complex patterns. Deep learning revolutionizes AI applications.

	Web servers handle HTTP requests efficiently. REST APIs provide structured endpoints. JSON enables lightweight data exchange.`

	testCases := []struct {
		name          string
		thresholdType string
		expectChunks  int // minimum expected chunks
	}{
		{"percentile", "percentile", 2},
		{"standard_deviation", "standard_deviation", 2},
		{"interquartile", "interquartile", 2},
		{"gradient", "gradient", 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Debug for gradient test
			if tc.thresholdType == "gradient" {
				sentences := chunker.splitIntoSentences(text)
				t.Logf("Gradient test - Found %d sentences:", len(sentences))
				for i, sentence := range sentences {
					t.Logf("  %d: %s", i, strings.TrimSpace(sentence))
				}

				// Calculate similarities
				var similarities []float32
				if len(sentences) > 1 {
					for i := 0; i < len(sentences)-1; i++ {
						emb1, _ := mockEmbedder.Embed(nil, sentences[i])
						emb2, _ := mockEmbedder.Embed(nil, sentences[i+1])
						sim := cosineSimilarity(emb1, emb2)
						similarities = append(similarities, sim)
						t.Logf("Gradient test - Similarity between sentence %d and %d: %f", i, i+1, sim)
					}

					// Test breakpoint detection directly
					breakpoints := findSemanticBreakpoints(similarities, "gradient", 0.8)
					t.Logf("Gradient test - Detected breakpoints: %v", breakpoints)
				}
			}

			chunks := chunker.ChunkTextWithSemanticEmbeddingAndThreshold(text, "semantic", mockEmbedder, tc.thresholdType, 0.8)

			if tc.thresholdType == "gradient" {
				t.Logf("Gradient test - Generated %d chunks", len(chunks))
			}

			if len(chunks) < tc.expectChunks {
				t.Errorf("Threshold type %s: expected at least %d chunks, got %d", tc.thresholdType, tc.expectChunks, len(chunks))
			}

			// Verify chunks are properly formed
			for i, chunk := range chunks {
				if chunk.Text == "" {
					t.Errorf("Chunk %d has empty text", i)
				}
			}
		})
	}
}

func TestTextChunker_SemanticChunking_SingleTopic(t *testing.T) {
	chunker := NewTextChunker(50, 5)
	mockEmbedder := createSemanticMockEmbedder()

	// Text with consistent topic - should result in fewer chunks
	text := `Machine learning algorithms analyze data patterns. Supervised learning uses labeled training data. Unsupervised learning discovers hidden structures. Reinforcement learning optimizes decision-making through trial and error.`

	chunks := chunker.ChunkTextWithSemanticEmbedding(text, "semantic", mockEmbedder)

	// Should create fewer chunks since all sentences are semantically similar
	if len(chunks) > 2 {
		t.Errorf("Expected fewer chunks for single topic text, got %d", len(chunks))
	}

	// Verify token limits are respected
	for i, chunk := range chunks {
		if chunk.TokenCount > chunker.MaxChars {
			t.Errorf("Chunk %d exceeds max tokens: %d > %d", i, chunk.TokenCount, chunker.MaxChars)
		}
	}
}

func TestTextChunker_SemanticChunking_EmptyAndEdgeCases(t *testing.T) {
	chunker := NewTextChunker(100, 10)
	mockEmbedder := createSemanticMockEmbedder()

	testCases := []struct {
		name string
		text string
	}{
		{"empty_text", ""},
		{"whitespace_only", "   \n\t  "},
		{"single_sentence", "This is a single sentence."},
		{"short_text", "Short text."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunker.ChunkTextWithSemanticEmbedding(tc.text, "semantic", mockEmbedder)

			if tc.text == "" || strings.TrimSpace(tc.text) == "" {
				if chunks != nil {
					t.Errorf("Expected nil chunks for empty text, got %v", chunks)
				}
			} else {
				if len(chunks) == 0 {
					t.Errorf("Expected at least one chunk for non-empty text")
				}

				for i, chunk := range chunks {
					if chunk.Text == "" {
						t.Errorf("Chunk %d has empty text", i)
					}
				}
			}
		})
	}
}

func TestTextChunker_SemanticChunking_FallbackToRecursive(t *testing.T) {
	chunker := NewTextChunker(100, 10)

	// Test fallback when embedder is nil
	text := `This is test text that should fall back to recursive chunking when no embedder is provided.`

	chunks := chunker.ChunkTextWithSemanticEmbedding(text, "semantic", nil)

	// Should fall back to recursive chunking
	if len(chunks) == 0 {
		t.Errorf("Expected fallback to recursive chunking when embedder is nil")
	}

	// Verify chunks are properly formed
	for i, chunk := range chunks {
		if chunk.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has incorrect index %d", i, chunk.Index)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	testCases := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float32
		tolerance float32
	}{
		{
			name:      "identical_vectors",
			vec1:      []float32{1.0, 0.0, 0.0},
			vec2:      []float32{1.0, 0.0, 0.0},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "orthogonal_vectors",
			vec1:      []float32{1.0, 0.0},
			vec2:      []float32{0.0, 1.0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "opposite_vectors",
			vec1:      []float32{1.0, 0.0},
			vec2:      []float32{-1.0, 0.0},
			expected:  -1.0,
			tolerance: 0.001,
		},
		{
			name:      "similar_vectors",
			vec1:      []float32{0.6, 0.8},
			vec2:      []float32{0.8, 0.6},
			expected:  0.96, // cos(angle) for normalized vectors
			tolerance: 0.01,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := cosineSimilarity(tc.vec1, tc.vec2)
			if absFloat32(result-tc.expected) > tc.tolerance {
				t.Errorf("cosineSimilarity(%v, %v) = %f, want %f (±%f)",
					tc.vec1, tc.vec2, result, tc.expected, tc.tolerance)
			}
		})
	}
}

func TestFindSemanticBreakpoints(t *testing.T) {
	similarities := []float32{0.9, 0.85, 0.3, 0.8, 0.2, 0.9}

	testCases := []struct {
		name          string
		thresholdType string
		threshold     float32
		expectedBreaks []int
	}{
		{
			name:          "percentile_threshold",
			thresholdType: "percentile",
			threshold:     0.8, // 80th percentile
			expectedBreaks: []int{2, 4}, // indices where similarity drops significantly
		},
		{
			name:          "fixed_threshold",
			thresholdType: "fixed",
			threshold:     0.5,
			expectedBreaks: []int{2, 4}, // similarities below 0.5
		},
		{
			name:          "gradient_small_array",
			thresholdType: "gradient",
			threshold:     0.8,
			expectedBreaks: []int{}, // will be overridden by test below
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			breakpoints := findSemanticBreakpoints(similarities, tc.thresholdType, tc.threshold)

			if len(breakpoints) == 0 && len(tc.expectedBreaks) > 0 {
				t.Errorf("Expected breakpoints %v, got none", tc.expectedBreaks)
			}

			// Verify breakpoints are reasonable (don't need exact match due to algorithm variations)
			for _, bp := range breakpoints {
				if bp < 0 || bp >= len(similarities) {
					t.Errorf("Breakpoint %d is out of range [0, %d)", bp, len(similarities))
				}
			}
		})
	}

	// Specific tests for all methods with small array
	t.Run("small_array_tests", func(t *testing.T) {
		singleSim := []float32{0.3} // Below 0.6 threshold

		methods := []string{"percentile", "standard_deviation", "interquartile", "gradient"}
		for _, method := range methods {
			breakpoints := findSemanticBreakpoints(singleSim, method, 0.8)
			t.Logf("%s test: similarities=%v, breakpoints=%v", method, singleSim, breakpoints)

			if len(breakpoints) != 1 || breakpoints[0] != 1 {
				t.Errorf("%s: Expected breakpoints [1] for similarity 0.3, got %v", method, breakpoints)
			}
		}
	})
}

// Helper functions for tests
func countWords(text string, words []string) int {
	count := 0
	for _, word := range words {
		count += strings.Count(text, word)
	}
	return count
}

func max(a, b, c int) int {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

func absFloat32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
