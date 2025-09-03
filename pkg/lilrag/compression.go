package lilrag

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// CompressText compresses text using gzip and returns compressed bytes
func CompressText(text string) ([]byte, error) {
	if text == "" {
		return nil, nil
	}

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)

	_, err := gzipWriter.Write([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// DecompressText decompresses gzip-compressed bytes back to text
func DecompressText(compressedData []byte) (string, error) {
	if len(compressedData) == 0 {
		return "", nil
	}

	reader := bytes.NewReader(compressedData)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	var buf bytes.Buffer
	// #nosec G110 - Controlled decompression of trusted content
	_, err = io.Copy(&buf, gzipReader)
	if err != nil {
		return "", fmt.Errorf("failed to decompress data: %w", err)
	}

	return buf.String(), nil
}

// CompressFile compresses a file using gzip and saves it with .gz extension
func CompressFile(inputPath, outputPath string) error {
	// Read input file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Compress data
	compressed, err := CompressText(string(data))
	if err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}

	// Write compressed file
	err = os.WriteFile(outputPath, compressed, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write compressed file: %w", err)
	}

	return nil
}

// DecompressFile decompresses a gzip file
func DecompressFile(compressedPath string) ([]byte, error) {
	// Read compressed file
	compressedData, err := os.ReadFile(compressedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed file: %w", err)
	}

	// Decompress
	text, err := DecompressText(compressedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress file: %w", err)
	}

	return []byte(text), nil
}

// GetCompressionRatio calculates compression ratio as percentage
func GetCompressionRatio(originalSize, compressedSize int) float64 {
	if originalSize == 0 {
		return 0
	}
	return float64(compressedSize) / float64(originalSize) * 100
}
