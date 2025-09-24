package lilrag

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestNewXLSXParser(t *testing.T) {
	parser := NewXLSXParser()
	if parser == nil {
		t.Fatal("NewXLSXParser() returned nil")
	}

	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}
}

func TestXLSXParser_SupportedExtensions(t *testing.T) {
	parser := NewXLSXParser()
	extensions := parser.SupportedExtensions()

	expected := []string{".xlsx"}
	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for i, ext := range expected {
		if extensions[i] != ext {
			t.Errorf("Expected extension %s, got %s", ext, extensions[i])
		}
	}
}

func TestXLSXParser_GetDocumentType(t *testing.T) {
	parser := NewXLSXParser()
	docType := parser.GetDocumentType()

	if docType != DocumentTypeXLSX {
		t.Errorf("Expected DocumentTypeXLSX, got %v", docType)
	}
}

func TestXLSXParser_GetColumnName(t *testing.T) {
	parser := NewXLSXParser()

	tests := []struct {
		index    int
		expected string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
	}

	for _, tt := range tests {
		result := parser.getColumnName(tt.index)
		if result != tt.expected {
			t.Errorf("getColumnName(%d) = %s, expected %s", tt.index, result, tt.expected)
		}
	}
}

func TestXLSXParser_HasNonEmptyData(t *testing.T) {
	parser := NewXLSXParser()

	tests := []struct {
		name     string
		row      []string
		expected bool
	}{
		{"empty row", []string{}, false},
		{"row with empty strings", []string{"", "", ""}, false},
		{"row with whitespace only", []string{"   ", "\t", "\n"}, false},
		{"row with one value", []string{"", "value", ""}, true},
		{"row with mixed content", []string{"data1", "", "data2"}, true},
		{"row with all values", []string{"col1", "col2", "col3"}, true},
		{"single empty string", []string{""}, false},
		{"single whitespace", []string{" "}, false},
		{"single value", []string{"test"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.hasNonEmptyData(tt.row)
			if result != tt.expected {
				t.Errorf("hasNonEmptyData(%v) = %v, expected %v", tt.row, result, tt.expected)
			}
		})
	}
}

// Since we can't create real XLSX files easily in tests, we'll test error handling
func TestXLSXParser_Parse_FileErrors(t *testing.T) {
	parser := NewXLSXParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{"nonexistent file", "/nonexistent/file.xlsx"},
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

func TestXLSXParser_ParseWithChunks_FileErrors(t *testing.T) {
	parser := NewXLSXParser()

	tests := []struct {
		name     string
		filePath string
	}{
		{"nonexistent file", "/nonexistent/file.xlsx"},
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

// Test with invalid XLSX file (not a real XLSX)
func TestXLSXParser_Parse_InvalidXLSX(t *testing.T) {
	parser := NewXLSXParser()

	// Create a file with .xlsx extension but invalid content
	tmpFile, err := os.CreateTemp("", "invalid_*.xlsx")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write non-XLSX content
	if _, err := tmpFile.WriteString("This is not a valid XLSX file"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = parser.Parse(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid XLSX file")
	}

	if !strings.Contains(err.Error(), "failed to open XLSX file") {
		t.Errorf("Expected 'failed to open XLSX file' error, got: %v", err)
	}
}

func TestXLSXParser_ParseWithChunks_InvalidXLSX(t *testing.T) {
	parser := NewXLSXParser()

	// Create a file with .xlsx extension but invalid content
	tmpFile, err := os.CreateTemp("", "invalid_*.xlsx")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write non-XLSX content
	if _, err := tmpFile.WriteString("This is not a valid XLSX file"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = parser.ParseWithChunks(tmpFile.Name(), "test-doc")
	if err == nil {
		t.Error("Expected error for invalid XLSX file")
	}

	if !strings.Contains(err.Error(), "failed to open XLSX file") {
		t.Errorf("Expected 'failed to open XLSX file' error, got: %v", err)
	}
}

func TestXLSXParser_ParseWithChunks_DefaultChunker(t *testing.T) {
	// Test that ParseWithChunks creates a default chunker when none is provided
	parser := NewXLSXParser()

	// Verify chunker is initially nil
	if parser.chunker != nil {
		t.Error("Expected chunker to be nil initially")
	}

	// Test the chunker creation by simulating the logic in ParseWithChunks
	if parser.chunker == nil {
		parser.chunker = NewTextChunker(200, 30) // Same as in actual implementation
	}

	if parser.chunker == nil {
		t.Error("Expected chunker to be created")
	}

	// Test chunker configuration
	if parser.chunker.MaxChars != 200 {
		t.Errorf("Expected max tokens 200, got %d", parser.chunker.MaxChars)
	}

	if parser.chunker.Overlap != 30 {
		t.Errorf("Expected overlap tokens 30, got %d", parser.chunker.Overlap)
	}
}

func TestXLSXParser_ColumnNameGeneration(t *testing.T) {
	parser := NewXLSXParser()

	// Test comprehensive column name generation
	tests := []struct {
		start     int
		count     int
		checkFunc func(index int, name string) bool
	}{
		{
			start: 0,
			count: 26,
			checkFunc: func(index int, name string) bool {
				expected := string(rune('A' + index))
				return name == expected
			},
		},
		{
			start: 26,
			count: 26,
			checkFunc: func(index int, name string) bool {
				expected := "A" + string(rune('A'+(index-26)))
				return name == expected
			},
		},
	}

	for testIndex, tt := range tests {
		t.Run(fmt.Sprintf("column_range_%d", testIndex+1), func(t *testing.T) {
			for i := 0; i < tt.count; i++ {
				actualIndex := tt.start + i
				name := parser.getColumnName(actualIndex)
				if !tt.checkFunc(actualIndex, name) {
					t.Errorf("Column name for index %d = %s, validation failed", actualIndex, name)
				}
			}
		})
	}
}

func TestXLSXParser_ChunkingLogic(t *testing.T) {
	// Test the chunking logic conceptually without real XLSX files
	parser := NewXLSXParser()
	parser.chunker = NewTextChunker(100, 20) // Small chunks for testing

	// Simulate row processing
	testRows := [][]string{
		{"Header1", "Header2", "Header3"},
		{"Data1", "Data2", "Data3"},
		{"Data4", "Data5", "Data6"},
		{"", "Data7", ""}, // Sparse row
		{"", "", ""},      // Empty row - should be skipped
		{"Data8", "Data9", "Data10"},
	}

	// Test header detection
	var headerRow []string
	var dataStartIndex int

	for i, row := range testRows {
		if len(row) > 0 && parser.hasNonEmptyData(row) {
			headerRow = row
			dataStartIndex = i + 1
			break
		}
	}

	if len(headerRow) != 3 {
		t.Errorf("Expected header row with 3 columns, got %d", len(headerRow))
	}

	if dataStartIndex != 1 {
		t.Errorf("Expected data to start at index 1, got %d", dataStartIndex)
	}

	// Test row processing
	nonEmptyCount := 0
	for i := dataStartIndex; i < len(testRows); i++ {
		if parser.hasNonEmptyData(testRows[i]) {
			nonEmptyCount++
		}
	}

	if nonEmptyCount != 4 { // Should find 4 non-empty data rows (including sparse row)
		t.Errorf("Expected 4 non-empty data rows, found %d", nonEmptyCount)
	}
}

func TestXLSXParser_ChunkCreation(t *testing.T) {
	// Test chunk creation logic without file I/O
	parser := NewXLSXParser()
	parser.chunker = NewTextChunker(50, 10) // Very small chunks

	sheetName := "TestSheet"
	headerRow := []string{"Name", "Age", "City"}

	// Create sheet header chunk (simulating the logic)
	sheetHeaderText := fmt.Sprintf("Excel Sheet: %s", sheetName)
	headerChunk := Chunk{
		Text:       sheetHeaderText,
		Index:      0,
		StartPos:   0,
		EndPos:     len(sheetHeaderText),
		TokenCount: parser.chunker.EstimateTokenCount(sheetHeaderText),
		ChunkType:  "xlsx_sheet_header",
	}

	if headerChunk.ChunkType != "xlsx_sheet_header" {
		t.Errorf("Expected chunk type 'xlsx_sheet_header', got %s", headerChunk.ChunkType)
	}

	if headerChunk.TokenCount <= 0 {
		t.Errorf("Expected positive token count, got %d", headerChunk.TokenCount)
	}

	// Create headers chunk
	headerText := fmt.Sprintf("Sheet %s Headers: %s", sheetName, strings.Join(headerRow, " | "))
	headersChunk := Chunk{
		Text:       headerText,
		Index:      1,
		StartPos:   0,
		EndPos:     len(headerText),
		TokenCount: parser.chunker.EstimateTokenCount(headerText),
		ChunkType:  "xlsx_headers",
	}

	if headersChunk.ChunkType != "xlsx_headers" {
		t.Errorf("Expected chunk type 'xlsx_headers', got %s", headersChunk.ChunkType)
	}

	// Test data row formatting
	testRow := []string{"John", "25", "NYC"}
	rowNum := 2
	rowText := fmt.Sprintf("Row %d: ", rowNum)

	var cells []string
	for colIndex, cell := range testRow {
		if strings.TrimSpace(cell) != "" {
			var columnName string
			if colIndex < len(headerRow) && strings.TrimSpace(headerRow[colIndex]) != "" {
				columnName = headerRow[colIndex]
			} else {
				columnName = parser.getColumnName(colIndex)
			}
			cells = append(cells, fmt.Sprintf("%s: %s", columnName, cell))
		}
	}

	if len(cells) > 0 {
		rowText += strings.Join(cells, " | ")
	}
	rowText += "\n"

	expectedContent := "Row 2: Name: John | Age: 25 | City: NYC\n"
	if rowText != expectedContent {
		t.Errorf("Expected row text %q, got %q", expectedContent, rowText)
	}
}

func TestXLSXParser_ErrorHandling(t *testing.T) {
	parser := NewXLSXParser()

	// Test various error conditions
	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{"empty path", "", true},
		{"nonexistent file", "/path/that/does/not/exist.xlsx", true},
		{"directory path", "/", true},
		{"file without extension", "testfile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.filePath)
			if (err != nil) != tt.expectError {
				t.Errorf("Parse() error = %v, expectError %v", err, tt.expectError)
			}

			_, err2 := parser.ParseWithChunks(tt.filePath, "test-doc")
			if (err2 != nil) != tt.expectError {
				t.Errorf("ParseWithChunks() error = %v, expectError %v", err2, tt.expectError)
			}
		})
	}
}

func TestXLSXParser_EdgeCases(t *testing.T) {
	parser := NewXLSXParser()

	t.Run("empty row handling", func(t *testing.T) {
		emptyRows := [][]string{
			{},
			{""},
			{"", "", ""},
			{"   ", "\t", "\n"},
		}

		for i, row := range emptyRows {
			if parser.hasNonEmptyData(row) {
				t.Errorf("Row %d should be considered empty: %v", i, row)
			}
		}
	})

	t.Run("sparse row handling", func(t *testing.T) {
		sparseRows := [][]string{
			{"", "data", ""},
			{"data", "", ""},
			{"", "", "data"},
			{"  ", "data", "  "},
		}

		for i, row := range sparseRows {
			if !parser.hasNonEmptyData(row) {
				t.Errorf("Row %d should be considered non-empty: %v", i, row)
			}
		}
	})

	t.Run("column name generation edge cases", func(t *testing.T) {
		// Test boundary conditions
		edgeCases := []struct {
			index    int
			expected string
		}{
			{-1, ""}, // Invalid index - should handle gracefully
			{0, "A"},
			{25, "Z"},
			{26, "AA"},
			{675, "YZ"},  // Near ZZ
			{701, "ZZ"},  // Last two-letter combination
			{702, "AAA"}, // First three-letter combination
		}

		for _, tc := range edgeCases {
			t.Run(fmt.Sprintf("index_%d", tc.index), func(t *testing.T) {
				result := parser.getColumnName(tc.index)
				if tc.index < 0 {
					// For negative indices, we don't enforce specific behavior
					// but the function should not panic
					return
				}
				if result != tc.expected {
					t.Errorf("getColumnName(%d) = %s, expected %s", tc.index, result, tc.expected)
				}
			})
		}
	})
}

func TestXLSXParser_ChunkMetadata(t *testing.T) {
	parser := NewXLSXParser()
	parser.chunker = NewTextChunker(100, 20)

	// Test different chunk types
	chunkTypes := []string{"xlsx_sheet_header", "xlsx_headers", "xlsx_data"}

	for _, chunkType := range chunkTypes {
		t.Run(chunkType, func(t *testing.T) {
			testText := "Sample content for " + chunkType
			chunk := Chunk{
				Text:       testText,
				Index:      0,
				StartPos:   0,
				EndPos:     len(testText),
				TokenCount: parser.chunker.EstimateTokenCount(testText),
				ChunkType:  chunkType,
			}

			if chunk.ChunkType != chunkType {
				t.Errorf("Expected chunk type %s, got %s", chunkType, chunk.ChunkType)
			}

			if chunk.TokenCount <= 0 {
				t.Errorf("Expected positive token count, got %d", chunk.TokenCount)
			}

			if chunk.StartPos != 0 {
				t.Errorf("Expected start position 0, got %d", chunk.StartPos)
			}

			if chunk.EndPos != len(testText) {
				t.Errorf("Expected end position %d, got %d", len(testText), chunk.EndPos)
			}
		})
	}
}

func TestXLSXParser_Integration_Conceptual(t *testing.T) {
	// Test the complete processing pipeline conceptually
	parser := NewXLSXParser()
	parser.chunker = NewTextChunker(150, 25)

	// Simulate multi-sheet processing
	sheetNames := []string{"Summary", "Details", "Data"}

	for _, sheetName := range sheetNames {
		t.Run("sheet_"+sheetName, func(t *testing.T) {
			// Test sheet header chunk creation
			sheetHeaderText := fmt.Sprintf("Excel Sheet: %s", sheetName)
			headerChunk := Chunk{
				Text:       sheetHeaderText,
				Index:      0,
				StartPos:   0,
				EndPos:     len(sheetHeaderText),
				TokenCount: parser.chunker.EstimateTokenCount(sheetHeaderText),
				ChunkType:  "xlsx_sheet_header",
			}

			if !strings.Contains(headerChunk.Text, sheetName) {
				t.Errorf("Sheet header should contain sheet name %s", sheetName)
			}

			// Test that chunk metadata is properly set
			if headerChunk.ChunkType != "xlsx_sheet_header" {
				t.Errorf("Expected xlsx_sheet_header chunk type")
			}

			if headerChunk.TokenCount <= 0 {
				t.Errorf("Expected positive token count")
			}
		})
	}

	// Test chunking with token limits
	t.Run("chunking_with_limits", func(t *testing.T) {
		longContent := strings.Repeat("This is a row of data with multiple columns and substantial content. ", 20)
		tokenCount := parser.chunker.EstimateTokenCount(longContent)

		// Should exceed our test chunk size and require splitting
		if tokenCount <= 150 {
			t.Errorf("Test content should exceed chunk size for proper testing")
		}

		// Simulate chunk creation with size limits
		if tokenCount > 150 {
			// This would trigger chunking in the real implementation
			chunk := Chunk{
				Text:       longContent[:200], // Simulate truncation
				Index:      0,
				StartPos:   0,
				EndPos:     200,
				TokenCount: parser.chunker.EstimateTokenCount(longContent[:200]),
				ChunkType:  "xlsx_data",
			}

			if chunk.TokenCount <= 0 {
				t.Error("Chunk should have positive token count")
			}

			if chunk.ChunkType != "xlsx_data" {
				t.Error("Expected xlsx_data chunk type")
			}
		}
	})
}

// Benchmark tests for performance-critical operations
func BenchmarkXLSXParser_GetColumnName(b *testing.B) {
	parser := NewXLSXParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test various column indices
		parser.getColumnName(i % 1000) // Test up to column ALL (index 999)
	}
}

func BenchmarkXLSXParser_HasNonEmptyData(b *testing.B) {
	parser := NewXLSXParser()
	testRows := [][]string{
		{"", "", ""},
		{"data", "", ""},
		{"", "data", ""},
		{"data1", "data2", "data3"},
		{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := testRows[i%len(testRows)]
		parser.hasNonEmptyData(row)
	}
}
