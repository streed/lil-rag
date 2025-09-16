//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"lil-rag/pkg/lilrag"
)

// This example demonstrates lil-rag's chat functionality:
// - RAG-powered conversations with document context
// - Chat session management
// - Context preservation across messages
// - Source citation in responses
// - Different chat models and their capabilities
//
// The chat system retrieves relevant documents to provide context-aware responses.

func main() {
	fmt.Println("=== LilRag Chat Functionality Example ===")
	fmt.Println("Demonstrating RAG-powered conversational AI")
	fmt.Println()

	// Initialize LilRag with chat model
	config := &lilrag.Config{
		DatabasePath:   "chat_example.db",
		DataDir:        "./data_chat",
		OllamaURL:      "http://localhost:11434",
		Model:          "nomic-embed-text", // Embedding model for retrieval
		ChatModel:      "llama3.2:3b",      // Chat model for conversations
		VisionModel:    "llama3.2-vision",  // Vision model for image understanding
		VectorSize:     768,
		TimeoutSeconds: 30,
		MaxTokens:      256,
		Overlap:        38,
	}

	rag, err := lilrag.New(config)
	if err != nil {
		log.Fatal("Failed to create LilRag:", err)
	}
	defer rag.Close()

	if err := rag.Initialize(); err != nil {
		log.Fatal("Failed to initialize LilRag:", err)
	}

	ctx := context.Background()

	// First, let's create a knowledge base to chat about
	fmt.Println("1. Building Knowledge Base")
	fmt.Println(strings.Repeat("-", 50))

	knowledgeBase := []struct {
		id      string
		topic   string
		content string
	}{
		{
			"programming_basics",
			"Programming Fundamentals",
			"Programming is the process of creating instructions for computers to execute. Key concepts include variables (storage for data), functions (reusable code blocks), loops (repetitive execution), and conditionals (decision-making). Popular programming languages include Python (known for readability), JavaScript (web development), Go (systems programming), and Rust (memory safety).",
		},
		{
			"ai_overview",
			"Artificial Intelligence",
			"Artificial Intelligence (AI) is the simulation of human intelligence in machines. It includes machine learning (algorithms that improve through experience), deep learning (neural networks with multiple layers), natural language processing (understanding human language), and computer vision (interpreting visual data). AI applications range from chatbots and recommendation systems to autonomous vehicles and medical diagnosis.",
		},
		{
			"web_development",
			"Web Development",
			"Web development involves creating websites and web applications. Frontend development uses HTML (structure), CSS (styling), and JavaScript (interactivity). Backend development handles server logic, databases, and APIs. Modern frameworks include React and Vue.js for frontend, Node.js and Express for backend. Key concepts include responsive design, RESTful APIs, database integration, and user authentication.",
		},
		{
			"data_science",
			"Data Science",
			"Data science combines statistics, programming, and domain expertise to extract insights from data. The process includes data collection, cleaning, analysis, and visualization. Tools include Python (pandas, scikit-learn), R (statistical analysis), SQL (database queries), and Jupyter notebooks (interactive development). Machine learning models help predict outcomes and identify patterns in large datasets.",
		},
		{
			"cloud_computing",
			"Cloud Computing",
			"Cloud computing provides on-demand access to computing resources over the internet. Service models include Infrastructure as a Service (IaaS), Platform as a Service (PaaS), and Software as a Service (SaaS). Major providers are AWS, Google Cloud, and Microsoft Azure. Benefits include scalability, cost efficiency, and reduced maintenance. Key services include virtual machines, databases, storage, and serverless computing.",
		},
	}

	// Index the knowledge base
	for _, doc := range knowledgeBase {
		fmt.Printf("Indexing: %s\n", doc.topic)
		if err := rag.Index(ctx, doc.content, doc.id); err != nil {
			fmt.Printf("  ❌ Failed to index %s: %v\n", doc.id, err)
		} else {
			fmt.Printf("  ✅ Successfully indexed %s\n", doc.topic)
		}
	}
	fmt.Println()

	fmt.Println("2. Interactive Chat Examples")
	fmt.Println(strings.Repeat("-", 50))

	// Simulate different types of conversations
	chatSessions := []struct {
		name     string
		messages []string
	}{
		{
			"Programming Beginner Questions",
			[]string{
				"What is programming and why is it important?",
				"What programming language should I learn first?",
				"Can you explain what functions are in programming?",
			},
		},
		{
			"AI and Machine Learning Discussion",
			[]string{
				"What's the difference between AI and machine learning?",
				"How does deep learning work?",
				"What are some real-world applications of AI?",
			},
		},
		{
			"Technical Implementation Questions",
			[]string{
				"I want to build a web application. What technologies should I use?",
				"How do I connect my web app to a database?",
				"What's the difference between frontend and backend development?",
			},
		},
	}

	for sessionIdx, session := range chatSessions {
		fmt.Printf("📱 Chat Session %d: %s\n", sessionIdx+1, session.name)
		fmt.Println(strings.Repeat("─", 40))

		// Start a conversation
		for msgIdx, message := range session.messages {
			fmt.Printf("👤 User: %s\n", message)

			// Get RAG response
			response, sources, err := rag.Chat(ctx, message, 3)
			if err != nil {
				fmt.Printf("❌ Chat failed: %v\n", err)
				continue
			}

			fmt.Printf("🤖 Assistant: %s\n", response)

			// Show sources if available
			if len(sources) > 0 {
				fmt.Printf("📚 Sources: ")
				sourceIDs := make([]string, len(sources))
				for i, source := range sources {
					sourceIDs[i] = source.ID
				}
				fmt.Printf("%s\n", strings.Join(sourceIDs, ", "))
			}

			fmt.Println()

			// Add a small delay to simulate natural conversation
			if msgIdx < len(session.messages)-1 {
				time.Sleep(500 * time.Millisecond)
			}
		}
		fmt.Println()
	}

	fmt.Println("3. Advanced Chat Features")
	fmt.Println(strings.Repeat("-", 50))

	// Demonstrate context preservation
	fmt.Println("🔄 Context Preservation Example:")

	contextMessages := []string{
		"Tell me about Python programming",
		"What makes it different from other languages?",
		"Can it be used for web development?",
		"What about data science applications?",
	}

	for i, message := range contextMessages {
		fmt.Printf("👤 Message %d: %s\n", i+1, message)

		response, sources, err := rag.Chat(ctx, message, 3)
		if err != nil {
			fmt.Printf("❌ Chat failed: %v\n", err)
			continue
		}

		fmt.Printf("🤖 Response: %s\n", response)
		if len(sources) > 0 {
			fmt.Printf("📚 Sources: %s\n", sources[0].ID)
		}
		fmt.Println()
	}

	fmt.Println("4. Search vs Chat Comparison")
	fmt.Println(strings.Repeat("-", 50))

	query := "machine learning applications"

	// Show search results
	fmt.Printf("🔍 Search Results for '%s':\n", query)
	searchResults, err := rag.Search(ctx, query, 3)
	if err != nil {
		fmt.Printf("❌ Search failed: %v\n", err)
	} else {
		for i, result := range searchResults {
			fmt.Printf("  %d. %s (Score: %.4f)\n", i+1, result.ID, result.Score)
			preview := result.Text
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("     %s\n", preview)
		}
	}
	fmt.Println()

	// Show chat response
	fmt.Printf("💬 Chat Response for '%s':\n", query)
	chatResponse, sources, err := rag.Chat(ctx, "What are some applications of machine learning?", 3)
	if err != nil {
		fmt.Printf("❌ Chat failed: %v\n", err)
	} else {
		fmt.Printf("🤖 %s\n", chatResponse)
		if len(sources) > 0 {
			fmt.Printf("📚 Used %d source(s)\n", len(sources))
		}
	}
	fmt.Println()

	fmt.Println("5. Chat Quality Factors")
	fmt.Println(strings.Repeat("-", 50))

	// Test different types of queries to show chat capabilities
	testQueries := []struct {
		query string
		type_ string
	}{
		{"What is the simplest programming language to learn?", "Recommendation"},
		{"Compare Python and JavaScript for web development", "Comparison"},
		{"Explain how neural networks work in simple terms", "Explanation"},
		{"What are the steps to become a data scientist?", "Process/Guidance"},
		{"Can you give me an example of using APIs in web development?", "Example Request"},
	}

	for _, tq := range testQueries {
		fmt.Printf("📝 Query Type: %s\n", tq.type_)
		fmt.Printf("👤 Question: %s\n", tq.query)

		response, sources, err := rag.Chat(ctx, tq.query, 3)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			fmt.Printf("🤖 Response: %s\n", response)
			fmt.Printf("📊 Retrieved %d relevant documents\n", len(sources))
		}
		fmt.Println()
	}

	fmt.Println("6. Performance and Monitoring")
	fmt.Println(strings.Repeat("-", 50))

	// Show some statistics
	docs, err := rag.ListDocuments(ctx)
	if err == nil {
		fmt.Printf("📊 Knowledge Base Statistics:\n")
		fmt.Printf("   Documents: %d\n", len(docs))

		totalChunks := 0
		for _, doc := range docs {
			totalChunks += doc.ChunkCount
		}

		fmt.Printf("   Total chunks: %d\n", totalChunks)
		fmt.Printf("   Average chunks per document: %.1f\n", float64(totalChunks)/float64(len(docs)))
	}
	fmt.Println()

	fmt.Println("=== Chat Example Complete ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- RAG-powered conversational AI")
	fmt.Println("- Context-aware responses using document retrieval")
	fmt.Println("- Session management for multi-turn conversations")
	fmt.Println("- Source citation and transparency")
	fmt.Println("- Different types of queries and responses")
	fmt.Println("- Comparison between search and chat interfaces")
	fmt.Println("- Knowledge base building and statistics")
	fmt.Println()
	fmt.Printf("Database saved to: %s\n", config.DatabasePath)
	fmt.Printf("Chat session data stored in: %s\n", config.DataDir)
	fmt.Println()
	fmt.Println("💡 Tip: You can also use the web interface at http://localhost:8080/chat")
	fmt.Println("   or try the REST API endpoints for integration into your applications.")
}
