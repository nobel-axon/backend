// Package lobby provides matchmaking lobby functionality for pre-payment interest signaling.
package lobby

import (
	"time"
)

// Interest represents a player's interest in joining a match.
type Interest struct {
	AgentAddr     string    `json:"agentAddr"`
	SignaledAt    time.Time `json:"signaledAt"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
}

// ReadyGroup represents a group of players ready to register on-chain.
type ReadyGroup struct {
	GroupID   string    `json:"groupId"`
	Players   []string  `json:"players"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// IsExpired returns true if the ready group has expired.
func (g *ReadyGroup) IsExpired() bool {
	return time.Now().After(g.ExpiresAt)
}

// LobbyStatus represents the current state of the lobby.
type LobbyStatus struct {
	InterestCount int        `json:"interestCount"`
	Players       []string   `json:"players"`
	ReadyGroup    *ReadyGroup `json:"readyGroup,omitempty"`
	MinPlayers    int        `json:"minPlayers"`
	MaxPlayers    int        `json:"maxPlayers"`
}

// ClientMessage represents an incoming WebSocket message from a client.
type ClientMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}
