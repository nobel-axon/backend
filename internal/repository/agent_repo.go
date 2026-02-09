// Package repository provides data access layer for axon-server.
package repository

import (
	"context"
	"database/sql"
	"math/big"
	"time"

	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/models"
)

// AgentRepository handles agent statistics data access.
type AgentRepository struct {
	db *db.DB
}

// NewAgentRepository creates a new agent repository.
func NewAgentRepository(database *db.DB) *AgentRepository {
	return &AgentRepository{db: database}
}

// GetByAddress retrieves agent stats by address (case-insensitive).
func (r *AgentRepository) GetByAddress(ctx context.Context, agentAddr string) (*models.AgentStats, error) {
	var stats models.AgentStats
	err := r.db.GetContext(ctx, &stats,
		`SELECT * FROM app_agent_stats WHERE LOWER(agent_addr) = LOWER($1)`,
		agentAddr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &stats, nil
}

// Upsert creates or updates agent stats.
func (r *AgentRepository) Upsert(ctx context.Context, stats *models.AgentStats) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_agent_stats (agent_addr, matches_played, matches_won, total_earned_mon,
			total_earned_neuron, total_burned_neuron, wrong_answers, correct_answers,
			avg_answer_time_ms, last_active, first_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (agent_addr) DO UPDATE SET
			matches_played = $2, matches_won = $3, total_earned_mon = $4,
			total_earned_neuron = $5, total_burned_neuron = $6, wrong_answers = $7,
			correct_answers = $8, avg_answer_time_ms = $9, last_active = $10`,
		stats.AgentAddr, stats.MatchesPlayed, stats.MatchesWon, stats.TotalEarnedMON,
		stats.TotalEarnedNeuron, stats.TotalBurnedNeuron, stats.WrongAnswers,
		stats.CorrectAnswers, stats.AvgAnswerTimeMs, stats.LastActive, stats.FirstSeen,
	)
	return err
}

// IncrementMatchesPlayed increments the matches played count.
func (r *AgentRepository) IncrementMatchesPlayed(ctx context.Context, agentAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_agent_stats (agent_addr, matches_played, last_active)
		VALUES ($1, 1, NOW())
		ON CONFLICT (agent_addr) DO UPDATE SET
			matches_played = app_agent_stats.matches_played + 1,
			last_active = NOW()`,
		agentAddr,
	)
	return err
}

// RecordWin records a match win for an agent.
func (r *AgentRepository) RecordWin(ctx context.Context, agentAddr string, earnedMON string, earnedNeuron string) error {
	// Default empty strings to "0" to avoid numeric cast errors
	if earnedMON == "" {
		earnedMON = "0"
	}
	if earnedNeuron == "" {
		earnedNeuron = "0"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_agent_stats (agent_addr, matches_played, matches_won, total_earned_mon, total_earned_neuron, last_active)
		VALUES ($1, 1, 1, $2, $3, NOW())
		ON CONFLICT (agent_addr) DO UPDATE SET
			matches_won = app_agent_stats.matches_won + 1,
			total_earned_mon = (COALESCE(app_agent_stats.total_earned_mon::numeric, 0) + $2::numeric)::text,
			total_earned_neuron = (COALESCE(app_agent_stats.total_earned_neuron::numeric, 0) + $3::numeric)::text,
			last_active = NOW()`,
		agentAddr, earnedMON, earnedNeuron,
	)
	return err
}

// RecordAnswer records an answer attempt.
func (r *AgentRepository) RecordAnswer(ctx context.Context, agentAddr string, isCorrect bool, neuronBurned string) error {
	correctInc := 0
	wrongInc := 0
	if isCorrect {
		correctInc = 1
	} else {
		wrongInc = 1
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_agent_stats (agent_addr, correct_answers, wrong_answers, total_burned_neuron, last_active)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (agent_addr) DO UPDATE SET
			correct_answers = app_agent_stats.correct_answers + $2,
			wrong_answers = app_agent_stats.wrong_answers + $3,
			total_burned_neuron = (COALESCE(app_agent_stats.total_burned_neuron::numeric, 0) + $4::numeric)::text,
			last_active = NOW()`,
		agentAddr, correctInc, wrongInc, neuronBurned,
	)
	return err
}

// RollbackWin reverses a match win for an agent (used when cancelling a previously-settled match).
func (r *AgentRepository) RollbackWin(ctx context.Context, agentAddr string, earnedMON string, earnedNeuron string) error {
	if earnedMON == "" {
		earnedMON = "0"
	}
	if earnedNeuron == "" {
		earnedNeuron = "0"
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE app_agent_stats SET
			matches_won = GREATEST(matches_won - 1, 0),
			total_earned_mon = GREATEST((COALESCE(total_earned_mon::numeric, 0) - $2::numeric), 0)::text,
			total_earned_neuron = GREATEST((COALESCE(total_earned_neuron::numeric, 0) - $3::numeric), 0)::text,
			last_active = NOW()
		WHERE LOWER(agent_addr) = LOWER($1)`,
		agentAddr, earnedMON, earnedNeuron,
	)
	return err
}

// DecrementMatchesPlayed decrements the matches played count.
func (r *AgentRepository) DecrementMatchesPlayed(ctx context.Context, agentAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_agent_stats SET
			matches_played = GREATEST(matches_played - 1, 0),
			last_active = NOW()
		WHERE LOWER(agent_addr) = LOWER($1)`,
		agentAddr,
	)
	return err
}

// GetTotalEarnings returns the sum of all agents' total_earned_mon.
func (r *AgentRepository) GetTotalEarnings(ctx context.Context) (string, error) {
	var total sql.NullString
	err := r.db.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(total_earned_mon::numeric), 0)::text FROM app_agent_stats`,
	)
	if err != nil {
		return "0", err
	}
	if !total.Valid {
		return "0", nil
	}
	return total.String, nil
}

// GetLeaderboard retrieves the top agents.
func (r *AgentRepository) GetLeaderboard(ctx context.Context, params models.LeaderboardParams) ([]models.LeaderboardEntry, error) {
	orderBy := "matches_won DESC"
	switch params.SortBy {
	case "earnings":
		orderBy = "total_earned_mon::numeric DESC"
	case "accuracy":
		orderBy = "CASE WHEN correct_answers + wrong_answers > 0 THEN correct_answers::float / (correct_answers + wrong_answers) ELSE 0 END DESC"
	case "burned":
		orderBy = "total_burned_neuron::numeric DESC"
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}

	var stats []models.AgentStats
	err := r.db.SelectContext(ctx, &stats,
		`SELECT * FROM app_agent_stats ORDER BY `+orderBy+` LIMIT $1 OFFSET $2`,
		limit, params.Offset,
	)
	if err != nil {
		return nil, err
	}

	entries := make([]models.LeaderboardEntry, len(stats))
	for i, s := range stats {
		winRate := 0.0
		if s.MatchesPlayed > 0 {
			winRate = float64(s.MatchesWon) / float64(s.MatchesPlayed)
		}
		entries[i] = models.LeaderboardEntry{
			Rank:              params.Offset + i + 1,
			AgentAddr:         s.AgentAddr,
			MatchesWon:        s.MatchesWon,
			MatchesPlayed:     s.MatchesPlayed,
			WinRate:           winRate,
			TotalEarnedMON:    s.TotalEarnedMON,
			TotalBurnedNeuron: s.TotalBurnedNeuron,
		}
	}

	return entries, nil
}

// CountTotal counts total unique agents.
func (r *AgentRepository) CountTotal(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM app_agent_stats`)
	return count, err
}

// UpdateLastActive updates the last active timestamp.
func (r *AgentRepository) UpdateLastActive(ctx context.Context, agentAddr string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE app_agent_stats SET last_active = $2 WHERE agent_addr = $1`,
		agentAddr, time.Now(),
	)
	return err
}

// AddBurnedNeuron adds to the total burned neuron for an agent.
func (r *AgentRepository) AddBurnedNeuron(ctx context.Context, agentAddr string, amount string) error {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		amountBig = big.NewInt(0)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_agent_stats (agent_addr, total_burned_neuron, last_active)
		VALUES ($1, $2, NOW())
		ON CONFLICT (agent_addr) DO UPDATE SET
			total_burned_neuron = (COALESCE(app_agent_stats.total_burned_neuron::numeric, 0) + $2::numeric)::text,
			last_active = NOW()`,
		agentAddr, amountBig.String(),
	)
	return err
}
