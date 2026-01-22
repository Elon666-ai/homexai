package middleware

import (
	"homexai/internal/config"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware configures CORS settings
func CORSMiddleware() gin.HandlerFunc {
	cfg := config.Yaml

	corsConfig := cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Property-Subdomain", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           12 * time.Hour,
	}

	// If origins include "*", set AllowAllOrigins
	for _, origin := range cfg.CORS.AllowedOrigins {
		if origin == "*" {
			corsConfig.AllowAllOrigins = true
			corsConfig.AllowOrigins = nil
			break
		}
	}

	return cors.New(corsConfig)
}
