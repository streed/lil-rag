package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"lil-rag/pkg/auth"
	"lil-rag/pkg/chathistory"
	"lil-rag/pkg/config"
	"lil-rag/pkg/lilrag"
)

const (
	helpFlag  = "--help"
	forceFlag = "--force"
	yesAnswer = "yes"
)

// version is set during build time via ldflags
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dbPath      = flag.String("db", "", "Database path (overrides profile config)")
		dataDir     = flag.String("data-dir", "", "Data directory (overrides profile config)")
		ollamaURL   = flag.String("ollama", "", "Ollama URL (overrides profile config)")
		model       = flag.String("model", "", "Embedding model (overrides profile config)")
		chatModel   = flag.String("chat-model", "", "Chat model (overrides profile config)")
		vectorSize  = flag.Int("vector-size", 0, "Vector size (overrides profile config)")
		help        = flag.Bool("help", false, "Show help")
		showVersion = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("lil-rag version %s\n", version)
		return nil
	}

	if *help {
		printUsage()
		return nil
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return fmt.Errorf("no command specified")
	}

	command := args[0]

	profileConfig, err := config.LoadProfile()
	if err != nil {
		return fmt.Errorf("failed to load profile config: %w", err)
	}

	// Override with command line flags
	if *dbPath != "" {
		profileConfig.StoragePath = *dbPath
	}
	if *dataDir != "" {
		profileConfig.DataDir = *dataDir
	}
	if *ollamaURL != "" {
		profileConfig.Ollama.Endpoint = *ollamaURL
	}
	if *model != "" {
		profileConfig.Ollama.EmbeddingModel = *model
	}
	if *chatModel != "" {
		profileConfig.Ollama.ChatModel = *chatModel
	}
	if *vectorSize > 0 {
		profileConfig.Ollama.VectorSize = *vectorSize
	}

	lilragConfig := &lilrag.Config{
		DatabasePath: profileConfig.StoragePath,
		DataDir:      profileConfig.DataDir,
		OllamaURL:    profileConfig.Ollama.Endpoint,
		Model:        profileConfig.Ollama.EmbeddingModel,
		ChatModel:    profileConfig.Ollama.ChatModel,
		VectorSize:   profileConfig.Ollama.VectorSize,
		MaxTokens:    profileConfig.Chunking.MaxTokens,
		Overlap:      profileConfig.Chunking.Overlap,
	}

	rag, err := lilrag.New(lilragConfig)
	if err != nil {
		return fmt.Errorf("failed to create MiniRag: %w", err)
	}

	if err := rag.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize MiniRag: %w", err)
	}
	defer func() {
		_ = rag.Close() // Ignore close errors in defer
	}()

	// Use longer timeout for reindex operation
	timeout := 5 * time.Minute
	if command == "reindex" {
		timeout = 30 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch command {
	case "index":
		return handleIndex(ctx, rag, args[1:])
	case "search":
		return handleSearch(ctx, rag, args[1:])
	case "chat":
		return handleChat(ctx, rag, profileConfig, args[1:])
	case "documents", "docs":
		return handleDocuments(ctx, rag, args[1:])
	case "delete", "rm":
		return handleDelete(ctx, rag, args[1:])
	case "reindex":
		return handleReindex(ctx, rag, args[1:])
	case "health":
		return handleHealth(rag)
	case "config":
		return handleConfig(profileConfig, args[1:])
	case "auth":
		return handleAuth(profileConfig, args[1:])
	case "reset":
		return handleReset(profileConfig, args[1:])
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func handleIndex(ctx context.Context, rag *lilrag.LilRag, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lil-rag index [id] <text|file|-> or just: lil-rag index <text|file|->")
	}

	var id string
	var input string

	if len(args) == 1 {
		// Only one argument - could be ID with stdin, or direct text/file without ID
		arg := args[0]

		if arg == "-" {
			// Reading from stdin without explicit ID
			text, err := readFromStdin()
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}

			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("no text to index")
			}

			// Generate ID automatically
			id = lilrag.GenerateDocumentID()
			fmt.Printf("Indexing text with auto-generated ID '%s'...\n", id)
			if err := rag.Index(ctx, text, id); err != nil {
				return fmt.Errorf("failed to index: %w", err)
			}

			fmt.Printf("Successfully indexed %d characters with ID '%s'\n", len(text), id)
			return nil
		}

		if fileExists(arg) {
			// File exists, index with auto-generated ID
			id = lilrag.GenerateDocumentID()
			fmt.Printf("Indexing file '%s' with auto-generated ID '%s'...\n", arg, id)
			if err := rag.IndexFile(ctx, arg, id); err != nil {
				return fmt.Errorf("failed to index file: %w", err)
			}
			fmt.Printf("Successfully indexed file '%s' with ID '%s'\n", arg, id)
			return nil
		}

		// Treat as direct text input with auto-generated ID
		text := arg
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("no text to index")
		}

		id = lilrag.GenerateDocumentID()
		fmt.Printf("Indexing text with auto-generated ID '%s'...\n", id)
		if err := rag.Index(ctx, text, id); err != nil {
			return fmt.Errorf("failed to index: %w", err)
		}

		fmt.Printf("Successfully indexed %d characters with ID '%s'\n", len(text), id)
		return nil
	}

	// Two arguments: first is ID, second is input
	id = args[0]
	input = args[1]

	if input == "-" {
		// Read from stdin with explicit ID
		text, err := readFromStdin()
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}

		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("no text to index")
		}

		fmt.Printf("Indexing text with ID '%s'...\n", id)
		if err := rag.Index(ctx, text, id); err != nil {
			return fmt.Errorf("failed to index: %w", err)
		}

		fmt.Printf("Successfully indexed %d characters with ID '%s'\n", len(text), id)
		return nil
	}

	if fileExists(input) {
		// Handle file using the document handler (supports PDF, DOCX, XLSX, HTML, CSV, etc.)
		fmt.Printf("Indexing file '%s' with ID '%s'...\n", input, id)
		if err := rag.IndexFile(ctx, input, id); err != nil {
			return fmt.Errorf("failed to index file: %w", err)
		}
		fmt.Printf("Successfully indexed file '%s' with ID '%s'\n", input, id)
		return nil
	}

	// Treat as direct text input with explicit ID
	text := input
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("no text to index")
	}

	fmt.Printf("Indexing text with ID '%s'...\n", id)
	if err := rag.Index(ctx, text, id); err != nil {
		return fmt.Errorf("failed to index: %w", err)
	}

	fmt.Printf("Successfully indexed %d characters with ID '%s'\n", len(text), id)
	return nil
}

func handleSearch(ctx context.Context, rag *lilrag.LilRag, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lil-rag search <query> [limit]")
	}

	query := args[0]
	limit := 10

	if len(args) > 1 {
		if _, err := fmt.Sscanf(args[1], "%d", &limit); err != nil {
			return fmt.Errorf("invalid limit: %s", args[1])
		}
	}

	fmt.Printf("Searching for: %s\n", query)
	results, err := rag.Search(ctx, query, limit)
	if err != nil {
		return fmt.Errorf("failed to search: %w", err)
	}

	// Try to render with glow first
	if isGlowAvailable() {
		markdown := formatSearchResultsAsMarkdown(query, results)
		if err := renderWithGlow(markdown); err == nil {
			return nil
		}
		// If glow fails, fall back to plain text
		fmt.Fprintf(os.Stderr, "Warning: Failed to render with glow, falling back to plain text\n")
	}

	// Fallback to original plain text output
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for i, result := range results {
		matchInfo := ""

		if result.Metadata != nil {
			// Show information about which part matched
			if pageNum, ok := result.Metadata["page_number"].(int); ok {
				matchInfo = fmt.Sprintf(" [Best match: Page %d]", pageNum)
			} else if chunkType, ok := result.Metadata["chunk_type"].(string); ok && chunkType == "pdf_page" {
				matchInfo = " [Best match: PDF Page]"
			} else if isChunk, ok := result.Metadata["is_chunk"].(bool); ok && isChunk {
				if chunkIndex, ok := result.Metadata["chunk_index"].(int); ok {
					matchInfo = fmt.Sprintf(" [Best match: Chunk %d]", chunkIndex)
				}
			}
		}

		fmt.Printf("%d. ID: %s%s (Score: %.4f)\n", i+1, result.ID, matchInfo, result.Score)

		// Always show full document content (result.Text now contains the full document)
		fmt.Printf("   %s\n\n", result.Text)
	}

	return nil
}

func handleConfig(profileConfig *config.ProfileConfig, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lil-rag config <init|show|set>")
	}

	command := args[0]

	switch command {
	case "init":
		defaultConfig := config.DefaultProfile()
		if err := defaultConfig.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		configPath, err := config.GetProfileConfigPath()
		if err != nil {
			fmt.Println("Profile config initialized successfully")
		} else {
			fmt.Printf("Profile config initialized at: %s\n", configPath)
		}
		return nil

	case "show":
		configPath, err := config.GetProfileConfigPath()
		if err != nil {
			fmt.Printf("Config file: <error getting path: %v>\n", err)
		} else {
			fmt.Printf("Config file: %s\n", configPath)
		}
		fmt.Printf("Storage Path: %s\n", profileConfig.StoragePath)
		fmt.Printf("Data Directory: %s\n", profileConfig.DataDir)
		fmt.Printf("Ollama Endpoint: %s\n", profileConfig.Ollama.Endpoint)
		fmt.Printf("Embedding Model: %s\n", profileConfig.Ollama.EmbeddingModel)
		fmt.Printf("Chat Model: %s\n", profileConfig.Ollama.ChatModel)
		fmt.Printf("Vector Size: %d\n", profileConfig.Ollama.VectorSize)
		fmt.Printf("Chunk Max Tokens: %d\n", profileConfig.Chunking.MaxTokens)
		fmt.Printf("Chunk Overlap: %d\n", profileConfig.Chunking.Overlap)
		fmt.Printf("Server Host: %s\n", profileConfig.Server.Host)
		fmt.Printf("Server Port: %d\n", profileConfig.Server.Port)
		fmt.Printf("Server Secure: %t\n", profileConfig.Server.Secure)
		fmt.Printf("Server Read Timeout: %d seconds\n", profileConfig.Server.ReadTimeout)
		fmt.Printf("Server Write Timeout: %d seconds\n", profileConfig.Server.WriteTimeout)
		fmt.Printf("Server Idle Timeout: %d seconds\n", profileConfig.Server.IdleTimeout)
		fmt.Printf("Server Max Header Bytes: %d\n", profileConfig.Server.MaxHeaderBytes)
		fmt.Printf("Server Enable CORS: %t\n", profileConfig.Server.EnableCORS)
		return nil

	case "set":
		return handleConfigSet(profileConfig, args[1:])

	default:
		return fmt.Errorf("unknown config command: %s", command)
	}
}

func handleConfigSet(profileConfig *config.ProfileConfig, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: lil-rag config set <key> <value>")
	}

	key, value := args[0], args[1]

	switch key {
	case "ollama.endpoint":
		profileConfig.Ollama.Endpoint = value
	case "ollama.model":
		profileConfig.Ollama.EmbeddingModel = value
	case "ollama.chat-model":
		profileConfig.Ollama.ChatModel = value
	case "ollama.vector-size":
		var size int
		if _, err := fmt.Sscanf(value, "%d", &size); err != nil {
			return fmt.Errorf("invalid vector size: %s", value)
		}
		profileConfig.Ollama.VectorSize = size
	case "storage.path":
		profileConfig.StoragePath = value
	case "data.dir":
		profileConfig.DataDir = value
	case "server.host":
		profileConfig.Server.Host = value
	case "server.port":
		var port int
		if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
			return fmt.Errorf("invalid port: %s", value)
		}
		profileConfig.Server.Port = port
	case "server.secure":
		var secure bool
		if _, err := fmt.Sscanf(value, "%t", &secure); err != nil {
			return fmt.Errorf("invalid secure value (use true/false): %s", value)
		}
		profileConfig.Server.Secure = secure
	case "server.read-timeout":
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return fmt.Errorf("invalid read timeout: %s", value)
		}
		profileConfig.Server.ReadTimeout = timeout
	case "server.write-timeout":
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return fmt.Errorf("invalid write timeout: %s", value)
		}
		profileConfig.Server.WriteTimeout = timeout
	case "server.idle-timeout":
		var timeout int
		if _, err := fmt.Sscanf(value, "%d", &timeout); err != nil {
			return fmt.Errorf("invalid idle timeout: %s", value)
		}
		profileConfig.Server.IdleTimeout = timeout
	case "server.max-header-bytes":
		var maxBytes int
		if _, err := fmt.Sscanf(value, "%d", &maxBytes); err != nil {
			return fmt.Errorf("invalid max header bytes: %s", value)
		}
		profileConfig.Server.MaxHeaderBytes = maxBytes
	case "server.enable-cors":
		var enableCors bool
		if _, err := fmt.Sscanf(value, "%t", &enableCors); err != nil {
			return fmt.Errorf("invalid enable-cors value (use true/false): %s", value)
		}
		profileConfig.Server.EnableCORS = enableCors
	case "chunking.max-tokens":
		var maxTokens int
		if _, err := fmt.Sscanf(value, "%d", &maxTokens); err != nil {
			return fmt.Errorf("invalid max tokens: %s", value)
		}
		profileConfig.Chunking.MaxTokens = maxTokens
	case "chunking.overlap":
		var overlap int
		if _, err := fmt.Sscanf(value, "%d", &overlap); err != nil {
			return fmt.Errorf("invalid overlap: %s", value)
		}
		profileConfig.Chunking.Overlap = overlap
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := profileConfig.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Config updated: %s = %s\n", key, value)
	return nil
}

func handleReset(profileConfig *config.ProfileConfig, args []string) error {
	if len(args) > 0 && args[0] == helpFlag {
		fmt.Println("Usage: lil-rag reset [--force]")
		fmt.Println("")
		fmt.Println("Delete the current database and all indexed data.")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --force    Skip confirmation prompt")
		fmt.Println("")
		fmt.Println("This operation is irreversible and will permanently delete:")
		fmt.Println("  • The vector database file")
		fmt.Println("  • All indexed documents and embeddings")
		fmt.Println("  • Search history and cache")
		return nil
	}

	// Check if --force flag is provided
	force := false
	for _, arg := range args {
		if arg == forceFlag {
			force = true
			break
		}
	}

	dbPath := profileConfig.StoragePath
	dataDir := profileConfig.DataDir

	// Show what will be deleted
	fmt.Printf("This will permanently delete:\n")
	fmt.Printf("  • Database: %s\n", dbPath)
	if dataDir != "" {
		fmt.Printf("  • Data directory: %s\n", dataDir)
	}
	fmt.Printf("\n")

	// Check if files exist
	dbExists := fileExists(dbPath)
	dataDirExists := dataDir != "" && fileExists(dataDir)

	if !dbExists && !dataDirExists {
		fmt.Println("No database or data directory found. Nothing to delete.")
		return nil
	}

	if !force {
		// Prompt for confirmation
		fmt.Printf("Are you sure you want to delete all data? This cannot be undone. (y/N): ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("failed to read input")
		}

		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response != "y" && response != yesAnswer {
			fmt.Println("Operation canceled.")
			return nil
		}
	}

	// Perform the deletion
	var errors []string

	// Delete database file
	if dbExists {
		if err := os.Remove(dbPath); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete database %s: %v", dbPath, err))
		} else {
			fmt.Printf("✓ Deleted database: %s\n", dbPath)
		}
	}

	// Delete data directory
	if dataDirExists {
		if err := os.RemoveAll(dataDir); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete data directory %s: %v", dataDir, err))
		} else {
			fmt.Printf("✓ Deleted data directory: %s\n", dataDir)
		}
	}

	// Check for related files (like WAL, journal files)
	relatedFiles := []string{
		dbPath + "-wal",
		dbPath + "-shm",
		dbPath + ".backup",
	}

	for _, file := range relatedFiles {
		if fileExists(file) {
			if err := os.Remove(file); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete related file %s: %v", file, err))
			} else {
				fmt.Printf("✓ Deleted related file: %s\n", file)
			}
		}
	}

	if len(errors) > 0 {
		fmt.Println("\nErrors occurred during deletion:")
		for _, err := range errors {
			fmt.Printf("  • %s\n", err)
		}
		return fmt.Errorf("deletion completed with %d errors", len(errors))
	}

	fmt.Println("\n✓ Database reset completed successfully.")
	fmt.Println("You can now start fresh with 'lil-rag index' command.")

	return nil
}

func readFromStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", fmt.Errorf("no data available on stdin")
	}

	var text strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text.WriteString(scanner.Text())
		text.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return "", err
	}

	return text.String(), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func handleChat(ctx context.Context, rag *lilrag.LilRag, profileConfig *config.ProfileConfig, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lil-rag chat [flags] <message> [limit]\n" +
			"  Flags:\n" +
			"    --session-id <id>    Resume existing chat session\n" +
			"    --new-session       Start new chat session\n" +
			"    --list-sessions     List all chat sessions")
	}

	var sessionID string
	var forceNewSession bool
	var listSessions bool
	var message string
	limit := 5

	// Parse flags and arguments
	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "--session-id":
			if i+1 >= len(args) {
				return fmt.Errorf("--session-id requires a session ID")
			}
			sessionID = args[i+1]
			i += 2
		case "--new-session":
			forceNewSession = true
			i++
		case "--list-sessions":
			listSessions = true
			i++
		default:
			// This is the message
			message = arg
			i++
			// Check if there's a limit parameter after the message
			if i < len(args) {
				if _, err := fmt.Sscanf(args[i], "%d", &limit); err == nil {
					i++
				}
			}
			break
		}
	}

	// Initialize chat history
	chatHistoryPath := profileConfig.DataDir + "/chat_history.db"
	chatHistory, err := chathistory.New(chatHistoryPath)
	if err != nil {
		return fmt.Errorf("failed to initialize chat history: %w", err)
	}
	defer func() {
		_ = chatHistory.Close() // Ignore close errors in defer
	}()

	// Handle list sessions command
	if listSessions {
		return handleListChatSessions(ctx, chatHistory)
	}

	if message == "" {
		return fmt.Errorf("message is required")
	}

	// Handle session management
	var session *chathistory.ChatSession
	if sessionID != "" && !forceNewSession {
		// Try to resume existing session
		session, err = chatHistory.GetSession(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to get chat session '%s': %w", sessionID, err)
		}
		fmt.Printf("📝 Resuming chat session: %s (Title: %s)\n", session.ID, session.Title)
	} else {
		// Create new session
		session, err = chatHistory.CreateSession(ctx)
		if err != nil {
			return fmt.Errorf("failed to create chat session: %w", err)
		}
		fmt.Printf("🆕 Created new chat session: %s\n", session.ID)
		fmt.Printf("💡 Use --session-id %s to resume this conversation later\n", session.ID)
	}

	// Add user message to history
	userMsg, err := chatHistory.AddMessage(ctx, session.ID, "user", message)
	if err != nil {
		return fmt.Errorf("failed to add user message to history: %w", err)
	}

	fmt.Printf("💬 User: %s\n", message)

	// Get chat response with conversation history
	response, sources, err := rag.Chat(ctx, message, limit)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		fmt.Printf("💾 Session ID: %s (Messages: %d - user message saved)\n", session.ID, userMsg.MessageOrder)
		return fmt.Errorf("failed to chat: %w", err)
	}

	// Add assistant response to history
	assistantMsg, err := chatHistory.AddMessage(ctx, session.ID, "assistant", response)
	if err != nil {
		return fmt.Errorf("failed to add assistant message to history: %w", err)
	}

	// Try to render with glow first
	if isGlowAvailable() {
		markdown := formatChatResponseAsMarkdownWithSession(session.ID, message, response, sources)
		if err := renderWithGlow(markdown); err == nil {
			return nil
		}
		// If glow fails, fall back to plain text
		fmt.Fprintf(os.Stderr, "Warning: Failed to render with glow, falling back to plain text\n")
	}

	// Fallback to original plain text output
	fmt.Printf("\n🤖 Assistant: %s\n\n", response)

	if len(sources) > 0 {
		fmt.Printf("📚 Sources (%d):\n", len(sources))
		for i, source := range sources {
			fmt.Printf("%d. %s (Score: %.4f)\n", i+1, source.ID, source.Score)
			fmt.Printf("   %s\n\n", truncateText(source.Text, 200))
		}
	}

	// Show session info
	fmt.Printf("💾 Session ID: %s (Messages: %d)\n", session.ID, assistantMsg.MessageOrder)

	return nil
}

func handleDocuments(ctx context.Context, rag *lilrag.LilRag, args []string) error {
	if len(args) > 0 && args[0] == helpFlag {
		fmt.Println("Usage: lil-rag documents")
		fmt.Println("")
		fmt.Println("List all indexed documents with their metadata.")
		return nil
	}

	fmt.Println("Listing all documents...")
	documents, err := rag.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	if len(documents) == 0 {
		fmt.Println("No documents found.")
		return nil
	}

	fmt.Printf("Found %d documents:\n\n", len(documents))
	for i, doc := range documents {
		fmt.Printf("%d. ID: %s\n", i+1, doc.ID)
		fmt.Printf("   Type: %s\n", doc.DocType)
		fmt.Printf("   Chunks: %d\n", doc.ChunkCount)
		if doc.SourcePath != "" {
			fmt.Printf("   Source: %s\n", doc.SourcePath)
		}
		fmt.Printf("   Created: %s\n", doc.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Updated: %s\n\n", doc.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func handleDelete(ctx context.Context, rag *lilrag.LilRag, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lil-rag delete <document-id> [--force]")
	}

	if args[0] == "--help" {
		fmt.Println("Usage: lil-rag delete <document-id> [--force]")
		fmt.Println("")
		fmt.Println("Delete a document and all its chunks from the database.")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --force    Skip confirmation prompt")
		return nil
	}

	documentID := args[0]

	// Check if --force flag is provided
	force := false
	for _, arg := range args[1:] {
		if arg == forceFlag {
			force = true
			break
		}
	}

	if !force {
		// Prompt for confirmation
		fmt.Printf("Are you sure you want to delete document '%s'? This cannot be undone. (y/N): ", documentID)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("failed to read input")
		}

		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response != "y" && response != yesAnswer {
			fmt.Println("Operation canceled.")
			return nil
		}
	}

	fmt.Printf("Deleting document '%s'...\n", documentID)
	err := rag.DeleteDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	fmt.Printf("✓ Document '%s' deleted successfully.\n", documentID)
	return nil
}

func handleReindex(ctx context.Context, rag *lilrag.LilRag, args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("Usage: lil-rag reindex [--force] [--chunking=STRATEGY]")
		fmt.Println("")
		fmt.Println("Reprocess all documents with the specified chunking configuration.")
		fmt.Println("This will:")
		fmt.Println("  • Re-chunk all documents using the specified algorithm")
		fmt.Println("  • Regenerate embeddings for all chunks")
		fmt.Println("  • Update chunk boundaries and overlap")
		fmt.Println("  • Preserve original document content and metadata")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --force              Skip confirmation prompt")
		fmt.Println("  --chunking=STRATEGY  Chunking strategy to use (default: recursive)")
		fmt.Println("")
		fmt.Println("Available chunking strategies:")
		fmt.Println("  recursive   Hierarchical text splitting with semantic boundaries (default)")
		fmt.Println("  semantic    Adaptive chunking focused on semantic coherence")
		fmt.Println("  simple      Basic word-based chunking")
		fmt.Println("")
		fmt.Println("Note: This operation can take several minutes depending on the number")
		fmt.Println("of documents and their size. The system will remain accessible during")
		fmt.Println("reindexing, but performance may be impacted.")
		return nil
	}

	// Parse flags
	force := false
	chunkingStrategy := "recursive" // default strategy
	for _, arg := range args {
		if arg == forceFlag {
			force = true
		} else if strings.HasPrefix(arg, "--chunking=") {
			chunkingStrategy = strings.TrimPrefix(arg, "--chunking=")
		}
	}

	// Validate chunking strategy
	validStrategies := []string{"recursive", "semantic", "simple"}
	isValid := false
	for _, valid := range validStrategies {
		if chunkingStrategy == valid {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid chunking strategy '%s'. Valid strategies: %s", 
			chunkingStrategy, strings.Join(validStrategies, ", "))
	}

	// Get document count for confirmation
	documents, err := rag.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to check documents: %w", err)
	}

	if len(documents) == 0 {
		fmt.Println("No documents found to reindex.")
		return nil
	}

	fmt.Printf("This will reindex %d documents with %s chunking.\n", len(documents), chunkingStrategy)
	fmt.Println("The process will:")
	fmt.Println("  • Re-chunk all documents using the specified algorithm")
	fmt.Println("  • Regenerate embeddings for improved search performance")
	fmt.Println("  • Update chunk boundaries for better semantic coherence")
	fmt.Printf("\nEstimated time: %d-%d minutes (depending on document size and Ollama performance)\n",
		len(documents)/10, len(documents)/5)

	if !force {
		// Prompt for confirmation
		fmt.Printf("\nProceed with reindexing? (y/N): ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("failed to read input")
		}

		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response != "y" && response != yesAnswer {
			fmt.Println("Operation canceled.")
			return nil
		}
	}

	fmt.Printf("\n🔄 Starting reindex with %s chunking...\n", chunkingStrategy)
	fmt.Println("This may take several minutes. Please do not interrupt the process.")

	startTime := time.Now()
	err = rag.ReindexAllDocumentsWithStrategy(ctx, chunkingStrategy)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("\n❌ Reindex failed after %v: %v\n", duration, err)
		return fmt.Errorf("reindex failed: %w", err)
	}

	fmt.Printf("\n✅ Reindex completed successfully in %v\n", duration)
	fmt.Printf("All %d documents have been reprocessed with %s chunking.\n", len(documents), chunkingStrategy)
	fmt.Println("Your RAG system now uses improved chunk boundaries for better search performance.")

	return nil
}

func handleHealth(rag *lilrag.LilRag) error {
	// Simple health check - verify we can initialize the database
	fmt.Println("Checking system health...")

	// Check if we can connect to the database
	if rag == nil {
		fmt.Println("❌ RAG system is not initialized")
		return fmt.Errorf("RAG system not available")
	}

	fmt.Println("✓ RAG system is running")
	fmt.Println("✓ Database is accessible")
	fmt.Println("✓ System is healthy")

	return nil
}

func truncateText(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// isGlowAvailable checks if the glow command is available on the system
func isGlowAvailable() bool {
	_, err := exec.LookPath("glow")
	return err == nil
}

// renderWithGlow renders markdown content using glow if available
func renderWithGlow(markdown string) error {
	if !isGlowAvailable() {
		return fmt.Errorf("glow not available")
	}

	cmd := exec.Command("glow", "-")
	cmd.Stdin = strings.NewReader(markdown)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// formatSearchResultsAsMarkdown formats search results as markdown
func formatSearchResultsAsMarkdown(query string, results []lilrag.SearchResult) string {
	var md strings.Builder

	md.WriteString("# Search Results\n\n")
	md.WriteString(fmt.Sprintf("**Query:** %s  \n", query))
	md.WriteString(fmt.Sprintf("**Results found:** %d\n\n", len(results)))

	if len(results) == 0 {
		md.WriteString("*No results found.*\n")
		return md.String()
	}

	md.WriteString("---\n\n")

	for i, result := range results {
		matchInfo := ""

		if result.Metadata != nil {
			// Show information about which part matched
			if pageNum, ok := result.Metadata["page_number"].(int); ok {
				matchInfo = fmt.Sprintf(" *(Best match: Page %d)*", pageNum)
			} else if chunkType, ok := result.Metadata["chunk_type"].(string); ok && chunkType == "pdf_page" {
				matchInfo = " *(Best match: PDF Page)*"
			} else if isChunk, ok := result.Metadata["is_chunk"].(bool); ok && isChunk {
				if chunkIndex, ok := result.Metadata["chunk_index"].(int); ok {
					matchInfo = fmt.Sprintf(" *(Best match: Chunk %d)*", chunkIndex)
				}
			}
		}

		md.WriteString(fmt.Sprintf("## %d. %s%s\n\n", i+1, result.ID, matchInfo))
		md.WriteString(fmt.Sprintf("**Score:** %.4f\n\n", result.Score))
		md.WriteString(fmt.Sprintf("```\n%s\n```\n\n", result.Text))

		if i < len(results)-1 {
			md.WriteString("---\n\n")
		}
	}

	return md.String()
}

// handleListChatSessions lists all available chat sessions
func handleListChatSessions(ctx context.Context, chatHistory *chathistory.ChatHistory) error {
	sessions, err := chatHistory.GetSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list chat sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No chat sessions found.")
		return nil
	}

	fmt.Printf("Found %d chat sessions:\n\n", len(sessions))
	for i, session := range sessions {
		fmt.Printf("%d. ID: %s\n", i+1, session.ID)
		fmt.Printf("   Title: %s\n", session.Title)
		fmt.Printf("   Messages: %d\n", session.MessageCount)
		fmt.Printf("   Created: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Updated: %s\n\n", session.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// formatChatResponseAsMarkdownWithSession formats chat response with session info
func formatChatResponseAsMarkdownWithSession(sessionID, message, response string,
	sources []lilrag.SearchResult) string {
	var md strings.Builder

	md.WriteString("# Chat Session Response\n\n")
	md.WriteString(fmt.Sprintf("**Session ID:** %s\n\n", sessionID))
	md.WriteString(fmt.Sprintf("**Your message:** %s\n\n", message))
	md.WriteString("---\n\n")
	md.WriteString(fmt.Sprintf("## 🤖 Assistant\n\n%s\n\n", response))

	if len(sources) > 0 {
		md.WriteString("---\n\n")
		md.WriteString(fmt.Sprintf("## 📚 Sources (%d)\n\n", len(sources)))

		for i, source := range sources {
			md.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, source.ID))
			md.WriteString(fmt.Sprintf("**Score:** %.4f\n\n", source.Score))
			truncatedText := truncateText(source.Text, 200)
			md.WriteString(fmt.Sprintf("```\n%s\n```\n\n", truncatedText))
		}
	}

	return md.String()
}

func printUsage() {
	fmt.Printf("LilRag - A simple RAG system with SQLite and Ollama (version %s)\n", version)
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  lil-rag [flags] <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  index [id] <text|file|->     Index text, file, or stdin (ID optional, auto-generated if not provided)")
	fmt.Println("  search <query> [limit]       Search for similar text (default limit: 10)")
	fmt.Println("  chat [flags] <message> [limit]  Interactive chat with RAG context (default limit: 5)")
	fmt.Println("      --session-id <id>        Resume existing chat session")
	fmt.Println("      --new-session           Start new chat session")
	fmt.Println("      --list-sessions         List all chat sessions")
	fmt.Println("  documents                    List all indexed documents")
	fmt.Println("  delete <id> [--force]        Delete a document by ID")
	fmt.Println("  reindex [--force] [--chunking=STRATEGY]  Reprocess all documents with specified chunking")
	fmt.Println("  health                       Check system health status")
	fmt.Println("  config <init|show|set>       Manage user profile configuration")
	fmt.Println("  reset [--force]              Delete database and all indexed data")
	fmt.Println("")
	fmt.Println("Flags:")
	fmt.Println("  -db string           Database path (overrides profile config)")
	fmt.Println("  -data-dir string     Data directory (overrides profile config)")
	fmt.Println("  -ollama string       Ollama URL (overrides profile config)")
	fmt.Println("  -model string        Embedding model (overrides profile config)")
	fmt.Println("  -chat-model string   Chat model (overrides profile config)")
	fmt.Println("  -vector-size int     Vector size (overrides profile config)")
	fmt.Println("  -help               Show this help")
	fmt.Println("  -version            Show version")
	fmt.Println("")
	fmt.Println("Configuration:")
	fmt.Println("  config init                     Initialize profile configuration")
	fmt.Println("  config show                     Show current configuration")
	fmt.Println("  config set <key> <value>        Update configuration")
	fmt.Println("")
	fmt.Println("Config Keys:")
	fmt.Println("  ollama.endpoint                 Ollama server URL")
	fmt.Println("  ollama.model                    Embedding model name")
	fmt.Println("  ollama.chat-model               Chat model name")
	fmt.Println("  ollama.vector-size              Vector dimension size")
	fmt.Println("  storage.path                    Database file path")
	fmt.Println("  data.dir                        Data directory path")
	fmt.Println("  server.host                     HTTP server host")
	fmt.Println("  server.port                     HTTP server port")
	fmt.Println("  server.secure                   Enable/disable authentication (true/false)")
	fmt.Println("  server.read-timeout             HTTP read timeout in seconds")
	fmt.Println("  server.write-timeout            HTTP write timeout in seconds")
	fmt.Println("  server.idle-timeout             HTTP idle timeout in seconds")
	fmt.Println("  server.max-header-bytes         Maximum header size in bytes")
	fmt.Println("  server.enable-cors              Enable CORS headers (true/false)")
	fmt.Println("  chunking.max-tokens             Maximum tokens per chunk")
	fmt.Println("  chunking.overlap                Token overlap between chunks")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  lil-rag config init")
	fmt.Println("  lil-rag config set ollama.endpoint http://localhost:11434")
	fmt.Println("  lil-rag config set ollama.model nomic-embed-text")
	fmt.Println("  lil-rag config set ollama.chat-model llama3.2")
	fmt.Println("  lil-rag index \"Hello world\"               # Auto-generated ID")
	fmt.Println("  lil-rag index doc1 \"Hello world\"         # Explicit ID")
	fmt.Println("  lil-rag index document.pdf                # Auto-generated ID")
	fmt.Println("  lil-rag index doc2 document.txt           # Explicit ID")
	fmt.Println("  echo \"Hello world\" | lil-rag index -    # Auto-generated ID from stdin")
	fmt.Println("  echo \"Hello world\" | lil-rag index doc3 -  # Explicit ID from stdin")
	fmt.Println("  lil-rag search \"hello\" 5")
	fmt.Println("  lil-rag chat \"What is machine learning?\" 3")
	fmt.Println("  lil-rag chat --new-session \"Start a new conversation\"")
	fmt.Println("  lil-rag chat --session-id abc123 \"Continue our discussion\"")
	fmt.Println("  lil-rag chat --list-sessions             # List all chat sessions")
	fmt.Println("  lil-rag documents               # List all documents")
	fmt.Println("  lil-rag delete doc1 --force     # Delete document")
	fmt.Println("  lil-rag reindex                 # Reindex all documents with recursive chunking (default)")
	fmt.Println("  lil-rag reindex --force         # Reindex without confirmation")
	fmt.Println("  lil-rag reindex --chunking=simple # Reindex with simple chunking strategy")
	fmt.Println("  lil-rag health                  # Check system health")
	fmt.Println("  lil-rag auth add alice password123  # Add user with username and password")
	fmt.Println("  lil-rag auth list               # List all users")
	fmt.Println("  lil-rag reset                   # Reset database (with confirmation)")
	fmt.Println("  lil-rag reset --force           # Reset database (skip confirmation)")
}

// handleAuth handles authentication commands
func handleAuth(profileConfig *config.ProfileConfig, args []string) error {
	if len(args) == 0 {
		fmt.Println("Auth commands:")
		fmt.Println("  add <username> <password>     # Add a new user")
		fmt.Println("  list                          # List all users")
		fmt.Println("  delete <username>             # Delete a user")
		fmt.Println("  reset-password <username> <new-password>  # Reset user's password")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  lil-rag auth add alice password123")
		fmt.Println("  lil-rag auth list")
		fmt.Println("  lil-rag auth delete alice")
		fmt.Println("  lil-rag auth reset-password alice newpassword456")
		return nil
	}

	// Initialize auth system
	authPath := profileConfig.DataDir + "/auth.db"
	authSystem, err := auth.New(authPath)
	if err != nil {
		return fmt.Errorf("failed to initialize auth system: %w", err)
	}
	defer func() {
		_ = authSystem.Close() // Ignore close errors in defer
	}()

	command := args[0]
	ctx := context.Background()

	switch command {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("auth add requires username and password: lil-rag auth add <username> <password>")
		}
		username := args[1]
		password := args[2]

		// Check if user already exists
		exists, err := authSystem.UserExists(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to check if user exists: %w", err)
		}
		if exists {
			return fmt.Errorf("user '%s' already exists", username)
		}

		// Create user
		user, err := authSystem.CreateUser(ctx, username, password)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		fmt.Printf("User '%s' created successfully (ID: %d)\n", user.Username, user.ID)
		return nil

	case "list":
		users, err := authSystem.ListUsers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list users: %w", err)
		}

		if len(users) == 0 {
			fmt.Println("No users found.")
			return nil
		}

		fmt.Printf("%-5s %-20s %-20s\n", "ID", "Username", "Created")
		fmt.Println(strings.Repeat("-", 50))
		for _, user := range users {
			fmt.Printf("%-5d %-20s %-20s\n", user.ID, user.Username, user.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		return nil

	case "delete":
		if len(args) != 2 {
			return fmt.Errorf("auth delete requires username: lil-rag auth delete <username>")
		}
		username := args[1]

		// Check if user exists
		exists, err := authSystem.UserExists(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to check if user exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("user '%s' does not exist", username)
		}

		// Delete user
		err = authSystem.DeleteUser(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		fmt.Printf("User '%s' deleted successfully\n", username)
		return nil

	case "reset-password":
		if len(args) != 3 {
			return fmt.Errorf("auth reset-password requires username and new password: " +
				"lil-rag auth reset-password <username> <new-password>")
		}
		username := args[1]
		newPassword := args[2]

		// Check if user exists
		exists, err := authSystem.UserExists(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to check if user exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("user '%s' does not exist", username)
		}

		// Update password
		err = authSystem.UpdatePassword(ctx, username, newPassword)
		if err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}

		fmt.Printf("Password for user '%s' reset successfully\n", username)
		return nil

	default:
		return fmt.Errorf("unknown auth command: %s", command)
	}
}
