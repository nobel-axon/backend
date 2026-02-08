// Package handlers provides HTTP request handlers for the axon-server API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/lobby"
)

// LobbyHandler handles lobby-related HTTP requests.
type LobbyHandler struct {
	manager *lobby.Manager
}

// NewLobbyHandler creates a new lobby handler.
func NewLobbyHandler(manager *lobby.Manager) *LobbyHandler {
	return &LobbyHandler{
		manager: manager,
	}
}

// GetLobbyStatus returns the current lobby status.
// GET /api/lobby
func (h *LobbyHandler) GetLobbyStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// SignalInterest adds a player to the interest pool.
// POST /api/lobby/interest
func (h *LobbyHandler) SignalInterest(c *gin.Context) {
	var req struct {
		AgentAddr string `json:"agentAddr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agentAddr is required"})
		return
	}

	h.manager.SignalInterest(req.AgentAddr)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Interest signaled",
	})
}

// LeaveInterest removes a player from the interest pool.
// DELETE /api/lobby/interest/:address
func (h *LobbyHandler) LeaveInterest(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address is required"})
		return
	}

	h.manager.LeaveInterest(address)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Left interest pool",
	})
}

// Heartbeat keeps the interest alive.
// POST /api/lobby/heartbeat
func (h *LobbyHandler) Heartbeat(c *gin.Context) {
	var req struct {
		AgentAddr string `json:"agentAddr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agentAddr is required"})
		return
	}

	h.manager.Heartbeat(req.AgentAddr)

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// GetReadyGroup returns the status of a ready group.
// GET /api/lobby/ready/:groupId
func (h *LobbyHandler) GetReadyGroup(c *gin.Context) {
	groupID := c.Param("groupId")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "groupId is required"})
		return
	}

	group := h.manager.GetReadyGroup(groupID)
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ready group not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groupId":   group.GroupID,
		"players":   group.Players,
		"expiresAt": group.ExpiresAt.Unix(),
		"expired":   group.IsExpired(),
	})
}
