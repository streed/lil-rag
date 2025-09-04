package lilrag

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// HTMLParser handles HTML files
type HTMLParser struct {
	chunker *TextChunker
}

// NewHTMLParser creates a new HTML parser
func NewHTMLParser() *HTMLParser {
	return &HTMLParser{}
}

// Parse extracts text content from an HTML file
func (hp *HTMLParser) Parse(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open HTML file: %w", err)
	}
	defer file.Close()

	doc, err := html.Parse(file)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML file: %w", err)
	}

	var content strings.Builder

	// Extract title if present
	title := hp.extractTitle(doc)
	if title != "" {
		content.WriteString("Title: ")
		content.WriteString(title)
		content.WriteString("\n\n")
	}

	// Extract body text
	bodyText := hp.extractText(doc)
	content.WriteString(bodyText)

	// Clean up excessive whitespace
	cleanText := hp.cleanWhitespace(content.String())
	return cleanText, nil
}

// ParseWithChunks extracts and chunks content from an HTML file
func (hp *HTMLParser) ParseWithChunks(filePath, _ string) ([]Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open HTML file: %w", err)
	}
	defer file.Close()

	doc, err := html.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML file: %w", err)
	}

	// Use a default chunker if none provided
	if hp.chunker == nil {
		hp.chunker = NewTextChunker(256, 38) // Use optimized defaults for structured content
	}

	var chunks []Chunk
	chunkIndex := 0

	// Extract title as separate chunk
	title := hp.extractTitle(doc)
	if title != "" {
		titleText := fmt.Sprintf("Document Title: %s", title)
		titleChunk := Chunk{
			Text:       titleText,
			Index:      chunkIndex,
			StartPos:   0,
			EndPos:     len(titleText),
			TokenCount: hp.chunker.EstimateTokenCount(titleText),
			ChunkType:  "html_title",
		}
		chunks = append(chunks, titleChunk)
		chunkIndex++
	}

	// Extract structured content by sections
	sections := hp.extractSections(doc)

	if len(sections) > 0 {
		// Chunk each section separately to preserve HTML structure
		for _, section := range sections {
			if strings.TrimSpace(section) == "" {
				continue
			}

			sectionChunks := hp.chunker.ChunkText(section)
			for _, chunk := range sectionChunks {
				chunk.Index = chunkIndex
				chunk.ChunkType = "html_section"
				chunks = append(chunks, chunk)
				chunkIndex++
			}
		}
	} else {
		// Fallback to regular text chunking
		bodyText := hp.extractText(doc)
		if strings.TrimSpace(bodyText) != "" {
			bodyText = hp.cleanWhitespace(bodyText)
			bodyChunks := hp.chunker.ChunkText(bodyText)

			for _, chunk := range bodyChunks {
				chunk.Index = chunkIndex
				chunk.ChunkType = "html_content"
				chunks = append(chunks, chunk)
				chunkIndex++
			}
		}
	}

	return chunks, nil
}

// extractTitle finds and extracts the HTML title
func (hp *HTMLParser) extractTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		return hp.extractNodeText(n)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if title := hp.extractTitle(c); title != "" {
			return title
		}
	}

	return ""
}

// extractSections extracts content organized by HTML sections/headings
func (hp *HTMLParser) extractSections(n *html.Node) []string {
	var sections []string
	var currentSection strings.Builder

	hp.walkSections(n, &sections, &currentSection)

	// Add final section if there's content
	if currentSection.Len() > 0 {
		sections = append(sections, currentSection.String())
	}

	return sections
}

// walkSections traverses HTML nodes to extract sectioned content
func (hp *HTMLParser) walkSections(n *html.Node, sections *[]string, currentSection *strings.Builder) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			// New section found - save current section and start new one
			if currentSection.Len() > 0 {
				*sections = append(*sections, currentSection.String())
				currentSection.Reset()
			}

			// Add heading to new section
			headingText := hp.extractNodeText(n)
			fmt.Fprintf(currentSection, "## %s\n\n", headingText)
			return // Don't process children separately

		case "p", "div", "article", "section":
			text := hp.extractNodeText(n)
			if strings.TrimSpace(text) != "" {
				currentSection.WriteString(text)
				currentSection.WriteString("\n\n")
			}
			return // Don't process children separately

		case "ul", "ol":
			listText := hp.extractListText(n)
			if listText != "" {
				currentSection.WriteString(listText)
				currentSection.WriteString("\n\n")
			}
			return // Don't process children separately
		}
	}

	// Process children for other node types
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		hp.walkSections(c, sections, currentSection)
	}
}

// extractText extracts all readable text from HTML
func (hp *HTMLParser) extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	if n.Type == html.ElementNode {
		// Skip script and style tags
		if n.Data == "script" || n.Data == "style" {
			return ""
		}
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childText := hp.extractText(c)
		if strings.TrimSpace(childText) != "" {
			text.WriteString(childText)
			text.WriteString(" ")
		}
	}

	return text.String()
}

// extractNodeText extracts text from a specific node and its children
func (hp *HTMLParser) extractNodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(hp.extractNodeText(c))
	}

	return text.String()
}

// extractListText extracts formatted text from lists
func (hp *HTMLParser) extractListText(n *html.Node) string {
	var items []string

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			itemText := strings.TrimSpace(hp.extractNodeText(c))
			if itemText != "" {
				items = append(items, fmt.Sprintf("• %s", itemText))
			}
		}
	}

	return strings.Join(items, "\n")
}

// cleanWhitespace removes excessive whitespace and normalizes text
func (hp *HTMLParser) cleanWhitespace(text string) string {
	// Replace multiple whitespace with single spaces
	re := regexp.MustCompile(`\s+`)
	cleaned := re.ReplaceAllString(text, " ")

	// Replace multiple newlines with double newlines
	re = regexp.MustCompile(`\n\s*\n\s*\n+`)
	cleaned = re.ReplaceAllString(cleaned, "\n\n")

	return strings.TrimSpace(cleaned)
}

// SupportedExtensions returns the file extensions this parser supports
func (hp *HTMLParser) SupportedExtensions() []string {
	return []string{".html", ".htm"}
}

// GetDocumentType returns the type of documents this parser handles
func (hp *HTMLParser) GetDocumentType() DocumentType {
	return DocumentTypeHTML
}
