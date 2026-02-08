// Package poller provides event detection and reporting functionality.
package poller

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/axon-arena/axon-server/internal/chief"
	"github.com/axon-arena/axon-server/internal/config"
	"github.com/axon-arena/axon-server/internal/db"
	"github.com/axon-arena/axon-server/internal/repository"
)

// Poller detects time-based events by polling the database and reports them to Chief.
// Chain event forwarding (agent_joined_queue, answer_submitted) is handled by webhook
// handlers in internal.go — the indexer POSTs directly and the handler forwards to Chief.
type Poller struct {
	cfg         *config.Config
	db          *db.DB
	repos       *repository.Repositories
	chiefClient *chief.Client

	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool
}

// NewPoller creates a new event poller.
func NewPoller(cfg *config.Config, database *db.DB, repos *repository.Repositories, chiefClient *chief.Client) *Poller {
	return &Poller{
		cfg:         cfg,
		db:          database,
		repos:       repos,
		chiefClient: chiefClient,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the polling loop.
func (p *Poller) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	p.wg.Add(1)
	go p.run(ctx)
}

// Stop stops the polling loop.
func (p *Poller) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	close(p.stopCh)
	p.wg.Wait()
}

func (p *Poller) run(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cfg.Poller.GetInterval())
	defer ticker.Stop()

	log.Println("Event poller started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Event poller stopping (context cancelled)")
			return
		case <-p.stopCh:
			log.Println("Event poller stopping (stop signal)")
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	// Time-based detectors only — chain event forwarding (agent_joined_queue,
	// answer_submitted) is now handled by the webhook handlers in internal.go.
	p.detectRegistrationClosed(ctx)
	p.detectTimeouts(ctx)
}

// detectRegistrationClosed finds matches that have passed their registration deadline.
func (p *Poller) detectRegistrationClosed(ctx context.Context) {
	matches, err := p.repos.Matches.GetMatchesExpiredRegistration(ctx)
	if err != nil {
		log.Printf("Poller: failed to detect registration closed: %v", err)
		return
	}

	for _, match := range matches {
		log.Printf("Poller: detected registration closed for match %d", match.MatchID)

		// Mark as reported BEFORE sending to Chief to prevent duplicate sends.
		// If the mark fails (e.g. already marked by a concurrent poll cycle), skip this match.
		if err := p.repos.Matches.MarkRegistrationReported(ctx, match.MatchID); err != nil {
			log.Printf("Poller: failed to mark registration reported for match %d (skipping): %v", match.MatchID, err)
			continue
		}

		// Report to Chief
		_, err := p.chiefClient.ReportEvent(ctx, chief.EventTypeRegistrationClosed, match.MatchID, map[string]interface{}{
			"playerCount": match.PlayerCount,
		})
		if err != nil {
			log.Printf("Poller: failed to report registration closed for match %d: %v", match.MatchID, err)
			// Already marked — Chief will not get this event. On next poll the match
			// won't appear in GetMatchesExpiredRegistration because it's already marked.
			// This is acceptable: the stale watcher on Chief is the safety net.
			continue
		}

		log.Printf("Poller: reported registration_closed for match %d to Chief", match.MatchID)
	}
}

// detectTimeouts finds matches that have passed their answer deadline.
func (p *Poller) detectTimeouts(ctx context.Context) {
	matches, err := p.repos.Matches.GetMatchesExpiredAnswer(ctx)
	if err != nil {
		log.Printf("Poller: failed to detect timeouts: %v", err)
		return
	}

	for _, match := range matches {
		log.Printf("Poller: detected answer timeout for match %d", match.MatchID)

		// Mark as reported BEFORE sending to Chief to prevent duplicate sends.
		// If the mark fails (e.g. already marked by a concurrent poll cycle), skip this match.
		if err := p.repos.Matches.MarkTimeoutReported(ctx, match.MatchID); err != nil {
			log.Printf("Poller: failed to mark timeout reported for match %d (skipping): %v", match.MatchID, err)
			continue
		}

		// Report to Chief
		_, err := p.chiefClient.ReportEvent(ctx, chief.EventTypeAnswerTimeout, match.MatchID, map[string]interface{}{})
		if err != nil {
			log.Printf("Poller: failed to report timeout for match %d: %v", match.MatchID, err)
			// Already marked — stale watcher on Chief is the safety net.
			continue
		}

		log.Printf("Poller: reported answer_timeout for match %d to Chief", match.MatchID)
	}
}

// isTableNotFoundError checks if the error indicates a missing table.
// This happens if the Ponder indexer hasn't created the tables yet.
func isTableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist")
}

// IsRunning returns whether the poller is running.
func (p *Poller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}
