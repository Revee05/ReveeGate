package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/reveegate/reveegate/internal/repository/redis"
)

// RateLimiter middleware handles rate limiting
type RateLimiter struct {
	cache         *redis.Cache
	requestsLimit int
	windowSize    time.Duration
}

// NewRateLimiter creates a new rate limiter middleware
func NewRateLimiter(cache *redis.Cache, requestsLimit int, windowSize time.Duration) *RateLimiter {
	return &RateLimiter{
		cache:         cache,
		requestsLimit: requestsLimit,
		windowSize:    windowSize,
	}
}

// Limit returns a middleware that rate limits requests
func (rl *RateLimiter) Limit(endpoint string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			key := redis.RateLimitKey(ip, endpoint)

			// Increment counter with TTL
			count, err := rl.cache.IncrementWithTTL(r.Context(), key, rl.windowSize)
			if err != nil {
				// If Redis fails, allow the request but log error
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requestsLimit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(rl.requestsLimit)-count)))

			// Check if rate limit exceeded
			if count > int64(rl.requestsLimit) {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.windowSize.Seconds())))
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitConfig holds rate limit configuration for different endpoints
type RateLimitConfig struct {
	DonationLimit int           // requests per minute for donation endpoints
	APILimit      int           // requests per minute for general API
	WebhookLimit  int           // requests per minute for webhook endpoints
	AdminLimit    int           // requests per minute for admin endpoints
	WindowSize    time.Duration // time window for rate limiting
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DonationLimit: 10,
		APILimit:      100,
		WebhookLimit:  1000,
		AdminLimit:    300,
		WindowSize:    time.Minute,
	}
}

// RateLimitMiddleware creates rate limiters for different endpoints
type RateLimitMiddleware struct {
	donation *RateLimiter
	api      *RateLimiter
	webhook  *RateLimiter
	admin    *RateLimiter
}

// NewRateLimitMiddleware creates a new rate limit middleware set
func NewRateLimitMiddleware(cache *redis.Cache, config RateLimitConfig) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		donation: NewRateLimiter(cache, config.DonationLimit, config.WindowSize),
		api:      NewRateLimiter(cache, config.APILimit, config.WindowSize),
		webhook:  NewRateLimiter(cache, config.WebhookLimit, config.WindowSize),
		admin:    NewRateLimiter(cache, config.AdminLimit, config.WindowSize),
	}
}

// Donation returns the donation rate limiter middleware
func (rlm *RateLimitMiddleware) Donation() func(http.Handler) http.Handler {
	return rlm.donation.Limit("donation")
}

// API returns the general API rate limiter middleware
func (rlm *RateLimitMiddleware) API() func(http.Handler) http.Handler {
	return rlm.api.Limit("api")
}

// Webhook returns the webhook rate limiter middleware
func (rlm *RateLimitMiddleware) Webhook() func(http.Handler) http.Handler {
	return rlm.webhook.Limit("webhook")
}

// Admin returns the admin rate limiter middleware
func (rlm *RateLimitMiddleware) Admin() func(http.Handler) http.Handler {
	return rlm.admin.Limit("admin")
}

// IPWhitelistMiddleware checks if the request IP is whitelisted
func IPWhitelistMiddleware(whitelist []string) func(http.Handler) http.Handler {
	// Convert CIDR notation to list of IPs/ranges
	allowedIPs := make(map[string]bool)
	for _, ip := range whitelist {
		allowedIPs[ip] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)

			// Check if IP is whitelisted
			if !isIPWhitelisted(clientIP, whitelist) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isIPWhitelisted checks if an IP is in the whitelist
func isIPWhitelisted(ip string, whitelist []string) bool {
	// For simplicity, just check exact match
	// In production, should handle CIDR notation
	for _, allowed := range whitelist {
		if ip == allowed {
			return true
		}
		// Simple CIDR check (basic implementation)
		if len(allowed) > 0 && allowed[len(allowed)-1] == '*' {
			prefix := allowed[:len(allowed)-1]
			if len(ip) >= len(prefix) && ip[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// TimeoutMiddleware adds a timeout to the request context
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
