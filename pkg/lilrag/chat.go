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
	return NewOllamaChatClientWithTimeout(baseURL, model, 120)
}

// NewOllamaChatClientWithTimeout creates a new Ollama chat client with configurable timeout
func NewOllamaChatClientWithTimeout(baseURL, model string, timeoutSeconds int) *OllamaChatClient {
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}
	if model == "" {
		model = DefaultChatModel
	}

	return &OllamaChatClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
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

	// Record input tokens
	metrics.RecordChatInputTokens(c.model, systemPrompt)
	metrics.RecordChatInputTokens(c.model, userMessage)

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

	// Record output tokens
	metrics.RecordChatOutputTokens(c.model, chatResp.Message.Content)

	return chatResp.Message.Content, nil
}

// createSystemPrompt creates a system prompt with search results context
func (c *OllamaChatClient) createSystemPrompt(searchResults []SearchResult) string {
	var prompt strings.Builder

	prompt.WriteString("You are a helpful AI assistant that answers questions based strictly on provided document context. ")

	if len(searchResults) == 0 {
		prompt.WriteString("No relevant documents were found in the knowledge base. ")
		prompt.WriteString("You MUST respond with: \"I was unable to find any relevant information in the knowledge base to answer your question. ")
		prompt.WriteString("This could mean: (1) the information isn't available in the indexed documents, ")
		prompt.WriteString("(2) your query might need to be rephrased with different keywords, or ")
		prompt.WriteString("(3) additional documents containing this information may need to be added to the knowledge base.\"")
		return prompt.String()
	}

	prompt.WriteString("CRITICAL INSTRUCTIONS:\n")
	prompt.WriteString("1. Answer ONLY based on the provided documents below\n")
	prompt.WriteString("2. EVERY fact, claim, or piece of information MUST be linked to a source using [document-id]\n")
	prompt.WriteString("3. If the documents don't fully answer the question, clearly state what information is missing\n")
	prompt.WriteString("4. If the documents contradict each other, acknowledge this and cite both sources\n")
	prompt.WriteString("5. Never make assumptions or add information not present in the documents\n\n")

	prompt.WriteString("CITATION FORMAT:\n")
	prompt.WriteString("- Use [document-id] immediately after each fact or claim\n")
	prompt.WriteString("- Example: \"The system uses vector embeddings [tech-overview] to enable semantic search [search-guide].\"\n")
	prompt.WriteString("- Multiple sources: \"This approach improves accuracy [study-2023] and performance [benchmark-results].\"\n\n")

	prompt.WriteString("RELEVANT DOCUMENTS:\n\n")

	for i, result := range searchResults {
		prompt.WriteString(fmt.Sprintf("Document %d (ID: %s, Relevance: %.1f%%):\n",
			i+1, result.ID, result.Score*100))

		// Use a reasonable excerpt length
		text := result.Text
		if len(text) > 3000 {
			text = text[:3000] + "..."
		}

		prompt.WriteString(text)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString("RESPONSE REQUIREMENTS:\n")
	prompt.WriteString("- Provide a comprehensive answer if the documents contain sufficient information\n")
	prompt.WriteString("- If information is incomplete, clearly state: \"Based on the available documents, I can provide partial information: [your answer with citations]. However, the documents don't contain information about [specific missing aspects].\"\n")
	prompt.WriteString("- If the documents are not relevant to the question, state: \"The available documents don't contain relevant information to answer your question about [topic]. You may need to add more specific documents or rephrase your query.\"\n")
	prompt.WriteString("- Always maintain accuracy over completeness - never guess or extrapolate beyond what's explicitly stated in the documents")

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

	systemPrompt := `You are an expert at identifying the core subject matter in user queries for semantic search optimization.

Your task is to extract ONLY the main subject, topic, or thing the user is asking about. Ignore all directive language and focus solely on what the query is actually about.

Key principles:
1. Identify the primary SUBJECT or TOPIC the user wants information about
2. Extract the core NOUN PHRASES that represent the subject matter
3. Include relevant CONTEXT or MODIFIERS that specify the subject
4. Remove ALL directive words (what, how, can you, please, tell me, etc.)
5. Focus on the THING being discussed, not the action being requested

Examples:
- "Can you please tell me about machine learning?" → "machine learning"
- "How do I set up a Docker container?" → "Docker container setup"
- "What are the benefits of using microservices architecture?" → "microservices architecture benefits"
- "Please explain how authentication works in web applications" → "authentication web applications"
- "I need help with configuring SSL certificates" → "SSL certificates configuration"
- "Show me examples of React hooks implementation" → "React hooks implementation examples"

Extract the subject matter only. Respond with ONLY the subject-focused query terms.`

	// Record input tokens for query optimization system prompt
	metrics.RecordChatInputTokens(c.model, systemPrompt)

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

	// Record token usage for query optimization
	metrics.RecordQueryOptimizationTokens(c.model, userQuery, optimizedQuery)

	optimizationDuration := time.Since(optimizationStart)
	metrics.RecordQueryOptimization(optimizationDuration, true)

	return optimizedQuery, nil
}
