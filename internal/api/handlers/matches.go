// Package handlers provides HTTP handlers for the API.
package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/chief"
	"github.com/axon-arena/axon-server/internal/models"
	"github.com/axon-arena/axon-server/internal/repository"
)

const defaultDBTimeout = 10 * time.Second

// MatchHandler handles match-related API requests.
type MatchHandler struct {
	repos       *repository.Repositories
	chiefClient *chief.Client
}

// NewMatchHandler creates a new match handler.
func NewMatchHandler(repos *repository.Repositories, chiefClient *chief.Client) *MatchHandler {
	return &MatchHandler{
		repos:       repos,
		chiefClient: chiefClient,
	}
}

// ListMatches handles GET /api/matches
func (h *MatchHandler) ListMatches(c *gin.Context) {
	params := models.MatchListParams{
		Phase:  c.Query("phase"),
		Limit:  50,
		Offset: 0,
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	matches, err := h.repos.Matches.List(ctx, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch matches"})
		return
	}

	// Get actual total count for pagination
	total, err := h.repos.Matches.CountWithFilter(ctx, params.Phase)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count matches"})
		return
	}

	response := make([]models.MatchResponse, len(matches))
	matchIDs := make([]int64, len(matches))
	for i, m := range matches {
		response[i] = m.ToResponse()
		matchIDs[i] = m.MatchID
	}

	// Batch fetch answer and commentary counts
	if len(matchIDs) > 0 {
		answerCounts, err := h.repos.Answers.CountByMatches(ctx, matchIDs)
		if err != nil {
			log.Printf("Failed to fetch answer counts: %v", err)
		} else {
			for i := range response {
				response[i].AnswerCount = answerCounts[response[i].MatchID]
			}
		}

		commentaryCounts, err := h.repos.Commentary.CountByMatches(ctx, matchIDs)
		if err != nil {
			log.Printf("Failed to fetch commentary counts: %v", err)
		} else {
			for i := range response {
				response[i].CommentaryCount = commentaryCounts[response[i].MatchID]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"matches": response,
		"total":   total,
		"limit":   params.Limit,
		"offset":  params.Offset,
	})
}

// GetMatch handles GET /api/matches/:id
func (h *MatchHandler) GetMatch(c *gin.Context) {
	matchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	match, err := h.repos.Matches.GetByID(ctx, matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match"})
		return
	}
	if match == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	resp := match.ToResponse()

	// Include personalities if available
	mp, err := h.repos.Personalities.GetByMatch(ctx, matchID)
	if err != nil {
		log.Printf("Failed to fetch personalities for match %d: %v", matchID, err)
	}

	if mp != nil {
		c.JSON(http.StatusOK, gin.H{
			"match":         resp,
			"personalities": mp.Personalities,
			"judgePanel":    mp.JudgePanel,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"match": resp})
}

// GetMatchPersonalities handles GET /api/matches/:id/personalities
func (h *MatchHandler) GetMatchPersonalities(c *gin.Context) {
	matchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	mp, err := h.repos.Personalities.GetByMatch(ctx, matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch personalities"})
		return
	}
	if mp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Personalities not found for this match"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"matchId":       mp.MatchID,
		"personalities": mp.Personalities,
		"judgePanel":    mp.JudgePanel,
	})
}

// GetOpenMatches handles GET /api/matches/open and GET /api/matches/queue
func (h *MatchHandler) GetOpenMatches(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	matches, err := h.repos.Matches.GetOpenMatches(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch open matches"})
		return
	}

	response := make([]models.MatchResponse, len(matches))
	for i, m := range matches {
		response[i] = m.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"matches": response})
}

// GetLiveMatches handles GET /api/matches/live
func (h *MatchHandler) GetLiveMatches(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	matches, err := h.repos.Matches.GetLiveMatches(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch live matches"})
		return
	}

	response := make([]models.MatchResponse, len(matches))
	for i, m := range matches {
		response[i] = m.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"matches": response})
}

// GetMatchAnswers handles GET /api/matches/:id/answers
func (h *MatchHandler) GetMatchAnswers(c *gin.Context) {
	matchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	// Check match exists
	match, err := h.repos.Matches.GetByID(c.Request.Context(), matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match"})
		return
	}
	if match == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	answers, err := h.repos.Answers.GetByMatch(c.Request.Context(), matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch answers"})
		return
	}

	response := make([]models.AnswerResponse, len(answers))
	for i, a := range answers {
		response[i] = a.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"answers": response})
}

// GetMatchCommentary handles GET /api/matches/:id/commentary
func (h *MatchHandler) GetMatchCommentary(c *gin.Context) {
	matchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	// Check match exists
	match, err := h.repos.Matches.GetByID(c.Request.Context(), matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match"})
		return
	}
	if match == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	commentary, err := h.repos.Commentary.GetByMatch(c.Request.Context(), matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch commentary"})
		return
	}

	response := make([]models.CommentaryResponse, len(commentary))
	for i, cm := range commentary {
		response[i] = cm.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"commentary": response})
}

// GetMatchPlayers handles GET /api/matches/:id/players
func (h *MatchHandler) GetMatchPlayers(c *gin.Context) {
	matchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	// Check match exists
	match, err := h.repos.Matches.GetByID(ctx, matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match"})
		return
	}
	if match == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	players, err := h.repos.Matches.GetPlayers(ctx, matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch players"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"players": players})
}

// GetCategories handles GET /api/categories
func (h *MatchHandler) GetCategories(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	categories, err := h.repos.Matches.GetCategories(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// GetMatchState handles GET /api/matches/:id/state
// Proxies to Chief for live matches, falls back to DB for settled/cancelled matches.
func (h *MatchHandler) GetMatchState(c *gin.Context) {
	matchID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid match ID"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	// Check if match is settled/cancelled — serve from DB
	match, err := h.repos.Matches.GetByID(ctx, matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch match"})
		return
	}
	if match == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Match not found"})
		return
	}

	if match.Phase == string(models.PhaseSettled) || match.Phase == string(models.PhaseCancelled) {
		// Serve from DB for completed matches
		resp := match.ToResponse()

		result := gin.H{"match": resp}

		// Include personalities
		mp, err := h.repos.Personalities.GetByMatch(ctx, matchID)
		if err == nil && mp != nil {
			result["personalities"] = mp.Personalities
			result["judgePanel"] = mp.JudgePanel
		}

		// Include answers with evaluations
		answers, err := h.repos.Answers.GetByMatch(ctx, matchID)
		if err == nil {
			answerResponses := make([]models.AnswerResponse, len(answers))
			for i, a := range answers {
				answerResponses[i] = a.ToResponse()
			}
			result["answers"] = answerResponses
		}

		c.JSON(http.StatusOK, result)
		return
	}

	// For live matches, proxy to Chief
	if h.chiefClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Chief service not available"})
		return
	}

	state, err := h.chiefClient.GetMatchState(ctx, matchID)
	if err != nil {
		log.Printf("Failed to get match state from Chief: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to get match state from Chief"})
		return
	}

	c.JSON(http.StatusOK, state)
}

// GetFeed handles GET /api/feed — cursor-paginated settled match feed.
func (h *MatchHandler) GetFeed(c *gin.Context) {
	// Parse limit
	limit := 10
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 50 {
		limit = 50
	}

	// Parse cursor
	var cursorTime *time.Time
	var cursorID *int64
	if cursor := c.Query("cursor"); cursor != "" {
		t, id, err := parseFeedCursor(cursor)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cursor"})
			return
		}
		cursorTime = &t
		cursorID = &id
	}

	category := c.Query("category")

	ctx, cancel := context.WithTimeout(c.Request.Context(), defaultDBTimeout)
	defer cancel()

	// Fetch limit+1 to determine hasMore
	matches, err := h.repos.Matches.ListSettledForFeed(ctx, limit+1, cursorTime, cursorID, category)
	if err != nil {
		log.Printf("Feed query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feed"})
		return
	}

	hasMore := len(matches) > limit
	if hasMore {
		matches = matches[:limit]
	}

	if len(matches) == 0 {
		c.JSON(http.StatusOK, models.FeedResponse{Items: []models.FeedItem{}, HasMore: false})
		return
	}

	// Collect match IDs
	matchIDs := make([]int64, len(matches))
	for i, m := range matches {
		matchIDs[i] = m.MatchID
	}

	// Batch fetch all related data
	answersMap, err := h.repos.Answers.GetByMatches(ctx, matchIDs)
	if err != nil {
		log.Printf("Feed answers error: %v", err)
		answersMap = map[int64][]models.Answer{}
	}

	personalitiesMap, err := h.repos.Personalities.GetByMatches(ctx, matchIDs)
	if err != nil {
		log.Printf("Feed personalities error: %v", err)
		personalitiesMap = map[int64]*models.MatchPersonalities{}
	}

	playersMap, err := h.repos.Matches.GetPlayersByMatches(ctx, matchIDs)
	if err != nil {
		log.Printf("Feed players error: %v", err)
		playersMap = map[int64][]models.MatchPlayer{}
	}

	commentaryMap, err := h.repos.Commentary.GetByMatchesCapped(ctx, matchIDs, 10)
	if err != nil {
		log.Printf("Feed commentary error: %v", err)
		commentaryMap = map[int64][]models.Commentary{}
	}

	// Assemble feed items
	items := make([]models.FeedItem, len(matches))
	for i, m := range matches {
		item := models.FeedItem{
			Match:   m.ToResponse(),
			Players: playersMap[m.MatchID],
		}
		if item.Players == nil {
			item.Players = []models.MatchPlayer{}
		}

		// Personalities
		if mp, ok := personalitiesMap[m.MatchID]; ok {
			item.Personalities = mp.Personalities
			item.JudgePanel = mp.JudgePanel
		}

		// Answers
		if answers, ok := answersMap[m.MatchID]; ok {
			item.Answers = make([]models.AnswerResponse, len(answers))
			for j, a := range answers {
				item.Answers[j] = a.ToResponse()
			}
		} else {
			item.Answers = []models.AnswerResponse{}
		}

		// Commentary
		if comms, ok := commentaryMap[m.MatchID]; ok {
			item.Commentary = make([]models.CommentaryResponse, len(comms))
			for j, cm := range comms {
				item.Commentary[j] = cm.ToResponse()
			}
		} else {
			item.Commentary = []models.CommentaryResponse{}
		}

		items[i] = item
	}

	// Build next cursor from last item
	var nextCursor string
	if hasMore {
		last := matches[len(matches)-1]
		if last.SettledAt.Valid {
			nextCursor = encodeFeedCursor(last.SettledAt.Time, last.MatchID)
		}
	}

	c.JSON(http.StatusOK, models.FeedResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	})
}

// encodeFeedCursor encodes a compound cursor as "unixNano:matchID".
func encodeFeedCursor(settledAt time.Time, matchID int64) string {
	return fmt.Sprintf("%d:%d", settledAt.UnixNano(), matchID)
}

// parseFeedCursor decodes a compound cursor from "unixNano:matchID".
func parseFeedCursor(cursor string) (time.Time, int64, error) {
	parts := strings.SplitN(cursor, ":", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor time: %w", err)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor id: %w", err)
	}
	return time.Unix(0, nanos), id, nil
}
