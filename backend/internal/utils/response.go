package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents the standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"total_pages"`
}

// SuccessResponse sends a success response
func SuccessResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// SuccessResponseWithMeta sends a success response with metadata
func SuccessResponseWithMeta(c *gin.Context, message string, data interface{}, meta interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// CreatedResponse sends a 201 created response
func CreatedResponse(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, statusCode int, message string, err error) {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}

	c.JSON(statusCode, Response{
		Success: false,
		Message: message,
		Error:   errorMessage,
	})
}

// BadRequestResponse sends a 400 bad request response
func BadRequestResponse(c *gin.Context, message string, err error) {
	ErrorResponse(c, http.StatusBadRequest, message, err)
}

// UnauthorizedResponse sends a 401 unauthorized response
func UnauthorizedResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusUnauthorized, message, nil)
}

// ForbiddenResponse sends a 403 forbidden response
func ForbiddenResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusForbidden, message, nil)
}

// NotFoundResponse sends a 404 not found response
func NotFoundResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusNotFound, message, nil)
}

// ConflictResponse sends a 409 conflict response
func ConflictResponse(c *gin.Context, message string, err error) {
	ErrorResponse(c, http.StatusConflict, message, err)
}

// InternalServerErrorResponse sends a 500 internal server error response
func InternalServerErrorResponse(c *gin.Context, message string, err error) {
	ErrorResponse(c, http.StatusInternalServerError, message, err)
}

// ValidationErrorResponse sends a validation error response
func ValidationErrorResponse(c *gin.Context, errors interface{}) {
	c.JSON(http.StatusUnprocessableEntity, Response{
		Success: false,
		Message: "Validation failed",
		Data:    errors,
	})
}

// NoContentResponse sends a 204 no content response
func NoContentResponse(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// SuccessResponseWithPagination sends a success response with pagination
func SuccessResponseWithPagination(c *gin.Context, data interface{}, total int64, page, perPage int, message string) {
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
		Meta: PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			TotalPages:  totalPages,
		},
	})
}

// Success sends a simple success response (short alias)
func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// SuccessWithCode sends a success response with custom status code
func SuccessWithCode(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// SuccessWithPagination sends a success response with pagination (short alias)
func SuccessWithPagination(c *gin.Context, message string, data interface{}, total int64, page, perPage int) {
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: message,
		Data:    data,
		Meta: PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			TotalPages:  totalPages,
		},
	})
}

// Error sends an error response (short alias)
func Error(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, Response{
		Success: false,
		Message: message,
	})
}

// ErrorWithData sends an error response with additional data
func ErrorWithData(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, Response{
		Success: false,
		Message: message,
		Data:    data,
	})
}

// ValidationError sends a validation error response (short alias)
func ValidationError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, Response{
		Success: false,
		Message: "Validation failed",
		Error:   err.Error(),
	})
}
