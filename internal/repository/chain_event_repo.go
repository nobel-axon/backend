// Package repository provides data access layer for axon-server.
package repository

import (
	"context"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// ChainEventRepository handles access to Ponder-indexed chain event tables.
type ChainEventRepository struct {
	db *db.DB
}

// NewChainEventRepository creates a new chain event repository.
func NewChainEventRepository(database *db.DB) *ChainEventRepository {
	return &ChainEventRepository{db: database}
}

// GetNewAgentJoinedQueue retrieves AgentJoinedQueue events from blocks > lastBlockNumber.
// Events are ordered by blockNumber to ensure chronological processing.
// Chief deduplicates events via txHash:logIndex, so re-sending within the same block is safe.
func (r *ChainEventRepository) GetNewAgentJoinedQueue(ctx context.Context, lastBlockNumber int64, limit int) ([]models.ChainAgentJoinedQueue, error) {
	var events []models.ChainAgentJoinedQueue

	err := r.db.SelectContext(ctx, &events,
		`SELECT id, "matchId" AS match_id, agent, "playerCount" AS player_count,
			"poolTotal" AS pool_total, "blockNumber" AS block_number,
			"blockTimestamp" AS block_timestamp, "transactionHash" AS transaction_hash
		FROM chain_agent_joined_queue
		WHERE "blockNumber" > $1
		ORDER BY "blockNumber" ASC, id ASC
		LIMIT $2`,
		lastBlockNumber, limit,
	)
	return events, err
}

// GetNewAnswerSubmitted retrieves AnswerSubmitted events from blocks > lastBlockNumber.
// Events are ordered by blockNumber to ensure chronological processing.
// Chief deduplicates events via txHash:logIndex, so re-sending within the same block is safe.
func (r *ChainEventRepository) GetNewAnswerSubmitted(ctx context.Context, lastBlockNumber int64, limit int) ([]models.ChainAnswerSubmitted, error) {
	var events []models.ChainAnswerSubmitted

	err := r.db.SelectContext(ctx, &events,
		`SELECT id, "matchId" AS match_id, agent, answer,
			"attemptNumber" AS attempt_number, "neuronBurned" AS neuron_burned,
			"blockNumber" AS block_number, "blockTimestamp" AS block_timestamp,
			"transactionHash" AS transaction_hash
		FROM chain_answer_submitted
		WHERE "blockNumber" > $1
		ORDER BY "blockNumber" ASC, id ASC
		LIMIT $2`,
		lastBlockNumber, limit,
	)
	return events, err
}
