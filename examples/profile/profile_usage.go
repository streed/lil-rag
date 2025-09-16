package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"lil-rag/pkg/config"
	"lil-rag/pkg/lilrag"
)

// This example demonstrates advanced profile-based configuration usage:
// - Loading and using profile configurations
// - Profile management and customization
// - Environment-specific configurations
// - Configuration validation and optimization
// - Profile-based application deployment
//
// Profiles provide a user-friendly way to manage complex configurations
// and make lil-rag easily configurable for different environments.

func main() {
	fmt.Println("=== LilRag Advanced Profile Usage Example ===")
	fmt.Println("Demonstrating profile-based configuration management")
	fmt.Println()

	fmt.Println("1. Profile Configuration Management")
	fmt.Println(strings.Repeat("-", 50))

	// Load configuration from user profile
	profileConfig, err := config.LoadProfile()
	if err != nil {
		fmt.Printf("❌ Failed to load profile: %v\n", err)
		fmt.Println()
		fmt.Println("💡 This usually means no profile has been initialized.")
		fmt.Println("   To create a default profile, run:")
		fmt.Println("   lil-rag config init")
		fmt.Println()
		fmt.Println("   Then try this example again.")
		return
	}

	// Show current profile information
	configPath, _ := config.GetProfileConfigPath()
	fmt.Printf("✅ Profile loaded successfully\n")
	fmt.Printf("📁 Configuration file: %s\n", configPath)
	fmt.Printf("📂 Data directory: %s\n", profileConfig.DataDir)
	fmt.Printf("🔗 Ollama endpoint: %s\n", profileConfig.Ollama.Endpoint)
	fmt.Printf("🤖 Embedding model: %s\n", profileConfig.Ollama.EmbeddingModel)
	fmt.Printf("💬 Chat model: %s\n", profileConfig.Ollama.ChatModel)
	fmt.Printf("👁️  Vision model: %s\n", profileConfig.Ollama.VisionModel)
	fmt.Printf("📏 Vector size: %d dimensions\n", profileConfig.Ollama.VectorSize)
	fmt.Println()

	fmt.Println("2. Converting Profile to LilRag Configuration")
	fmt.Println(strings.Repeat("-", 50))

	// Convert profile to LilRag config with custom database name
	ragConfig := &lilrag.Config{
		DatabasePath:   "profile_example.db", // Custom database for this example
		DataDir:        profileConfig.DataDir,
		OllamaURL:      profileConfig.Ollama.Endpoint,
		Model:          profileConfig.Ollama.EmbeddingModel,
		ChatModel:      profileConfig.Ollama.ChatModel,
		VisionModel:    profileConfig.Ollama.VisionModel,
		TimeoutSeconds: profileConfig.Ollama.TimeoutSeconds,
		VectorSize:     profileConfig.Ollama.VectorSize,
		MaxTokens:      profileConfig.Chunking.MaxTokens,
		Overlap:        profileConfig.Chunking.Overlap,
		ImageMaxSize:   profileConfig.Ollama.ImageMaxSize,
	}

	fmt.Printf("✅ Profile converted to LilRag configuration\n")
	fmt.Printf("📦 Database: %s\n", ragConfig.DatabasePath)
	fmt.Printf("🔧 Chunking: %d tokens with %d overlap\n", ragConfig.MaxTokens, ragConfig.Overlap)
	fmt.Printf("⏱️  Timeout: %d seconds\n", ragConfig.TimeoutSeconds)
	fmt.Printf("🖼️  Image size limit: %d pixels\n", ragConfig.ImageMaxSize)
	fmt.Println()

	rag, err := lilrag.New(ragConfig)
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

	fmt.Println("3. Profile-Optimized Document Processing")
	fmt.Println(strings.Repeat("-", 50))

	// Use profile settings to process different types of content
	documents := []struct {
		id       string
		text     string
		source   string
		metadata map[string]string
	}{
		{
			"profile_tech_doc",
			"Technical Documentation: This document describes the architecture of our microservices platform. We use Go for backend services, React for frontend applications, and PostgreSQL for data persistence. The system is deployed on Kubernetes with automated CI/CD pipelines.",
			"Internal Wiki",
			map[string]string{
				"type":       "technical",
				"department": "engineering",
				"priority":   "high",
			},
		},
		{
			"profile_meeting_notes",
			"Team Meeting Notes (2024-01-15): Discussed Q1 objectives, reviewed sprint progress, and planned upcoming feature releases. Action items include updating documentation, optimizing search performance, and scheduling security reviews. Next meeting: January 22nd.",
			"Meeting Minutes",
			map[string]string{
				"type":      "meeting",
				"date":      "2024-01-15",
				"attendees": "12",
			},
		},
		{
			"profile_company_policy",
			"Remote Work Policy: Employees may work remotely up to 3 days per week. Requirements include stable internet connection, secure workspace, and attendance at mandatory in-person meetings. Equipment is provided including laptop, monitor, and ergonomic accessories.",
			"HR Policies",
			map[string]string{
				"type":       "policy",
				"effective":  "2024-01-01",
				"department": "human-resources",
			},
		},
		{
			"profile_project_update",
			"Project Alpha Update: Backend API development is 85% complete. Frontend components are being tested. Database migration scheduled for next week. Expected launch date moved to March 15th due to additional security requirements. Team morale is high despite timeline changes.",
			"Project Management",
			map[string]string{
				"type":     "project",
				"status":   "in-progress",
				"priority": "high",
			},
		},
	}

	fmt.Printf("📚 Indexing documents using profile settings...\n")
	for _, doc := range documents {
		fmt.Printf("Processing: %s (%s)\n", doc.id, doc.source)

		// The profile's chunking settings will be used automatically
		if err := rag.Index(ctx, doc.text, doc.id); err != nil {
			fmt.Printf("  ❌ Failed to index %s: %v\n", doc.id, err)
		} else {
			fmt.Printf("  ✅ Successfully indexed using profile chunk settings\n")
			fmt.Printf("     Chunk size: %d tokens, Overlap: %d tokens\n",
				profileConfig.Chunking.MaxTokens, profileConfig.Chunking.Overlap)
		}
		fmt.Println()
	}

	fmt.Println("4. Profile-Based Search Optimization")
	fmt.Println(strings.Repeat("-", 50))

	// Search examples that benefit from profile-optimized settings
	searchScenarios := []struct {
		query       string
		description string
		context     string
	}{
		{
			"microservices architecture deployment",
			"Technical architecture search",
			"Looking for technical implementation details",
		},
		{
			"team meeting action items Q1",
			"Meeting content search",
			"Finding specific meeting outcomes and tasks",
		},
		{
			"remote work policy requirements",
			"Policy information search",
			"Searching for HR policy details",
		},
		{
			"project timeline security requirements",
			"Project status search",
			"Finding project updates and constraints",
		},
	}

	for _, scenario := range searchScenarios {
		fmt.Printf("🔍 %s\n", scenario.description)
		fmt.Printf("Query: \"%s\"\n", scenario.query)
		fmt.Printf("Context: %s\n", scenario.context)

		results, err := rag.Search(ctx, scenario.query, 2)
		switch {
		case err != nil:
			fmt.Printf("❌ Search failed: %v\n", err)
		case len(results) == 0:
			fmt.Printf("ℹ️  No results found\n")
		default:
			for i, result := range results {
				fmt.Printf("  %d. %s (Score: %.4f)\n", i+1, result.ID, result.Score)
				preview := result.Text
				if len(preview) > 150 {
					preview = preview[:150] + "..."
				}
				fmt.Printf("     %s\n", preview)
			}
		}
		fmt.Println()
	}

	fmt.Println("5. Profile-Based Chat Integration")
	fmt.Println(strings.Repeat("-", 50))

	// Chat examples using profile-configured chat model
	fmt.Printf("💬 Using chat model: %s\n", profileConfig.Ollama.ChatModel)
	fmt.Printf("⏱️  Chat timeout: %d seconds (4x base timeout)\n", profileConfig.Ollama.TimeoutSeconds*4)
	fmt.Println()

	chatQueries := []struct {
		question string
		purpose  string
	}{
		{
			"What is our current remote work policy?",
			"Policy inquiry using company documents",
		},
		{
			"What technologies are we using for our microservices platform?",
			"Technical architecture question",
		},
		{
			"What are the recent action items from team meetings?",
			"Meeting follow-up inquiry",
		},
	}

	for i, cq := range chatQueries {
		fmt.Printf("💭 Chat %d: %s\n", i+1, cq.purpose)
		fmt.Printf("❓ Question: %s\n", cq.question)

		response, sources, err := rag.Chat(ctx, cq.question, 3)
		if err != nil {
			fmt.Printf("❌ Chat failed: %v\n", err)
		} else {
			fmt.Printf("🤖 Response: %s\n", response)
			if len(sources) > 0 {
				fmt.Printf("📚 Sources: ")
				sourceNames := make([]string, len(sources))
				for j, source := range sources {
					sourceNames[j] = source.ID
				}
				fmt.Printf("%s\n", strings.Join(sourceNames, ", "))
			}
		}
		fmt.Println()
	}

	fmt.Println("6. File Processing with Profile Settings")
	fmt.Println(strings.Repeat("-", 50))

	// Test file processing if sample files exist
	testFiles := []struct {
		path        string
		description string
		docID       string
	}{
		{"../../test_pdfs/test_document.pdf", "PDF Document", "profile_pdf_test"},
		{"../../test_images/sample_document.png", "Image with OCR", "profile_image_test"},
	}

	fmt.Printf("🗂️  Using profile settings for file processing:\n")
	fmt.Printf("   Vision model: %s\n", profileConfig.Ollama.VisionModel)
	fmt.Printf("   Vision timeout: %d seconds (10x base timeout)\n", profileConfig.Ollama.TimeoutSeconds*10)
	fmt.Printf("   Image max size: %d pixels\n", profileConfig.Ollama.ImageMaxSize)
	fmt.Println()

	for _, tf := range testFiles {
		fmt.Printf("Testing: %s\n", tf.description)
		fmt.Printf("Path: %s\n", tf.path)

		// Check if file exists
		if err := rag.IndexFile(ctx, tf.path, tf.docID); err != nil {
			fmt.Printf("  ⚠️  File not found or processing failed: %v\n", err)
		} else {
			fmt.Printf("  ✅ File processed successfully using profile settings\n")

			// Test search for the file content
			results, err := rag.Search(ctx, "document content", 1)
			if err == nil && len(results) > 0 {
				for _, result := range results {
					if strings.Contains(result.ID, "profile_") {
						fmt.Printf("     Search result: %s (Score: %.4f)\n", result.ID, result.Score)
						break
					}
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("7. Profile Statistics and Management")
	fmt.Println(strings.Repeat("-", 50))

	// Show statistics about the profile-based setup
	docs, err := rag.ListDocuments(ctx)
	if err == nil {
		fmt.Printf("📊 Profile-Based Setup Statistics:\n")
		fmt.Printf("   Documents indexed: %d\n", len(docs))

		totalChunks := 0
		for _, doc := range docs {
			totalChunks += doc.ChunkCount
		}

		fmt.Printf("   Total chunks: %d\n", totalChunks)
		fmt.Printf("   Chunks per document: %.1f\n", float64(totalChunks)/float64(len(docs)))
		fmt.Println()

		fmt.Printf("📁 Storage Information:\n")
		fmt.Printf("   Database: %s\n", ragConfig.DatabasePath)
		fmt.Printf("   Data directory: %s\n", ragConfig.DataDir)
		fmt.Printf("   Profile config: %s\n", configPath)
	}
	fmt.Println()

	fmt.Println("8. Profile Optimization Tips")
	fmt.Println(strings.Repeat("-", 50))

	fmt.Println("🎯 Profile Configuration Recommendations:")
	fmt.Println()

	// Analyze current profile settings and provide recommendations
	currentChunkSize := profileConfig.Chunking.MaxTokens
	currentOverlap := profileConfig.Chunking.Overlap
	overlapPercentage := float64(currentOverlap) / float64(currentChunkSize) * 100

	fmt.Printf("Current Settings Analysis:\n")
	fmt.Printf("  Chunk size: %d tokens ", currentChunkSize)
	switch {
	case currentChunkSize < 200:
		fmt.Printf("(⚡ Fast searches, precise results)\n")
	case currentChunkSize < 400:
		fmt.Printf("(⚖️  Balanced performance)\n")
	default:
		fmt.Printf("(🎯 Better context, slower searches)\n")
	}

	fmt.Printf("  Overlap: %d tokens (%.1f%%) ", currentOverlap, overlapPercentage)
	switch {
	case overlapPercentage < 10:
		fmt.Printf("(⚡ Minimal redundancy)\n")
	case overlapPercentage < 20:
		fmt.Printf("(⚖️  Good balance)\n")
	default:
		fmt.Printf("(🎯 Maximum context preservation)\n")
	}

	fmt.Printf("  Vector size: %d dimensions ", profileConfig.Ollama.VectorSize)
	switch {
	case profileConfig.Ollama.VectorSize <= 384:
		fmt.Printf("(⚡ Fast, compact)\n")
	case profileConfig.Ollama.VectorSize <= 768:
		fmt.Printf("(⚖️  Standard quality)\n")
	default:
		fmt.Printf("(🎯 High precision)\n")
	}
	fmt.Println()

	fmt.Println("💡 Optimization Suggestions:")
	fmt.Println("  • For speed: lil-rag config set chunking.max-tokens 128")
	fmt.Println("  • For quality: lil-rag config set chunking.max-tokens 512")
	fmt.Println("  • For balance: lil-rag config set chunking.max-tokens 256")
	fmt.Println("  • Adjust overlap to 15% of max-tokens for optimal performance")
	fmt.Println()

	fmt.Println("=== Profile Usage Example Complete ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- Loading and using profile-based configurations")
	fmt.Println("- Converting profiles to LilRag configurations")
	fmt.Println("- Profile-optimized document processing")
	fmt.Println("- Search and chat with profile settings")
	fmt.Println("- File processing with vision model settings")
	fmt.Println("- Profile statistics and optimization analysis")
	fmt.Println("- Configuration recommendations and tuning")
	fmt.Println()
	fmt.Printf("Profile configuration: %s\n", configPath)
	fmt.Printf("Example database: %s\n", ragConfig.DatabasePath)
	fmt.Printf("Data directory: %s\n", profileConfig.DataDir)
	fmt.Println()
	fmt.Println("🔧 Profile Management Commands:")
	fmt.Println("  lil-rag config show                 # Display current profile")
	fmt.Println("  lil-rag config set key value        # Update configuration")
	fmt.Println("  lil-rag config set ollama.model X   # Change embedding model")
	fmt.Println("  lil-rag config set chunking.max-tokens N  # Adjust chunk size")
}
