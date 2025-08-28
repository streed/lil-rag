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
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".minirag", "data")

	return &ProfileConfig{
		Ollama: OllamaConfig{
			Endpoint:       "http://localhost:11434",
			EmbeddingModel: "nomic-embed-text",
			VectorSize:     768,
		},
		StoragePath: filepath.Join(dataDir, "minirag.db"),
		DataDir:     dataDir,
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Chunking: ChunkConfig{
			MaxTokens: 1800, // Conservative limit under 2k tokens
			Overlap:   200,  // Overlap between chunks for context
		},
	}
}

func GetProfileConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".minirag")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(configDir, "config.json"), nil
}

func LoadProfile() (*ProfileConfig, error) {
	configPath, err := GetProfileConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultProfile()
		if err := config.Save(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
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

	if err := p.ensureDirectories(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (p *ProfileConfig) ensureDirectories() error {
	if p.DataDir != "" {
		if err := os.MkdirAll(p.DataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	if p.StoragePath != "" {
		storageDir := filepath.Dir(p.StoragePath)
		if err := os.MkdirAll(storageDir, 0755); err != nil {
			return fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	return nil
}

func (p *ProfileConfig) ToMiniRagConfig() *Config {
	return &Config{
		Database: Database{
			Path:       p.StoragePath,
			VectorSize: p.Ollama.VectorSize,
		},
		Ollama: Ollama{
			URL:   p.Ollama.Endpoint,
			Model: p.Ollama.EmbeddingModel,
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
