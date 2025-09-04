package lilrag

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"os/exec"

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

// cleanText removes invisible characters and normalizes text for better searchability
func cleanText(text string) string {
	if text == "" {
		return text
	}
	
	// Step 1: Remove specific invisible Unicode characters by iterating through the string
	var cleanedRunes []rune
	for _, r := range text {
		// Skip zero-width and invisible characters
		if r >= 0x200B && r <= 0x200F || // Zero width spaces and marks
		   r >= 0x2028 && r <= 0x202F || // Line/paragraph separators and thin spaces
		   r >= 0x205F && r <= 0x206F || // Mathematical spaces and marks
		   r == 0xFEFF { // Byte order mark
			continue
		}
		cleanedRunes = append(cleanedRunes, r)
	}
	text = string(cleanedRunes)
	
	// Step 2: Fix common PDF OCR issue - spaced characters
	// First, handle sequences of single spaced letters (like "G o a l s" -> "Goals")
	spacedLetters := regexp.MustCompile(`\b([a-zA-Z])(?:\s+([a-zA-Z]))+\b`)
	
	// Custom function to collapse spaced letter sequences
	text = spacedLetters.ReplaceAllStringFunc(text, func(match string) string {
		// Remove all spaces within the word
		return regexp.MustCompile(`\s+`).ReplaceAllString(match, "")
	})
	
	// Also handle cases where individual words got separated (like "of C oncept" -> "of Concept")
	spacedWords := regexp.MustCompile(`\b([a-zA-Z]{1,2})\s+([A-Z][a-zA-Z]*)\b`)
	for i := 0; i < 5; i++ { // Limit iterations
		newText := spacedWords.ReplaceAllString(text, "$1$2")
		if newText == text {
			break
		}
		text = newText
	}
	
	// Add spaces back between likely word boundaries (like "ProofofConcept" -> "Proof of Concept")
	wordBoundaries := regexp.MustCompile(`([a-z])([A-Z])`)
	text = wordBoundaries.ReplaceAllString(text, "$1 $2")
	
	// Step 3: Normalize multiple whitespace to single spaces
	multipleSpaces := regexp.MustCompile(`\s+`)
	text = multipleSpaces.ReplaceAllString(text, " ")
	
	// Step 4: Remove non-printable characters but keep valid Unicode letters, numbers, and punctuation
	var cleaned strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			// Keep ASCII and valid Unicode characters
			if r <= 127 || unicode.IsLetter(r) || unicode.IsNumber(r) {
				cleaned.WriteRune(r)
			}
		}
	}
	
	// Step 5: Final cleanup - normalize spaces and trim
	result := strings.TrimSpace(cleaned.String())
	result = multipleSpaces.ReplaceAllString(result, " ")
	
	return result
}

// cleanTextPreserveLayout is a less aggressive cleaning function that preserves layout and spacing
func cleanTextPreserveLayout(text string) string {
	if text == "" {
		return text
	}
	
	// Step 1: Remove specific invisible Unicode characters but preserve regular spaces
	var cleanedRunes []rune
	for _, r := range text {
		// Skip zero-width and invisible characters but keep regular spaces and tabs
		if r >= 0x200B && r <= 0x200F || // Zero width spaces and marks
		   r >= 0x2028 && r <= 0x202F || // Line/paragraph separators (but keep thin spaces)
		   r >= 0x205F && r <= 0x206F || // Mathematical spaces and marks
		   r == 0xFEFF { // Byte order mark
			continue
		}
		cleanedRunes = append(cleanedRunes, r)
	}
	text = string(cleanedRunes)
	
	// Step 2: Only fix very obvious OCR issues - don't be too aggressive
	// Fix sequences like "G o a l s" -> "Goals" but preserve intentional spacing
	spacedLetters := regexp.MustCompile(`\b([a-zA-Z])\s([a-zA-Z])\s([a-zA-Z])\s([a-zA-Z]+)\b`)
	text = spacedLetters.ReplaceAllStringFunc(text, func(match string) string {
		// Only collapse if it looks like OCR error (single chars separated by single spaces)
		parts := strings.Fields(match)
		if len(parts) >= 3 {
			allSingleChars := true
			for i := 0; i < len(parts)-1; i++ {
				if len(parts[i]) != 1 {
					allSingleChars = false
					break
				}
			}
			if allSingleChars {
				return strings.Join(parts, "")
			}
		}
		return match
	})
	
	// Step 3: Remove non-printable characters but keep spaces, tabs, and newlines
	var cleaned strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) || 
		   unicode.IsPunct(r) || unicode.IsSymbol(r) || r == '\t' || r == '\n' {
			cleaned.WriteRune(r)
		}
	}
	
	// Step 4: Minimal cleanup - just trim and normalize line endings
	result := strings.TrimSpace(cleaned.String())
	
	// Only normalize excessive newlines (3+ in a row) to preserve layout
	excessiveNewlines := regexp.MustCompile(`\n{3,}`)
	result = excessiveNewlines.ReplaceAllString(result, "\n\n")
	
	return result
}

func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

func (p *PDFParser) ParsePDF(filePath string) (*PDFDocument, error) {
	// Try pdftotext CLI tool first for best compatibility
	doc, err := p.parsePDFWithPDFToText(filePath)
	if err != nil {
		// Fallback to dslipak/pdf library
		doc, dslipakErr := p.parsePDFWithDslipak(filePath)
		if dslipakErr != nil {
			log.Printf("WARNING: PDF text extraction may have issues - all parsers failed for %s. Text may not be fully searchable.", filePath)
			return nil, fmt.Errorf("failed to extract text with pdftotext: %w, and dslipak/pdf: %v", err, dslipakErr)
		}
		log.Printf("WARNING: Using fallback PDF parser for %s - text extraction quality may vary.", filePath)
		return doc, nil
	}
	return doc, nil
}

func (p *PDFParser) parsePDFWithPDFToText(filePath string) (*PDFDocument, error) {
	// Check if pdftotext is available
	_, err := exec.LookPath("pdftotext")
	if err != nil {
		return nil, fmt.Errorf("pdftotext command not found: %w", err)
	}

	// Run pdftotext with layout preservation
	cmd := exec.Command("pdftotext", "-layout", "-colspacing", "0.3", filePath, "-")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext failed: %w", err)
	}

	text := string(output)
	cleanedText := cleanTextPreserveLayout(text)
	
	doc := &PDFDocument{
		Pages:      make([]PDFPage, 0),
		Title:      filepath.Base(filePath),
		TotalPages: 1, // pdftotext output as single combined text with preserved layout
	}

	if strings.TrimSpace(cleanedText) != "" {
		wordCount := len(strings.Fields(cleanedText))
		
		doc.Pages = append(doc.Pages, PDFPage{
			PageNumber: 1,
			Text:       cleanedText,
			Words:      wordCount,
		})
	}

	return doc, nil
}

func (p *PDFParser) parsePDFWithDslipak(filePath string) (*PDFDocument, error) {
	r, err := pdf.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF with dslipak/pdf: %w", err)
	}

	doc := &PDFDocument{
		Pages:      make([]PDFPage, 0),
		Title:      filepath.Base(filePath),
		TotalPages: r.NumPage(),
	}

	// Try to get all text at once first
	plainTextReader, err := r.GetPlainText()
	if err == nil {
		buf := make([]byte, 1024*1024) // 1MB buffer
		var allText strings.Builder
		for {
			n, readErr := plainTextReader.Read(buf)
			if n > 0 {
				allText.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
		
		text := allText.String()
		cleanedText := cleanText(text)
		if strings.TrimSpace(cleanedText) != "" {
			wordCount := len(strings.Fields(cleanedText))
			doc.Pages = append(doc.Pages, PDFPage{
				PageNumber: 1,
				Text:       cleanedText,
				Words:      wordCount,
			})
		}
		return doc, nil
	}

	// If that fails, try page by page
	var allText strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}

		plainTextBytes, err := page.GetPlainText(nil)
		if err != nil {
			// Continue with other pages if one fails
			continue
		}

		text := string(plainTextBytes)
		cleanedText := cleanText(text)
		if strings.TrimSpace(cleanedText) != "" {
			allText.WriteString(cleanedText)
			allText.WriteString("\n\n")
		}
	}

	// Since dslipak/pdf doesn't easily give us per-page text, we'll treat the entire document as one page
	fullText := strings.TrimSpace(allText.String())
	if fullText != "" {
		wordCount := len(strings.Fields(fullText))
		doc.Pages = append(doc.Pages, PDFPage{
			PageNumber: 1,
			Text:       fullText,
			Words:      wordCount,
		})
	}

	return doc, nil
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

// Parse extracts all text content from a PDF file (implements DocumentParser interface)
func (p *PDFParser) Parse(filePath string) (string, error) {
	doc, err := p.ParsePDF(filePath)
	if err != nil {
		return "", err
	}
	
	var allText strings.Builder
	for _, page := range doc.Pages {
		if strings.TrimSpace(page.Text) != "" {
			allText.WriteString(page.Text)
			allText.WriteString("\n\n")
		}
	}
	
	return strings.TrimSpace(allText.String()), nil
}

// ParseWithChunks extracts and chunks PDF content (implements DocumentParser interface)
func (p *PDFParser) ParseWithChunks(filePath, documentID string) ([]Chunk, error) {
	return p.ParsePDFWithPageChunks(filePath, documentID)
}

// SupportedExtensions returns the file extensions this parser supports
func (p *PDFParser) SupportedExtensions() []string {
	return []string{".pdf"}
}

// GetDocumentType returns the type of documents this parser handles
func (p *PDFParser) GetDocumentType() DocumentType {
	return DocumentTypePDF
}

// IsPDFFile checks if a file is a PDF based on its extension
func IsPDFFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".pdf"
}
