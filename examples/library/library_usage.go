package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"lil-rag/pkg/lilrag"
)

func main() {
	// You can either load from profile config or create manually
	config := &lilrag.Config{
		DatabasePath: "example.db",
		DataDir:      "./data",
		OllamaURL:    "http://localhost:11434",
		Model:        "nomic-embed-text",
		VectorSize:   768,
	}

	rag, err := lilrag.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer rag.Close()

	if err := rag.Initialize(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	documents := []struct {
		id   string
		text string
	}{
		{"doc1", "Go is a programming language developed by Google. It's known for its simplicity and performance."},
		{"doc2", "Python is a high-level programming language known for its readability and versatility."},
		{"doc3", "JavaScript is the language of the web, running in browsers and servers via Node.js."},
		{"doc4", "Rust is a systems programming language focused on safety, speed, and concurrency."},
		{"doc5", "Machine learning is a subset of AI that enables computers to learn from data."},
	}

	fmt.Println("Indexing documents...")
	for _, doc := range documents {
		fmt.Printf("Indexing %s...\n", doc.id)
		if err := rag.Index(ctx, doc.text, doc.id); err != nil {
			log.Printf("Failed to index %s: %v", doc.id, err)
			continue
		}
	}

	queries := []string{
		"programming languages",
		"Google Go language",
		"artificial intelligence",
		"web development",
	}

	fmt.Println("\nSearching documents...")
	for _, query := range queries {
		fmt.Printf("\nQuery: %s\n", query)
		fmt.Println(strings.Repeat("-", 50))

		results, err := rag.Search(ctx, query, 3)
		if err != nil {
			log.Printf("Search failed: %v", err)
			continue
		}

		if len(results) == 0 {
			fmt.Println("No results found")
			continue
		}

		for i, result := range results {
			fmt.Printf("%d. ID: %s (Score: %.4f)\n", i+1, result.ID, result.Score)
			fmt.Printf("   %s\n", result.Text)
		}
	}
}
