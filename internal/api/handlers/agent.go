// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"context"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/models"
	"github.com/axon-arena/axon-server/internal/repository"
)

// AgentHandler handles agent profile API requests.
type AgentHandler struct {
	repos *repository.Repositories
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(repos *repository.Repositories) *AgentHandler {
	return &AgentHandler{repos: repos}
}

// isValidAddress checks if an Ethereum address is valid (hex format).
func isValidAddress(addr string) bool {
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		return false
	}
	for _, c := range addr[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// normalizeAddress normalizes and validates an Ethereum address.
func normalizeAddress(addr string) (string, bool) {
	// Add 0x prefix if missing
	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}
	addr = strings.ToLower(addr)

	if !isValidAddress(addr) {
		return "", false
	}
	return addr, true
}

// GetAgentProfile handles GET /api/agent/:address
func (h *AgentHandler) GetAgentProfile(c *gin.Context) {
	address, valid := normalizeAddress(c.Param("address"))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address format"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get agent stats
	stats, err := h.repos.Agents.GetByAddress(ctx, address)
	if err != nil {
		log.Printf("GetAgentProfile: failed to fetch stats for %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agent stats"})
		return
	}

	// If no stats, return empty profile
	if stats == nil {
		c.JSON(http.StatusOK, models.AgentProfile{
			Stats: models.AgentStatsResponse{
				AgentAddr:         address,
				MatchesPlayed:     0,
				MatchesWon:        0,
				WinRate:           0,
				TotalEarnedMON:    "0",
				TotalEarnedNeuron: "0",
				TotalBurnedNeuron: "0",
				WrongAnswers:      0,
				CorrectAnswers:    0,
				AnswerAccuracy:    0,
			},
			RecentMatches: []models.MatchResponse{},
		})
		return
	}

	// Get recent matches
	matches, err := h.repos.Matches.GetMatchesByAgent(ctx, address, 10)
	if err != nil {
		log.Printf("GetAgentProfile: failed to fetch recent matches for %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent matches"})
		return
	}

	matchResponses := make([]models.MatchResponse, len(matches))
	for i, m := range matches {
		matchResponses[i] = m.ToResponse()
	}

	c.JSON(http.StatusOK, models.AgentProfile{
		Stats:         stats.ToResponse(),
		RecentMatches: matchResponses,
	})
}

// GetAgentHistory handles GET /api/agent/:address/history
func (h *AgentHandler) GetAgentHistory(c *gin.Context) {
	address, valid := normalizeAddress(c.Param("address"))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address format"})
		return
	}

	// Parse pagination params
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	matches, total, err := h.repos.Matches.GetMatchesByAgentPaginated(ctx, address, limit, offset)
	if err != nil {
		log.Printf("GetAgentHistory: failed to fetch history for %s: %v", address, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match history"})
		return
	}

	response := make([]models.MatchResponse, len(matches))
	for i, m := range matches {
		response[i] = m.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"matches": response,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetAgentEconomics handles GET /api/agent/:address/economics
func (h *AgentHandler) GetAgentEconomics(c *gin.Context) {
	address, valid := normalizeAddress(c.Param("address"))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address format"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	stats, err := h.repos.Agents.GetByAddress(ctx, address)
	if err != nil {
		log.Printf("GetAgentEconomics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agent stats"})
		return
	}

	// MON-only PnL: earned (prizes) minus spent (entry fees)
	totalSpentMon := "0"
	totalEarnedMon := "0"
	netPnlMon := "0"
	totalBurnedNeuron := "0"
	var matchRoi float64

	if stats != nil {
		totalEarnedMon = stats.TotalEarnedMON
		totalBurnedNeuron = stats.TotalBurnedNeuron

		// Get actual MON entry fees spent from match history
		monSpent, err := h.repos.Agents.GetTotalMonSpent(ctx, address)
		if err != nil {
			log.Printf("GetAgentEconomics: GetTotalMonSpent: %v", err)
		} else {
			totalSpentMon = monSpent
		}

		earned, _ := new(big.Int).SetString(totalEarnedMon, 10)
		spent, _ := new(big.Int).SetString(totalSpentMon, 10)
		if earned == nil {
			earned = big.NewInt(0)
		}
		if spent == nil {
			spent = big.NewInt(0)
		}
		pnl := new(big.Int).Sub(earned, spent)
		netPnlMon = pnl.String()

		if spent.Sign() > 0 {
			earnedF, _ := new(big.Float).SetInt(earned).Float64()
			spentF, _ := new(big.Float).SetInt(spent).Float64()
			if spentF > 0 {
				matchRoi = earnedF / spentF
			}
		}
	}

	bountiesPlayed, bountiesWon, err := h.repos.Bounties.CountPlayerBounties(ctx, address)
	if err != nil {
		log.Printf("GetAgentEconomics: CountPlayerBounties: %v", err)
		// Non-fatal — continue with zeros
	}

	c.JSON(http.StatusOK, models.AgentEconomics{
		AgentAddr:            address,
		NeuronBalance:        "0", // On-chain query not available server-side
		TotalSpentMon:        totalSpentMon,
		TotalEarnedMon:       totalEarnedMon,
		NetPnlMon:            netPnlMon,
		MatchRoi:             matchRoi,
		TotalBurnedNeuron:    totalBurnedNeuron,
		BountyRoi:            0,
		BountiesParticipated: bountiesPlayed,
		BountiesWon:          bountiesWon,
	})
}
