package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"lil-rag/pkg/lilrag"
)

// This example demonstrates comprehensive library usage of lil-rag:
// - Manual configuration setup
// - Builder pattern with configuration templates
// - Custom parsers and chunking strategies
// - Document indexing and management
// - Advanced search capabilities
// - Chat functionality with RAG context
// - Error handling and best practices
//
// This shows how to integrate lil-rag directly into Go applications.

func main() {
	fmt.Println("=== LilRag Library Usage Example ===")
	fmt.Println("Demonstrating direct Go library integration")
	fmt.Println()

	fmt.Println("1. Manual Configuration Setup")
	fmt.Println(strings.Repeat("-", 50))

	// Method 1: Manual configuration
	config := &lilrag.Config{
		DatabasePath:   "library_example.db",
		DataDir:        "./data_library",
		OllamaURL:      "http://localhost:11434",
		Model:          "nomic-embed-text", // Embedding model
		ChatModel:      "llama3.2:3b",      // Chat model for conversations
		VisionModel:    "llama3.2-vision",  // Vision model for images
		VectorSize:     768,
		TimeoutSeconds: 30,
		MaxTokens:      256,  // Optimal chunk size
		Overlap:        38,   // 15% overlap for context
		ImageMaxSize:   1120, // Max image size for OCR
	}

	fmt.Printf("✅ Manual configuration created\n")
	fmt.Printf("   Database: %s\n", config.DatabasePath)
	fmt.Printf("   Embedding Model: %s\n", config.Model)
	fmt.Printf("   Chat Model: %s\n", config.ChatModel)
	fmt.Printf("   Chunk Size: %d tokens\n", config.MaxTokens)
	fmt.Println()

	fmt.Println("2. Builder Pattern Examples")
	fmt.Println(strings.Repeat("-", 50))

	// Method 2: Using builder pattern with templates
	fmt.Println("🏃 Fast Search Builder:")
	fastBuilder := lilrag.ConfigurationTemplate{}.FastSearch()
	fastBuilder.WithDatabase("fast_library_example.db", 384).
		WithDataDir("./data_fast").
		WithOllama("http://localhost:11434", "all-MiniLM-L6-v2", 30).
		WithChunking(128, 19). // Small chunks for speed
		WithMetrics(true)      // Enable metrics collection

	fmt.Printf("   Optimized for: Speed and precision\n")
	fmt.Printf("   Chunk size: 128 tokens (small for fast processing)\n")
	fmt.Printf("   Model: all-MiniLM-L6-v2 (lightweight)\n")
	fmt.Println()

	fmt.Println("🎯 Contextual Search Builder:")
	contextBuilder := lilrag.ConfigurationTemplate{}.ContextualSearch()
	contextBuilder.WithDatabase("contextual_library_example.db", 768).
		WithDataDir("./data_contextual").
		WithOllama("http://localhost:11434", "nomic-embed-text", 30).
		WithChunking(512, 77). // Large chunks for context
		WithMetrics(true)

	fmt.Printf("   Optimized for: Context preservation\n")
	fmt.Printf("   Chunk size: 512 tokens (large for better context)\n")
	fmt.Printf("   Model: nomic-embed-text (high quality)\n")
	fmt.Println()

	// Use the manual config for this example
	rag, err := lilrag.New(config)
	if err != nil {
		log.Fatal("Failed to create LilRag:", err)
	}

	if err := rag.Initialize(); err != nil {
		_ = rag.Close() // Ignore close errors before fatal exit
		log.Fatal("Failed to initialize LilRag:", err)
	}
	defer func() {
		_ = rag.Close() // Ignore close errors in defer
	}()

	ctx := context.Background()

	fmt.Println("3. Document Indexing and Management")
	fmt.Println(strings.Repeat("-", 50))

	// Enhanced document set with more variety
	documents := []struct {
		id       string
		text     string
		category string
	}{
		{
			"go_lang",
			"Go (Golang) is a programming language developed by Google. It's statically typed, compiled, and " +
				"designed for simplicity and efficiency. Key features include garbage collection, memory safety, " +
				"structural typing, and CSP-style concurrency with goroutines and channels.",
			"Programming Languages",
		},
		{
			"python_overview",
			"Python is a high-level, interpreted programming language known for its clear syntax and readability. " +
				"It supports multiple programming paradigms including procedural, object-oriented, and functional " +
				"programming. Python is widely used in web development, data science, artificial intelligence, and automation.",
			"Programming Languages",
		},
		{
			"javascript_ecosystem",
			"JavaScript is a dynamic programming language that's essential for web development. Originally designed for client-side scripting in browsers, it now runs on servers via Node.js. The ecosystem includes frameworks like React, Vue, and Angular for frontend development.",
			"Web Development",
		},
		{
			"rust_systems",
			"Rust is a systems programming language focused on safety, speed, and concurrency. It prevents segfaults and guarantees thread safety without using a garbage collector. Rust achieves memory safety through a unique ownership system that tracks object lifetimes.",
			"Systems Programming",
		},
		{
			"machine_learning_intro",
			"Machine learning is a subset of artificial intelligence that enables computers to learn and improve from experience without being explicitly programmed. It includes supervised learning (classification, regression), unsupervised learning (clustering), and reinforcement learning.",
			"Artificial Intelligence",
		},
		{
			"web_security",
			"Web application security involves protecting websites and web applications from various threats including SQL injection, cross-site scripting (XSS), cross-site request forgery (CSRF), and authentication vulnerabilities. Best practices include input validation, HTTPS, and secure session management.",
			"Security",
		},
	}

	fmt.Println("📚 Indexing Enhanced Document Set:")
	for _, doc := range documents {
		fmt.Printf("Indexing: %s (%s)\n", doc.id, doc.category)
		if err := rag.Index(ctx, doc.text, doc.id); err != nil {
			log.Printf("  ❌ Failed to index %s: %v", doc.id, err)
			continue
		}
		fmt.Printf("  ✅ Successfully indexed\n")
	}
	fmt.Println()

	fmt.Println("4. Advanced Search Capabilities")
	fmt.Println(strings.Repeat("-", 50))

	// Advanced search examples
	searchScenarios := []struct {
		query       string
		description string
		limit       int
	}{
		{
			"programming languages comparison",
			"Comparative search across programming languages",
			3,
		},
		{
			"memory safety systems programming",
			"Specific technical concept search",
			2,
		},
		{
			"web development security",
			"Cross-domain topic search",
			2,
		},
		{
			"Google technology artificial intelligence",
			"Multi-keyword semantic search",
			3,
		},
	}

	for _, scenario := range searchScenarios {
		fmt.Printf("🔍 %s\n", scenario.description)
		fmt.Printf("Query: \"%s\"\n", scenario.query)

		results, err := rag.Search(ctx, scenario.query, scenario.limit)
		if err != nil {
			fmt.Printf("❌ Search failed: %v\n", err)
			continue
		}

		if len(results) == 0 {
			fmt.Printf("ℹ️  No results found\n")
			continue
		}

		for i, result := range results {
			fmt.Printf("  %d. %s (Score: %.4f)\n", i+1, result.ID, result.Score)
			// Show preview of content
			preview := result.Text
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			fmt.Printf("     %s\n", preview)
		}
		fmt.Println()
	}

	fmt.Println("5. Document Management Operations")
	fmt.Println(strings.Repeat("-", 50))

	// Document management examples
	fmt.Println("📋 Document List:")
	docs, err := rag.ListDocuments(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to list documents: %v\n", err)
	} else {
		fmt.Printf("Total documents: %d\n", len(docs))
		for i, doc := range docs {
			fmt.Printf("  %d. %s (%d chunks)\n",
				i+1, doc.ID, doc.ChunkCount)
		}
	}
	fmt.Println()

	// Get specific document details
	if len(docs) > 0 {
		docID := docs[0].ID
		fmt.Printf("📄 Document Details for '%s':\n", docID)

		chunks, err := rag.GetDocumentChunks(ctx, docID)
		if err != nil {
			fmt.Printf("❌ Failed to get chunks: %v\n", err)
		} else {
			fmt.Printf("Chunks: %d\n", len(chunks))
			for i, chunk := range chunks {
				fmt.Printf("  Chunk %d: %d tokens\n",
					i+1, chunk.TokenCount)
				if i >= 2 { // Show only first 3 chunks
					if len(chunks) > 3 {
						fmt.Printf("  ... and %d more chunks\n", len(chunks)-3)
					}
					break
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("6. Chat Functionality with RAG")
	fmt.Println(strings.Repeat("-", 50))

	// Chat examples using the indexed knowledge
	chatQueries := []string{
		"What programming language is best for systems programming?",
		"How does Go handle concurrency?",
		"What are the main security concerns in web development?",
		"Compare Python and JavaScript for different use cases",
	}

	for i, query := range chatQueries {
		fmt.Printf("💬 Chat %d: %s\n", i+1, query)

		response, sources, err := rag.Chat(ctx, query, 3)
		if err != nil {
			fmt.Printf("❌ Chat failed: %v\n", err)
		} else {
			fmt.Printf("🤖 Response: %s\n", response)
			if len(sources) > 0 {
				fmt.Printf("📚 Sources: ")
				sourceIDs := make([]string, len(sources))
				for i, source := range sources {
					sourceIDs[i] = source.ID
				}
				fmt.Printf("%s\n", strings.Join(sourceIDs, ", "))
			}
		}
		fmt.Println()
	}

	fmt.Println("7. Error Handling and Best Practices")
	fmt.Println(strings.Repeat("-", 50))

	// Demonstrate error handling
	fmt.Println("🛡️  Error Handling Examples:")

	// Try to index duplicate content
	fmt.Printf("Testing duplicate indexing...\n")
	if err := rag.Index(ctx, "Duplicate content test", "go_lang"); err != nil {
		fmt.Printf("  ⚠️  Expected behavior: %v\n", err)
	} else {
		fmt.Printf("  ✅ Duplicate handled gracefully\n")
	}

	// Try to search with empty query
	fmt.Printf("Testing empty search query...\n")
	if results, err := rag.Search(ctx, "", 5); err != nil {
		fmt.Printf("  ⚠️  Expected behavior: %v\n", err)
	} else {
		fmt.Printf("  ℹ️  Empty query returned %d results\n", len(results))
	}

	// Try to get non-existent document
	fmt.Printf("Testing non-existent document retrieval...\n")
	if chunks, err := rag.GetDocumentChunks(ctx, "non_existent"); err != nil {
		fmt.Printf("  ⚠️  Expected behavior: %v\n", err)
	} else {
		fmt.Printf("  ℹ️  Non-existent document returned %d chunks\n", len(chunks))
	}
	fmt.Println()

	fmt.Println("8. Performance and Resource Management")
	fmt.Println(strings.Repeat("-", 50))

	// Show resource usage information
	fmt.Printf("📊 Resource Usage:\n")
	fmt.Printf("  Database: %s\n", config.DatabasePath)
	fmt.Printf("  Data Directory: %s\n", config.DataDir)
	fmt.Printf("  Total Documents: %d\n", len(docs))

	if len(docs) > 0 {
		totalChunks := 0
		for _, doc := range docs {
			totalChunks += doc.ChunkCount
		}
		fmt.Printf("  Total Chunks: %d\n", totalChunks)
		fmt.Printf("  Average Chunks per Document: %.1f\n", float64(totalChunks)/float64(len(docs)))
	}
	fmt.Println()

	// Cleanup example (delete a document)
	fmt.Println("🧹 Cleanup Example:")
	if len(docs) > 0 {
		deleteID := "machine_learning_intro"
		fmt.Printf("Deleting document: %s\n", deleteID)
		if err := rag.DeleteDocument(ctx, deleteID); err != nil {
			fmt.Printf("  ❌ Failed to delete: %v\n", err)
		} else {
			fmt.Printf("  ✅ Document deleted successfully\n")

			// Verify deletion
			newDocs, _ := rag.ListDocuments(ctx)
			fmt.Printf("  Documents after deletion: %d (was %d)\n", len(newDocs), len(docs))
		}
	}
	fmt.Println()

	fmt.Println("=== Library Usage Example Complete ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- Manual configuration vs builder patterns")
	fmt.Println("- Configuration templates for different use cases")
	fmt.Println("- Document indexing and management operations")
	fmt.Println("- Advanced search capabilities and scenarios")
	fmt.Println("- Chat functionality with RAG-powered responses")
	fmt.Println("- Proper error handling and edge cases")
	fmt.Println("- Resource management and cleanup")
	fmt.Println("- Performance monitoring and statistics")
	fmt.Println()
	fmt.Printf("Database saved to: %s\n", config.DatabasePath)
	fmt.Printf("Content files stored in: %s\n", config.DataDir)
	fmt.Println()
	fmt.Println("💡 Next steps:")
	fmt.Println("  - Try the HTTP API with lil-rag-server")
	fmt.Println("  - Explore MCP integration with lil-rag-mcp")
	fmt.Println("  - Check the advanced examples in ./advanced_examples/")
}
