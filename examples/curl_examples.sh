#!/bin/bash

# LilRag Comprehensive API Examples using curl
# 
# This script demonstrates all major features of the lil-rag HTTP API:
# - Document indexing (text and file upload)
# - Semantic search capabilities
# - Document management (CRUD operations)
# - Chat functionality with RAG context
# - System monitoring and health checks
#
# Prerequisites:
# - lil-rag-server running on localhost:8080
# - jq installed for JSON formatting
# - Test files available (../test_pdfs/test_document.pdf, ../test_images/sample_document.png)

BASE_URL="http://localhost:8080"

echo "=== LilRag Comprehensive API Examples ==="
echo "Demonstrating all available features via HTTP API"
echo ""

# Check if server is running
echo "0. Server Connectivity Check"
echo "curl -s $BASE_URL/api/health -o /dev/null -w '%{http_code}'"
HTTP_CODE=$(curl -s "$BASE_URL/api/health" -o /dev/null -w '%{http_code}')
if [ "$HTTP_CODE" != "200" ]; then
    echo "❌ Server not responding (HTTP $HTTP_CODE). Please ensure lil-rag-server is running on $BASE_URL"
    exit 1
fi
echo "✅ Server is running"
echo ""

# Health check
echo "1. Health Check"
echo "curl $BASE_URL/api/health"
curl -s "$BASE_URL/api/health" | jq '.'
echo ""

# Index some documents
echo "2. Indexing Documents"
echo ""

documents=(
    '{"id": "doc1", "text": "Go is a programming language developed by Google. It is known for its simplicity and performance."}'
    '{"id": "doc2", "text": "Python is a high-level programming language known for its readability and versatility."}'
    '{"id": "doc3", "text": "JavaScript is the language of the web, running in browsers and servers via Node.js."}'
    '{"id": "doc4", "text": "Rust is a systems programming language focused on safety, speed, and concurrency."}'
    '{"id": "doc5", "text": "Machine learning is a subset of artificial intelligence that enables computers to learn from data."}'
)

for i in "${!documents[@]}"; do
    echo "Indexing document $((i+1))..."
    echo "curl -X POST $BASE_URL/api/index -H 'Content-Type: application/json' -d '${documents[i]}'"
    curl -s -X POST "$BASE_URL/api/index" \
        -H "Content-Type: application/json" \
        -d "${documents[i]}" | jq '.'
    echo ""
done

# Search examples
echo "3. Search Examples"
echo ""

# Search using GET
echo "Search using GET (query parameters):"
echo "curl '$BASE_URL/api/search?query=programming+languages&limit=3'"
curl -s "$BASE_URL/api/search?query=programming+languages&limit=3" | jq '.'
echo ""

# Search using POST
echo "Search using POST (JSON body):"
echo "curl -X POST $BASE_URL/api/search -H 'Content-Type: application/json' -d '{\"query\": \"artificial intelligence\", \"limit\": 2}'"
curl -s -X POST "$BASE_URL/api/search" \
    -H "Content-Type: application/json" \
    -d '{"query": "artificial intelligence", "limit": 2}' | jq '.'
echo ""

echo "Search for Google-related content:"
echo "curl -X POST $BASE_URL/api/search -H 'Content-Type: application/json' -d '{\"query\": \"Google Go\", \"limit\": 3}'"
curl -s -X POST "$BASE_URL/api/search" \
    -H "Content-Type: application/json" \
    -d '{"query": "Google Go", "limit": 3}' | jq '.'
echo ""

# File upload examples
echo "4. File Upload Examples"
echo ""

echo "Upload a PDF file (if available):"
if [ -f "../test_pdfs/test_document.pdf" ]; then
    echo "curl -X POST $BASE_URL/api/index -F 'id=pdf1' -F 'file=@../test_pdfs/test_document.pdf'"
    curl -s -X POST "$BASE_URL/api/index" \
        -F "id=pdf1" \
        -F "file=@../test_pdfs/test_document.pdf" | jq '.'
    echo ""
else
    echo "No PDF test file found, skipping..."
    echo ""
fi

echo "Upload an image file (if available):"
if [ -f "../test_images/sample_document.png" ]; then
    echo "curl -X POST $BASE_URL/api/index -F 'id=img1' -F 'file=@../test_images/sample_document.png'"
    curl -s -X POST "$BASE_URL/api/index" \
        -F "id=img1" \
        -F "file=@../test_images/sample_document.png" | jq '.'
    echo ""
else
    echo "No image test file found, skipping..."
    echo ""
fi

# Document management examples
echo "5. Document Management"
echo ""

echo "List all documents:"
echo "curl $BASE_URL/api/documents"
curl -s "$BASE_URL/api/documents" | jq '.'
echo ""

echo "Get specific document details:"
echo "curl $BASE_URL/api/documents/doc1"
curl -s "$BASE_URL/api/documents/doc1" | jq '.'
echo ""

echo "Get document content:"
echo "curl $BASE_URL/api/documents/doc1/content"
curl -s "$BASE_URL/api/documents/doc1/content" | jq '.'
echo ""

echo "Get document chunks:"
echo "curl $BASE_URL/api/documents/doc1/chunks"
curl -s "$BASE_URL/api/documents/doc1/chunks" | jq '.'
echo ""

# Chat examples
echo "6. Chat Functionality"
echo ""

echo "Create a new chat session:"
echo "curl -X POST $BASE_URL/api/chat/sessions/new"
SESSION_RESPONSE=$(curl -s -X POST "$BASE_URL/api/chat/sessions/new")
echo "$SESSION_RESPONSE" | jq '.'
SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.session_id' 2>/dev/null || echo "")

if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    echo ""
    echo "Chat with RAG context:"
    echo "curl -X POST $BASE_URL/api/chat -H 'Content-Type: application/json' -d '{\"message\": \"What programming languages are mentioned?\", \"session_id\": \"$SESSION_ID\"}'"
    curl -s -X POST "$BASE_URL/api/chat" \
        -H "Content-Type: application/json" \
        -d "{\"message\": \"What programming languages are mentioned?\", \"session_id\": \"$SESSION_ID\"}" | jq '.'
    echo ""

    echo "List chat sessions:"
    echo "curl $BASE_URL/api/chat/sessions"
    curl -s "$BASE_URL/api/chat/sessions" | jq '.'
    echo ""

    echo "Get chat history:"
    echo "curl $BASE_URL/api/chat/history/$SESSION_ID"
    curl -s "$BASE_URL/api/chat/history/$SESSION_ID" | jq '.'
    echo ""
else
    echo "Chat session creation failed or not supported, skipping chat examples..."
    echo ""
fi

# System monitoring examples
echo "7. System Monitoring"
echo ""

echo "Health check:"
echo "curl $BASE_URL/api/health"
curl -s "$BASE_URL/api/health" | jq '.'
echo ""

echo "System metrics:"
echo "curl $BASE_URL/api/metrics"
curl -s "$BASE_URL/api/metrics" | jq '.'
echo ""

# Cleanup example
echo "8. Cleanup (Delete Document)"
echo ""

echo "Delete a document:"
echo "curl -X DELETE $BASE_URL/api/documents/doc5"
curl -s -X DELETE "$BASE_URL/api/documents/doc5" | jq '.'
echo ""

echo "Verify deletion by listing documents:"
echo "curl $BASE_URL/api/documents"
curl -s "$BASE_URL/api/documents" | jq '.'
echo ""

echo "=== Comprehensive API Examples Completed ==="
echo ""
echo "This example demonstrated:"
echo "- Text and file indexing"
echo "- Semantic search (GET and POST)"
echo "- File upload (PDF and image processing)"
echo "- Document management (CRUD operations)"
echo "- Chat functionality with RAG context"
echo "- System health and metrics monitoring"
echo ""
echo "For more features, see the other examples in this directory."