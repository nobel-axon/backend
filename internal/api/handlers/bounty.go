// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/chief"
	"github.com/axon-arena/axon-server/internal/models"
	"github.com/axon-arena/axon-server/internal/repository"
	"github.com/axon-arena/axon-server/internal/websocket"
)

// BountyHandler handles bounty API requests.
type BountyHandler struct {
	repos       *repository.Repositories
	hub         *websocket.Hub
	chiefClient *chief.Client
}

// NewBountyHandler creates a new bounty handler.
func NewBountyHandler(repos *repository.Repositories, hub *websocket.Hub, chiefClient *chief.Client) *BountyHandler {
	return &BountyHandler{repos: repos, hub: hub, chiefClient: chiefClient}
}

// ListBounties handles GET /api/bounties
// By default, excludes rejected and pending bounties unless explicitly requested.
func (h *BountyHandler) ListBounties(c *gin.Context) {
	phase := c.Query("phase")
	category := c.Query("category")
	creator := c.Query("creator")
	includeRejected := c.Query("include_rejected") == "true"
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

	ctx := c.Request.Context()
	bounties, err := h.repos.Bounties.List(ctx, phase, category, creator, limit, offset)
	if err != nil {
		log.Printf("ListBounties: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bounties"})
		return
	}

	// Get total count for pagination
	var total int
	if phase != "" {
		total, _ = h.repos.Bounties.CountByPhase(ctx, phase)
	} else {
		total, _ = h.repos.Bounties.CountTotal(ctx)
	}

	// Filter out rejected/pending bounties by default
	responses := make([]models.BountyResponse, 0, len(bounties))
	for _, b := range bounties {
		if !includeRejected && (b.Phase == "rejected" || b.Phase == "pending") {
			continue
		}
		responses = append(responses, b.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"bounties": responses,
		"total":    total,
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

	resp := bounty.ToResponse()

	// Populate winnerAnswer if bounty has a winner
	if bounty.WinnerAddress.Valid && len(answers) > 0 {
		winnerAddr := bounty.WinnerAddress.String
		var bestScore int32
		var bestAnswer string
		found := false
		for _, a := range answers {
			if a.AgentAddr == winnerAddr {
				if !found || (a.TotalScore.Valid && a.TotalScore.Int32 > bestScore) {
					bestAnswer = models.SanitizeString(a.AnswerText)
					if a.TotalScore.Valid {
						bestScore = a.TotalScore.Int32
					}
					found = true
				}
			}
		}
		if found {
			resp.WinnerAnswer = &bestAnswer
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"bounty":  resp,
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
		Phase:           "pending",
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

	ctx := c.Request.Context()

	// Fetch bounty participation stats (non-fatal if it fails)
	bountiesPlayed, bountiesWon, _ := h.repos.Bounties.CountPlayerBounties(ctx, address)

	score, feedbackCount, registered, err := h.repos.Agents.GetReputation(ctx, address)
	if err != nil {
		if err == sql.ErrNoRows {
			// Check indexer tables for ERC-8004 registration and reputation data
			idxScore, idxCount, idxReg, idxAgentID := h.repos.Agents.GetReputationFromIndexer(ctx, address)
			if idxReg || idxCount > 0 {
				c.JSON(http.StatusOK, models.ReputationResponse{
					AgentAddr:             address,
					ReputationScore:       idxScore,
					ReputationFeedbackCnt: idxCount,
					ERC8004Registered:     idxReg,
					ERC8004AgentID:        idxAgentID,
					BountiesPlayed:        bountiesPlayed,
					BountiesWon:           bountiesWon,
				})
				return
			}
			c.JSON(http.StatusOK, models.ReputationResponse{
				AgentAddr:      address,
				BountiesPlayed: bountiesPlayed,
				BountiesWon:    bountiesWon,
			})
			return
		}
		log.Printf("GetAgentReputation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reputation"})
		return
	}

	// Fetch ERC-8004 agent ID from app_agent_stats
	agentID, _ := h.repos.Agents.GetERC8004AgentID(ctx, address)

	c.JSON(http.StatusOK, models.ReputationResponse{
		AgentAddr:             address,
		ReputationScore:       score,
		ReputationFeedbackCnt: feedbackCount,
		ERC8004Registered:     registered,
		ERC8004AgentID:        agentID,
		BountiesPlayed:        bountiesPlayed,
		BountiesWon:           bountiesWon,
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

	activeBounties, _ := h.repos.Bounties.CountByPhase(ctx, "active")
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

// InternalBountySettledRequest is the request for recording a bounty settlement.
type InternalBountySettledRequest struct {
	BountyID     int64  `json:"bountyId" binding:"required"`
	WinnerAddr   string `json:"winnerAddr"`
	Reward       string `json:"reward"`
	SettleTxHash string `json:"settleTxHash"`
}

// RecordBountySettled handles POST /internal/bounty-settled
func (h *BountyHandler) RecordBountySettled(c *gin.Context) {
	var req InternalBountySettledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	if err := h.repos.Bounties.SetWinner(ctx, req.BountyID, req.WinnerAddr, req.SettleTxHash); err != nil {
		log.Printf("Internal: bounty-settled failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to settle bounty"})
		return
	}

	// Track bounty win in agent stats
	if req.WinnerAddr != "" {
		if err := h.repos.Agents.IncrementBountiesWon(ctx, req.WinnerAddr); err != nil {
			log.Printf("Internal: bounty-settled IncrementBountiesWon failed: %v", err)
		}
	}

	// Prefer reward from indexer payload; fall back to DB if absent
	rewardAmount := req.Reward
	if rewardAmount == "" {
		if bounty, err := h.repos.Bounties.GetByID(ctx, req.BountyID); err == nil && bounty != nil {
			rewardAmount = bounty.PoolTotal
		}
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventBountySettled,
		Data: websocket.BountySettledData{
			BountyID:     req.BountyID,
			WinnerAddr:   req.WinnerAddr,
			RewardAmount: rewardAmount,
		},
	})

	// Forward to Chief
	if h.chiefClient != nil {
		go h.chiefClient.ReportEvent(context.Background(), chief.EventTypeBountySettled, req.BountyID, map[string]interface{}{
			"winnerAddr":   req.WinnerAddr,
			"settleTxHash": req.SettleTxHash,
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "settled", "bountyId": req.BountyID})
}

// InternalBountyAnswerRequest is the request for recording a bounty answer.
type InternalBountyAnswerRequest struct {
	BountyID      int64  `json:"bountyId" binding:"required"`
	AgentAddr     string `json:"agent" binding:"required"`
	AnswerText    string `json:"answer"`
	AttemptNumber int    `json:"attemptNumber"`
	NeuronBurned  string `json:"neuronBurned"`
	Reasoning     string `json:"reasoning"`
	TxHash        string `json:"txHash"`
}

// RecordBountyAnswerSubmitted handles POST /internal/bounty-answer-submitted
func (h *BountyHandler) RecordBountyAnswerSubmitted(c *gin.Context) {
	var req InternalBountyAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	attemptNum := req.AttemptNumber
	if attemptNum <= 0 {
		attemptNum = 1
	}
	neuronBurned := req.NeuronBurned
	if neuronBurned == "" {
		neuronBurned = "0"
	}

	answer := &models.BountyAnswer{
		BountyID:      req.BountyID,
		AgentAddr:     req.AgentAddr,
		AnswerText:    req.AnswerText,
		Reasoning:     sql.NullString{String: req.Reasoning, Valid: req.Reasoning != ""},
		TxHash:        sql.NullString{String: req.TxHash, Valid: req.TxHash != ""},
		AttemptNumber: attemptNum,
		NeuronBurned:  neuronBurned,
	}

	ctx := c.Request.Context()
	if _, err := h.repos.BountyAnswers.Create(ctx, answer); err != nil {
		log.Printf("Internal: bounty-answer-submitted failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record bounty answer"})
		return
	}

	// Increment answer count on the bounty
	if err := h.repos.Bounties.IncrementAnswerCount(ctx, req.BountyID); err != nil {
		log.Printf("Internal: bounty-answer-submitted IncrementAnswerCount failed: %v", err)
	}

	// Track burned NEURON in agent stats
	if req.AgentAddr != "" && neuronBurned != "0" {
		if err := h.repos.Agents.AddBurnedNeuron(ctx, req.AgentAddr, neuronBurned); err != nil {
			log.Printf("Internal: bounty-answer-submitted AddBurnedNeuron failed: %v", err)
		}
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventBountyAnswerSubmitted,
		Data: websocket.BountyAnswerSubmittedData{
			BountyID:      req.BountyID,
			AgentAddr:     req.AgentAddr,
			AttemptNumber: attemptNum,
		},
	})

	// Forward to Chief for judge evaluation (include txHash for dedup)
	if h.chiefClient != nil {
		go h.chiefClient.ReportEvent(context.Background(), chief.EventTypeBountyAnswerSubmitted, req.BountyID, map[string]interface{}{
			"agent":         req.AgentAddr,
			"answer":        req.AnswerText,
			"reasoning":     req.Reasoning,
			"attemptNumber": float64(attemptNum),
			"neuronBurned":  neuronBurned,
			"txHash":        req.TxHash,
		})
	}

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

	newTotal, _, _, err := h.repos.Agents.GetReputation(c.Request.Context(), req.AgentAddr)
	if err != nil {
		log.Printf("Internal: reputation-updated could not read new total: %v", err)
		newTotal = req.Score // fallback to delta
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventReputationUpdated,
		Data: websocket.ReputationUpdatedData{
			AgentAddr: req.AgentAddr,
			Score:     newTotal,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// BountyUpdateRequest is the generic bounty update from the indexer.
// The indexer sends different event shapes through the same endpoint:
// - BountyCreated: bountyId, phase:"pending", creator, reward, question, category, difficulty, minRating, deadline, maxAgents, baseAnswerFee
// - BountyApproved: bountyId, phase:"active"
// - BountyRejected: bountyId, phase:"rejected", rejectionReason
// - AgentJoinedBounty: bountyId, agent, agentId, agentCount, snapshotReputation
type BountyUpdateRequest struct {
	BountyID           int64  `json:"bountyId" binding:"required"`
	Phase              string `json:"phase"`
	Creator            string `json:"creator"`
	Reward             string `json:"reward"`
	Question           string `json:"question"`
	Category           string `json:"category"`
	Difficulty         int    `json:"difficulty"`
	MinRating          string `json:"minRating"`
	Deadline           string `json:"deadline"`
	MaxAgents          int    `json:"maxAgents"`
	Agent              string `json:"agent"`
	AgentID            int64  `json:"agentId"`
	AgentCount         int    `json:"agentCount"`
	BaseAnswerFee      string `json:"baseAnswerFee"`
	SnapshotReputation string `json:"snapshotReputation"`
	RejectionReason    string `json:"rejectionReason"`
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
		if err := h.repos.Bounties.AddPlayer(ctx, req.BountyID, req.Agent, req.AgentID, req.SnapshotReputation); err != nil {
			log.Printf("Internal: bounty-update AddPlayer failed: %v", err)
		}
		if err := h.repos.Agents.IncrementBountiesPlayed(ctx, req.Agent); err != nil {
			log.Printf("Internal: bounty-update IncrementBountiesPlayed failed: %v", err)
		}
		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventAgentJoinedBounty,
			Data: websocket.AgentJoinedBountyData{
				BountyID:  req.BountyID,
				AgentAddr: req.Agent,
			},
		})
		// Forward to Chief (include agentId for reputation feedback)
		if h.chiefClient != nil {
			go h.chiefClient.ReportEvent(context.Background(), chief.EventTypeAgentJoinedBounty, req.BountyID, map[string]interface{}{
				"agent":       req.Agent,
				"agentId":     float64(req.AgentID),
				"playerCount": float64(req.AgentCount),
			})
		}
		c.JSON(http.StatusOK, gin.H{"status": "player_added", "bountyId": req.BountyID})
		return
	}

	// Handle phase transitions for existing bounties (approved/rejected)
	if req.Phase == "active" && req.Creator == "" {
		// BountyApproved event — update phase and approved_at
		now := time.Now()
		if err := h.repos.Bounties.UpdatePhase(ctx, req.BountyID, "active"); err != nil {
			log.Printf("Internal: bounty-update phase->active failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve bounty"})
			return
		}
		if err := h.repos.Bounties.SetApprovedAt(ctx, req.BountyID, now); err != nil {
			log.Printf("Internal: bounty-update SetApprovedAt failed: %v", err)
		}

		// Broadcast approval WS event
		if bounty, err := h.repos.Bounties.GetByID(ctx, req.BountyID); err == nil && bounty != nil {
			h.hub.Broadcast(websocket.WSEvent{
				Type: websocket.EventBountyApproved,
				Data: websocket.BountyApprovedData{
					BountyID: bounty.BountyID,
					Bounty:   bounty.ToResponse(),
				},
			})
		}

		c.JSON(http.StatusOK, gin.H{"status": "approved", "bountyId": req.BountyID})
		return
	}

	if req.Phase == "rejected" {
		// BountyRejected event — update phase and rejection info
		now := time.Now()
		if err := h.repos.Bounties.UpdatePhase(ctx, req.BountyID, "rejected"); err != nil {
			log.Printf("Internal: bounty-update phase->rejected failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject bounty"})
			return
		}
		if err := h.repos.Bounties.SetRejectedAt(ctx, req.BountyID, now, req.RejectionReason); err != nil {
			log.Printf("Internal: bounty-update SetRejectedAt failed: %v", err)
		}

		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventBountyRejected,
			Data: websocket.BountyRejectedData{
				BountyID: req.BountyID,
				Reason:   req.RejectionReason,
			},
		})

		log.Printf("Internal: bounty %d rejected: %s", req.BountyID, req.RejectionReason)
		c.JSON(http.StatusOK, gin.H{"status": "rejected", "bountyId": req.BountyID})
		return
	}

	// Otherwise upsert the bounty record (BountyCreated — starts as pending)
	var deadline sql.NullTime
	if req.Deadline != "" {
		if t, err := time.Parse(time.RFC3339, req.Deadline); err == nil {
			deadline = sql.NullTime{Time: t, Valid: true}
		}
	}

	bounty := &models.Bounty{
		BountyID:        req.BountyID,
		CreatorAddress:  req.Creator,
		QuestionText:    req.Question,
		Category:        sql.NullString{String: req.Category, Valid: req.Category != ""},
		Difficulty:      req.Difficulty,
		EntryFee:        "0",
		BaseAnswerFee:   req.BaseAnswerFee,
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

	// Forward to Chief for bounty lifecycle management (pending bounties need screening)
	if req.Phase == "pending" && h.chiefClient != nil {
		var deadlineUnix float64
		if deadline.Valid {
			deadlineUnix = float64(deadline.Time.Unix())
		}
		go h.chiefClient.ReportEvent(context.Background(), chief.EventTypeBountyCreated, req.BountyID, map[string]interface{}{
			"creator":         req.Creator,
			"question":        req.Question,
			"category":        req.Category,
			"difficulty":      float64(req.Difficulty),
			"entryFee":        "0",
			"deadline":        deadlineUnix,
			"maxParticipants": float64(req.MaxAgents),
			"minRating":       req.MinRating,
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

	if err := h.repos.Agents.SetERC8004Registered(c.Request.Context(), req.Wallet, req.AgentID); err != nil {
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

	// Look up the agent's wallet from agentId (the agent who received feedback)
	var wallet string
	if req.AgentID > 0 {
		w, err := h.repos.Agents.GetWalletByERC8004AgentID(c.Request.Context(), req.AgentID)
		if err == nil && w != "" {
			wallet = w
		} else {
			// Fallback: query indexer's chain_agent_registered table (shared PG)
			w, err := h.repos.Agents.GetWalletByERC8004AgentIDFromIndexer(c.Request.Context(), req.AgentID)
			if err == nil && w != "" {
				wallet = w
				// Backfill the app_agent_stats record
				_ = h.repos.Agents.SetERC8004Registered(c.Request.Context(), w, req.AgentID)
			}
		}
	}
	if wallet == "" {
		log.Printf("Internal: feedback-submitted could not resolve wallet for agentId=%d, skipping", req.AgentID)
		c.JSON(http.StatusOK, gin.H{"status": "skipped"})
		return
	}

	if err := h.repos.Agents.UpdateReputation(c.Request.Context(), wallet, req.Value); err != nil {
		log.Printf("Internal: feedback-submitted reputation update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reputation"})
		return
	}

	newTotal, _, _, err := h.repos.Agents.GetReputation(c.Request.Context(), wallet)
	if err != nil {
		log.Printf("Internal: feedback-submitted could not read new total: %v", err)
		newTotal = req.Value // fallback to delta
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventReputationUpdated,
		Data: websocket.ReputationUpdatedData{
			AgentAddr: wallet,
			Score:     newTotal,
		},
	})

	log.Printf("Internal: feedback submitted for agent %d (client=%s, value=%d, tx=%s)", req.AgentID, req.Client, req.Value, req.TxHash)
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// InternalBountyClaimRequest is the request for recording bounty claims.
type InternalBountyClaimRequest struct {
	BountyID int64  `json:"bountyId" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Winner   string `json:"winner"`
	Agent    string `json:"agent"`
	Creator  string `json:"creator"`
	Amount   string `json:"amount"`
}

// RecordBountyClaim handles POST /internal/bounty-claim
func (h *BountyHandler) RecordBountyClaim(c *gin.Context) {
	var req InternalBountyClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Determine the claimant address for DB persistence
	var claimantAddr string
	switch req.Type {
	case "winner_reward_claimed":
		claimantAddr = req.Winner
	case "proportional_claimed":
		claimantAddr = req.Agent
	case "refund_claimed":
		claimantAddr = req.Creator
	}

	// Persist claim to DB
	if claimantAddr != "" {
		if err := h.repos.BountyClaims.Create(ctx, req.BountyID, req.Type, claimantAddr, req.Amount); err != nil {
			log.Printf("Internal: bounty-claim persist failed: %v", err)
		}
	}

	// Update bounty phase to "refunded" on refund claim (no on-chain phase change for this)
	if req.Type == "refund_claimed" {
		if err := h.repos.Bounties.UpdatePhase(ctx, req.BountyID, "refunded"); err != nil {
			log.Printf("Internal: bounty-claim UpdatePhase to refunded failed: %v", err)
		}
	}

	switch req.Type {
	case "winner_reward_claimed":
		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventWinnerRewardClaimed,
			Data: websocket.WinnerRewardClaimedData{
				BountyID: req.BountyID,
				Winner:   req.Winner,
				Amount:   req.Amount,
			},
		})
	case "proportional_claimed":
		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventProportionalClaimed,
			Data: websocket.ProportionalClaimedData{
				BountyID: req.BountyID,
				Agent:    req.Agent,
				Amount:   req.Amount,
			},
		})
	case "refund_claimed":
		h.hub.Broadcast(websocket.WSEvent{
			Type: websocket.EventRefundClaimed,
			Data: websocket.RefundClaimedData{
				BountyID: req.BountyID,
				Creator:  req.Creator,
				Amount:   req.Amount,
			},
		})
	}

	log.Printf("Internal: bounty-claim type=%s bountyId=%d", req.Type, req.BountyID)
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// BountyAnswerResultRequest is the request from Chief after evaluating a bounty answer.
type BountyAnswerResultRequest struct {
	BountyID      int64           `json:"bountyId" binding:"required"`
	AgentAddr     string          `json:"agentAddr" binding:"required"`
	AttemptNumber int             `json:"attemptNumber"`
	TotalScore    int             `json:"totalScore"`
	Agreement     string          `json:"agreement"`
	Evaluations   json.RawMessage `json:"evaluations,omitempty"`
}

// RecordBountyAnswerResult handles POST /internal/bounty-answer-result
func (h *BountyHandler) RecordBountyAnswerResult(c *gin.Context) {
	var req BountyAnswerResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: bounty-answer-result bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	attemptNum := req.AttemptNumber
	if attemptNum <= 0 {
		attemptNum = 1
	}

	ctx := c.Request.Context()
	if err := h.repos.BountyAnswers.UpdateEvaluation(ctx, req.BountyID, req.AgentAddr, attemptNum, req.TotalScore, req.Agreement, req.Evaluations); err != nil {
		log.Printf("Internal: failed to update bounty answer evaluation bountyId=%d agent=%s: %v", req.BountyID, req.AgentAddr, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update evaluation"})
		return
	}

	h.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventBountyAnswerEvaluated,
		Data: websocket.BountyAnswerEvaluatedData{
			BountyID:   req.BountyID,
			AgentAddr:  req.AgentAddr,
			TotalScore: req.TotalScore,
			Agreement:  req.Agreement,
		},
	})

	log.Printf("Internal: bounty-answer-result bountyId=%d agent=%s score=%d agreement=%s", req.BountyID, req.AgentAddr, req.TotalScore, req.Agreement)
	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}
