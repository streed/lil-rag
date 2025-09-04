package lilrag

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// XLSXParser handles Excel .xlsx files
type XLSXParser struct {
	chunker *TextChunker
}

// NewXLSXParser creates a new XLSX parser
func NewXLSXParser() *XLSXParser {
	return &XLSXParser{}
}

// Parse extracts text content from an XLSX file
func (xp *XLSXParser) Parse(filePath string) (string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open XLSX file: %w", err)
	}
	defer f.Close()

	var content strings.Builder
	
	// Get all sheet names
	sheetNames := f.GetSheetList()
	
	for _, sheetName := range sheetNames {
		content.WriteString(fmt.Sprintf("Sheet: %s\n", sheetName))
		content.WriteString("=" + strings.Repeat("=", len(sheetName)+6) + "\n\n")
		
		// Get all rows from the sheet
		rows, err := f.GetRows(sheetName)
		if err != nil {
			content.WriteString(fmt.Sprintf("Error reading sheet %s: %v\n\n", sheetName, err))
			continue
		}
		
		// Process rows
		for rowIndex, row := range rows {
			if len(row) == 0 {
				continue // Skip empty rows
			}
			
			rowNum := rowIndex + 1
			content.WriteString(fmt.Sprintf("Row %d: ", rowNum))
			
			// Join non-empty cells
			var cells []string
			for colIndex, cell := range row {
				if strings.TrimSpace(cell) != "" {
					columnName := xp.getColumnName(colIndex)
					cells = append(cells, fmt.Sprintf("%s: %s", columnName, cell))
				}
			}
			
			if len(cells) > 0 {
				content.WriteString(strings.Join(cells, " | "))
			}
			content.WriteString("\n")
		}
		
		content.WriteString("\n")
	}

	return content.String(), nil
}

// ParseWithChunks extracts and chunks content from an XLSX file
func (xp *XLSXParser) ParseWithChunks(filePath, documentID string) ([]Chunk, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSX file: %w", err)
	}
	defer f.Close()

	// Use a default chunker if none provided
	if xp.chunker == nil {
		xp.chunker = NewTextChunker(200, 30) // Smaller chunks for tabular data
	}

	var chunks []Chunk
	chunkIndex := 0
	
	// Get all sheet names
	sheetNames := f.GetSheetList()
	
	for _, sheetName := range sheetNames {
		// Create sheet header chunk
		sheetHeaderText := fmt.Sprintf("Excel Sheet: %s", sheetName)
		headerChunk := Chunk{
			Text:       sheetHeaderText,
			Index:      chunkIndex,
			StartPos:   0,
			EndPos:     len(sheetHeaderText),
			TokenCount: xp.chunker.EstimateTokenCount(sheetHeaderText),
			ChunkType:  "xlsx_sheet_header",
		}
		chunks = append(chunks, headerChunk)
		chunkIndex++
		
		// Get all rows from the sheet
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue // Skip sheets with errors
		}
		
		// Detect header row (usually first non-empty row)
		var headerRow []string
		var dataStartIndex int
		
		for i, row := range rows {
			if len(row) > 0 && xp.hasNonEmptyData(row) {
				headerRow = row
				dataStartIndex = i + 1
				break
			}
		}
		
		// Create header chunk if we found headers
		if len(headerRow) > 0 {
			headerText := fmt.Sprintf("Sheet %s Headers: %s", sheetName, strings.Join(headerRow, " | "))
			headerChunk := Chunk{
				Text:       headerText,
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     len(headerText),
				TokenCount: xp.chunker.EstimateTokenCount(headerText),
				ChunkType:  "xlsx_headers",
			}
			chunks = append(chunks, headerChunk)
			chunkIndex++
		}
		
		// Process data rows in chunks
		var currentChunk strings.Builder
		var rowsInChunk []int
		estimatedTokens := 0
		
		for i := dataStartIndex; i < len(rows); i++ {
			row := rows[i]
			
			if !xp.hasNonEmptyData(row) {
				continue // Skip empty rows
			}
			
			rowNum := i + 1
			rowText := fmt.Sprintf("Row %d: ", rowNum)
			
			// Create meaningful row representation
			var cells []string
			for colIndex, cell := range row {
				if strings.TrimSpace(cell) != "" {
					var columnName string
					if colIndex < len(headerRow) && strings.TrimSpace(headerRow[colIndex]) != "" {
						columnName = headerRow[colIndex]
					} else {
						columnName = xp.getColumnName(colIndex)
					}
					cells = append(cells, fmt.Sprintf("%s: %s", columnName, cell))
				}
			}
			
			if len(cells) > 0 {
				rowText += strings.Join(cells, " | ")
			}
			rowText += "\n"
			
			rowTokens := xp.chunker.EstimateTokenCount(rowText)
			
			// If adding this row would exceed chunk size, create a chunk
			if estimatedTokens+rowTokens > 150 && len(rowsInChunk) > 0 { // Smaller chunks for spreadsheet data
				chunk := Chunk{
					Text:       currentChunk.String(),
					Index:      chunkIndex,
					StartPos:   0,
					EndPos:     currentChunk.Len(),
					TokenCount: estimatedTokens,
					ChunkType:  "xlsx_data",
				}
				chunks = append(chunks, chunk)
				chunkIndex++
				
				// Reset for next chunk
				currentChunk.Reset()
				rowsInChunk = []int{}
				estimatedTokens = 0
			}
			
			// Add row to current chunk
			currentChunk.WriteString(rowText)
			rowsInChunk = append(rowsInChunk, rowNum)
			estimatedTokens += rowTokens
		}
		
		// Add final chunk for this sheet if there's content
		if currentChunk.Len() > 0 {
			chunk := Chunk{
				Text:       currentChunk.String(),
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     currentChunk.Len(),
				TokenCount: estimatedTokens,
				ChunkType:  "xlsx_data",
			}
			chunks = append(chunks, chunk)
			chunkIndex++
		}
	}

	return chunks, nil
}

// hasNonEmptyData checks if a row has any non-empty data
func (xp *XLSXParser) hasNonEmptyData(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return true
		}
	}
	return false
}

// getColumnName converts column index to Excel-style column name (A, B, C, etc.)
func (xp *XLSXParser) getColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+(index%26))) + name
		index = index/26 - 1
		if index < 0 {
			break
		}
	}
	return name
}

// SupportedExtensions returns the file extensions this parser supports
func (xp *XLSXParser) SupportedExtensions() []string {
	return []string{".xlsx"}
}

// GetDocumentType returns the type of documents this parser handles
func (xp *XLSXParser) GetDocumentType() DocumentType {
	return DocumentTypeXLSX
}