package handler

import (
	"homexai/internal/models/property"
	"homexai/pkg/errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Example: Announcement handler with unified error handling
type ErrorHandlingExampleHandler struct {
	db *gorm.DB
}

// GetAnnouncement demonstrates unified error handling
func (h *ErrorHandlingExampleHandler) GetAnnouncement(c *gin.Context) {
	announcementID := c.Param("id")
	propertyID := c.GetUint("property_id")

	var announcement property.Announcement
	err := h.db.Where("id = ? AND property_id = ?", announcementID, propertyID).
		First(&announcement).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Use predefined error
			errors.Response(c, errors.NotFound("Announcement"))
			return
		}
		// Wrap database error
		errors.Response(c, errors.WrapDBError(err, "fetch announcement"))
		return
	}

	// Success response
	errors.Success(c, announcement)
}

// CreateAnnouncement demonstrates validation and creation errors
func (h *ErrorHandlingExampleHandler) CreateAnnouncement(c *gin.Context) {
	// propertyID := c.GetUint("property_id")
	userRole := c.GetString("user_role")

	// Check permission
	if userRole != "property_admin" && userRole != "super_admin" {
		errors.Response(c, errors.InsufficientPermission("create announcements"))
		return
	}

	// Bind request
	var req struct {
		Title   string `json:"title" binding:"required,min=3,max=200"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Validation error with field details
		appErr := errors.Validation("Invalid input")
		appErr.WithField("title", "Title is required and must be 3-200 characters")
		appErr.WithField("content", "Content is required")
		errors.Response(c, appErr)
		return
	}

	// Create announcement
	announcement := property.Announcement{
		// PropertyID: propertyID,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := h.db.Create(&announcement).Error; err != nil {
		errors.Response(c, errors.WrapDBError(err, "create announcement"))
		return
	}

	// Success with 201 Created
	errors.CreatedWithMessage(c, "Announcement created successfully", announcement)
}

// UpdateAnnouncement demonstrates update with conflict handling
func (h *ErrorHandlingExampleHandler) UpdateAnnouncement(c *gin.Context) {
	announcementID := c.Param("id")
	propertyID := c.GetUint("property_id")

	// Check if announcement exists
	var existing property.Announcement
	if err := h.db.Where("id = ? AND property_id = ?", announcementID, propertyID).
		First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			errors.Response(c, errors.NotFound("Announcement"))
			return
		}
		errors.Response(c, errors.WrapDBError(err, "fetch announcement"))
		return
	}

	// Check if announcement is published (can't edit)
	if existing.Status == "published" {
		err := errors.Conflict("Cannot edit published announcement")
		err.WithDetails("Unpublish the announcement before editing")
		errors.Response(c, err)
		return
	}

	// Bind and update
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errors.Response(c, errors.Validation("Invalid input").WithError(err))
		return
	}

	// Update
	if err := h.db.Model(&existing).Updates(req).Error; err != nil {
		errors.Response(c, errors.WrapDBError(err, "update announcement"))
		return
	}

	// Success
	errors.SuccessWithMessage(c, "Announcement updated successfully", existing)
}

// DeleteAnnouncement demonstrates delete with dependencies check
func (h *ErrorHandlingExampleHandler) DeleteAnnouncement(c *gin.Context) {
	announcementID := c.Param("id")
	propertyID := c.GetUint("property_id")

	// Check if exists
	var announcement property.Announcement
	if err := h.db.Where("id = ? AND property_id = ?", announcementID, propertyID).
		First(&announcement).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			errors.Response(c, errors.NotFound("Announcement"))
			return
		}
		errors.Response(c, errors.WrapDBError(err, "fetch announcement"))
		return
	}

	// Delete
	if err := h.db.Delete(&announcement).Error; err != nil {
		// WrapDBError will detect foreign key errors
		errors.Response(c, errors.WrapDBError(err, "delete announcement"))
		return
	}

	// Success with no content
	errors.NoContent(c)
}

// ListAnnouncements demonstrates paginated response
func (h *ErrorHandlingExampleHandler) ListAnnouncements(c *gin.Context) {
	propertyID := c.GetUint("property_id")

	// Parse pagination
	page := 1
	pageSize := 20

	var announcements []property.Announcement
	var total int64

	db := h.db.Where("property_id = ?", propertyID)

	// Count total
	if err := db.Model(&property.Announcement{}).Count(&total).Error; err != nil {
		errors.Response(c, errors.WrapDBError(err, "count announcements"))
		return
	}

	// Fetch page
	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&announcements).Error; err != nil {
		errors.Response(c, errors.WrapDBError(err, "fetch announcements"))
		return
	}

	// Paginated response
	errors.Paginated(c, announcements, total, page, pageSize)
}

// UploadAttachment demonstrates file upload error handling
func (h *ErrorHandlingExampleHandler) UploadAttachment(c *gin.Context) {
	// Get file
	file, err := c.FormFile("file")
	if err != nil {
		errors.Response(c, errors.BadRequest("No file uploaded"))
		return
	}

	// Check file size (10MB limit)
	maxSize := int64(10 * 1024 * 1024)
	if file.Size > maxSize {
		errors.Response(c, errors.FileTooLarge(maxSize))
		return
	}

	// Check file type
	allowedTypes := []string{"image/jpeg", "image/png", "application/pdf"}
	contentType := file.Header.Get("Content-Type")

	isAllowed := false
	for _, t := range allowedTypes {
		if t == contentType {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		errors.Response(c, errors.InvalidFileType(allowedTypes))
		return
	}

	// Save file (simplified)
	// ...

	errors.CreatedWithMessage(c, "File uploaded successfully", gin.H{
		"filename": file.Filename,
		"size":     file.Size,
	})
}

// ProcessPayment demonstrates business logic errors
func (h *ErrorHandlingExampleHandler) ProcessPayment(c *gin.Context) {
	billID := c.Param("bill_id")
	propertyID := c.GetUint("property_id")

	// Get bill
	var bill property.Bill
	if err := h.db.Where("id = ? AND property_id = ?", billID, propertyID).
		First(&bill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			errors.Response(c, errors.NotFound("Bill"))
			return
		}
		errors.Response(c, errors.WrapDBError(err, "fetch bill"))
		return
	}

	// Check bill state
	if bill.Status == "paid" {
		err := errors.New(errors.ErrInvalidState, "Bill already paid")
		err.WithDetails("This bill has already been paid and cannot be paid again")
		errors.Response(c, err)
		return
	}

	if bill.Status == "cancelled" {
		err := errors.New(errors.ErrInvalidState, "Bill is cancelled")
		err.WithDetails("Cancelled bills cannot be paid")
		errors.Response(c, err)
		return
	}

	// Process payment (simplified)
	// ...

	// Update bill status
	bill.Status = "paid"
	if err := h.db.Save(&bill).Error; err != nil {
		errors.Response(c, errors.WrapDBError(err, "update bill"))
		return
	}

	errors.SuccessWithMessage(c, "Payment processed successfully", bill)
}

// RateLimitedEndpoint demonstrates rate limiting errors
func (h *ErrorHandlingExampleHandler) RateLimitedEndpoint(c *gin.Context) {
	userID := c.GetUint("user_id")

	// Check rate limit (simplified)
	requestCount := getUserRequestCount(userID)
	limit := 100

	if requestCount > limit {
		err := errors.RateLimitExceeded(limit, "hour")
		err.WithDetails("Please try again after one hour")
		errors.Response(c, err)
		return
	}

	// Process request
	// ...

	errors.Success(c, gin.H{"message": "Request processed"})
}

// Helper function
func getUserRequestCount(userID uint) int {
	// Simplified - would use Redis in production
	return 0
}

// Example: Using error helpers

// AuthExample demonstrates authentication errors
func AuthExample(c *gin.Context) {
	token := c.GetHeader("Authorization")

	if token == "" {
		errors.Response(c, errors.Unauthorized("Missing authorization token"))
		return
	}

	// Validate token
	valid := false // Simplified
	if !valid {
		errors.Response(c, errors.TokenInvalid())
		return
	}

	// Check if expired
	expired := true // Simplified
	if expired {
		errors.Response(c, errors.TokenExpired())
		return
	}

	// Success
	errors.Success(c, gin.H{"message": "Authenticated"})
}

// PermissionExample demonstrates authorization errors
func PermissionExample(c *gin.Context) {
	userRole := c.GetString("user_role")

	// Check permission
	if userRole != "admin" {
		errors.Response(c, errors.InsufficientPermission("access admin panel"))
		return
	}

	// Success
	errors.Success(c, gin.H{"message": "Access granted"})
}

// DuplicateExample demonstrates duplicate entry errors
func DuplicateExample(c *gin.Context, db *gorm.DB) {
	email := "test@example.com"

	// Check if user exists
	var count int64
	db.Model(&property.Unit{}).Where("email = ?", email).Count(&count)

	if count > 0 {
		errors.Response(c, errors.ResourceExists("User with this email"))
		return
	}

	// Create user
	// ...

	errors.Created(c, gin.H{"message": "User created"})
}

// ComplexValidationExample demonstrates field-specific validation errors
func ComplexValidationExample(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Age      int    `json:"age"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errors.Response(c, errors.Validation("Validation failed").WithError(err))
		return
	}

	// Custom validation
	appErr := errors.Validation("Please fix the following errors")
	hasError := false

	if len(req.Email) == 0 {
		appErr.WithField("email", "Email is required")
		hasError = true
	}

	if len(req.Password) < 8 {
		appErr.WithField("password", "Password must be at least 8 characters")
		hasError = true
	}

	if req.Age < 18 {
		appErr.WithField("age", "Must be 18 or older")
		hasError = true
	}

	if hasError {
		errors.Response(c, appErr)
		return
	}

	// Success
	errors.Success(c, gin.H{"message": "Validation passed"})
}

// ChainedErrorExample demonstrates error chaining
func ChainedErrorExample(c *gin.Context, db *gorm.DB) {
	// Get user
	var user property.Tenant
	err := db.First(&user, 123).Error

	if err != nil {
		// Wrap the database error with context
		appErr := errors.NotFound("User")
		appErr.WithError(err)
		appErr.WithDetails("User ID 123 not found in database")
		errors.Response(c, appErr)
		return
	}

	// Success
	errors.Success(c, user)
}
