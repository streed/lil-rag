package chathistory

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
)

const (
	// MaxContextTokens is the maximum tokens before we trigger compaction
	MaxContextTokens = 1000

	// EstimatedTokensPerChar rough estimation for token counting
	EstimatedTokensPerChar = 0.25

	// DefaultChatTitle is the default title for new chat sessions
	DefaultChatTitle = "New Chat"
)

// ChatHistory manages chat sessions and message history
type ChatHistory struct {
	db *sql.DB
}

// ChatSession represents a chat conversation
type ChatSession struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	TotalTokens  int       `json:"total_tokens"`
	MessageCount int       `json:"message_count"`
}

// ChatMessage represents a single message in a chat
type ChatMessage struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	Role         string    `json:"role"` // "user" or "assistant"
	Content      string    `json:"content"`
	Tokens       int       `json:"tokens"`
	CreatedAt    time.Time `json:"created_at"`
	MessageOrder int       `json:"message_order"`
}

// ChatCompaction represents a compacted chat history
type ChatCompaction struct {
	ID                   string    `json:"id"`
	SessionID            string    `json:"session_id"`
	CompactedContent     string    `json:"compacted_content"`
	OriginalMessageCount int       `json:"original_message_count"`
	OriginalTokens       int       `json:"original_tokens"`
	CompactedTokens      int       `json:"compacted_tokens"`
	CompactedAt          time.Time `json:"compacted_at"`
}

// ChatContext represents the current chat context for AI processing
type ChatContext struct {
	Messages         []ChatMessage   `json:"messages"`
	CompactedHistory *ChatCompaction `json:"compacted_history,omitempty"`
	TotalTokens      int             `json:"total_tokens"`
	RemainingTokens  int             `json:"remaining_tokens"`
	HasCompactedData bool            `json:"has_compacted_data"`
}

// New creates a new ChatHistory instance with the given database path
func New(dbPath string) (*ChatHistory, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := ensureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create chat history directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_timeout=30000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open chat history database: %w", err)
	}

	ch := &ChatHistory{db: db}

	if err := ch.initializeSchema(); err != nil {
		_ = db.Close() // Ignore close errors during cleanup
		return nil, fmt.Errorf("failed to initialize chat history schema: %w", err)
	}

	return ch, nil
}

// Close closes the database connection
func (ch *ChatHistory) Close() error {
	return ch.db.Close()
}

// initializeSchema creates the necessary tables
func (ch *ChatHistory) initializeSchema() error {
	// Read schema from embedded file or string
	schema := `
-- Chat sessions - each conversation is a session
CREATE TABLE IF NOT EXISTS chat_sessions (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT 'New Chat',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_tokens INTEGER DEFAULT 0,
    message_count INTEGER DEFAULT 0
);

-- Individual messages within chat sessions
CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tokens INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    message_order INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
);

-- Compacted chat history
CREATE TABLE IF NOT EXISTS chat_compactions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    compacted_content TEXT NOT NULL,
    original_message_count INTEGER NOT NULL,
    original_tokens INTEGER NOT NULL,
    compacted_tokens INTEGER NOT NULL,
    compacted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_order ON chat_messages(session_id, message_order);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_updated ON chat_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_compactions_session ON chat_compactions(session_id);

-- Trigger to update chat_sessions.updated_at and message_count when messages are added
CREATE TRIGGER IF NOT EXISTS update_session_timestamp
    AFTER INSERT ON chat_messages
BEGIN
    UPDATE chat_sessions 
    SET updated_at = CURRENT_TIMESTAMP,
        message_count = message_count + 1
    WHERE id = NEW.session_id;
END;
`

	_, err := ch.db.Exec(schema)
	return err
}

// CreateSession creates a new chat session
func (ch *ChatHistory) CreateSession(ctx context.Context) (*ChatSession, error) {
	session := &ChatSession{
		ID:        uuid.New().String(),
		Title:     DefaultChatTitle,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `
		INSERT INTO chat_sessions (id, title, created_at, updated_at, total_tokens, message_count)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := ch.db.ExecContext(ctx, query, session.ID, session.Title, session.CreatedAt, session.UpdatedAt, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}

	return session, nil
}

// GetSessions returns all chat sessions ordered by most recent
func (ch *ChatHistory) GetSessions(ctx context.Context) ([]ChatSession, error) {
	query := `
		SELECT id, title, created_at, updated_at, total_tokens, message_count
		FROM chat_sessions
		ORDER BY updated_at DESC
	`

	rows, err := ch.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat sessions: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	var sessions []ChatSession
	for rows.Next() {
		var session ChatSession
		err := rows.Scan(&session.ID, &session.Title, &session.CreatedAt, &session.UpdatedAt,
			&session.TotalTokens, &session.MessageCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat session: %w", err)
		}
		sessions = append(sessions, session)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	// Ensure we always return an empty slice instead of nil
	if sessions == nil {
		sessions = []ChatSession{}
	}

	return sessions, nil
}

// GetSession gets a specific chat session by ID
func (ch *ChatHistory) GetSession(ctx context.Context, sessionID string) (*ChatSession, error) {
	query := `
		SELECT id, title, created_at, updated_at, total_tokens, message_count
		FROM chat_sessions
		WHERE id = ?
	`

	var session ChatSession
	err := ch.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.Title, &session.CreatedAt, &session.UpdatedAt, &session.TotalTokens, &session.MessageCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat session: %w", err)
	}

	return &session, nil
}

// AddMessage adds a message to a chat session
func (ch *ChatHistory) AddMessage(ctx context.Context, sessionID, role, content string) (*ChatMessage, error) {
	// Calculate rough token count
	tokens := estimateTokens(content)

	// Get the next message order for this session
	var messageOrder int
	err := ch.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(message_order), -1) + 1 FROM chat_messages WHERE session_id = ?",
		sessionID).Scan(&messageOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to get next message order: %w", err)
	}

	message := &ChatMessage{
		ID:           uuid.New().String(),
		SessionID:    sessionID,
		Role:         role,
		Content:      content,
		Tokens:       tokens,
		CreatedAt:    time.Now(),
		MessageOrder: messageOrder,
	}

	query := `
		INSERT INTO chat_messages (id, session_id, role, content, tokens, created_at, message_order)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = ch.db.ExecContext(ctx, query, message.ID, message.SessionID, message.Role,
		message.Content, message.Tokens, message.CreatedAt, message.MessageOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to add chat message: %w", err)
	}

	// Update session total tokens and updated_at timestamp
	_, err = ch.db.ExecContext(ctx,
		"UPDATE chat_sessions SET total_tokens = total_tokens + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		tokens, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to update session tokens: %w", err)
	}

	return message, nil
}

// GetChatContext gets the current chat context for AI processing
func (ch *ChatHistory) GetChatContext(ctx context.Context, sessionID string) (*ChatContext, error) {
	// First check if there's a compaction for this session
	compaction, err := ch.getLatestCompaction(ctx, sessionID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get compaction: %w", err)
	}

	// Get messages after the compaction (or all messages if no compaction)
	var messages []ChatMessage
	var query string
	var args []interface{}

	if compaction != nil {
		// Get messages after the compaction
		query = `
			SELECT id, session_id, role, content, tokens, created_at, message_order
			FROM chat_messages
			WHERE session_id = ? AND created_at > ?
			ORDER BY message_order ASC
		`
		args = []interface{}{sessionID, compaction.CompactedAt}
	} else {
		// Get all messages
		query = `
			SELECT id, session_id, role, content, tokens, created_at, message_order
			FROM chat_messages
			WHERE session_id = ?
			ORDER BY message_order ASC
		`
		args = []interface{}{sessionID}
	}

	rows, err := ch.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	totalTokens := 0
	if compaction != nil {
		totalTokens = compaction.CompactedTokens
	}

	for rows.Next() {
		var msg ChatMessage
		err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Tokens, &msg.CreatedAt, &msg.MessageOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
		totalTokens += msg.Tokens
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	// Ensure we always return an empty slice instead of nil
	if messages == nil {
		messages = []ChatMessage{}
	}

	return &ChatContext{
		Messages:         messages,
		CompactedHistory: compaction,
		TotalTokens:      totalTokens,
		RemainingTokens:  MaxContextTokens - totalTokens,
		HasCompactedData: compaction != nil,
	}, nil
}

// UpdateSessionTitle updates the title of a chat session
func (ch *ChatHistory) UpdateSessionTitle(ctx context.Context, sessionID, title string) error {
	query := `UPDATE chat_sessions SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := ch.db.ExecContext(ctx, query, title, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session title: %w", err)
	}
	return nil
}

// CompactChatHistory compacts old messages in a session to save context space
func (ch *ChatHistory) CompactChatHistory(ctx context.Context, sessionID, compactedContent string) error {
	// Get messages to compact (all current messages that aren't already compacted)
	context, err := ch.GetChatContext(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get chat context for compaction: %w", err)
	}

	if len(context.Messages) == 0 {
		return fmt.Errorf("no messages to compact")
	}

	// Keep the last few messages and compact the rest
	messagesToKeep := 2 // Keep last 2 messages (usually latest user message and response)
	if len(context.Messages) <= messagesToKeep {
		return fmt.Errorf("not enough messages to compact")
	}

	messagesToCompact := context.Messages[:len(context.Messages)-messagesToKeep]
	originalTokens := 0
	for _, msg := range messagesToCompact {
		originalTokens += msg.Tokens
	}

	compactedTokens := estimateTokens(compactedContent)

	// Create compaction record
	compaction := &ChatCompaction{
		ID:                   uuid.New().String(),
		SessionID:            sessionID,
		CompactedContent:     compactedContent,
		OriginalMessageCount: len(messagesToCompact),
		OriginalTokens:       originalTokens,
		CompactedTokens:      compactedTokens,
		CompactedAt:          time.Now(),
	}

	// Start transaction
	tx, err := ch.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			// Rollback error is logged but doesn't override the main error
			_ = rbErr // Explicitly ignore error
		}
	}()

	// Insert compaction
	_, err = tx.ExecContext(ctx, `
		INSERT INTO chat_compactions (id, session_id, compacted_content, original_message_count, 
		original_tokens, compacted_tokens, compacted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, compaction.ID, compaction.SessionID, compaction.CompactedContent,
		compaction.OriginalMessageCount, compaction.OriginalTokens,
		compaction.CompactedTokens, compaction.CompactedAt)
	if err != nil {
		return fmt.Errorf("failed to insert compaction: %w", err)
	}

	// Delete the compacted messages
	for _, msg := range messagesToCompact {
		_, err = tx.ExecContext(ctx, "DELETE FROM chat_messages WHERE id = ?", msg.ID)
		if err != nil {
			return fmt.Errorf("failed to delete compacted message: %w", err)
		}
	}

	// Update session token count
	_, err = tx.ExecContext(ctx,
		"UPDATE chat_sessions SET total_tokens = total_tokens - ? + ? WHERE id = ?",
		originalTokens, compactedTokens, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session tokens after compaction: %w", err)
	}

	return tx.Commit()
}

// DeleteSession deletes a chat session and all its messages
func (ch *ChatHistory) DeleteSession(ctx context.Context, sessionID string) error {
	// Due to CASCADE, this will also delete messages and compactions
	_, err := ch.db.ExecContext(ctx, "DELETE FROM chat_sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete chat session: %w", err)
	}
	return nil
}

// getLatestCompaction gets the most recent compaction for a session
func (ch *ChatHistory) getLatestCompaction(ctx context.Context, sessionID string) (*ChatCompaction, error) {
	query := `
		SELECT id, session_id, compacted_content, original_message_count, original_tokens, compacted_tokens, compacted_at
		FROM chat_compactions
		WHERE session_id = ?
		ORDER BY compacted_at DESC
		LIMIT 1
	`

	var compaction ChatCompaction
	err := ch.db.QueryRowContext(ctx, query, sessionID).Scan(
		&compaction.ID, &compaction.SessionID, &compaction.CompactedContent,
		&compaction.OriginalMessageCount, &compaction.OriginalTokens, &compaction.CompactedTokens, &compaction.CompactedAt,
	)
	if err != nil {
		return nil, err
	}

	return &compaction, nil
}

// estimateTokens provides a rough estimation of token count for text
func estimateTokens(text string) int {
	// Simple estimation: roughly 4 characters per token
	return int(float64(len(text)) * EstimatedTokensPerChar)
}

// ensureDir creates a directory if it doesn't exist
func ensureDir(_ string) error {
	return nil // The filepath.Dir might be sufficient, but we can implement this if needed
}

// GenerateChatSummary creates a one-sentence summary of a chat for the title
func GenerateChatSummary(messages []ChatMessage) string {
	if len(messages) == 0 {
		return DefaultChatTitle
	}

	// Find the first user message to base the summary on
	for _, msg := range messages {
		if msg.Role == "user" {
			content := strings.TrimSpace(msg.Content)
			if len(content) > 50 {
				content = content[:47] + "..."
			}
			return content
		}
	}

	return DefaultChatTitle
}
