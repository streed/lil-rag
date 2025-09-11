package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"lil-rag/pkg/lilrag"
)

// This example demonstrates the Model Context Protocol (MCP) server functionality:
// - Running lil-rag as an MCP server for AI assistant integration
// - MCP tool definitions and capabilities
// - Integration with AI assistants like Claude, ChatGPT, etc.
// - Programmatic interaction with MCP server
//
// MCP allows AI assistants to use lil-rag as a knowledge retrieval tool,
// enabling them to access and search through your indexed documents.

func main() {
	fmt.Println("=== LilRag MCP (Model Context Protocol) Example ===")
	fmt.Println("Demonstrating AI assistant integration via MCP")
	fmt.Println()

	fmt.Println("🔌 What is MCP?")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("The Model Context Protocol (MCP) is a standard for connecting")
	fmt.Println("AI assistants with external tools and data sources. When lil-rag")
	fmt.Println("runs as an MCP server, AI assistants can:")
	fmt.Println("• Search through your indexed documents")
	fmt.Println("• Index new content")
	fmt.Println("• Manage documents and retrieve context")
	fmt.Println("• Use your knowledge base to answer questions")
	fmt.Println()

	fmt.Println("🚀 Setting up the MCP Server")
	fmt.Println(strings.Repeat("-", 50))

	// First, let's set up a knowledge base for the MCP server to use
	config := &lilrag.Config{
		DatabasePath:   "mcp_example.db",
		DataDir:        "./data_mcp",
		OllamaURL:      "http://localhost:11434",
		Model:          "nomic-embed-text",
		ChatModel:      "llama3.2:3b",
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

	// Create a sample knowledge base
	fmt.Println("📚 Creating Sample Knowledge Base for MCP Demo")
	knowledgeBase := []struct {
		id      string
		content string
	}{
		{
			"company_handbook",
			"Company Handbook: Our company values include innovation, collaboration, and customer focus. We offer flexible work arrangements, comprehensive health benefits, and professional development opportunities. Our remote work policy allows employees to work from anywhere while maintaining productivity and team collaboration.",
		},
		{
			"project_alpha",
			"Project Alpha Status: The new mobile application is currently in beta testing phase. Key features include user authentication, real-time messaging, and data synchronization. Expected launch date is Q2 2024. The development team has resolved 95% of reported bugs and performance has improved by 40%.",
		},
		{
			"tech_stack",
			"Technical Stack: Backend built with Go and PostgreSQL, frontend uses React and TypeScript. Infrastructure deployed on AWS with Docker containers. CI/CD pipeline uses GitHub Actions. Monitoring implemented with Prometheus and Grafana. Security includes OAuth2 and JWT tokens.",
		},
		{
			"meeting_notes",
			"Weekly Team Meeting Notes: Discussed upcoming feature releases, resolved technical blockers, and planned sprint goals. Action items include code review improvements, documentation updates, and performance optimization. Next meeting scheduled for Friday at 2 PM EST.",
		},
	}

	for _, doc := range knowledgeBase {
		if err := rag.Index(ctx, doc.content, doc.id); err != nil {
			fmt.Printf("❌ Failed to index %s: %v\n", doc.id, err)
		} else {
			fmt.Printf("✅ Indexed: %s\n", doc.id)
		}
	}
	fmt.Println()

	fmt.Println("🛠️ MCP Server Tools and Capabilities")
	fmt.Println(strings.Repeat("-", 50))

	// Define the MCP tools that lil-rag provides
	mcpTools := []struct {
		name        string
		description string
		parameters  map[string]interface{}
	}{
		{
			"search_documents",
			"Search through indexed documents using semantic similarity",
			map[string]interface{}{
				"query":     "The search query (required)",
				"limit":     "Maximum number of results to return (optional, default 5)",
				"min_score": "Minimum similarity score threshold (optional)",
			},
		},
		{
			"index_document",
			"Add new content to the knowledge base",
			map[string]interface{}{
				"content":     "The text content to index (required)",
				"document_id": "Unique identifier for the document (required)",
				"metadata":    "Additional metadata as JSON (optional)",
			},
		},
		{
			"get_document",
			"Retrieve a specific document by ID",
			map[string]interface{}{
				"document_id": "The unique identifier of the document (required)",
			},
		},
		{
			"list_documents",
			"List all indexed documents with metadata",
			map[string]interface{}{
				"limit":  "Maximum number of documents to return (optional)",
				"offset": "Number of documents to skip (optional)",
			},
		},
		{
			"delete_document",
			"Remove a document from the knowledge base",
			map[string]interface{}{
				"document_id": "The unique identifier of the document to delete (required)",
			},
		},
	}

	fmt.Println("📋 Available MCP Tools:")
	for i, tool := range mcpTools {
		fmt.Printf("%d. %s\n", i+1, tool.name)
		fmt.Printf("   Description: %s\n", tool.description)
		fmt.Printf("   Parameters:\n")
		for param, desc := range tool.parameters {
			fmt.Printf("     • %s: %s\n", param, desc)
		}
		fmt.Println()
	}

	fmt.Println("🔍 Simulating MCP Tool Usage")
	fmt.Println(strings.Repeat("-", 50))

	// Simulate how an AI assistant would use the MCP tools
	fmt.Println("Scenario: AI assistant helping with company information")
	fmt.Println()

	// Tool 1: Search documents
	fmt.Println("🤖 AI Assistant: I need to find information about the company's remote work policy")
	fmt.Println("📡 MCP Call: search_documents")
	fmt.Println("📝 Parameters: {\"query\": \"remote work policy\", \"limit\": 2}")

	searchResults, err := rag.Search(ctx, "remote work policy", 2)
	if err != nil {
		fmt.Printf("❌ Search failed: %v\n", err)
	} else {
		fmt.Printf("✅ Results found: %d documents\n", len(searchResults))
		for i, result := range searchResults {
			fmt.Printf("   %d. Document: %s (Score: %.4f)\n", i+1, result.ID, result.Score)
			preview := result.Text
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("      Content: %s\n", preview)
		}
	}
	fmt.Println()

	// Tool 2: Get specific document
	fmt.Println("🤖 AI Assistant: Let me get the full company handbook")
	fmt.Println("📡 MCP Call: get_document")
	fmt.Println("📝 Parameters: {\"document_id\": \"company_handbook\"}")

	docs, err := rag.ListDocuments(ctx)
	if err == nil {
		for _, doc := range docs {
			if doc.ID == "company_handbook" {
				fmt.Printf("✅ Document retrieved: %s\n", doc.ID)
				fmt.Printf("   Size: %d characters\n", len(doc.Text))
				fmt.Printf("   Chunks: %d\n", doc.ChunkCount)
				break
			}
		}
	}
	fmt.Println()

	// Tool 3: Index new document
	fmt.Println("🤖 AI Assistant: I need to add information about a new policy")
	fmt.Println("📡 MCP Call: index_document")
	fmt.Println("📝 Parameters: {\"document_id\": \"security_policy\", \"content\": \"New security policy...\"}")

	newPolicyContent := "Security Policy Update: All employees must use two-factor authentication for accessing company systems. VPN is required for remote access. Regular security training is mandatory quarterly. Report any suspicious activities immediately to the security team."

	if err := rag.Index(ctx, newPolicyContent, "security_policy"); err != nil {
		fmt.Printf("❌ Indexing failed: %v\n", err)
	} else {
		fmt.Printf("✅ Document indexed successfully: security_policy\n")
	}
	fmt.Println()

	// Tool 4: List documents
	fmt.Println("🤖 AI Assistant: Show me all available documents")
	fmt.Println("📡 MCP Call: list_documents")
	fmt.Println("📝 Parameters: {\"limit\": 10}")

	allDocs, err := rag.ListDocuments(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to list documents: %v\n", err)
	} else {
		fmt.Printf("✅ Total documents: %d\n", len(allDocs))
		for i, doc := range allDocs {
			fmt.Printf("   %d. %s (%d chunks, %d chars)\n", 
				i+1, doc.ID, doc.ChunkCount, len(doc.Text))
		}
	}
	fmt.Println()

	fmt.Println("⚙️ MCP Server Configuration")
	fmt.Println(strings.Repeat("-", 50))

	// Show how to configure the MCP server
	fmt.Println("💻 Starting the MCP Server:")
	fmt.Println("   Command: lil-rag-mcp")
	fmt.Println("   Default port: stdio (standard input/output)")
	fmt.Println("   Protocol: JSON-RPC over stdio")
	fmt.Println()

	fmt.Println("🔧 Server Configuration Options:")
	fmt.Println("   --db <path>           Database file path")
	fmt.Println("   --data-dir <path>     Data directory for file storage")
	fmt.Println("   --ollama <url>        Ollama server URL")
	fmt.Println("   --model <name>        Embedding model name")
	fmt.Println("   --vector-size <int>   Vector dimensions")
	fmt.Println()

	fmt.Println("📁 Configuration Options:")
	fmt.Printf("   Database: %s\n", config.DatabasePath)
	fmt.Printf("   Data Dir: %s\n", config.DataDir)
	fmt.Printf("   Ollama: %s\n", config.OllamaURL)
	fmt.Printf("   Model: %s\n", config.Model)
	fmt.Println()

	fmt.Println("🔗 AI Assistant Integration")
	fmt.Println(strings.Repeat("-", 50))

	fmt.Println("📝 MCP Client Configuration (for AI assistants):")
	
	mcpConfig := map[string]interface{}{
		"name":        "lil-rag",
		"description": "Document search and knowledge management system",
		"command":     "lil-rag-mcp",
		"args":        []string{},
		"env": map[string]string{
			"LIL_RAG_DB":       config.DatabasePath,
			"LIL_RAG_DATA_DIR": config.DataDir,
		},
	}

	configJSON, _ := json.MarshalIndent(mcpConfig, "", "  ")
	fmt.Printf("```json\n%s\n```\n", string(configJSON))
	fmt.Println()

	fmt.Println("🎯 Use Cases for MCP Integration")
	fmt.Println(strings.Repeat("-", 50))

	useCases := []struct {
		title       string
		description string
		example     string
	}{
		{
			"Personal Knowledge Assistant",
			"AI assistant with access to your personal documents and notes",
			"Search my meeting notes for action items from last week",
		},
		{
			"Customer Support",
			"AI agent that can search company documentation to help customers",
			"Find troubleshooting steps for product installation issues",
		},
		{
			"Research Assistant",
			"AI that can search through research papers and documentation",
			"What does our research say about machine learning applications?",
		},
		{
			"Code Documentation",
			"AI assistant with access to code documentation and APIs",
			"How do I use the authentication API in our system?",
		},
		{
			"Company Wiki",
			"AI that can search company policies, procedures, and knowledge base",
			"What is our vacation policy for international employees?",
		},
	}

	for i, uc := range useCases {
		fmt.Printf("%d. %s\n", i+1, uc.title)
		fmt.Printf("   %s\n", uc.description)
		fmt.Printf("   Example: \"%s\"\n", uc.example)
		fmt.Println()
	}

	fmt.Println("🧪 Interactive MCP Testing")
	fmt.Println(strings.Repeat("-", 50))

	fmt.Println("Would you like to test MCP functionality interactively? (y/n)")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		fmt.Println("\n🎮 Interactive MCP Testing Mode")
		fmt.Println("Enter search queries to test the search_documents tool (type 'quit' to exit):")
		
		for {
			fmt.Print("\n> Search query: ")
			query, _ := reader.ReadString('\n')
			query = strings.TrimSpace(query)
			
			if query == "quit" || query == "exit" {
				break
			}
			
			if query == "" {
				continue
			}
			
			fmt.Printf("📡 MCP Call: search_documents({\"query\": \"%s\", \"limit\": 3})\n", query)
			
			results, err := rag.Search(ctx, query, 3)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			} else if len(results) == 0 {
				fmt.Printf("ℹ️  No results found for: %s\n", query)
			} else {
				fmt.Printf("✅ Found %d results:\n", len(results))
				for i, result := range results {
					fmt.Printf("   %d. %s (Score: %.4f)\n", i+1, result.ID, result.Score)
					preview := result.Text
					if len(preview) > 150 {
						preview = preview[:150] + "..."
					}
					fmt.Printf("      %s\n", preview)
				}
			}
		}
	}

	fmt.Println("\n=== MCP Example Complete ===")
	fmt.Println()
	fmt.Println("This example demonstrated:")
	fmt.Println("- Model Context Protocol (MCP) server functionality")
	fmt.Println("- Available MCP tools and their capabilities")
	fmt.Println("- AI assistant integration scenarios")
	fmt.Println("- Configuration and setup procedures")
	fmt.Println("- Real-world use cases and applications")
	fmt.Println("- Interactive testing capabilities")
	fmt.Println()
	fmt.Println("🚀 To start the actual MCP server:")
	fmt.Println("   lil-rag-mcp")
	fmt.Println()
	fmt.Println("📖 For integration with specific AI assistants:")
	fmt.Println("   - Claude Desktop: Add to claude_desktop_config.json")
	fmt.Println("   - ChatGPT: Configure as custom tool")
	fmt.Println("   - Custom apps: Use JSON-RPC over stdio")
	fmt.Printf("\nDatabase created: %s\n", config.DatabasePath)
	fmt.Printf("Data directory: %s\n", config.DataDir)
}