package lilrag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestNewTextParser(t *testing.T) {
	parser := NewTextParser()
	if parser == nil {
		t.Fatal("NewTextParser() returned nil")
	}
	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}
}

func TestTextParser_SupportedExtensions(t *testing.T) {
	parser := NewTextParser()
	extensions := parser.SupportedExtensions()

	expected := []string{".txt", ".md", ".text"}
	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for i, ext := range expected {
		if extensions[i] != ext {
			t.Errorf("Expected extension %s, got %s", ext, extensions[i])
		}
	}
}

func TestTextParser_GetDocumentType(t *testing.T) {
	parser := NewTextParser()
	docType := parser.GetDocumentType()

	if docType != DocumentTypeTXT {
		t.Errorf("Expected DocumentTypeTXT, got %v", docType)
	}
}

func TestTextParser_Parse(t *testing.T) {
	parser := NewTextParser()

	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:    "simple text",
			content: "Hello, World!",
		},
		{
			name:    "empty file",
			content: "",
		},
		{
			name:    "multiline text",
			content: "Line 1\nLine 2\nLine 3",
		},
		{
			name:    "text with unicode",
			content: "Hello 世界 🌍 Ñoño",
		},
		{
			name:    "markdown content",
			content: "# Header\n\n**Bold** text with *italics*.\n\n- List item 1\n- List item 2",
		},
		{
			name:    "text with special characters",
			content: "Special: !@#$%^&*()[]{}|\\:;\"'<>,.?/~`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "test_*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Write test content
			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Parse the file
			result, err := parser.Parse(tmpFile.Name())

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.content {
				t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", tt.content, result)
			}
		})
	}
}

func TestTextParser_Parse_FileErrors(t *testing.T) {
	parser := NewTextParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file",
			filePath: "/nonexistent/file.txt",
		},
		{
			name:     "empty path",
			filePath: "",
		},
		{
			name:     "directory instead of file",
			filePath: os.TempDir(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.filePath)
			if err == nil {
				t.Error("Expected error for invalid file path")
			}
		})
	}
}

func TestTextParser_ParseWithChunks(t *testing.T) {
	parser := NewTextParser()

	tests := []struct {
		name           string
		content        string
		expectedChunks int
		minChunkSize   int
	}{
		{
			name:           "short text - single chunk",
			content:        "This is a short text that should fit in one chunk.",
			expectedChunks: 1,
			minChunkSize:   40,
		},
		{
			name:           "long text - multiple chunks",
			content:        strings.Repeat("This is a sentence that will be repeated many times to create a long document. ", 20),
			expectedChunks: 20, // Recursive chunker splits by sentences, each repeat is a sentence
			minChunkSize:   50,
		},
		{
			name:           "empty content",
			content:        "",
			expectedChunks: 0,
			minChunkSize:   0,
		},
		{
			name:           "single word",
			content:        "word",
			expectedChunks: 1,
			minChunkSize:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "test_*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Write test content
			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Parse with chunks
			chunks, err := parser.ParseWithChunks(tmpFile.Name(), "test-doc")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(chunks) != tt.expectedChunks {
				t.Errorf("Expected %d chunks, got %d", tt.expectedChunks, len(chunks))
			}

			// Verify chunk content
			totalContent := ""
			for i, chunk := range chunks {
				if len(chunk.Text) < tt.minChunkSize && tt.content != "" && len(chunks) == 1 {
					// Allow single chunks to be smaller than minChunkSize for short content
				} else if len(chunk.Text) < tt.minChunkSize && tt.minChunkSize > 0 && len(chunks) > 1 {
					t.Errorf("Chunk %d too small: %d characters (expected at least %d)", i, len(chunk.Text), tt.minChunkSize)
				}

				if chunk.Index != i {
					t.Errorf("Chunk %d has wrong index: %d", i, chunk.Index)
				}

				totalContent += chunk.Text
			}

			// Verify content preservation (allowing for overlap in multiple chunks)
			if tt.content != "" {
				originalLen := len(strings.TrimSpace(strings.ReplaceAll(tt.content, " ", "")))
				reconstructedLen := len(strings.TrimSpace(strings.ReplaceAll(totalContent, " ", "")))

				// For single chunks, content should be preserved exactly (within 10%)
				// For multiple chunks, allow for overlap which can increase total length significantly
				if len(chunks) == 1 {
					if abs(originalLen-reconstructedLen) > originalLen/10 {
						t.Errorf("Significant content difference in single chunk.\nOriginal length: %d\nReconstructed length: %d",
							originalLen, reconstructedLen)
					}
				} else {
					// With multiple chunks and overlap, reconstructed length should be >= original
					// but not unreasonably large (allow up to 3x for heavy overlap)
					if reconstructedLen < originalLen || reconstructedLen > originalLen*3 {
						t.Errorf("Content length out of expected range with overlap.\nOriginal length: %d\nReconstructed length: %d\nExpected: %d to %d",
							originalLen, reconstructedLen, originalLen, originalLen*3)
					}
				}
			}
		})
	}
}

func TestTextParser_ParseWithChunks_CustomChunker(t *testing.T) {
	// Test that chunker is initialized automatically
	parser := NewTextParser()

	content := "This is test content for chunking."
	tmpFile, err := os.CreateTemp("", "test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// First call should create chunker
	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}

	chunks, err := parser.ParseWithChunks(tmpFile.Name(), "test-doc")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if parser.chunker == nil {
		t.Error("Expected chunker to be created after first call")
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
}

func TestTextParser_ParseWithChunks_FileErrors(t *testing.T) {
	parser := NewTextParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file",
			filePath: "/nonexistent/file.txt",
		},
		{
			name:     "directory instead of file",
			filePath: os.TempDir(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ParseWithChunks(tt.filePath, "test-doc")
			if err == nil {
				t.Error("Expected error for invalid file path")
			}
		})
	}
}

func TestTextParser_Integration(t *testing.T) {
	// Test a complete workflow with different file types
	parser := NewTextParser()

	testFiles := []struct {
		name      string
		extension string
		content   string
	}{
		{
			name:      "plain text",
			extension: ".txt",
			content:   "This is a plain text file with some content.",
		},
		{
			name:      "markdown",
			extension: ".md",
			content:   "# Markdown File\n\nThis is **markdown** content with *formatting*.",
		},
		{
			name:      "text file",
			extension: ".text",
			content:   "This is a .text file which should also be supported.",
		},
	}

	for _, tf := range testFiles {
		t.Run(tf.name, func(t *testing.T) {
			// Create temporary file with specific extension
			tmpFile, err := os.CreateTemp("", "test_*"+tf.extension)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Write content
			if _, err := tmpFile.WriteString(tf.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test Parse
			content, err := parser.Parse(tmpFile.Name())
			if err != nil {
				t.Errorf("Parse failed: %v", err)
			}
			if content != tf.content {
				t.Errorf("Content mismatch for %s", tf.name)
			}

			// Test ParseWithChunks
			chunks, err := parser.ParseWithChunks(tmpFile.Name(), "test-doc")
			if err != nil {
				t.Errorf("ParseWithChunks failed: %v", err)
			}
			if len(chunks) == 0 {
				t.Error("Expected at least one chunk")
			}

			// Verify extension is supported
			ext := filepath.Ext(tmpFile.Name())
			supported := false
			for _, supportedExt := range parser.SupportedExtensions() {
				if ext == supportedExt {
					supported = true
					break
				}
			}
			if !supported {
				t.Errorf("Extension %s should be supported but isn't in list", ext)
			}
		})
	}
}

func TestTextParser_LargeFile(t *testing.T) {
	parser := NewTextParser()

	// Create a larger content for testing chunking
	var content strings.Builder
	for i := 0; i < 100; i++ {
		content.WriteString("This is sentence number ")
		content.WriteString(string(rune('0' + i%10)))
		content.WriteString(". It contains some meaningful content for testing purposes. ")
		if i%10 == 9 {
			content.WriteString("\n\n")
		}
	}

	tmpFile, err := os.CreateTemp("", "large_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test parsing
	result, err := parser.Parse(tmpFile.Name())
	if err != nil {
		t.Errorf("Failed to parse large file: %v", err)
	}
	if result != content.String() {
		t.Error("Large file content not preserved")
	}

	// Test chunking
	chunks, err := parser.ParseWithChunks(tmpFile.Name(), "large-doc")
	if err != nil {
		t.Errorf("Failed to chunk large file: %v", err)
	}

	if len(chunks) < 2 {
		t.Error("Expected multiple chunks for large file")
	}

	// Verify all chunks have content
	for i, chunk := range chunks {
		if len(chunk.Text) == 0 {
			t.Errorf("Chunk %d is empty", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has incorrect index %d", i, chunk.Index)
		}
	}
}
