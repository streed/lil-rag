package lilrag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"lil-rag/pkg/config"
)

// Default configuration constants
const (
	DefaultDataDir      = "data"
	DefaultDatabaseName = "lilrag.db"
	DefaultChatModel    = "gemma3:4b"
)

// Builder provides a fluent interface for building LilRag instances
type Builder struct {
	config        *ServiceConfig
	withVision    bool
	withMetrics   bool
	customParsers []Parser
}

// NewBuilder creates a new LilRag builder
func NewBuilder() *Builder {
	return &Builder{
		config:      &ServiceConfig{},
		withVision:  false,
		withMetrics: true,
	}
}

// WithConfig sets the service configuration
func (b *Builder) WithConfig(cfg *ServiceConfig) *Builder {
	b.config = cfg
	return b
}

// WithProfileConfig loads configuration from a profile config
func (b *Builder) WithProfileConfig(profileConfig *config.Config, dataDir string) *Builder {
	b.config = &ServiceConfig{
		DatabasePath:   profileConfig.Database.Path,
		DataDir:        dataDir,
		OllamaURL:      profileConfig.Ollama.URL,
		Model:          profileConfig.Ollama.Model,
		ChatModel:      DefaultChatModel, // Default chat model
		VisionModel:    profileConfig.Ollama.VisionModel,
		TimeoutSeconds: profileConfig.Ollama.TimeoutSeconds,
		VectorSize:     profileConfig.Database.VectorSize,
		MaxTokens:      profileConfig.Chunking.MaxTokens,
		Overlap:        profileConfig.Chunking.Overlap,
		ImageMaxSize:   profileConfig.Ollama.ImageMaxSize,
	}
	return b
}

// WithDatabase configures database settings
func (b *Builder) WithDatabase(path string, vectorSize int) *Builder {
	b.config.DatabasePath = path
	b.config.VectorSize = vectorSize
	return b
}

// WithDataDir sets the data directory
func (b *Builder) WithDataDir(dataDir string) *Builder {
	b.config.DataDir = dataDir
	return b
}

// WithOllama configures Ollama settings
func (b *Builder) WithOllama(url, model string, timeoutSeconds int) *Builder {
	b.config.OllamaURL = url
	b.config.Model = model
	b.config.TimeoutSeconds = timeoutSeconds
	return b
}

// WithChatModel sets the chat model
func (b *Builder) WithChatModel(chatModel string) *Builder {
	b.config.ChatModel = chatModel
	return b
}

// WithVision enables vision model support
func (b *Builder) WithVision(visionModel string, imageMaxSize int) *Builder {
	b.config.VisionModel = visionModel
	b.config.ImageMaxSize = imageMaxSize
	b.withVision = true
	return b
}

// WithChunking configures text chunking parameters
func (b *Builder) WithChunking(maxTokens, overlap int) *Builder {
	b.config.MaxTokens = maxTokens
	b.config.Overlap = overlap
	return b
}

// WithCustomParser adds a custom parser
func (b *Builder) WithCustomParser(parser Parser) *Builder {
	b.customParsers = append(b.customParsers, parser)
	return b
}

// WithMetrics enables/disables metrics collection
func (b *Builder) WithMetrics(enabled bool) *Builder {
	b.withMetrics = enabled
	return b
}

// applyDefaults applies default values for unset configuration
func (b *Builder) applyDefaults() {
	if b.config.DatabasePath == "" {
		b.config.DatabasePath = DefaultDatabaseName
	}
	if b.config.DataDir == "" {
		b.config.DataDir = DefaultDataDir
	}
	if b.config.OllamaURL == "" {
		b.config.OllamaURL = DefaultOllamaURL
	}
	if b.config.Model == "" {
		b.config.Model = DefaultChatModel
	}
	if b.config.ChatModel == "" {
		b.config.ChatModel = DefaultChatModel
	}
	if b.config.VisionModel == "" {
		b.config.VisionModel = "llama3.2-vision"
	}
	if b.config.TimeoutSeconds == 0 {
		b.config.TimeoutSeconds = 30
	}
	if b.config.VectorSize == 0 {
		b.config.VectorSize = 768
	}
	if b.config.MaxTokens == 0 {
		b.config.MaxTokens = 256 // Modern RAG best practice
	}
	if b.config.Overlap == 0 {
		b.config.Overlap = 38 // 15% of MaxTokens
	}
	if b.config.ImageMaxSize == 0 {
		b.config.ImageMaxSize = 1120
	}
}

// Build creates and initializes a new LilRag instance
func (b *Builder) Build(ctx context.Context) (*LilRag, error) {
	// Apply defaults
	b.applyDefaults()

	// Validate configuration
	if err := b.validateConfig(); err != nil {
		return nil, NewConfigError("invalid configuration", err)
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(b.config.DataDir, 0o755); err != nil {
		return nil, NewConfigError("failed to create data directory", err)
	}

	// Create services using factory
	factory := NewServiceFactory(b.config)
	services, err := factory.CreateServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create services: %w", err)
	}

	// Create legacy LilRag instance for backward compatibility
	lilrag := &LilRag{
		config: &Config{
			DatabasePath:   b.config.DatabasePath,
			DataDir:        b.config.DataDir,
			OllamaURL:      b.config.OllamaURL,
			Model:          b.config.Model,
			ChatModel:      b.config.ChatModel,
			VisionModel:    b.config.VisionModel,
			TimeoutSeconds: b.config.TimeoutSeconds,
			VectorSize:     b.config.VectorSize,
			MaxTokens:      b.config.MaxTokens,
			Overlap:        b.config.Overlap,
			ImageMaxSize:   b.config.ImageMaxSize,
		},
		// Store services for modern access
		services: services,
	}

	// Initialize legacy components for backward compatibility
	if err := lilrag.initializeLegacyComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize legacy components: %w", err)
	}

	return lilrag, nil
}

// BuildServices creates only the services without legacy wrapper
func (b *Builder) BuildServices(ctx context.Context) (*Services, error) {
	// Apply defaults
	b.applyDefaults()

	// Validate configuration
	if err := b.validateConfig(); err != nil {
		return nil, NewConfigError("invalid configuration", err)
	}

	// Create services using factory
	factory := NewServiceFactory(b.config)
	return factory.CreateServices(ctx)
}

// validateConfig validates the builder configuration
func (b *Builder) validateConfig() error {
	if b.config.DatabasePath == "" {
		return fmt.Errorf("database path is required")
	}
	if b.config.OllamaURL == "" {
		return fmt.Errorf("ollama URL is required")
	}
	if b.config.Model == "" {
		return fmt.Errorf("embedding model is required")
	}
	if b.config.VectorSize <= 0 {
		return fmt.Errorf("vector size must be positive")
	}
	if b.config.MaxTokens <= 0 {
		return fmt.Errorf("max tokens must be positive")
	}
	if b.config.Overlap < 0 {
		return fmt.Errorf("overlap cannot be negative")
	}
	if b.config.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

// ConfigurationTemplate provides pre-configured builder templates
type ConfigurationTemplate struct{}

// FastSearch creates a builder optimized for fast search with smaller chunks
func (ConfigurationTemplate) FastSearch() *Builder {
	return NewBuilder().
		WithChunking(128, 19).
		WithOllama("http://localhost:11434", "nomic-embed-text", 15)
}

// ContextualSearch creates a builder optimized for preserving context with larger chunks
func (ConfigurationTemplate) ContextualSearch() *Builder {
	return NewBuilder().
		WithChunking(512, 76).
		WithOllama("http://localhost:11434", "nomic-embed-text", 45)
}

// LegacyCompatible creates a builder with legacy chunking settings
func (ConfigurationTemplate) LegacyCompatible() *Builder {
	return NewBuilder().
		WithChunking(1800, 200).
		WithOllama("http://localhost:11434", "nomic-embed-text", 30)
}

// ProductionReady creates a builder with production-optimized settings
func (ConfigurationTemplate) ProductionReady(dataDir string) *Builder {
	return NewBuilder().
		WithDataDir(dataDir).
		WithDatabase(filepath.Join(dataDir, "lilrag.db"), 768).
		WithChunking(256, 38).
		WithOllama("http://localhost:11434", "nomic-embed-text", 30).
		WithVision("llama3.2-vision", 1120).
		WithMetrics(true)
}

// Development creates a builder for development with relaxed timeouts
func (ConfigurationTemplate) Development() *Builder {
	return NewBuilder().
		WithDatabase("dev.db", 768).
		WithChunking(256, 38).
		WithOllama("http://localhost:11434", "nomic-embed-text", 60).
		WithVision("llama3.2-vision", 1120)
}

// ConfigFactory provides factory methods for common configurations
type ConfigFactory struct{}

// FromProfile creates a service config from a profile configuration
func (ConfigFactory) FromProfile(profileConfig *config.Config, dataDir string) *ServiceConfig {
	chatModel := DefaultChatModel
	if profileConfig.Ollama.Model != "" {
		// Use embedding model as chat model if no separate chat model is specified
		chatModel = profileConfig.Ollama.Model
	}

	return &ServiceConfig{
		DatabasePath:   profileConfig.Database.Path,
		DataDir:        dataDir,
		OllamaURL:      profileConfig.Ollama.URL,
		Model:          profileConfig.Ollama.Model,
		ChatModel:      chatModel,
		VisionModel:    profileConfig.Ollama.VisionModel,
		TimeoutSeconds: profileConfig.Ollama.TimeoutSeconds,
		VectorSize:     profileConfig.Database.VectorSize,
		MaxTokens:      profileConfig.Chunking.MaxTokens,
		Overlap:        profileConfig.Chunking.Overlap,
		ImageMaxSize:   profileConfig.Ollama.ImageMaxSize,
	}
}

// FromEnvironment creates a service config from environment variables
func (ConfigFactory) FromEnvironment() *ServiceConfig {
	return &ServiceConfig{
		DatabasePath:   getEnvOrDefault("LILRAG_DB_PATH", DefaultDatabaseName),
		DataDir:        getEnvOrDefault("LILRAG_DATA_DIR", DefaultDataDir),
		OllamaURL:      getEnvOrDefault("LILRAG_OLLAMA_URL", DefaultOllamaURL),
		Model:          getEnvOrDefault("LILRAG_MODEL", DefaultChatModel),
		ChatModel:      getEnvOrDefault("LILRAG_CHAT_MODEL", DefaultChatModel),
		VisionModel:    getEnvOrDefault("LILRAG_VISION_MODEL", "llama3.2-vision"),
		TimeoutSeconds: getEnvIntOrDefault("LILRAG_TIMEOUT", 30),
		VectorSize:     getEnvIntOrDefault("LILRAG_VECTOR_SIZE", 768),
		MaxTokens:      getEnvIntOrDefault("LILRAG_MAX_TOKENS", 256),
		Overlap:        getEnvIntOrDefault("LILRAG_OVERLAP", 38),
		ImageMaxSize:   getEnvIntOrDefault("LILRAG_IMAGE_MAX_SIZE", 1120),
	}
}

// Helper functions for environment variable handling
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Simple conversion - in production you'd want proper error handling
		if intValue := parseIntOrDefault(value, defaultValue); intValue > 0 {
			return intValue
		}
	}
	return defaultValue
}

func parseIntOrDefault(value string, defaultValue int) int {
	// Simplified integer parsing
	var result int
	for _, r := range value {
		if r < '0' || r > '9' {
			return defaultValue
		}
		result = result*10 + int(r-'0')
	}
	return result
}
