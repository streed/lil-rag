package lilrag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaChatClient_GenerateResponseStreaming(t *testing.T) {
	// Mock Ollama server that returns streaming responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		var req ChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			t.Errorf("Failed to decode request: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Verify streaming is enabled
		if !req.Stream {
			t.Error("Expected streaming to be enabled")
			http.Error(w, "Stream not enabled", http.StatusBadRequest)
			return
		}

		// Send streaming response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Send multiple chunks
		chunks := []ChatResponse{
			{
				Model:     "test-model",
				CreatedAt: time.Now(),
				Message:   ChatMessage{Role: "assistant", Content: "Hello"},
				Done:      false,
			},
			{
				Model:     "test-model",
				CreatedAt: time.Now(),
				Message:   ChatMessage{Role: "assistant", Content: " world"},
				Done:      false,
			},
			{
				Model:     "test-model",
				CreatedAt: time.Now(),
				Message:   ChatMessage{Role: "assistant", Content: "!"},
				Done:      true,
			},
		}

		encoder := json.NewEncoder(w)
		for _, chunk := range chunks {
			if err := encoder.Encode(chunk); err != nil {
				t.Errorf("Failed to encode chunk: %v", err)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	// Create client
	client := NewOllamaChatClient(server.URL, "test-model")

	// Track streaming chunks
	var receivedChunks []string
	var doneReceived bool

	handler := func(chunk string, done bool) error {
		receivedChunks = append(receivedChunks, chunk)
		if done {
			doneReceived = true
		}
		return nil
	}

	// Test streaming
	ctx := context.Background()
	err := client.GenerateResponseStreaming(ctx, "test message", nil, handler)

	if err != nil {
		t.Fatalf("GenerateResponseStreaming failed: %v", err)
	}

	// Verify chunks were received
	expectedChunks := []string{"Hello", " world", "!"}
	if len(receivedChunks) != len(expectedChunks) {
		t.Errorf("Expected %d chunks, got %d", len(expectedChunks), len(receivedChunks))
	}

	for i, expected := range expectedChunks {
		if i < len(receivedChunks) && receivedChunks[i] != expected {
			t.Errorf("Chunk %d: expected %q, got %q", i, expected, receivedChunks[i])
		}
	}

	if !doneReceived {
		t.Error("Expected to receive done signal")
	}
}

func TestOllamaChatClient_GenerateResponseStreaming_Error(t *testing.T) {
	// Mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewOllamaChatClient(server.URL, "test-model")

	handler := func(chunk string, done bool) error {
		t.Error("Handler should not be called on server error")
		return nil
	}

	ctx := context.Background()
	err := client.GenerateResponseStreaming(ctx, "test message", nil, handler)

	if err == nil {
		t.Error("Expected error for server failure")
	}

	if !strings.Contains(err.Error(), "failed with status 500") {
		t.Errorf("Expected status error, got: %v", err)
	}
}

func TestOllamaChatClient_GenerateResponseStreaming_HandlerError(t *testing.T) {
	// Mock server that returns streaming response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		chunk := ChatResponse{
			Model:     "test-model",
			CreatedAt: time.Now(),
			Message:   ChatMessage{Role: "assistant", Content: "test"},
			Done:      false,
		}

		json.NewEncoder(w).Encode(chunk)
	}))
	defer server.Close()

	client := NewOllamaChatClient(server.URL, "test-model")

	// Handler that returns an error
	handler := func(chunk string, done bool) error {
		return &testError{"handler error"}
	}

	ctx := context.Background()
	err := client.GenerateResponseStreaming(ctx, "test message", nil, handler)

	if err == nil {
		t.Error("Expected error from handler")
	}

	if !strings.Contains(err.Error(), "handler error") {
		t.Errorf("Expected handler error, got: %v", err)
	}
}

// testError is a custom error type for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
