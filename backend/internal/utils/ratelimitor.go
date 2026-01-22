package utils

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type rateEntry struct {
	count atomic.Int64
}

var rateLimitMap sync.Map

// ✅ 限流规则: 每IP 每api qps<2000次请求
const (
	MaxRequestsPerSecond = 2000
	CleanupInterval      = time.Second
)

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		path := c.Request.URL.Path
		if ip == "" {
			ip = "unknown"
		}

		key := ip + ":" + path
		val, _ := rateLimitMap.LoadOrStore(key, &rateEntry{})
		entry := val.(*rateEntry)

		newCount := entry.count.Add(1)
		if newCount > MaxRequestsPerSecond {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "rate limit exceeded: max 2000 req/s per IP per API",
				"data": gin.H{"ip": ip, "path": path},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
