// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"encoding/json"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// BountyAnswerRepository handles bounty answer data access.
type BountyAnswerRepository struct {
	db *db.DB
}

// NewBountyAnswerRepository creates a new bounty answer repository.
func NewBountyAnswerRepository(database *db.DB) *BountyAnswerRepository {
	return &BountyAnswerRepository{db: database}
}

// Create inserts a new bounty answer.
func (r *BountyAnswerRepository) Create(ctx context.Context, answer *models.BountyAnswer) (int64, error) {
	var id int64
	err := r.db.GetContext(ctx, &id,
		`INSERT INTO app_bounty_answers (bounty_id, agent_addr, answer_text, reasoning, tx_hash)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (bounty_id, agent_addr) DO UPDATE SET
			answer_text = $3, reasoning = $4, tx_hash = $5
		RETURNING id`,
		answer.BountyID, answer.AgentAddr, answer.AnswerText, answer.Reasoning, answer.TxHash,
	)
	return id, err
}

// GetByBounty retrieves all answers for a bounty.
func (r *BountyAnswerRepository) GetByBounty(ctx context.Context, bountyID int64) ([]models.BountyAnswer, error) {
	var answers []models.BountyAnswer
	err := r.db.SelectContext(ctx, &answers,
		`SELECT * FROM app_bounty_answers WHERE bounty_id = $1 ORDER BY submitted_at ASC`,
		bountyID,
	)
	return answers, err
}

// UpdateEvaluation updates the evaluation result for a bounty answer.
func (r *BountyAnswerRepository) UpdateEvaluation(ctx context.Context, bountyID int64, agentAddr string, totalScore int, agreement string, evaluations json.RawMessage) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_bounty_answers SET
			total_score = $3, agreement = $4, evaluations = $5, evaluated_at = NOW()
		WHERE bounty_id = $1 AND LOWER(agent_addr) = LOWER($2)`,
		bountyID, agentAddr, totalScore, agreement, evaluations,
	)
	return err
}
