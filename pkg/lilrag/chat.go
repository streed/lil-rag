package lilrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lil-rag/pkg/metrics"
)

// OllamaChatClient handles chat interactions with Ollama
type OllamaChatClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaChatClient creates a new Ollama chat client
func NewOllamaChatClient(baseURL, model string) *OllamaChatClient {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	if model == "" {
		model = "gemma3:4b"
	}

	return &OllamaChatClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 120 * time.Second, // Longer timeout for chat generation
		},
	}
}

// ChatRequest represents a request to Ollama's chat API
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  *ChatOptions  `json:"options,omitempty"`
}

// ChatMessage represents a single message in a chat conversation
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// ChatOptions for controlling chat generation
type ChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

// ChatResponse represents Ollama's chat response
type ChatResponse struct {
	Model     string      `json:"model"`
	CreatedAt time.Time   `json:"created_at"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
}

// GenerateResponse generates a chat response using the provided context and user message
func (c *OllamaChatClient) GenerateResponse(ctx context.Context, userMessage string,
	searchResults []SearchResult) (string, error) {
	// Create system prompt with search results context
	systemPrompt := c.createSystemPrompt(searchResults)

	// Build chat messages
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	// Create request
	requestBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Options: &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
		},
	}

	// Marshal request
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send chat request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("ollama server returned status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("chat request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode chat response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// createSystemPrompt creates a system prompt with search results context
func (c *OllamaChatClient) createSystemPrompt(searchResults []SearchResult) string {
	var prompt strings.Builder

	prompt.WriteString("You are a helpful AI assistant that answers questions based on provided document context. ")
	prompt.WriteString("Use the following documents to answer the user's question. ")
	prompt.WriteString("Be accurate, concise, and cite which documents you're referencing.\n\n")

	if len(searchResults) == 0 {
		prompt.WriteString("No relevant documents were found. Please inform the user that you don't " +
			"have enough context to answer their question and suggest they provide more relevant " +
			"documents or rephrase their query.")
		return prompt.String()
	}

	prompt.WriteString("RELEVANT DOCUMENTS:\n\n")

	for i, result := range searchResults {
		prompt.WriteString(fmt.Sprintf("Document %d (ID: %s, Relevance: %.1f%%):\n",
			i+1, result.ID, result.Score*100))

		// Use a reasonable excerpt length
		text := result.Text
		if len(text) > 2000 {
			text = text[:2000] + "..."
		}

		prompt.WriteString(text)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString("Please answer the user's question based on these documents. ")
	prompt.WriteString("If the documents don't contain relevant information, say so clearly. ")
	prompt.WriteString("When referencing information from a document, cite it using square " +
		"brackets with the document ID: [document-id]. ")
	prompt.WriteString("For example: \"According to [lilrag-overview]...\" or \"As mentioned in [vector-search]...\". ")
	prompt.WriteString("Use only the document ID inside the brackets, not \"Document 1\" or similar.")

	return prompt.String()
}

// TestConnection tests if the Ollama server is reachable and the model is available
func (c *OllamaChatClient) TestConnection(ctx context.Context) error {
	// Check if the server is reachable
	url := fmt.Sprintf("%s/api/tags", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama server returned status %d", resp.StatusCode)
	}

	// Parse response to check if model is available
	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return fmt.Errorf("failed to decode tags response: %w", err)
	}

	// Check if the chat model is available
	modelFound := false
	for _, model := range tagsResp.Models {
		if strings.HasPrefix(model.Name, strings.Split(c.model, ":")[0]) {
			modelFound = true
			break
		}
	}

	if !modelFound {
		return fmt.Errorf("chat model '%s' not found in Ollama. Available models: %v",
			c.model, tagsResp.Models)
	}

	return nil
}

// OptimizeQuery uses the LLM to optimize a user query for better semantic search results
func (c *OllamaChatClient) OptimizeQuery(ctx context.Context, userQuery string) (string, error) {
	if userQuery == "" {
		return userQuery, nil
	}

	optimizationStart := time.Now()

	systemPrompt := `You are an expert at optimizing search queries for semantic/vector search in document databases. 

Your task is to take a user's question or query and reformulate it to be more effective for semantic search. This means:

1. Extract the key concepts and main topics
2. Use more specific, searchable keywords
3. Remove unnecessary words like "please", "can you", "I want to know"
4. Focus on the core information need
5. Use synonyms or related terms that might appear in documents
6. Keep it concise but comprehensive

Examples:
- "Can you please tell me about machine learning?" → "machine learning algorithms models training"
- "I want to know how to set up a database" → "database setup installation configuration"
- "What are the benefits of using Docker?" → "Docker benefits advantages containerization"

Respond with ONLY the optimized query, no explanations or additional text.`

	// Build chat messages for query optimization
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userQuery,
		},
	}

	// Create request with lower temperature for more consistent optimization
	requestBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Options: &ChatOptions{
			Temperature: 0.3, // Lower temperature for more consistent results
			TopP:        0.9,
		},
	}

	// Marshal request
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		optimizationDuration := time.Since(optimizationStart)
		metrics.RecordQueryOptimization(optimizationDuration, false)
		return userQuery, fmt.Errorf("failed to marshal query optimization request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		optimizationDuration := time.Since(optimizationStart)
		metrics.RecordQueryOptimization(optimizationDuration, false)
		return userQuery, fmt.Errorf("failed to create query optimization request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		optimizationDuration := time.Since(optimizationStart)
		metrics.RecordQueryOptimization(optimizationDuration, false)
		return userQuery, fmt.Errorf("failed to send query optimization request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		optimizationDuration := time.Since(optimizationStart)
		metrics.RecordQueryOptimization(optimizationDuration, false)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return userQuery, fmt.Errorf("query optimization request failed with status %d", resp.StatusCode)
		}
		return userQuery, fmt.Errorf("query optimization request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		optimizationDuration := time.Since(optimizationStart)
		metrics.RecordQueryOptimization(optimizationDuration, false)
		return userQuery, fmt.Errorf("failed to decode query optimization response: %w", err)
	}

	optimizedQuery := strings.TrimSpace(chatResp.Message.Content)
	if optimizedQuery == "" {
		optimizationDuration := time.Since(optimizationStart)
		metrics.RecordQueryOptimization(optimizationDuration, false)
		return userQuery, nil // Fall back to original query if optimization failed
	}

	optimizationDuration := time.Since(optimizationStart)
	metrics.RecordQueryOptimization(optimizationDuration, true)

	return optimizedQuery, nil
}
