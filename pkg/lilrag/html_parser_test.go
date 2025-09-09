package lilrag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHTMLParser(t *testing.T) {
	parser := NewHTMLParser()
	if parser == nil {
		t.Fatal("NewHTMLParser() returned nil")
	}
	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}
}

func TestHTMLParser_SupportedExtensions(t *testing.T) {
	parser := NewHTMLParser()
	extensions := parser.SupportedExtensions()

	expected := []string{".html", ".htm"}
	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for i, ext := range expected {
		if extensions[i] != ext {
			t.Errorf("Expected extension %s, got %s", ext, extensions[i])
		}
	}
}

func TestHTMLParser_GetDocumentType(t *testing.T) {
	parser := NewHTMLParser()
	docType := parser.GetDocumentType()

	if docType != DocumentTypeHTML {
		t.Errorf("Expected DocumentTypeHTML, got %v", docType)
	}
}

func createTestHTMLFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test_*.html")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write HTML content: %v", err)
	}

	return tmpFile.Name()
}

func TestHTMLParser_Parse(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name           string
		htmlContent    string
		expectContains []string
		expectError    bool
	}{
		{
			name: "simple HTML",
			htmlContent: `<!DOCTYPE html>
<html>
<head><title>Test Document</title></head>
<body>
<h1>Main Heading</h1>
<p>This is a paragraph.</p>
</body>
</html>`,
			expectContains: []string{
				"Title: Test Document",
				"Main Heading",
				"This is a paragraph",
			},
		},
		{
			name: "HTML with lists",
			htmlContent: `<html>
<body>
<h2>Features</h2>
<ul>
<li>First item</li>
<li>Second item</li>
</ul>
</body>
</html>`,
			expectContains: []string{
				"Features",
				"First item",
				"Second item",
			},
		},
		{
			name: "HTML with script and style tags",
			htmlContent: `<html>
<head>
<style>body { color: red; }</style>
<script>console.log('test');</script>
</head>
<body>
<p>Visible content</p>
</body>
</html>`,
			expectContains: []string{
				"Visible content",
			},
		},
		{
			name: "HTML with nested elements",
			htmlContent: `<html>
<body>
<div>
  <article>
    <h3>Article Title</h3>
    <p>Article content with <strong>bold text</strong>.</p>
  </article>
</div>
</body>
</html>`,
			expectContains: []string{
				"Article Title",
				"Article content",
				"bold text",
			},
		},
		{
			name:           "empty HTML",
			htmlContent:    `<html><body></body></html>`,
			expectContains: []string{},
		},
		{
			name:           "HTML with only whitespace",
			htmlContent:    `<html><body>   \n   </body></html>`,
			expectContains: []string{},
		},
		{
			name:        "malformed HTML",
			htmlContent: `<html><body><p>Unclosed paragraph<div>Content</div></body></html>`,
			expectContains: []string{
				"Unclosed paragraph",
				"Content",
			},
		},
		{
			name:        "HTML with special characters",
			htmlContent: `<html><body><p>Special chars: &lt; &gt; &amp; &quot;</p></body></html>`,
			expectContains: []string{
				"Special chars",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createTestHTMLFile(t, tt.htmlContent)
			defer os.Remove(filePath)

			result, err := parser.Parse(filePath)

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

			// Check that all expected content appears
			for _, expected := range tt.expectContains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected content not found: %q\nFull result:\n%s", expected, result)
				}
			}
		})
	}
}

func TestHTMLParser_Parse_FileErrors(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file",
			filePath: "/nonexistent/file.html",
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

func TestHTMLParser_ParseWithChunks(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name           string
		htmlContent    string
		expectedChunks int
		checkTitle     bool
		checkSections  bool
	}{
		{
			name: "simple HTML with title",
			htmlContent: `<!DOCTYPE html>
<html>
<head><title>Test Document</title></head>
<body>
<p>Simple content</p>
</body>
</html>`,
			expectedChunks: 2, // title + content
			checkTitle:     true,
		},
		{
			name: "HTML with sections",
			htmlContent: `<html>
<body>
<h1>First Section</h1>
<p>Content for first section</p>
<h2>Second Section</h2>
<p>Content for second section</p>
</body>
</html>`,
			expectedChunks: 2, // Two sections
			checkSections:  true,
		},
		{
			name: "HTML with lists and paragraphs",
			htmlContent: `<html>
<body>
<h3>Features</h3>
<ul>
<li>Feature one</li>
<li>Feature two</li>
</ul>
<p>Additional content</p>
</body>
</html>`,
			expectedChunks: 1, // One section
			checkSections:  true,
		},
		{
			name:           "empty HTML",
			htmlContent:    `<html><body></body></html>`,
			expectedChunks: 0,
		},
		{
			name: "HTML without structured sections",
			htmlContent: `<html>
<body>
<p>Just plain paragraphs</p>
<div>Some div content</div>
</body>
</html>`,
			expectedChunks: 1, // Fallback to content chunking
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createTestHTMLFile(t, tt.htmlContent)
			defer os.Remove(filePath)

			chunks, err := parser.ParseWithChunks(filePath, "test-doc")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(chunks) != tt.expectedChunks {
				t.Errorf("Expected %d chunks, got %d", tt.expectedChunks, len(chunks))
			}

			// Verify chunk structure
			for i, chunk := range chunks {
				if chunk.Index != i {
					t.Errorf("Chunk %d has wrong index: %d", i, chunk.Index)
				}

				if len(chunk.Text) == 0 {
					t.Errorf("Chunk %d is empty", i)
				}

				if chunk.TokenCount <= 0 {
					t.Errorf("Chunk %d has invalid token count: %d", i, chunk.TokenCount)
				}
			}

			// Check title chunk
			if tt.checkTitle && len(chunks) > 0 {
				titleChunk := chunks[0]
				if titleChunk.ChunkType != "html_title" {
					t.Errorf("Expected first chunk to be html_title, got %s", titleChunk.ChunkType)
				}
				if !strings.Contains(titleChunk.Text, "Document Title:") {
					t.Error("Title chunk doesn't contain expected title text")
				}
			}

			// Check section chunks
			if tt.checkSections && len(chunks) > 0 {
				foundSection := false
				for _, chunk := range chunks {
					if chunk.ChunkType == "html_section" {
						foundSection = true
						break
					}
				}
				if !foundSection {
					t.Error("Expected to find html_section chunk type")
				}
			}
		})
	}
}

func TestHTMLParser_ParseWithChunks_CustomChunker(t *testing.T) {
	// Test that chunker is initialized automatically
	parser := NewHTMLParser()

	htmlContent := `<html><body><p>Test content for chunking.</p></body></html>`
	filePath := createTestHTMLFile(t, htmlContent)
	defer os.Remove(filePath)

	// First call should create chunker
	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}

	chunks, err := parser.ParseWithChunks(filePath, "test-doc")
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

func TestHTMLParser_ParseWithChunks_FileErrors(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "nonexistent file",
			filePath: "/nonexistent/file.html",
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

func TestHTMLParser_ExtractTitle(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name          string
		htmlContent   string
		expectedTitle string
	}{
		{
			name:          "HTML with title",
			htmlContent:   `<html><head><title>Test Title</title></head></html>`,
			expectedTitle: "Test Title",
		},
		{
			name:          "HTML without title",
			htmlContent:   `<html><head></head></html>`,
			expectedTitle: "",
		},
		{
			name:          "HTML with empty title",
			htmlContent:   `<html><head><title></title></head></html>`,
			expectedTitle: "",
		},
		{
			name:          "HTML with title containing whitespace",
			htmlContent:   `<html><head><title>  Spaced Title  </title></head></html>`,
			expectedTitle: "Spaced Title", // cleanWhitespace trims whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createTestHTMLFile(t, tt.htmlContent)
			defer os.Remove(filePath)

			result, err := parser.Parse(filePath)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectedTitle == "" {
				if strings.Contains(result, "Title:") {
					t.Error("Expected no title, but found title in result")
				}
			} else {
				expectedContent := "Title: " + tt.expectedTitle
				if !strings.Contains(result, expectedContent) {
					t.Errorf("Expected title %q not found in result", expectedContent)
				}
			}
		})
	}
}

func TestHTMLParser_CleanWhitespace(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multiple spaces",
			input:    "Hello    world",
			expected: "Hello world",
		},
		{
			name:     "multiple newlines",
			input:    "Para1\n\n\n\nPara2",
			expected: "Para1 Para2", // cleanWhitespace converts all whitespace to single spaces
		},
		{
			name:     "mixed whitespace",
			input:    "Text  with\t\tmixed   \n\n\n  whitespace",
			expected: "Text with mixed whitespace", // cleanWhitespace converts all whitespace to single spaces
		},
		{
			name:     "leading and trailing whitespace",
			input:    "  \n  Content  \n  ",
			expected: "Content",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\n\t  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.cleanWhitespace(tt.input)
			if result != tt.expected {
				t.Errorf("cleanWhitespace() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLParser_ExtractListText(t *testing.T) {
	parser := NewHTMLParser()

	htmlContent := `<html>
<body>
<ul>
<li>First item</li>
<li>Second item</li>
<li>Third item</li>
</ul>
<ol>
<li>Ordered first</li>
<li>Ordered second</li>
</ol>
</body>
</html>`

	filePath := createTestHTMLFile(t, htmlContent)
	defer os.Remove(filePath)

	result, err := parser.Parse(filePath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	// Should contain bullet points for list items
	expectedItems := []string{
		"First item",
		"Second item",
		"Third item",
		"Ordered first",
		"Ordered second",
	}

	for _, item := range expectedItems {
		if !strings.Contains(result, item) {
			t.Errorf("Expected list item %q not found in result", item)
		}
	}
}

func TestHTMLParser_Integration(t *testing.T) {
	// Test a complete workflow with realistic HTML content
	parser := NewHTMLParser()

	htmlContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Sample Document</title>
    <style>
        body { font-family: Arial, sans-serif; }
        .highlight { background-color: yellow; }
    </style>
</head>
<body>
    <header>
        <h1>Main Article</h1>
    </header>
    
    <main>
        <section>
            <h2>Introduction</h2>
            <p>This is the introduction paragraph with some <em>emphasized text</em>.</p>
        </section>
        
        <section>
            <h2>Features</h2>
            <ul>
                <li>Feature A: Does something useful</li>
                <li>Feature B: Does something else</li>
                <li>Feature C: Additional functionality</li>
            </ul>
        </section>
        
        <section>
            <h2>Conclusion</h2>
            <p>This concludes our document.</p>
        </section>
    </main>
    
    <script>
        // This script should be ignored
        console.log("Should not appear in text");
    </script>
</body>
</html>`

	filePath := createTestHTMLFile(t, htmlContent)
	defer os.Remove(filePath)

	// Test Parse method
	content, err := parser.Parse(filePath)
	if err != nil {
		t.Errorf("Parse failed: %v", err)
		return
	}

	// Verify expected content is present
	expectedElements := []string{
		"Title: Sample Document",
		"Main Article",
		"Introduction",
		"emphasized text",
		"Features",
		"Feature A",
		"Conclusion",
	}

	for _, element := range expectedElements {
		if !strings.Contains(content, element) {
			t.Errorf("Expected element not found: %q", element)
		}
	}

	// Script content should not be present
	if strings.Contains(content, "console.log") {
		t.Error("Script content should not be included in parsed text")
	}

	// Test ParseWithChunks method
	chunks, err := parser.ParseWithChunks(filePath, "integration-test")
	if err != nil {
		t.Errorf("ParseWithChunks failed: %v", err)
		return
	}

	if len(chunks) < 2 {
		t.Error("Expected multiple chunks for structured HTML")
	}

	// Should have title chunk
	titleFound := false
	sectionFound := false
	for _, chunk := range chunks {
		if chunk.ChunkType == "html_title" {
			titleFound = true
		}
		if chunk.ChunkType == "html_section" {
			sectionFound = true
		}
	}

	if !titleFound {
		t.Error("Expected to find html_title chunk")
	}

	if !sectionFound {
		t.Error("Expected to find html_section chunk")
	}

	// Verify extension is supported
	ext := filepath.Ext(filePath)
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

func TestHTMLParser_EdgeCases(t *testing.T) {
	parser := NewHTMLParser()

	tests := []struct {
		name        string
		htmlContent string
		shouldWork  bool
	}{
		{
			name:        "completely empty file",
			htmlContent: "",
			shouldWork:  true, // HTML parser handles empty files gracefully
		},
		{
			name:        "invalid HTML structure",
			htmlContent: "<html><body><p>Unclosed tags<div>Content</body></html>",
			shouldWork:  true, // HTML parser is forgiving
		},
		{
			name:        "only text, no HTML",
			htmlContent: "Just plain text content",
			shouldWork:  true,
		},
		{
			name:        "nested lists",
			htmlContent: `<ul><li>Item 1<ul><li>Nested item</li></ul></li><li>Item 2</li></ul>`,
			shouldWork:  true,
		},
		{
			name:        "HTML with comments",
			htmlContent: `<html><!-- This is a comment --><body><p>Content</p></body></html>`,
			shouldWork:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createTestHTMLFile(t, tt.htmlContent)
			defer os.Remove(filePath)

			_, err := parser.Parse(filePath)

			if tt.shouldWork && err != nil {
				t.Errorf("Expected parsing to work, but got error: %v", err)
			}

			if !tt.shouldWork && err == nil {
				t.Error("Expected parsing to fail, but it succeeded")
			}
		})
	}
}

func TestHTMLParser_LargeDocument(t *testing.T) {
	parser := NewHTMLParser()

	// Create a large HTML document
	var htmlBuilder strings.Builder
	htmlBuilder.WriteString(`<!DOCTYPE html><html><head><title>Large Document</title></head><body>`)

	// Add many sections
	for i := 1; i <= 50; i++ {
		htmlBuilder.WriteString(`<h2>Section `)
		htmlBuilder.WriteString(string(rune('0' + i%10)))
		htmlBuilder.WriteString(`</h2><p>This is content for section `)
		htmlBuilder.WriteString(string(rune('0' + i%10)))
		htmlBuilder.WriteString(`. It contains important information about this topic.</p>`)
	}

	htmlBuilder.WriteString(`</body></html>`)

	filePath := createTestHTMLFile(t, htmlBuilder.String())
	defer os.Remove(filePath)

	// Test parsing
	content, err := parser.Parse(filePath)
	if err != nil {
		t.Errorf("Failed to parse large HTML: %v", err)
		return
	}

	if len(content) == 0 {
		t.Error("Large HTML produced no content")
	}

	// Test chunking
	chunks, err := parser.ParseWithChunks(filePath, "large-html")
	if err != nil {
		t.Errorf("Failed to chunk large HTML: %v", err)
		return
	}

	if len(chunks) < 10 {
		t.Error("Expected multiple chunks for large HTML document")
	}

	// Verify all chunks have content and proper structure
	for i, chunk := range chunks {
		if len(chunk.Text) == 0 {
			t.Errorf("Chunk %d is empty", i)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d has incorrect index %d", i, chunk.Index)
		}
	}
}
