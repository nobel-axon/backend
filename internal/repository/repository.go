// Package repository provides data access layer for axon-server.
package repository

import (
	"github.com/axon-arena/axon-server/internal/db"
)

// Repositories contains all repository instances.
type Repositories struct {
	Matches       *MatchRepository
	Answers       *AnswerRepository
	Agents        *AgentRepository
	Commentary    *CommentaryRepository
	Burns         *BurnRepository
	Poller        *PollerRepository
	Personalities *PersonalityRepository
	ChainEvents   *ChainEventRepository
}

// NewRepositories creates all repository instances.
func NewRepositories(database *db.DB) *Repositories {
	return &Repositories{
		Matches:       NewMatchRepository(database),
		Answers:       NewAnswerRepository(database),
		Agents:        NewAgentRepository(database),
		Commentary:    NewCommentaryRepository(database),
		Burns:         NewBurnRepository(database),
		Poller:        NewPollerRepository(database),
		Personalities: NewPersonalityRepository(database),
		ChainEvents:   NewChainEventRepository(database),
	}
}
