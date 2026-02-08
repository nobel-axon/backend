// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/chief"
	"github.com/axon-arena/axon-server/internal/models"
	"github.com/axon-arena/axon-server/internal/repository"
	"github.com/axon-arena/axon-server/internal/websocket"
)

// InternalHandler handles internal API requests from Chief.
type InternalHandler struct {
	repos       *repository.Repositories
	hub         *websocket.Hub
	chiefClient *chief.Client
}

// NewInternalHandler creates a new internal handler.
func NewInternalHandler(repos *repository.Repositories, hub *websocket.Hub, chiefClient *chief.Client) *InternalHandler {
	return &InternalHandler{repos: repos, hub: hub, chiefClient: chiefClient}
}

// MatchUpdateRequest is the request for match updates.
type MatchUpdateRequest struct {
	MatchID         int64   `json:"matchId" binding:"required"`
	Phase           *string `json:"phase,omitempty"`
	QuestionText    *string `json:"questionText,omitempty"`
	Category        *string `json:"category,omitempty"`
	Difficulty      *int    `json:"difficulty,omitempty"`
	FormatHint      *string `json:"formatHint,omitempty"`
	AnswerHash      *string `json:"answerHash,omitempty"`
	GeneratorAgent  *string `json:"generatorAgent,omitempty"`
	RegistrationEnd *string `json:"registrationEnd,omitempty"`
	AnswerTimeout   *string `json:"answerTimeout,omitempty"`
	EntryFee        *string `json:"entryFee,omitempty"`
	AnswerFee       *string `json:"answerFee,omitempty"`
	PoolTotal       *string `json:"poolTotal,omitempty"`
	PlayerCount     *int    `json:"playerCount,omitempty"`
	MinPlayers      *int    `json:"minPlayers,omitempty"`
	MaxPlayers      *int    `json:"maxPlayers,omitempty"`
	Agent           *string `json:"agent,omitempty"` // Agent address for player joins
}

// UpdateMatch handles POST /internal/match-update
func (h *InternalHandler) UpdateMatch(c *gin.Context) {
	var req MatchUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: match-update bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	phase := "<unchanged>"
	if req.Phase != nil {
		phase = *req.Phase
	}
	log.Printf("Internal: received match-update for match %d (phase: %s)", req.MatchID, phase)

	ctx := c.Request.Context()

	// Get existing match or create new one
	match, err := h.repos.Matches.GetByID(ctx, req.MatchID)
	if err != nil {
		log.Printf("UpdateMatch: failed to fetch match %d: %v", req.MatchID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match"})
		return
	}

	isNew := match == nil
	if isNew {
		match = &models.Match{
			MatchID:   req.MatchID,
			Phase:     "created",
			EntryFee:  "0",
			AnswerFee: "0",
			PoolTotal: "0",
		}
	}

	// Update fields
	if req.Phase != nil {
		match.Phase = *req.Phase
	}
	if req.QuestionText != nil {
		match.QuestionText = sql.NullString{String: *req.QuestionText, Valid: true}
	}
	if req.Category != nil {
		match.Category = sql.NullString{String: *req.Category, Valid: true}
	}
	if req.Difficulty != nil {
		match.Difficulty = sql.NullInt32{Int32: int32(*req.Difficulty), Valid: true}
	}
	if req.FormatHint != nil {
		match.FormatHint = sql.NullString{String: *req.FormatHint, Valid: true}
	}
	if req.AnswerHash != nil {
		match.AnswerHash = sql.NullString{String: *req.AnswerHash, Valid: true}
	}
	if req.GeneratorAgent != nil {
		match.GeneratorAgent = sql.NullString{String: *req.GeneratorAgent, Valid: true}
	}
	if req.EntryFee != nil {
		match.EntryFee = *req.EntryFee
	}
	if req.AnswerFee != nil {
		match.AnswerFee = *req.AnswerFee
	}
	if req.PoolTotal != nil {
		match.PoolTotal = *req.PoolTotal
	}
	if req.PlayerCount != nil {
		match.PlayerCount = *req.PlayerCount
	}
	if req.RegistrationEnd != nil {
		if t, err := time.Parse(time.RFC3339, *req.RegistrationEnd); err == nil {
			match.RegistrationEnd = sql.NullTime{Time: t, Valid: true}
		}
	}
	if req.AnswerTimeout != nil {
		if t, err := time.Parse(time.RFC3339, *req.AnswerTimeout); err == nil {
			match.AnswerTimeout = sql.NullTime{Time: t, Valid: true}
		}
	}

	// Save to database
	if isNew {
		if err := h.repos.Matches.Create(ctx, match); err != nil {
			log.Printf("Failed to create match %d: %v", req.MatchID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create match"})
			return
		}
	} else {
		if err := h.repos.Matches.Update(ctx, match); err != nil {
			log.Printf("Failed to update match %d: %v", req.MatchID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update match"})
			return
		}
	}

	// Determine event type based on phase
	eventType := "match_updated"
	if isNew {
		eventType = "match_created"
	} else if req.Phase != nil {
		switch *req.Phase {
		case "registration":
			eventType = "match_created"
		case "question_live":
			eventType = "question_posted"
		case "settled":
			eventType = "match_settled"
		case "cancelled":
			eventType = "match_cancelled"
		}
	}

	// Handle player join if agent address is provided
	if req.Agent != nil && *req.Agent != "" {
		if err := h.repos.Matches.AddPlayer(ctx, req.MatchID, *req.Agent); err != nil {
			log.Printf("Failed to add player %s to match %d: %v", *req.Agent, req.MatchID, err)
		}

		// Forward agent_joined_queue to Chief for grace period + auto-start logic (async).
		playerCount := 0
		if req.PlayerCount != nil {
			playerCount = *req.PlayerCount
		}
		agent := *req.Agent
		matchID := req.MatchID
		go func() {
			_, err := h.chiefClient.ReportEvent(context.Background(), chief.EventTypeAgentJoinedQueue, matchID, map[string]interface{}{
				"agent":       agent,
				"playerCount": playerCount,
			})
			if err != nil {
				log.Printf("Failed to forward agent_joined_queue for match %d to Chief: %v", matchID, err)
			} else {
				log.Printf("Forwarded agent_joined_queue for match %d to Chief (agent=%s)", matchID, agent)
			}
		}()
	}

	// Broadcast to WebSocket clients
	h.hub.Broadcast(websocket.WSEvent{
		Type: eventType,
		Data: match.ToResponse(),
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated", "matchId": req.MatchID})
}

// PersonalitiesRequest is the request for storing match personalities.
type PersonalitiesRequest struct {
	MatchID       int64           `json:"matchId" binding:"required"`
	Personalities json.RawMessage `json:"personalities" binding:"required"`
	JudgePanel    json.RawMessage `json:"judgePanel" binding:"required"`
}

// StorePersonalities handles POST /internal/match-personalities
func (h *InternalHandler) StorePersonalities(c *gin.Context) {
	var req PersonalitiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: match-personalities bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Internal: received match-personalities for match %d", req.MatchID)

	ctx := c.Request.Context()

	if err := h.repos.Personalities.Upsert(ctx, req.MatchID, req.Personalities, req.JudgePanel); err != nil {
		log.Printf("Failed to store personalities for match %d: %v", req.MatchID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store personalities"})
		return
	}

	// Broadcast to WebSocket clients
	h.hub.Broadcast(websocket.WSEvent{
		Type: "personalities_assigned",
		Data: map[string]interface{}{
			"matchId":       req.MatchID,
			"personalities": req.Personalities,
			"judgePanel":    req.JudgePanel,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "stored", "matchId": req.MatchID})
}

// AddCommentary handles POST /internal/commentary
func (h *InternalHandler) AddCommentary(c *gin.Context) {
	var req models.CommentaryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: commentary bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Internal: received commentary for match %d (type: %s)", req.MatchID, req.EventType)

	ctx := c.Request.Context()

	// Create commentary
	id, err := h.repos.Commentary.Create(ctx, &req)
	if err != nil {
		log.Printf("Failed to create commentary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create commentary"})
		return
	}

	// Fetch the created commentary
	commentary, err := h.repos.Commentary.GetByID(ctx, id)
	if err != nil {
		log.Printf("Failed to fetch created commentary: %v", err)
	}

	// Broadcast to WebSocket clients
	if commentary != nil {
		h.hub.Broadcast(websocket.WSEvent{
			Type: "commentary",
			Data: commentary.ToResponse(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"status": "created", "id": id})
}

// AnswerResultRequest is the request for recording answer results.
type AnswerResultRequest struct {
	MatchID       int64           `json:"matchId" binding:"required"`
	AgentAddr     string          `json:"agentAddr" binding:"required"`
	AttemptNumber int             `json:"attemptNumber" binding:"required"`
	IsCorrect     bool            `json:"isCorrect"`
	Consensus     string          `json:"consensus"`
	Confidence    float64         `json:"confidence"`
	TotalScore    *int            `json:"totalScore,omitempty"`
	Agreement     *string         `json:"agreement,omitempty"`
	Reasoning     *string         `json:"reasoning,omitempty"`
	Evaluations   json.RawMessage `json:"evaluations,omitempty"`
}

// RecordAnswerResult handles POST /internal/answer-result
func (h *InternalHandler) RecordAnswerResult(c *gin.Context) {
	var req AnswerResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: answer-result bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Internal: received answer-result for match %d (agent: %s, attempt: %d, correct: %v, consensus: %s)",
		req.MatchID, req.AgentAddr, req.AttemptNumber, req.IsCorrect, req.Consensus)

	ctx := c.Request.Context()

	// Update answer verification result
	err := h.repos.Answers.UpdateVerificationByMatch(
		ctx, req.MatchID, req.AgentAddr, req.AttemptNumber,
		req.IsCorrect, req.Consensus, req.Confidence,
	)
	if err != nil {
		log.Printf("Failed to update answer result: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update answer result"})
		return
	}

	// Persist evaluation details if provided
	if req.TotalScore != nil && req.Agreement != nil {
		reasoning := ""
		if req.Reasoning != nil {
			reasoning = *req.Reasoning
		}
		if err := h.repos.Answers.UpdateEvaluation(
			ctx, req.MatchID, req.AgentAddr, req.AttemptNumber,
			*req.TotalScore, *req.Agreement, reasoning, req.Evaluations,
		); err != nil {
			log.Printf("Failed to update answer evaluation: %v", err)
		}
	}

	// Update agent stats
	answers, err := h.repos.Answers.GetByMatchAndAgent(ctx, req.MatchID, req.AgentAddr)
	if err == nil && len(answers) > 0 {
		for _, a := range answers {
			if a.AttemptNumber == req.AttemptNumber {
				if err := h.repos.Agents.RecordAnswer(ctx, req.AgentAddr, req.IsCorrect, a.NeuronBurned); err != nil {
					log.Printf("Failed to update agent stats: %v", err)
				}
				break
			}
		}
	}

	// Broadcast to WebSocket clients
	h.hub.Broadcast(websocket.WSEvent{
		Type: "answer_verified",
		Data: map[string]interface{}{
			"matchId":       req.MatchID,
			"agentAddr":     req.AgentAddr,
			"attemptNumber": req.AttemptNumber,
			"isCorrect":     req.IsCorrect,
			"consensus":     req.Consensus,
			"confidence":    req.Confidence,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// SettleMatchRequest is the request for settling a match.
type SettleMatchRequest struct {
	MatchID      int64  `json:"matchId" binding:"required"`
	WinnerAddr   string `json:"winnerAddr"`
	PrizeMON     string `json:"prizeMon"`
	PrizeNeuron  string `json:"prizeNeuron"`
	Reason       string `json:"reason,omitempty"` // For cancellations
	IsCancelled  bool   `json:"isCancelled"`
}

// SettleMatch handles POST /internal/match-settled
func (h *InternalHandler) SettleMatch(c *gin.Context) {
	var req SettleMatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: match-settled bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsCancelled {
		log.Printf("Internal: received match-cancelled for match %d (reason: %s)", req.MatchID, req.Reason)
	} else {
		log.Printf("Internal: received match-settled for match %d (winner: %s)", req.MatchID, req.WinnerAddr)
	}

	ctx := c.Request.Context()

	// Update match status
	if req.IsCancelled {
		if err := h.repos.Matches.UpdatePhase(ctx, req.MatchID, "cancelled"); err != nil {
			log.Printf("Failed to cancel match %d: %v", req.MatchID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel match"})
			return
		}

		// Broadcast cancellation
		h.hub.Broadcast(websocket.WSEvent{
			Type: "match_cancelled",
			Data: map[string]interface{}{
				"matchId": req.MatchID,
				"reason":  req.Reason,
			},
		})

		c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
		return
	}

	// Set winner
	if err := h.repos.Matches.SetWinner(ctx, req.MatchID, req.WinnerAddr); err != nil {
		log.Printf("Failed to settle match %d: %v", req.MatchID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to settle match"})
		return
	}

	// Update winner's stats
	if req.WinnerAddr != "" {
		if err := h.repos.Agents.RecordWin(ctx, req.WinnerAddr, req.PrizeMON, req.PrizeNeuron); err != nil {
			log.Printf("Failed to update winner stats: %v", err)
		}
	}

	// Update all players' matches_played count
	players, err := h.repos.Matches.GetPlayers(ctx, req.MatchID)
	if err == nil {
		for _, p := range players {
			if err := h.repos.Agents.IncrementMatchesPlayed(ctx, p.AgentAddr); err != nil {
				log.Printf("Failed to increment matches_played for %s: %v", p.AgentAddr, err)
			}
		}
	}

	// Broadcast settlement
	h.hub.Broadcast(websocket.WSEvent{
		Type: "match_settled",
		Data: map[string]interface{}{
			"matchId":     req.MatchID,
			"winnerAddr":  req.WinnerAddr,
			"prizeMon":    req.PrizeMON,
			"prizeNeuron": req.PrizeNeuron,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "settled"})
}

// AnswerSubmittedRequest is the request for recording an answer submission from chain.
type AnswerSubmittedRequest struct {
	MatchID       int64  `json:"matchId" binding:"required"`
	Agent         string `json:"agent" binding:"required"`
	Answer        string `json:"answer"`
	Reasoning     string `json:"reasoning"`
	AttemptNumber int    `json:"attemptNumber"`
	NeuronBurned  string `json:"neuronBurned"`
	TxHash        string `json:"txHash"`
	LogIndex      int    `json:"logIndex"`
}

// RecordAnswerSubmitted handles POST /internal/answer-submitted
func (h *InternalHandler) RecordAnswerSubmitted(c *gin.Context) {
	var req AnswerSubmittedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: answer-submitted bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Internal: received answer-submitted for match %d (agent: %s, attempt: %d, neuronBurned: %s)",
		req.MatchID, req.Agent, req.AttemptNumber, req.NeuronBurned)

	ctx := c.Request.Context()

	// Record the answer
	answerModel := &models.Answer{
		MatchID:       req.MatchID,
		AgentAddr:     req.Agent,
		AnswerText:    req.Answer,
		AttemptNumber: req.AttemptNumber,
		NeuronBurned:  req.NeuronBurned,
	}
	if req.Reasoning != "" {
		answerModel.Reasoning = sql.NullString{String: req.Reasoning, Valid: true}
	}
	if req.TxHash != "" {
		answerModel.TxHash = sql.NullString{String: req.TxHash, Valid: true}
	}
	_, err := h.repos.Answers.Create(ctx, answerModel)
	if err != nil {
		log.Printf("Failed to record answer for match %d from %s: %v", req.MatchID, req.Agent, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record answer"})
		return
	}

	// Record burn event for stats tracking — only from Ponder (txHash present) to avoid
	// double-counting since Chief's reporter also calls this endpoint.
	if req.TxHash != "" && req.NeuronBurned != "" && req.NeuronBurned != "0" {
		matchID := req.MatchID
		if err := h.repos.Burns.RecordBurn(ctx, &matchID, req.Agent, req.NeuronBurned, h.repos.Agents); err != nil {
			log.Printf("Failed to record burn for match %d from %s: %v", req.MatchID, req.Agent, err)
		}
	}

	// Forward to Chief for judge evaluation — only when the event comes from Ponder
	// (has txHash). Chief's reporter also calls this endpoint to record answers, but
	// Chief already knows about those — forwarding would create a bounce-back loop.
	if req.TxHash != "" {
		go func() {
			txIndex := fmt.Sprintf("%d", req.LogIndex)
			_, err := h.chiefClient.ReportEvent(context.Background(), chief.EventTypeAnswerSubmitted, req.MatchID, map[string]interface{}{
				"agent":         req.Agent,
				"answer":        req.Answer,
				"attemptNumber": req.AttemptNumber,
				"neuronBurned":  req.NeuronBurned,
				"blockNumber":   0,
				"txHash":        req.TxHash,
				"txIndex":       txIndex,
			})
			if err != nil {
				log.Printf("Failed to forward answer_submitted for match %d to Chief: %v", req.MatchID, err)
			} else {
				log.Printf("Forwarded answer_submitted for match %d to Chief (agent=%s)", req.MatchID, req.Agent)
			}
		}()
	}

	// Broadcast to WebSocket clients
	h.hub.Broadcast(websocket.WSEvent{
		Type: "answer_submitted",
		Data: websocket.AnswerSubmittedData{
			MatchID:       req.MatchID,
			AgentAddr:     req.Agent,
			AnswerText:    req.Answer,
			AttemptNumber: req.AttemptNumber,
			NeuronBurned:  req.NeuronBurned,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}

// AnswerRevealedRequest is the request for recording an answer reveal from chain.
type AnswerRevealedRequest struct {
	MatchID int64  `json:"matchId" binding:"required"`
	Answer  string `json:"answer"`
	Salt    string `json:"salt"`
}

// RecordAnswerRevealed handles POST /internal/answer-revealed
func (h *InternalHandler) RecordAnswerRevealed(c *gin.Context) {
	var req AnswerRevealedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Internal: answer-revealed bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Internal: received answer-revealed for match %d", req.MatchID)

	ctx := c.Request.Context()

	// Persist revealed answer on the match record
	if err := h.repos.Matches.SetRevealedAnswer(ctx, req.MatchID, req.Answer, req.Salt); err != nil {
		log.Printf("Failed to set revealed answer for match %d: %v", req.MatchID, err)
	}

	// Also store as commentary for the feed (deduplicate: Chief reports + Ponder indexes the same reveal)
	existing, _ := h.repos.Commentary.GetByEventType(ctx, req.MatchID, "answer_reveal")
	if len(existing) == 0 {
		_, err := h.repos.Commentary.Create(ctx, &models.CommentaryInput{
			MatchID:   req.MatchID,
			AgentID:   "",
			EventType: "answer_reveal",
			Text:      "Answer revealed: " + req.Answer,
		})
		if err != nil {
			log.Printf("Failed to record answer reveal for match %d: %v", req.MatchID, err)
		}
	}

	// Broadcast to WebSocket clients
	h.hub.Broadcast(websocket.WSEvent{
		Type: "answer_revealed",
		Data: map[string]interface{}{
			"matchId": req.MatchID,
			"answer":  req.Answer,
			"salt":    req.Salt,
		},
	})

	c.JSON(http.StatusOK, gin.H{"status": "revealed"})
}
