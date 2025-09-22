package lilrag

import (
	"testing"
)

// Test the new ChunkTextWithFallback method
func TestTextChunker_ChunkTextWithFallback(t *testing.T) {
	tests := []struct {
		name        string
		maxTokens   int
		overlap     int
		text        string
		wantChunks  int
		wantPattern string // Pattern to check in first chunk
	}{
		{
			name:        "simple_text",
			maxTokens:   5,
			overlap:     1,
			text:        "This is a simple test document.",
			wantChunks:  2,
			wantPattern: "This is a simple test",
		},
		{
			name:        "single_chunk",
			maxTokens:   10,
			overlap:     2,
			text:        "Short text.",
			wantChunks:  1,
			wantPattern: "Short text.",
		},
		{
			name:        "empty_text",
			maxTokens:   5,
			overlap:     1,
			text:        "",
			wantChunks:  0,
			wantPattern: "",
		},
		{
			name:        "long_text",
			maxTokens:   3,
			overlap:     0,
			text:        "One two three four five six seven eight nine ten",
			wantChunks:  4,
			wantPattern: "One two three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewTextChunker(tt.maxTokens, tt.overlap)
			chunks := chunker.ChunkTextWithFallback(tt.text)

			if len(chunks) != tt.wantChunks {
				t.Errorf("ChunkTextWithFallback() got %d chunks, want %d", len(chunks), tt.wantChunks)
			}

			if tt.wantChunks > 0 && chunks[0].Text != tt.wantPattern {
				t.Errorf("ChunkTextWithFallback() first chunk = %q, want %q", chunks[0].Text, tt.wantPattern)
			}

			// Verify each chunk has correct token count
			for i, chunk := range chunks {
				expectedTokens := chunker.EstimateTokenCount(chunk.Text)
				if chunk.TokenCount != expectedTokens {
					t.Errorf("Chunk %d token count = %d, want %d", i, chunk.TokenCount, expectedTokens)
				}
			}
		})
	}
}

// Test chunker creation for different strategies
func TestCreateChunkerForStrategy(t *testing.T) {
	// Create a mock LilRag with default chunker
	defaultChunker := NewTextChunker(256, 38)
	rag := &LilRag{chunker: defaultChunker}

	tests := []struct {
		strategy     string
		wantTokens   int
		wantOverlap  int
	}{
		{"fast", 128, 19},
		{"contextual", 512, 76},
		{"legacy", 1800, 200},
		{"fallback", 256, 38}, // Uses current settings
		{"recursive", 256, 38}, // Uses current settings
		{"invalid", 256, 38},   // Uses current settings (default)
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			chunker := rag.createChunkerForStrategy(tt.strategy)
			
			if chunker.MaxTokens != tt.wantTokens {
				t.Errorf("createChunkerForStrategy(%s) MaxTokens = %d, want %d", 
					tt.strategy, chunker.MaxTokens, tt.wantTokens)
			}
			
			if chunker.Overlap != tt.wantOverlap {
				t.Errorf("createChunkerForStrategy(%s) Overlap = %d, want %d", 
					tt.strategy, chunker.Overlap, tt.wantOverlap)
			}
		})
	}
}