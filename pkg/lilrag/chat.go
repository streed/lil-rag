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

// StreamingChatHandler is a function type for handling streaming chat chunks
type StreamingChatHandler func(chunk string, done bool) error

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
	defer func() {
		_ = resp.Body.Close() // Ignore close errors in defer
	}()

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

// GenerateResponseStreaming generates a chat response using streaming API with provided context and user message
func (c *OllamaChatClient) GenerateResponseStreaming(ctx context.Context, userMessage string,
	searchResults []SearchResult, handler StreamingChatHandler) error {
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

	// Create request with streaming enabled
	requestBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true, // Enable streaming
		Options: &ChatOptions{
			Temperature: 0.7,
			TopP:        0.9,
		},
	}

	// Marshal request
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal chat request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send chat request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("ollama server returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("chat request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Process streaming response
	decoder := json.NewDecoder(resp.Body)
	var completeResponse strings.Builder

	for {
		var chatResp ChatResponse
		if err := decoder.Decode(&chatResp); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode streaming chat response: %w", err)
		}

		// Add this chunk to the complete response
		chunk := chatResp.Message.Content
		completeResponse.WriteString(chunk)

		// Call the handler with the chunk
		if err := handler(chunk, chatResp.Done); err != nil {
			return fmt.Errorf("handler error: %w", err)
		}

		// Break if this is the final chunk
		if chatResp.Done {
			break
		}
	}

	// Record output tokens for the complete response
	finalResponse := completeResponse.String()
	metrics.RecordChatOutputTokens(c.model, finalResponse)

	return nil
}

// createSystemPrompt creates a system prompt with search results context
func (c *OllamaChatClient) createSystemPrompt(searchResults []SearchResult) string {
	var prompt strings.Builder

	prompt.WriteString("You are a helpful AI assistant that answers questions based strictly on " +
		"provided document context. ")

	if len(searchResults) == 0 {
		prompt.WriteString("No relevant documents were found in the knowledge base. ")
		prompt.WriteString("You MUST respond with: \"I was unable to find any relevant information in " +
			"the knowledge base to answer your question. ")
		prompt.WriteString("This could mean: (1) the information isn't available in the indexed documents, ")
		prompt.WriteString("(2) your query might need to be rephrased with different keywords, or ")
		prompt.WriteString("(3) additional documents containing this information may need to be " +
			"added to the knowledge base.\"")
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
	prompt.WriteString("- Example: \"The system uses vector embeddings [tech-overview] to enable " +
		"semantic search [search-guide].\"\n")
	prompt.WriteString("- Multiple sources: \"This approach improves accuracy [study-2023] and " +
		"performance [benchmark-results].\"\n\n")

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
	prompt.WriteString("- If information is incomplete, clearly state: \"Based on the available documents, " +
		"I can provide partial information: [your answer with citations]. However, the documents don't " +
		"contain information about [specific missing aspects].\"\n")
	prompt.WriteString("- If the documents are not relevant to the question, state: \"The available " +
		"documents don't contain relevant information to answer your question about [topic]. You may " +
		"need to add more specific documents or rephrase your query.\"\n")
	prompt.WriteString("- Always maintain accuracy over completeness - never guess or extrapolate " +
		"beyond what's explicitly stated in the documents")

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
	defer func() {
		_ = resp.Body.Close() // Ignore close errors in defer
	}()

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

	// Preprocess the query to remove obvious chat history references
	preprocessedQuery := c.preprocessUserQuery(userQuery)
	if preprocessedQuery == "" {
		// If preprocessing removed everything, fall back to local optimization
		return c.localQueryOptimization(userQuery), nil
	}

	optimizationStart := time.Now()

	systemPrompt := `You are an expert at identifying the core subject matter in user queries for 
semantic search optimization.

Your task is to extract ONLY the main subject, topic, or thing the user is asking about. Ignore all 
directive language and focus solely on what the query is actually about.

OPTIMIZATION STRATEGIES:
1. EXTRACT CORE CONCEPTS: Identify the main subjects, topics, and entities
2. EXPAND KEY TERMS: Include synonyms, alternative phrases, and related concepts
3. ADD CONTEXT: Include domain-specific terminology and technical terms
4. REMOVE NOISE: Strip out conversational elements, directive words, and filler
5. STRUCTURE FOR SEARCH: Organize terms for optimal semantic matching

TRANSFORMATION RULES:
- Convert questions into declarative search terms
- Include both broad concepts and specific details
- Add relevant synonyms and alternative terminology
- Preserve technical terms and proper nouns
- Remove: "what", "how", "can you", "please", "tell me", "explain", "show me", "you said", "mentioned", "earlier", "before", "previous"
- Remove: references to prior conversations, responses, or chat history
- Keep: domain terms, technical concepts, specific entities, actionable contexts

EXAMPLES:
Input: "Can you please tell me about machine learning algorithms?"
Output: "machine learning algorithms artificial intelligence ML neural networks supervised unsupervised deep learning"

Input: "How do I configure SSL certificates for web servers?"
Output: "SSL certificates configuration web servers HTTPS TLS certificate installation setup security"

Input: "What are the best practices for React component testing?"
Output: "React component testing best practices unit tests Jest testing library enzyme component testing patterns"

Input: "You mentioned database indexing earlier, can you explain performance optimization?"
Output: "database indexing performance optimization B-tree indexes query optimization database performance tuning"

Input: "Following up on your previous response about APIs, how do I handle authentication?"
Output: "API authentication authorization OAuth JWT token security access control"

Respond with ONLY the optimized search terms, separated by spaces. Include 8-15 relevant terms that cover the core concept plus related terminology.`

	// Record input tokens for query optimization system prompt
	metrics.RecordChatInputTokens(c.model, systemPrompt)

	// Build chat messages for query optimization using preprocessed query
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: preprocessedQuery,
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
	defer func() {
		_ = resp.Body.Close() // Ignore close errors in defer
	}()

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
		// Fall back to local optimization with preprocessed query if available, otherwise original
		fallbackQuery := preprocessedQuery
		if fallbackQuery == "" {
			fallbackQuery = userQuery
		}
		return c.localQueryOptimization(fallbackQuery), nil
	}

	// Record token usage for query optimization
	metrics.RecordQueryOptimizationTokens(c.model, preprocessedQuery, optimizedQuery)

	optimizationDuration := time.Since(optimizationStart)
	metrics.RecordQueryOptimization(optimizationDuration, true)

	return optimizedQuery, nil
}

// localQueryOptimization provides a fallback query optimization when LLM optimization fails
func (c *OllamaChatClient) localQueryOptimization(userQuery string) string {
	if userQuery == "" {
		return userQuery
	}

	// Apply basic query optimization techniques
	optimized := userQuery

	// 1. Remove common stop words, directive phrases, and chat history references
	stopPhrases := []string{
		"can you", "please", "tell me", "explain", "show me", "how do", "what is", "what are",
		"help me", "i need", "i want", "could you", "would you", "can you please",
		"you said", "you mentioned", "earlier", "before", "previous", "previously",
		"in your response", "your answer", "last time", "just now", "above",
		"following up", "based on", "regarding your", "about what you",
	}

	for _, phrase := range stopPhrases {
		optimized = strings.ReplaceAll(strings.ToLower(optimized), phrase, "")
	}

	// 2. Clean up extra whitespace
	optimized = strings.Join(strings.Fields(optimized), " ")

	// 3. Extract key terms and add synonyms for common technical concepts
	synonymMap := map[string][]string{
		"ai":              {"artificial intelligence", "machine learning", "ML"},
		"ml":              {"machine learning", "artificial intelligence", "AI"},
		"database":        {"DB", "data storage", "DBMS"},
		"api":             {"REST", "endpoint", "web service"},
		"authentication":  {"auth", "login", "security", "access control"},
		"configuration":   {"config", "setup", "settings"},
		"performance":     {"optimization", "speed", "efficiency"},
		"troubleshooting": {"debugging", "problem solving", "error resolution"},
		"implementation":  {"development", "coding", "programming"},
		"best practices":  {"recommendations", "guidelines", "standards"},
	}

	// Add relevant synonyms
	words := strings.Fields(strings.ToLower(optimized))
	var expandedTerms []string
	expandedTerms = append(expandedTerms, words...)

	for _, word := range words {
		if synonyms, exists := synonymMap[word]; exists {
			expandedTerms = append(expandedTerms, synonyms...)
		}
	}

	// 4. Remove duplicates and join
	seen := make(map[string]bool)
	var uniqueTerms []string
	for _, term := range expandedTerms {
		if term != "" && !seen[term] && len(term) > 1 {
			seen[term] = true
			uniqueTerms = append(uniqueTerms, term)
		}
	}

	result := strings.Join(uniqueTerms, " ")

	// Return original if optimization made it too short
	if len(result) < len(userQuery)/2 {
		return userQuery
	}

	return result
}

// preprocessUserQuery removes obvious chat history references before optimization
func (c *OllamaChatClient) preprocessUserQuery(userQuery string) string {
	if userQuery == "" {
		return userQuery
	}

	query := strings.TrimSpace(userQuery)

	// Remove chat history references using simple string replacements for performance
	cleanQuery := query

	// Remove phrases that indicate references to previous conversation
	referencePatterns := []struct {
		pattern string
		replace string
	}{
		{"you said", ""},
		{"you mentioned", ""},
		{"you told me", ""},
		{"you explained", ""},
		{"you answered", ""},
		{"earlier", ""},
		{"before", ""},
		{"previously", ""},
		{"above", ""},
		{"last time", ""},
		{"just now", ""},
		{"in your response", ""},
		{"in your answer", ""},
		{"in your reply", ""},
		{"following up on your previous response", ""},
		{"following up on", ""},
		{"following up", ""},
		{"based on what you said", ""},
		{"based on what you mentioned", ""},
		{"based on", ""},
		{"regarding your", ""},
		{"what you said", ""},
		{"what you mentioned", ""},
		{"what you told", ""},
		{"about what you", ""},
	}

	// Apply replacements in a case-insensitive manner
	for _, pattern := range referencePatterns {
		cleanQuery = replaceCaseInsensitive(cleanQuery, pattern.pattern, pattern.replace)
	}

	// Clean up starting conjunctions that often follow removed content
	startingWords := []string{"and ", "also ", "additionally ", "furthermore ", "but ", "however "}
	for _, word := range startingWords {
		if strings.HasPrefix(strings.ToLower(cleanQuery), word) {
			cleanQuery = strings.TrimSpace(cleanQuery[len(word):])
		}
	}

	// Clean up extra whitespace and punctuation
	cleanQuery = strings.Join(strings.Fields(cleanQuery), " ")
	cleanQuery = strings.Trim(cleanQuery, " .,!?;:")

	return cleanQuery
}

// replaceCaseInsensitive performs case-insensitive string replacement
func replaceCaseInsensitive(text, old, replacement string) string {
	// Find all occurrences and replace them preserving original case in non-matching parts
	result := text
	for {
		index := strings.Index(strings.ToLower(result), strings.ToLower(old))
		if index == -1 {
			break
		}

		// Replace the found occurrence
		before := result[:index]
		after := result[index+len(old):]
		result = before + replacement + after
	}

	return result
}
