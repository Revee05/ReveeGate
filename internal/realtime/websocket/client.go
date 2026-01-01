package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 4096 // 4KB

	// Size of client send channel buffer
	sendBufferSize = 256
)

// Client represents a WebSocket client
type Client struct {
	// Unique client ID
	id string

	// Channel this client is subscribed to
	channel string

	// WebSocket connection
	conn *websocket.Conn

	// Hub reference
	hub *Hub

	// Buffered channel of outbound messages
	send chan []byte

	// Logger
	logger *slog.Logger

	// Client type (overlay, admin)
	clientType string

	// User identifier (token or user ID)
	userID string
}

// IncomingMessage represents a message from client
type IncomingMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// OutgoingMessage represents a message to client
type OutgoingMessage struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, hub *Hub, channel, clientType, userID string, logger *slog.Logger) *Client {
	return &Client{
		id:         uuid.New().String(),
		channel:    channel,
		conn:       conn,
		hub:        hub,
		send:       make(chan []byte, sendBufferSize),
		logger:     logger,
		clientType: clientType,
		userID:     userID,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn("websocket read error",
					"client_id", c.id,
					"error", err,
				)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(data []byte) {
	var msg IncomingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Warn("invalid message format",
			"client_id", c.id,
			"error", err,
		)
		c.sendError("Invalid message format")
		return
	}

	switch msg.Type {
	case "ping":
		c.sendPong()
	case "subscribe":
		// Already subscribed on connect
		c.sendAck("subscribed")
	default:
		c.logger.Debug("unknown message type",
			"client_id", c.id,
			"type", msg.Type,
		)
	}
}

// sendPong sends a pong response
func (c *Client) sendPong() {
	msg := OutgoingMessage{
		Type:      "pong",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	c.sendJSON(msg)
}

// sendAck sends an acknowledgment
func (c *Client) sendAck(action string) {
	msg := OutgoingMessage{
		Type:      "ack",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: map[string]interface{}{
			"action": action,
		},
	}
	c.sendJSON(msg)
}

// sendError sends an error message
func (c *Client) sendError(message string) {
	msg := OutgoingMessage{
		Type:      "error",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: map[string]interface{}{
			"message": message,
		},
	}
	c.sendJSON(msg)
}

// sendJSON marshals and sends a JSON message
func (c *Client) sendJSON(msg OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error("failed to marshal message", "error", err)
		return
	}

	select {
	case c.send <- data:
	default:
		c.logger.Warn("client send buffer full", "client_id", c.id)
	}
}

// SendWelcome sends a welcome message to the client
func (c *Client) SendWelcome() {
	msg := OutgoingMessage{
		Type:      "welcome",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: map[string]interface{}{
			"client_id": c.id,
			"channel":   c.channel,
			"message":   "Connected to ReveeGate WebSocket",
		},
	}
	c.sendJSON(msg)
}
