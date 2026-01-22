package handler

import (
	"net/http"

	"homexai/internal/database"

	"github.com/gin-gonic/gin"
)

// HealthCheckResponse represents the health check response
type HealthCheckResponse struct {
	Status   string                 `json:"status"`
	Database string                 `json:"database,omitempty"`
	Redis    string                 `json:"redis,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// HealthCheckHandler handles health check requests
// ✅ 新增：完整的健康检查接口
func HealthCheckHandler(c *gin.Context) {
	response := HealthCheckResponse{
		Status:  "healthy",
		Details: make(map[string]interface{}),
	}

	// Check master database
	if err := database.HealthCheck(); err != nil {
		response.Status = "unhealthy"
		response.Error = "database connection failed"
		response.Database = "disconnected"
		response.Details["database_error"] = err.Error()

		c.JSON(http.StatusServiceUnavailable, response)
		return
	}
	response.Database = "connected"

	// Get database connection stats
	stats := database.GetConnectionStats()
	response.Details["db_connections_open"] = stats.OpenConnections
	response.Details["db_connections_in_use"] = stats.InUse
	response.Details["db_connections_idle"] = stats.Idle
	response.Details["db_max_open_connections"] = stats.MaxOpenConnections

	// Check Redis (if needed)
	redisClient := database.GetRedisClient()
	if redisClient != nil {
		if err := redisClient.Ping(c.Request.Context()).Err(); err != nil {
			response.Status = "unhealthy"
			response.Error = "redis connection failed"
			response.Redis = "disconnected"
			response.Details["redis_error"] = err.Error()

			c.JSON(http.StatusServiceUnavailable, response)
			return
		}
		response.Redis = "connected"
	} else {
		response.Redis = "not_configured"
	}

	c.JSON(http.StatusOK, response)
}

// ReadinessHandler handles readiness probe requests
// ✅ 新增：Kubernetes readiness probe
func ReadinessHandler(c *gin.Context) {
	// Check if database is ready
	if err := database.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ready": true,
	})
}

// LivenessHandler handles liveness probe requests
// ✅ 新增：Kubernetes liveness probe
func LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"alive": true,
	})
}

// MetricsHandler handles basic metrics requests
// ✅ 新增：基础指标接口
func MetricsHandler(c *gin.Context) {
	stats := database.GetConnectionStats()

	metrics := gin.H{
		"database": gin.H{
			"connections_open":         stats.OpenConnections,
			"connections_in_use":       stats.InUse,
			"connections_idle":         stats.Idle,
			"connections_max_open":     stats.MaxOpenConnections,
			"connections_max_idle":     stats.MaxIdleClosed,
			"connections_max_lifetime": stats.MaxLifetimeClosed,
			"wait_count":               stats.WaitCount,
			"wait_duration_ms":         stats.WaitDuration.Milliseconds(),
		},
	}

	// Add Redis metrics if available
	redisClient := database.GetRedisClient()
	if redisClient != nil {
		poolStats := redisClient.PoolStats()
		metrics["redis"] = gin.H{
			"hits":        poolStats.Hits,
			"misses":      poolStats.Misses,
			"timeouts":    poolStats.Timeouts,
			"total_conns": poolStats.TotalConns,
			"idle_conns":  poolStats.IdleConns,
			"stale_conns": poolStats.StaleConns,
		}
	}

	c.JSON(http.StatusOK, metrics)
}
