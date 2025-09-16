package chathistory

import (
	"context"
	"fmt"
	"strings"
)

// ChatSummarizer interface for generating chat summaries and compactions
type ChatSummarizer interface {
	GenerateTitle(ctx context.Context, messages []ChatMessage) (string, error)
	CompactHistory(ctx context.Context, messages []ChatMessage) (string, error)
}

// LLMSummarizer implements ChatSummarizer using an LLM service
type LLMSummarizer struct {
	llmClient LLMClient
}

// LLMClient interface for making LLM requests (to be implemented by the chat client)
type LLMClient interface {
	GenerateResponse(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// NewLLMSummarizer creates a new LLM-based summarizer
func NewLLMSummarizer(client LLMClient) *LLMSummarizer {
	return &LLMSummarizer{llmClient: client}
}

// GenerateTitle creates a concise title for a chat session
func (s *LLMSummarizer) GenerateTitle(ctx context.Context, messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return DefaultChatTitle, nil
	}

	// Take first few messages to understand the topic
	conversationText := formatMessagesForSummary(messages, 3)
	if conversationText == "" {
		return DefaultChatTitle, nil
	}

	systemPrompt := `You are an expert at creating concise, descriptive titles for chat conversations. 

Your task is to read a conversation and create a single, clear title that captures the main topic or 
question being discussed.

Requirements:
- Maximum 8 words
- No quotes or special characters
- Focus on the main topic, not the action
- Be specific but concise
- Use title case

Examples:
- "How to configure Docker containers" → "Docker Container Configuration"
- "Debugging Python memory issues" → "Python Memory Issue Debugging"
- "Best practices for API design" → "API Design Best Practices"
- "Setting up CI/CD pipeline" → "CI/CD Pipeline Setup"

Respond with ONLY the title, no explanations.`

	userPrompt := fmt.Sprintf("Create a title for this conversation:\n\n%s", conversationText)

	title, err := s.llmClient.GenerateResponse(ctx, systemPrompt, userPrompt)
	if err != nil {
		// Fallback to simple title generation
		return generateFallbackTitle(messages), nil
	}

	title = strings.TrimSpace(title)
	if title == "" || len(title) > 100 {
		return generateFallbackTitle(messages), nil
	}

	return title, nil
}

// CompactHistory creates a compact summary of chat messages
func (s *LLMSummarizer) CompactHistory(ctx context.Context, messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages to compact")
	}

	conversationText := formatMessagesForSummary(messages, -1) // Include all messages

	systemPrompt := `You are an expert at summarizing chat conversations while preserving important context.

Your task is to create a concise summary of the conversation that:
1. Captures the main topics and questions discussed
2. Preserves key technical details and decisions made
3. Maintains the logical flow of the conversation
4. Removes redundant or unimportant details
5. Uses clear, professional language

The summary will be used as context for continuing the conversation, so it must:
- Include relevant facts and information shared
- Note any problems solved or decisions made
- Mention important preferences or requirements stated
- Preserve technical context and terminology

Format the summary as a coherent narrative, not bullet points.
Keep it focused and under 200 words.`

	userPrompt := fmt.Sprintf("Summarize this conversation:\n\n%s", conversationText)

	summary, err := s.llmClient.GenerateResponse(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate chat summary: %w", err)
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		return "", fmt.Errorf("received empty summary from LLM")
	}

	return summary, nil
}

// formatMessagesForSummary formats messages for LLM processing
func formatMessagesForSummary(messages []ChatMessage, limit int) string {
	var parts []string

	count := 0
	for _, msg := range messages {
		if limit > 0 && count >= limit {
			break
		}

		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}

		content := strings.TrimSpace(msg.Content)
		if content != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", role, content))
			count++
		}
	}

	return strings.Join(parts, "\n\n")
}

// generateFallbackTitle creates a simple title when LLM is unavailable
func generateFallbackTitle(messages []ChatMessage) string {
	// Find the first substantial user message
	for _, msg := range messages {
		if msg.Role == "user" {
			content := strings.TrimSpace(msg.Content)
			if len(content) > 10 {
				// Extract first sentence or up to 50 characters
				words := strings.Fields(content)
				if len(words) > 8 {
					return strings.Join(words[:8], " ") + "..."
				}
				if len(content) > 50 {
					return content[:47] + "..."
				}
				return content
			}
		}
	}
	return DefaultChatTitle
}
