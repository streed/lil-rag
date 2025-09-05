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

	"lil-rag/pkg/metrics"
)

type OllamaEmbedder struct {
	baseURL      string
	model        string
	client       *http.Client
	cache        map[string][]float32
	cacheMutex   sync.RWMutex
	cacheMaxSize int
	preprocessor *TextPreprocessor
	totalRequests int64
	cacheHits     int64
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
	return NewOllamaEmbedderWithTimeout(baseURL, model, 30)
}

func NewOllamaEmbedderWithTimeout(baseURL, model string, timeoutSeconds int) (*OllamaEmbedder, error) {
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
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

	// Update total requests counter
	o.totalRequests++

	// Check cache first
	if embedding, found := o.getFromCache(processedText); found {
		// Record cache hit
		o.cacheHits++
		metrics.RecordEmbeddingTokens(o.model, processedText, true)
		o.updateCacheHitRate()
		return embedding, nil
	}

	// Create embedding with retry logic
	embedding, err := o.embedWithRetry(ctx, processedText, 3)
	if err != nil {
		return nil, err
	}

	// Record tokens for cache miss
	metrics.RecordEmbeddingTokens(o.model, processedText, false)
	o.updateCacheHitRate()

	// Cache the result
	o.addToCache(processedText, embedding)

	return embedding, nil
}

func (tp *TextPreprocessor) preprocess(text string) string {
	// Normalize unicode and clean text
	text = strings.ToValidUTF8(text, "")

	// Enhanced preprocessing for image-derived content
	text = tp.enhanceImageContent(text)

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

// enhanceImageContent improves semantic searchability of image-derived text
func (tp *TextPreprocessor) enhanceImageContent(text string) string {
	// Check if this looks like image-derived content (has markdown formatting and structured data)
	hasMarkdownHeaders := strings.Contains(text, "###") || strings.Contains(text, "**")
	hasStructuredData := strings.Contains(text, "+ ") || strings.Contains(text, "\t+")
	hasBusinessInfo := strings.Contains(text, "Phone:") || strings.Contains(text, "Email:")
	
	if hasMarkdownHeaders || hasStructuredData || hasBusinessInfo {
		// Clean up markdown formatting for better semantic matching
		enhanced := text
		
		// Remove markdown formatting but preserve content
		enhanced = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(enhanced, "$1")  // **bold** -> bold
		enhanced = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(enhanced, "$1")      // *italic* -> italic
		enhanced = regexp.MustCompile(`#{1,6}\s*`).ReplaceAllString(enhanced, "")           // ### headers -> content
		enhanced = regexp.MustCompile(`\t\+\s*`).ReplaceAllString(enhanced, " ")           // \t+ -> space
		enhanced = regexp.MustCompile(`^\s*[\+\-\*]\s*`).ReplaceAllStringFunc(enhanced, func(s string) string {
			return " " // Convert bullet points to spaces
		})
		
		// Extract key searchable terms and add them at the beginning for better retrieval
		var keyTerms []string
		
		// Extract business/contact info
		phoneRegex := regexp.MustCompile(`(?i)phone:\s*([^\n\s]+(?:\s+[^\n\s]+)*)`)
		if matches := phoneRegex.FindAllStringSubmatch(text, -1); matches != nil {
			for _, match := range matches {
				keyTerms = append(keyTerms, "phone "+match[1])
			}
		}
		
		emailRegex := regexp.MustCompile(`(?i)email:\s*([^\n\s]+(?:\s+[^\n\s]+)*)`)
		if matches := emailRegex.FindAllStringSubmatch(text, -1); matches != nil {
			for _, match := range matches {
				keyTerms = append(keyTerms, "email "+match[1])
			}
		}
		
		// Extract company/service names (look for capitalized words before phone/email)
		companyRegex := regexp.MustCompile(`\*\*([^*]+)\*\*(?:\s|\n)*(?:[^\n]*(?:Phone|Email|Services):)`)
		if matches := companyRegex.FindAllStringSubmatch(text, -1); matches != nil {
			for _, match := range matches {
				keyTerms = append(keyTerms, match[1])
			}
		}
		
		// Extract services mentioned
		servicesRegex := regexp.MustCompile(`(?i)services?:\s*([^\n]+)`)
		if matches := servicesRegex.FindAllStringSubmatch(text, -1); matches != nil {
			for _, match := range matches {
				// Split services by commas and add each
				services := strings.Split(match[1], ",")
				for _, service := range services {
					service = strings.TrimSpace(service)
					if service != "" {
						keyTerms = append(keyTerms, service)
					}
				}
			}
		}
		
		// Prepend key terms to the enhanced text for better semantic matching
		if len(keyTerms) > 0 {
			enhanced = strings.Join(keyTerms, " ") + " " + enhanced
		}
		
		return enhanced
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

	// Update total requests counter
	o.totalRequests++

	// Check cache first
	if embedding, found := o.getFromCache(processedQuery); found {
		// Record cache hit
		o.cacheHits++
		metrics.RecordEmbeddingTokens(o.model, processedQuery, true)
		o.updateCacheHitRate()
		return embedding, nil
	}

	// Create embedding
	embedding, err := o.embedWithRetry(ctx, processedQuery, 3)
	if err != nil {
		return nil, err
	}

	// Record tokens for cache miss
	metrics.RecordEmbeddingTokens(o.model, processedQuery, false)
	o.updateCacheHitRate()

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

	// Enhanced query processing for better compatibility with image-derived content
	if !strings.Contains(query, "?") && len(strings.Fields(query)) < 10 {
		// For simple queries that might match business cards or structured content,
		// create multiple search variants to improve matching
		var variants []string
		
		// Add the original query
		variants = append(variants, query)
		
		// Check if query looks like a business name, phone number, or person name
		isBusinessQuery := regexp.MustCompile(`(?i)(painting|services?|cleaning|construction|repair)`).MatchString(query)
		isPhoneQuery := regexp.MustCompile(`\d{3}[-.]?\d{3}[-.]?\d{4}`).MatchString(query)
		isNameQuery := regexp.MustCompile(`^[A-Z][a-z]+\s+[A-Z][a-z]+$`).MatchString(query)
		
		if isBusinessQuery {
			variants = append(variants, "business company service provider "+query)
		}
		if isPhoneQuery {
			variants = append(variants, "phone contact number "+query)
		}
		if isNameQuery {
			variants = append(variants, "person owner operator name "+query)
		}
		
		// Combine variants for better semantic matching
		enhancedQuery := strings.Join(variants, " ")
		
		// Keep it concise but semantically rich
		return enhancedQuery
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
	o.totalRequests = 0
	o.cacheHits = 0
}

// updateCacheHitRate calculates and updates the cache hit rate metric
func (o *OllamaEmbedder) updateCacheHitRate() {
	if o.totalRequests == 0 {
		metrics.UpdateTokenCacheHitRate("embedding", 0.0)
		return
	}
	hitRate := float64(o.cacheHits) / float64(o.totalRequests)
	metrics.UpdateTokenCacheHitRate("embedding", hitRate)
}
