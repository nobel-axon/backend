// Package models defines data structures for axon-server.
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Answer represents an answer submission stored in the database.
type Answer struct {
	ID            int64          `db:"id" json:"id"`
	MatchID       int64          `db:"match_id" json:"matchId"`
	AgentAddr     string         `db:"agent_addr" json:"agentAddr"`
	AnswerText    string         `db:"answer_text" json:"answerText"`
	IsCorrect     sql.NullBool   `db:"is_correct" json:"-"`
	Consensus     sql.NullString `db:"consensus" json:"-"`
	Confidence    sql.NullFloat64 `db:"confidence" json:"-"`
	BlockNumber   int64          `db:"block_number" json:"blockNumber"`
	TxIndex       int            `db:"tx_index" json:"txIndex"`
	AttemptNumber int            `db:"attempt_number" json:"attemptNumber"`
	NeuronBurned  string         `db:"neuron_burned" json:"neuronBurned"`
	TxHash        sql.NullString `db:"tx_hash" json:"-"`
	TotalScore    sql.NullInt32   `db:"total_score" json:"-"`
	Agreement     sql.NullString  `db:"agreement" json:"-"`
	Reasoning     sql.NullString  `db:"reasoning" json:"-"`
	Evaluations   json.RawMessage `db:"evaluations" json:"-"`
	SubmittedAt   time.Time       `db:"submitted_at" json:"submittedAt"`
	VerifiedAt    sql.NullTime    `db:"verified_at" json:"-"`
}

// AnswerResponse is the JSON response for an answer.
type AnswerResponse struct {
	ID            int64    `json:"id"`
	MatchID       int64    `json:"matchId"`
	AgentAddr     string   `json:"agentAddr"`
	AnswerText    string   `json:"answerText"`
	IsCorrect     *bool    `json:"isCorrect,omitempty"`
	Consensus     *string  `json:"consensus,omitempty"`
	Confidence    *float64 `json:"confidence,omitempty"`
	BlockNumber   int64    `json:"blockNumber"`
	TxIndex       int      `json:"txIndex"`
	AttemptNumber int      `json:"attemptNumber"`
	NeuronBurned  string          `json:"neuronBurned"`
	TxHash        *string         `json:"txHash,omitempty"`
	TotalScore    *int            `json:"totalScore,omitempty"`
	Agreement     *string         `json:"agreement,omitempty"`
	Reasoning     *string         `json:"reasoning,omitempty"`
	Evaluations   json.RawMessage `json:"evaluations,omitempty"`
	SubmittedAt   string          `json:"submittedAt"`
	VerifiedAt    *string         `json:"verifiedAt,omitempty"`
}

// ToResponse converts an Answer to AnswerResponse.
func (a *Answer) ToResponse() AnswerResponse {
	resp := AnswerResponse{
		ID:            a.ID,
		MatchID:       a.MatchID,
		AgentAddr:     a.AgentAddr,
		AnswerText:    a.AnswerText,
		BlockNumber:   a.BlockNumber,
		TxIndex:       a.TxIndex,
		AttemptNumber: a.AttemptNumber,
		NeuronBurned:  a.NeuronBurned,
		SubmittedAt:   a.SubmittedAt.Format(time.RFC3339),
	}

	if a.TxHash.Valid {
		s := a.TxHash.String
		resp.TxHash = &s
	}
	if a.IsCorrect.Valid {
		v := a.IsCorrect.Bool
		resp.IsCorrect = &v
	}
	if a.Consensus.Valid {
		s := a.Consensus.String
		resp.Consensus = &s
	}
	if a.Confidence.Valid {
		v := a.Confidence.Float64
		resp.Confidence = &v
	}
	if a.TotalScore.Valid {
		v := int(a.TotalScore.Int32)
		resp.TotalScore = &v
	}
	if a.Agreement.Valid {
		s := a.Agreement.String
		resp.Agreement = &s
	}
	if a.Reasoning.Valid {
		s := a.Reasoning.String
		resp.Reasoning = &s
	}
	if len(a.Evaluations) > 0 {
		resp.Evaluations = a.Evaluations
	}
	if a.VerifiedAt.Valid {
		s := a.VerifiedAt.Time.Format(time.RFC3339)
		resp.VerifiedAt = &s
	}

	return resp
}

// AnswerResult represents the result from Chief after verification.
type AnswerResult struct {
	MatchID    int64   `json:"matchId"`
	AgentAddr  string  `json:"agentAddr"`
	IsCorrect  bool    `json:"isCorrect"`
	Consensus  string  `json:"consensus"`
	Confidence float64 `json:"confidence"`
}
