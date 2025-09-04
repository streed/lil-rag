package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lil-rag/internal/handlers"
	"lil-rag/pkg/config"
	"lil-rag/pkg/lilrag"
)

// version is set during build time via ldflags
var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
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
		host        = flag.String("host", "", "Server host (overrides profile config)")
		port        = flag.Int("port", 0, "Server port (overrides profile config)")
		showVersion = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("lil-rag-server version %s\n", version)
		return nil
	}

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
	if *host != "" {
		profileConfig.Server.Host = *host
	}
	if *port > 0 {
		profileConfig.Server.Port = *port
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
	defer rag.Close()

	handler := handlers.NewWithVersion(rag, version)
	mux := http.NewServeMux()

	mux.Handle("/api/index", handler.Index())
	mux.Handle("/api/search", handler.Search())
	mux.Handle("/api/chat", handler.Chat())
	mux.Handle("/api/documents", handler.Documents())
	mux.Handle("/api/documents/", handler.DocumentRouter())
	mux.Handle("/api/health", handler.Health())
	mux.Handle("/api/metrics", handler.Metrics())
	mux.Handle("/chat", handler.Chat())
	mux.Handle("/documents", handler.DocumentsList())
	mux.Handle("/docs", handler.Documentation())
	mux.Handle("/view/", handler.ViewDocument())
	mux.Handle("/", handler.Static())

	// Wrap the mux with logging middleware
	loggedHandler := handlers.LoggingMiddleware(mux)

	addr := fmt.Sprintf("%s:%d", profileConfig.Server.Host, profileConfig.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      loggedHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start periodic system metrics updates
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// Update initial metrics
		ctx := context.Background()
		handler.UpdateSystemMetrics(ctx)

		for {
			select {
			case <-ticker.C:
				handler.UpdateSystemMetrics(ctx)
			case <-quit:
				return
			}
		}
	}()

	go func() {
		log.Printf("Starting lil-rag-server version %s on %s", version, addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}
