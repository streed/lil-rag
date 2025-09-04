package lilrag

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// CSVParser handles CSV files
type CSVParser struct {
	chunker *TextChunker
}

// NewCSVParser creates a new CSV parser
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// Parse extracts text content from a CSV file
func (cp *CSVParser) Parse(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("failed to read CSV file: %w", err)
	}

	var content strings.Builder
	
	// Process header row if exists
	if len(records) > 0 {
		header := records[0]
		content.WriteString("CSV Headers: ")
		content.WriteString(strings.Join(header, " | "))
		content.WriteString("\n\n")
		
		// Process data rows
		for i, record := range records[1:] {
			rowNum := i + 1
			content.WriteString(fmt.Sprintf("Row %d: ", rowNum))
			
			// Create meaningful row representation
			for j, cell := range record {
				if j < len(header) {
					content.WriteString(fmt.Sprintf("%s: %s", header[j], cell))
				} else {
					content.WriteString(fmt.Sprintf("Column%d: %s", j+1, cell))
				}
				if j < len(record)-1 {
					content.WriteString(" | ")
				}
			}
			content.WriteString("\n")
		}
	}

	return content.String(), nil
}

// ParseWithChunks extracts and chunks content from a CSV file
func (cp *CSVParser) ParseWithChunks(filePath, documentID string) ([]Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) == 0 {
		return []Chunk{}, nil
	}

	// Use a default chunker if none provided
	if cp.chunker == nil {
		cp.chunker = NewTextChunker(256, 38) // Use optimized defaults
	}

	var chunks []Chunk
	chunkIndex := 0
	header := records[0]
	
	// Create header chunk
	headerText := fmt.Sprintf("CSV Document Headers: %s", strings.Join(header, " | "))
	headerChunk := Chunk{
		Text:       headerText,
		Index:      chunkIndex,
		StartPos:   0,
		EndPos:     len(headerText),
		TokenCount: cp.chunker.EstimateTokenCount(headerText),
		ChunkType:  "csv_header",
	}
	chunks = append(chunks, headerChunk)
	chunkIndex++

	// Group rows into optimally sized chunks
	var currentChunk strings.Builder
	var rowsInChunk []int
	estimatedTokens := 0
	
	for i, record := range records[1:] {
		rowNum := i + 1
		rowText := fmt.Sprintf("Row %d: ", rowNum)
		
		// Create meaningful row representation
		for j, cell := range record {
			if j < len(header) {
				rowText += fmt.Sprintf("%s: %s", header[j], cell)
			} else {
				rowText += fmt.Sprintf("Column%d: %s", j+1, cell)
			}
			if j < len(record)-1 {
				rowText += " | "
			}
		}
		rowText += "\n"
		
		rowTokens := cp.chunker.EstimateTokenCount(rowText)
		
		// If adding this row would exceed chunk size, create a chunk
		if estimatedTokens+rowTokens > 200 && len(rowsInChunk) > 0 { // Smaller chunks for tabular data
			chunk := Chunk{
				Text:       currentChunk.String(),
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     currentChunk.Len(),
				TokenCount: estimatedTokens,
				ChunkType:  "csv_rows",
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
	
	// Add final chunk if there's content
	if currentChunk.Len() > 0 {
		chunk := Chunk{
			Text:       currentChunk.String(),
			Index:      chunkIndex,
			StartPos:   0,
			EndPos:     currentChunk.Len(),
			TokenCount: estimatedTokens,
			ChunkType:  "csv_rows",
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// SupportedExtensions returns the file extensions this parser supports
func (cp *CSVParser) SupportedExtensions() []string {
	return []string{".csv"}
}

// GetDocumentType returns the type of documents this parser handles
func (cp *CSVParser) GetDocumentType() DocumentType {
	return DocumentTypeCSV
}