package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database Database `json:"database" yaml:"database"`
	Ollama   Ollama   `json:"ollama" yaml:"ollama"`
	Server   Server   `json:"server" yaml:"server"`
	Chunking Chunk    `json:"chunking" yaml:"chunking"`
}

type Database struct {
	Path       string `json:"path" yaml:"path"`
	VectorSize int    `json:"vector_size" yaml:"vector_size"`
}

type Ollama struct {
	URL   string `json:"url" yaml:"url"`
	Model string `json:"model" yaml:"model"`
}

type Server struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

type Chunk struct {
	MaxTokens int `json:"max_tokens" yaml:"max_tokens"`
	Overlap   int `json:"overlap" yaml:"overlap"`
}

func Default() *Config {
	return &Config{
		Database: Database{
			Path:       "minirag.db",
			VectorSize: 768,
		},
		Ollama: Ollama{
			URL:   "http://localhost:11434",
			Model: "nomic-embed-text",
		},
		Server: Server{
			Host: "localhost",
			Port: 8080,
		},
		Chunking: Chunk{
			MaxTokens: 1800,
			Overlap:   200,
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
