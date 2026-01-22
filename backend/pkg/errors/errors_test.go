package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "Simple error",
			err:      New(ErrNotFound, "Resource not found"),
			expected: "RESOURCE_NOT_FOUND: Resource not found",
		},
		{
			name:     "Error with underlying error",
			err:      New(ErrDatabase, "Database error").WithError(fmt.Errorf("connection timeout")),
			expected: "DATABASE_ERROR: Database error (caused by: connection timeout)",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestAppError_WithDetails(t *testing.T) {
	err := New(ErrNotFound, "Resource not found")
	err.WithDetails("ID 123 not found in database")
	
	assert.Equal(t, "ID 123 not found in database", err.Details)
}

func TestAppError_WithField(t *testing.T) {
	err := Validation("Validation failed")
	err.WithField("email", "Email is required")
	err.WithField("password", "Password must be at least 8 characters")
	
	assert.Equal(t, 2, len(err.Fields))
	assert.Equal(t, "Email is required", err.Fields["email"])
	assert.Equal(t, "Password must be at least 8 characters", err.Fields["password"])
}

func TestAppError_WithFields(t *testing.T) {
	err := Validation("Validation failed")
	fields := map[string]string{
		"name":  "Name is required",
		"email": "Invalid email format",
	}
	err.WithFields(fields)
	
	assert.Equal(t, 2, len(err.Fields))
	assert.Equal(t, "Name is required", err.Fields["name"])
	assert.Equal(t, "Invalid email format", err.Fields["email"])
}

func TestNew(t *testing.T) {
	err := New(ErrBadRequest, "Invalid input")
	
	assert.Equal(t, ErrBadRequest, err.Code)
	assert.Equal(t, "Invalid input", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.StatusCode)
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantCode   ErrorCode
		wantStatus int
	}{
		{
			name:       "BadRequest",
			err:        BadRequest("Invalid input"),
			wantCode:   ErrBadRequest,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Unauthorized",
			err:        Unauthorized("Not authenticated"),
			wantCode:   ErrUnauthorized,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Forbidden",
			err:        Forbidden("Access denied"),
			wantCode:   ErrForbidden,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "NotFound",
			err:        NotFound("User"),
			wantCode:   ErrResourceNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Conflict",
			err:        Conflict("Resource exists"),
			wantCode:   ErrConflict,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "Internal",
			err:        Internal("Server error"),
			wantCode:   ErrInternal,
			wantStatus: http.StatusInternalServerError,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantCode, tt.err.Code)
			assert.Equal(t, tt.wantStatus, tt.err.StatusCode)
		})
	}
}

func TestResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name       string
		err        *AppError
		wantStatus int
		wantBody   map[string]interface{}
	}{
		{
			name:       "Simple error",
			err:        NotFound("User"),
			wantStatus: http.StatusNotFound,
			wantBody: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    string(ErrResourceNotFound),
					"message": "User not found",
				},
			},
		},
		{
			name: "Error with details",
			err:  NotFound("User").WithDetails("ID 123 not found"),
			wantStatus: http.StatusNotFound,
			wantBody: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    string(ErrResourceNotFound),
					"message": "User not found",
					"details": "ID 123 not found",
				},
			},
		},
		{
			name: "Error with fields",
			err: Validation("Validation failed").
				WithField("email", "Email is required").
				WithField("password", "Password too short"),
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]interface{}{
				"error": map[string]interface{}{
					"code":    string(ErrValidation),
					"message": "Validation failed",
					"fields": map[string]interface{}{
						"email":    "Email is required",
						"password": "Password too short",
					},
				},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			Response(c, tt.err)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			
			assert.Equal(t, tt.wantBody, response)
		})
	}
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, IsNotFound(NotFound("User")))
	assert.False(t, IsNotFound(BadRequest("Invalid")))
}

func TestIsUnauthorized(t *testing.T) {
	assert.True(t, IsUnauthorized(Unauthorized("Not authenticated")))
	assert.True(t, IsUnauthorized(TokenExpired()))
	assert.True(t, IsUnauthorized(TokenInvalid()))
	assert.False(t, IsUnauthorized(NotFound("User")))
}

func TestIsForbidden(t *testing.T) {
	assert.True(t, IsForbidden(Forbidden("Access denied")))
	assert.True(t, IsForbidden(InsufficientPermission("delete")))
	assert.False(t, IsForbidden(Unauthorized("Not authenticated")))
}

func TestIsConflict(t *testing.T) {
	assert.True(t, IsConflict(Conflict("Already exists")))
	assert.True(t, IsConflict(ResourceExists("User")))
	assert.False(t, IsConflict(NotFound("User")))
}

func TestWrapDBError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode ErrorCode
	}{
		{
			name:     "Duplicate entry",
			err:      fmt.Errorf("Error 1062: Duplicate entry"),
			wantCode: ErrDuplicateEntry,
		},
		{
			name:     "Foreign key",
			err:      fmt.Errorf("foreign key constraint failed"),
			wantCode: ErrForeignKey,
		},
		{
			name:     "Record not found",
			err:      fmt.Errorf("record not found"),
			wantCode: ErrResourceNotFound,
		},
		{
			name:     "Generic error",
			err:      fmt.Errorf("connection timeout"),
			wantCode: ErrDatabase,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapDBError(tt.err, "test operation")
			assert.NotNil(t, wrapped)
			assert.Equal(t, tt.wantCode, wrapped.Code)
			assert.Equal(t, tt.err, wrapped.Err)
		})
	}
}

func TestSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	data := map[string]interface{}{
		"id":   1,
		"name": "Test",
	}
	
	Success(c, data)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["data"])
}

func TestSuccessWithMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	SuccessWithMessage(c, "Operation successful", map[string]interface{}{"id": 1})
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	assert.Equal(t, "Operation successful", response["message"])
}

func TestCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	Created(c, map[string]interface{}{"id": 1})
	
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	NoContent(c)
	
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPaginated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	data := []map[string]interface{}{
		{"id": 1, "name": "Item 1"},
		{"id": 2, "name": "Item 2"},
	}
	
	Paginated(c, data, 100, 1, 20)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	
	pagination := response["pagination"].(map[string]interface{})
	assert.Equal(t, float64(100), pagination["total"])
	assert.Equal(t, float64(1), pagination["page"])
	assert.Equal(t, float64(20), pagination["page_size"])
	assert.Equal(t, float64(5), pagination["total_pages"]) // 100 / 20 = 5
}

func TestErrorCodes(t *testing.T) {
	// Test that all error codes map to correct HTTP status
	tests := []struct {
		code       ErrorCode
		wantStatus int
	}{
		{ErrBadRequest, http.StatusBadRequest},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrForbidden, http.StatusForbidden},
		{ErrNotFound, http.StatusNotFound},
		{ErrConflict, http.StatusConflict},
		{ErrTooManyRequest, http.StatusTooManyRequests},
		{ErrInternal, http.StatusInternalServerError},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			status := getHTTPStatus(tt.code)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

// Benchmark tests

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(ErrNotFound, "Resource not found")
	}
}

func BenchmarkResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)
	err := NotFound("User")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		Response(c, err)
	}
}

func BenchmarkWithFields(b *testing.B) {
	for i := 0; i < b.N; i++ {
		err := Validation("Validation failed")
		err.WithField("field1", "Error 1")
		err.WithField("field2", "Error 2")
		err.WithField("field3", "Error 3")
	}
}
