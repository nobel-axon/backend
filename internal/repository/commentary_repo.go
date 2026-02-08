// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"

	"github.com/lib/pq"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// CommentaryRepository handles commentary data access.
type CommentaryRepository struct {
	db *db.DB
}

// NewCommentaryRepository creates a new commentary repository.
func NewCommentaryRepository(database *db.DB) *CommentaryRepository {
	return &CommentaryRepository{db: database}
}

// GetByID retrieves commentary by ID.
func (r *CommentaryRepository) GetByID(ctx context.Context, id int64) (*models.Commentary, error) {
	var commentary models.Commentary
	err := r.db.GetContext(ctx, &commentary,
		`SELECT * FROM app_commentary WHERE id = $1`,
		id,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &commentary, nil
}

// GetByMatch retrieves all commentary for a match.
func (r *CommentaryRepository) GetByMatch(ctx context.Context, matchID int64) ([]models.Commentary, error) {
	var commentary []models.Commentary
	err := r.db.SelectContext(ctx, &commentary,
		`SELECT * FROM app_commentary WHERE match_id = $1 ORDER BY created_at`,
		matchID,
	)
	return commentary, err
}

// GetLatestByMatch retrieves the latest N commentary entries for a match.
func (r *CommentaryRepository) GetLatestByMatch(ctx context.Context, matchID int64, limit int) ([]models.Commentary, error) {
	var commentary []models.Commentary
	err := r.db.SelectContext(ctx, &commentary,
		`SELECT * FROM app_commentary WHERE match_id = $1 ORDER BY created_at DESC LIMIT $2`,
		matchID, limit,
	)
	return commentary, err
}

// CountByMatches returns the commentary count per match for a batch of match IDs.
func (r *CommentaryRepository) CountByMatches(ctx context.Context, matchIDs []int64) (map[int64]int, error) {
	if len(matchIDs) == 0 {
		return map[int64]int{}, nil
	}

	type row struct {
		MatchID int64 `db:"match_id"`
		Count   int   `db:"cnt"`
	}
	var rows []row
	err := r.db.SelectContext(ctx, &rows,
		`SELECT match_id, COUNT(*) AS cnt FROM app_commentary WHERE match_id = ANY($1) GROUP BY match_id`,
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

// GetByMatchesCapped retrieves commentary for multiple matches, capped at capPerMatch per match.
func (r *CommentaryRepository) GetByMatchesCapped(ctx context.Context, matchIDs []int64, capPerMatch int) (map[int64][]models.Commentary, error) {
	if len(matchIDs) == 0 {
		return map[int64][]models.Commentary{}, nil
	}

	var rows []models.Commentary
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, match_id, agent_id, event_type, text, created_at FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY match_id ORDER BY created_at) AS rn
			FROM app_commentary
			WHERE match_id = ANY($1)
		) sub WHERE rn <= $2`,
		pq.Array(matchIDs), capPerMatch,
	)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]models.Commentary, len(matchIDs))
	for _, c := range rows {
		result[c.MatchID] = append(result[c.MatchID], c)
	}
	return result, nil
}

// Create inserts new commentary.
func (r *CommentaryRepository) Create(ctx context.Context, input *models.CommentaryInput) (int64, error) {
	var id int64
	err := r.db.GetContext(ctx, &id,
		`INSERT INTO app_commentary (match_id, agent_id, event_type, text)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		input.MatchID, sql.NullString{String: input.AgentID, Valid: input.AgentID != ""},
		input.EventType, input.Text,
	)
	return id, err
}

// GetByEventType retrieves commentary by event type.
func (r *CommentaryRepository) GetByEventType(ctx context.Context, matchID int64, eventType string) ([]models.Commentary, error) {
	var commentary []models.Commentary
	err := r.db.SelectContext(ctx, &commentary,
		`SELECT * FROM app_commentary WHERE match_id = $1 AND event_type = $2 ORDER BY created_at`,
		matchID, eventType,
	)
	return commentary, err
}
