#!/bin/bash

# MiniRag API Examples using curl

BASE_URL="http://localhost:8080"

echo "=== MiniRag API Examples ==="
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

echo "=== Examples completed ==="