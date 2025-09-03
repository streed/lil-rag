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

// ParsePDFWithPageChunks parses PDF and returns page-based chunks
func (p *PDFParser) ParsePDFWithPageChunks(filePath, _ string) ([]Chunk, error) {
	doc, err := p.ParsePDF(filePath)
	if err != nil {
		return nil, err
	}

	chunks := make([]Chunk, 0, len(doc.Pages))

	for i, page := range doc.Pages {
		// Skip empty pages
		if strings.TrimSpace(page.Text) == "" {
			continue
		}

		pageNum := page.PageNumber
		chunk := Chunk{
			Text:       page.Text,
			Index:      i, // Sequential index for non-empty pages
			StartPos:   0,
			EndPos:     len(page.Text),
			TokenCount: page.Words,
			PageNumber: &pageNum,
			ChunkType:  "pdf_page",
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
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
