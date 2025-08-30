package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lil-rag/pkg/config"
	"lil-rag/pkg/minirag"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dbPath     = flag.String("db", "", "Database path (overrides profile config)")
		dataDir    = flag.String("data-dir", "", "Data directory (overrides profile config)")
		ollamaURL  = flag.String("ollama", "", "Ollama URL (overrides profile config)")
		model      = flag.String("model", "", "Embedding model (overrides profile config)")
		vectorSize = flag.Int("vector-size", 0, "Vector size (overrides profile config)")
		help       = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

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
	if *vectorSize > 0 {
		profileConfig.Ollama.VectorSize = *vectorSize
	}

	miniragConfig := &minirag.Config{
		DatabasePath: profileConfig.StoragePath,
		DataDir:      profileConfig.DataDir,
		OllamaURL:    profileConfig.Ollama.Endpoint,
		Model:        profileConfig.Ollama.EmbeddingModel,
		VectorSize:   profileConfig.Ollama.VectorSize,
		MaxTokens:    profileConfig.Chunking.MaxTokens,
		Overlap:      profileConfig.Chunking.Overlap,
	}

	rag, err := minirag.New(miniragConfig)
	if err != nil {
		return fmt.Errorf("failed to create MiniRag: %w", err)
	}

	if err := rag.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize MiniRag: %w", err)
	}
	defer rag.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch command {
	case "index":
		return handleIndex(ctx, rag, args[1:])
	case "search":
		return handleSearch(ctx, rag, args[1:])
	case "config":
		return handleConfig(profileConfig, args[1:])
	case "reset":
		return handleReset(profileConfig, args[1:])
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func handleIndex(ctx context.Context, rag *minirag.MiniRag, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: lil-rag index <id> [text|file|-]")
	}

	id := args[0]

	if len(args) == 1 {
		// Read from stdin
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

	input := args[1]

	if input == "-" {
		// Read from stdin
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
		// Handle file - check if it's PDF or regular text
		if isPDFFile(input) {
			fmt.Printf("Indexing PDF file '%s' with ID '%s'...\n", input, id)
			if err := rag.IndexPDF(ctx, input, id); err != nil {
				return fmt.Errorf("failed to index PDF: %w", err)
			}
			fmt.Printf("Successfully indexed PDF file '%s' with ID '%s'\n", input, id)
			return nil
		}
		// Regular text file
		text, err := readFromFile(input)
		if err != nil {
			return fmt.Errorf("failed to read from file: %w", err)
		}

		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("no text to index")
		}

		fmt.Printf("Indexing text file '%s' with ID '%s'...\n", input, id)
		if err := rag.Index(ctx, text, id); err != nil {
			return fmt.Errorf("failed to index: %w", err)
		}

		fmt.Printf("Successfully indexed %d characters with ID '%s'\n", len(text), id)
		return nil
	}
	// Treat as direct text input
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

func handleSearch(ctx context.Context, rag *minirag.MiniRag, args []string) error {
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
		fmt.Printf("Vector Size: %d\n", profileConfig.Ollama.VectorSize)
		fmt.Printf("Chunk Max Tokens: %d\n", profileConfig.Chunking.MaxTokens)
		fmt.Printf("Chunk Overlap: %d\n", profileConfig.Chunking.Overlap)
		fmt.Printf("Server Host: %s\n", profileConfig.Server.Host)
		fmt.Printf("Server Port: %d\n", profileConfig.Server.Port)
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
	if len(args) > 0 && args[0] == "--help" {
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
		if arg == "--force" {
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
		if response != "y" && response != "yes" {
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

func readFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isPDFFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".pdf"
}

func printUsage() {
	fmt.Println("LilRag - A simple RAG system with SQLite and Ollama")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  lil-rag [flags] <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  index <id> [text|file|-]  Index text, file, or stdin with given ID")
	fmt.Println("  search <query> [limit]    Search for similar text (default limit: 10)")
	fmt.Println("  config <init|show|set>    Manage user profile configuration")
	fmt.Println("  reset [--force]           Delete database and all indexed data")
	fmt.Println("")
	fmt.Println("Flags:")
	fmt.Println("  -db string           Database path (overrides profile config)")
	fmt.Println("  -data-dir string     Data directory (overrides profile config)")
	fmt.Println("  -ollama string       Ollama URL (overrides profile config)")
	fmt.Println("  -model string        Embedding model (overrides profile config)")
	fmt.Println("  -vector-size int     Vector size (overrides profile config)")
	fmt.Println("  -help               Show this help")
	fmt.Println("")
	fmt.Println("Configuration:")
	fmt.Println("  config init                     Initialize profile configuration")
	fmt.Println("  config show                     Show current configuration")
	fmt.Println("  config set <key> <value>        Update configuration")
	fmt.Println("")
	fmt.Println("Config Keys:")
	fmt.Println("  ollama.endpoint                 Ollama server URL")
	fmt.Println("  ollama.model                    Embedding model name")
	fmt.Println("  ollama.vector-size              Vector dimension size")
	fmt.Println("  storage.path                    Database file path")
	fmt.Println("  data.dir                        Data directory path")
	fmt.Println("  server.host                     HTTP server host")
	fmt.Println("  server.port                     HTTP server port")
	fmt.Println("  chunking.max-tokens             Maximum tokens per chunk")
	fmt.Println("  chunking.overlap                Token overlap between chunks")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  lil-rag config init")
	fmt.Println("  lil-rag config set ollama.endpoint http://localhost:11434")
	fmt.Println("  lil-rag config set ollama.model nomic-embed-text")
	fmt.Println("  lil-rag index doc1 \"Hello world\"")
	fmt.Println("  lil-rag index doc2 document.txt")
	fmt.Println("  echo \"Hello world\" | lil-rag index doc3 -")
	fmt.Println("  lil-rag search \"hello\" 5")
	fmt.Println("  lil-rag reset                   # Reset database (with confirmation)")
	fmt.Println("  lil-rag reset --force           # Reset database (skip confirmation)")
}
