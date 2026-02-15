// Package models defines data structures for axon-server.
package models

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// sanitizeString strips control characters (except \n, \t) from a string.
func sanitizeString(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// Bounty represents a bounty in the database.
type Bounty struct {
	BountyID        int64          `db:"bounty_id" json:"bountyId"`
	CreatorAddress  string         `db:"creator_address" json:"creatorAddress"`
	QuestionText    string         `db:"question_text" json:"questionText"`
	Category        sql.NullString `db:"category" json:"-"`
	Difficulty      int            `db:"difficulty" json:"difficulty"`
	EntryFee        string         `db:"entry_fee" json:"entryFee"`
	BaseAnswerFee   string         `db:"base_answer_fee" json:"baseAnswerFee"`
	PoolTotal       string         `db:"pool_total" json:"poolTotal"`
	MinRating       string         `db:"min_rating" json:"minRating"`
	MaxParticipants int            `db:"max_participants" json:"maxParticipants"`
	PlayerCount     int            `db:"player_count" json:"playerCount"`
	AnswerCount     int            `db:"answer_count" json:"answerCount"`
	Phase           string         `db:"phase" json:"phase"`
	Deadline        sql.NullTime   `db:"deadline" json:"-"`
	WinnerAddress   sql.NullString `db:"winner_address" json:"-"`
	SettleTxHash    sql.NullString `db:"settle_tx_hash" json:"-"`
	CreatedAt       time.Time      `db:"created_at" json:"createdAt"`
	SettledAt       sql.NullTime   `db:"settled_at" json:"-"`
	ApprovedAt      sql.NullTime   `db:"approved_at" json:"-"`
	RejectedAt      sql.NullTime   `db:"rejected_at" json:"-"`
	RejectionReason sql.NullString `db:"rejection_reason" json:"-"`
	UpdatedAt       time.Time      `db:"updated_at" json:"updatedAt"`
}

// BountyResponse is the JSON response for a bounty.
type BountyResponse struct {
	BountyID        int64   `json:"bountyId"`
	CreatorAddr     string  `json:"creatorAddr"`
	QuestionText    string  `json:"questionText"`
	Category        string  `json:"category,omitempty"`
	Difficulty      int     `json:"difficulty"`
	EntryFee        string  `json:"entryFee"`
	RewardAmount    string  `json:"rewardAmount"`
	MinRating       string  `json:"minRating"`
	MaxParticipants int     `json:"maxParticipants"`
	AgentCount      int     `json:"agentCount"`
	AnswerCount     int     `json:"answerCount"`
	Phase           string  `json:"phase"`
	ExpiresAt       *string `json:"expiresAt,omitempty"`
	WinnerAddr      *string `json:"winnerAddr,omitempty"`
	WinnerAnswer    *string `json:"winnerAnswer,omitempty"`
	BaseAnswerFee   string  `json:"baseAnswerFee,omitempty"`
	SettleTxHash    *string `json:"settleTxHash,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	SettledAt       *string `json:"settledAt,omitempty"`
	ApprovedAt      *string `json:"approvedAt,omitempty"`
	RejectedAt      *string `json:"rejectedAt,omitempty"`
	RejectionReason *string `json:"rejectionReason,omitempty"`
}

// ToResponse converts a Bounty to a BountyResponse.
func (b *Bounty) ToResponse() BountyResponse {
	resp := BountyResponse{
		BountyID:        b.BountyID,
		CreatorAddr:     b.CreatorAddress,
		QuestionText:    sanitizeString(b.QuestionText),
		Difficulty:      b.Difficulty,
		EntryFee:        b.EntryFee,
		RewardAmount:    b.PoolTotal,
		MinRating:       b.MinRating,
		MaxParticipants: b.MaxParticipants,
		AgentCount:      b.PlayerCount,
		AnswerCount:     b.AnswerCount,
		Phase:           b.Phase,
		CreatedAt:       b.CreatedAt.Format(time.RFC3339),
	}
	if b.Category.Valid {
		resp.Category = b.Category.String
	}
	if b.Deadline.Valid {
		s := b.Deadline.Time.Format(time.RFC3339)
		resp.ExpiresAt = &s
	}
	if b.WinnerAddress.Valid {
		resp.WinnerAddr = &b.WinnerAddress.String
	}
	if b.SettleTxHash.Valid {
		resp.SettleTxHash = &b.SettleTxHash.String
	}
	if b.BaseAnswerFee != "" && b.BaseAnswerFee != "0" {
		resp.BaseAnswerFee = b.BaseAnswerFee
	}
	if b.SettledAt.Valid {
		s := b.SettledAt.Time.Format(time.RFC3339)
		resp.SettledAt = &s
	}
	if b.ApprovedAt.Valid {
		s := b.ApprovedAt.Time.Format(time.RFC3339)
		resp.ApprovedAt = &s
	}
	if b.RejectedAt.Valid {
		s := b.RejectedAt.Time.Format(time.RFC3339)
		resp.RejectedAt = &s
	}
	if b.RejectionReason.Valid {
		resp.RejectionReason = &b.RejectionReason.String
	}
	return resp
}

// BountyAnswer represents a bounty answer in the database.
type BountyAnswer struct {
	ID            int64           `db:"id" json:"id"`
	BountyID      int64           `db:"bounty_id" json:"bountyId"`
	AgentAddr     string          `db:"agent_addr" json:"agentAddr"`
	AnswerText    string          `db:"answer_text" json:"answerText"`
	Reasoning     sql.NullString  `db:"reasoning" json:"-"`
	TotalScore    sql.NullInt32   `db:"total_score" json:"-"`
	Agreement     sql.NullString  `db:"agreement" json:"-"`
	Evaluations   json.RawMessage `db:"evaluations" json:"-"`
	TxHash        sql.NullString  `db:"tx_hash" json:"-"`
	AttemptNumber int             `db:"attempt_number" json:"attemptNumber"`
	NeuronBurned  string          `db:"neuron_burned" json:"neuronBurned"`
	SubmittedAt   time.Time       `db:"submitted_at" json:"submittedAt"`
	EvaluatedAt   sql.NullTime    `db:"evaluated_at" json:"-"`
}

// BountyAnswerResponse is the JSON response for a bounty answer.
type BountyAnswerResponse struct {
	ID            int64           `json:"id"`
	BountyID      int64           `json:"bountyId"`
	AgentAddr     string          `json:"agentAddr"`
	AnswerText    string          `json:"answerText"`
	Reasoning     string          `json:"reasoning,omitempty"`
	TotalScore    *int            `json:"totalScore,omitempty"`
	Agreement     string          `json:"agreement,omitempty"`
	Evaluations   json.RawMessage `json:"evaluations,omitempty"`
	AttemptNumber int             `json:"attemptNumber"`
	NeuronBurned  string          `json:"neuronBurned"`
	SubmittedAt   string          `json:"submittedAt"`
	EvaluatedAt   *string         `json:"evaluatedAt,omitempty"`
}

// ToResponse converts a BountyAnswer to BountyAnswerResponse.
func (a *BountyAnswer) ToResponse() BountyAnswerResponse {
	resp := BountyAnswerResponse{
		ID:            a.ID,
		BountyID:      a.BountyID,
		AgentAddr:     a.AgentAddr,
		AnswerText:    a.AnswerText,
		AttemptNumber: a.AttemptNumber,
		NeuronBurned:  a.NeuronBurned,
		SubmittedAt:   a.SubmittedAt.Format(time.RFC3339),
	}
	if a.Reasoning.Valid {
		resp.Reasoning = a.Reasoning.String
	}
	if a.TotalScore.Valid {
		v := int(a.TotalScore.Int32)
		resp.TotalScore = &v
	}
	if a.Agreement.Valid {
		resp.Agreement = a.Agreement.String
	}
	if a.Evaluations != nil {
		resp.Evaluations = a.Evaluations
	}
	if a.EvaluatedAt.Valid {
		s := a.EvaluatedAt.Time.Format(time.RFC3339)
		resp.EvaluatedAt = &s
	}
	return resp
}

// BountyStatsResponse is the JSON response for bounty stats.
type BountyStatsResponse struct {
	TotalBounties   int    `json:"totalBounties"`
	ActiveBounties  int    `json:"activeBounties"`
	SettledBounties int    `json:"settledBounties"`
	TotalRewardPool string `json:"totalRewardPool"`
	AvgReward       string `json:"avgReward"`
}

// ReputationResponse is the JSON response for agent reputation.
type ReputationResponse struct {
	AgentAddr             string `json:"agentAddr"`
	ReputationScore       int    `json:"reputationScore"`
	ReputationFeedbackCnt int    `json:"reputationFeedbackCount"`
	ERC8004Registered     bool   `json:"erc8004Registered"`
	ERC8004AgentID        int64  `json:"erc8004AgentId,omitempty"`
	BountiesPlayed        int    `json:"bountiesPlayed"`
	BountiesWon           int    `json:"bountiesWon"`
}
