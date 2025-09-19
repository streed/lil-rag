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
	MaxTokens int `json:"max_tokens"`
	Overlap   int `json:"overlap"`
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
			MaxTokens: 1024, // Optimal size based on 2024 research (512-1024 range, 1024 shows best balance)
			Overlap:   128,  // 12.5% overlap ratio, optimal for context preservation and retrieval accuracy
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
		// Return default profile WITHOUT saving it automatically
		config := DefaultProfile()
		if ensureErr := config.ensureDirectories(); ensureErr != nil {
			return nil, ensureErr
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

// LoadOrCreateProfile loads an existing profile or creates and saves a default one
// Use this when you explicitly want to create a config file if it doesn't exist
func LoadOrCreateProfile() (*ProfileConfig, error) {
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

	// Config exists, load it normally
	return LoadProfile()
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
			MaxTokens: p.Chunking.MaxTokens,
			Overlap:   p.Chunking.Overlap,
		},
	}
}

func (p *ProfileConfig) GetDataPath(filename string) string {
	return filepath.Join(p.DataDir, filename)
}
