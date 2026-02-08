// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// PollerRepository handles poller state persistence.
type PollerRepository struct {
	db *db.DB
}

// NewPollerRepository creates a new poller repository.
func NewPollerRepository(database *db.DB) *PollerRepository {
	return &PollerRepository{db: database}
}

// Get retrieves a poller state value by key.
func (r *PollerRepository) Get(ctx context.Context, key string) (*models.PollerState, error) {
	var state models.PollerState
	err := r.db.GetContext(ctx, &state,
		`SELECT * FROM app_poller_state WHERE key = $1`,
		key,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

// Set creates or updates a poller state value.
func (r *PollerRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_poller_state (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value,
	)
	return err
}

// GetLastBlockNumber retrieves the last processed block number.
func (r *PollerRepository) GetLastBlockNumber(ctx context.Context) (int64, error) {
	state, err := r.Get(ctx, "last_block_number")
	if err != nil {
		return 0, err
	}
	if state == nil {
		return 0, nil
	}
	blockNum, err := strconv.ParseInt(state.Value, 10, 64)
	if err != nil {
		return 0, nil
	}
	return blockNum, nil
}

// SetLastBlockNumber sets the last processed block number.
func (r *PollerRepository) SetLastBlockNumber(ctx context.Context, blockNum int64) error {
	return r.Set(ctx, "last_block_number", strconv.FormatInt(blockNum, 10))
}

// GetAll retrieves all poller state values.
func (r *PollerRepository) GetAll(ctx context.Context) ([]models.PollerState, error) {
	var states []models.PollerState
	err := r.db.SelectContext(ctx, &states, `SELECT * FROM app_poller_state`)
	return states, err
}
