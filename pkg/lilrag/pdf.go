package lilrag

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dslipak/pdf"
)

type PDFPage struct {
	PageNumber int
	Text       string
	Words      int
}

type PDFDocument struct {
	Pages      []PDFPage
	Title      string
	TotalPages int
}

type PDFParser struct{}

func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

func (p *PDFParser) ParsePDF(filePath string) (*PDFDocument, error) {
	// Open the PDF file
	r, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}

	totalPages := r.NumPage()
	doc := &PDFDocument{
		Pages:      make([]PDFPage, 0, totalPages),
		Title:      filepath.Base(filePath),
		TotalPages: totalPages,
	}

	// Extract text from each page
	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			// Add empty page if null
			doc.Pages = append(doc.Pages, PDFPage{
				PageNumber: pageNum,
				Text:       "",
				Words:      0,
			})
			continue
		}

		pageText := p.extractPageText(page)

		wordCount := len(strings.Fields(pageText))

		doc.Pages = append(doc.Pages, PDFPage{
			PageNumber: pageNum,
			Text:       pageText,
			Words:      wordCount,
		})
	}

	return doc, nil
}

func (p *PDFParser) extractPageText(page pdf.Page) string {
	var textBuilder strings.Builder

	// Try to get text by rows first (better formatting)
	rows, err := page.GetTextByRow()
	if err != nil {
		// Fallback to plain text if row extraction fails
		plainTexts := page.Content().Text
		var plainTextBuilder strings.Builder
		for _, text := range plainTexts {
			plainTextBuilder.WriteString(text.S)
			plainTextBuilder.WriteString(" ")
		}
		return strings.TrimSpace(plainTextBuilder.String())
	}

	// Process rows and words
	for _, row := range rows {
		rowTexts := make([]string, 0, len(row.Content))
		for _, word := range row.Content {
			if word.S != "" {
				rowTexts = append(rowTexts, word.S)
			}
		}

		if len(rowTexts) > 0 {
			rowText := strings.Join(rowTexts, " ")
			textBuilder.WriteString(rowText)
			textBuilder.WriteString("\n")
		}
	}

	return strings.TrimSpace(textBuilder.String())
}

// GetPageID generates a chunk ID for a PDF page
func GetPDFPageID(documentID string, pageNumber int) string {
	return fmt.Sprintf("%s:page_%d", documentID, pageNumber)
}

// ParsePDFWithPageChunks parses PDF and returns optimally chunked content
func (p *PDFParser) ParsePDFWithPageChunks(filePath, documentID string) ([]Chunk, error) {
	doc, err := p.ParsePDF(filePath)
	if err != nil {
		return nil, err
	}

	// Create a simple chunker for PDF content
	chunker := NewTextChunker(1800, 200) // Use default chunking params
	var allChunks []Chunk
	chunkIndex := 0

	for _, page := range doc.Pages {
		// Skip empty pages
		if strings.TrimSpace(page.Text) == "" {
			continue
		}

		pageNum := page.PageNumber
		
		// For smaller pages, keep as single chunk
		if page.Words <= 1600 {
			chunk := Chunk{
				Text:       page.Text,
				Index:      chunkIndex,
				StartPos:   0,
				EndPos:     len(page.Text),
				TokenCount: page.Words,
				PageNumber: &pageNum,
				ChunkType:  "pdf_page",
			}
			allChunks = append(allChunks, chunk)
			chunkIndex++
		} else {
			// For larger pages, use semantic chunking while preserving page context
			pageChunks := chunker.ChunkText(page.Text)
			
			for _, chunk := range pageChunks {
				// Add page number and adjust chunk metadata
				chunk.Index = chunkIndex
				chunk.PageNumber = &pageNum
				chunk.ChunkType = "pdf_page_section"
				
				// Add page context prefix for better semantic understanding
				pageContext := fmt.Sprintf("Page %d: ", pageNum)
				chunk.Text = pageContext + chunk.Text
				chunk.TokenCount = chunker.EstimateTokenCount(chunk.Text)
				
				allChunks = append(allChunks, chunk)
				chunkIndex++
			}
		}
	}

	return p.optimizePDFChunks(allChunks), nil
}

// optimizePDFChunks post-processes PDF chunks for optimal retrieval
func (p *PDFParser) optimizePDFChunks(chunks []Chunk) []Chunk {
	if len(chunks) <= 1 {
		return chunks
	}

	var optimizedChunks []Chunk
	
	for i, chunk := range chunks {
		// Add cross-page context for better coherence
		if i > 0 && chunks[i-1].PageNumber != nil && chunk.PageNumber != nil {
			// If this chunk is from a different page, add context from previous page
			if *chunks[i-1].PageNumber != *chunk.PageNumber {
				chunk = p.addCrossPageContext(chunk, chunks[i-1])
			}
		}
		
		optimizedChunks = append(optimizedChunks, chunk)
	}
	
	return optimizedChunks
}

// addCrossPageContext adds relevant context from the previous page
func (p *PDFParser) addCrossPageContext(currentChunk, previousChunk Chunk) Chunk {
	// Extract last sentence or two from previous chunk for context
	prevText := strings.TrimSpace(previousChunk.Text)
	sentences := strings.Split(prevText, ". ")
	
	if len(sentences) > 1 {
		// Take last 1-2 sentences for context
		contextSentences := sentences[len(sentences)-2:]
		context := strings.Join(contextSentences, ". ")
		
		// Limit context to avoid making chunks too large
		if len(context) > 200 {
			context = context[:200] + "..."
		}
		
		// Prepend context with clear marker
		contextPrefix := fmt.Sprintf("(Previous context: %s) ", context)
		currentChunk.Text = contextPrefix + currentChunk.Text
		
		// Update token count
		chunker := NewTextChunker(1800, 200)
		currentChunk.TokenCount = chunker.EstimateTokenCount(currentChunk.Text)
	}
	
	return currentChunk
}

// EstimatePDFTokens estimates total tokens in a PDF document
func (p *PDFParser) EstimatePDFTokens(doc *PDFDocument) int {
	total := 0
	for _, page := range doc.Pages {
		total += page.Words
	}
	return total
}

// IsPDFFile checks if a file is a PDF based on its extension
func IsPDFFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".pdf"
}
