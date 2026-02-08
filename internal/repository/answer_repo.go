// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// AnswerRepository handles answer data access.
type AnswerRepository struct {
	db *db.DB
}

// NewAnswerRepository creates a new answer repository.
func NewAnswerRepository(database *db.DB) *AnswerRepository {
	return &AnswerRepository{db: database}
}

// GetByID retrieves an answer by ID.
func (r *AnswerRepository) GetByID(ctx context.Context, id int64) (*models.Answer, error) {
	var answer models.Answer
	err := r.db.GetContext(ctx, &answer,
		`SELECT * FROM app_answers WHERE id = $1`,
		id,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &answer, nil
}

// GetByMatch retrieves all answers for a match.
func (r *AnswerRepository) GetByMatch(ctx context.Context, matchID int64) ([]models.Answer, error) {
	var answers []models.Answer
	err := r.db.SelectContext(ctx, &answers,
		`SELECT * FROM app_answers WHERE match_id = $1 ORDER BY block_number, tx_index`,
		matchID,
	)
	return answers, err
}

// GetByMatchAndAgent retrieves answers for a specific agent in a match.
func (r *AnswerRepository) GetByMatchAndAgent(ctx context.Context, matchID int64, agentAddr string) ([]models.Answer, error) {
	var answers []models.Answer
	err := r.db.SelectContext(ctx, &answers,
		`SELECT * FROM app_answers WHERE match_id = $1 AND agent_addr = $2 ORDER BY attempt_number`,
		matchID, agentAddr,
	)
	return answers, err
}

// GetCorrectAnswer retrieves the winning answer for a match.
func (r *AnswerRepository) GetCorrectAnswer(ctx context.Context, matchID int64) (*models.Answer, error) {
	var answer models.Answer
	err := r.db.GetContext(ctx, &answer,
		`SELECT * FROM app_answers WHERE match_id = $1 AND is_correct = true
		ORDER BY block_number, tx_index LIMIT 1`,
		matchID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &answer, nil
}

// Create inserts a new answer. Uses ON CONFLICT DO NOTHING for idempotency.
func (r *AnswerRepository) Create(ctx context.Context, answer *models.Answer) (int64, error) {
	var id int64
	err := r.db.GetContext(ctx, &id,
		`INSERT INTO app_answers (match_id, agent_addr, answer_text, block_number, tx_index,
			attempt_number, neuron_burned, reasoning, tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (match_id, agent_addr, attempt_number) DO NOTHING
		RETURNING id`,
		answer.MatchID, answer.AgentAddr, answer.AnswerText, answer.BlockNumber,
		answer.TxIndex, answer.AttemptNumber, answer.NeuronBurned, answer.Reasoning, answer.TxHash,
	)
	if err == sql.ErrNoRows {
		// ON CONFLICT DO NOTHING — row already exists, not an error
		return 0, nil
	}
	return id, err
}

// UpdateEvaluation updates the evaluation result for an answer.
func (r *AnswerRepository) UpdateEvaluation(ctx context.Context, matchID int64, agentAddr string, attemptNum int, totalScore int, agreement string, reasoning string, evaluations json.RawMessage) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_answers SET total_score = $4, agreement = $5, reasoning = $6, evaluations = $7
		WHERE match_id = $1 AND agent_addr = $2 AND attempt_number = $3`,
		matchID, agentAddr, attemptNum, totalScore, agreement, reasoning, evaluations,
	)
	return err
}

// UpdateVerification updates the verification result for an answer.
func (r *AnswerRepository) UpdateVerification(ctx context.Context, id int64, isCorrect bool, consensus string, confidence float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_answers SET is_correct = $2, consensus = $3, confidence = $4, verified_at = $5
		WHERE id = $1`,
		id, isCorrect, consensus, confidence, time.Now(),
	)
	return err
}

// UpdateVerificationByMatch updates verification for an answer by match and agent.
func (r *AnswerRepository) UpdateVerificationByMatch(ctx context.Context, matchID int64, agentAddr string, attemptNum int, isCorrect bool, consensus string, confidence float64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_answers SET is_correct = $4, consensus = $5, confidence = $6, verified_at = $7
		WHERE match_id = $1 AND agent_addr = $2 AND attempt_number = $3`,
		matchID, agentAddr, attemptNum, isCorrect, consensus, confidence, time.Now(),
	)
	return err
}

// GetUnverified retrieves answers that haven't been verified yet.
func (r *AnswerRepository) GetUnverified(ctx context.Context, limit int) ([]models.Answer, error) {
	var answers []models.Answer
	err := r.db.SelectContext(ctx, &answers,
		`SELECT * FROM app_answers WHERE verified_at IS NULL
		ORDER BY block_number, tx_index LIMIT $1`,
		limit,
	)
	return answers, err
}

// CountByAgent counts answers by an agent.
func (r *AnswerRepository) CountByAgent(ctx context.Context, agentAddr string) (correct int, wrong int, err error) {
	err = r.db.GetContext(ctx, &correct,
		`SELECT COUNT(*) FROM app_answers WHERE agent_addr = $1 AND is_correct = true`,
		agentAddr,
	)
	if err != nil {
		return
	}
	err = r.db.GetContext(ctx, &wrong,
		`SELECT COUNT(*) FROM app_answers WHERE agent_addr = $1 AND is_correct = false`,
		agentAddr,
	)
	return
}

// GetLatestByBlockNumber retrieves the highest block number in answers.
func (r *AnswerRepository) GetLatestByBlockNumber(ctx context.Context) (int64, error) {
	var blockNum sql.NullInt64
	err := r.db.GetContext(ctx, &blockNum,
		`SELECT MAX(block_number) FROM app_answers`,
	)
	if err != nil {
		return 0, err
	}
	if !blockNum.Valid {
		return 0, nil
	}
	return blockNum.Int64, nil
}

// ExistsByBlockAndTx checks if an answer already exists for a block/tx.
func (r *AnswerRepository) ExistsByBlockAndTx(ctx context.Context, blockNum int64, txIndex int) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists,
		`SELECT EXISTS(SELECT 1 FROM app_answers WHERE block_number = $1 AND tx_index = $2)`,
		blockNum, txIndex,
	)
	return exists, err
}

// CountByMatches returns the answer count per match for a batch of match IDs.
func (r *AnswerRepository) CountByMatches(ctx context.Context, matchIDs []int64) (map[int64]int, error) {
	if len(matchIDs) == 0 {
		return map[int64]int{}, nil
	}

	type row struct {
		MatchID int64 `db:"match_id"`
		Count   int   `db:"cnt"`
	}
	var rows []row
	err := r.db.SelectContext(ctx, &rows,
		`SELECT match_id, COUNT(*) AS cnt FROM app_answers WHERE match_id = ANY($1) GROUP BY match_id`,
		pq.Array(matchIDs),
	)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]int, len(rows))
	for _, r := range rows {
		result[r.MatchID] = r.Count
	}
	return result, nil
}

// GetByMatches retrieves answers for multiple matches in one query.
func (r *AnswerRepository) GetByMatches(ctx context.Context, matchIDs []int64) (map[int64][]models.Answer, error) {
	if len(matchIDs) == 0 {
		return map[int64][]models.Answer{}, nil
	}

	var answers []models.Answer
	err := r.db.SelectContext(ctx, &answers,
		`SELECT * FROM app_answers WHERE match_id = ANY($1) ORDER BY match_id, block_number, tx_index`,
		pq.Array(matchIDs),
	)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]models.Answer, len(matchIDs))
	for _, a := range answers {
		result[a.MatchID] = append(result[a.MatchID], a)
	}
	return result, nil
}
