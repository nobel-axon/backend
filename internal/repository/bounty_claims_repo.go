package repository

import (
	"context"

	"github.com/axon-arena/axon-server/internal/db"
)

// BountyClaimsRepository handles bounty claim data access.
type BountyClaimsRepository struct {
	db *db.DB
}

// NewBountyClaimsRepository creates a new bounty claims repository.
func NewBountyClaimsRepository(database *db.DB) *BountyClaimsRepository {
	return &BountyClaimsRepository{db: database}
}

// Create inserts a new bounty claim record.
func (r *BountyClaimsRepository) Create(ctx context.Context, bountyID int64, claimType, address, amount string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_bounty_claims (bounty_id, claim_type, address, amount) VALUES ($1, $2, $3, $4)`,
		bountyID, claimType, address, amount,
	)
	return err
}
