package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefault(t *testing.T) {
	config := Default()

	if config == nil {
		t.Fatal("Default() returned nil")
	}

	// Test default values
	if config.Database.Path != "minirag.db" {
		t.Errorf("Expected database path 'minirag.db', got %q", config.Database.Path)
	}
	if config.Database.VectorSize != 768 {
		t.Errorf("Expected vector size 768, got %d", config.Database.VectorSize)
	}
	if config.Ollama.URL != "http://localhost:11434" {
		t.Errorf("Expected Ollama URL 'http://localhost:11434', got %q", config.Ollama.URL)
	}
	if config.Ollama.Model != "nomic-embed-text" {
		t.Errorf("Expected Ollama model 'nomic-embed-text', got %q", config.Ollama.Model)
	}
	if config.Server.Host != "localhost" {
		t.Errorf("Expected server host 'localhost', got %q", config.Server.Host)
	}
	if config.Server.Port != 8080 {
		t.Errorf("Expected server port 8080, got %d", config.Server.Port)
	}
	if config.Chunking.MaxTokens != 1800 {
		t.Errorf("Expected max tokens 1800, got %d", config.Chunking.MaxTokens)
	}
	if config.Chunking.Overlap != 200 {
		t.Errorf("Expected overlap 200, got %d", config.Chunking.Overlap)
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	config, err := Load("")

	if err != nil {
		t.Errorf("Load with empty path returned error: %v", err)
	}

	// Should return default config
	defaultConfig := Default()
	if config.Database.Path != defaultConfig.Database.Path {
		t.Error("Load with empty path should return default config")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	config, err := Load("/nonexistent/file.json")

	if err != nil {
		t.Errorf("Load with nonexistent file returned error: %v", err)
	}

	// Should return default config
	defaultConfig := Default()
	if config.Database.Path != defaultConfig.Database.Path {
		t.Error("Load with nonexistent file should return default config")
	}
}

func TestLoad_JSONFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test JSON config
	testConfig := &Config{
		Database: Database{
			Path:       "test.db",
			VectorSize: 384,
		},
		Ollama: Ollama{
			URL:   "http://test:11434",
			Model: "test-model",
		},
		Server: Server{
			Host: "0.0.0.0",
			Port: 9090,
		},
		Chunking: Chunk{
			MaxTokens: 1000,
			Overlap:   100,
		},
	}

	configPath := filepath.Join(tempDir, "config.json")
	data, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load and verify
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load JSON config: %v", err)
	}

	if loadedConfig.Database.Path != "test.db" {
		t.Errorf("Expected database path 'test.db', got %q", loadedConfig.Database.Path)
	}
	if loadedConfig.Database.VectorSize != 384 {
		t.Errorf("Expected vector size 384, got %d", loadedConfig.Database.VectorSize)
	}
	if loadedConfig.Ollama.URL != "http://test:11434" {
		t.Errorf("Expected Ollama URL 'http://test:11434', got %q", loadedConfig.Ollama.URL)
	}
}

func TestLoad_YAMLFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test YAML config
	yamlContent := `
database:
  path: test.db
  vector_size: 384
ollama:
  url: http://test:11434
  model: test-model
server:
  host: 0.0.0.0
  port: 9090
chunking:
  max_tokens: 1000
  overlap: 100
`

	configPath := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test YAML config: %v", err)
	}

	// Load and verify
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	if loadedConfig.Database.Path != "test.db" {
		t.Errorf("Expected database path 'test.db', got %q", loadedConfig.Database.Path)
	}
	if loadedConfig.Ollama.Model != "test-model" {
		t.Errorf("Expected Ollama model 'test-model', got %q", loadedConfig.Ollama.Model)
	}
}

func TestLoad_UnsupportedFormat(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.txt")
	err = os.WriteFile(configPath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for unsupported file format")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	err = os.WriteFile(configPath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestConfig_Save_JSON(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Default()
	config.Database.Path = "saved.db"
	config.Server.Port = 9999

	configPath := filepath.Join(tempDir, "saved_config.json")
	err = config.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	// Verify it can be unmarshaled
	var savedConfig Config
	err = json.Unmarshal(data, &savedConfig)
	if err != nil {
		t.Fatalf("Failed to unmarshal saved config: %v", err)
	}

	if savedConfig.Database.Path != "saved.db" {
		t.Errorf("Expected saved database path 'saved.db', got %q", savedConfig.Database.Path)
	}
	if savedConfig.Server.Port != 9999 {
		t.Errorf("Expected saved server port 9999, got %d", savedConfig.Server.Port)
	}
}

func TestConfig_Save_YAML(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Default()
	config.Database.Path = "saved.db"
	config.Server.Port = 9999

	configPath := filepath.Join(tempDir, "saved_config.yaml")
	err = config.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	// Verify it can be unmarshaled
	var savedConfig Config
	err = yaml.Unmarshal(data, &savedConfig)
	if err != nil {
		t.Fatalf("Failed to unmarshal saved YAML config: %v", err)
	}

	if savedConfig.Database.Path != "saved.db" {
		t.Errorf("Expected saved database path 'saved.db', got %q", savedConfig.Database.Path)
	}
	if savedConfig.Server.Port != 9999 {
		t.Errorf("Expected saved server port 9999, got %d", savedConfig.Server.Port)
	}
}

func TestConfig_Save_UnsupportedFormat(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Default()
	configPath := filepath.Join(tempDir, "config.txt")

	err = config.Save(configPath)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestConfig_Save_CreateDirectory(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := Default()
	// Use nested path that doesn't exist
	configPath := filepath.Join(tempDir, "nested", "dir", "config.json")

	err = config.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config with nested directory: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file was not created: %v", err)
	}
}

// Test round-trip: Save and Load
func TestConfig_RoundTrip_JSON(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create custom config
	originalConfig := &Config{
		Database: Database{
			Path:       "roundtrip.db",
			VectorSize: 512,
		},
		Ollama: Ollama{
			URL:   "http://custom:11434",
			Model: "custom-model",
		},
		Server: Server{
			Host: "127.0.0.1",
			Port: 7777,
		},
		Chunking: Chunk{
			MaxTokens: 2500,
			Overlap:   300,
		},
	}

	configPath := filepath.Join(tempDir, "roundtrip.json")

	// Save
	err = originalConfig.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify all fields match
	if loadedConfig.Database.Path != originalConfig.Database.Path {
		t.Errorf("Database path mismatch: got %q, want %q", loadedConfig.Database.Path, originalConfig.Database.Path)
	}
	if loadedConfig.Database.VectorSize != originalConfig.Database.VectorSize {
		t.Errorf("Vector size mismatch: got %d, want %d", loadedConfig.Database.VectorSize, originalConfig.Database.VectorSize)
	}
	if loadedConfig.Ollama.URL != originalConfig.Ollama.URL {
		t.Errorf("Ollama URL mismatch: got %q, want %q", loadedConfig.Ollama.URL, originalConfig.Ollama.URL)
	}
	if loadedConfig.Ollama.Model != originalConfig.Ollama.Model {
		t.Errorf("Ollama model mismatch: got %q, want %q", loadedConfig.Ollama.Model, originalConfig.Ollama.Model)
	}
	if loadedConfig.Server.Host != originalConfig.Server.Host {
		t.Errorf("Server host mismatch: got %q, want %q", loadedConfig.Server.Host, originalConfig.Server.Host)
	}
	if loadedConfig.Server.Port != originalConfig.Server.Port {
		t.Errorf("Server port mismatch: got %d, want %d", loadedConfig.Server.Port, originalConfig.Server.Port)
	}
	if loadedConfig.Chunking.MaxTokens != originalConfig.Chunking.MaxTokens {
		t.Errorf("Max tokens mismatch: got %d, want %d", loadedConfig.Chunking.MaxTokens, originalConfig.Chunking.MaxTokens)
	}
	if loadedConfig.Chunking.Overlap != originalConfig.Chunking.Overlap {
		t.Errorf("Overlap mismatch: got %d, want %d", loadedConfig.Chunking.Overlap, originalConfig.Chunking.Overlap)
	}
}

func TestConfig_RoundTrip_YAML(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create custom config
	originalConfig := Default()
	originalConfig.Database.Path = "yaml_test.db"
	originalConfig.Server.Port = 8888

	configPath := filepath.Join(tempDir, "roundtrip.yaml")

	// Save
	err = originalConfig.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save YAML config: %v", err)
	}

	// Load
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	// Verify key fields match
	if loadedConfig.Database.Path != originalConfig.Database.Path {
		t.Errorf("Database path mismatch: got %q, want %q", loadedConfig.Database.Path, originalConfig.Database.Path)
	}
	if loadedConfig.Server.Port != originalConfig.Server.Port {
		t.Errorf("Server port mismatch: got %d, want %d", loadedConfig.Server.Port, originalConfig.Server.Port)
	}
}
