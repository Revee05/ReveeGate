package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/reveegate/reveegate/internal/config"
	httpServer "github.com/reveegate/reveegate/internal/http"
	"github.com/reveegate/reveegate/internal/http/middleware"
	"github.com/reveegate/reveegate/internal/provider/midtrans"
	"github.com/reveegate/reveegate/internal/realtime/websocket"
	postgresRepo "github.com/reveegate/reveegate/internal/repository/postgres"
	redisRepo "github.com/reveegate/reveegate/internal/repository/redis"
	"github.com/reveegate/reveegate/internal/service"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Set log level based on environment
	if cfg.App.Environment == "development" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		slog.SetDefault(logger)
	}

	logger.Info("starting ReveeGate",
		"version", cfg.App.Version,
		"environment", cfg.App.Environment,
	)

	// Initialize PostgreSQL connection pool
	ctx := context.Background()
	dbPool, err := initDatabase(ctx, cfg.Database, logger)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Initialize Redis client
	redisClient, err := initRedis(ctx, cfg.Redis, logger)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// Initialize repositories
	donationRepo := postgresRepo.NewDonationRepository(dbPool)
	paymentRepo := postgresRepo.NewPaymentRepository(dbPool)
	webhookLogRepo := postgresRepo.NewWebhookLogRepository(dbPool)
	adminRepo := postgresRepo.NewAdminRepository(dbPool)

	// Initialize Redis cache and pubsub
	cache := redisRepo.NewCache(redisClient)
	pubsub := redisRepo.NewPubSub(redisClient, logger)

	// Initialize payment provider (Midtrans by default)
	paymentProvider := midtrans.NewProvider(cfg.Midtrans)

	// Initialize services
	donationService := service.NewDonationService(
		donationRepo,
		paymentRepo,
		webhookLogRepo,
		paymentProvider,
		pubsub,
		cache,
		logger,
	)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(pubsub, logger)
	go wsHub.Run()

	// Initialize auth middleware
	authMiddleware := middleware.NewAuth(cfg.JWT, cache, logger)

	// Initialize HTTP server
	server := httpServer.NewServer(
		cfg,
		donationService,
		adminRepo,
		authMiddleware,
		cache,
		wsHub,
		logger,
	)

	// Create HTTP server
	httpSrv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      server.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Stop WebSocket hub
	wsHub.Stop()

	logger.Info("server stopped")
}

// initDatabase initializes PostgreSQL connection pool
func initDatabase(ctx context.Context, cfg config.DatabaseConfig, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, err
	}

	// Configure pool
	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime

	// Connect
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	// Ping to verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	logger.Info("database connected",
		"host", cfg.Host,
		"database", cfg.Name,
		"max_conns", cfg.MaxOpenConns,
	)

	return pool, nil
}

// initRedis initializes Redis client
func initRedis(ctx context.Context, cfg config.RedisConfig, logger *slog.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// Ping to verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	logger.Info("redis connected",
		"addr", cfg.Addr,
		"db", cfg.DB,
		"pool_size", cfg.PoolSize,
	)

	return client, nil
}
