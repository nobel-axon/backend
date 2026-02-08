// Package websocket provides WebSocket server functionality for axon-server.
package websocket

import "encoding/json"

// WSEvent represents a WebSocket event to broadcast.
type WSEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ToJSON serializes the event to JSON bytes.
func (e *WSEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// Event types
const (
	EventMatchCreated     = "match_created"
	EventAgentRegistered  = "agent_registered"
	EventQuestionPosted   = "question_posted"
	EventAnswerSubmitted  = "answer_submitted"
	EventAnswerVerified   = "answer_verified"
	EventMatchSettled     = "match_settled"
	EventCommentary       = "commentary"
	EventMatchTimeout     = "match_timeout"
	EventMatchCancelled   = "match_cancelled"

	// Lobby events (server → client)
	EventLobbyInterestUpdate = "lobby_interest_update"
	EventLobbyReady          = "lobby_ready"
	EventLobbyGroupExpired   = "lobby_group_expired"

	// Lobby events (client → server via WS message)
	EventLobbyJoin      = "lobby_join"
	EventLobbyLeave     = "lobby_leave"
	EventLobbyHeartbeat = "lobby_heartbeat"
)

// MatchCreatedData is the data for match_created events.
type MatchCreatedData struct {
	MatchID         int64  `json:"matchId"`
	EntryFee        string `json:"entryFee"`
	AnswerFee       string `json:"answerFee"`
	RegistrationEnd string `json:"registrationEnd"`
}

// AgentRegisteredData is the data for agent_registered events.
type AgentRegisteredData struct {
	MatchID     int64  `json:"matchId"`
	AgentAddr   string `json:"agentAddr"`
	PlayerCount int    `json:"playerCount"`
}

// QuestionPostedData is the data for question_posted events.
type QuestionPostedData struct {
	MatchID      int64  `json:"matchId"`
	QuestionText string `json:"questionText"`
	Category     string `json:"category"`
	Difficulty   int    `json:"difficulty"`
	FormatHint   string `json:"formatHint"`
	AnswerHash   string `json:"answerHash"`
	Deadline     string `json:"deadline"`
}

// AnswerSubmittedData is the data for answer_submitted events.
type AnswerSubmittedData struct {
	MatchID       int64  `json:"matchId"`
	AgentAddr     string `json:"agentAddr"`
	AnswerText    string `json:"answerText"`
	AttemptNumber int    `json:"attemptNumber"`
	NeuronBurned  string `json:"neuronBurned"`
}

// AnswerVerifiedData is the data for answer_verified events.
type AnswerVerifiedData struct {
	MatchID       int64   `json:"matchId"`
	AgentAddr     string  `json:"agentAddr"`
	AttemptNumber int     `json:"attemptNumber"`
	IsCorrect     bool    `json:"isCorrect"`
	Consensus     string  `json:"consensus"`
	Confidence    float64 `json:"confidence"`
}

// MatchSettledData is the data for match_settled events.
type MatchSettledData struct {
	MatchID     int64  `json:"matchId"`
	WinnerAddr  string `json:"winnerAddr"`
	PrizeMON    string `json:"prizeMon"`
	PrizeNeuron string `json:"prizeNeuron"`
}

// CommentaryData is the data for commentary events.
type CommentaryData struct {
	MatchID   int64  `json:"matchId"`
	AgentID   string `json:"agentId,omitempty"`
	EventType string `json:"eventType"`
	Text      string `json:"text"`
}

// MatchTimeoutData is the data for match_timeout events.
type MatchTimeoutData struct {
	MatchID int64  `json:"matchId"`
	Reason  string `json:"reason"`
}

// MatchCancelledData is the data for match_cancelled events.
type MatchCancelledData struct {
	MatchID int64  `json:"matchId"`
	Reason  string `json:"reason"`
}

// LobbyInterestUpdateData is the data for lobby_interest_update events.
type LobbyInterestUpdateData struct {
	Count   int      `json:"count"`
	Players []string `json:"players"`
}

// LobbyReadyData is the data for lobby_ready events.
type LobbyReadyData struct {
	GroupID   string   `json:"groupId"`
	Players   []string `json:"players"`
	ExpiresAt int64    `json:"expiresAt"`
	Message   string   `json:"message"`
}

// LobbyGroupExpiredData is the data for lobby_group_expired events.
type LobbyGroupExpiredData struct {
	GroupID string `json:"groupId"`
}
