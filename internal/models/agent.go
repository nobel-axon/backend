// Package models defines data structures for axon-server.
package models

import (
	"database/sql"
	"time"
)

// AgentStats represents aggregated statistics for an agent.
type AgentStats struct {
	AgentAddr         string       `db:"agent_addr" json:"agentAddr"`
	MatchesPlayed     int          `db:"matches_played" json:"matchesPlayed"`
	MatchesWon        int          `db:"matches_won" json:"matchesWon"`
	TotalEarnedMON    string       `db:"total_earned_mon" json:"totalEarnedMon"`
	TotalEarnedNeuron string       `db:"total_earned_neuron" json:"totalEarnedNeuron"`
	TotalBurnedNeuron string       `db:"total_burned_neuron" json:"totalBurnedNeuron"`
	WrongAnswers      int          `db:"wrong_answers" json:"wrongAnswers"`
	CorrectAnswers    int          `db:"correct_answers" json:"correctAnswers"`
	AvgAnswerTimeMs        sql.NullInt64 `db:"avg_answer_time_ms" json:"-"`
	LastActive             sql.NullTime  `db:"last_active" json:"-"`
	FirstSeen              time.Time     `db:"first_seen" json:"firstSeen"`
	ReputationScore        int           `db:"reputation_score" json:"-"`
	ReputationFeedbackCnt  int           `db:"reputation_feedback_count" json:"-"`
	ERC8004Registered      bool          `db:"erc8004_registered" json:"-"`
	ERC8004AgentID         sql.NullInt64 `db:"erc8004_agent_id" json:"-"`
	BountiesPlayed         int           `db:"bounties_played" json:"-"`
	BountiesWon            int           `db:"bounties_won" json:"-"`
}

// AgentStatsResponse is the JSON response for agent stats.
type AgentStatsResponse struct {
	AgentAddr         string  `json:"agentAddr"`
	MatchesPlayed     int     `json:"matchesPlayed"`
	MatchesWon        int     `json:"matchesWon"`
	WinRate           float64 `json:"winRate"`
	TotalEarnedMON    string  `json:"totalEarnedMon"`
	TotalEarnedNeuron string  `json:"totalEarnedNeuron"`
	TotalBurnedNeuron string  `json:"totalBurnedNeuron"`
	WrongAnswers      int     `json:"wrongAnswers"`
	CorrectAnswers    int     `json:"correctAnswers"`
	AnswerAccuracy    float64 `json:"answerAccuracy"`
	AvgAnswerTimeMs   *int64  `json:"avgAnswerTimeMs,omitempty"`
	LastActive        *string `json:"lastActive,omitempty"`
	FirstSeen         string  `json:"firstSeen"`
	ReputationScore   *int    `json:"reputationScore,omitempty"`
	ERC8004Rating     *int    `json:"erc8004Rating,omitempty"`
}

// ToResponse converts AgentStats to AgentStatsResponse.
func (a *AgentStats) ToResponse() AgentStatsResponse {
	resp := AgentStatsResponse{
		AgentAddr:         a.AgentAddr,
		MatchesPlayed:     a.MatchesPlayed,
		MatchesWon:        a.MatchesWon,
		TotalEarnedMON:    a.TotalEarnedMON,
		TotalEarnedNeuron: a.TotalEarnedNeuron,
		TotalBurnedNeuron: a.TotalBurnedNeuron,
		WrongAnswers:      a.WrongAnswers,
		CorrectAnswers:    a.CorrectAnswers,
		FirstSeen:         a.FirstSeen.Format(time.RFC3339),
	}

	// Calculate win rate
	if a.MatchesPlayed > 0 {
		resp.WinRate = float64(a.MatchesWon) / float64(a.MatchesPlayed)
	}

	// Calculate answer accuracy
	totalAnswers := a.CorrectAnswers + a.WrongAnswers
	if totalAnswers > 0 {
		resp.AnswerAccuracy = float64(a.CorrectAnswers) / float64(totalAnswers)
	}

	if a.AvgAnswerTimeMs.Valid {
		v := a.AvgAnswerTimeMs.Int64
		resp.AvgAnswerTimeMs = &v
	}
	if a.LastActive.Valid {
		s := a.LastActive.Time.Format(time.RFC3339)
		resp.LastActive = &s
	}

	if a.ReputationScore > 0 {
		s := a.ReputationScore
		resp.ReputationScore = &s
	}
	if a.ERC8004Registered && a.ReputationScore > 0 {
		s := a.ReputationScore
		resp.ERC8004Rating = &s
	}

	return resp
}

// AgentProfile combines stats with recent match history.
type AgentProfile struct {
	Stats         AgentStatsResponse `json:"stats"`
	RecentMatches []MatchResponse    `json:"recentMatches"`
}

// LeaderboardEntry represents an entry in the leaderboard.
type LeaderboardEntry struct {
	Rank              int     `json:"rank"`
	AgentAddr         string  `json:"agentAddr"`
	MatchesWon        int     `json:"matchesWon"`
	MatchesPlayed     int     `json:"matchesPlayed"`
	WinRate           float64 `json:"winRate"`
	TotalEarnedMON    string  `json:"totalEarnedMon"`
	TotalBurnedNeuron string  `json:"totalBurnedNeuron"`
	ReputationScore   int     `json:"reputationScore,omitempty"`
}

// AgentEconomics represents the economic metrics for an agent.
type AgentEconomics struct {
	AgentAddr            string  `json:"agentAddr"`
	NeuronBalance        string  `json:"neuronBalance"`
	TotalSpent           string  `json:"totalSpent"`
	TotalEarned          string  `json:"totalEarned"`
	NetPnl               string  `json:"netPnl"`
	MatchRoi             float64 `json:"matchRoi"`
	BountyRoi            float64 `json:"bountyRoi"`
	BountiesParticipated int     `json:"bountiesParticipated"`
	BountiesWon          int     `json:"bountiesWon"`
}

// LeaderboardParams represents parameters for the leaderboard query.
type LeaderboardParams struct {
	SortBy string // "wins", "earnings", "accuracy", "burned"
	Limit  int
	Offset int
}
