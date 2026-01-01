package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides Redis caching functionality
type Cache struct {
	client *redis.Client
}

// NewCache creates a new Redis cache client
func NewCache(client *redis.Client) *Cache {
	return &Cache{client: client}
}

// Set sets a value in the cache with TTL
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

// Get gets a value from the cache
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrCacheMiss
		}
		return fmt.Errorf("failed to get value: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return nil
}

// Delete deletes a value from the cache
func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in the cache
func (c *Cache) Exists(ctx context.Context, key string) bool {
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return result > 0
}

// SetNX sets a value only if it doesn't exist (for idempotency)
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal value: %w", err)
	}

	return c.client.SetNX(ctx, key, data, ttl).Result()
}

// Increment increments a counter
func (c *Cache) Increment(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

// IncrementWithTTL increments a counter and sets TTL if key is new
func (c *Cache) IncrementWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to increment with TTL: %w", err)
	}

	return incr.Val(), nil
}

// GetCounter gets the current value of a counter
func (c *Cache) GetCounter(ctx context.Context, key string) (int64, error) {
	result, err := c.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get counter: %w", err)
	}
	return result, nil
}

// SetString sets a string value in the cache
func (c *Cache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// GetString gets a string value from the cache
func (c *Cache) GetString(ctx context.Context, key string) (string, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrCacheMiss
		}
		return "", fmt.Errorf("failed to get string: %w", err)
	}
	return result, nil
}

// TTL gets the remaining TTL for a key
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// Keys returns all keys matching a pattern
func (c *Cache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.client.Keys(ctx, pattern).Result()
}

// Ping checks if Redis is available
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	return c.client.Close()
}

// ErrCacheMiss is returned when a key is not found in cache
var ErrCacheMiss = fmt.Errorf("cache miss")

// Key prefixes for different data types
const (
	KeyPrefixIdempotency   = "idempotency:"
	KeyPrefixRateLimit     = "ratelimit:"
	KeyPrefixSession       = "session:"
	KeyPrefixOverlayToken  = "overlay_token:"
	KeyPrefixPaymentStatus = "payment_status:"
	KeyPrefixWebhook       = "webhook:"
)

// IdempotencyKey generates an idempotency key
func IdempotencyKey(provider, externalID, transactionID string) string {
	return fmt.Sprintf("%s%s:%s:%s", KeyPrefixWebhook, provider, externalID, transactionID)
}

// RateLimitKey generates a rate limit key for an IP
func RateLimitKey(ip, endpoint string) string {
	return fmt.Sprintf("%s%s:%s", KeyPrefixRateLimit, endpoint, ip)
}

// SessionKey generates a session key
func SessionKey(sessionID string) string {
	return KeyPrefixSession + sessionID
}

// OverlayTokenKey generates an overlay token key
func OverlayTokenKey(token string) string {
	return KeyPrefixOverlayToken + token
}

// PaymentStatusKey generates a payment status cache key
func PaymentStatusKey(paymentID string) string {
	return KeyPrefixPaymentStatus + paymentID
}
