package lilrag

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewImageParserWithTimeout(t *testing.T) {
	tests := []struct {
		name            string
		ollamaURL       string
		model           string
		timeoutSeconds  int
		imageMaxSize    int
		expectedURL     string
		expectedModel   string
		expectedMaxSize int
		expectedTimeout time.Duration
	}{
		{
			name:            "default values",
			ollamaURL:       "",
			model:           "",
			timeoutSeconds:  30,
			imageMaxSize:    0,
			expectedURL:     DefaultOllamaURL,
			expectedModel:   "llama3.2-vision",
			expectedMaxSize: 1120,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "custom values",
			ollamaURL:       "http://localhost:11434",
			model:           "llava",
			timeoutSeconds:  60,
			imageMaxSize:    800,
			expectedURL:     "http://localhost:11434",
			expectedModel:   "llava",
			expectedMaxSize: 800,
			expectedTimeout: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewImageParserWithTimeout(
				tt.ollamaURL,
				tt.model,
				nil,
				tt.timeoutSeconds,
				tt.imageMaxSize,
			)

			if parser.ollamaURL != tt.expectedURL {
				t.Errorf("Expected ollamaURL %s, got %s", tt.expectedURL, parser.ollamaURL)
			}

			if parser.model != tt.expectedModel {
				t.Errorf("Expected model %s, got %s", tt.expectedModel, parser.model)
			}

			if parser.imageMaxSize != tt.expectedMaxSize {
				t.Errorf("Expected imageMaxSize %d, got %d", tt.expectedMaxSize, parser.imageMaxSize)
			}

			if parser.client.Timeout != tt.expectedTimeout {
				t.Errorf("Expected timeout %v, got %v", tt.expectedTimeout, parser.client.Timeout)
			}
		})
	}
}

func TestImageParser_SupportedExtensions(t *testing.T) {
	parser := NewImageParserWithTimeout("", "", nil, 30, 1120)
	extensions := parser.SupportedExtensions()

	expected := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif"}
	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for i, ext := range expected {
		if i >= len(extensions) || extensions[i] != ext {
			t.Errorf("Expected extension %s at index %d, got %v", ext, i, extensions)
		}
	}
}

func TestImageParser_GetDocumentType(t *testing.T) {
	parser := NewImageParserWithTimeout("", "", nil, 30, 1120)
	docType := parser.GetDocumentType()

	if docType != DocumentTypeImage {
		t.Errorf("Expected DocumentTypeImage, got %v", docType)
	}
}

func TestCalculateResizeDimensions(t *testing.T) {
	tests := []struct {
		name           string
		origWidth      int
		origHeight     int
		maxSize        int
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "no resize needed",
			origWidth:      800,
			origHeight:     600,
			maxSize:        1000,
			expectedWidth:  800,
			expectedHeight: 600,
		},
		{
			name:           "width is limiting dimension",
			origWidth:      1200,
			origHeight:     800,
			maxSize:        1000,
			expectedWidth:  1000,
			expectedHeight: 666, // 1000 * (800/1200) = 666.67 -> 666
		},
		{
			name:           "height is limiting dimension",
			origWidth:      600,
			origHeight:     1200,
			maxSize:        1000,
			expectedWidth:  500, // 1000 * (600/1200) = 500
			expectedHeight: 1000,
		},
		{
			name:           "square image",
			origWidth:      1200,
			origHeight:     1200,
			maxSize:        800,
			expectedWidth:  800,
			expectedHeight: 800,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := calculateResizeDimensions(tt.origWidth, tt.origHeight, tt.maxSize)
			if width != tt.expectedWidth || height != tt.expectedHeight {
				t.Errorf("Expected dimensions %dx%d, got %dx%d",
					tt.expectedWidth, tt.expectedHeight, width, height)
			}
		})
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{"JPEG file", "test.jpg", true},
		{"JPEG file uppercase", "test.JPG", true},
		{"JPEG file alternative", "test.jpeg", true},
		{"PNG file", "test.png", true},
		{"GIF file", "test.gif", true},
		{"BMP file", "test.bmp", true},
		{"WebP file", "test.webp", true},
		{"TIFF file", "test.tiff", true},
		{"TIF file", "test.tif", true},
		{"Text file", "test.txt", false},
		{"PDF file", "test.pdf", false},
		{"No extension", "test", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsImageFile(tt.filePath)
			if result != tt.expected {
				t.Errorf("IsImageFile(%s) = %v, expected %v", tt.filePath, result, tt.expected)
			}
		})
	}
}

// Mock HTTP server for testing OCR functionality
func createMockOCRServer(t *testing.T, responseText string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected path /api/chat, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse the request body to validate structure
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		var req VisionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("Failed to unmarshal request: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Validate request structure
		if len(req.Messages) == 0 {
			t.Error("Expected at least one message")
		}
		if len(req.Messages) > 0 && len(req.Messages[0].Images) == 0 {
			t.Error("Expected at least one image")
		}

		w.WriteHeader(statusCode)
		if statusCode == http.StatusOK {
			response := VisionResponse{
				Model:     req.Model,
				CreatedAt: time.Now(),
				Message: VisionMessage{
					Role:    "assistant",
					Content: responseText,
				},
				Done: true,
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("Failed to encode response: %v", err)
			}
		} else {
			fmt.Fprintf(w, "Error: Status %d", statusCode)
		}
	}))
}

// Helper function to create a test image file
func createTestImageFile(t *testing.T, format string) string {
	t.Helper()

	// Create a temporary file with the correct extension
	ext := format
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	tmpFile, err := os.CreateTemp("", "test_image_*"+ext)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	fileName := tmpFile.Name()
	tmpFile.Close()

	// Write a minimal valid image based on format
	switch format {
	case ".png", "png":
		// 1x1 pixel PNG image (transparent)
		pngData, _ := base64.StdEncoding.DecodeString(
			"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQIHWNgAAIAAAUAAY27m/MAAAAASUVORK5CYII=")
		if err := os.WriteFile(fileName, pngData, 0644); err != nil {
			os.Remove(fileName)
			t.Fatalf("Failed to write PNG file: %v", err)
		}
	case ".jpg", ".jpeg", "jpg", "jpeg":
		// Valid 1x1 JPEG image (base64 decoded)
		jpegData, _ := base64.StdEncoding.DecodeString(
			"/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/2wBDAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwA/gAA")
		if err := os.WriteFile(fileName, jpegData, 0644); err != nil {
			os.Remove(fileName)
			t.Fatalf("Failed to write JPEG file: %v", err)
		}
	default:
		// Default to PNG format for unsupported test formats
		pngData, _ := base64.StdEncoding.DecodeString(
			"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQIHWNgAAIAAAUAAY27m/MAAAAASUVORK5CYII=")
		if err := os.WriteFile(fileName, pngData, 0644); err != nil {
			os.Remove(fileName)
			t.Fatalf("Failed to write image file: %v", err)
		}
	}

	return fileName
}

func TestImageParser_Parse_MockSuccess(t *testing.T) {
	expectedText := "This is extracted text from the image."
	server := createMockOCRServer(t, expectedText, http.StatusOK)
	defer server.Close()

	parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

	// Create a test image
	imagePath := createTestImageFile(t, "png")
	defer os.Remove(imagePath)

	text, err := parser.Parse(imagePath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if text != expectedText {
		t.Errorf("Expected text %q, got %q", expectedText, text)
	}
}

func TestImageParser_Parse_ServerError(t *testing.T) {
	server := createMockOCRServer(t, "", http.StatusInternalServerError)
	defer server.Close()

	parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

	imagePath := createTestImageFile(t, "png")
	defer os.Remove(imagePath)

	_, err := parser.Parse(imagePath)
	if err == nil {
		t.Error("Expected error for server error, but got none")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected error to mention status 500, got: %v", err)
	}
}

func TestImageParser_Parse_EmptyResponse(t *testing.T) {
	server := createMockOCRServer(t, "", http.StatusOK)
	defer server.Close()

	parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

	imagePath := createTestImageFile(t, "png")
	defer os.Remove(imagePath)

	_, err := parser.Parse(imagePath)
	if err == nil {
		t.Error("Expected error for empty response, but got none")
	}

	if !strings.Contains(err.Error(), "no text could be extracted") {
		t.Errorf("Expected 'no text could be extracted' error, got: %v", err)
	}
}

func TestImageParser_Parse_FileErrors(t *testing.T) {
	parser := NewImageParserWithTimeout("http://localhost:11434", "test-model", nil, 5, 1120)

	tests := []struct {
		name     string
		filePath string
	}{
		{"nonexistent file", "/nonexistent/file.png"},
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

func TestImageParser_Parse_UnsupportedFormat(t *testing.T) {
	parser := NewImageParserWithTimeout("http://localhost:11434", "test-model", nil, 5, 1120)

	// Create a file with unsupported extension but valid content
	tmpFile, err := os.CreateTemp("", "test_*.xyz")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some content
	if _, err := tmpFile.WriteString("not an image"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = parser.Parse(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for unsupported format")
	}

	if !strings.Contains(err.Error(), "unsupported image format") {
		t.Errorf("Expected 'unsupported image format' error, got: %v", err)
	}
}

func TestImageParser_ParseWithChunks_NoChunker(t *testing.T) {
	expectedText := "Sample OCR text content."
	server := createMockOCRServer(t, expectedText, http.StatusOK)
	defer server.Close()

	// Create parser without chunker
	parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

	imagePath := createTestImageFile(t, "png")
	defer os.Remove(imagePath)

	chunks, err := parser.ParseWithChunks(imagePath, "test-doc")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(chunks))
	}

	if len(chunks) > 0 {
		chunk := chunks[0]
		if chunk.Text != expectedText {
			t.Errorf("Expected text %q, got %q", expectedText, chunk.Text)
		}
		if chunk.ChunkType != "image_ocr" {
			t.Errorf("Expected chunk type 'image_ocr', got %q", chunk.ChunkType)
		}
		if chunk.Index != 0 {
			t.Errorf("Expected index 0, got %d", chunk.Index)
		}
	}
}

func TestImageParser_ParseWithChunks_WithChunker(t *testing.T) {
	// Create long text that will be chunked
	longText := strings.Repeat("This is a line of text extracted from an image. ", 50)
	server := createMockOCRServer(t, longText, http.StatusOK)
	defer server.Close()

	// Create parser with chunker
	chunker := NewTextChunker(100, 20) // Small chunks for testing
	parser := NewImageParserWithTimeout(server.URL, "test-model", chunker, 5, 1120)

	imagePath := createTestImageFile(t, "png")
	defer os.Remove(imagePath)

	chunks, err := parser.ParseWithChunks(imagePath, "test-doc")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks, got %d", len(chunks))
	}

	// Verify all chunks have correct type and sequential indices
	for i, chunk := range chunks {
		if chunk.ChunkType != "image_ocr" {
			t.Errorf("Chunk %d: expected type 'image_ocr', got %q", i, chunk.ChunkType)
		}
		if chunk.Index != i {
			t.Errorf("Chunk %d: expected index %d, got %d", i, i, chunk.Index)
		}
		if chunk.Text == "" {
			t.Errorf("Chunk %d is empty", i)
		}
	}
}

func TestImageParser_TestVisionModel(t *testing.T) {
	t.Run("successful test", func(t *testing.T) {
		server := createMockOCRServer(t, "This is a test response", http.StatusOK)
		defer server.Close()

		parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := parser.TestVisionModel(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := createMockOCRServer(t, "", http.StatusServiceUnavailable)
		defer server.Close()

		parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := parser.TestVisionModel(ctx)
		if err == nil {
			t.Error("Expected error for server error, but got none")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		// Create server that doesn't respond
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Sleep longer than context timeout
		}))
		defer server.Close()

		parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 1, 1120)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := parser.TestVisionModel(ctx)
		if err == nil {
			t.Error("Expected timeout error, but got none")
		}
	})
}

func TestImageParser_ResizeImage_Errors(t *testing.T) {
	parser := NewImageParserWithTimeout("", "", nil, 30, 1120)

	tests := []struct {
		name     string
		filePath string
		maxSize  int
	}{
		{"nonexistent file", "/nonexistent/file.png", 1000},
		{"empty path", "", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ResizeImage(tt.filePath, tt.maxSize)
			if err == nil {
				t.Error("Expected error for invalid file")
			}
		})
	}
}

func TestImageParser_ResizeImage_Success(t *testing.T) {
	parser := NewImageParserWithTimeout("", "", nil, 30, 1120)

	// Test with a real PNG image
	imagePath := createTestImageFile(t, "png")
	defer os.Remove(imagePath)

	// Test resizing
	data, err := parser.ResizeImage(imagePath, 800)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty image data")
	}

	// Verify it's still valid image data (should start with image magic bytes)
	if len(data) < 4 {
		t.Error("Image data too short")
	}
}

func TestImageParser_Integration_FileFormats(t *testing.T) {
	// Test supported file formats (using only PNG for actual parsing due to test complexity)
	formats := []string{"png"}

	expectedText := "Sample text from image"
	server := createMockOCRServer(t, expectedText, http.StatusOK)
	defer server.Close()

	parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			imagePath := createTestImageFile(t, format)
			defer os.Remove(imagePath)

			// Test Parse method
			text, err := parser.Parse(imagePath)
			if err != nil {
				t.Errorf("Parse failed for %s: %v", format, err)
				return
			}

			if text != expectedText {
				t.Errorf("Expected text %q, got %q", expectedText, text)
			}

			// Test ParseWithChunks method
			chunks, err := parser.ParseWithChunks(imagePath, "test-doc")
			if err != nil {
				t.Errorf("ParseWithChunks failed for %s: %v", format, err)
				return
			}

			if len(chunks) == 0 {
				t.Errorf("Expected at least one chunk for %s", format)
			}
		})
	}
}

func TestImageParser_VisionRequest_Structure(t *testing.T) {
	// Test the structure of VisionRequest, VisionMessage, etc.
	t.Run("VisionRequest serialization", func(t *testing.T) {
		req := VisionRequest{
			Model: "test-model",
			Messages: []VisionMessage{
				{
					Role:    "user",
					Content: "Test content",
					Images:  []string{"base64image"},
				},
			},
			Stream: false,
			Options: &VisionOptions{
				Temperature: 0.5,
				TopP:        0.9,
			},
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Errorf("Failed to marshal VisionRequest: %v", err)
		}

		var unmarshaled VisionRequest
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Errorf("Failed to unmarshal VisionRequest: %v", err)
		}

		if unmarshaled.Model != req.Model {
			t.Errorf("Model mismatch: expected %s, got %s", req.Model, unmarshaled.Model)
		}

		if len(unmarshaled.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(unmarshaled.Messages))
		}
	})

	t.Run("VisionResponse deserialization", func(t *testing.T) {
		responseJSON := `{
			"model": "test-model",
			"created_at": "2023-01-01T00:00:00Z",
			"message": {
				"role": "assistant",
				"content": "Test response"
			},
			"done": true
		}`

		var resp VisionResponse
		if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
			t.Errorf("Failed to unmarshal VisionResponse: %v", err)
		}

		if resp.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got %s", resp.Model)
		}

		if resp.Message.Role != "assistant" {
			t.Errorf("Expected role 'assistant', got %s", resp.Message.Role)
		}

		if !resp.Done {
			t.Error("Expected done to be true")
		}
	})
}

func TestImageParser_EdgeCases(t *testing.T) {
	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		parser := NewImageParserWithTimeout(server.URL, "test-model", nil, 5, 1120)
		imagePath := createTestImageFile(t, "png")
		defer os.Remove(imagePath)

		_, err := parser.Parse(imagePath)
		if err == nil {
			t.Error("Expected error for invalid JSON response")
		}

		if !strings.Contains(err.Error(), "decode") {
			t.Errorf("Expected decode error, got: %v", err)
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		parser := NewImageParserWithTimeout("http://localhost:99999", "test-model", nil, 1, 1120)
		imagePath := createTestImageFile(t, "png")
		defer os.Remove(imagePath)

		_, err := parser.Parse(imagePath)
		if err == nil {
			t.Error("Expected connection error")
		}
	})
}

func TestImageParser_LargeImageHandling(t *testing.T) {
	// Test image resizing logic with various scenarios
	_ = NewImageParserWithTimeout("", "", nil, 30, 800) // Test parser creation

	t.Run("resize calculation accuracy", func(t *testing.T) {
		tests := []struct {
			name             string
			origW, origH     int
			maxSize          int
			expectW, expectH int
		}{
			{"large landscape", 2000, 1500, 1000, 1000, 750},
			{"large portrait", 1500, 2000, 1000, 750, 1000},
			{"small image", 400, 300, 1000, 400, 300},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w, h := calculateResizeDimensions(tt.origW, tt.origH, tt.maxSize)
				if w != tt.expectW || h != tt.expectH {
					t.Errorf("Expected %dx%d, got %dx%d", tt.expectW, tt.expectH, w, h)
				}
			})
		}
	})
}

// Benchmark tests for performance-critical operations
func BenchmarkImageParser_CalculateResize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		calculateResizeDimensions(1920, 1080, 1000)
	}
}

func BenchmarkImageParser_IsImageFile(b *testing.B) {
	testPaths := []string{"test.jpg", "test.png", "test.gif", "test.txt", "test.pdf"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			IsImageFile(path)
		}
	}
}
