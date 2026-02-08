// Package models defines data structures for axon-server.
package models

import (
	"database/sql"
	"time"
)

// MatchPhase represents the current phase of a match.
type MatchPhase string

const (
	PhaseCreated        MatchPhase = "created"
	PhaseRegistration   MatchPhase = "registration"
	PhaseQuestionLive   MatchPhase = "question_live"
	PhaseSettling       MatchPhase = "settling"
	PhaseSettled        MatchPhase = "settled"
	PhaseCancelled      MatchPhase = "cancelled"
)

// Match represents a match stored in the database.
type Match struct {
	MatchID          int64          `db:"match_id" json:"matchId"`
	Phase            string         `db:"phase" json:"phase"`
	EntryFee         string         `db:"entry_fee" json:"entryFee"`
	AnswerFee        string         `db:"answer_fee" json:"answerFee"`
	PoolTotal        string         `db:"pool_total" json:"poolTotal"`
	PlayerCount      int            `db:"player_count" json:"playerCount"`
	QuestionText     sql.NullString `db:"question_text" json:"-"`
	Category         sql.NullString `db:"category" json:"-"`
	Difficulty       sql.NullInt32  `db:"difficulty" json:"-"`
	FormatHint       sql.NullString `db:"format_hint" json:"-"`
	AnswerHash       sql.NullString `db:"answer_hash" json:"-"`
	WinnerAddress    sql.NullString `db:"winner_address" json:"-"`
	GeneratorAgent   sql.NullString `db:"generator_agent" json:"-"`
	RegistrationEnd  sql.NullTime   `db:"registration_end" json:"-"`
	AnswerTimeout    sql.NullTime   `db:"answer_timeout" json:"-"`
	RevealedAnswer         sql.NullString `db:"revealed_answer" json:"-"`
	RevealedSalt           sql.NullString `db:"revealed_salt" json:"-"`
	CreatedAt              time.Time      `db:"created_at" json:"createdAt"`
	SettledAt              sql.NullTime   `db:"settled_at" json:"-"`
	UpdatedAt              time.Time      `db:"updated_at" json:"updatedAt"`
	RegistrationReportedAt sql.NullTime   `db:"registration_reported_at" json:"-"`
	TimeoutReportedAt      sql.NullTime   `db:"timeout_reported_at" json:"-"`
}

// MatchResponse is the JSON response for a match.
type MatchResponse struct {
	MatchID         int64   `json:"matchId"`
	Phase           string  `json:"phase"`
	EntryFee        string  `json:"entryFee"`
	AnswerFee       string  `json:"answerFee"`
	PoolTotal       string  `json:"poolTotal"`
	PlayerCount     int     `json:"playerCount"`
	QuestionText    *string `json:"questionText,omitempty"`
	Category        *string `json:"category,omitempty"`
	Difficulty      *int    `json:"difficulty,omitempty"`
	FormatHint      *string `json:"formatHint,omitempty"`
	AnswerHash      *string `json:"answerHash,omitempty"`
	WinnerAddress   *string `json:"winnerAddress,omitempty"`
	GeneratorAgent  *string `json:"generatorAgent,omitempty"`
	RegistrationEnd *string `json:"registrationEnd,omitempty"`
	AnswerTimeout   *string `json:"answerTimeout,omitempty"`
	RevealedAnswer   *string `json:"revealedAnswer,omitempty"`
	RevealedSalt     *string `json:"revealedSalt,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	SettledAt        *string `json:"settledAt,omitempty"`
	AnswerCount      int     `json:"answerCount"`
	CommentaryCount  int     `json:"commentaryCount"`
}

// ToResponse converts a Match to MatchResponse.
func (m *Match) ToResponse() MatchResponse {
	resp := MatchResponse{
		MatchID:     m.MatchID,
		Phase:       m.Phase,
		EntryFee:    m.EntryFee,
		AnswerFee:   m.AnswerFee,
		PoolTotal:   m.PoolTotal,
		PlayerCount: m.PlayerCount,
		CreatedAt:   m.CreatedAt.Format(time.RFC3339),
	}

	if m.QuestionText.Valid {
		s := m.QuestionText.String
		resp.QuestionText = &s
	}
	if m.Category.Valid {
		s := m.Category.String
		resp.Category = &s
	}
	if m.Difficulty.Valid {
		d := int(m.Difficulty.Int32)
		resp.Difficulty = &d
	}
	if m.FormatHint.Valid {
		s := m.FormatHint.String
		resp.FormatHint = &s
	}
	if m.AnswerHash.Valid {
		s := m.AnswerHash.String
		resp.AnswerHash = &s
	}
	if m.WinnerAddress.Valid {
		s := m.WinnerAddress.String
		resp.WinnerAddress = &s
	}
	if m.GeneratorAgent.Valid {
		s := m.GeneratorAgent.String
		resp.GeneratorAgent = &s
	}
	if m.RegistrationEnd.Valid {
		s := m.RegistrationEnd.Time.Format(time.RFC3339)
		resp.RegistrationEnd = &s
	}
	if m.AnswerTimeout.Valid {
		s := m.AnswerTimeout.Time.Format(time.RFC3339)
		resp.AnswerTimeout = &s
	}
	if m.RevealedAnswer.Valid {
		s := m.RevealedAnswer.String
		resp.RevealedAnswer = &s
	}
	if m.RevealedSalt.Valid {
		s := m.RevealedSalt.String
		resp.RevealedSalt = &s
	}
	if m.SettledAt.Valid {
		s := m.SettledAt.Time.Format(time.RFC3339)
		resp.SettledAt = &s
	}

	return resp
}

// MatchPlayer represents a player registered for a match.
type MatchPlayer struct {
	MatchID      int64     `db:"match_id" json:"matchId"`
	AgentAddr    string    `db:"agent_addr" json:"agentAddr"`
	RegisteredAt time.Time `db:"registered_at" json:"registeredAt"`
}

// MatchListParams represents parameters for listing matches.
type MatchListParams struct {
	Phase  string
	Limit  int
	Offset int
}
