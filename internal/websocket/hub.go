// Package websocket provides WebSocket server functionality for axon-server.
package websocket

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Hub maintains the set of active clients and broadcasts messages to clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients (for future use).
	broadcast chan WSEvent

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Mutex for thread-safe access to clients map.
	mu sync.RWMutex

	// Dedup: recent broadcast cache to suppress duplicate events from
	// Ponder indexer and Chief reporter hitting the same internal endpoints.
	recentEvents map[string]time.Time
	recentMu     sync.Mutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		broadcast:    make(chan WSEvent, 256),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		recentEvents: make(map[string]time.Time),
	}
}

// Run starts the hub main loop.
func (h *Hub) Run() {
	// Periodically evict stale dedup entries.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			h.recentMu.Lock()
			now := time.Now()
			for k, t := range h.recentEvents {
				if now.Sub(t) > 5*time.Second {
					delete(h.recentEvents, k)
				}
			}
			h.recentMu.Unlock()
		}
	}()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))

		case event := <-h.broadcast:
			message, err := event.ToJSON()
			if err != nil {
				log.Printf("Failed to serialize WebSocket event: %v", err)
				continue
			}

			h.mu.RLock()
			clientCount := len(h.clients)
			h.mu.RUnlock()
			log.Printf("WS broadcast: %s (clients: %d)", event.Type, clientCount)

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's buffer is full, schedule for removal
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends an event to all connected clients.
// Deduplicates events with the same {type, matchId} (plus agent for answer
// events) within a 5-second window. Commentary and lobby events are never
// deduped since they can legitimately repeat.
func (h *Hub) Broadcast(event WSEvent) {
	key := h.dedupKey(event)
	if key != "" {
		h.recentMu.Lock()
		if t, ok := h.recentEvents[key]; ok && time.Since(t) < 5*time.Second {
			h.recentMu.Unlock()
			log.Printf("WS dedup: skipping duplicate %s", key)
			return
		}
		h.recentEvents[key] = time.Now()
		h.recentMu.Unlock()
	}
	h.broadcast <- event
}

// dedupKey builds a dedup key for an event. Returns "" for events that should
// never be deduped (commentary, lobby_*).
func (h *Hub) dedupKey(event WSEvent) string {
	// Never dedup commentary or lobby events.
	switch event.Type {
	case "commentary", "lobby_interest_update", "lobby_ready", "lobby_group_expired":
		return ""
	}

	var matchID interface{}
	var agent interface{}

	switch d := event.Data.(type) {
	case map[string]interface{}:
		matchID = d["matchId"]
		// For answer events, include agent so different agents aren't deduped.
		if event.Type == "answer_submitted" || event.Type == "answer_verified" {
			if a, ok := d["agentAddr"]; ok {
				agent = a
			} else {
				agent = d["agent"]
			}
		}
	case AnswerSubmittedData:
		matchID = d.MatchID
		agent = d.AgentAddr
	case AnswerVerifiedData:
		matchID = d.MatchID
		agent = d.AgentAddr
	case MatchSettledData:
		matchID = d.MatchID
	case MatchCancelledData:
		matchID = d.MatchID
	case MatchCreatedData:
		matchID = d.MatchID
	case AgentRegisteredData:
		matchID = d.MatchID
		agent = d.AgentAddr
	case QuestionPostedData:
		matchID = d.MatchID
	case MatchTimeoutData:
		matchID = d.MatchID
	}

	if agent != nil {
		return fmt.Sprintf("%s:%v:%v", event.Type, matchID, agent)
	}
	return fmt.Sprintf("%s:%v", event.Type, matchID)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
