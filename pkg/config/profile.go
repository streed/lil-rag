package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProfileConfig struct {
	Ollama      OllamaConfig `json:"ollama"`
	StoragePath string       `json:"storage_path"`
	DataDir     string       `json:"data_dir"`
	Server      ServerConfig `json:"server"`
	Chunking    ChunkConfig  `json:"chunking"`
	Search      SearchConfig `json:"search"`
}

type OllamaConfig struct {
	Endpoint       string `json:"endpoint"`
	EmbeddingModel string `json:"embedding_model"`
	VectorSize     int    `json:"vector_size"`
	ChatModel      string `json:"chat_model"`
	VisionModel    string `json:"vision_model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	ImageMaxSize   int    `json:"image_max_size"`
}

type ServerConfig struct {
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Secure         bool     `json:"secure"`           // Enable/disable authentication
	ReadTimeout    int      `json:"read_timeout"`     // Read timeout in seconds
	WriteTimeout   int      `json:"write_timeout"`    // Write timeout in seconds
	IdleTimeout    int      `json:"idle_timeout"`     // Idle timeout in seconds
	MaxHeaderBytes int      `json:"max_header_bytes"` // Maximum header size in bytes
	EnableCORS     bool     `json:"enable_cors"`      // Enable CORS headers
	TrustedProxies []string `json:"trusted_proxies"`  // List of trusted proxy IPs
}

type ChunkConfig struct {
	MaxTokens         int     `json:"max_tokens"`
	Overlap           int     `json:"overlap"`
	DefaultStrategy   string  `json:"default_strategy"`
	SemanticThreshold float32 `json:"semantic_threshold"`
	ThresholdType     string  `json:"threshold_type"`
}

type SearchConfig struct {
	DefaultLimit             int  `json:"default_limit"`
	DefaultChatLimit         int  `json:"default_chat_limit"`
	MaxMCPSearchLimit        int  `json:"max_mcp_search_limit"`
	MaxMCPChatLimit          int  `json:"max_mcp_chat_limit"`
	TruncateDocuments        bool `json:"truncate_documents"`
	MaxDocumentLength        int  `json:"max_document_length"`
	EnableQueryOptimization  bool `json:"enable_query_optimization"`
	ReturnMatchingChunksOnly bool `json:"return_matching_chunks_only"`
}

func DefaultProfile() *ProfileConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory cannot be determined
		homeDir = "."
	}
	dataDir := filepath.Join(homeDir, ".lilrag", "data")

	return &ProfileConfig{
		Ollama: OllamaConfig{
			Endpoint:       "http://localhost:11434",
			EmbeddingModel: "nomic-embed-text",
			VectorSize:     768,
			ChatModel:      "gemma3:4b",
			VisionModel:    "llama3.2-vision",
			TimeoutSeconds: 30,
			ImageMaxSize:   1120,
		},
		StoragePath: filepath.Join(dataDir, "lilrag.db"),
		DataDir:     dataDir,
		Server: ServerConfig{
			Host:           "localhost",
			Port:           12121,
			Secure:         true,       // Enable authentication by default
			ReadTimeout:    30,         // 30 seconds
			WriteTimeout:   30,         // 30 seconds
			IdleTimeout:    120,        // 2 minutes
			MaxHeaderBytes: 1048576,    // 1MB
			EnableCORS:     false,      // Disabled by default for security
			TrustedProxies: []string{}, // Empty by default
		},
		Chunking: ChunkConfig{
			MaxTokens:         256,          // Optimized for 2025 RAG best practices (128-512 range)
			Overlap:           38,           // 15% overlap ratio for optimal context preservation
			DefaultStrategy:   "recursive",  // Default chunking strategy
			SemanticThreshold: 0.95,         // 95th percentile threshold for semantic chunking
			ThresholdType:     "percentile", // Threshold type for semantic chunking
		},
		Search: SearchConfig{
			DefaultLimit:             25,    // Increased default for comprehensive search results
			DefaultChatLimit:         15,    // Increased default for richer chat context
			MaxMCPSearchLimit:        100,   // Increased MCP search limit for flexibility
			MaxMCPChatLimit:          50,    // Increased MCP chat limit for better context
			TruncateDocuments:        false, // Disable truncation for full document access
			MaxDocumentLength:        0,     // 0 means no limit when truncation is disabled
			EnableQueryOptimization:  false, // Disable query optimization by default, can be enabled for better semantic search
			ReturnMatchingChunksOnly: false, // Return full context by default, can be enabled to return only matching chunks
		},
	}
}

func GetProfileConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".lilrag")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "config.json"), nil
}

func LoadProfile() (*ProfileConfig, error) {
	configPath, err := GetProfileConfigPath()
	if err != nil {
		return nil, err
	}

	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		config := DefaultProfile()
		if saveErr := config.Save(); saveErr != nil {
			return nil, fmt.Errorf("failed to create default config: %w", saveErr)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ProfileConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.ensureDirectories(); err != nil {
		return nil, err
	}

	return &config, nil
}

func (p *ProfileConfig) Save() error {
	configPath, err := GetProfileConfigPath()
	if err != nil {
		return err
	}

	if saveErr := p.ensureDirectories(); saveErr != nil {
		return saveErr
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (p *ProfileConfig) ensureDirectories() error {
	if p.DataDir != "" {
		if err := os.MkdirAll(p.DataDir, 0o755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	if p.StoragePath != "" {
		storageDir := filepath.Dir(p.StoragePath)
		if err := os.MkdirAll(storageDir, 0o755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	return nil
}

func (p *ProfileConfig) ToLilRagConfig() *Config {
	return &Config{
		Database: Database{
			Path:       p.StoragePath,
			VectorSize: p.Ollama.VectorSize,
		},
		Ollama: Ollama{
			URL:            p.Ollama.Endpoint,
			Model:          p.Ollama.EmbeddingModel,
			VisionModel:    p.Ollama.VisionModel,
			TimeoutSeconds: p.Ollama.TimeoutSeconds,
		},
		Server: Server{
			Host: p.Server.Host,
			Port: p.Server.Port,
		},
		Chunking: Chunk{
			MaxTokens:         p.Chunking.MaxTokens,
			Overlap:           p.Chunking.Overlap,
			DefaultStrategy:   p.Chunking.DefaultStrategy,
			SemanticThreshold: p.Chunking.SemanticThreshold,
			ThresholdType:     p.Chunking.ThresholdType,
		},
	}
}

func (p *ProfileConfig) GetDataPath(filename string) string {
	return filepath.Join(p.DataDir, filename)
}
