// Package models defines data structures for axon-server.
package models

// ChainAgentJoinedQueue represents a row from the Ponder chain_agent_joined_queue table.
type ChainAgentJoinedQueue struct {
	ID              string `db:"id"`
	MatchID         int64  `db:"match_id"`
	Agent           string `db:"agent"`
	PlayerCount     int64  `db:"player_count"`
	PoolTotal       string `db:"pool_total"`
	BlockNumber     int64  `db:"block_number"`
	BlockTimestamp  int64  `db:"block_timestamp"`
	TransactionHash string `db:"transaction_hash"`
}

// ChainAnswerSubmitted represents a row from the Ponder chain_answer_submitted table.
type ChainAnswerSubmitted struct {
	ID              string `db:"id"`
	MatchID         int64  `db:"match_id"`
	Agent           string `db:"agent"`
	Answer          string `db:"answer"`
	AttemptNumber   int64  `db:"attempt_number"`
	NeuronBurned    string `db:"neuron_burned"`
	BlockNumber     int64  `db:"block_number"`
	BlockTimestamp  int64  `db:"block_timestamp"`
	TransactionHash string `db:"transaction_hash"`
}
