// Package main is the entry point for the axon-server service.
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

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/axon-arena/axon-server/internal/api"
	"github.com/axon-arena/axon-server/internal/chief"
	"github.com/axon-arena/axon-server/internal/config"
	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/poller"
	"github.com/axon-arena/axon-server/internal/repository"
	"github.com/axon-arena/axon-server/internal/websocket"
)

// Build-time variables (set via -ldflags)
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

var (
	configPath     = flag.String("config", "", "Path to config file")
	migrate        = flag.Bool("migrate", false, "Run database migrations and exit")
	showVersion    = flag.Bool("version", false, "Show version and exit")
)

func main() {
	_ = godotenv.Load() // .env optional

	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if *showVersion {
		fmt.Printf("axon-server version %s (commit: %s, built: %s)\n", version, commit, buildTime)
		return
	}

	log.Printf("Starting axon-server %s (commit: %s)", version, commit)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	log.Printf("Connecting to database: %s@%s:%d/%s",
		cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
	database, err := db.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := database.Ping(ctx); err != nil {
		cancel()
		log.Fatalf("Failed to ping database: %v", err)
	}
	cancel()
	log.Println("Database connection established")

	// Run migrations on startup
	log.Println("Running database migrations...")
	migCtx, migCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	if err := database.Migrate(migCtx); err != nil {
		migCancel()
		log.Fatalf("Migration failed: %v", err)
	}
	migCancel()
	log.Println("Migrations completed successfully")

	if *migrate {
		return
	}

	// Create repositories
	repos := repository.NewRepositories(database)

	// Create WebSocket hub
	wsHub := websocket.NewHub()
	wsHub.OnConnect = func(clientCount int) {
		wsHub.BroadcastWelcome()
	}
	go wsHub.Run()

	// Create Chief client
	chiefClient := chief.NewClient(&cfg.Chief)

	// Create poller
	eventPoller := poller.NewPoller(cfg, database, repos, chiefClient)

	// Create API server (with chief client for proxying)
	srv := api.NewServer(cfg, database, repos, wsHub, chiefClient)

	// Setup gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
		)
	}))

	// Setup routes
	srv.SetupRoutes(r, wsHub)

	// Start background services
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Start poller
	if cfg.Poller.Enabled {
		log.Printf("Starting event poller (interval: %dms)", cfg.Poller.IntervalMs)
		go eventPoller.Start(ctx)
	} else {
		log.Println("Event poller disabled")
	}

	// Start lobby manager
	if cfg.Lobby.Enabled {
		log.Printf("Starting lobby manager (minPlayers: %d, maxPlayers: %d)", cfg.Lobby.MinPlayers, cfg.Lobby.MaxPlayers)
		go srv.StartLobby(ctx)
	} else {
		log.Println("Lobby disabled")
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	// Cancel context to stop background services
	cancel()

	// Stop poller
	eventPoller.Stop()

	// Stop API server (cleanup rate limiter)
	srv.Stop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
