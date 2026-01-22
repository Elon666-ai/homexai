package middleware

import (
	"fmt"
	"strconv"
	"time"

	"homexai/internal/config"
	"homexai/internal/database"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultQPS is the default queries per second limit (maximum 2000 QPS)
	DefaultQPS = 2000
	// RateLimitWindow is the time window for rate limiting (1 second)
	RateLimitWindow = time.Second
)

// RateLimitMiddleware implements per-second rate limiting using Redis
// Maximum QPS: 2000 requests per second
//
// Algorithm: Token bucket with per-second window
// - Each second gets a fresh bucket of tokens
// - Key format: rate_limit:{identifier}:{unix_second}
// - Automatic cleanup after 2 seconds
func RateLimitMiddleware() gin.HandlerFunc {
	cfg := config.Yaml

	if !cfg.RateLimit.Enabled {
		// Rate limiting disabled, skip
		return func(c *gin.Context) {
			c.Next()
		}
	}

	// Get QPS limit from config, default to 2000
	qpsLimit := DefaultQPS
	if cfg.RateLimit.RequestsPerSecond > 0 {
		// If config has RequestsPerMinute, convert to QPS
		// RequestsPerMinute / 60 = requests per second
		qpsLimit = cfg.RateLimit.RequestsPerSecond
		if qpsLimit > DefaultQPS {
			qpsLimit = DefaultQPS // Cap at 2000 QPS
		}
		if qpsLimit <= 0 {
			qpsLimit = DefaultQPS
		}
	}

	return func(c *gin.Context) {
		// Get client identifier (IP or user ID)
		identifier := getClientIdentifier(c)

		// Create rate limit key with current second timestamp
		// Key format: rate_limit:{identifier}:{unix_second}
		// This creates a new bucket every second
		currentSecond := time.Now().Unix()
		key := fmt.Sprintf("rate_limit:%s:%d", identifier, currentSecond)

		// Get current count from Redis
		redis := database.GetRedisClient()
		if redis == nil {
			// Fail open - allow request if Redis is not initialized
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Atomically increment counter
		count, err := redis.Incr(ctx, key).Result()
		if err != nil {
			// If Redis fails, allow request but log error
			// This prevents rate limiter from blocking all traffic if Redis is down
			fmt.Printf("Rate limit Redis error: %v\n", err)
			c.Next()
			return
		}

		// Set expiration to 2 seconds (gives buffer for cleanup)
		// Only set on first request to avoid resetting TTL
		if count == 1 {
			redis.Expire(ctx, key, 2*time.Second)
		}

		limit := int64(qpsLimit)

		// Set standard rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d per second", limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(max(0, limit-count), 10))
		c.Header("X-RateLimit-Window", "1s")

		// Calculate reset time (next second)
		resetTime := currentSecond + 1
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		// Check if limit exceeded
		if count > limit {
			// Tell client to retry after 1 second
			c.Header("Retry-After", "1")

			utils.ErrorResponse(c, 429, fmt.Sprintf("Rate limit exceeded: maximum %d requests per second", limit), nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// CustomRateLimitMiddleware creates a custom per-second rate limiter
// qps: queries per second limit (capped at 2000)
// keyPrefix: prefix for Redis key to separate different limiters
//
// Example: CustomRateLimitMiddleware(100, "api:login")
func CustomRateLimitMiddleware(qps int, keyPrefix string) gin.HandlerFunc {
	// Validate and cap QPS
	if qps > DefaultQPS {
		qps = DefaultQPS
	}
	if qps <= 0 {
		qps = DefaultQPS
	}

	return func(c *gin.Context) {
		identifier := getClientIdentifier(c)
		currentSecond := time.Now().Unix()
		key := fmt.Sprintf("rate_limit:%s:%s:%d", keyPrefix, identifier, currentSecond)

		redis := database.GetRedisClient()
		if redis == nil {
			// Fail open - allow request if Redis is not initialized
			c.Next()
			return
		}

		ctx := c.Request.Context()

		count, err := redis.Incr(ctx, key).Result()
		if err != nil {
			// Fail open - allow request if Redis is down
			c.Next()
			return
		}

		if count == 1 {
			redis.Expire(ctx, key, 2*time.Second)
		}

		limit := int64(qps)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d per second", limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(max(0, limit-count), 10))

		if count > limit {
			c.Header("Retry-After", "1")
			utils.ErrorResponse(c, 429, fmt.Sprintf("Rate limit exceeded: maximum %d requests per second", limit), nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// getClientIdentifier returns a unique identifier for the client
// Prefers authenticated user ID, falls back to IP address
func getClientIdentifier(c *gin.Context) string {
	// Prefer user ID if authenticated
	userID := GetUserID(c)
	if userID != 0 {
		return fmt.Sprintf("user:%d", userID)
	}

	// Fall back to IP address
	// Check X-Forwarded-For header for proxy/load balancer setups
	ip := c.ClientIP()
	return fmt.Sprintf("ip:%s", ip)
}

// max returns the maximum of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// RateLimitByIP limits requests by IP address only (per second)
// qps: queries per second limit (capped at 2000)
func RateLimitByIP(qps int) gin.HandlerFunc {
	if qps > DefaultQPS {
		qps = DefaultQPS
	}
	if qps <= 0 {
		qps = DefaultQPS
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		currentSecond := time.Now().Unix()
		key := fmt.Sprintf("rate_limit:ip:%s:%d", ip, currentSecond)

		redis := database.GetRedisClient()
		if redis == nil {
			// Fail open - allow request if Redis is not initialized
			c.Next()
			return
		}

		ctx := c.Request.Context()

		count, err := redis.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		if count == 1 {
			redis.Expire(ctx, key, 2*time.Second)
		}

		limit := int64(qps)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d per second", limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(max(0, limit-count), 10))

		if count > limit {
			c.Header("Retry-After", "1")
			utils.ErrorResponse(c, 429, "Too many requests from this IP address", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByUser limits requests by authenticated user only (per second)
// qps: queries per second limit (capped at 2000)
func RateLimitByUser(qps int) gin.HandlerFunc {
	if qps > DefaultQPS {
		qps = DefaultQPS
	}
	if qps <= 0 {
		qps = DefaultQPS
	}

	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			// Not authenticated, skip rate limiting
			c.Next()
			return
		}

		currentSecond := time.Now().Unix()
		key := fmt.Sprintf("rate_limit:user:%d:%d", userID, currentSecond)
		redis := database.GetRedisClient()
		if redis == nil {
			// Fail open - allow request if Redis is not initialized
			c.Next()
			return
		}

		ctx := c.Request.Context()

		count, err := redis.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		if count == 1 {
			redis.Expire(ctx, key, 2*time.Second)
		}

		limit := int64(qps)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d per second", limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(max(0, limit-count), 10))

		if count > limit {
			c.Header("Retry-After", "1")
			utils.ErrorResponse(c, 429, "Too many requests", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// StrictRateLimitMiddleware implements strict QPS limiting at exactly 2000 QPS
// Use this for production to ensure QPS never exceeds 2000
func StrictRateLimitMiddleware() gin.HandlerFunc {
	return CustomRateLimitMiddleware(DefaultQPS, "strict")
}
