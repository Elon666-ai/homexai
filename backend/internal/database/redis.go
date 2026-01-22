package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"homexai/internal/config"

	"github.com/go-redis/redis/v8"
)

var RedisClient *redis.Client
var ctx = context.Background()

// InitRedis initializes Redis connection
func InitRedis() error {
	// Get pool size from config or use default
	poolSize := config.Yaml.Redis.PoolSize
	if poolSize <= 0 {
		poolSize = 10 // Default pool size
	}

	minIdleConns := config.Yaml.Redis.MinIdleConns
	if minIdleConns <= 0 {
		minIdleConns = 5 // Default min idle connections
	}

	poolTimeout := config.Yaml.Redis.PoolTimeout
	if poolTimeout <= 0 {
		poolTimeout = 4 * time.Second // Default pool timeout
	}

	idleTimeout := config.Yaml.Redis.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 5 * time.Minute // Default idle timeout
	}

	maxRetries := config.Yaml.Redis.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Default max retries
	}

	// Create Redis client with proper configuration
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         config.Yaml.Redis.Host + ":" + fmt.Sprint(config.Yaml.Redis.Port),
		Password:     config.Yaml.Redis.Password,
		DB:           config.Yaml.Redis.DB,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		PoolTimeout:  poolTimeout,
		IdleTimeout:  idleTimeout,
		MaxRetries:   maxRetries,
		// Add connection health check
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			return nil
		},
	})

	// Verify client is not nil
	if RedisClient == nil {
		return fmt.Errorf("failed to create Redis client")
	}

	// Test connection with timeout to ensure connection pool is initialized
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(pingCtx).Result()
	if err != nil {
		// Close client if ping fails
		if RedisClient != nil {
			RedisClient.Close()
		}
		RedisClient = nil // Reset client on failure
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Perform an additional operation to ensure connection pool is working
	// This helps catch any connection pool initialization issues early
	testCtx, testCancel := context.WithTimeout(ctx, 2*time.Second)
	defer testCancel()

	if err := RedisClient.Set(testCtx, "__health_check__", "ok", time.Second).Err(); err != nil {
		RedisClient.Close()
		RedisClient = nil
		return fmt.Errorf("failed to verify Redis connection pool: %w", err)
	}

	// Clean up test key
	RedisClient.Del(testCtx, "__health_check__")

	log.Println("✓ Redis connected successfully")
	return nil
}

// GetRedis returns the Redis client instance
func GetRedisClient() *redis.Client {
	return RedisClient
}

// CloseRedis closes the Redis connection
func CloseRedisClient() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

// Cache helper functions

// SetCache sets a value in cache with expiration
func SetCache(key string, value interface{}, expiration time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Set(ctx, key, value, expiration).Err()
}

// GetCache gets a value from cache
func GetCache(key string) (string, error) {
	if RedisClient == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Get(ctx, key).Result()
}

// DeleteCache deletes a key from cache
func DeleteCache(key string) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Del(ctx, key).Err()
}

// ExistsCache checks if a key exists in cache
func ExistsCache(key string) (bool, error) {
	if RedisClient == nil {
		return false, fmt.Errorf("redis client not initialized")
	}
	result, err := RedisClient.Exists(ctx, key).Result()
	return result > 0, err
}

// SetCacheWithJSON sets a JSON value in cache
func SetCacheWithJSON(key string, value interface{}, expiration time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	// This will be implemented with JSON marshaling
	return RedisClient.Set(ctx, key, value, expiration).Err()
}

// IncrementCache increments a counter in cache
func IncrementCache(key string) (int64, error) {
	if RedisClient == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Incr(ctx, key).Result()
}

// ExpireCache sets expiration time for a key
func ExpireCache(key string, expiration time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Expire(ctx, key, expiration).Err()
}

// FlushCache removes all keys from current database
func FlushCache() error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return RedisClient.FlushDB(ctx).Err()
}
