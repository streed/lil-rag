package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"mini-rag/pkg/config"
	"mini-rag/pkg/minirag"
)

func main() {
	// Load configuration from user profile
	profileConfig, err := config.LoadProfile()
	if err != nil {
		log.Fatal(err)
	}

	// Convert to MiniRag config
	ragConfig := &minirag.Config{
		DatabasePath: profileConfig.StoragePath,
		DataDir:      profileConfig.DataDir,
		OllamaURL:    profileConfig.Ollama.Endpoint,
		Model:        profileConfig.Ollama.EmbeddingModel,
		VectorSize:   profileConfig.Ollama.VectorSize,
	}

	rag, err := minirag.New(ragConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer rag.Close()

	if err := rag.Initialize(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Show current configuration
	configPath, _ := config.GetProfileConfigPath()
	fmt.Printf("Using profile config: %s\n", configPath)
	fmt.Printf("Data directory: %s\n", profileConfig.DataDir)
	fmt.Printf("Ollama endpoint: %s\n", profileConfig.Ollama.Endpoint)
	fmt.Printf("Embedding model: %s\n", profileConfig.Ollama.EmbeddingModel)
	fmt.Printf("Vector size: %d\n\n", profileConfig.Ollama.VectorSize)

	// Index some documents
	documents := []struct {
		id   string
		text string
	}{
		{"tech1", "Go is a modern programming language created by Google with excellent concurrency support."},
		{"tech2", "Python is widely used for data science, web development, and artificial intelligence applications."},
		{"tech3", "Rust focuses on memory safety and performance, making it ideal for systems programming."},
		{"ai1", "Machine learning algorithms can automatically identify patterns in large datasets."},
		{"ai2", "Neural networks are inspired by biological neural systems and excel at pattern recognition."},
	}

	fmt.Println("Indexing documents...")
	for _, doc := range documents {
		fmt.Printf("Indexing %s...\n", doc.id)
		if err := rag.Index(ctx, doc.text, doc.id); err != nil {
			log.Printf("Failed to index %s: %v", doc.id, err)
			continue
		}
	}

	// Search for documents
	queries := []string{
		"programming languages",
		"artificial intelligence",
		"Google technology",
		"pattern recognition",
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
			text := result.Text
			if len(text) > 100 {
				text = text[:100] + "..."
			}
			fmt.Printf("   %s\n", text)
		}
	}

	fmt.Printf("\nContent files are stored in: %s\n", profileConfig.DataDir)
}
