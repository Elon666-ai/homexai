package errors

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorCode represents standard error codes
type ErrorCode string

const (
	// General errors
	ErrInternal       ErrorCode = "INTERNAL_ERROR"
	ErrBadRequest     ErrorCode = "BAD_REQUEST"
	ErrUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrForbidden      ErrorCode = "FORBIDDEN"
	ErrNotFound       ErrorCode = "NOT_FOUND"
	ErrConflict       ErrorCode = "CONFLICT"
	ErrTooManyRequest ErrorCode = "TOO_MANY_REQUESTS"
	
	// Validation errors
	ErrValidation        ErrorCode = "VALIDATION_ERROR"
	ErrInvalidInput      ErrorCode = "INVALID_INPUT"
	ErrMissingField      ErrorCode = "MISSING_FIELD"
	ErrInvalidFormat     ErrorCode = "INVALID_FORMAT"
	ErrValueTooLarge     ErrorCode = "VALUE_TOO_LARGE"
	ErrValueTooSmall     ErrorCode = "VALUE_TOO_SMALL"
	
	// Authentication errors
	ErrInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrTokenInvalid       ErrorCode = "TOKEN_INVALID"
	ErrSessionExpired     ErrorCode = "SESSION_EXPIRED"
	
	// Authorization errors
	ErrInsufficientPermission ErrorCode = "INSUFFICIENT_PERMISSION"
	ErrResourceForbidden      ErrorCode = "RESOURCE_FORBIDDEN"
	
	// Resource errors
	ErrResourceNotFound    ErrorCode = "RESOURCE_NOT_FOUND"
	ErrResourceExists      ErrorCode = "RESOURCE_EXISTS"
	ErrResourceLocked      ErrorCode = "RESOURCE_LOCKED"
	ErrResourceDeleted     ErrorCode = "RESOURCE_DELETED"
	
	// Database errors
	ErrDatabase       ErrorCode = "DATABASE_ERROR"
	ErrDuplicateEntry ErrorCode = "DUPLICATE_ENTRY"
	ErrForeignKey     ErrorCode = "FOREIGN_KEY_ERROR"
	
	// Business logic errors
	ErrInvalidOperation ErrorCode = "INVALID_OPERATION"
	ErrInvalidState     ErrorCode = "INVALID_STATE"
	ErrLimitExceeded    ErrorCode = "LIMIT_EXCEEDED"
	ErrQuotaExceeded    ErrorCode = "QUOTA_EXCEEDED"
	
	// File errors
	ErrFileUpload     ErrorCode = "FILE_UPLOAD_ERROR"
	ErrFileTooLarge   ErrorCode = "FILE_TOO_LARGE"
	ErrInvalidFileType ErrorCode = "INVALID_FILE_TYPE"
	
	// External service errors
	ErrExternalService ErrorCode = "EXTERNAL_SERVICE_ERROR"
	ErrServiceTimeout  ErrorCode = "SERVICE_TIMEOUT"
	ErrServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// AppError represents a structured application error
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Fields     map[string]string      `json:"fields,omitempty"`
	StatusCode int                    `json:"-"`
	Err        error                  `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails adds detail information to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithField adds a field-specific error
func (e *AppError) WithField(field, message string) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	e.Fields[field] = message
	return e
}

// WithFields adds multiple field-specific errors
func (e *AppError) WithFields(fields map[string]string) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

// WithError wraps an underlying error
func (e *AppError) WithError(err error) *AppError {
	e.Err = err
	return e
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getHTTPStatus(code),
	}
}

// NewWithStatus creates a new AppError with custom status code
func NewWithStatus(code ErrorCode, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// getHTTPStatus maps error codes to HTTP status codes
func getHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrBadRequest, ErrValidation, ErrInvalidInput, ErrMissingField, 
		 ErrInvalidFormat, ErrValueTooLarge, ErrValueTooSmall, ErrInvalidOperation:
		return http.StatusBadRequest
		
	case ErrUnauthorized, ErrInvalidCredentials, ErrTokenExpired, 
		 ErrTokenInvalid, ErrSessionExpired:
		return http.StatusUnauthorized
		
	case ErrForbidden, ErrInsufficientPermission, ErrResourceForbidden:
		return http.StatusForbidden
		
	case ErrNotFound, ErrResourceNotFound:
		return http.StatusNotFound
		
	case ErrConflict, ErrResourceExists, ErrDuplicateEntry, 
		 ErrResourceLocked, ErrInvalidState:
		return http.StatusConflict
		
	case ErrTooManyRequest, ErrLimitExceeded, ErrQuotaExceeded:
		return http.StatusTooManyRequests
		
	case ErrFileTooLarge:
		return http.StatusRequestEntityTooLarge
		
	case ErrServiceTimeout:
		return http.StatusGatewayTimeout
		
	case ErrServiceUnavailable, ErrExternalService:
		return http.StatusServiceUnavailable
		
	default:
		return http.StatusInternalServerError
	}
}

// Predefined common errors

// BadRequest creates a bad request error
func BadRequest(message string) *AppError {
	return New(ErrBadRequest, message)
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *AppError {
	return New(ErrUnauthorized, message)
}

// Forbidden creates a forbidden error
func Forbidden(message string) *AppError {
	return New(ErrForbidden, message)
}

// NotFound creates a not found error
func NotFound(resource string) *AppError {
	return New(ErrResourceNotFound, fmt.Sprintf("%s not found", resource))
}

// Conflict creates a conflict error
func Conflict(message string) *AppError {
	return New(ErrConflict, message)
}

// Internal creates an internal server error
func Internal(message string) *AppError {
	return New(ErrInternal, message)
}

// Validation creates a validation error
func Validation(message string) *AppError {
	return New(ErrValidation, message)
}

// InvalidCredentials creates an invalid credentials error
func InvalidCredentials() *AppError {
	return New(ErrInvalidCredentials, "Invalid email or password")
}

// TokenExpired creates a token expired error
func TokenExpired() *AppError {
	return New(ErrTokenExpired, "Token has expired")
}

// TokenInvalid creates an invalid token error
func TokenInvalid() *AppError {
	return New(ErrTokenInvalid, "Invalid token")
}

// InsufficientPermission creates an insufficient permission error
func InsufficientPermission(action string) *AppError {
	return New(ErrInsufficientPermission, fmt.Sprintf("Insufficient permission to %s", action))
}

// ResourceExists creates a resource exists error
func ResourceExists(resource string) *AppError {
	return New(ErrResourceExists, fmt.Sprintf("%s already exists", resource))
}

// DatabaseError creates a database error
func DatabaseError(operation string) *AppError {
	return New(ErrDatabase, fmt.Sprintf("Database error during %s", operation))
}

// FileUploadError creates a file upload error
func FileUploadError(message string) *AppError {
	return New(ErrFileUpload, message)
}

// FileTooLarge creates a file too large error
func FileTooLarge(maxSize int64) *AppError {
	return New(ErrFileTooLarge, fmt.Sprintf("File size exceeds limit of %d bytes", maxSize))
}

// InvalidFileType creates an invalid file type error
func InvalidFileType(allowed []string) *AppError {
	return New(ErrInvalidFileType, fmt.Sprintf("Invalid file type. Allowed types: %v", allowed))
}

// RateLimitExceeded creates a rate limit error
func RateLimitExceeded(limit int, window string) *AppError {
	return New(ErrTooManyRequest, fmt.Sprintf("Rate limit exceeded: %d requests per %s", limit, window))
}

// Response sends a standardized error response
func Response(c *gin.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		// Log internal errors with details
		if appErr.StatusCode >= 500 {
			c.Error(appErr) // This will be logged by Gin's logger
		}
		
		// Build response
		response := gin.H{
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		}
		
		if appErr.Details != "" {
			response["error"].(gin.H)["details"] = appErr.Details
		}
		
		if len(appErr.Fields) > 0 {
			response["error"].(gin.H)["fields"] = appErr.Fields
		}
		
		c.JSON(appErr.StatusCode, response)
		return
	}
	
	// Handle standard errors
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    ErrInternal,
			"message": "Internal server error",
		},
	})
}

// AbortWithError aborts the request with an error response
func AbortWithError(c *gin.Context, err error) {
	Response(c, err)
	c.Abort()
}

// Handle is a middleware-friendly error handler
func Handle(c *gin.Context, err error) {
	if err != nil {
		Response(c, err)
	}
}

// FromValidationError converts validation errors to AppError
func FromValidationError(err error) *AppError {
	appErr := Validation("Validation failed")
	// TODO: Parse validation errors and add to Fields
	return appErr.WithError(err)
}

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrNotFound || appErr.Code == ErrResourceNotFound
	}
	return false
}

// IsUnauthorized checks if error is an unauthorized error
func IsUnauthorized(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrUnauthorized || 
		       appErr.Code == ErrTokenExpired || 
		       appErr.Code == ErrTokenInvalid
	}
	return false
}

// IsForbidden checks if error is a forbidden error
func IsForbidden(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrForbidden || 
		       appErr.Code == ErrInsufficientPermission
	}
	return false
}

// IsConflict checks if error is a conflict error
func IsConflict(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrConflict || 
		       appErr.Code == ErrResourceExists || 
		       appErr.Code == ErrDuplicateEntry
	}
	return false
}

// WrapDBError wraps database errors into AppError
func WrapDBError(err error, operation string) *AppError {
	if err == nil {
		return nil
	}
	
	// Check for common database errors
	errMsg := err.Error()
	
	// Duplicate entry
	if containsAny(errMsg, []string{"duplicate", "unique constraint", "UNIQUE constraint"}) {
		return New(ErrDuplicateEntry, "Record already exists").WithError(err)
	}
	
	// Foreign key constraint
	if containsAny(errMsg, []string{"foreign key", "FOREIGN KEY"}) {
		return New(ErrForeignKey, "Cannot delete record with dependencies").WithError(err)
	}
	
	// Record not found
	if containsAny(errMsg, []string{"record not found", "not found"}) {
		return New(ErrResourceNotFound, "Record not found").WithError(err)
	}
	
	// Generic database error
	return DatabaseError(operation).WithError(err)
}

// containsAny checks if string contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
	       (s == substr || 
	        (len(s) > len(substr) && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function for success responses

// Success sends a standardized success response
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// SuccessWithMessage sends a success response with a message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    data,
	})
}

// CreatedWithMessage sends a 201 Created response with message
func CreatedWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// NoContent sends a 204 No Content response
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Paginated sends a paginated response
func Paginated(c *gin.Context, data interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
		"pagination": gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}
