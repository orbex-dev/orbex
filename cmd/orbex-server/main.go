package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/orbex-dev/orbex/internal/api"
	"github.com/orbex-dev/orbex/internal/config"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/docker"
	"github.com/orbex-dev/orbex/internal/worker"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 Orbex — Run anything. Know everything.")
	log.Println("─────────────────────────────────────────")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Environment: %s", cfg.Env)

	ctx := context.Background()

	// Connect to database
	log.Println("Connecting to database...")
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✓ Database connected")

	// Run migrations
	log.Println("Running migrations...")
	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")
	migrationsDir := filepath.Join(projectRoot, "internal", "database", "migrations")
	if err := db.Migrate(ctx, migrationsDir); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("✓ Migrations complete")

	// Connect to Docker
	log.Println("Connecting to Docker...")
	dockerClient, err := docker.New()
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}
	defer dockerClient.Close()
	log.Println("✓ Docker connected")

	// Start background worker
	w := worker.New(db, dockerClient, worker.Config{
		MaxConcurrent: cfg.MaxConcurrentRuns,
		PollInterval:  time.Second,
	})

	workerCtx, workerCancel := context.WithCancel(ctx)
	go w.Run(workerCtx)
	go w.RunReaper(workerCtx)
	log.Printf("✓ Worker started (maxConcurrent=%d)", cfg.MaxConcurrentRuns)

	// Create router
	router := api.NewRouter(db, dockerClient)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("✓ Server listening on %s", cfg.Addr())
		log.Println("─────────────────────────────────────────")
		log.Println("Routes:")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("\nShutting down gracefully...")

	// Stop worker first
	workerCancel()
	w.Shutdown(30 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}

	log.Println("✓ Server stopped")
}
