package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lib/pq"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// PersonalityRepository handles personality data access.
type PersonalityRepository struct {
	db *db.DB
}

// NewPersonalityRepository creates a new personality repository.
func NewPersonalityRepository(database *db.DB) *PersonalityRepository {
	return &PersonalityRepository{db: database}
}

// Upsert inserts or updates personalities for a match.
func (r *PersonalityRepository) Upsert(ctx context.Context, matchID int64, personalities json.RawMessage, judgePanel json.RawMessage) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_match_personalities (match_id, personalities, judge_panel)
		VALUES ($1, $2, $3)
		ON CONFLICT (match_id) DO UPDATE SET
			personalities = EXCLUDED.personalities,
			judge_panel = EXCLUDED.judge_panel`,
		matchID, personalities, judgePanel,
	)
	return err
}

// GetByMatch retrieves personalities for a match.
func (r *PersonalityRepository) GetByMatch(ctx context.Context, matchID int64) (*models.MatchPersonalities, error) {
	var mp models.MatchPersonalities
	err := r.db.GetContext(ctx, &mp,
		`SELECT * FROM app_match_personalities WHERE match_id = $1`,
		matchID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &mp, nil
}

// GetByMatches retrieves personalities for multiple matches in one query.
func (r *PersonalityRepository) GetByMatches(ctx context.Context, matchIDs []int64) (map[int64]*models.MatchPersonalities, error) {
	if len(matchIDs) == 0 {
		return map[int64]*models.MatchPersonalities{}, nil
	}

	var rows []models.MatchPersonalities
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM app_match_personalities WHERE match_id = ANY($1)`,
		pq.Array(matchIDs),
	)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*models.MatchPersonalities, len(rows))
	for i := range rows {
		result[rows[i].MatchID] = &rows[i]
	}
	return result, nil
}
