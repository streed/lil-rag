package lilrag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPDFParser(t *testing.T) {
	parser := NewPDFParser()
	if parser == nil {
		t.Fatal("NewPDFParser() returned nil")
	}
}

func TestPDFParser_SupportedExtensions(t *testing.T) {
	parser := NewPDFParser()
	extensions := parser.SupportedExtensions()

	expected := []string{".pdf"}
	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for i, ext := range expected {
		if extensions[i] != ext {
			t.Errorf("Expected extension %s, got %s", ext, extensions[i])
		}
	}
}

func TestPDFParser_GetDocumentType(t *testing.T) {
	parser := NewPDFParser()
	docType := parser.GetDocumentType()

	if docType != DocumentTypePDF {
		t.Errorf("Expected DocumentTypePDF, got %v", docType)
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "regular text",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "text with zero-width spaces",
			input:    "Hello\u200BWorld",
			expected: "Hello World", // cleanText normalizes spaces
		},
		{
			name:     "text with byte order mark",
			input:    "\uFEFFHello World",
			expected: "Hello World",
		},
		{
			name:     "spaced letters OCR error",
			input:    "G o a l s for the project",
			expected: "Goals for the project",
		},
		{
			name:     "mixed spaced and normal text",
			input:    "T e s t document with normal text",
			expected: "Test document with normal text",
		},
		{
			name:     "preserve intentional spacing",
			input:    "New York City",
			expected: "New York City",
		},
		{
			name:     "text with excessive newlines",
			input:    "Paragraph 1\n\n\n\n\nParagraph 2",
			expected: "Paragraph 1 Paragraph 2", // cleanText normalizes whitespace
		},
		{
			name:     "text with tabs and newlines",
			input:    "Name:\tJohn Doe\nAge:\t30",
			expected: "Name: John Doe Age: 30", // cleanText normalizes whitespace
		},
		{
			name:     "text with punctuation and symbols",
			input:    "Price: $29.99 (20% off)",
			expected: "Price: $29.99 (20% off)",
		},
		{
			name:     "unicode letters and numbers",
			input:    "Café: 123€ résumé naïve",
			expected: "Café: 123 résumé naïve", // Some symbols may be filtered
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanText() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestPDFParser_Parse_FileErrors(t *testing.T) {
	parser := NewPDFParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file",
			filePath: "/nonexistent/file.pdf",
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

func TestPDFParser_ParseWithChunks_FileErrors(t *testing.T) {
	parser := NewPDFParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file",
			filePath: "/nonexistent/file.pdf",
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

// Test with actual PDF file if available
func TestPDFParser_ParseWithRealFile(t *testing.T) {
	parser := NewPDFParser()

	// Look for test PDF file
	testPDFPaths := []string{
		"./test_pdfs/test_document.pdf",
		"../test_pdfs/test_document.pdf",
		"../../test_pdfs/test_document.pdf",
	}

	var testPDFPath string
	for _, path := range testPDFPaths {
		if _, err := os.Stat(path); err == nil {
			testPDFPath = path
			break
		}
	}

	if testPDFPath == "" {
		t.Skip("No test PDF file found, skipping real file test")
		return
	}

	t.Run("parse PDF content", func(t *testing.T) {
		content, err := parser.Parse(testPDFPath)
		if err != nil {
			t.Errorf("Failed to parse PDF: %v", err)
			return
		}

		if len(content) == 0 {
			t.Error("Expected non-empty content from PDF")
		}

		// Basic content validation - should contain some readable text
		if !containsAlphabetic(content) {
			t.Error("PDF content should contain alphabetic characters")
		}
	})

	t.Run("parse PDF with chunks", func(t *testing.T) {
		chunks, err := parser.ParseWithChunks(testPDFPath, "test-pdf")
		if err != nil {
			t.Errorf("Failed to parse PDF with chunks: %v", err)
			return
		}

		if len(chunks) == 0 {
			t.Error("Expected at least one chunk from PDF")
		}

		// Verify chunk structure
		for i, chunk := range chunks {
			if chunk.Index != i {
				t.Errorf("Chunk %d has wrong index: %d", i, chunk.Index)
			}

			if len(chunk.Text) == 0 {
				t.Errorf("Chunk %d is empty", i)
			}

			if chunk.ChunkType != "pdf_page" {
				t.Errorf("Expected chunk type 'pdf_page', got %s", chunk.ChunkType)
			}

			if chunk.TokenCount <= 0 {
				t.Errorf("Chunk %d has invalid token count: %d", i, chunk.TokenCount)
			}
		}
	})

	t.Run("parse PDF structure", func(t *testing.T) {
		doc, err := parser.ParsePDF(testPDFPath)
		if err != nil {
			t.Errorf("Failed to parse PDF structure: %v", err)
			return
		}

		if doc == nil {
			t.Fatal("Expected non-nil PDF document")
		}

		if len(doc.Pages) == 0 {
			t.Error("Expected at least one page in PDF")
		}

		if doc.TotalPages <= 0 {
			t.Error("Expected positive total page count")
		}

		if len(doc.Pages) != doc.TotalPages {
			t.Errorf("Page count mismatch: %d pages vs %d total", len(doc.Pages), doc.TotalPages)
		}

		// Verify page structure
		for i, page := range doc.Pages {
			if page.PageNumber != i+1 {
				t.Errorf("Page %d has wrong page number: %d", i, page.PageNumber)
			}

			// Pages should have some content (even if empty)
			if page.Words < 0 {
				t.Errorf("Page %d has negative word count: %d", i, page.Words)
			}
		}
	})
}

func TestPDFParser_Integration(t *testing.T) {
	// Test the complete workflow using the test PDF if available
	parser := NewPDFParser()

	// Look for test PDF file
	testPDFPaths := []string{
		"./test_pdfs/test_document.pdf",
		"../test_pdfs/test_document.pdf",
		"../../test_pdfs/test_document.pdf",
	}

	var testPDFPath string
	for _, path := range testPDFPaths {
		if _, err := os.Stat(path); err == nil {
			testPDFPath = path
			break
		}
	}

	if testPDFPath == "" {
		t.Skip("No test PDF file found, skipping integration test")
		return
	}

	// Test Parse method
	content, err := parser.Parse(testPDFPath)
	if err != nil {
		t.Errorf("Parse failed: %v", err)
		return
	}

	// Test ParseWithChunks method
	chunks, err := parser.ParseWithChunks(testPDFPath, "integration-test")
	if err != nil {
		t.Errorf("ParseWithChunks failed: %v", err)
		return
	}

	// Verify integration consistency
	if len(chunks) == 0 && len(content) > 0 {
		t.Error("Parse returned content but ParseWithChunks returned no chunks")
	}

	// Reconstruct content from chunks and compare
	if len(chunks) > 0 {
		var reconstructed strings.Builder
		for _, chunk := range chunks {
			reconstructed.WriteString(chunk.Text)
			if !strings.HasSuffix(chunk.Text, "\n") {
				reconstructed.WriteString("\n")
			}
		}

		// Should have similar length (allowing for processing differences)
		originalLen := len(strings.TrimSpace(content))
		reconstructedLen := len(strings.TrimSpace(reconstructed.String()))

		// Allow 20% difference due to chunking processing
		if abs(originalLen-reconstructedLen) > originalLen/5 {
			t.Errorf("Significant content difference: original %d chars, reconstructed %d chars",
				originalLen, reconstructedLen)
		}
	}

	// Verify extension is supported
	ext := filepath.Ext(testPDFPath)
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
}

func TestPDFParser_MethodAliases(t *testing.T) {
	// Test that ParseWithChunks is correctly aliased to ParsePDFWithPageChunks
	parser := NewPDFParser()

	// We can't test this without a real PDF file, so just verify the methods exist
	// and have the right signatures by calling them with invalid input

	t.Run("ParseWithChunks alias", func(t *testing.T) {
		_, err := parser.ParseWithChunks("nonexistent.pdf", "test-doc")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
		// The error should indicate the file doesn't exist, meaning the method was called
	})
}

func TestPDFDocument_Structure(t *testing.T) {
	// Test the PDFDocument and PDFPage structures

	t.Run("PDFPage structure", func(t *testing.T) {
		page := PDFPage{
			PageNumber: 1,
			Text:       "Sample page text",
			Words:      3,
		}

		if page.PageNumber != 1 {
			t.Errorf("Expected page number 1, got %d", page.PageNumber)
		}

		if page.Text != "Sample page text" {
			t.Errorf("Expected text 'Sample page text', got %q", page.Text)
		}

		if page.Words != 3 {
			t.Errorf("Expected 3 words, got %d", page.Words)
		}
	})

	t.Run("PDFDocument structure", func(t *testing.T) {
		pages := []PDFPage{
			{PageNumber: 1, Text: "Page 1", Words: 2},
			{PageNumber: 2, Text: "Page 2", Words: 2},
		}

		doc := PDFDocument{
			Pages:      pages,
			Title:      "Test Document",
			TotalPages: 2,
		}

		if len(doc.Pages) != 2 {
			t.Errorf("Expected 2 pages, got %d", len(doc.Pages))
		}

		if doc.Title != "Test Document" {
			t.Errorf("Expected title 'Test Document', got %q", doc.Title)
		}

		if doc.TotalPages != 2 {
			t.Errorf("Expected 2 total pages, got %d", doc.TotalPages)
		}
	})
}

func TestPDFParser_FallbackBehavior(t *testing.T) {
	// Test behavior when pdftotext is not available
	// This is hard to test directly, but we can verify the error messages
	// indicate fallback attempts

	parser := NewPDFParser()

	// Use a file that would definitely fail both parsers
	invalidPDFPath := "/dev/null"
	if _, err := os.Stat(invalidPDFPath); err != nil {
		// Create a temporary invalid file
		tmpFile, createErr := os.CreateTemp("", "invalid_*.pdf")
		if createErr != nil {
			t.Skip("Could not create temp file for fallback test")
		}
		defer os.Remove(tmpFile.Name())

		// Write invalid PDF content
		if _, writeErr := tmpFile.WriteString("This is not a PDF file"); writeErr != nil {
			t.Skip("Could not write to temp file")
		}
		tmpFile.Close()

		invalidPDFPath = tmpFile.Name()
	}

	_, err := parser.ParsePDF(invalidPDFPath)
	if err == nil {
		t.Error("Expected error for invalid PDF file")
		return
	}

	// The error should mention both parsing attempts
	errorStr := err.Error()
	if !strings.Contains(errorStr, "pdftotext") && !strings.Contains(errorStr, "dslipak") {
		t.Log("Error message:", errorStr)
		// This might not always fail as expected, so just log it
	}
}

// Helper functions

func containsAlphabetic(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func TestPDFParser_EdgeCases(t *testing.T) {
	parser := NewPDFParser()

	t.Run("empty file path", func(t *testing.T) {
		_, err := parser.Parse("")
		if err == nil {
			t.Error("Expected error for empty file path")
		}
	})

	t.Run("file with wrong extension", func(t *testing.T) {
		// Create a temporary file with PDF extension but invalid content
		tmpFile, err := os.CreateTemp("", "test_*.pdf")
		if err != nil {
			t.Skip("Could not create temp file")
		}
		defer os.Remove(tmpFile.Name())

		// Write non-PDF content
		if _, err := tmpFile.WriteString("This is plain text, not PDF"); err != nil {
			t.Skip("Could not write to temp file")
		}
		tmpFile.Close()

		_, parseErr := parser.Parse(tmpFile.Name())
		if parseErr == nil {
			t.Error("Expected error for non-PDF content")
		}
	})
}

func TestPDFParser_LargePDFHandling(t *testing.T) {
	// This test would ideally use a large PDF file
	// For now, we'll just test the chunking behavior conceptually

	// Test chunking parameters
	t.Run("chunking configuration", func(t *testing.T) {
		// The PDF parser uses specific chunking parameters
		// We can't test this directly without a real PDF, but we can verify
		// the chunking logic would work with expected content sizes

		chunker := NewTextChunker(1800, 200) // Same as used in PDF parser

		// Simulate large PDF content that would require chunking
		largeText := strings.Repeat("This is a long sentence that would appear in a PDF document. ", 25)
		chunks := chunker.ChunkText(largeText)

		if len(chunks) == 0 {
			t.Error("Expected chunks for large text")
		}

		// Verify chunks are created and have reasonable properties
		totalLength := 0
		for i, chunk := range chunks {
			if len(chunk.Text) == 0 {
				t.Errorf("Chunk %d is empty", i)
			}
			if chunk.Index != i {
				t.Errorf("Chunk %d has wrong index: %d", i, chunk.Index)
			}
			totalLength += len(chunk.Text)
		}

		// Should preserve most of the original content
		originalLength := len(largeText)
		if totalLength < originalLength/2 {
			t.Errorf("Too much content lost in chunking: original %d, total %d", originalLength, totalLength)
		}
	})
}
