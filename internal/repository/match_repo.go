// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// MatchRepository handles match data access.
type MatchRepository struct {
	db *db.DB
}

// NewMatchRepository creates a new match repository.
func NewMatchRepository(database *db.DB) *MatchRepository {
	return &MatchRepository{db: database}
}

// GetByID retrieves a match by ID.
func (r *MatchRepository) GetByID(ctx context.Context, matchID int64) (*models.Match, error) {
	var match models.Match
	err := r.db.GetContext(ctx, &match,
		`SELECT * FROM app_matches WHERE match_id = $1`,
		matchID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &match, nil
}

// List retrieves matches with optional filtering.
func (r *MatchRepository) List(ctx context.Context, params models.MatchListParams) ([]models.Match, error) {
	query := `SELECT * FROM app_matches`
	args := []interface{}{}
	argNum := 1

	if params.Phase != "" {
		query += fmt.Sprintf(" WHERE phase = $%d", argNum)
		args = append(args, params.Phase)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, params.Limit)
		argNum++
	}
	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, params.Offset)
	}

	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches, query, args...)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// GetOpenMatches retrieves matches in registration phase with unexpired deadlines.
func (r *MatchRepository) GetOpenMatches(ctx context.Context) ([]models.Match, error) {
	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches,
		`SELECT * FROM app_matches
		WHERE phase = 'registration'
		AND (registration_end IS NULL OR registration_end > NOW())
		ORDER BY created_at DESC`,
	)
	return matches, err
}

// GetLiveMatches retrieves active matches (started, question_live, or answer_period) with players.
func (r *MatchRepository) GetLiveMatches(ctx context.Context) ([]models.Match, error) {
	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches,
		`SELECT * FROM app_matches
		WHERE phase IN ('started', 'question_live', 'answer_period')
		AND player_count > 0
		ORDER BY created_at DESC`,
	)
	return matches, err
}

// GetPlayers retrieves players for a match.
func (r *MatchRepository) GetPlayers(ctx context.Context, matchID int64) ([]models.MatchPlayer, error) {
	var players []models.MatchPlayer
	err := r.db.SelectContext(ctx, &players,
		`SELECT * FROM app_match_players WHERE match_id = $1 ORDER BY registered_at`,
		matchID,
	)
	return players, err
}

// Create inserts a new match.
func (r *MatchRepository) Create(ctx context.Context, match *models.Match) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_matches (match_id, phase, entry_fee, answer_fee, pool_total, player_count,
			question_text, category, difficulty, format_hint, answer_hash, winner_address,
			generator_agent, registration_end, answer_timeout, settled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		match.MatchID, match.Phase, match.EntryFee, match.AnswerFee, match.PoolTotal,
		match.PlayerCount, match.QuestionText, match.Category, match.Difficulty,
		match.FormatHint, match.AnswerHash, match.WinnerAddress, match.GeneratorAgent,
		match.RegistrationEnd, match.AnswerTimeout, match.SettledAt,
	)
	return err
}

// Update updates an existing match.
func (r *MatchRepository) Update(ctx context.Context, match *models.Match) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_matches SET
			phase = $2, entry_fee = $3, answer_fee = $4, pool_total = $5, player_count = $6,
			question_text = $7, category = $8, difficulty = $9, format_hint = $10,
			answer_hash = $11, winner_address = $12, generator_agent = $13,
			registration_end = $14, answer_timeout = $15, settled_at = $16
		WHERE match_id = $1`,
		match.MatchID, match.Phase, match.EntryFee, match.AnswerFee, match.PoolTotal,
		match.PlayerCount, match.QuestionText, match.Category, match.Difficulty,
		match.FormatHint, match.AnswerHash, match.WinnerAddress, match.GeneratorAgent,
		match.RegistrationEnd, match.AnswerTimeout, match.SettledAt,
	)
	return err
}

// UpdatePhase updates the phase of a match.
func (r *MatchRepository) UpdatePhase(ctx context.Context, matchID int64, phase string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_matches SET phase = $2 WHERE match_id = $1`,
		matchID, phase,
	)
	return err
}

// SetWinner sets the winner of a match.
func (r *MatchRepository) SetWinner(ctx context.Context, matchID int64, winnerAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_matches SET winner_address = $2, phase = 'settled', settled_at = $3 WHERE match_id = $1`,
		matchID, winnerAddr, time.Now(),
	)
	return err
}

// AddPlayer adds a player to a match.
func (r *MatchRepository) AddPlayer(ctx context.Context, matchID int64, agentAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_match_players (match_id, agent_addr) VALUES ($1, $2)
		ON CONFLICT (match_id, agent_addr) DO NOTHING`,
		matchID, agentAddr,
	)
	if err != nil {
		return err
	}

	// Update player count
	_, err = r.db.ExecContext(ctx,
		`UPDATE app_matches SET player_count = (
			SELECT COUNT(*) FROM app_match_players WHERE match_id = $1
		) WHERE match_id = $1`,
		matchID,
	)
	return err
}

// GetMatchesByAgent retrieves recent matches for an agent (case-insensitive).
func (r *MatchRepository) GetMatchesByAgent(ctx context.Context, agentAddr string, limit int) ([]models.Match, error) {
	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches,
		`SELECT m.* FROM app_matches m
		INNER JOIN app_match_players p ON m.match_id = p.match_id
		WHERE LOWER(p.agent_addr) = LOWER($1)
		ORDER BY m.created_at DESC
		LIMIT $2`,
		agentAddr, limit,
	)
	return matches, err
}

// GetMatchesExpiredRegistration gets matches past registration deadline that haven't been reported.
func (r *MatchRepository) GetMatchesExpiredRegistration(ctx context.Context) ([]models.Match, error) {
	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches,
		`SELECT * FROM app_matches
		WHERE phase = 'registration'
		AND registration_end < NOW()
		AND registration_reported_at IS NULL`,
	)
	return matches, err
}

// GetMatchesExpiredAnswer gets matches past answer deadline that haven't been reported.
func (r *MatchRepository) GetMatchesExpiredAnswer(ctx context.Context) ([]models.Match, error) {
	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches,
		`SELECT * FROM app_matches
		WHERE phase = 'question_live'
		AND answer_timeout < NOW()
		AND timeout_reported_at IS NULL`,
	)
	return matches, err
}

// CountByPhase counts matches by phase.
func (r *MatchRepository) CountByPhase(ctx context.Context, phase string) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM app_matches WHERE phase = $1`,
		phase,
	)
	return count, err
}

// CountTotal counts all matches.
func (r *MatchRepository) CountTotal(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM app_matches`)
	return count, err
}

// CountLast24h counts matches in last 24 hours.
func (r *MatchRepository) CountLast24h(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM app_matches WHERE created_at > NOW() - INTERVAL '24 hours'`,
	)
	return count, err
}

// CountWithFilter returns total count of matches with optional phase filter.
func (r *MatchRepository) CountWithFilter(ctx context.Context, phase string) (int, error) {
	var count int
	var err error
	if phase != "" {
		err = r.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM app_matches WHERE phase = $1`,
			phase,
		)
	} else {
		err = r.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM app_matches`,
		)
	}
	return count, err
}

// MarkRegistrationReported marks a match's registration expiry as reported to Chief.
func (r *MatchRepository) MarkRegistrationReported(ctx context.Context, matchID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_matches SET registration_reported_at = NOW() WHERE match_id = $1`,
		matchID,
	)
	return err
}

// MarkTimeoutReported marks a match's timeout as reported to Chief.
func (r *MatchRepository) MarkTimeoutReported(ctx context.Context, matchID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_matches SET timeout_reported_at = NOW() WHERE match_id = $1`,
		matchID,
	)
	return err
}

// SetRevealedAnswer stores the revealed answer and salt for a match.
func (r *MatchRepository) SetRevealedAnswer(ctx context.Context, matchID int64, answer string, salt string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_matches SET revealed_answer = $2, revealed_salt = $3 WHERE match_id = $1`,
		matchID, answer, salt,
	)
	return err
}

// GetCategories returns all distinct categories from matches.
func (r *MatchRepository) GetCategories(ctx context.Context) ([]string, error) {
	var categories []string
	err := r.db.SelectContext(ctx, &categories,
		`SELECT DISTINCT category FROM app_matches WHERE category IS NOT NULL ORDER BY category`,
	)
	return categories, err
}

// GetMatchesByAgentPaginated retrieves paginated match history for an agent (case-insensitive).
func (r *MatchRepository) GetMatchesByAgentPaginated(ctx context.Context, agentAddr string, limit, offset int) ([]models.Match, int, error) {
	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches,
		`SELECT m.* FROM app_matches m
		INNER JOIN app_match_players p ON m.match_id = p.match_id
		WHERE LOWER(p.agent_addr) = LOWER($1)
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3`,
		agentAddr, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}

	var total int
	err = r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM app_match_players WHERE LOWER(agent_addr) = LOWER($1)`,
		agentAddr,
	)
	if err != nil {
		return nil, 0, err
	}

	return matches, total, nil
}

// ListSettledForFeed returns settled/cancelled matches ordered by settled_at DESC
// for cursor-paginated feed. Returns up to limit rows.
func (r *MatchRepository) ListSettledForFeed(ctx context.Context, limit int, cursorTime *time.Time, cursorID *int64, category string) ([]models.Match, error) {
	query := `SELECT * FROM app_matches WHERE phase IN ('settled', 'cancelled') AND settled_at IS NOT NULL`
	args := []interface{}{}
	argNum := 1

	if cursorTime != nil && cursorID != nil {
		query += fmt.Sprintf(` AND (settled_at < $%d OR (settled_at = $%d AND match_id < $%d))`, argNum, argNum, argNum+1)
		args = append(args, *cursorTime, *cursorID)
		argNum += 2
	}

	if category != "" {
		query += fmt.Sprintf(` AND category = $%d`, argNum)
		args = append(args, category)
		argNum++
	}

	query += ` ORDER BY settled_at DESC, match_id DESC`
	query += fmt.Sprintf(` LIMIT $%d`, argNum)
	args = append(args, limit)

	var matches []models.Match
	err := r.db.SelectContext(ctx, &matches, query, args...)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// GetPlayersByMatches retrieves players for multiple matches in one query.
func (r *MatchRepository) GetPlayersByMatches(ctx context.Context, matchIDs []int64) (map[int64][]models.MatchPlayer, error) {
	if len(matchIDs) == 0 {
		return map[int64][]models.MatchPlayer{}, nil
	}

	var players []models.MatchPlayer
	err := r.db.SelectContext(ctx, &players,
		`SELECT * FROM app_match_players WHERE match_id = ANY($1) ORDER BY match_id, registered_at`,
		pq.Array(matchIDs),
	)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]models.MatchPlayer, len(matchIDs))
	for _, p := range players {
		result[p.MatchID] = append(result[p.MatchID], p)
	}
	return result, nil
}
