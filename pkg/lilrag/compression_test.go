package lilrag

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressText(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "empty string",
			text:        "",
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "short text",
			text:        "hello",
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "medium text",
			text:        "This is a medium length text for compression testing",
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "long text",
			text:        strings.Repeat("This is a long text that should compress well. ", 100),
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "text with special characters",
			text:        "Hello, 世界! 🌍 Special chars: @#$%^&*()_+-={}[]|\\:;\"'<>?,./ ",
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "text with newlines",
			text:        "Line 1\nLine 2\nLine 3\n\nDouble newline",
			expectError: false,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := CompressText(tt.text)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectNil && compressed != nil {
				t.Error("Expected nil result for empty string")
			}
			if !tt.expectNil && compressed == nil && !tt.expectError {
				t.Error("Expected non-nil result")
			}

			// For non-empty text, compressed data should be smaller than original (usually)
			if !tt.expectNil && len(tt.text) > 100 && len(compressed) >= len(tt.text) {
				t.Logf("Note: Compressed size (%d) >= original size (%d) for text: %s",
					len(compressed), len(tt.text), tt.text[:minInt(50, len(tt.text))])
				// This is not necessarily an error for small texts
			}
		})
	}
}

func TestDecompressText(t *testing.T) {
	tests := []struct {
		name         string
		originalText string
		expectError  bool
	}{
		{
			name:         "empty string",
			originalText: "",
			expectError:  false,
		},
		{
			name:         "short text",
			originalText: "hello",
			expectError:  false,
		},
		{
			name:         "medium text",
			originalText: "This is a medium length text for compression testing",
			expectError:  false,
		},
		{
			name:         "long text",
			originalText: strings.Repeat("This is a long text that should compress well. ", 100),
			expectError:  false,
		},
		{
			name:         "text with special characters",
			originalText: "Hello, 世界! 🌍 Special chars: @#$%^&*()_+-={}[]|\\:;\"'<>?,./ ",
			expectError:  false,
		},
		{
			name:         "text with newlines",
			originalText: "Line 1\nLine 2\nLine 3\n\nDouble newline",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First compress the text
			compressed, err := CompressText(tt.originalText)
			if err != nil {
				t.Fatalf("Failed to compress text: %v", err)
			}

			// Then decompress it
			decompressed, err := DecompressText(compressed)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if decompressed != tt.originalText {
				t.Errorf("Decompressed text doesn't match original:\nOriginal:    %q\nDecompressed: %q",
					tt.originalText, decompressed)
			}
		})
	}
}

func TestDecompressText_InvalidData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "invalid gzip data",
			data: []byte{0x12, 0x34, 0x56, 0x78},
		},
		{
			name: "corrupted gzip header",
			data: []byte{0x1f, 0x8b, 0x08, 0x00}, // Partial gzip header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecompressText(tt.data)

			if tt.name == "empty data" {
				// Empty data should return empty string without error
				if err != nil {
					t.Errorf("Unexpected error for empty data: %v", err)
				}
				if result != "" {
					t.Errorf("Expected empty string for empty data, got %q", result)
				}
			} else if err == nil {
				// Invalid data should return error
				t.Error("Expected error for invalid data but got none")
			}
		})
	}
}

func TestCompressFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "compression_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:        "small file",
			content:     "Hello, world!",
			expectError: false,
		},
		{
			name:        "medium file",
			content:     strings.Repeat("This is a test line.\n", 50),
			expectError: false,
		},
		{
			name:        "large file",
			content:     strings.Repeat("This is a test line with more content to compress.\n", 1000),
			expectError: false,
		},
		{
			name:        "empty file",
			content:     "",
			expectError: false,
		},
		{
			name:        "file with unicode",
			content:     "Hello, 世界! 🌍 Unicode content test",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputPath := filepath.Join(tempDir, "input.txt")
			outputPath := filepath.Join(tempDir, "output.gz")

			// Create input file
			err := os.WriteFile(inputPath, []byte(tt.content), 0o644)
			if err != nil {
				t.Fatalf("Failed to create input file: %v", err)
			}

			// Compress the file
			err = CompressFile(inputPath, outputPath)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify output file exists and has content
				info, err := os.Stat(outputPath)
				if err != nil {
					t.Errorf("Output file doesn't exist: %v", err)
				} else if tt.content == "" && info.Size() > 50 { // Gzip has some overhead even for empty files
					t.Errorf("Expected small output file for empty input, got size %d", info.Size())
				}

				// Verify we can decompress it back
				decompressed, err := DecompressFile(outputPath)
				if err != nil {
					t.Errorf("Failed to decompress file: %v", err)
				} else if string(decompressed) != tt.content {
					t.Errorf("Decompressed content doesn't match original:\nOriginal:    %q\nDecompressed: %q",
						tt.content, string(decompressed))
				}
			}

			// Clean up files for next test
			os.Remove(inputPath)
			os.Remove(outputPath)
		})
	}
}

func TestCompressFile_InvalidPaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "compression_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name       string
		inputPath  string
		outputPath string
	}{
		{
			name:       "nonexistent input file",
			inputPath:  "/nonexistent/file.txt",
			outputPath: filepath.Join(tempDir, "output.gz"),
		},
		{
			name:       "invalid output directory",
			inputPath:  filepath.Join(tempDir, "input.txt"),
			outputPath: "/nonexistent/dir/output.gz",
		},
	}

	// Create valid input file for some tests
	validInput := filepath.Join(tempDir, "input.txt")
	if err := os.WriteFile(validInput, []byte("test content"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid output directory" {
				tt.inputPath = validInput // Use valid input for this test
			}

			err := CompressFile(tt.inputPath, tt.outputPath)
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

func TestDecompressFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "compression_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalContent := "This is test content for file decompression testing."

	// Create and compress a test file
	inputPath := filepath.Join(tempDir, "input.txt")
	compressedPath := filepath.Join(tempDir, "compressed.gz")

	err = os.WriteFile(inputPath, []byte(originalContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	err = CompressFile(inputPath, compressedPath)
	if err != nil {
		t.Fatalf("Failed to compress file: %v", err)
	}

	// Test decompression
	decompressed, err := DecompressFile(compressedPath)
	if err != nil {
		t.Errorf("Failed to decompress file: %v", err)
	}

	if string(decompressed) != originalContent {
		t.Errorf("Decompressed content doesn't match original:\nOriginal:    %q\nDecompressed: %q",
			originalContent, string(decompressed))
	}
}

func TestDecompressFile_InvalidFile(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "nonexistent file",
			path: "/nonexistent/file.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecompressFile(tt.path)
			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

func TestGetCompressionRatio(t *testing.T) {
	tests := []struct {
		name           string
		originalSize   int
		compressedSize int
		expected       float64
	}{
		{
			name:           "50% compression",
			originalSize:   100,
			compressedSize: 50,
			expected:       50.0,
		},
		{
			name:           "no compression",
			originalSize:   100,
			compressedSize: 100,
			expected:       100.0,
		},
		{
			name:           "expansion (negative compression)",
			originalSize:   100,
			compressedSize: 120,
			expected:       120.0,
		},
		{
			name:           "high compression",
			originalSize:   1000,
			compressedSize: 50,
			expected:       5.0,
		},
		{
			name:           "zero original size",
			originalSize:   0,
			compressedSize: 10,
			expected:       0.0,
		},
		{
			name:           "zero compressed size",
			originalSize:   100,
			compressedSize: 0,
			expected:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := GetCompressionRatio(tt.originalSize, tt.compressedSize)
			if ratio != tt.expected {
				t.Errorf("GetCompressionRatio(%d, %d) = %.2f, want %.2f",
					tt.originalSize, tt.compressedSize, ratio, tt.expected)
			}
		})
	}
}

// Integration test: compress and decompress various types of data
func TestCompressionRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "json data",
			data: `{"name": "John", "age": 30, "city": "New York", "hobbies": ["reading", "swimming", "coding"]}`,
		},
		{
			name: "code snippet",
			data: `
func main() {
    fmt.Println("Hello, world!")
    for i := 0; i < 10; i++ {
        fmt.Printf("Count: %d\n", i)
    }
}`,
		},
		{
			name: "repetitive data",
			data: strings.Repeat("This is a repeating pattern. ", 50),
		},
		{
			name: "mixed content",
			data: "Text with numbers 12345, symbols !@#$%, and unicode: 世界 🌍",
		},
		{
			name: "binary-like data",
			data: string(bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03}, 100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			compressed, err := CompressText(tt.data)
			if err != nil {
				t.Fatalf("Compression failed: %v", err)
			}

			// Decompress
			decompressed, err := DecompressText(compressed)
			if err != nil {
				t.Fatalf("Decompression failed: %v", err)
			}

			// Verify
			if decompressed != tt.data {
				t.Errorf("Round-trip failed:\nOriginal:     %q\nDecompressed: %q", tt.data, decompressed)
			}

			// Log compression ratio for interest
			originalSize := len(tt.data)
			compressedSize := len(compressed)
			ratio := GetCompressionRatio(originalSize, compressedSize)
			t.Logf("Compression ratio: %.1f%% (original: %d bytes, compressed: %d bytes)",
				ratio, originalSize, compressedSize)
		})
	}
}

// Benchmark tests
func BenchmarkCompressText_Small(b *testing.B) {
	text := "This is a small text for compression testing."
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := CompressText(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompressText_Medium(b *testing.B) {
	text := strings.Repeat("This is a medium text for compression testing. ", 100)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := CompressText(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompressText_Large(b *testing.B) {
	text := strings.Repeat("This is a large text for compression testing. ", 1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := CompressText(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecompressText_Small(b *testing.B) {
	text := "This is a small text for compression testing."
	compressed, err := CompressText(text)
	if err != nil {
		b.Fatalf("Failed to compress text: %v", err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := DecompressText(compressed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecompressText_Large(b *testing.B) {
	text := strings.Repeat("This is a large text for compression testing. ", 1000)
	compressed, err := CompressText(text)
	if err != nil {
		b.Fatalf("Failed to compress text: %v", err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := DecompressText(compressed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function for Go versions that don't have built-in min
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
