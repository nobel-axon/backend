// Package models defines data structures for axon-server.
package models

import (
	"time"
)

// GlobalStats represents overall platform statistics.
type GlobalStats struct {
	TotalMatches    int    `json:"totalMatches"`
	ActiveMatches   int    `json:"activeMatches"`
	SettledMatches  int    `json:"settledMatches"`
	TotalAgents     int    `json:"totalAgents"`
	TotalBurned     string `json:"totalBurned"` // Total NEURON burned
	TotalPoolVolume string `json:"totalPoolVolume"` // Total MON in pools
	Last24hMatches  int    `json:"last24hMatches"`
	Last24hBurned   string `json:"last24hBurned"`
}

// BurnStats represents NEURON burn data for analytics.
type BurnStats struct {
	ID           int64     `db:"id" json:"id"`
	MatchID      *int64    `db:"match_id" json:"matchId,omitempty"`
	AgentAddr    string    `db:"agent_addr" json:"agentAddr"`
	AmountBurned string    `db:"amount_burned" json:"amountBurned"`
	RecordedAt   time.Time `db:"recorded_at" json:"recordedAt"`
}

// BurnStatsResponse is the JSON response for burn stats.
type BurnStatsResponse struct {
	ID           int64   `json:"id"`
	MatchID      *int64  `json:"matchId,omitempty"`
	AgentAddr    string  `json:"agentAddr"`
	AmountBurned string  `json:"amountBurned"`
	RecordedAt   string  `json:"recordedAt"`
}

// ToResponse converts BurnStats to BurnStatsResponse.
func (b *BurnStats) ToResponse() BurnStatsResponse {
	return BurnStatsResponse{
		ID:           b.ID,
		MatchID:      b.MatchID,
		AgentAddr:    b.AgentAddr,
		AmountBurned: b.AmountBurned,
		RecordedAt:   b.RecordedAt.Format(time.RFC3339),
	}
}

// BurnTimeline represents aggregated burn data over time.
type BurnTimeline struct {
	Timestamp    string `db:"timestamp" json:"timestamp"`
	TotalBurned  string `db:"total_burned" json:"totalBurned"`
	BurnCount    int    `db:"burn_count" json:"burnCount"`
	UniqueAgents int    `db:"unique_agents" json:"uniqueAgents"`
}

// PollerState represents the state of the poller for resuming.
type PollerState struct {
	Key       string    `db:"key" json:"key"`
	Value     string    `db:"value" json:"value"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}
