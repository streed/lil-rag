package lilrag

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

// ImageParser handles OCR extraction from image documents using Ollama vision models
type ImageParser struct {
	ollamaURL string
	model     string
	client    *http.Client
	chunker   *TextChunker
}

// NewImageParser creates a new image parser with OCR capabilities
func NewImageParser(ollamaURL, model string, chunker *TextChunker) *ImageParser {
	if ollamaURL == "" {
		ollamaURL = DefaultOllamaURL
	}
	if model == "" {
		model = "llama3.2-vision" // Default vision model
	}

	return &ImageParser{
		ollamaURL: ollamaURL,
		model:     model,
		client: &http.Client{
			Timeout: 300 * time.Second, // Longer timeout for vision processing
		},
		chunker: chunker,
	}
}

// VisionRequest represents a request to Ollama's vision API
type VisionRequest struct {
	Model    string           `json:"model"`
	Messages []VisionMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  *VisionOptions   `json:"options,omitempty"`
}

// VisionMessage represents a message with image content
type VisionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Images  []string `json:"images,omitempty"` // Base64 encoded images
}

// VisionOptions for controlling vision model generation
type VisionOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// VisionResponse represents Ollama's vision response
type VisionResponse struct {
	Model     string        `json:"model"`
	CreatedAt time.Time     `json:"created_at"`
	Message   VisionMessage `json:"message"`
	Done      bool          `json:"done"`
}

// ResizeImage resizes an image to fit within maxSize while preserving aspect ratio
func (p *ImageParser) ResizeImage(imagePath string, maxSize int) ([]byte, error) {
	// Open and decode the image
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode image based on file extension
	ext := strings.ToLower(filepath.Ext(imagePath))
	var img image.Image
	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
	case ".png":
		img, err = png.Decode(file)
	case ".gif":
		img, err = gif.Decode(file)
	case ".webp":
		img, err = webp.Decode(file)
	case ".tiff", ".tif":
		img, err = tiff.Decode(file)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// Calculate new dimensions while preserving aspect ratio
	newWidth, newHeight := calculateResizeDimensions(origWidth, origHeight, maxSize)

	// If no resizing needed, return original
	if newWidth == origWidth && newHeight == origHeight {
		file.Seek(0, 0)
		return io.ReadAll(file)
	}

	// Create resized image
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// Encode to bytes (always use JPEG for efficiency)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode resized image: %w", err)
	}

	return buf.Bytes(), nil
}

// calculateResizeDimensions calculates new dimensions that fit within maxSize while preserving aspect ratio
func calculateResizeDimensions(origWidth, origHeight, maxSize int) (int, int) {
	if origWidth <= maxSize && origHeight <= maxSize {
		return origWidth, origHeight
	}

	aspectRatio := float64(origWidth) / float64(origHeight)
	
	if origWidth > origHeight {
		// Width is the limiting dimension
		newWidth := maxSize
		newHeight := int(float64(maxSize) / aspectRatio)
		return newWidth, newHeight
	} else {
		// Height is the limiting dimension
		newHeight := maxSize
		newWidth := int(float64(maxSize) * aspectRatio)
		return newWidth, newHeight
	}
}

// Parse extracts text content from an image file using OCR
func (p *ImageParser) Parse(filePath string) (string, error) {
	// Resize image to fit within 1120x1120 pixels for optimal OCR processing
	imageData, err := p.ResizeImage(filePath, 1120)
	if err != nil {
		return "", fmt.Errorf("failed to resize image for OCR: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Create OCR prompt for proper markdown output
	ocrPrompt := `Analyze this image and extract all visible text content using proper markdown formatting. Please:

**CONTENT TO EXTRACT:**
1. Read all text in the image, including:
   - Headers, titles, and subtitles
   - Body text and paragraphs
   - Lists, bullet points, and numbered items
   - Tables, captions, and labels
   - Any handwritten text (if legible)
   - Text in charts, diagrams, or figures

**MARKDOWN FORMATTING REQUIREMENTS:**
- Use proper markdown headers: # for main titles, ## for sections, ### for subsections
- Use **bold text** for emphasized or important text
- Use *italic text* for subtle emphasis
- Use standard markdown lists:
  - For unordered lists: use "- " (dash + space) consistently
  - For ordered lists: use "1. ", "2. ", etc.
  - For nested lists: indent with 2 spaces per level
- Use proper code blocks with triple backticks for code sections
- Use > for blockquotes if applicable
- Use | tables | format | for tabular data
- Use [link text](url) format for any URLs or references

**STRUCTURE REQUIREMENTS:**
- Maintain the original reading order (left to right, top to bottom)
- Preserve document hierarchy and organization
- Use consistent formatting throughout
- Separate sections with blank lines
- If text is unclear or partially obscured, indicate with *[unclear text]*

**OUTPUT FORMAT:**
Provide ONLY the extracted text content in proper markdown format, without any additional commentary, explanations, or meta-text.`

	// Create vision request
	messages := []VisionMessage{
		{
			Role:    "user",
			Content: ocrPrompt,
			Images:  []string{base64Image},
		},
	}

	requestBody := VisionRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   false,
		Options: &VisionOptions{
			Temperature: 0.1, // Low temperature for more accurate OCR
		},
	}

	// Marshal request
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OCR request: %w", err)
	}

	// Send request to Ollama
	url := fmt.Sprintf("%s/api/chat", p.ollamaURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create OCR request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send OCR request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("OCR request failed with status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("OCR request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var visionResp VisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&visionResp); err != nil {
		return "", fmt.Errorf("failed to decode OCR response: %w", err)
	}

	extractedText := strings.TrimSpace(visionResp.Message.Content)
	if extractedText == "" {
		return "", fmt.Errorf("no text could be extracted from the image")
	}

	return extractedText, nil
}

// ParseWithChunks extracts text from image and creates chunks optimized for the content
func (p *ImageParser) ParseWithChunks(filePath, documentID string) ([]Chunk, error) {
	// Extract text using OCR
	text, err := p.Parse(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text from image: %w", err)
	}

	if p.chunker == nil {
		// Create a single chunk if no chunker available
		return []Chunk{
			{
				Text:       text,
				Index:      0,
				StartPos:   0,
				EndPos:     len(text),
				TokenCount: len(strings.Fields(text)), // Simple token estimation
				ChunkType:  "image_ocr",
				PageNumber: nil, // Images don't have page numbers
			},
		}, nil
	}

	// Use chunker to split the extracted text
	chunks := p.chunker.ChunkText(text)
	
	// Update chunk metadata to indicate these are from image OCR
	for i := range chunks {
		chunks[i].ChunkType = "image_ocr"
	}

	return chunks, nil
}

// SupportedExtensions returns the image file extensions this parser supports
func (p *ImageParser) SupportedExtensions() []string {
	return []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"}
}

// GetDocumentType returns the document type this parser handles
func (p *ImageParser) GetDocumentType() DocumentType {
	return DocumentTypeImage
}

// TestVisionModel tests if the vision model is available and working
func (p *ImageParser) TestVisionModel(ctx context.Context) error {
	// Create a simple test request with a minimal image (1x1 pixel PNG)
	testImage := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
	
	messages := []VisionMessage{
		{
			Role:    "user",
			Content: "What do you see in this image?",
			Images:  []string{testImage},
		},
	}

	requestBody := VisionRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   false,
		Options: &VisionOptions{
			Temperature: 0.1,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal test request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", p.ollamaURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to vision model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("vision model test failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("vision model test failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// IsImageFile checks if a file is a supported image format
func IsImageFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	supportedExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"}
	
	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	return false
}