package service

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

// setupTestCache creates a test cache service
func setupTestCache(t *testing.T) (*CacheService, func()) {
	// Use a test Redis database (DB 15)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Test database
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing")
	}

	cache := NewCacheService(client, "test")

	// Cleanup function
	cleanup := func() {
		client.FlushDB(ctx)
		client.Close()
	}

	return cache, cleanup
}

func TestCacheService_BasicOperations(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Set and Get string", func(t *testing.T) {
		err := cache.Set(ctx, "test:string", "hello world", time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "test:string")
		assert.NoError(t, err)
		assert.Equal(t, "hello world", value)
	})

	t.Run("Set and Get JSON", func(t *testing.T) {
		type TestStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		original := TestStruct{Name: "test", Value: 123}
		err := cache.Set(ctx, "test:json", original, time.Minute)
		assert.NoError(t, err)

		var result TestStruct
		err = cache.GetJSON(ctx, "test:json", &result)
		assert.NoError(t, err)
		assert.Equal(t, original, result)
	})

	t.Run("Delete", func(t *testing.T) {
		cache.Set(ctx, "test:delete", "value", time.Minute)

		err := cache.Delete(ctx, "test:delete")
		assert.NoError(t, err)

		_, err = cache.Get(ctx, "test:delete")
		assert.Error(t, err) // Should not exist
	})

	t.Run("Exists", func(t *testing.T) {
		cache.Set(ctx, "test:exists", "value", time.Minute)

		exists, err := cache.Exists(ctx, "test:exists")
		assert.NoError(t, err)
		assert.True(t, exists)

		exists, err = cache.Exists(ctx, "test:notexists")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestCacheService_GetOrSet(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Cache miss loads from loader", func(t *testing.T) {
		loaderCalled := false

		var result string
		err := cache.GetOrSet(
			ctx,
			"test:getorset:miss",
			time.Minute,
			func(ctx context.Context) (interface{}, error) {
				loaderCalled = true
				return "loaded value", nil
			},
			&result,
		)

		assert.NoError(t, err)
		assert.True(t, loaderCalled)
		assert.Equal(t, "loaded value", result)
	})

	t.Run("Cache hit does not call loader", func(t *testing.T) {
		// Pre-populate cache
		cache.Set(ctx, "test:getorset:hit", "cached value", time.Minute)

		loaderCalled := false

		var result string
		err := cache.GetOrSet(
			ctx,
			"test:getorset:hit",
			time.Minute,
			func(ctx context.Context) (interface{}, error) {
				loaderCalled = true
				return "should not be called", nil
			},
			&result,
		)

		assert.NoError(t, err)
		assert.False(t, loaderCalled)
		assert.Equal(t, "cached value", result)
	})

	t.Run("GetOrSet with struct", func(t *testing.T) {
		type User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		var result User
		err := cache.GetOrSet(
			ctx,
			"test:getorset:struct",
			time.Minute,
			func(ctx context.Context) (interface{}, error) {
				return User{ID: 1, Name: "John"}, nil
			},
			&result,
		)

		assert.NoError(t, err)
		assert.Equal(t, 1, result.ID)
		assert.Equal(t, "John", result.Name)
	})
}

func TestCacheService_Counter(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Increment", func(t *testing.T) {
		count, err := cache.Increment(ctx, "test:counter")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)

		count, err = cache.Increment(ctx, "test:counter")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("IncrementBy", func(t *testing.T) {
		count, err := cache.IncrementBy(ctx, "test:counter:by", 5)
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		count, err = cache.IncrementBy(ctx, "test:counter:by", 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(15), count)
	})

	t.Run("Decrement", func(t *testing.T) {
		cache.IncrementBy(ctx, "test:counter:dec", 10)

		count, err := cache.Decrement(ctx, "test:counter:dec")
		assert.NoError(t, err)
		assert.Equal(t, int64(9), count)
	})

	t.Run("IncrementWithExpiry", func(t *testing.T) {
		count, err := cache.IncrementWithExpiry(ctx, "test:counter:expiry", time.Second)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)

		// Verify TTL is set
		ttl, err := cache.GetTTL(ctx, "test:counter:expiry")
		assert.NoError(t, err)
		assert.True(t, ttl > 0 && ttl <= time.Second)
	})
}

func TestCacheService_DeleteByPattern(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple keys
	cache.Set(ctx, "user:1:profile", "data1", time.Minute)
	cache.Set(ctx, "user:2:profile", "data2", time.Minute)
	cache.Set(ctx, "user:3:profile", "data3", time.Minute)
	cache.Set(ctx, "order:1", "order data", time.Minute)

	// Delete all user profiles
	err := cache.DeleteByPattern(ctx, "user:*:profile")
	assert.NoError(t, err)

	// Verify user profiles are gone
	exists, _ := cache.Exists(ctx, "user:1:profile")
	assert.False(t, exists)

	exists, _ = cache.Exists(ctx, "user:2:profile")
	assert.False(t, exists)

	// Verify order still exists
	exists, _ = cache.Exists(ctx, "order:1")
	assert.True(t, exists)
}

func TestCacheService_Lists(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("PushToList and GetListRange", func(t *testing.T) {
		err := cache.PushToList(ctx, "test:list", "item1", "item2", "item3")
		assert.NoError(t, err)

		items, err := cache.GetListRange(ctx, "test:list", 0, -1)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(items))
		assert.Equal(t, "item1", items[0])
	})

	t.Run("GetListLength", func(t *testing.T) {
		cache.PushToList(ctx, "test:list:length", "a", "b", "c")

		length, err := cache.GetListLength(ctx, "test:list:length")
		assert.NoError(t, err)
		assert.Equal(t, int64(3), length)
	})

	t.Run("TrimList", func(t *testing.T) {
		cache.PushToList(ctx, "test:list:trim", "1", "2", "3", "4", "5")

		err := cache.TrimList(ctx, "test:list:trim", 0, 2)
		assert.NoError(t, err)

		length, err := cache.GetListLength(ctx, "test:list:trim")
		assert.NoError(t, err)
		assert.Equal(t, int64(3), length)
	})
}

func TestCacheService_Hash(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SetHash and GetHash", func(t *testing.T) {
		err := cache.SetHash(ctx, "test:hash", "field1", "value1")
		assert.NoError(t, err)

		value, err := cache.GetHash(ctx, "test:hash", "field1")
		assert.NoError(t, err)
		assert.Equal(t, "value1", value)
	})

	t.Run("GetAllHash", func(t *testing.T) {
		cache.SetHash(ctx, "test:hash:all", "f1", "v1")
		cache.SetHash(ctx, "test:hash:all", "f2", "v2")

		all, err := cache.GetAllHash(ctx, "test:hash:all")
		assert.NoError(t, err)
		assert.Equal(t, 2, len(all))
		assert.Equal(t, "v1", all["f1"])
		assert.Equal(t, "v2", all["f2"])
	})

	t.Run("DeleteHashField", func(t *testing.T) {
		cache.SetHash(ctx, "test:hash:del", "field", "value")

		err := cache.DeleteHashField(ctx, "test:hash:del", "field")
		assert.NoError(t, err)

		_, err = cache.GetHash(ctx, "test:hash:del", "field")
		assert.Error(t, err) // Should not exist
	})
}

func TestCacheService_Set(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("AddToSet and GetSetMembers", func(t *testing.T) {
		err := cache.AddToSet(ctx, "test:set", "member1", "member2", "member3")
		assert.NoError(t, err)

		members, err := cache.GetSetMembers(ctx, "test:set")
		assert.NoError(t, err)
		assert.Equal(t, 3, len(members))
	})

	t.Run("IsSetMember", func(t *testing.T) {
		cache.AddToSet(ctx, "test:set:member", "exists")

		isMember, err := cache.IsSetMember(ctx, "test:set:member", "exists")
		assert.NoError(t, err)
		assert.True(t, isMember)

		isMember, err = cache.IsSetMember(ctx, "test:set:member", "notexists")
		assert.NoError(t, err)
		assert.False(t, isMember)
	})

	t.Run("RemoveFromSet", func(t *testing.T) {
		cache.AddToSet(ctx, "test:set:remove", "a", "b", "c")

		err := cache.RemoveFromSet(ctx, "test:set:remove", "b")
		assert.NoError(t, err)

		members, err := cache.GetSetMembers(ctx, "test:set:remove")
		assert.NoError(t, err)
		assert.Equal(t, 2, len(members))
	})

	t.Run("GetSetSize", func(t *testing.T) {
		cache.AddToSet(ctx, "test:set:size", "1", "2", "3", "4")

		size, err := cache.GetSetSize(ctx, "test:set:size")
		assert.NoError(t, err)
		assert.Equal(t, int64(4), size)
	})
}

func TestCacheService_Lock(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("AcquireLock success", func(t *testing.T) {
		acquired, err := cache.AcquireLock(ctx, "test:lock", time.Second)
		assert.NoError(t, err)
		assert.True(t, acquired)
	})

	t.Run("AcquireLock conflict", func(t *testing.T) {
		// First acquisition
		cache.AcquireLock(ctx, "test:lock:conflict", time.Minute)

		// Second acquisition should fail
		acquired, err := cache.AcquireLock(ctx, "test:lock:conflict", time.Second)
		assert.NoError(t, err)
		assert.False(t, acquired)
	})

	t.Run("ReleaseLock", func(t *testing.T) {
		cache.AcquireLock(ctx, "test:lock:release", time.Minute)

		err := cache.ReleaseLock(ctx, "test:lock:release")
		assert.NoError(t, err)

		// Should be able to acquire again
		acquired, err := cache.AcquireLock(ctx, "test:lock:release", time.Second)
		assert.NoError(t, err)
		assert.True(t, acquired)
	})
}

func TestCacheService_RateLimit(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Within limit", func(t *testing.T) {
		allowed, remaining, err := cache.CheckRateLimit(ctx, "user:123", 5, time.Minute)
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, int64(4), remaining)
	})

	t.Run("Exceed limit", func(t *testing.T) {
		identifier := "user:456"

		// Use up the limit
		for i := 0; i < 3; i++ {
			cache.CheckRateLimit(ctx, identifier, 3, time.Minute)
		}

		// Next request should be denied
		allowed, remaining, err := cache.CheckRateLimit(ctx, identifier, 3, time.Minute)
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.True(t, remaining < 0)
	})
}

func TestCacheService_Session(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SetSession and GetSession", func(t *testing.T) {
		sessionData := map[string]interface{}{
			"user_id": 123,
			"role":    "admin",
		}

		err := cache.SetSession(ctx, "session123", sessionData, time.Minute)
		assert.NoError(t, err)

		var retrieved map[string]interface{}
		err = cache.GetSession(ctx, "session123", &retrieved)
		assert.NoError(t, err)
		assert.Equal(t, float64(123), retrieved["user_id"]) // JSON numbers are float64
		assert.Equal(t, "admin", retrieved["role"])
	})

	t.Run("DeleteSession", func(t *testing.T) {
		cache.SetSession(ctx, "session456", map[string]interface{}{"test": "data"}, time.Minute)

		err := cache.DeleteSession(ctx, "session456")
		assert.NoError(t, err)

		var data map[string]interface{}
		err = cache.GetSession(ctx, "session456", &data)
		assert.Error(t, err) // Should not exist
	})

	t.Run("RefreshSession", func(t *testing.T) {
		cache.SetSession(ctx, "session789", map[string]interface{}{"test": "data"}, time.Second)

		// Refresh with longer TTL
		err := cache.RefreshSession(ctx, "session789", time.Minute)
		assert.NoError(t, err)

		ttl, err := cache.GetTTL(ctx, "session:session789")
		assert.NoError(t, err)
		assert.True(t, ttl > time.Second)
	})
}

func TestCacheService_BatchOperations(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SetMultiple", func(t *testing.T) {
		items := map[string]interface{}{
			"batch:1": "value1",
			"batch:2": "value2",
			"batch:3": "value3",
		}

		err := cache.SetMultiple(ctx, items, time.Minute)
		assert.NoError(t, err)

		// Verify all items exist
		val, err := cache.Get(ctx, "batch:1")
		assert.NoError(t, err)
		assert.Equal(t, "value1", val)
	})

	t.Run("GetMultiple", func(t *testing.T) {
		cache.Set(ctx, "multi:1", "v1", time.Minute)
		cache.Set(ctx, "multi:2", "v2", time.Minute)
		cache.Set(ctx, "multi:3", "v3", time.Minute)

		values, err := cache.GetMultiple(ctx, "multi:1", "multi:2", "multi:3")
		assert.NoError(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, "v1", values["multi:1"])
		assert.Equal(t, "v2", values["multi:2"])
		assert.Equal(t, "v3", values["multi:3"])
	})
}

func TestCacheService_BusinessScenarios(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CacheAnnouncement", func(t *testing.T) {
		data := map[string]interface{}{
			"id":    123,
			"title": "Test Announcement",
		}

		err := cache.CacheAnnouncement(ctx, 1001, 123, data)
		assert.NoError(t, err)

		var result map[string]interface{}
		err = cache.GetCachedAnnouncement(ctx, 1001, 123, &result)
		assert.NoError(t, err)
		assert.Equal(t, "Test Announcement", result["title"])
	})

	t.Run("InvalidateAnnouncement", func(t *testing.T) {
		cache.CacheAnnouncement(ctx, 1001, 456, map[string]interface{}{"test": "data"})

		err := cache.InvalidateAnnouncement(ctx, 1001, 456)
		assert.NoError(t, err)

		var result map[string]interface{}
		err = cache.GetCachedAnnouncement(ctx, 1001, 456, &result)
		assert.Error(t, err) // Should be invalidated
	})

	t.Run("CachePropertySettings", func(t *testing.T) {
		settings := map[string]interface{}{
			"timezone": "Asia/Manila",
			"currency": "PHP",
		}

		err := cache.CachePropertySettings(ctx, 2001, settings)
		assert.NoError(t, err)

		var result map[string]interface{}
		err = cache.GetCachedPropertySettings(ctx, 2001, &result)
		assert.NoError(t, err)
		assert.Equal(t, "Asia/Manila", result["timezone"])
	})
}

func TestCacheService_Ping(t *testing.T) {
	cache, cleanup := setupTestCache(t)
	defer cleanup()

	ctx := context.Background()

	err := cache.Ping(ctx)
	assert.NoError(t, err)
}

// Benchmark tests

func BenchmarkCacheService_Set(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer client.Close()

	cache := NewCacheService(client, "bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, "bench:key", "value", time.Minute)
	}
}

func BenchmarkCacheService_Get(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer client.Close()

	cache := NewCacheService(client, "bench")
	ctx := context.Background()

	// Pre-populate
	cache.Set(ctx, "bench:key", "value", time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ctx, "bench:key")
	}
}

func BenchmarkCacheService_GetOrSet(b *testing.B) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	defer client.Close()

	cache := NewCacheService(client, "bench")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result string
		cache.GetOrSet(
			ctx,
			"bench:getorset",
			time.Minute,
			func(ctx context.Context) (interface{}, error) {
				return "loaded", nil
			},
			&result,
		)
	}
}
