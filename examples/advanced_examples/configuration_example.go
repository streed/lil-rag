//go:build ignore

package main

import (
	"context"
	"fmt"
	"strings"

	"lil-rag/pkg/config"
	"lil-rag/pkg/lilrag"
)

// This example demonstrates lil-rag's configuration management features:
// - Profile-based configuration system
// - Runtime configuration overrides
// - Different configuration templates (FastSearch, ContextualSearch, etc.)
// - Configuration validation and best practices
// - Performance tuning options
//
// The configuration system allows users to create reusable profiles and
// fine-tune lil-rag for different use cases and performance requirements.

func main() {
	fmt.Println("=== LilRag Configuration Management Example ===")
	fmt.Println("Demonstrating profile management and configuration options")
	fmt.Println()

	fmt.Println("1. Loading Current Profile Configuration")
	fmt.Println(strings.Repeat("-", 50))

	// Load the current profile configuration
	profileConfig, err := config.LoadProfile()
	if err != nil {
		fmt.Printf("❌ Failed to load profile: %v\n", err)
		fmt.Println("This usually means no profile has been initialized yet.")
		fmt.Println("Run 'lil-rag config init' to create a default profile.")
		return
	}

	// Display current configuration
	configPath, _ := config.GetProfileConfigPath()
	fmt.Printf("✅ Profile loaded from: %s\n", configPath)
	fmt.Println()

	fmt.Println("📋 Current Configuration:")
	fmt.Printf("  Storage Path: %s\n", profileConfig.StoragePath)
	fmt.Printf("  Data Directory: %s\n", profileConfig.DataDir)
	fmt.Println()
	fmt.Printf("  Ollama Endpoint: %s\n", profileConfig.Ollama.Endpoint)
	fmt.Printf("  Embedding Model: %s\n", profileConfig.Ollama.EmbeddingModel)
	fmt.Printf("  Chat Model: %s\n", profileConfig.Ollama.ChatModel)
	fmt.Printf("  Vision Model: %s\n", profileConfig.Ollama.VisionModel)
	fmt.Printf("  Vector Size: %d\n", profileConfig.Ollama.VectorSize)
	fmt.Printf("  Timeout: %d seconds\n", profileConfig.Ollama.TimeoutSeconds)
	fmt.Printf("  Image Max Size: %d pixels\n", profileConfig.Ollama.ImageMaxSize)
	fmt.Println()
	fmt.Printf("  Chunking Max Tokens: %d\n", profileConfig.Chunking.MaxTokens)
	fmt.Printf("  Chunking Overlap: %d\n", profileConfig.Chunking.Overlap)
	fmt.Println()
	fmt.Printf("  Server Host: %s\n", profileConfig.Server.Host)
	fmt.Printf("  Server Port: %d\n", profileConfig.Server.Port)
	fmt.Printf("  Server Secure: %t\n", profileConfig.Server.Secure)
	fmt.Printf("  Enable CORS: %t\n", profileConfig.Server.EnableCORS)
	fmt.Println()

	fmt.Println("2. Configuration Templates and Builder Patterns")
	fmt.Println(strings.Repeat("-", 50))

	// Demonstrate different configuration templates using the builder pattern
	fmt.Println("🏃 Fast Search Configuration Template:")
	fastBuilder := lilrag.ConfigurationTemplate{}.FastSearch()
	fastBuilder.WithDatabase("fast_search_example.db", 384).
		WithDataDir("./data_fast").
		WithOllama(profileConfig.Ollama.Endpoint, "all-MiniLM-L6-v2", 30).
		WithChunking(128, 19). // Smaller chunks for precise search
		WithMetrics(true)

	fmt.Printf("  Optimized for: Speed and precision\n")
	fmt.Printf("  Max Tokens: Small chunks (128) for fast processing\n")
	fmt.Printf("  Overlap: Minimal (19) for efficiency\n")
	fmt.Printf("  Model: Lightweight embedding model\n")
	fmt.Printf("  Use case: Large document collections, quick searches\n")
	fmt.Println()

	fmt.Println("🎯 Contextual Search Configuration Template:")
	contextualBuilder := lilrag.ConfigurationTemplate{}.ContextualSearch()
	contextualBuilder.WithDatabase("contextual_search_example.db", 768).
		WithDataDir("./data_contextual").
		WithOllama(profileConfig.Ollama.Endpoint, "nomic-embed-text", 30).
		WithChunking(512, 77). // Larger chunks for context preservation
		WithMetrics(true)

	fmt.Printf("  Optimized for: Context preservation and quality\n")
	fmt.Printf("  Max Tokens: Large chunks (512) for better context\n")
	fmt.Printf("  Overlap: Substantial (77) for continuity\n")
	fmt.Printf("  Model: High-quality embedding model\n")
	fmt.Printf("  Use case: Complex documents, detailed analysis\n")
	fmt.Println()

	fmt.Println("🔄 Legacy Compatible Configuration Template:")
	legacyBuilder := lilrag.ConfigurationTemplate{}.LegacyCompatible()
	legacyBuilder.WithDatabase("legacy_example.db", 768).
		WithDataDir("./data_legacy").
		WithOllama(profileConfig.Ollama.Endpoint, "nomic-embed-text", 30)

	fmt.Printf("  Optimized for: Compatibility with older setups\n")
	fmt.Printf("  Max Tokens: Standard chunks (256)\n")
	fmt.Printf("  Overlap: Balanced (38)\n")
	fmt.Printf("  Use case: Migration from older systems\n")
	fmt.Println()

	fmt.Println("3. Runtime Configuration Overrides")
	fmt.Println(strings.Repeat("-", 50))

	// Demonstrate creating a custom configuration by modifying the profile
	fmt.Println("🛠️  Creating Custom Configuration from Profile:")

	// Start with profile config and customize
	customConfig := &lilrag.Config{
		DatabasePath:   "custom_example.db",
		DataDir:        "./data_custom",
		OllamaURL:      profileConfig.Ollama.Endpoint,
		Model:          profileConfig.Ollama.EmbeddingModel,
		ChatModel:      profileConfig.Ollama.ChatModel,
		VisionModel:    profileConfig.Ollama.VisionModel,
		VectorSize:     profileConfig.Ollama.VectorSize,
		TimeoutSeconds: profileConfig.Ollama.TimeoutSeconds,
		MaxTokens:      384, // Custom chunk size
		Overlap:        57,  // Custom overlap (15% of MaxTokens)
		ImageMaxSize:   profileConfig.Ollama.ImageMaxSize,
	}

	fmt.Printf("  Database Path: %s (custom)\n", customConfig.DatabasePath)
	fmt.Printf("  Max Tokens: %d (custom - balanced size)\n", customConfig.MaxTokens)
	fmt.Printf("  Overlap: %d (custom - 15%% of max tokens)\n", customConfig.Overlap)
	fmt.Printf("  Model: %s (from profile)\n", customConfig.Model)
	fmt.Println()

	fmt.Println("4. Performance Optimization Examples")
	fmt.Println(strings.Repeat("-", 50))

	performanceConfigs := []struct {
		name        string
		description string
		config      *lilrag.Config
	}{
		{
			"🚀 Speed Optimized",
			"Minimal latency, good for real-time applications",
			&lilrag.Config{
				DatabasePath:   "speed_optimized.db",
				DataDir:        "./data_speed",
				OllamaURL:      profileConfig.Ollama.Endpoint,
				Model:          "all-MiniLM-L6-v2", // Fast, small model
				ChatModel:      "llama3.2:3b",      // Smaller chat model
				VectorSize:     384,                // Smaller vectors
				TimeoutSeconds: 15,                 // Shorter timeouts
				MaxTokens:      128,                // Small chunks
				Overlap:        19,                 // Minimal overlap
				ImageMaxSize:   800,                // Smaller images
			},
		},
		{
			"🎯 Quality Optimized",
			"Best accuracy, good for research and analysis",
			&lilrag.Config{
				DatabasePath:   "quality_optimized.db",
				DataDir:        "./data_quality",
				OllamaURL:      profileConfig.Ollama.Endpoint,
				Model:          "mxbai-embed-large", // High-quality model
				ChatModel:      "llama3.2:7b",       // Larger chat model
				VectorSize:     1024,                // Larger vectors
				TimeoutSeconds: 60,                  // Longer timeouts
				MaxTokens:      512,                 // Large chunks
				Overlap:        77,                  // Substantial overlap
				ImageMaxSize:   1600,                // High-res images
			},
		},
		{
			"⚖️ Balanced",
			"Good trade-off between speed and quality",
			&lilrag.Config{
				DatabasePath:   "balanced.db",
				DataDir:        "./data_balanced",
				OllamaURL:      profileConfig.Ollama.Endpoint,
				Model:          "nomic-embed-text", // Standard model
				ChatModel:      "llama3.2:3b",      // Standard chat model
				VectorSize:     768,                // Standard vectors
				TimeoutSeconds: 30,                 // Standard timeout
				MaxTokens:      256,                // Standard chunks
				Overlap:        38,                 // Standard overlap
				ImageMaxSize:   1120,               // Standard images
			},
		},
	}

	for _, pc := range performanceConfigs {
		fmt.Printf("%s\n", pc.name)
		fmt.Printf("  %s\n", pc.description)
		fmt.Printf("  Model: %s (Vector size: %d)\n", pc.config.Model, pc.config.VectorSize)
		fmt.Printf("  Chunking: %d tokens with %d overlap\n", pc.config.MaxTokens, pc.config.Overlap)
		fmt.Printf("  Timeout: %d seconds\n", pc.config.TimeoutSeconds)
		fmt.Printf("  Image size: %d pixels\n", pc.config.ImageMaxSize)
		fmt.Println()
	}

	fmt.Println("5. Configuration Validation and Testing")
	fmt.Println(strings.Repeat("-", 50))

	// Test one of the configurations
	fmt.Println("🧪 Testing Balanced Configuration:")

	balancedConfig := performanceConfigs[2].config // Balanced config

	rag, err := lilrag.New(balancedConfig)
	if err != nil {
		fmt.Printf("❌ Configuration validation failed: %v\n", err)
		return
	}
	defer rag.Close()

	if err := rag.Initialize(); err != nil {
		fmt.Printf("❌ Initialization failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Configuration validated successfully\n")
	fmt.Printf("✅ Database initialized: %s\n", balancedConfig.DatabasePath)

	ctx := context.Background()

	// Test basic functionality
	testDoc := "Configuration testing document. This demonstrates that the custom configuration is working properly with optimized settings for balanced performance."

	if err := rag.Index(ctx, testDoc, "config_test"); err != nil {
		fmt.Printf("❌ Indexing test failed: %v\n", err)
	} else {
		fmt.Printf("✅ Indexing test passed\n")

		// Test search
		results, err := rag.Search(ctx, "configuration testing", 1)
		if err != nil {
			fmt.Printf("❌ Search test failed: %v\n", err)
		} else if len(results) > 0 {
			fmt.Printf("✅ Search test passed (Score: %.4f)\n", results[0].Score)
		} else {
			fmt.Printf("⚠️  Search test returned no results\n")
		}
	}
	fmt.Println()

	fmt.Println("6. Configuration Best Practices")
	fmt.Println(strings.Repeat("-", 50))

	fmt.Println("📚 Configuration Guidelines:")
	fmt.Println()
	fmt.Println("🔧 Chunk Size (MaxTokens):")
	fmt.Println("  • 128-256: Fast searches, precise results")
	fmt.Println("  • 256-384: Balanced performance")
	fmt.Println("  • 384-512: Better context, slower searches")
	fmt.Println("  • 512+: Research/analysis workloads")
	fmt.Println()
	fmt.Println("🔗 Overlap:")
	fmt.Println("  • 10-15% of MaxTokens: Minimal redundancy")
	fmt.Println("  • 15-20% of MaxTokens: Good balance")
	fmt.Println("  • 20-25% of MaxTokens: Maximum context preservation")
	fmt.Println()
	fmt.Println("🤖 Model Selection:")
	fmt.Println("  • all-MiniLM-L6-v2: Fast, 384D vectors")
	fmt.Println("  • nomic-embed-text: Balanced, 768D vectors")
	fmt.Println("  • mxbai-embed-large: High quality, 1024D vectors")
	fmt.Println()
	fmt.Println("⏱️ Timeouts:")
	fmt.Println("  • 15-30s: Fast local GPU setups")
	fmt.Println("  • 30-60s: CPU inference or remote servers")
	fmt.Println("  • 60s+: Large models or slow hardware")
	fmt.Println()

	fmt.Println("=== Configuration Example Complete ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- Profile-based configuration management")
	fmt.Println("- Configuration templates and builder patterns")
	fmt.Println("- Runtime configuration overrides")
	fmt.Println("- Performance optimization strategies")
	fmt.Println("- Configuration validation and testing")
	fmt.Println("- Best practices for different use cases")
	fmt.Println()
	fmt.Println("💡 Configuration Management Commands:")
	fmt.Println("  lil-rag config init          # Initialize default profile")
	fmt.Println("  lil-rag config show          # Show current configuration")
	fmt.Println("  lil-rag config set key value # Set configuration values")
	fmt.Println()
	fmt.Println("📖 For detailed configuration options, see:")
	fmt.Println("  docs/CONFIGURATION.md")
}
