package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	Database Database `json:"database" yaml:"database"`
	Ollama   Ollama   `json:"ollama" yaml:"ollama"`
	Server   Server   `json:"server" yaml:"server"`
	Chunking Chunk    `json:"chunking" yaml:"chunking"`
	Search   Search   `json:"search" yaml:"search"`
}

type Database struct {
	Path       string `json:"path" yaml:"path"`
	VectorSize int    `json:"vector_size" yaml:"vector_size"`
}

type Ollama struct {
	URL            string `json:"url" yaml:"url"`
	Model          string `json:"model" yaml:"model"`
	VisionModel    string `json:"vision_model" yaml:"vision_model"`
	TimeoutSeconds int    `json:"timeout_seconds" yaml:"timeout_seconds"`
	ImageMaxSize   int    `json:"image_max_size" yaml:"image_max_size"`
}

type Server struct {
	Host           string   `json:"host" yaml:"host"`
	Port           int      `json:"port" yaml:"port"`
	Secure         bool     `json:"secure" yaml:"secure"`
	ReadTimeout    int      `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout   int      `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout    int      `json:"idle_timeout" yaml:"idle_timeout"`
	MaxHeaderBytes int      `json:"max_header_bytes" yaml:"max_header_bytes"`
	EnableCORS     bool     `json:"enable_cors" yaml:"enable_cors"`
	TrustedProxies []string `json:"trusted_proxies" yaml:"trusted_proxies"`
}

type Chunk struct {
	MaxTokens         int     `json:"max_tokens" yaml:"max_tokens"`
	Overlap           int     `json:"overlap" yaml:"overlap"`
	DefaultStrategy   string  `json:"default_strategy" yaml:"default_strategy"`
	SemanticThreshold float32 `json:"semantic_threshold" yaml:"semantic_threshold"`
	ThresholdType     string  `json:"threshold_type" yaml:"threshold_type"`
}

type Search struct {
	DefaultLimit        int  `json:"default_limit" yaml:"default_limit"`
	DefaultChatLimit    int  `json:"default_chat_limit" yaml:"default_chat_limit"`
	MaxMCPSearchLimit   int  `json:"max_mcp_search_limit" yaml:"max_mcp_search_limit"`
	MaxMCPChatLimit     int  `json:"max_mcp_chat_limit" yaml:"max_mcp_chat_limit"`
	TruncateDocuments   bool `json:"truncate_documents" yaml:"truncate_documents"`
	MaxDocumentLength   int  `json:"max_document_length" yaml:"max_document_length"`
	EnableQueryOptimization bool `json:"enable_query_optimization" yaml:"enable_query_optimization"`
	ReturnMatchingChunksOnly bool `json:"return_matching_chunks_only" yaml:"return_matching_chunks_only"`
}

// Default returns a new Config with default values.
func Default() *Config {
	return &Config{
		Database: Database{
			Path:       "lilrag.db",
			VectorSize: 768,
		},
		Ollama: Ollama{
			URL:            "http://localhost:11434",
			Model:          "nomic-embed-text",
			VisionModel:    "llama3.2-vision",
			TimeoutSeconds: 30,
			ImageMaxSize:   1120,
		},
		Server: Server{
			Host:           "localhost",
			Port:           12121,
			Secure:         true,
			ReadTimeout:    30,
			WriteTimeout:   30,
			IdleTimeout:    120,
			MaxHeaderBytes: 1048576,
			EnableCORS:     false,
			TrustedProxies: []string{},
		},
		Chunking: Chunk{
			MaxTokens:         800,          // Optimal size based on 2024 research (200-800 range)
			Overlap:           100,          // 12.5% overlap ratio, optimal for context preservation
			DefaultStrategy:   "recursive",  // Default chunking strategy
			SemanticThreshold: 0.95,         // 95th percentile threshold for semantic chunking
			ThresholdType:     "percentile", // Threshold type for semantic chunking
		},
		Search: Search{
			DefaultLimit:        25,   // Increased default for comprehensive search results
			DefaultChatLimit:    15,   // Increased default for richer chat context
			MaxMCPSearchLimit:   100,  // Increased MCP search limit for flexibility
			MaxMCPChatLimit:     50,   // Increased MCP chat limit for better context
			TruncateDocuments:   false, // Disable truncation for full document access
			MaxDocumentLength:   0,    // 0 means no limit when truncation is disabled
			EnableQueryOptimization: false, // Disable query optimization by default, can be enabled for better semantic search
			ReturnMatchingChunksOnly: false, // Return full context by default, can be enabled to return only matching chunks
		},
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		return Default(), nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := Default()
	ext := filepath.Ext(path)

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	return config, nil
}

func (c *Config) Save(path string) error {
	ext := filepath.Ext(path)
	var data []byte
	var err error

	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(c)
		if err != nil {
			return fmt.Errorf("failed to marshal config to YAML: %w", err)
		}
	case ".json":
		data, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
