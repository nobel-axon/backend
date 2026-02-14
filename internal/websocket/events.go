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

	// Bounty events (V2)
	EventBountySettled         = "bounty_settled"
	EventBountyAnswerSubmitted = "bounty_answer_submitted"
	EventAgentJoinedBounty     = "bounty_agent_joined"
	EventPersonalitiesAssigned = "personalities_assigned"
	EventAnswerRevealed        = "answer_revealed"
	EventReputationUpdated     = "reputation_updated"
	EventWinnerRewardClaimed   = "winner_reward_claimed"
	EventProportionalClaimed   = "proportional_claimed"
	EventRefundClaimed         = "refund_claimed"
	EventBountyAnswerEvaluated = "bounty_answer_evaluated"
	EventBountyApproved        = "bounty_approved"
	EventBountyRejected        = "bounty_rejected"
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

// MatchBroadcastData wraps a MatchResponse with its MatchID for hub dedup.
// The hub extracts MatchID from this struct; the full match response is nested
// under the "match" key for the frontend.
type MatchBroadcastData struct {
	MatchID int64       `json:"matchId"`
	Match   interface{} `json:"match"`
}

// PersonalitiesAssignedData is the data for personalities_assigned events.
type PersonalitiesAssignedData struct {
	MatchID       int64           `json:"matchId"`
	Personalities json.RawMessage `json:"personalities"`
	JudgePanel    json.RawMessage `json:"judgePanel"`
}

// AnswerRevealedData is the data for answer_revealed events.
type AnswerRevealedData struct {
	MatchID int64  `json:"matchId"`
	Answer  string `json:"answer"`
	Salt    string `json:"salt"`
}

// BountySettledData is the data for bounty_settled events.
type BountySettledData struct {
	BountyID     int64  `json:"bountyId"`
	WinnerAddr   string `json:"winnerAddr"`
	RewardAmount string `json:"rewardAmount"`
}

// BountyAnswerSubmittedData is the data for bounty_answer_submitted events.
type BountyAnswerSubmittedData struct {
	BountyID      int64  `json:"bountyId"`
	AgentAddr     string `json:"agentAddr"`
	AttemptNumber int    `json:"attemptNumber"`
}

// AgentJoinedBountyData is the data for agent_joined_bounty events.
type AgentJoinedBountyData struct {
	BountyID  int64  `json:"bountyId"`
	AgentAddr string `json:"agentAddr"`
}

// ReputationUpdatedData is the data for reputation_updated events.
type ReputationUpdatedData struct {
	AgentAddr string `json:"agentAddr"`
	Score     int    `json:"score"`
}

// WinnerRewardClaimedData is the data for winner_reward_claimed events.
type WinnerRewardClaimedData struct {
	BountyID int64  `json:"bountyId"`
	Winner   string `json:"winner"`
	Amount   string `json:"amount"`
}

// ProportionalClaimedData is the data for proportional_claimed events.
type ProportionalClaimedData struct {
	BountyID int64  `json:"bountyId"`
	Agent    string `json:"agent"`
	Amount   string `json:"amount"`
}

// RefundClaimedData is the data for refund_claimed events.
type RefundClaimedData struct {
	BountyID int64  `json:"bountyId"`
	Creator  string `json:"creator"`
	Amount   string `json:"amount"`
}

// BountyAnswerEvaluatedData is the data for bounty_answer_evaluated events.
type BountyAnswerEvaluatedData struct {
	BountyID   int64  `json:"bountyId"`
	AgentAddr  string `json:"agentAddr"`
	TotalScore int    `json:"totalScore"`
	Agreement  string `json:"agreement"`
}

// BountyApprovedData wraps the full BountyResponse for hub dedup.
type BountyApprovedData struct {
	BountyID int64       `json:"bountyId"`
	Bounty   interface{} `json:"bounty"`
}

// BountyRejectedData is the data for bounty_rejected events.
type BountyRejectedData struct {
	BountyID int64  `json:"bountyId"`
	Reason   string `json:"reason"`
}
