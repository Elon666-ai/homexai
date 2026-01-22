package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// CacheService provides caching functionality for the application
type CacheService struct {
	client *redis.Client
	prefix string
}

// NewCacheService creates a new cache service instance
func NewCacheService(client *redis.Client, prefix string) *CacheService {
	if prefix == "" {
		prefix = "homex"
	}
	return &CacheService{
		client: client,
		prefix: prefix,
	}
}

// buildKey builds a cache key with prefix
func (s *CacheService) buildKey(key string) string {
	return fmt.Sprintf("%s:%s", s.prefix, key)
}

// Set stores a value in cache with expiration
func (s *CacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	fullKey := s.buildKey(key)
	
	// Marshal value to JSON if not string
	var data interface{}
	switch v := value.(type) {
	case string:
		data = v
	case []byte:
		data = v
	default:
		jsonData, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		data = jsonData
	}
	
	return s.client.Set(ctx, fullKey, data, expiration).Err()
}

// Get retrieves a value from cache
func (s *CacheService) Get(ctx context.Context, key string) (string, error) {
	fullKey := s.buildKey(key)
	result, err := s.client.Get(ctx, fullKey).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("cache miss: key not found")
	}
	return result, err
}

// GetJSON retrieves and unmarshals JSON value from cache
func (s *CacheService) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	
	return json.Unmarshal([]byte(data), dest)
}

// Delete removes a key from cache
func (s *CacheService) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.buildKey(key)
	}
	
	return s.client.Del(ctx, fullKeys...).Err()
}

// Exists checks if a key exists in cache
func (s *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := s.buildKey(key)
	result, err := s.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// Expire sets expiration time for a key
func (s *CacheService) Expire(ctx context.Context, key string, expiration time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Expire(ctx, fullKey, expiration).Err()
}

// Increment increments a counter
func (s *CacheService) Increment(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.Incr(ctx, fullKey).Result()
}

// IncrementBy increments a counter by a specific amount
func (s *CacheService) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.IncrBy(ctx, fullKey, value).Result()
}

// Decrement decrements a counter
func (s *CacheService) Decrement(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.Decr(ctx, fullKey).Result()
}

// IncrementWithExpiry increments a counter and sets expiration if new
func (s *CacheService) IncrementWithExpiry(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	fullKey := s.buildKey(key)
	
	// Use pipeline for atomic operation
	pipe := s.client.Pipeline()
	incrCmd := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, expiration)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	
	return incrCmd.Val(), nil
}

// GetOrSet retrieves a value from cache, or sets it using the loader function if not found
func (s *CacheService) GetOrSet(ctx context.Context, key string, expiration time.Duration, loader func(ctx context.Context) (interface{}, error), dest interface{}) error {
	// Try to get from cache first
	err := s.GetJSON(ctx, key, dest)
	if err == nil {
		return nil // Cache hit
	}
	
	// Cache miss - load from source
	value, err := loader(ctx)
	if err != nil {
		return fmt.Errorf("loader function failed: %w", err)
	}
	
	// Store in cache
	if err := s.Set(ctx, key, value, expiration); err != nil {
		// Log error but don't fail the request
		// The value is still returned to the caller
		fmt.Printf("Warning: failed to cache value: %v\n", err)
	}
	
	// Marshal and unmarshal to dest
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal loaded value: %w", err)
	}
	
	return json.Unmarshal(jsonData, dest)
}

// DeleteByPattern deletes all keys matching a pattern
func (s *CacheService) DeleteByPattern(ctx context.Context, pattern string) error {
	fullPattern := s.buildKey(pattern)
	
	var cursor uint64
	var keys []string
	
	// Scan for matching keys
	for {
		var scanKeys []string
		var err error
		scanKeys, cursor, err = s.client.Scan(ctx, cursor, fullPattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}
		
		keys = append(keys, scanKeys...)
		
		if cursor == 0 {
			break
		}
	}
	
	// Delete all matching keys
	if len(keys) > 0 {
		return s.client.Del(ctx, keys...).Err()
	}
	
	return nil
}

// SetNX sets a value only if the key does not exist (for distributed locks)
func (s *CacheService) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	fullKey := s.buildKey(key)
	return s.client.SetNX(ctx, fullKey, value, expiration).Result()
}

// GetTTL returns the remaining time to live for a key
func (s *CacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := s.buildKey(key)
	return s.client.TTL(ctx, fullKey).Result()
}

// PushToList pushes values to a list
func (s *CacheService) PushToList(ctx context.Context, key string, values ...interface{}) error {
	fullKey := s.buildKey(key)
	return s.client.RPush(ctx, fullKey, values...).Err()
}

// GetListRange retrieves a range of elements from a list
func (s *CacheService) GetListRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	fullKey := s.buildKey(key)
	return s.client.LRange(ctx, fullKey, start, stop).Result()
}

// GetListLength returns the length of a list
func (s *CacheService) GetListLength(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.LLen(ctx, fullKey).Result()
}

// TrimList trims a list to the specified range
func (s *CacheService) TrimList(ctx context.Context, key string, start, stop int64) error {
	fullKey := s.buildKey(key)
	return s.client.LTrim(ctx, fullKey, start, stop).Err()
}

// SetHash sets a hash field
func (s *CacheService) SetHash(ctx context.Context, key, field string, value interface{}) error {
	fullKey := s.buildKey(key)
	return s.client.HSet(ctx, fullKey, field, value).Err()
}

// GetHash retrieves a hash field
func (s *CacheService) GetHash(ctx context.Context, key, field string) (string, error) {
	fullKey := s.buildKey(key)
	return s.client.HGet(ctx, fullKey, field).Result()
}

// GetAllHash retrieves all fields and values of a hash
func (s *CacheService) GetAllHash(ctx context.Context, key string) (map[string]string, error) {
	fullKey := s.buildKey(key)
	return s.client.HGetAll(ctx, fullKey).Result()
}

// DeleteHashField deletes a hash field
func (s *CacheService) DeleteHashField(ctx context.Context, key string, fields ...string) error {
	fullKey := s.buildKey(key)
	return s.client.HDel(ctx, fullKey, fields...).Err()
}

// AddToSet adds members to a set
func (s *CacheService) AddToSet(ctx context.Context, key string, members ...interface{}) error {
	fullKey := s.buildKey(key)
	return s.client.SAdd(ctx, fullKey, members...).Err()
}

// RemoveFromSet removes members from a set
func (s *CacheService) RemoveFromSet(ctx context.Context, key string, members ...interface{}) error {
	fullKey := s.buildKey(key)
	return s.client.SRem(ctx, fullKey, members...).Err()
}

// GetSetMembers returns all members of a set
func (s *CacheService) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	fullKey := s.buildKey(key)
	return s.client.SMembers(ctx, fullKey).Result()
}

// IsSetMember checks if a value is a member of a set
func (s *CacheService) IsSetMember(ctx context.Context, key string, member interface{}) (bool, error) {
	fullKey := s.buildKey(key)
	return s.client.SIsMember(ctx, fullKey, member).Result()
}

// GetSetSize returns the size of a set
func (s *CacheService) GetSetSize(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.SCard(ctx, fullKey).Result()
}

// Business-specific cache methods

// CacheAnnouncement caches an announcement
func (s *CacheService) CacheAnnouncement(ctx context.Context, propertyID, announcementID uint, data interface{}) error {
	key := fmt.Sprintf("announcement:%d:%d", propertyID, announcementID)
	return s.Set(ctx, key, data, 10*time.Minute)
}

// GetCachedAnnouncement retrieves a cached announcement
func (s *CacheService) GetCachedAnnouncement(ctx context.Context, propertyID, announcementID uint, dest interface{}) error {
	key := fmt.Sprintf("announcement:%d:%d", propertyID, announcementID)
	return s.GetJSON(ctx, key, dest)
}

// InvalidateAnnouncement invalidates announcement cache
func (s *CacheService) InvalidateAnnouncement(ctx context.Context, propertyID, announcementID uint) error {
	key := fmt.Sprintf("announcement:%d:%d", propertyID, announcementID)
	return s.Delete(ctx, key)
}

// InvalidateAnnouncementList invalidates announcement list cache
func (s *CacheService) InvalidateAnnouncementList(ctx context.Context, propertyID uint) error {
	pattern := fmt.Sprintf("announcements:list:%d:*", propertyID)
	return s.DeleteByPattern(ctx, pattern)
}

// CachePropertySettings caches property settings
func (s *CacheService) CachePropertySettings(ctx context.Context, propertyID uint, data interface{}) error {
	key := fmt.Sprintf("property:settings:%d", propertyID)
	return s.Set(ctx, key, data, 1*time.Hour)
}

// GetCachedPropertySettings retrieves cached property settings
func (s *CacheService) GetCachedPropertySettings(ctx context.Context, propertyID uint, dest interface{}) error {
	key := fmt.Sprintf("property:settings:%d", propertyID)
	return s.GetJSON(ctx, key, dest)
}

// InvalidatePropertySettings invalidates property settings cache
func (s *CacheService) InvalidatePropertySettings(ctx context.Context, propertyID uint) error {
	key := fmt.Sprintf("property:settings:%d", propertyID)
	return s.Delete(ctx, key)
}

// CacheBillSummary caches bill summary
func (s *CacheService) CacheBillSummary(ctx context.Context, propertyID uint, filters string, data interface{}) error {
	key := fmt.Sprintf("bills:summary:%d:%s", propertyID, filters)
	return s.Set(ctx, key, data, 5*time.Minute)
}

// GetCachedBillSummary retrieves cached bill summary
func (s *CacheService) GetCachedBillSummary(ctx context.Context, propertyID uint, filters string, dest interface{}) error {
	key := fmt.Sprintf("bills:summary:%d:%s", propertyID, filters)
	return s.GetJSON(ctx, key, dest)
}

// InvalidateBillCache invalidates all bill-related cache for a property
func (s *CacheService) InvalidateBillCache(ctx context.Context, propertyID uint) error {
	pattern := fmt.Sprintf("bills:*:%d:*", propertyID)
	return s.DeleteByPattern(ctx, pattern)
}

// CacheUserPermissions caches user permissions
func (s *CacheService) CacheUserPermissions(ctx context.Context, userID, propertyID uint, data interface{}) error {
	key := fmt.Sprintf("user:permissions:%d:%d", userID, propertyID)
	return s.Set(ctx, key, data, 30*time.Minute)
}

// GetCachedUserPermissions retrieves cached user permissions
func (s *CacheService) GetCachedUserPermissions(ctx context.Context, userID, propertyID uint, dest interface{}) error {
	key := fmt.Sprintf("user:permissions:%d:%d", userID, propertyID)
	return s.GetJSON(ctx, key, dest)
}

// InvalidateUserPermissions invalidates user permissions cache
func (s *CacheService) InvalidateUserPermissions(ctx context.Context, userID, propertyID uint) error {
	key := fmt.Sprintf("user:permissions:%d:%d", userID, propertyID)
	return s.Delete(ctx, key)
}

// InvalidateAllUserPermissions invalidates all permissions for a user
func (s *CacheService) InvalidateAllUserPermissions(ctx context.Context, userID uint) error {
	pattern := fmt.Sprintf("user:permissions:%d:*", userID)
	return s.DeleteByPattern(ctx, pattern)
}

// CacheDashboardStats caches dashboard statistics
func (s *CacheService) CacheDashboardStats(ctx context.Context, propertyID uint, userRole string, data interface{}) error {
	key := fmt.Sprintf("dashboard:stats:%d:%s", propertyID, userRole)
	return s.Set(ctx, key, data, 5*time.Minute)
}

// GetCachedDashboardStats retrieves cached dashboard statistics
func (s *CacheService) GetCachedDashboardStats(ctx context.Context, propertyID uint, userRole string, dest interface{}) error {
	key := fmt.Sprintf("dashboard:stats:%d:%s", propertyID, userRole)
	return s.GetJSON(ctx, key, dest)
}

// InvalidateDashboardStats invalidates dashboard statistics cache
func (s *CacheService) InvalidateDashboardStats(ctx context.Context, propertyID uint) error {
	pattern := fmt.Sprintf("dashboard:stats:%d:*", propertyID)
	return s.DeleteByPattern(ctx, pattern)
}

// CacheSearchResults caches search results
func (s *CacheService) CacheSearchResults(ctx context.Context, propertyID uint, query string, data interface{}) error {
	key := fmt.Sprintf("search:results:%d:%s", propertyID, query)
	return s.Set(ctx, key, data, 10*time.Minute)
}

// GetCachedSearchResults retrieves cached search results
func (s *CacheService) GetCachedSearchResults(ctx context.Context, propertyID uint, query string, dest interface{}) error {
	key := fmt.Sprintf("search:results:%d:%s", propertyID, query)
	return s.GetJSON(ctx, key, dest)
}

// Rate limiting helpers

// CheckRateLimit checks if rate limit is exceeded
func (s *CacheService) CheckRateLimit(ctx context.Context, identifier string, limit int64, window time.Duration) (bool, int64, error) {
	key := fmt.Sprintf("ratelimit:%s", identifier)
	
	// Increment counter
	count, err := s.IncrementWithExpiry(ctx, key, window)
	if err != nil {
		return false, 0, err
	}
	
	// Check if limit exceeded
	if count > limit {
		return false, limit - count, nil // Rate limit exceeded
	}
	
	return true, limit - count, nil // Within limit
}

// Session helpers

// SetSession stores session data
func (s *CacheService) SetSession(ctx context.Context, sessionID string, data interface{}, expiration time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.Set(ctx, key, data, expiration)
}

// GetSession retrieves session data
func (s *CacheService) GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.GetJSON(ctx, key, dest)
}

// DeleteSession deletes session data
func (s *CacheService) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.Delete(ctx, key)
}

// RefreshSession extends session expiration
func (s *CacheService) RefreshSession(ctx context.Context, sessionID string, expiration time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.Expire(ctx, key, expiration)
}

// Lock helpers for distributed locking

// AcquireLock attempts to acquire a distributed lock
func (s *CacheService) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf("lock:%s", lockKey)
	return s.SetNX(ctx, key, "1", ttl)
}

// ReleaseLock releases a distributed lock
func (s *CacheService) ReleaseLock(ctx context.Context, lockKey string) error {
	key := fmt.Sprintf("lock:%s", lockKey)
	return s.Delete(ctx, key)
}

// ExtendLock extends the TTL of a lock
func (s *CacheService) ExtendLock(ctx context.Context, lockKey string, ttl time.Duration) error {
	key := fmt.Sprintf("lock:%s", lockKey)
	return s.Expire(ctx, key, ttl)
}

// Batch operations

// SetMultiple sets multiple key-value pairs
func (s *CacheService) SetMultiple(ctx context.Context, items map[string]interface{}, expiration time.Duration) error {
	pipe := s.client.Pipeline()
	
	for key, value := range items {
		fullKey := s.buildKey(key)
		
		var data interface{}
		switch v := value.(type) {
		case string:
			data = v
		case []byte:
			data = v
		default:
			jsonData, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
			}
			data = jsonData
		}
		
		pipe.Set(ctx, fullKey, data, expiration)
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

// GetMultiple retrieves multiple values
func (s *CacheService) GetMultiple(ctx context.Context, keys ...string) (map[string]string, error) {
	if len(keys) == 0 {
		return make(map[string]string), nil
	}
	
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.buildKey(key)
	}
	
	values, err := s.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]string)
	for i, val := range values {
		if val != nil {
			if str, ok := val.(string); ok {
				result[keys[i]] = str
			}
		}
	}
	
	return result, nil
}

// Health check

// Ping checks if Redis connection is alive
func (s *CacheService) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// GetStats returns cache statistics
func (s *CacheService) GetStats(ctx context.Context) (map[string]string, error) {
	info, err := s.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, err
	}
	
	// Parse info string into map
	stats := make(map[string]string)
	// Simple parsing - can be enhanced
	stats["raw_info"] = info
	
	return stats, nil
}
