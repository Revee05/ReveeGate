package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"

	"github.com/reveegate/reveegate/internal/config"
	"github.com/reveegate/reveegate/internal/http/handler"
	"github.com/reveegate/reveegate/internal/http/middleware"
	"github.com/reveegate/reveegate/internal/realtime/websocket"
	postgresRepo "github.com/reveegate/reveegate/internal/repository/postgres"
	redisRepo "github.com/reveegate/reveegate/internal/repository/redis"
	"github.com/reveegate/reveegate/internal/service"
)

// Server represents the HTTP server
type Server struct {
	router *chi.Mux
	config *config.Config
	logger *slog.Logger
	wsHub  *websocket.Hub
}

// NewServer creates a new HTTP server
func NewServer(
	cfg *config.Config,
	donationService *service.DonationService,
	adminRepo *postgresRepo.AdminRepository,
	authMiddleware *middleware.Auth,
	cache *redisRepo.Cache,
	wsHub *websocket.Hub,
	logger *slog.Logger,
) *Server {
	router := chi.NewRouter()
	validator := validator.New()

	server := &Server{
		router: router,
		config: cfg,
		logger: logger,
		wsHub:  wsHub,
	}

	// Create handlers
	donationHandler := handler.NewDonationHandler(donationService, validator, logger)
	webhookHandler := handler.NewWebhookHandler(donationService, nil, cfg, logger)
	adminHandler := handler.NewAdminHandler(donationService, adminRepo, authMiddleware, validator, logger)
	wsHandler := websocket.NewHandler(wsHub, authMiddleware, logger)

	// Setup middleware
	server.setupMiddleware(cfg, cache, logger)

	// Setup routes
	server.setupRoutes(donationHandler, webhookHandler, adminHandler, wsHandler, authMiddleware)

	return server
}

// setupMiddleware configures global middleware
func (s *Server) setupMiddleware(cfg *config.Config, cache *redisRepo.Cache, logger *slog.Logger) {
	// Built-in chi middleware
	s.router.Use(chimiddleware.RequestID)
	s.router.Use(chimiddleware.RealIP)
	s.router.Use(chimiddleware.Recoverer)
	s.router.Use(chimiddleware.Timeout(30 * time.Second))

	// Custom middleware
	s.router.Use(middleware.Logger(logger))

	// Convert config.CORSConfig to middleware.CORSConfig
	corsConfig := middleware.CORSConfig{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
	s.router.Use(middleware.CORS(corsConfig))
	s.router.Use(middleware.Security())

	// Rate limiting is applied per-route, not globally
}

// setupRoutes configures all routes
func (s *Server) setupRoutes(
	donationHandler *handler.DonationHandler,
	webhookHandler *handler.WebhookHandler,
	adminHandler *handler.AdminHandler,
	wsHandler *websocket.Handler,
	authMiddleware *middleware.Auth,
) {
	// Health check (no rate limit)
	s.router.Get("/health", s.healthCheck)
	s.router.Get("/ready", s.readinessCheck)

	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Public donation routes
		r.Route("/donations", func(r chi.Router) {
			r.Post("/", donationHandler.Create)
			r.Get("/{id}", donationHandler.GetByID)
			r.Get("/{id}/status", donationHandler.GetStatus)
		})

		// Webhook routes (no rate limit, signature verified)
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/midtrans", webhookHandler.HandleMidtrans)
			r.Post("/xendit", webhookHandler.HandleXendit)
			r.Post("/verify", webhookHandler.VerifyWebhook)

			// Development only
			if s.config.App.Environment != "production" {
				r.Post("/simulate", webhookHandler.SimulatePaidWebhook)
			}
		})

		// Admin routes (protected)
		r.Route("/admin", func(r chi.Router) {
			// Public admin routes
			r.Post("/login", adminHandler.Login)
			r.Post("/refresh", adminHandler.RefreshToken)

			// Protected admin routes
			r.Group(func(r chi.Router) {
				r.Use(authMiddleware.Middleware())

				r.Get("/dashboard", adminHandler.GetDashboard)
				r.Get("/donations", donationHandler.List)
				r.Get("/donations/stats", donationHandler.GetStats)
				r.Post("/reconcile", adminHandler.ReconcilePayment)
				r.Post("/overlay-token", adminHandler.GenerateOverlayToken)
				r.Get("/webhook-logs", adminHandler.GetWebhookLogs)
				r.Get("/health", adminHandler.GetSystemHealth)
			})
		})
	})

	// WebSocket routes
	s.router.Route("/ws", func(r chi.Router) {
		// Overlay WebSocket (token auth via query param)
		r.Get("/overlay", wsHandler.HandleOverlay)

		// Admin WebSocket (JWT auth)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Middleware())
			r.Get("/admin", wsHandler.HandleAdmin)
		})
	})

	// Static file serving for web assets
	s.router.Route("/", func(r chi.Router) {
		// Redirect root to donor page
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/donate", http.StatusFound)
		})

		// Serve donor page
		r.Get("/donate", s.serveDonorPage)
		r.Get("/donate/{streamer}", s.serveDonorPage)

		// Serve overlay page
		r.Get("/overlay", s.serveOverlayPage)
		r.Get("/overlay/{token}", s.serveOverlayPage)

		// Serve admin page (static UI)
		r.Get("/admin", s.serveAdminPage)

		// Serve static assets
		fileServer := http.FileServer(http.Dir("./web/static"))
		r.Handle("/static/*", http.StripPrefix("/static/", fileServer))
	})
}

// serveAdminPage serves the admin UI
func (s *Server) serveAdminPage(w http.ResponseWriter, r *http.Request) {
	// Serve admin index.html
	http.ServeFile(w, r, "./web/admin/index.html")
}

// Handler returns the HTTP handler
func (s *Server) Handler() http.Handler {
	return s.router
}

// healthCheck handles GET /health
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// readinessCheck handles GET /ready
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	// TODO: Check database and redis connectivity
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// serveDonorPage serves the donor page
func (s *Server) serveDonorPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/donor/index.html")
}

// serveOverlayPage serves the overlay page
func (s *Server) serveOverlayPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/overlay/index.html")
}
