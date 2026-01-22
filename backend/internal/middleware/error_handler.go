package middleware

import (
	"fmt"
	"log"
	"runtime/debug"

	"homexai/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ErrorHandler is a middleware that handles errors uniformly
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Check if there were any errors
		if len(c.Errors) > 0 {
			// Get the last error
			err := c.Errors.Last()

			// Handle the error
			if appErr, ok := err.Err.(*errors.AppError); ok {
				errors.Response(c, appErr)
			} else {
				// Unknown error - return as internal server error
				errors.Response(c, errors.Internal("Internal server error"))
			}
		}
	}
}

// RecoveryHandler recovers from panics and returns 500
func RecoveryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				log.Printf("PANIC: %v\n%s", err, debug.Stack())

				// Return error response
				appErr := errors.Internal("Internal server error")
				appErr.Details = fmt.Sprintf("Panic recovered: %v", err)

				errors.Response(c, appErr)
				c.Abort()
			}
		}()

		c.Next()
	}
}

// RequestLogger logs request details with error information
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				log.Printf("[ERROR] %s %s - %v",
					c.Request.Method,
					c.Request.URL.Path,
					err.Err)
			}
		}
	}
}

// ValidationErrorHandler handles validation errors from binding
func ValidationErrorHandler(c *gin.Context, err error) {
	// Convert validation error to AppError
	appErr := errors.Validation("Invalid input data")

	// TODO: Parse validation errors and extract field-specific errors
	// This would require inspecting the validator.ValidationErrors type

	errors.Response(c, appErr.WithError(err))
}
