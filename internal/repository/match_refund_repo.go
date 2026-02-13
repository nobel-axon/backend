// Package repository provides data access layer for axon-server.
package repository

import (
	"context"

	"github.com/axon-arena/axon-server/internal/db"
)

// MatchRefundRepository handles match refund and burn claim data access.
type MatchRefundRepository struct {
	db *db.DB
}

// NewMatchRefundRepository creates a new match refund repository.
func NewMatchRefundRepository(database *db.DB) *MatchRefundRepository {
	return &MatchRefundRepository{db: database}
}

// RecordRefundCredited records a refund credit for a player.
func (r *MatchRefundRepository) RecordRefundCredited(ctx context.Context, matchID int64, playerAddr string, amount string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_match_refunds (match_id, player_addr, amount)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`,
		matchID, playerAddr, amount,
	)
	return err
}

// RecordRefundWithdrawn marks a player's refund as withdrawn.
func (r *MatchRefundRepository) RecordRefundWithdrawn(ctx context.Context, playerAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_match_refunds SET refund_withdrawn = TRUE, withdrawn_at = NOW()
		WHERE LOWER(player_addr) = LOWER($1) AND refund_withdrawn = FALSE`,
		playerAddr,
	)
	return err
}

// RecordBurnAllocationClaimed records a burn allocation claim.
func (r *MatchRefundRepository) RecordBurnAllocationClaimed(ctx context.Context, operator string, winner string, amount string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_match_burn_claims (operator, winner, amount)
		VALUES ($1, $2, $3)`,
		operator, winner, amount,
	)
	return err
}
