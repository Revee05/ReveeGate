package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	redisRepo "github.com/reveegate/reveegate/internal/repository/redis"
)

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients by channel
	clients map[string]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast message to specific channel
	broadcast chan *BroadcastMessage

	// Pubsub for receiving events from other services
	pubsub *redisRepo.PubSub

	// Logger
	logger *slog.Logger

	// Mutex for thread-safe client access
	mu sync.RWMutex

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	Channel string
	Message []byte
}

// NewHub creates a new Hub
func NewHub(pubsub *redisRepo.PubSub, logger *slog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *BroadcastMessage, 256),
		pubsub:     pubsub,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run starts the hub
func (h *Hub) Run() {
	// Start Redis subscription
	go h.subscribeToEvents()

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Info("hub shutting down")
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case msg := <-h.broadcast:
			h.broadcastToChannel(msg)
		}
	}
}

// Stop gracefully stops the hub
func (h *Hub) Stop() {
	h.cancel()
}

// registerClient adds a client to a channel
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.channel] == nil {
		h.clients[client.channel] = make(map[*Client]bool)
	}
	h.clients[client.channel][client] = true

	h.logger.Debug("client registered",
		"channel", client.channel,
		"client_id", client.id,
		"clients_in_channel", len(h.clients[client.channel]),
	)
}

// unregisterClient removes a client from its channel
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.channel]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send)

			if len(clients) == 0 {
				delete(h.clients, client.channel)
			}

			h.logger.Debug("client unregistered",
				"channel", client.channel,
				"client_id", client.id,
			)
		}
	}
}

// broadcastToChannel sends message to all clients in a channel
func (h *Hub) broadcastToChannel(msg *BroadcastMessage) {
	h.mu.RLock()
	clients, ok := h.clients[msg.Channel]
	h.mu.RUnlock()

	if !ok {
		return
	}

	for client := range clients {
		select {
		case client.send <- msg.Message:
		default:
			// Client buffer full, unregister
			h.unregister <- client
		}
	}
}

// BroadcastDonation broadcasts a donation event to overlay clients
func (h *Hub) BroadcastDonation(event *redisRepo.DonationEvent) {
	msg := OutgoingMessage{
		Type:      "donation",
		Timestamp: time.Now().Format(time.RFC3339),
		Data: map[string]interface{}{
			"id":         event.ID,
			"donor_name": event.DonorName,
			"message":    event.Message,
			"amount":     event.Amount,
			"paid_at":    event.PaidAt,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal donation event", "error", err)
		return
	}

	// Broadcast to all overlay channels
	h.mu.RLock()
	defer h.mu.RUnlock()

	for channel := range h.clients {
		if len(channel) > 8 && channel[:8] == "overlay:" {
			h.broadcast <- &BroadcastMessage{
				Channel: channel,
				Message: data,
			}
		}
	}
}

// subscribeToEvents subscribes to Redis events
func (h *Hub) subscribeToEvents() {
	err := h.pubsub.SubscribeDonations(h.ctx, func(event *redisRepo.DonationEvent) {
		h.BroadcastDonation(event)
	})

	if err != nil {
		h.logger.Error("failed to subscribe to donations", "error", err)
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	channelStats := make(map[string]int)

	for channel, clients := range h.clients {
		count := len(clients)
		channelStats[channel] = count
		totalClients += count
	}

	return map[string]interface{}{
		"total_clients":  totalClients,
		"total_channels": len(h.clients),
		"channels":       channelStats,
	}
}

// GetClientCount returns the number of clients in a channel
func (h *Hub) GetClientCount(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[channel]; ok {
		return len(clients)
	}
	return 0
}
