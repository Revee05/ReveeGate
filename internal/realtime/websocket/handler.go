package websocket

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/reveegate/reveegate/internal/http/middleware"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin check based on config
		return true
	},
}

// Handler handles WebSocket connections
type Handler struct {
	hub            *Hub
	authMiddleware *middleware.Auth
	logger         *slog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, authMiddleware *middleware.Auth, logger *slog.Logger) *Handler {
	return &Handler{
		hub:            hub,
		authMiddleware: authMiddleware,
		logger:         logger,
	}
}

// HandleOverlay handles GET /ws/overlay
func (h *Handler) HandleOverlay(w http.ResponseWriter, r *http.Request) {
	// Get overlay token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing overlay token", http.StatusUnauthorized)
		return
	}

	// Validate overlay token
	valid, err := h.authMiddleware.ValidateOverlayToken(r.Context(), token)
	if err != nil || !valid {
		h.logger.Warn("invalid overlay token",
			"token", maskToken(token),
			"error", err,
		)
		http.Error(w, "Invalid overlay token", http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket",
			"error", err,
		)
		return
	}

	// Create client
	channel := "overlay:" + token
	client := NewClient(conn, h.hub, channel, "overlay", token, h.logger)

	// Register client
	h.hub.register <- client

	// Send welcome message
	client.SendWelcome()

	h.logger.Info("overlay client connected",
		"client_id", client.id,
		"channel", channel,
	)

	// Start goroutines
	go client.WritePump()
	go client.ReadPump()
}

// HandleAdmin handles GET /ws/admin
func (h *Handler) HandleAdmin(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (set by auth middleware)
	claims, ok := r.Context().Value(middleware.ClaimsContextKey{}).(*middleware.Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket",
			"error", err,
		)
		return
	}

	// Create client
	channel := "admin:" + claims.Subject
	client := NewClient(conn, h.hub, channel, "admin", claims.Subject, h.logger)

	// Register client
	h.hub.register <- client

	// Send welcome message
	client.SendWelcome()

	h.logger.Info("admin client connected",
		"client_id", client.id,
		"channel", channel,
		"user", claims.Subject,
	)

	// Start goroutines
	go client.WritePump()
	go client.ReadPump()
}

// maskToken masks a token for logging
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****"
}
