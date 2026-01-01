package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	App       AppConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Midtrans  MidtransConfig
	Xendit    XenditConfig
	Overlay   OverlayConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Env         string
	Environment string // Alias for Env
	Version     string
	Port        string
	URL         string
	Debug       bool
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	URL             string
	Host            string
	Name            string
	MaxConns        int
	MinConns        int
	MaxOpenConns    int // Alias for MaxConns
	MaxIdleConns    int // Alias for MinConns
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	ConnMaxLifetime time.Duration // Alias
	ConnMaxIdleTime time.Duration // Alias
}

// DSN returns the database connection string
func (d DatabaseConfig) DSN() string {
	return d.URL
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL          string
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// MidtransConfig holds Midtrans payment provider configuration
type MidtransConfig struct {
	ServerKey    string
	ClientKey    string
	MerchantID   string
	APIURL       string
	IPWhitelist  []string
	IsProduction bool
}

// XenditConfig holds Xendit payment provider configuration
type XenditConfig struct {
	SecretKey    string
	PublicKey    string
	APIKey       string
	WebhookToken string
	IPWhitelist  []string
}

// OverlayConfig holds OBS overlay configuration
type OverlayConfig struct {
	Token string
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins []string
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	DonationPerMinute int
	APIPerMinute      int
	WebhookPerMinute  int
	AdminPerMinute    int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	env := getEnv("APP_ENV", "development")
	maxConns := getEnvInt("DATABASE_MAX_CONNS", 25)
	minConns := getEnvInt("DATABASE_MIN_CONNS", 5)
	maxConnLifetime := getEnvDuration("DATABASE_MAX_CONN_LIFETIME", 30*time.Minute)
	maxConnIdleTime := getEnvDuration("DATABASE_MAX_CONN_IDLE_TIME", 5*time.Minute)

	cfg := &Config{
		App: AppConfig{
			Env:         env,
			Environment: env,
			Version:     getEnv("APP_VERSION", "1.0.0"),
			Port:        getEnv("APP_PORT", "8080"),
			URL:         getEnv("APP_URL", "http://localhost:8080"),
			Debug:       getEnvBool("APP_DEBUG", true),
		},
		Database: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://reveegate:password@localhost:5432/reveegate?sslmode=disable"),
			Host:            getEnv("DATABASE_HOST", "localhost"),
			Name:            getEnv("DATABASE_NAME", "reveegate"),
			MaxConns:        maxConns,
			MinConns:        minConns,
			MaxOpenConns:    maxConns,
			MaxIdleConns:    minConns,
			MaxConnLifetime: maxConnLifetime,
			MaxConnIdleTime: maxConnIdleTime,
			ConnMaxLifetime: maxConnLifetime,
			ConnMaxIdleTime: maxConnIdleTime,
		},
		Redis: RedisConfig{
			URL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "your-super-secret-jwt-key-min-32-chars"),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		},
		Midtrans: MidtransConfig{
			ServerKey:    getEnv("MIDTRANS_SERVER_KEY", ""),
			ClientKey:    getEnv("MIDTRANS_CLIENT_KEY", ""),
			MerchantID:   getEnv("MIDTRANS_MERCHANT_ID", ""),
			APIURL:       getEnv("MIDTRANS_API_URL", "https://api.sandbox.midtrans.com"),
			IPWhitelist:  getEnvSlice("MIDTRANS_IP_WHITELIST", []string{"103.127.16.0/23", "103.208.23.0/24"}),
			IsProduction: getEnv("APP_ENV", "development") == "production",
		},
		Xendit: XenditConfig{
			SecretKey:    getEnv("XENDIT_SECRET_KEY", ""),
			PublicKey:    getEnv("XENDIT_PUBLIC_KEY", ""),
			APIKey:       getEnv("XENDIT_API_KEY", ""),
			WebhookToken: getEnv("XENDIT_WEBHOOK_TOKEN", ""),
			IPWhitelist:  getEnvSlice("XENDIT_IP_WHITELIST", []string{"18.139.71.0/24", "13.229.120.0/24"}),
		},
		Overlay: OverlayConfig{
			Token: getEnv("OVERLAY_TOKEN", ""),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		},
		RateLimit: RateLimitConfig{
			DonationPerMinute: getEnvInt("RATE_LIMIT_DONATION", 10),
			APIPerMinute:      getEnvInt("RATE_LIMIT_API", 100),
			WebhookPerMinute:  getEnvInt("RATE_LIMIT_WEBHOOK", 1000),
			AdminPerMinute:    getEnvInt("RATE_LIMIT_ADMIN", 300),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.JWT.Secret == "" || len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	return nil
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// Helper functions for environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		// Try parsing as seconds
		if i, err := strconv.Atoi(value); err == nil {
			return time.Duration(i) * time.Second
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}
