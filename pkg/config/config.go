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
	MaxTokens int `json:"max_tokens" yaml:"max_tokens"`
	Overlap   int `json:"overlap" yaml:"overlap"`
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
			MaxTokens: 1024, // Optimal size based on 2024 research (512-1024 range, 1024 shows best balance)
			Overlap:   128,  // 12.5% overlap ratio, optimal for context preservation and retrieval accuracy
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
