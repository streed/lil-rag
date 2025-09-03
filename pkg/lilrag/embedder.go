package lilrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type OllamaEmbedder struct {
	baseURL      string
	model        string
	client       *http.Client
	cache        map[string][]float32
	cacheMutex   sync.RWMutex
	cacheMaxSize int
	preprocessor *TextPreprocessor
}

type TextPreprocessor struct {
	normalizeWhitespace bool
	removeExtraSpaces   bool
	maxLength           int
}

type EmbedderMetrics struct {
	TotalRequests  int64
	CacheHits      int64
	AverageLatency time.Duration
	ErrorCount     int64
}

type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func NewOllamaEmbedder(baseURL, model string) (*OllamaEmbedder, error) {
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:        make(map[string][]float32),
		cacheMaxSize: 1000,
		preprocessor: &TextPreprocessor{
			normalizeWhitespace: true,
			removeExtraSpaces:   true,
			maxLength:           8192,
		},
	}, nil
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Preprocess text
	processedText := o.preprocessor.preprocess(text)

	// Check cache first
	if embedding, found := o.getFromCache(processedText); found {
		return embedding, nil
	}

	// Create embedding with retry logic
	embedding, err := o.embedWithRetry(ctx, processedText, 3)
	if err != nil {
		return nil, err
	}

	// Cache the result
	o.addToCache(processedText, embedding)

	return embedding, nil
}

func (tp *TextPreprocessor) preprocess(text string) string {
	// Normalize unicode and clean text
	text = strings.ToValidUTF8(text, "")

	// Normalize whitespace - preserve leading/trailing spaces if removeExtraSpaces is false
	if tp.normalizeWhitespace {
		// Match leading spaces, internal whitespace, and trailing spaces separately
		leadingSpaces := regexp.MustCompile(`^\s*`).FindString(text)
		trailingSpaces := regexp.MustCompile(`\s*$`).FindString(text)

		// Normalize internal whitespace only
		text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

		// If we're not removing extra spaces, restore the original leading/trailing patterns
		if !tp.removeExtraSpaces {
			if leadingSpaces != "" {
				text = regexp.MustCompile(`^\s*`).ReplaceAllString(text, leadingSpaces)
			}
			if trailingSpaces != "" {
				text = regexp.MustCompile(`\s*$`).ReplaceAllString(text, trailingSpaces)
			}
		}
	}

	// Remove extra spaces (trim leading/trailing)
	if tp.removeExtraSpaces {
		text = strings.TrimSpace(text)
	}

	// Limit length to prevent token overflow
	if tp.maxLength > 0 && len(text) > tp.maxLength {
		// Try to cut at word boundary
		truncated := text[:tp.maxLength]
		if lastSpace := strings.LastIndex(truncated, " "); lastSpace > tp.maxLength/2 {
			text = truncated[:lastSpace]
		} else {
			text = truncated
		}
	}

	return text
}

func (o *OllamaEmbedder) embedWithRetry(ctx context.Context, text string, maxRetries int) ([]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		embedding, err := o.embedDirect(ctx, text)
		if err == nil {
			return embedding, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (o *OllamaEmbedder) embedDirect(ctx context.Context, text string) ([]float32, error) {
	request := OllamaEmbedRequest{
		Model:  o.model,
		Prompt: text,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/embeddings", o.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ollama API returned status %d and failed to read error response: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned from ollama")
	}

	return response.Embedding, nil
}

func (o *OllamaEmbedder) getFromCache(text string) ([]float32, bool) {
	o.cacheMutex.RLock()
	defer o.cacheMutex.RUnlock()

	embedding, found := o.cache[text]
	return embedding, found
}

func (o *OllamaEmbedder) addToCache(text string, embedding []float32) {
	o.cacheMutex.Lock()
	defer o.cacheMutex.Unlock()

	// Simple LRU - remove oldest if cache is full
	if len(o.cache) >= o.cacheMaxSize {
		// Remove first entry (not truly LRU but simple)
		for k := range o.cache {
			delete(o.cache, k)
			break
		}
	}

	// Copy embedding to avoid reference issues
	embeddingCopy := make([]float32, len(embedding))
	copy(embeddingCopy, embedding)
	o.cache[text] = embeddingCopy
}

// EmbedQuery processes queries with different preprocessing for better search results
func (o *OllamaEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Enhanced query preprocessing
	processedQuery := o.preprocessQuery(query)

	// Check cache first
	if embedding, found := o.getFromCache(processedQuery); found {
		return embedding, nil
	}

	// Create embedding
	embedding, err := o.embedWithRetry(ctx, processedQuery, 3)
	if err != nil {
		return nil, err
	}

	// Cache the result
	o.addToCache(processedQuery, embedding)

	return embedding, nil
}

func (o *OllamaEmbedder) preprocessQuery(query string) string {
	// Clean the query
	query = strings.TrimSpace(query)
	query = strings.ToValidUTF8(query, "")

	// Normalize whitespace
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	// Add semantic context for better retrieval (simple query enhancement)
	if !strings.Contains(query, "?") && len(strings.Fields(query)) < 10 {
		return fmt.Sprintf("Find information about: %s", query)
	}

	return query
}

// GetCacheStats returns cache performance statistics
func (o *OllamaEmbedder) GetCacheStats() map[string]interface{} {
	o.cacheMutex.RLock()
	defer o.cacheMutex.RUnlock()

	return map[string]interface{}{
		"cache_size":    len(o.cache),
		"max_size":      o.cacheMaxSize,
		"usage_percent": float64(len(o.cache)) / float64(o.cacheMaxSize) * 100,
	}
}

// ClearCache clears the embedding cache
func (o *OllamaEmbedder) ClearCache() {
	o.cacheMutex.Lock()
	defer o.cacheMutex.Unlock()

	o.cache = make(map[string][]float32)
}
