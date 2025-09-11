-- Chat History Database Schema
-- This database stores chat sessions, messages, and compacted history

-- Chat sessions - each conversation is a session
CREATE TABLE IF NOT EXISTS chat_sessions (
    id TEXT PRIMARY KEY,                    -- UUID for the session
    title TEXT NOT NULL DEFAULT 'New Chat', -- Summary title of the chat
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_tokens INTEGER DEFAULT 0,        -- Running total of tokens used
    message_count INTEGER DEFAULT 0        -- Number of messages in this session
);

-- Individual messages within chat sessions
CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,                    -- UUID for the message
    session_id TEXT NOT NULL,              -- Reference to chat_sessions.id
    role TEXT NOT NULL,                    -- 'user' or 'assistant'
    content TEXT NOT NULL,                 -- The actual message content
    tokens INTEGER DEFAULT 0,              -- Token count for this message
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    message_order INTEGER NOT NULL,        -- Order within the session (0, 1, 2, ...)
    FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
);

-- Compacted chat history - when history gets too long, we compact it
CREATE TABLE IF NOT EXISTS chat_compactions (
    id TEXT PRIMARY KEY,                    -- UUID for the compaction
    session_id TEXT NOT NULL,              -- Reference to chat_sessions.id
    compacted_content TEXT NOT NULL,       -- The compacted/summarized content
    original_message_count INTEGER NOT NULL, -- How many messages were compacted
    original_tokens INTEGER NOT NULL,      -- Total tokens of compacted messages
    compacted_tokens INTEGER NOT NULL,     -- Tokens in the compacted version
    compacted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES chat_sessions(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_chat_messages_session_order ON chat_messages(session_id, message_order);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_updated ON chat_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_compactions_session ON chat_compactions(session_id);

-- Trigger to update chat_sessions.updated_at when messages are added
CREATE TRIGGER IF NOT EXISTS update_session_timestamp
    AFTER INSERT ON chat_messages
BEGIN
    UPDATE chat_sessions 
    SET updated_at = CURRENT_TIMESTAMP,
        message_count = message_count + 1
    WHERE id = NEW.session_id;
END;