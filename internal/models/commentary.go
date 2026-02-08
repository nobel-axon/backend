// Package models defines data structures for axon-server.
package models

import (
	"database/sql"
	"time"
)

// Commentary represents judge commentary for a match.
type Commentary struct {
	ID        int64          `db:"id" json:"id"`
	MatchID   int64          `db:"match_id" json:"matchId"`
	AgentID   sql.NullString `db:"agent_id" json:"-"`
	EventType string         `db:"event_type" json:"eventType"`
	Text      string         `db:"text" json:"text"`
	CreatedAt time.Time      `db:"created_at" json:"createdAt"`
}

// CommentaryResponse is the JSON response for commentary.
type CommentaryResponse struct {
	ID        int64   `json:"id"`
	MatchID   int64   `json:"matchId"`
	AgentID   *string `json:"agentId,omitempty"`
	EventType string  `json:"eventType"`
	Text      string  `json:"text"`
	CreatedAt string  `json:"createdAt"`
}

// ToResponse converts Commentary to CommentaryResponse.
func (c *Commentary) ToResponse() CommentaryResponse {
	resp := CommentaryResponse{
		ID:        c.ID,
		MatchID:   c.MatchID,
		EventType: c.EventType,
		Text:      c.Text,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}

	if c.AgentID.Valid {
		s := c.AgentID.String
		resp.AgentID = &s
	}

	return resp
}

// CommentaryInput is the input for creating commentary.
type CommentaryInput struct {
	MatchID   int64  `json:"matchId" binding:"required"`
	AgentID   string `json:"agentId"`
	EventType string `json:"eventType" binding:"required"`
	Text      string `json:"text" binding:"required"`
}
