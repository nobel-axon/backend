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

// StatsHandler handles statistics API requests.
type StatsHandler struct {
	repos *repository.Repositories
}

// NewStatsHandler creates a new stats handler.
func NewStatsHandler(repos *repository.Repositories) *StatsHandler {
	return &StatsHandler{repos: repos}
}

// GetGlobalStats handles GET /api/stats
func (h *StatsHandler) GetGlobalStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Get total matches
	totalMatches, err := h.repos.Matches.CountTotal(ctx)
	if err != nil {
		log.Printf("GetGlobalStats: failed to count total matches: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats"})
		return
	}

	// Get active matches (registration + question_live)
	activeReg, _ := h.repos.Matches.CountByPhase(ctx, "registration")
	activeLive, _ := h.repos.Matches.CountByPhase(ctx, "question_live")
	activeMatches := activeReg + activeLive

	// Get settled matches
	settledMatches, _ := h.repos.Matches.CountByPhase(ctx, "settled")

	// Get total agents
	totalAgents, _ := h.repos.Agents.CountTotal(ctx)

	// Get total burned
	totalBurned, _ := h.repos.Burns.GetTotalBurned(ctx)

	// Get total pool volume
	totalPoolVolume, _ := h.repos.Burns.GetTotalPoolVolume(ctx)

	// Get last 24h stats
	last24hMatches, _ := h.repos.Matches.CountLast24h(ctx)
	last24hBurned, _ := h.repos.Burns.GetTotalBurnedLast24h(ctx)

	c.JSON(http.StatusOK, models.GlobalStats{
		TotalMatches:    totalMatches,
		ActiveMatches:   activeMatches,
		SettledMatches:  settledMatches,
		TotalAgents:     totalAgents,
		TotalBurned:     totalBurned,
		TotalPoolVolume: totalPoolVolume,
		Last24hMatches:  last24hMatches,
		Last24hBurned:   last24hBurned,
	})
}

// GetBurnHistory handles GET /api/stats/burns
func (h *StatsHandler) GetBurnHistory(c *gin.Context) {
	ctx := c.Request.Context()

	// Determine granularity (hourly or daily)
	granularity := c.DefaultQuery("granularity", "hourly")
	period := 24 // default 24 hours/days

	if p := c.Query("period"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 && parsed <= 365 {
			period = parsed
		}
	}

	var timeline []models.BurnTimeline
	var err error

	if granularity == "daily" {
		timeline, err = h.repos.Burns.GetDailyTimeline(ctx, period)
	} else {
		timeline, err = h.repos.Burns.GetHourlyTimeline(ctx, period)
	}

	if err != nil {
		log.Printf("GetBurnHistory: failed to fetch burn timeline (granularity=%s, period=%d): %v", granularity, period, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch burn history"})
		return
	}

	// Get recent burns
	recent, err := h.repos.Burns.GetRecent(ctx, 50)
	if err != nil {
		recent = []models.BurnStats{}
	}

	recentResponse := make([]models.BurnStatsResponse, len(recent))
	for i, b := range recent {
		recentResponse[i] = b.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"timeline":    timeline,
		"recent":      recentResponse,
		"granularity": granularity,
		"period":      period,
	})
}
