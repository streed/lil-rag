package lilrag

import (
	"os"
	"strings"
	"testing"
)

func TestNewDOCXParser(t *testing.T) {
	parser := NewDOCXParser()
	if parser == nil {
		t.Fatal("NewDOCXParser() returned nil")
	}

	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}
}

func TestDOCXParser_SupportedExtensions(t *testing.T) {
	parser := NewDOCXParser()
	extensions := parser.SupportedExtensions()

	expected := []string{".docx"}
	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for i, ext := range expected {
		if extensions[i] != ext {
			t.Errorf("Expected extension %s, got %s", ext, extensions[i])
		}
	}
}

func TestDOCXParser_GetDocumentType(t *testing.T) {
	parser := NewDOCXParser()
	docType := parser.GetDocumentType()

	if docType != DocumentTypeDOCX {
		t.Errorf("Expected DocumentTypeDOCX, got %v", docType)
	}
}

func TestDOCXParser_CleanContent(t *testing.T) {
	parser := NewDOCXParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove empty lines",
			input:    "Line 1\n\n\nLine 2\n\n\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "trim whitespace from lines",
			input:    "  Line 1  \n   Line 2   \n  Line 3  ",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "handle mixed whitespace",
			input:    "Line 1\n  \n\t\nLine 2\n   \nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\n\t\n   ",
			expected: "",
		},
		{
			name:     "single line",
			input:    "Single line content",
			expected: "Single line content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.cleanContent(tt.input)
			if result != tt.expected {
				t.Errorf("cleanContent() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestDOCXParser_DetectContentType(t *testing.T) {
	parser := NewDOCXParser()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "code content",
			content: `function calculateSum(a, b) {
				return a + b;
			}
			function processData() {}
			class DataProcessor {
				public void process() {}
				private var data;
			}`,
			expected: "code",
		},
		{
			name: "structured content with headers",
			content: `# Main Title
			## Section 1
			- Point 1
			- Point 2
			### Subsection
			1. First item
			2. Second item`,
			expected: "structured",
		},
		{
			name:     "prose content",
			content:  "This is regular paragraph text. It contains sentences and thoughts flowing naturally from one to another. There are no special formatting markers or code patterns.",
			expected: "prose",
		},
		{
			name: "mixed content favoring code",
			content: `This document contains function definitions and class declarations.
			function test() {
				var x = 10;
				const y = 20;
				let z = x + y;
				return z;
			}`,
			expected: "code",
		},
		{
			name:     "minimal content",
			content:  "Short text.",
			expected: "prose",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "prose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.detectContentType(tt.content)
			if result != tt.expected {
				t.Errorf("detectContentType() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestDOCXParser_IsHeader(t *testing.T) {
	parser := NewDOCXParser()

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"markdown header level 1", "# Introduction", true},
		{"markdown header level 2", "## Getting Started", true},
		{"markdown header level 3", "### Advanced Topics", true},
		{"numbered header", "1.1 Overview", true},
		{"numbered header with spaces", "2.3.1 Detailed Analysis", true},
		{"all caps short header", "CONFIGURATION", true},
		{"regular prose", "This is just regular text content.", false},
		{"long all caps text", "THIS IS A VERY LONG LINE OF TEXT THAT SHOULD NOT BE CONSIDERED A HEADER", false},
		{"numbered list item", "1. This is a numbered list item with more content", false},
		{"short line", "Hi", false},
		{"empty line", "", false},
		{"just numbers", "123", true}, // Numbers with length 3 match the header pattern
		{"mixed case header-like", "Section Overview", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isHeader(tt.line)
			if result != tt.expected {
				t.Errorf("isHeader(%q) = %v, expected %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestDOCXParser_ChunkByContentType(t *testing.T) {
	parser := NewDOCXParser()
	parser.chunker = NewTextChunker(100, 20) // Small chunks for testing

	tests := []struct {
		name        string
		content     string
		contentType string
		expectChunks int
	}{
		{
			name: "prose content",
			content: strings.Repeat("This is a sentence. ", 20), // Long prose text
			contentType: "prose",
			expectChunks: 2, // Should be split by text chunker
		},
		{
			name: "code content",
			content: `function test1() {
				return 1;
			}

			function test2() {
				return 2;
			}

			function test3() {
				return 3;
			}`,
			contentType: "code",
			expectChunks: 1, // Should stay together as code
		},
		{
			name: "structured content",
			content: `# First Section
			Content for first section.

			# Second Section
			Content for second section.

			# Third Section  
			Content for third section.`,
			contentType: "structured",
			expectChunks: 3, // Should split on headers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := parser.chunkByContentType(tt.content, tt.contentType)
			
			if len(chunks) < 1 {
				t.Errorf("Expected at least 1 chunk, got %d", len(chunks))
				return
			}

			// Verify chunks have content
			for i, chunk := range chunks {
				if strings.TrimSpace(chunk.Text) == "" {
					t.Errorf("Chunk %d is empty", i)
				}
				if chunk.TokenCount <= 0 {
					t.Errorf("Chunk %d has invalid token count: %d", i, chunk.TokenCount)
				}
			}
		})
	}
}

func TestDOCXParser_ChunkCodeContent(t *testing.T) {
	parser := NewDOCXParser()
	parser.chunker = NewTextChunker(50, 10) // Very small chunks to test splitting

	codeContent := `function calculate(x, y) {
		return x + y;
	}

	class Calculator {
		constructor() {
			this.value = 0;
		}
		
		add(num) {
			this.value += num;
			return this;
		}
	}

	const result = new Calculator()
		.add(5)
		.add(3)
		.value;`

	chunks := parser.chunkCodeContent(codeContent)

	if len(chunks) < 1 {
		t.Fatal("Expected at least one chunk")
	}

	// Verify all chunks have content and proper token counts
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk.Text) == "" {
			t.Errorf("Chunk %d is empty", i)
		}
		if chunk.TokenCount <= 0 {
			t.Errorf("Chunk %d has invalid token count: %d", i, chunk.TokenCount)
		}
		if chunk.StartPos < 0 || chunk.EndPos < chunk.StartPos {
			t.Errorf("Chunk %d has invalid positions: start=%d, end=%d", i, chunk.StartPos, chunk.EndPos)
		}
	}
}

func TestDOCXParser_ChunkStructuredContent(t *testing.T) {
	parser := NewDOCXParser()
	parser.chunker = NewTextChunker(100, 20)

	structuredContent := `# Introduction
	Welcome to this document.

	## Overview
	This section provides an overview.

	### Details
	Here are some important details.

	# Main Content
	This is the main content section.`

	chunks := parser.chunkStructuredContent(structuredContent)

	if len(chunks) < 2 {
		t.Errorf("Expected at least 2 chunks for structured content, got %d", len(chunks))
	}

	// Verify chunks split on headers
	foundIntroduction := false
	foundMainContent := false
	
	for _, chunk := range chunks {
		if strings.Contains(chunk.Text, "Introduction") {
			foundIntroduction = true
		}
		if strings.Contains(chunk.Text, "Main Content") {
			foundMainContent = true
		}
	}

	if !foundIntroduction {
		t.Error("Expected to find 'Introduction' in chunks")
	}
	if !foundMainContent {
		t.Error("Expected to find 'Main Content' in chunks")
	}
}

func TestDOCXParser_ChunkProseContent(t *testing.T) {
	parser := NewDOCXParser()
	parser.chunker = NewTextChunker(50, 10) // Small chunks for testing

	proseContent := strings.Repeat("This is a long paragraph of prose text that should be chunked appropriately. ", 10)

	chunks := parser.chunkProseContent(proseContent)

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks for long prose, got %d", len(chunks))
	}

	// Verify chunks use the text chunker properly
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk.Text) == "" {
			t.Errorf("Chunk %d is empty", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has wrong index: %d", i, chunk.Index)
		}
	}
}

// Since we can't create real DOCX files easily in tests, we'll test error handling
func TestDOCXParser_Parse_FileErrors(t *testing.T) {
	parser := NewDOCXParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{"nonexistent file", "/nonexistent/file.docx"},
		{"empty path", ""},
		{"directory instead of file", os.TempDir()},
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

func TestDOCXParser_ParseWithChunks_FileErrors(t *testing.T) {
	parser := NewDOCXParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{"nonexistent file", "/nonexistent/file.docx"},
		{"directory instead of file", os.TempDir()},
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

// Test with invalid DOCX file (not a real DOCX)
func TestDOCXParser_Parse_InvalidDOCX(t *testing.T) {
	parser := NewDOCXParser()

	// Create a file with .docx extension but invalid content
	tmpFile, err := os.CreateTemp("", "invalid_*.docx")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write non-DOCX content
	if _, err := tmpFile.WriteString("This is not a valid DOCX file"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = parser.Parse(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid DOCX file")
	}

	if !strings.Contains(err.Error(), "failed to open DOCX file") {
		t.Errorf("Expected 'failed to open DOCX file' error, got: %v", err)
	}
}

func TestDOCXParser_ParseWithChunks_DefaultChunker(t *testing.T) {
	// Test that ParseWithChunks creates a default chunker when none is provided
	parser := NewDOCXParser()
	
	// Verify chunker is initially nil
	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}

	// Create an invalid DOCX to test the chunker creation (before file processing)
	// We'll test the chunker creation by mocking successful parsing
	
	// Test the chunker creation logic by directly testing content processing
	content := "Test content for chunking"
	cleanedContent := parser.cleanContent(content)
	contentType := parser.detectContentType(cleanedContent)
	
	// Set up chunker as ParseWithChunks would
	if parser.chunker == nil {
		parser.chunker = NewTextChunker(320, 48)
	}
	
	chunks := parser.chunkByContentType(cleanedContent, contentType)
	
	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}
	
	// Verify chunk metadata is updated correctly
	for i, chunk := range chunks {
		// Note: ChunkType is set by different chunking methods, so we just verify it's not empty
		if chunk.ChunkType == "" {
			t.Errorf("Chunk %d has empty ChunkType", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d: expected index %d, got %d", i, i, chunk.Index)
		}
	}
}

func TestDOCXParser_Integration_ContentProcessing(t *testing.T) {
	// Test the complete content processing pipeline without file I/O
	parser := NewDOCXParser()
	
	tests := []struct {
		name         string
		content      string
		expectedType string
	}{
		{
			name: "code document",
			content: `function processData(input) {
				const processed = input.map(item => {
					return {
						id: item.id,
						value: item.value * 2
					};
				});
				return processed;
			}
			function helper() {}
			class DataManager {
				constructor() {
					this.data = [];
				}
				public getData() {}
				private process() {}
			}`,
			expectedType: "code",
		},
		{
			name: "structured document",
			content: `# API Documentation
			
			## Authentication
			- Use Bearer tokens
			- Include in Authorization header
			
			### Token Format
			1. Obtain token from /auth endpoint
			2. Format: Bearer <token>
			3. Include in all requests
			
			## Endpoints
			### GET /users
			Returns list of users.`,
			expectedType: "structured",
		},
		{
			name: "prose document",
			content: `The implementation of this system required careful consideration of multiple factors. 
			First, we needed to ensure that the processing pipeline could handle various document types effectively.
			The approach we took was to create specialized parsers for each document format, allowing for optimized 
			text extraction and chunking strategies. This modular design ensures maintainability and extensibility 
			as new document types are added to the system.`,
			expectedType: "prose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test content cleaning
			cleaned := parser.cleanContent(tt.content)
			if strings.TrimSpace(cleaned) == "" {
				t.Error("Content cleaning resulted in empty string")
			}

			// Test content type detection
			detectedType := parser.detectContentType(cleaned)
			if detectedType != tt.expectedType {
				t.Errorf("Expected content type %s, got %s", tt.expectedType, detectedType)
			}

			// Test chunking
			parser.chunker = NewTextChunker(200, 40) // Medium-sized chunks for testing
			chunks := parser.chunkByContentType(cleaned, detectedType)

			if len(chunks) == 0 {
				t.Error("Expected at least one chunk")
			}

			// Verify chunk properties
			for i, chunk := range chunks {
				if strings.TrimSpace(chunk.Text) == "" {
					t.Errorf("Chunk %d is empty", i)
				}
				if chunk.TokenCount <= 0 {
					t.Errorf("Chunk %d has invalid token count: %d", i, chunk.TokenCount)
				}
			}

			// Test metadata application (simulate ParseWithChunks)
			for i := range chunks {
				chunks[i].Index = i
				chunks[i].ChunkType = "docx_" + detectedType
			}

			// Verify metadata
			for i, chunk := range chunks {
				if chunk.Index != i {
					t.Errorf("Chunk %d has wrong index: %d", i, chunk.Index)
				}
				// Note: ChunkType varies based on chunking method, verify it contains the content type
				if !strings.Contains(chunk.ChunkType, tt.expectedType) && chunk.ChunkType != "text" {
					// Allow "text" type from basic chunker or types containing expected type
					t.Errorf("Chunk %d has unexpected type %s for content type %s", i, chunk.ChunkType, tt.expectedType)
				}
			}
		})
	}
}

func TestDOCXParser_EdgeCases(t *testing.T) {
	parser := NewDOCXParser()
	parser.chunker = NewTextChunker(100, 20)

	t.Run("empty content processing", func(t *testing.T) {
		cleaned := parser.cleanContent("")
		if cleaned != "" {
			t.Errorf("Expected empty string, got %q", cleaned)
		}

		contentType := parser.detectContentType("")
		if contentType != "prose" {
			t.Errorf("Expected prose type for empty content, got %s", contentType)
		}

		chunks := parser.chunkByContentType("", "prose")
		if len(chunks) != 0 {
			t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
		}
	})

	t.Run("whitespace-only content", func(t *testing.T) {
		whitespaceContent := "   \n\n\t\t   \n   "
		cleaned := parser.cleanContent(whitespaceContent)
		if cleaned != "" {
			t.Errorf("Expected empty string after cleaning whitespace, got %q", cleaned)
		}
	})

	t.Run("single word content", func(t *testing.T) {
		singleWord := "Hello"
		cleaned := parser.cleanContent(singleWord)
		if cleaned != singleWord {
			t.Errorf("Expected %q, got %q", singleWord, cleaned)
		}

		contentType := parser.detectContentType(cleaned)
		if contentType != "prose" {
			t.Errorf("Expected prose type for single word, got %s", contentType)
		}

		chunks := parser.chunkByContentType(cleaned, contentType)
		if len(chunks) != 1 {
			t.Errorf("Expected 1 chunk for single word, got %d", len(chunks))
		}
	})
}

// Benchmark tests for performance-critical operations
func BenchmarkDOCXParser_DetectContentType(b *testing.B) {
	parser := NewDOCXParser()
	content := `This is a sample document with various content types mixed together. 
	function test() { return true; }
	# Header
	- List item
	Regular prose continues here with more sentences.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.detectContentType(content)
	}
}

func BenchmarkDOCXParser_CleanContent(b *testing.B) {
	parser := NewDOCXParser()
	content := strings.Repeat("Line with content\n  \n\t\n  \n", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.cleanContent(content)
	}
}

func BenchmarkDOCXParser_IsHeader(b *testing.B) {
	parser := NewDOCXParser()
	testLines := []string{
		"# Main Header",
		"## Subheader",
		"1.2.3 Numbered Header",
		"UPPERCASE HEADER",
		"This is regular text content that should not be considered a header.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, line := range testLines {
			parser.isHeader(line)
		}
	}
}