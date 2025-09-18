package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
		dbPath         = flag.String("db", "", "Database path (overrides profile config)")
		dataDir        = flag.String("data-dir", "", "Data directory (overrides profile config)")
		ollamaURL      = flag.String("ollama", "", "Ollama URL (overrides profile config)")
		model          = flag.String("model", "", "Embedding model (overrides profile config)")
		chatModel      = flag.String("chat-model", "", "Chat model (overrides profile config)")
		vectorSize     = flag.Int("vector-size", 0, "Vector size (overrides profile config)")
		host           = flag.String("host", "", "Server host (overrides profile config)")
		port           = flag.Int("port", 0, "Server port (overrides profile config)")
		secureFlag     = flag.Bool("secure", false, "Enable authentication (overrides profile config)")
		noSecureFlag   = flag.Bool("no-secure", false, "Disable authentication (overrides profile config)")
		readTimeout    = flag.Int("read-timeout", 0, "HTTP read timeout in seconds (overrides profile config)")
		writeTimeout   = flag.Int("write-timeout", 0, "HTTP write timeout in seconds (overrides profile config)")
		idleTimeout    = flag.Int("idle-timeout", 0, "HTTP idle timeout in seconds (overrides profile config)")
		maxHeaderBytes = flag.Int("max-header-bytes", 0, "Maximum header size in bytes (overrides profile config)")
		corsFlag       = flag.Bool("enable-cors", false, "Enable CORS headers (overrides profile config)")
		noCorsFlag     = flag.Bool("no-cors", false, "Disable CORS headers (overrides profile config)")
		showVersion    = flag.Bool("version", false, "Show version")
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
	if *secureFlag {
		profileConfig.Server.Secure = true
	}
	if *noSecureFlag {
		profileConfig.Server.Secure = false
	}
	if *readTimeout > 0 {
		profileConfig.Server.ReadTimeout = *readTimeout
	}
	if *writeTimeout > 0 {
		profileConfig.Server.WriteTimeout = *writeTimeout
	}
	if *idleTimeout > 0 {
		profileConfig.Server.IdleTimeout = *idleTimeout
	}
	if *maxHeaderBytes > 0 {
		profileConfig.Server.MaxHeaderBytes = *maxHeaderBytes
	}
	if *corsFlag {
		profileConfig.Server.EnableCORS = true
	}
	if *noCorsFlag {
		profileConfig.Server.EnableCORS = false
	}

	lilragConfig := &lilrag.Config{
		DatabasePath:   profileConfig.StoragePath,
		DataDir:        profileConfig.DataDir,
		OllamaURL:      profileConfig.Ollama.Endpoint,
		Model:          profileConfig.Ollama.EmbeddingModel,
		ChatModel:      profileConfig.Ollama.ChatModel,
		VisionModel:    profileConfig.Ollama.VisionModel,
		TimeoutSeconds: profileConfig.Ollama.TimeoutSeconds,
		VectorSize:     profileConfig.Ollama.VectorSize,
		MaxTokens:      profileConfig.Chunking.MaxTokens,
		Overlap:        profileConfig.Chunking.Overlap,
		ImageMaxSize:   profileConfig.Ollama.ImageMaxSize,
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

	handler := handlers.NewWithVersion(rag, version, profileConfig.DataDir, profileConfig.Server.Secure)
	mux := http.NewServeMux()

	mux.Handle("/api/index", handler.Index())
	mux.Handle("/api/search", handler.Search())
	mux.Handle("/api/chat", handler.Chat())
	mux.Handle("/api/chat/sessions", handler.ChatSessions())
	mux.Handle("/api/chat/sessions/new", handler.CreateChatSession())
	mux.HandleFunc("/api/chat/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/title") {
			handler.UpdateChatSessionTitle()(w, r)
		} else {
			handler.DeleteChatSession()(w, r)
		}
	})
	mux.Handle("/api/chat/history/", handler.ChatHistory())
	mux.Handle("/api/documents", handler.Documents())
	mux.Handle("/api/documents/", handler.DocumentRouter())
	mux.HandleFunc("/api/chunks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			handler.UpdateChunk()(w, r)
		} else {
			handler.GetChunk()(w, r)
		}
	})
	mux.Handle("/api/health", handler.Health())
	mux.Handle("/api/metrics", handler.Metrics())
	mux.Handle("/api/reindex", handler.Reindex())

	// Authentication routes (not protected)
	mux.Handle("/api/auth/login", handler.Login())
	mux.Handle("/api/auth/logout", handler.Logout())
	mux.Handle("/api/auth/status", handler.AuthStatus())
	mux.Handle("/login", handler.LoginPage())

	// Protected UI routes (wrapped with auth middleware)
	mux.Handle("/chat", handler.AuthMiddleware(handler.Chat()))
	mux.Handle("/documents", handler.AuthMiddleware(handler.DocumentsList()))
	mux.Handle("/docs", handler.AuthMiddleware(handler.Documentation()))
	mux.Handle("/view/", handler.AuthMiddleware(handler.ViewDocument()))
	mux.Handle("/file/", handler.AuthMiddleware(handler.ServeFile()))
	mux.Handle("/", handler.AuthMiddleware(handler.Static()))

	// Wrap the mux with logging middleware
	loggedHandler := handlers.LoggingMiddleware(mux)

	addr := fmt.Sprintf("%s:%d", profileConfig.Server.Host, profileConfig.Server.Port)
	server := &http.Server{
		Addr:           addr,
		Handler:        loggedHandler,
		ReadTimeout:    time.Duration(profileConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(profileConfig.Server.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(profileConfig.Server.IdleTimeout) * time.Second,
		MaxHeaderBytes: profileConfig.Server.MaxHeaderBytes,
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
