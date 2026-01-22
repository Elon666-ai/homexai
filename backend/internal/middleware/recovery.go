package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"homexai/internal/config"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

// RecoveryMiddleware recovers from panics and returns 500 error
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get stack trace
				stack := debug.Stack()

				// Log the panic
				fmt.Printf("PANIC RECOVERED: %v\n", err)
				fmt.Printf("Stack Trace:\n%s\n", stack)

				// Get request details
				method := c.Request.Method
				path := c.Request.URL.Path
				clientIP := c.ClientIP()
				userID := GetUserID(c)
				requestID := GetRequestID(c)

				// Log request context
				fmt.Printf("Request Details:\n")
				fmt.Printf("  Method: %s\n", method)
				fmt.Printf("  Path: %s\n", path)
				fmt.Printf("  Client IP: %s\n", clientIP)
				fmt.Printf("  User ID: %d\n", userID)
				fmt.Printf("  Request ID: %s\n", requestID)

				// Prepare error response
				errorMessage := "Internal server error"
				var errorDetails interface{}

				// In debug mode, include error details
				if config.Yaml.Server.AppDebug {
					errorDetails = map[string]interface{}{
						"error":      fmt.Sprintf("%v", err),
						"stack":      string(stack),
						"request_id": requestID,
					}
				}

				// Return error response
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": errorMessage,
					"error":   errorDetails,
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}

// CustomRecoveryMiddleware allows custom error handling
func CustomRecoveryMiddleware(handler func(*gin.Context, interface{})) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				handler(c, err)
				c.Abort()
			}
		}()

		c.Next()
	}
}

// ErrorHandlerMiddleware handles Gin errors
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			// Get the last error
			err := c.Errors.Last()

			// Log the error
			fmt.Printf("Request Error: %v\n", err.Err)

			// If no response has been written yet
			if !c.Writer.Written() {
				// Return error response
				utils.InternalServerErrorResponse(c, "An error occurred", err.Err)
			}
		}
	}
}
