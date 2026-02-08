// Package lobby provides matchmaking lobby functionality for pre-payment interest signaling.
package lobby

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/axon-arena/axon-server/internal/config"
	"github.com/axon-arena/axon-server/internal/websocket"
)

// Manager handles the interest signaling lobby.
type Manager struct {
	cfg *config.LobbyConfig
	hub *websocket.Hub

	mu        sync.RWMutex
	interests map[string]*Interest // agentAddr -> Interest

	// Track ready groups
	readyMu     sync.RWMutex
	readyGroups map[string]*ReadyGroup // groupID -> ReadyGroup

	// Last ready notification time (for cooldown)
	lastReadyTime time.Time
}

// NewManager creates a new lobby manager.
func NewManager(cfg *config.LobbyConfig, hub *websocket.Hub) *Manager {
	return &Manager{
		cfg:         cfg,
		hub:         hub,
		interests:   make(map[string]*Interest),
		readyGroups: make(map[string]*ReadyGroup),
	}
}

// Start starts the lobby manager background processes.
func (m *Manager) Start(ctx context.Context) {
	log.Printf("Lobby manager started (minPlayers: %d, maxPlayers: %d, staleTimeout: %ds)",
		m.cfg.MinPlayers, m.cfg.MaxPlayers, m.cfg.StaleTimeoutS)

	cleanupInterval := time.Duration(m.cfg.CleanupIntervalS) * time.Second
	if cleanupInterval == 0 {
		cleanupInterval = 10 * time.Second
	}

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Lobby manager stopped")
			return
		case <-ticker.C:
			m.cleanupStale()
			m.cleanupExpiredGroups()
		}
	}
}

// SignalInterest adds a player to the interest pool.
func (m *Manager) SignalInterest(agentAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Check if already interested
	if existing, ok := m.interests[agentAddr]; ok {
		existing.LastHeartbeat = now
		return
	}

	m.interests[agentAddr] = &Interest{
		AgentAddr:     agentAddr,
		SignaledAt:    now,
		LastHeartbeat: now,
	}

	log.Printf("Lobby: %s signaled interest (total: %d)", agentAddr, len(m.interests))

	// Broadcast update
	m.broadcastInterestUpdate()

	// Check if threshold reached
	m.checkThreshold()
}

// LeaveInterest removes a player from the interest pool.
func (m *Manager) LeaveInterest(agentAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.interests[agentAddr]; !ok {
		return
	}

	delete(m.interests, agentAddr)
	log.Printf("Lobby: %s left interest pool (total: %d)", agentAddr, len(m.interests))

	// Broadcast update
	m.broadcastInterestUpdate()
}

// Heartbeat updates the last heartbeat time for a player.
func (m *Manager) Heartbeat(agentAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if interest, ok := m.interests[agentAddr]; ok {
		interest.LastHeartbeat = time.Now()
	}
}

// GetStatus returns the current lobby status.
func (m *Manager) GetStatus() LobbyStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	players := make([]string, 0, len(m.interests))
	for addr := range m.interests {
		players = append(players, addr)
	}

	status := LobbyStatus{
		InterestCount: len(m.interests),
		Players:       players,
		MinPlayers:    m.cfg.MinPlayers,
		MaxPlayers:    m.cfg.MaxPlayers,
	}

	// Check for active ready group
	m.readyMu.RLock()
	for _, group := range m.readyGroups {
		if !group.IsExpired() {
			status.ReadyGroup = group
			break
		}
	}
	m.readyMu.RUnlock()

	return status
}

// GetReadyGroup returns a ready group by ID.
func (m *Manager) GetReadyGroup(groupID string) *ReadyGroup {
	m.readyMu.RLock()
	defer m.readyMu.RUnlock()

	return m.readyGroups[groupID]
}

// checkThreshold checks if the minimum player threshold is reached and triggers ready notification.
func (m *Manager) checkThreshold() {
	if len(m.interests) < m.cfg.MinPlayers {
		return
	}

	// Check cooldown
	cooldown := time.Duration(m.cfg.ReadyCooldownS) * time.Second
	if time.Since(m.lastReadyTime) < cooldown {
		return
	}

	// Create ready group
	players := make([]string, 0, len(m.interests))
	for addr := range m.interests {
		players = append(players, addr)
		if len(players) >= m.cfg.MaxPlayers {
			break
		}
	}

	groupID := generateGroupID()
	readyWindow := time.Duration(m.cfg.ReadyWindowS) * time.Second
	now := time.Now()

	group := &ReadyGroup{
		GroupID:   groupID,
		Players:   players,
		ExpiresAt: now.Add(readyWindow),
		CreatedAt: now,
	}

	m.readyMu.Lock()
	m.readyGroups[groupID] = group
	m.readyMu.Unlock()

	m.lastReadyTime = now

	log.Printf("Lobby: Ready group %s created with %d players (expires in %s)",
		groupID, len(players), readyWindow)

	// Broadcast ready event
	m.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventLobbyReady,
		Data: websocket.LobbyReadyData{
			GroupID:   groupID,
			Players:   players,
			ExpiresAt: group.ExpiresAt.Unix(),
			Message:   "Register on-chain within 2 minutes!",
		},
	})
}

// cleanupStale removes interests that haven't sent a heartbeat within the timeout.
func (m *Manager) cleanupStale() {
	m.mu.Lock()
	defer m.mu.Unlock()

	staleTimeout := time.Duration(m.cfg.StaleTimeoutS) * time.Second
	now := time.Now()
	removed := 0

	for addr, interest := range m.interests {
		if now.Sub(interest.LastHeartbeat) > staleTimeout {
			delete(m.interests, addr)
			removed++
			log.Printf("Lobby: %s removed (stale)", addr)
		}
	}

	if removed > 0 {
		m.broadcastInterestUpdate()
	}
}

// cleanupExpiredGroups removes expired ready groups.
func (m *Manager) cleanupExpiredGroups() {
	m.readyMu.Lock()
	defer m.readyMu.Unlock()

	for groupID, group := range m.readyGroups {
		if group.IsExpired() {
			delete(m.readyGroups, groupID)
			log.Printf("Lobby: Ready group %s expired", groupID)

			// Broadcast expiry
			m.hub.Broadcast(websocket.WSEvent{
				Type: websocket.EventLobbyGroupExpired,
				Data: websocket.LobbyGroupExpiredData{
					GroupID: groupID,
				},
			})
		}
	}
}

// broadcastInterestUpdate sends the current interest count to all clients.
// Must be called with m.mu held.
func (m *Manager) broadcastInterestUpdate() {
	players := make([]string, 0, len(m.interests))
	for addr := range m.interests {
		players = append(players, addr)
	}

	m.hub.Broadcast(websocket.WSEvent{
		Type: websocket.EventLobbyInterestUpdate,
		Data: websocket.LobbyInterestUpdateData{
			Count:   len(m.interests),
			Players: players,
		},
	})
}

// generateGroupID generates a random group ID.
func generateGroupID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
