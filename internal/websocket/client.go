// Package websocket provides WebSocket server functionality for axon-server.
package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/axon-arena/axon-server/internal/config"
)

// Maximum message size allowed from peer.
const maxMessageSize = 512

// LobbyHandler interface for handling lobby messages.
type LobbyHandler interface {
	SignalInterest(agentAddr string)
	LeaveInterest(agentAddr string)
	Heartbeat(agentAddr string)
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Timing configuration
	writeWait  time.Duration
	pongWait   time.Duration
	pingPeriod time.Duration

	// Lobby handler for processing lobby messages
	lobbyHandler LobbyHandler

	// Agent address for this client (if lobby participant)
	agentAddr string
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.onDisconnect()
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Process incoming messages for lobby functionality
		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// clientMessage represents an incoming WebSocket message.
type clientMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// handleMessage processes incoming WebSocket messages.
func (c *Client) handleMessage(message []byte) {
	var msg clientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return // Ignore malformed messages
	}

	// Skip if no lobby handler
	if c.lobbyHandler == nil {
		return
	}

	switch msg.Type {
	case EventLobbyJoin:
		if addr, ok := msg.Data["agentAddr"].(string); ok && addr != "" {
			c.agentAddr = addr
			c.lobbyHandler.SignalInterest(addr)
		}
	case EventLobbyLeave:
		if c.agentAddr != "" {
			c.lobbyHandler.LeaveInterest(c.agentAddr)
			c.agentAddr = ""
		}
	case EventLobbyHeartbeat:
		if c.agentAddr != "" {
			c.lobbyHandler.Heartbeat(c.agentAddr)
		}
	}
}

// onDisconnect handles cleanup when the client disconnects.
func (c *Client) onDisconnect() {
	// Remove from lobby if participating
	if c.lobbyHandler != nil && c.agentAddr != "" {
		c.lobbyHandler.LeaveInterest(c.agentAddr)
	}
}

// ServeWS handles websocket requests from the peer.
func ServeWS(hub *Hub, cfg config.WebSocketConfig, w http.ResponseWriter, r *http.Request) {
	ServeWSWithLobby(hub, cfg, nil, w, r)
}

// ServeWSWithLobby handles websocket requests with optional lobby support.
func ServeWSWithLobby(hub *Hub, cfg config.WebSocketConfig, lobbyHandler LobbyHandler, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Calculate ping period as 90% of pong timeout
	pingPeriod := (cfg.PongTimeout * 9) / 10

	client := &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, 256),
		writeWait:    cfg.WriteTimeout,
		pongWait:     cfg.PongTimeout,
		pingPeriod:   pingPeriod,
		lobbyHandler: lobbyHandler,
	}

	hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}
