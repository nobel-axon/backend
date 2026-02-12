// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// BountyRepository handles bounty data access.
type BountyRepository struct {
	db *db.DB
}

// NewBountyRepository creates a new bounty repository.
func NewBountyRepository(database *db.DB) *BountyRepository {
	return &BountyRepository{db: database}
}

// Create inserts a new bounty.
func (r *BountyRepository) Create(ctx context.Context, bounty *models.Bounty) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_bounties (bounty_id, creator_address, question_text, category, difficulty,
			entry_fee, pool_total, min_rating, max_participants, player_count, phase, deadline)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (bounty_id) DO NOTHING`,
		bounty.BountyID, bounty.CreatorAddress, bounty.QuestionText, bounty.Category,
		bounty.Difficulty, bounty.EntryFee, bounty.PoolTotal, bounty.MinRating,
		bounty.MaxParticipants, bounty.PlayerCount, bounty.Phase, bounty.Deadline,
	)
	return err
}

// GetByID retrieves a bounty by ID.
func (r *BountyRepository) GetByID(ctx context.Context, bountyID int64) (*models.Bounty, error) {
	var bounty models.Bounty
	err := r.db.GetContext(ctx, &bounty,
		`SELECT * FROM app_bounties WHERE bounty_id = $1`, bountyID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &bounty, nil
}

// List retrieves bounties with optional phase filter and pagination.
func (r *BountyRepository) List(ctx context.Context, phase string, limit, offset int) ([]models.Bounty, error) {
	var bounties []models.Bounty
	if phase != "" {
		err := r.db.SelectContext(ctx, &bounties,
			`SELECT * FROM app_bounties WHERE phase = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			phase, limit, offset,
		)
		return bounties, err
	}
	err := r.db.SelectContext(ctx, &bounties,
		`SELECT * FROM app_bounties ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	return bounties, err
}

// UpdatePhase updates the phase of a bounty.
func (r *BountyRepository) UpdatePhase(ctx context.Context, bountyID int64, phase string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_bounties SET phase = $2 WHERE bounty_id = $1`,
		bountyID, phase,
	)
	return err
}

// SetWinner sets the winner and settles the bounty.
func (r *BountyRepository) SetWinner(ctx context.Context, bountyID int64, winner, txHash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_bounties SET phase = 'settled', winner_address = $2, settle_tx_hash = $3, settled_at = NOW()
		WHERE bounty_id = $1`,
		bountyID, winner, txHash,
	)
	return err
}

// AddPlayer records a player joining a bounty.
func (r *BountyRepository) AddPlayer(ctx context.Context, bountyID int64, agentAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_bounty_players (bounty_id, agent_addr) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		bountyID, agentAddr,
	)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE app_bounties SET player_count = player_count + 1 WHERE bounty_id = $1`,
		bountyID,
	)
	return err
}

// UpdatePoolTotal updates the pool total for a bounty.
func (r *BountyRepository) UpdatePoolTotal(ctx context.Context, bountyID int64, poolTotal string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_bounties SET pool_total = $2 WHERE bounty_id = $1`,
		bountyID, poolTotal,
	)
	return err
}

// Upsert inserts or updates a bounty (used by generic bounty-update handler).
func (r *BountyRepository) Upsert(ctx context.Context, bounty *models.Bounty) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_bounties (bounty_id, creator_address, question_text, category, difficulty,
			entry_fee, pool_total, min_rating, max_participants, player_count, phase, deadline)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (bounty_id) DO UPDATE SET
			creator_address = COALESCE(NULLIF($2, ''), app_bounties.creator_address),
			question_text = COALESCE(NULLIF($3, ''), app_bounties.question_text),
			category = COALESCE($4, app_bounties.category),
			difficulty = CASE WHEN $5 > 0 THEN $5 ELSE app_bounties.difficulty END,
			entry_fee = COALESCE(NULLIF($6, ''), app_bounties.entry_fee),
			pool_total = COALESCE(NULLIF($7, ''), app_bounties.pool_total),
			min_rating = COALESCE(NULLIF($8, ''), app_bounties.min_rating),
			max_participants = CASE WHEN $9 > 0 THEN $9 ELSE app_bounties.max_participants END,
			player_count = CASE WHEN $10 > 0 THEN $10 ELSE app_bounties.player_count END,
			phase = COALESCE(NULLIF($11, ''), app_bounties.phase),
			deadline = COALESCE($12, app_bounties.deadline),
			updated_at = NOW()`,
		bounty.BountyID, bounty.CreatorAddress, bounty.QuestionText, bounty.Category,
		bounty.Difficulty, bounty.EntryFee, bounty.PoolTotal, bounty.MinRating,
		bounty.MaxParticipants, bounty.PlayerCount, bounty.Phase, bounty.Deadline,
	)
	return err
}

// CountTotal counts total bounties.
func (r *BountyRepository) CountTotal(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM app_bounties`)
	return count, err
}

// CountByPhase counts bounties in a given phase.
func (r *BountyRepository) CountByPhase(ctx context.Context, phase string) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM app_bounties WHERE phase = $1`, phase)
	return count, err
}

// GetTotalRewardPool returns the sum of all bounty pool totals.
func (r *BountyRepository) GetTotalRewardPool(ctx context.Context) (string, error) {
	var total sql.NullString
	err := r.db.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(pool_total::numeric), 0)::text FROM app_bounties`,
	)
	if err != nil || !total.Valid {
		return "0", err
	}
	return total.String, nil
}

// GetAvgReward returns the average reward pool for settled bounties.
func (r *BountyRepository) GetAvgReward(ctx context.Context) (string, error) {
	var avg sql.NullString
	err := r.db.GetContext(ctx, &avg,
		`SELECT COALESCE(AVG(pool_total::numeric), 0)::text FROM app_bounties WHERE phase = 'settled'`,
	)
	if err != nil || !avg.Valid {
		return "0", err
	}
	return avg.String, nil
}
