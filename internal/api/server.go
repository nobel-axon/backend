// Package api provides the HTTP API server for axon-server.
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/api/handlers"
	"github.com/axon-arena/axon-server/internal/api/middleware"
	"github.com/axon-arena/axon-server/internal/chief"
	"github.com/axon-arena/axon-server/internal/config"
	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/lobby"
	"github.com/axon-arena/axon-server/internal/repository"
	"github.com/axon-arena/axon-server/internal/websocket"
)

// Server is the main API server.
type Server struct {
	cfg          *config.Config
	db           *db.DB
	repos        *repository.Repositories
	hub          *websocket.Hub
	chiefClient  *chief.Client
	rateLimiter  *middleware.RateLimiter
	lobbyManager *lobby.Manager
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, database *db.DB, repos *repository.Repositories, hub *websocket.Hub, chiefClient *chief.Client) *Server {
	var rateLimiter *middleware.RateLimiter
	if cfg.RateLimit.Enabled {
		rateLimiter = middleware.NewRateLimiter(&cfg.RateLimit)
	}

	var lobbyManager *lobby.Manager
	if cfg.Lobby.Enabled {
		lobbyManager = lobby.NewManager(&cfg.Lobby, hub)
	}

	return &Server{
		cfg:          cfg,
		db:           database,
		repos:        repos,
		hub:          hub,
		chiefClient:  chiefClient,
		rateLimiter:  rateLimiter,
		lobbyManager: lobbyManager,
	}
}

// Stop cleans up server resources.
func (s *Server) Stop() {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
}

// StartLobby starts the lobby manager background processes.
func (s *Server) StartLobby(ctx context.Context) {
	if s.lobbyManager != nil {
		s.lobbyManager.Start(ctx)
	}
}

// SetupRoutes sets up all HTTP routes.
func (s *Server) SetupRoutes(r *gin.Engine, hub *websocket.Hub) {
	// Global middleware
	r.Use(corsMiddleware(s.cfg.CORS))
	r.Use(middleware.RequestID())

	// Create handlers
	matchHandler := handlers.NewMatchHandler(s.repos, s.chiefClient)
	leaderboardHandler := handlers.NewLeaderboardHandler(s.repos)
	agentHandler := handlers.NewAgentHandler(s.repos)
	statsHandler := handlers.NewStatsHandler(s.repos)
	internalHandler := handlers.NewInternalHandler(s.repos, hub, s.chiefClient)

	// Health check
	r.GET("/health", handlers.Health(s.db))

	// Public API routes
	api := r.Group("/api")
	if s.rateLimiter != nil {
		api.Use(middleware.RateLimit(s.rateLimiter))
	}
	{
		// Matches
		api.GET("/matches", matchHandler.ListMatches)
		api.GET("/matches/open", matchHandler.GetOpenMatches)
		api.GET("/matches/queue", matchHandler.GetOpenMatches) // Alias for /matches/open
		api.GET("/matches/live", matchHandler.GetLiveMatches)
		api.GET("/matches/:id", matchHandler.GetMatch)
		api.GET("/matches/:id/state", matchHandler.GetMatchState)     // Proxy to Chief for real-time state
		api.GET("/matches/:id/answers", matchHandler.GetMatchAnswers)
		api.GET("/matches/:id/commentary", matchHandler.GetMatchCommentary)
		api.GET("/matches/:id/players", matchHandler.GetMatchPlayers)
		api.GET("/matches/:id/personalities", matchHandler.GetMatchPersonalities)

		// Feed
		api.GET("/feed", matchHandler.GetFeed)

		// Categories
		api.GET("/categories", matchHandler.GetCategories)

		// Leaderboard
		api.GET("/leaderboard", leaderboardHandler.GetLeaderboard)

		// Agent
		api.GET("/agent/:address", agentHandler.GetAgentProfile)
		api.GET("/agent/:address/history", agentHandler.GetAgentHistory)

		// Stats
		api.GET("/stats", statsHandler.GetGlobalStats)
		api.GET("/stats/burns", statsHandler.GetBurnHistory)
	}

	// Internal API routes (called by Chief or Ponder)
	internal := r.Group("/internal")
	internal.Use(middleware.InternalAuth(s.cfg.Chief.InternalSecret))
	{
		internal.POST("/match-update", internalHandler.UpdateMatch)
		internal.POST("/commentary", internalHandler.AddCommentary)
		internal.POST("/answer-result", internalHandler.RecordAnswerResult)
		internal.POST("/match-settled", internalHandler.SettleMatch)
		internal.POST("/answer-submitted", internalHandler.RecordAnswerSubmitted)
		internal.POST("/answer-revealed", internalHandler.RecordAnswerRevealed)
		internal.POST("/match-personalities", internalHandler.StorePersonalities)
	}

	// Lobby routes (if enabled)
	if s.lobbyManager != nil {
		lobbyHandler := handlers.NewLobbyHandler(s.lobbyManager)
		lobbyGroup := api.Group("/lobby")
		{
			lobbyGroup.GET("", lobbyHandler.GetLobbyStatus)
			lobbyGroup.POST("/interest", lobbyHandler.SignalInterest)
			lobbyGroup.DELETE("/interest/:address", lobbyHandler.LeaveInterest)
			lobbyGroup.POST("/heartbeat", lobbyHandler.Heartbeat)
			lobbyGroup.GET("/ready/:groupId", lobbyHandler.GetReadyGroup)
		}
	}

	// WebSocket
	r.GET("/ws/live", func(c *gin.Context) {
		// Pass lobby manager to WebSocket for handling lobby messages
		var lobbyHandler websocket.LobbyHandler
		if s.lobbyManager != nil {
			lobbyHandler = s.lobbyManager
		}
		websocket.ServeWSWithLobby(hub, s.cfg.WebSocket, lobbyHandler, c.Writer, c.Request)
	})
}

// corsMiddleware returns a CORS middleware handler.
func corsMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range cfg.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if len(cfg.AllowedOrigins) > 0 && cfg.AllowedOrigins[0] == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		// Handle preflight
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
