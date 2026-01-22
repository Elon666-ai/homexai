package middleware

import (
	"fmt"
	"time"

	"homexai/internal/config"

	"github.com/gin-gonic/gin"
)

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(startTime)

		// Get status code
		statusCode := c.Writer.Status()

		// Get client IP
		clientIP := c.ClientIP()

		// Get request method and path
		method := c.Request.Method
		path := c.Request.URL.Path

		// Get error if any
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// Get user ID if authenticated
		userID := GetUserID(c)

		// Get property ID if in property context
		propertyID := GetPropertyID(c)

		// Log format
		logMessage := fmt.Sprintf(
			"[HTTP] %s | %3d | %13v | %15s | %-7s %s",
			time.Now().Format("2006/01/02 15:04:05"),
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)

		// Add user context if available
		if userID != 0 {
			logMessage += fmt.Sprintf(" | UserID: %d", userID)
		}

		// Add property context if available
		if propertyID != 0 {
			logMessage += fmt.Sprintf(" | PropertyID: %d", propertyID)
		}

		// Add error if any
		if errorMessage != "" {
			logMessage += fmt.Sprintf(" | Error: %s", errorMessage)
		}

		// Log based on status code
		if statusCode >= 500 {
			fmt.Println("ERROR:", logMessage)
		} else if statusCode >= 400 {
			fmt.Println("WARN:", logMessage)
		} else if config.Yaml.Server.AppDebug {
			fmt.Println("INFO:", logMessage)
		}
	}
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID already exists
		requestID := c.GetHeader("X-Request-ID")

		if requestID == "" {
			// Generate new request ID
			requestID = generateRequestID()
		}

		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
}

// GetRequestID retrieves request ID from context
func GetRequestID(c *gin.Context) string {
	requestID, exists := c.Get("request_id")
	if !exists {
		return ""
	}
	return requestID.(string)
}
