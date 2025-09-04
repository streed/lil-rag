package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lil-rag/pkg/lilrag"

	"github.com/gen2brain/go-fitz"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run compare_extractors.go <path_to_pdf>")
	}

	pdfPath := os.Args[1]
	if !filepath.IsAbs(pdfPath) {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v", err)
		}
		pdfPath = filepath.Join(wd, pdfPath)
	}

	fmt.Printf("Comparing PDF text extraction methods for: %s\n", pdfPath)
	fmt.Printf("=" + strings.Repeat("=", 60) + "\n\n")

	// Test 1: Current pdfcpu implementation
	fmt.Printf("1. CURRENT PDFCPU IMPLEMENTATION\n")
	fmt.Printf("-" + strings.Repeat("-", 32) + "\n")
	testPDFCPU(pdfPath)

	fmt.Printf("\n" + strings.Repeat("=", 70) + "\n\n")

	// Test 2: go-fitz (MuPDF) implementation
	fmt.Printf("2. GO-FITZ (MUPDF) IMPLEMENTATION\n")
	fmt.Printf("-" + strings.Repeat("-", 34) + "\n")
	testGoFitz(pdfPath)

	fmt.Printf("\n" + strings.Repeat("=", 70) + "\n\n")

	// Test 3: System pdftotext (if available)
	fmt.Printf("3. SYSTEM PDFTOTEXT (POPPLER) FALLBACK\n")
	fmt.Printf("-" + strings.Repeat("-", 39) + "\n")
	testPDFToText(pdfPath)
}

func testPDFCPU(pdfPath string) {
	start := time.Now()

	parser := lilrag.NewPDFParser()
	text, err := parser.Parse(pdfPath)

	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("Extraction time: %v\n", elapsed)
	fmt.Printf("Text length: %d characters\n", len(text))
	fmt.Printf("Word count: %d\n", len(strings.Fields(text)))

	// Show first 500 characters as sample
	sample := text
	if len(sample) > 500 {
		sample = sample[:500] + "..."
	}
	fmt.Printf("\nSample text:\n%s\n", sample)

	// Check for common artifacts
	artifacts := checkTextArtifacts(text)
	if len(artifacts) > 0 {
		fmt.Printf("\nPotential artifacts detected:\n")
		for _, artifact := range artifacts {
			fmt.Printf("- %s\n", artifact)
		}
	} else {
		fmt.Printf("\nNo obvious artifacts detected.\n")
	}
}

func testGoFitz(pdfPath string) {
	start := time.Now()

	doc, err := fitz.New(pdfPath)
	if err != nil {
		fmt.Printf("ERROR opening document: %v\n", err)
		return
	}
	defer doc.Close()

	var allText strings.Builder
	pageCount := doc.NumPage()

	for i := 0; i < pageCount; i++ {
		text, err := doc.Text(i)
		if err != nil {
			fmt.Printf("ERROR extracting text from page %d: %v\n", i+1, err)
			continue
		}
		allText.WriteString(text)
		allText.WriteString("\n\n")
	}

	elapsed := time.Since(start)
	finalText := strings.TrimSpace(allText.String())

	fmt.Printf("Extraction time: %v\n", elapsed)
	fmt.Printf("Pages processed: %d\n", pageCount)
	fmt.Printf("Text length: %d characters\n", len(finalText))
	fmt.Printf("Word count: %d\n", len(strings.Fields(finalText)))

	// Show first 500 characters as sample
	sample := finalText
	if len(sample) > 500 {
		sample = sample[:500] + "..."
	}
	fmt.Printf("\nSample text:\n%s\n", sample)

	// Check for common artifacts
	artifacts := checkTextArtifacts(finalText)
	if len(artifacts) > 0 {
		fmt.Printf("\nPotential artifacts detected:\n")
		for _, artifact := range artifacts {
			fmt.Printf("- %s\n", artifact)
		}
	} else {
		fmt.Printf("\nNo obvious artifacts detected.\n")
	}
}

func testPDFToText(pdfPath string) {
	// This is a fallback test using system pdftotext command
	// We'll just show what it would look like, but won't actually implement it here
	fmt.Printf("System pdftotext would be used as a fallback option.\n")
	fmt.Printf("This requires poppler-utils to be installed.\n")
	fmt.Printf("Command would be: pdftotext -layout '%s' -\n", pdfPath)
	fmt.Printf("This approach provides good text extraction but requires external dependency.\n")
}

func checkTextArtifacts(text string) []string {
	var artifacts []string

	// Check for spaced characters (like "T h i s")
	spacedPattern := 0
	for i, r := range text {
		if r == ' ' && i > 0 && i < len(text)-1 {
			if text[i-1] != ' ' && text[i+1] != ' ' {
				spacedPattern++
			}
		}
	}
	if spacedPattern > len(text)/20 { // If more than 5% of text is single spaced chars
		artifacts = append(artifacts, "Excessive character spacing detected (possible OCR artifact)")
	}

	// Check for lots of isolated single characters
	words := strings.Fields(text)
	singleChars := 0
	for _, word := range words {
		if len(word) == 1 && ((word[0] >= 'A' && word[0] <= 'Z') || (word[0] >= 'a' && word[0] <= 'z')) {
			singleChars++
		}
	}
	if len(words) > 0 && singleChars > len(words)/10 {
		artifacts = append(artifacts, "High number of isolated single characters")
	}

	// Check for missing spaces between words (like "wordword")
	concatenatedWords := 0
	for _, word := range words {
		if len(word) > 15 && strings.ToLower(word) == word {
			// Look for capital letters in the middle (indicating merged words)
			for i := 1; i < len(word)-1; i++ {
				if word[i] >= 'A' && word[i] <= 'Z' {
					concatenatedWords++
					break
				}
			}
		}
	}
	if concatenatedWords > 5 {
		artifacts = append(artifacts, "Possible concatenated words detected")
	}

	// Check for unicode/encoding issues
	nonPrintable := 0
	for _, r := range text {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Control characters except tab, newline, carriage return
			nonPrintable++
		}
	}
	if nonPrintable > 10 {
		artifacts = append(artifacts, "Non-printable control characters detected")
	}

	return artifacts
}
