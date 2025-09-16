package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"lil-rag/pkg/lilrag"
)

// This example demonstrates lil-rag's multi-format document support:
// - PDF parsing and OCR
// - DOCX document processing
// - XLSX spreadsheet parsing
// - HTML content extraction
// - CSV data processing
// - Image OCR with vision models
// - Text file processing
//
// It showcases the different parsers and chunking strategies used for each format.

func main() {
	fmt.Println("=== LilRag Multi-Format Document Processing Example ===")
	fmt.Println()

	// Initialize LilRag with vision model support for image processing
	config := &lilrag.Config{
		DatabasePath:   "file_formats_example.db",
		DataDir:        "./data_formats",
		OllamaURL:      "http://localhost:11434",
		Model:          "nomic-embed-text", // Embedding model
		ChatModel:      "llama3.2:3b",      // Chat model for conversations
		VisionModel:    "llama3.2-vision",  // Vision model for image OCR
		VectorSize:     768,
		TimeoutSeconds: 30,
		MaxTokens:      256,  // Optimal chunk size for most content
		Overlap:        38,   // Smart overlap for context preservation
		ImageMaxSize:   1120, // Max image size for processing
	}

	rag, err := lilrag.New(config)
	if err != nil {
		log.Fatal("Failed to create LilRag:", err)
	}
	defer rag.Close()

	if err := rag.Initialize(); err != nil {
		log.Fatal("Failed to initialize LilRag:", err)
	}

	ctx := context.Background()

	// Test different file formats with sample content
	testFiles := []struct {
		filename string
		content  string
		docType  string
	}{
		// Test with actual files if they exist
		{"../../test_pdfs/test_document.pdf", "", "PDF Document"},
		{"../../test_images/sample_document.png", "", "Image with OCR"},
		{"../../test_images/tech_spec.png", "", "Technical Specification Image"},
	}

	// Also test with programmatically created content
	sampleDocuments := []struct {
		id      string
		content string
		docType string
		format  string
	}{
		{
			"html_doc",
			"<html><body><h1>Web Development Guide</h1><p>HTML, CSS, and JavaScript form the foundation of web development. <strong>HTML</strong> provides structure, <em>CSS</em> handles styling, and JavaScript adds interactivity.</p><ul><li>Semantic markup</li><li>Responsive design</li><li>Progressive enhancement</li></ul></body></html>",
			"HTML Document",
			"html",
		},
		{
			"csv_data",
			"Name,Language,Paradigm,Year\nGo,Systems,Concurrent,2009\nPython,General,Object-oriented,1991\nRust,Systems,Functional,2010\nJavaScript,Web,Prototype-based,1995\nTypeScript,Web,Object-oriented,2012",
			"CSV Data",
			"csv",
		},
		{
			"markdown_doc",
			"# Machine Learning Fundamentals\n\n## Key Concepts\n\n### Supervised Learning\n- **Classification**: Predicting categories\n- **Regression**: Predicting continuous values\n\n### Unsupervised Learning\n- **Clustering**: Finding hidden patterns\n- **Dimensionality Reduction**: Simplifying data\n\n### Deep Learning\nNeural networks with multiple layers that can learn complex representations from data.\n\n```python\nimport tensorflow as tf\nmodel = tf.keras.Sequential([\n    tf.keras.layers.Dense(128, activation='relu'),\n    tf.keras.layers.Dense(10, activation='softmax')\n])\n```",
			"Markdown Document",
			"markdown",
		},
	}

	fmt.Println("1. Testing File Format Processing")
	fmt.Println(strings.Repeat("-", 50))

	// Process actual files if they exist
	for _, testFile := range testFiles {
		if _, err := os.Stat(testFile.filename); err == nil {
			fmt.Printf("Processing %s: %s\n", testFile.docType, testFile.filename)

			docID := fmt.Sprintf("file_%s", strings.ReplaceAll(filepath.Base(testFile.filename), ".", "_"))

			if err := rag.IndexFile(ctx, testFile.filename, docID); err != nil {
				fmt.Printf("  ❌ Failed to index %s: %v\n", testFile.filename, err)
			} else {
				fmt.Printf("  ✅ Successfully indexed %s\n", filepath.Base(testFile.filename))

				// Show document info
				docs, err := rag.ListDocuments(ctx)
				if err == nil {
					for _, doc := range docs {
						if doc.ID == docID {
							fmt.Printf("     📄 Document ID: %s\n", doc.ID)
							fmt.Printf("     📊 Chunks: %d\n", doc.ChunkCount)
							fmt.Printf("     📏 Size: %d characters\n", len(doc.Text))
							if len(doc.Text) > 100 {
								fmt.Printf("     📝 Preview: %s...\n", doc.Text[:100])
							}
							break
						}
					}
				}
			}
		} else {
			fmt.Printf("Skipping %s (file not found): %s\n", testFile.docType, testFile.filename)
		}
		fmt.Println()
	}

	// Process sample documents with different content types
	fmt.Println("2. Processing Different Content Types")
	fmt.Println(strings.Repeat("-", 50))

	for _, doc := range sampleDocuments {
		fmt.Printf("Processing %s (%s format)\n", doc.docType, doc.format)

		if err := rag.Index(ctx, doc.content, doc.id); err != nil {
			fmt.Printf("  ❌ Failed to index %s: %v\n", doc.id, err)
		} else {
			fmt.Printf("  ✅ Successfully indexed %s\n", doc.id)
		}
		fmt.Println()
	}

	fmt.Println("3. Demonstrating Format-Specific Search")
	fmt.Println(strings.Repeat("-", 50))

	// Test searches that should find content from different formats
	searchQueries := []struct {
		query       string
		description string
	}{
		{"web development HTML CSS", "Web technologies (should find HTML content)"},
		{"programming languages comparison", "Language comparison (should find CSV data)"},
		{"machine learning neural networks", "ML concepts (should find Markdown content)"},
		{"document text content", "General document search (should find PDF/image content)"},
	}

	for _, sq := range searchQueries {
		fmt.Printf("Query: %s\n", sq.query)
		fmt.Printf("Context: %s\n", sq.description)

		results, err := rag.Search(ctx, sq.query, 3)
		if err != nil {
			fmt.Printf("  ❌ Search failed: %v\n", err)
		} else if len(results) == 0 {
			fmt.Printf("  ℹ️  No results found\n")
		} else {
			for i, result := range results {
				fmt.Printf("  %d. Document: %s (Score: %.4f)\n", i+1, result.ID, result.Score)
				preview := result.Text
				if len(preview) > 150 {
					preview = preview[:150] + "..."
				}
				fmt.Printf("     Content: %s\n", preview)
			}
		}
		fmt.Println()
	}

	fmt.Println("4. Document Management")
	fmt.Println(strings.Repeat("-", 50))

	// List all documents and show their formats
	docs, err := rag.ListDocuments(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to list documents: %v\n", err)
	} else {
		fmt.Printf("Total documents indexed: %d\n\n", len(docs))

		for _, doc := range docs {
			fmt.Printf("📄 Document: %s\n", doc.ID)
			fmt.Printf("   Chunks: %d\n", doc.ChunkCount)
			fmt.Printf("   Size: %d characters\n", len(doc.Text))

			// Determine likely format from content
			content := doc.Text
			var format string
			if strings.Contains(content, "<html>") || strings.Contains(content, "<body>") {
				format = "HTML"
			} else if strings.Contains(content, ",") && strings.Count(content, "\n") > 1 {
				format = "CSV-like"
			} else if strings.Contains(content, "```") || strings.Contains(content, "##") {
				format = "Markdown-like"
			} else {
				format = "Text/Unknown"
			}
			fmt.Printf("   Format: %s\n", format)
			fmt.Println()
		}
	}

	fmt.Println("5. Advanced Chunking Information")
	fmt.Println(strings.Repeat("-", 50))

	// Show chunking details for different content types
	if len(docs) > 0 {
		for _, doc := range docs[:3] { // Show first 3 documents
			chunks, err := rag.GetDocumentChunks(ctx, doc.ID)
			if err == nil && len(chunks) > 0 {
				fmt.Printf("Document: %s\n", doc.ID)
				fmt.Printf("Total chunks: %d\n", len(chunks))

				for i, chunk := range chunks {
					fmt.Printf("  Chunk %d: %d tokens, %d chars\n",
						i+1, chunk.TokenCount, len(chunk.Text))
					if i >= 2 { // Show only first 3 chunks
						if len(chunks) > 3 {
							fmt.Printf("  ... and %d more chunks\n", len(chunks)-3)
						}
						break
					}
				}
				fmt.Println()
			}
		}
	}

	fmt.Println("=== File Formats Example Complete ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- PDF document parsing and indexing")
	fmt.Println("- Image OCR with vision models")
	fmt.Println("- HTML content extraction and chunking")
	fmt.Println("- CSV data processing")
	fmt.Println("- Markdown document handling")
	fmt.Println("- Format-specific search capabilities")
	fmt.Println("- Document management and chunk inspection")
	fmt.Println("- Adaptive chunking strategies for different content types")
	fmt.Println()
	fmt.Printf("Database saved to: %s\n", config.DatabasePath)
	fmt.Printf("Content files stored in: %s\n", config.DataDir)
}
