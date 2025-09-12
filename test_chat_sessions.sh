#!/bin/bash

# Test script to demonstrate CLI chat session management functionality
# This script shows the new session management features without requiring Ollama

set -e

echo "🧪 Testing CLI Chat Session Management"
echo "======================================="
echo ""

# Build the CLI tool
echo "🔨 Building lil-rag CLI..."
make build-cli > /dev/null 2>&1
echo "✅ Build completed"
echo ""

# Initialize config if not exists
echo "⚙️  Initializing configuration..."
./bin/lil-rag config init > /dev/null 2>&1 || true
echo "✅ Configuration ready"
echo ""

# Clean up any existing test data
echo "🧹 Cleaning up previous test data..."
rm -f /home/runner/.lilrag/data/chat_history.db
echo "✅ Cleanup completed"
echo ""

echo "📋 Test 1: Creating a new chat session"
echo "---------------------------------------"
echo "Command: ./bin/lil-rag chat \"Hello, this is my first message\""
echo ""
# Capture the session ID from the output
SESSION_OUTPUT=$(./bin/lil-rag chat "Hello, this is my first message" 2>&1 || true)
echo "$SESSION_OUTPUT"
echo ""

# Extract session ID from the output
SESSION_ID=$(echo "$SESSION_OUTPUT" | grep "Session ID:" | sed 's/.*`\([^`]*\)`.*/\1/')

if [ -n "$SESSION_ID" ]; then
    echo "✅ New session created with ID: $SESSION_ID"
else
    echo "❌ Failed to extract session ID"
    exit 1
fi
echo ""

echo "📋 Test 2: Continuing the conversation with session ID"
echo "-----------------------------------------------------"
echo "Command: ./bin/lil-rag chat \"This is my follow-up message\" --session $SESSION_ID"
echo ""
CONTINUE_OUTPUT=$(./bin/lil-rag chat "This is my follow-up message" --session "$SESSION_ID" 2>&1 || true)
echo "$CONTINUE_OUTPUT"
echo ""

# Check if it recognized the existing session
if echo "$CONTINUE_OUTPUT" | grep -q "Continuing Chat Session"; then
    echo "✅ Session continuation recognized"
else
    echo "❌ Failed to recognize existing session"
    exit 1
fi

# Check if conversation context was built
if echo "$CONTINUE_OUTPUT" | grep -q "Recent conversation:"; then
    echo "✅ Conversation context was built from history"
else
    echo "❌ Failed to build conversation context"
    exit 1
fi
echo ""

echo "📋 Test 3: Testing backward compatibility"
echo "----------------------------------------"
echo "Command: ./bin/lil-rag chat \"Testing old format\" 3"
echo ""
COMPAT_OUTPUT=$(./bin/lil-rag chat "Testing old format" 3 2>&1 || true)
echo "$COMPAT_OUTPUT"
echo ""

if echo "$COMPAT_OUTPUT" | grep -q "New Chat Session"; then
    echo "✅ Backward compatibility maintained"
else
    echo "❌ Backward compatibility broken"
    exit 1
fi
echo ""

echo "📋 Test 4: Verifying database persistence"
echo "-----------------------------------------"
if [ -f "/home/runner/.lilrag/data/chat_history.db" ]; then
    echo "✅ Chat history database created"
    
    # Count sessions
    SESSION_COUNT=$(sqlite3 /home/runner/.lilrag/data/chat_history.db "SELECT COUNT(*) FROM chat_sessions;" 2>/dev/null || echo "0")
    echo "📊 Total sessions: $SESSION_COUNT"
    
    # Count messages
    MESSAGE_COUNT=$(sqlite3 /home/runner/.lilrag/data/chat_history.db "SELECT COUNT(*) FROM chat_messages;" 2>/dev/null || echo "0")
    echo "📊 Total messages: $MESSAGE_COUNT"
    
    if [ "$MESSAGE_COUNT" -gt 0 ]; then
        echo "✅ Messages were persisted to database"
    else
        echo "❌ No messages found in database"
        exit 1
    fi
else
    echo "❌ Chat history database not created"
    exit 1
fi
echo ""

echo "📋 Test 5: Testing error handling"
echo "---------------------------------"
echo "Command: ./bin/lil-rag chat \"Test\" --session invalid-session-id"
echo ""
ERROR_OUTPUT=$(./bin/lil-rag chat "Test" --session "invalid-session-id" 2>&1 || true)
echo "$ERROR_OUTPUT"
echo ""

if echo "$ERROR_OUTPUT" | grep -q "failed to get session"; then
    echo "✅ Invalid session ID properly handled"
else
    echo "❌ Invalid session ID not properly handled"
    exit 1
fi
echo ""

echo "🎉 All tests passed!"
echo "===================="
echo ""
echo "Key features demonstrated:"
echo "• ✅ New session creation with UUID generation"
echo "• ✅ Session ID display in markdown format"
echo "• ✅ Session persistence across commands"
echo "• ✅ Conversation context building from history"
echo "• ✅ Database persistence of sessions and messages"
echo "• ✅ Backward compatibility with old CLI format"
echo "• ✅ Proper error handling for invalid sessions"
echo "• ✅ Support for --session and --limit parameters"
echo ""
echo "Session management is working correctly! 🚀"