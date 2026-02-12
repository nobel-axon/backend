// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/models"
	"github.com/axon-arena/axon-server/internal/repository"
)

// LeaderboardHandler handles leaderboard API requests.
type LeaderboardHandler struct {
	repos *repository.Repositories
}

// NewLeaderboardHandler creates a new leaderboard handler.
func NewLeaderboardHandler(repos *repository.Repositories) *LeaderboardHandler {
	return &LeaderboardHandler{repos: repos}
}

// GetLeaderboard handles GET /api/leaderboard
func (h *LeaderboardHandler) GetLeaderboard(c *gin.Context) {
	params := models.LeaderboardParams{
		SortBy: c.DefaultQuery("sortBy", "earnings"),
		Limit:  50,
		Offset: 0,
	}

	// Validate sortBy
	validSortBy := map[string]bool{"wins": true, "earnings": true, "accuracy": true, "burned": true, "reputation": true}
	if !validSortBy[params.SortBy] {
		params.SortBy = "earnings"
	}

	if l := c.Query("limit"); l != "" {
		if limit, err := strconv.Atoi(l); err == nil && limit > 0 && limit <= 100 {
			params.Limit = limit
		}
	}
	if o := c.Query("offset"); o != "" {
		if offset, err := strconv.Atoi(o); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	entries, err := h.repos.Agents.GetLeaderboard(c.Request.Context(), params)
	if err != nil {
		log.Printf("GetLeaderboard: failed to fetch (sortBy=%s): %v", params.SortBy, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"leaderboard": entries,
		"sortBy":      params.SortBy,
		"limit":       params.Limit,
		"offset":      params.Offset,
	})
}
