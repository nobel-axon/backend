// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/models"
	"github.com/axon-arena/axon-server/internal/repository"
	"github.com/axon-arena/axon-server/internal/websocket"
)

// BountyHandler handles bounty API requests.
type BountyHandler struct {
	repos *repository.Repositories
	hub   *websocket.Hub
}

// NewBountyHandler creates a new bounty handler.
func NewBountyHandler(repos *repository.Repositories, hub *websocket.Hub) *BountyHandler {
	return &BountyHandler{repos: repos, hub: hub}
}

// ListBounties handles GET /api/bounties
func (h *BountyHandler) ListBounties(c *gin.Context) {
	phase := c.Query("phase")
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

	bounties, err := h.repos.Bounties.List(c.Request.Context(), phase, limit, offset)
	if err != nil {
		log.Printf("ListBounties: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bounties"})
		return
	}

	responses := make([]models.BountyResponse, len(bounties))
	for i, b := range bounties {
		responses[i] = b.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"bounties": responses,
		"limit":    limit,
		"offset":   offset,
	})
}

// GetBounty handles GET /api/bounties/:id
func (h *BountyHandler) GetBounty(c *gin.Context) {
	idStr := c.Param("id")
	bountyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bounty ID"})
		return
	}

	bounty, err := h.repos.Bounties.GetByID(c.Request.Context(), bountyID)
	if err != nil {
		log.Printf("GetBounty: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bounty"})
		return
	}
	if bounty == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bounty not found"})
		return
	}

	// Get answers
	answers, err := h.repos.BountyAnswers.GetByBounty(c.Request.Context(), bountyID)
	if err != nil {
		log.Printf("GetBounty: failed to fetch answers: %v", err)
	}

	answerResponses := make([]models.BountyAnswerResponse, len(answers))
	for i, a := range answers {
		answerResponses[i] = a.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"bounty":  bounty.ToResponse(),
		"answers": answerResponses,
	})
}

// CreateBountyRequest is the request for creating a bounty.
type CreateBountyRequest struct {
	QuestionText    string `json:"questionText" binding:"required"`
	Category        string `json:"category"`
	Difficulty      int    `json:"difficulty"`
	EntryFee        string `json:"entryFee" binding:"required"`
	Deadline        string `json:"deadline" binding:"required"`
	MaxParticipants int    `json:"maxParticipants"`
	MinRating       string `json:"minRating"`
	CreatorAddress  string `json:"creatorAddress" binding:"required"`
}

// CreateBounty handles POST /api/bounties
func (h *BountyHandler) CreateBounty(c *gin.Context) {
	var req CreateBountyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deadline, err := time.Parse(time.RFC3339, req.Deadline)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid deadline format (use RFC3339)"})
		return
	}

	maxPart := req.MaxParticipants
	if maxPart <= 0 {
		maxPart = 10
	}
	minRating := req.MinRating
	if minRating == "" {
		minRating = "0"
	}

	bounty := &models.Bounty{
		CreatorAddress:  req.CreatorAddress,
		QuestionText:    req.QuestionText,
		Category:        sql.NullString{String: req.Category, Valid: req.Category != ""},
		Difficulty:      req.Difficulty,
		EntryFee:        req.EntryFee,
		PoolTotal:       "0",
		MinRating:       minRating,
		MaxParticipants: maxPart,
		Phase:           "open",
		Deadline:        sql.NullTime{Time: deadline, Valid: true},
	}

	if err := h.repos.Bounties.Create(c.Request.Context(), bounty); err != nil {
		log.Printf("CreateBounty: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bounty"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "created", "bounty": bounty.ToResponse()})
}

// GetAgentReputation handles GET /api/agent/:address/reputation
func (h *BountyHandler) GetAgentReputation(c *gin.Context) {
	address, valid := normalizeAddress(c.Param("address"))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address format"})
		return
	}

	score, feedbackCount, registered, err := h.repos.Agents.GetReputation(c.Request.Context(), address)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, models.ReputationResponse{AgentAddr: address})
			return
		}
		log.Printf("GetAgentReputation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reputation"})
		return
	}

	c.JSON(http.StatusOK, models.ReputationResponse{
		AgentAddr:             address,
		ReputationScore:       score,
		ReputationFeedbackCnt: feedbackCount,
		ERC8004Registered:     registered,
	})
}

// GetBountyStats handles GET /api/bounties/stats
func (h *BountyHandler) GetBountyStats(c *gin.Context) {
	ctx := c.Request.Context()

	totalBounties, err := h.repos.Bounties.CountTotal(ctx)
	if err != nil {
		log.Printf("GetBountyStats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bounty stats"})
		return
	}

	activeOpen, _ := h.repos.Bounties.CountByPhase(ctx, "open")
	activeAnswer, _ := h.repos.Bounties.CountByPhase(ctx, "answer_period")
	activeBounties := activeOpen + activeAnswer

	settledBounties, _ := h.repos.Bounties.CountByPhase(ctx, "settled")
	totalRewardPool, _ := h.repos.Bounties.GetTotalRewardPool(ctx)
	avgReward, _ := h.repos.Bounties.GetAvgReward(ctx)

	c.JSON(http.StatusOK, models.BountyStatsResponse{
		TotalBounties:   totalBounties,
		ActiveBounties:  activeBounties,
		SettledBounties: settledBounties,
		TotalRewardPool: totalRewardPool,
		AvgReward:       avgReward,
	})
}

// --- Internal bounty handlers (called by Chief or Indexer) ---

// InternalBountyCreatedRequest is the request for recording a bounty creation.
type InternalBountyCreatedRequest struct {
	BountyID        int64  `json:"bountyId" binding:"required"`
	CreatorAddress  string `json:"creatorAddress" binding:"required"`
	QuestionText    string `json:"questionText"`
	Category        string `json:"category"`
	Difficulty      int    `json:"difficulty"`
	EntryFee        string `json:"entryFee"`
	Deadline        string `json:"deadline"`
	MaxParticipants int    `json:"maxParticipants"`
	MinRating       string `json:"minRating"`
}

// RecordBountyCreated handles POST /internal/bounty-created
func (h *BountyHandler) RecordBountyCreated(c *gin.Context) {
	var req InternalBountyCreatedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var deadline sql.NullTime
	if req.Deadline != "" {
		if t, err := time.Parse(time.RFC3339, req.Deadline); err == nil {
			deadline = sql.NullTime{Time: t, Valid: true}
		}
	}

	bounty := &models.Bounty{
		BountyID:        req.BountyID,
		CreatorAddress:  req.CreatorAddress,
		QuestionText:    req.QuestionText,
		Category:        sql.NullString{String: req.Category, Valid: req.Category != ""},
		Difficulty:      req.Difficulty,
		EntryFee:        req.EntryFee,
		MinRating:       req.MinRating,
		MaxParticipants: req.MaxParticipants,
		Phase:           "open",
		Deadline:        deadline,
	}

	if err := h.repos.Bounties.Create(c.Request.Context(), bounty); err != nil {
		log.Printf("Internal: bounty-created failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record bounty"})
		return
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: "bounty_created",
		Data: bounty.ToResponse(),
	})

	c.JSON(http.StatusOK, gin.H{"status": "created", "bountyId": req.BountyID})
}

// InternalBountySettledRequest is the request for recording a bounty settlement.
type InternalBountySettledRequest struct {
	BountyID     int64  `json:"bountyId" binding:"required"`
	WinnerAddr   string `json:"winnerAddr"`
	SettleTxHash string `json:"settleTxHash"`
}

// RecordBountySettled handles POST /internal/bounty-settled
func (h *BountyHandler) RecordBountySettled(c *gin.Context) {
	var req InternalBountySettledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repos.Bounties.SetWinner(c.Request.Context(), req.BountyID, req.WinnerAddr, req.SettleTxHash); err != nil {
		log.Printf("Internal: bounty-settled failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to settle bounty"})
		return
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: "bounty_settled",
		Data: map[string]interface{}{
			"bountyId":   req.BountyID,
			"winnerAddr": req.WinnerAddr,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "settled", "bountyId": req.BountyID})
}

// InternalBountyAnswerRequest is the request for recording a bounty answer.
type InternalBountyAnswerRequest struct {
	BountyID   int64  `json:"bountyId" binding:"required"`
	AgentAddr  string `json:"agentAddr" binding:"required"`
	AnswerText string `json:"answerText"`
	Reasoning  string `json:"reasoning"`
	TxHash     string `json:"txHash"`
}

// RecordBountyAnswerSubmitted handles POST /internal/bounty-answer-submitted
func (h *BountyHandler) RecordBountyAnswerSubmitted(c *gin.Context) {
	var req InternalBountyAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	answer := &models.BountyAnswer{
		BountyID:   req.BountyID,
		AgentAddr:  req.AgentAddr,
		AnswerText: req.AnswerText,
		Reasoning:  sql.NullString{String: req.Reasoning, Valid: req.Reasoning != ""},
		TxHash:     sql.NullString{String: req.TxHash, Valid: req.TxHash != ""},
	}

	if _, err := h.repos.BountyAnswers.Create(c.Request.Context(), answer); err != nil {
		log.Printf("Internal: bounty-answer-submitted failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record bounty answer"})
		return
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: "bounty_answer_submitted",
		Data: map[string]interface{}{
			"bountyId":  req.BountyID,
			"agentAddr": req.AgentAddr,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// InternalReputationUpdatedRequest is the request for recording a reputation update.
type InternalReputationUpdatedRequest struct {
	AgentAddr string `json:"agentAddr" binding:"required"`
	Score     int    `json:"score"`
}

// RecordReputationUpdated handles POST /internal/reputation-updated
func (h *BountyHandler) RecordReputationUpdated(c *gin.Context) {
	var req InternalReputationUpdatedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repos.Agents.UpdateReputation(c.Request.Context(), req.AgentAddr, req.Score); err != nil {
		log.Printf("Internal: reputation-updated failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reputation"})
		return
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: "reputation_updated",
		Data: map[string]interface{}{
			"agentAddr": req.AgentAddr,
			"score":     req.Score,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// BountyUpdateRequest is the generic bounty update from the indexer.
// The indexer sends different event shapes through the same endpoint:
// - BountyCreated: bountyId, phase:"open", creator, reward, question, category, difficulty, minRating, joinDeadline, maxAgents
// - AgentJoinedBounty: bountyId, agent, agentId, agentCount
// - BountyAnswerPeriodStarted: bountyId, phase:"answer_period", answerTimeout
type BountyUpdateRequest struct {
	BountyID      int64  `json:"bountyId" binding:"required"`
	Phase         string `json:"phase"`
	Creator       string `json:"creator"`
	Reward        string `json:"reward"`
	Question      string `json:"question"`
	Category      string `json:"category"`
	Difficulty    int    `json:"difficulty"`
	MinRating     string `json:"minRating"`
	JoinDeadline  string `json:"joinDeadline"`
	MaxAgents     int    `json:"maxAgents"`
	Agent         string `json:"agent"`
	AgentID       int64  `json:"agentId"`
	AgentCount    int    `json:"agentCount"`
	AnswerTimeout string `json:"answerTimeout"`
}

// RecordBountyUpdate handles POST /internal/bounty-update
func (h *BountyHandler) RecordBountyUpdate(c *gin.Context) {
	var req BountyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// If an agent joined, record the player
	if req.Agent != "" {
		if err := h.repos.Bounties.AddPlayer(ctx, req.BountyID, req.Agent); err != nil {
			log.Printf("Internal: bounty-update AddPlayer failed: %v", err)
		}
		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventAgentJoinedBounty,
			Data: websocket.AgentJoinedBountyData{
				BountyID:  req.BountyID,
				AgentAddr: req.Agent,
			},
		})
		c.JSON(http.StatusOK, gin.H{"status": "player_added", "bountyId": req.BountyID})
		return
	}

	// Otherwise upsert the bounty record
	var deadline sql.NullTime
	if req.JoinDeadline != "" {
		if t, err := time.Parse(time.RFC3339, req.JoinDeadline); err == nil {
			deadline = sql.NullTime{Time: t, Valid: true}
		}
	}
	if req.AnswerTimeout != "" {
		if t, err := time.Parse(time.RFC3339, req.AnswerTimeout); err == nil {
			deadline = sql.NullTime{Time: t, Valid: true}
		}
	}

	bounty := &models.Bounty{
		BountyID:        req.BountyID,
		CreatorAddress:  req.Creator,
		QuestionText:    req.Question,
		Category:        sql.NullString{String: req.Category, Valid: req.Category != ""},
		Difficulty:      req.Difficulty,
		EntryFee:        req.Reward,
		PoolTotal:       req.Reward,
		MinRating:       req.MinRating,
		MaxParticipants: req.MaxAgents,
		PlayerCount:     req.AgentCount,
		Phase:           req.Phase,
		Deadline:        deadline,
	}

	if err := h.repos.Bounties.Upsert(ctx, bounty); err != nil {
		log.Printf("Internal: bounty-update upsert failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert bounty"})
		return
	}

	// Broadcast appropriate WS event
	if req.Phase == "open" {
		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventBountyCreated,
			Data: bounty.ToResponse(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "bountyId": req.BountyID})
}

// AgentRegisteredRequest is the request from the indexer when an agent registers on ERC-8004.
type AgentRegisteredRequest struct {
	AgentID int64  `json:"agentId"`
	Wallet  string `json:"wallet" binding:"required"`
	TxHash  string `json:"txHash"`
}

// RecordAgentRegistered handles POST /internal/agent-registered
func (h *BountyHandler) RecordAgentRegistered(c *gin.Context) {
	var req AgentRegisteredRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repos.Agents.SetERC8004Registered(c.Request.Context(), req.Wallet); err != nil {
		log.Printf("Internal: agent-registered failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record agent registration"})
		return
	}

	log.Printf("Internal: agent %s (id=%d) registered on ERC-8004 (tx=%s)", req.Wallet, req.AgentID, req.TxHash)
	c.JSON(http.StatusOK, gin.H{"status": "registered", "wallet": req.Wallet})
}

// FeedbackSubmittedRequest is the request from the indexer when feedback is submitted.
type FeedbackSubmittedRequest struct {
	AgentID  int64  `json:"agentId"`
	Client   string `json:"client"`
	Value    int    `json:"value"`
	Decimals int    `json:"decimals"`
	Tag1     string `json:"tag1"`
	Tag2     string `json:"tag2"`
	TxHash   string `json:"txHash"`
}

// RecordFeedbackSubmitted handles POST /internal/feedback-submitted
func (h *BountyHandler) RecordFeedbackSubmitted(c *gin.Context) {
	var req FeedbackSubmittedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert agentId to wallet address if needed — for now use the client field
	// The reputation update uses the value as the score delta
	wallet := req.Client
	if wallet == "" {
		log.Printf("Internal: feedback-submitted missing client address, skipping reputation update")
		c.JSON(http.StatusOK, gin.H{"status": "skipped"})
		return
	}

	if err := h.repos.Agents.UpdateReputation(c.Request.Context(), wallet, req.Value); err != nil {
		log.Printf("Internal: feedback-submitted reputation update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reputation"})
		return
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventReputationUpdated,
		Data: websocket.ReputationUpdatedData{
			AgentAddr: wallet,
			Score:     req.Value,
		},
	})

	log.Printf("Internal: feedback submitted for agent %d (client=%s, value=%d, tx=%s)", req.AgentID, req.Client, req.Value, req.TxHash)
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}
