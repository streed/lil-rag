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
	Host string `json:"host"`
	Port int    `json:"port"`
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
			Host: "localhost",
			Port: 8080,
		},
		Chunking: ChunkConfig{
			MaxTokens: 256, // Optimized for 2025 RAG best practices (128-512 range)
			Overlap:   38,  // 15% overlap ratio for optimal context preservation
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
			MaxTokens: p.Chunking.MaxTokens,
			Overlap:   p.Chunking.Overlap,
		},
	}
}

func (p *ProfileConfig) GetDataPath(filename string) string {
	return filepath.Join(p.DataDir, filename)
}
