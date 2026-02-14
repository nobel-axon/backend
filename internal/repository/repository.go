// Package repository provides data access layer for axon-server.
package repository

import (
	"github.com/axon-arena/axon-server/internal/db"
)

// Repositories contains all repository instances.
type Repositories struct {
	database      *db.DB
	Matches       *MatchRepository
	Answers       *AnswerRepository
	Agents        *AgentRepository
	Commentary    *CommentaryRepository
	Burns         *BurnRepository
	Poller        *PollerRepository
	Personalities *PersonalityRepository
	ChainEvents   *ChainEventRepository
	Bounties      *BountyRepository
	BountyAnswers *BountyAnswerRepository
	BountyClaims  *BountyClaimsRepository
	MatchRefunds  *MatchRefundRepository
}

// DB returns the underlying database connection.
func (r *Repositories) DB() *db.DB {
	return r.database
}

// NewRepositories creates all repository instances.
func NewRepositories(database *db.DB) *Repositories {
	return &Repositories{
		database:      database,
		Matches:       NewMatchRepository(database),
		Answers:       NewAnswerRepository(database),
		Agents:        NewAgentRepository(database),
		Commentary:    NewCommentaryRepository(database),
		Burns:         NewBurnRepository(database),
		Poller:        NewPollerRepository(database),
		Personalities: NewPersonalityRepository(database),
		ChainEvents:   NewChainEventRepository(database),
		Bounties:      NewBountyRepository(database),
		BountyAnswers: NewBountyAnswerRepository(database),
		BountyClaims:  NewBountyClaimsRepository(database),
		MatchRefunds:  NewMatchRefundRepository(database),
	}
}
