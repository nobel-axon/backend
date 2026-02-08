// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// BurnRepository handles burn statistics data access.
type BurnRepository struct {
	db *db.DB
}

// NewBurnRepository creates a new burn repository.
func NewBurnRepository(database *db.DB) *BurnRepository {
	return &BurnRepository{db: database}
}

// Create records a new burn event.
func (r *BurnRepository) Create(ctx context.Context, matchID *int64, agentAddr string, amount string) (int64, error) {
	var id int64
	err := r.db.GetContext(ctx, &id,
		`INSERT INTO app_burn_stats (match_id, agent_addr, amount_burned)
		VALUES ($1, $2, $3)
		RETURNING id`,
		matchID, agentAddr, amount,
	)
	return id, err
}

// GetByMatch retrieves burn stats for a match.
func (r *BurnRepository) GetByMatch(ctx context.Context, matchID int64) ([]models.BurnStats, error) {
	var burns []models.BurnStats
	err := r.db.SelectContext(ctx, &burns,
		`SELECT * FROM app_burn_stats WHERE match_id = $1 ORDER BY recorded_at`,
		matchID,
	)
	return burns, err
}

// GetByAgent retrieves burn stats for an agent.
func (r *BurnRepository) GetByAgent(ctx context.Context, agentAddr string, limit int) ([]models.BurnStats, error) {
	var burns []models.BurnStats
	err := r.db.SelectContext(ctx, &burns,
		`SELECT * FROM app_burn_stats WHERE agent_addr = $1 ORDER BY recorded_at DESC LIMIT $2`,
		agentAddr, limit,
	)
	return burns, err
}

// GetRecent retrieves recent burn events.
func (r *BurnRepository) GetRecent(ctx context.Context, limit int) ([]models.BurnStats, error) {
	var burns []models.BurnStats
	err := r.db.SelectContext(ctx, &burns,
		`SELECT * FROM app_burn_stats ORDER BY recorded_at DESC LIMIT $1`,
		limit,
	)
	return burns, err
}

// GetTotalBurned retrieves total amount burned.
func (r *BurnRepository) GetTotalBurned(ctx context.Context) (string, error) {
	var total sql.NullString
	err := r.db.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(amount_burned::numeric), 0)::text FROM app_burn_stats`,
	)
	if err != nil {
		return "0", err
	}
	if !total.Valid {
		return "0", nil
	}
	return total.String, nil
}

// GetTotalBurnedLast24h retrieves total amount burned in last 24 hours.
func (r *BurnRepository) GetTotalBurnedLast24h(ctx context.Context) (string, error) {
	var total sql.NullString
	err := r.db.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(amount_burned::numeric), 0)::text FROM app_burn_stats
		WHERE recorded_at > NOW() - INTERVAL '24 hours'`,
	)
	if err != nil {
		return "0", err
	}
	if !total.Valid {
		return "0", nil
	}
	return total.String, nil
}

// GetHourlyTimeline retrieves hourly burn timeline.
func (r *BurnRepository) GetHourlyTimeline(ctx context.Context, hours int) ([]models.BurnTimeline, error) {
	var timeline []models.BurnTimeline
	err := r.db.SelectContext(ctx, &timeline,
		`SELECT
			date_trunc('hour', recorded_at)::text as timestamp,
			COALESCE(SUM(amount_burned::numeric), 0)::text as total_burned,
			COUNT(*) as burn_count,
			COUNT(DISTINCT agent_addr) as unique_agents
		FROM app_burn_stats
		WHERE recorded_at > NOW() - make_interval(hours => $1)
		GROUP BY date_trunc('hour', recorded_at)
		ORDER BY timestamp DESC`,
		hours,
	)
	return timeline, err
}

// GetDailyTimeline retrieves daily burn timeline.
func (r *BurnRepository) GetDailyTimeline(ctx context.Context, days int) ([]models.BurnTimeline, error) {
	var timeline []models.BurnTimeline
	err := r.db.SelectContext(ctx, &timeline,
		`SELECT
			date_trunc('day', recorded_at)::text as timestamp,
			COALESCE(SUM(amount_burned::numeric), 0)::text as total_burned,
			COUNT(*) as burn_count,
			COUNT(DISTINCT agent_addr) as unique_agents
		FROM app_burn_stats
		WHERE recorded_at > NOW() - make_interval(days => $1)
		GROUP BY date_trunc('day', recorded_at)
		ORDER BY timestamp DESC`,
		days,
	)
	return timeline, err
}

// GetTotalPoolVolume retrieves total MON pool volume from matches.
func (r *BurnRepository) GetTotalPoolVolume(ctx context.Context) (string, error) {
	var total sql.NullString
	err := r.db.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(pool_total::numeric), 0)::text FROM app_matches WHERE phase = 'settled'`,
	)
	if err != nil {
		return "0", err
	}
	if !total.Valid {
		return "0", nil
	}
	return total.String, nil
}

// RecordBurn is a helper that creates a burn record and updates agent stats.
func (r *BurnRepository) RecordBurn(ctx context.Context, matchID *int64, agentAddr string, amount string, agentRepo *AgentRepository) error {
	_, err := r.Create(ctx, matchID, agentAddr, amount)
	if err != nil {
		return err
	}

	// Also update agent's total burned
	return agentRepo.AddBurnedNeuron(ctx, agentAddr, amount)
}

// GetLatestRecordedAt returns the most recent burn timestamp.
func (r *BurnRepository) GetLatestRecordedAt(ctx context.Context) (time.Time, error) {
	var t sql.NullTime
	err := r.db.GetContext(ctx, &t, `SELECT MAX(recorded_at) FROM app_burn_stats`)
	if err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}
