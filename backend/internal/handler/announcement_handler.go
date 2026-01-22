package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"homexai/internal/database"
	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AnnouncementHandler handles announcement-related requests
type AnnouncementHandler struct {
	masterDB *gorm.DB
	smtpSvc  *service.SmtpService
}

// NewAnnouncementHandler creates a new AnnouncementHandler
func NewAnnouncementHandler(masterDB *gorm.DB, smtpSvc *service.SmtpService) *AnnouncementHandler {
	return &AnnouncementHandler{
		masterDB: masterDB,
		smtpSvc:  smtpSvc,
	}
}

// CreateAnnouncementRequest represents the request body for creating an announcement
type CreateAnnouncementRequest struct {
	Title    string `json:"title" binding:"required,max=200"`
	Content  string `json:"content" binding:"required"`
	Category string `json:"category"`
	Priority string `json:"priority"`
}

// UpdateAnnouncementRequest represents the request body for updating an announcement
type UpdateAnnouncementRequest struct {
	Title    string `json:"title" binding:"max=200"`
	Content  string `json:"content"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// AnnouncementAttachment represents an attachment in the response
type AnnouncementAttachment struct {
	ID           uint   `json:"id"`
	DocumentName string `json:"document_name"`
	DocumentPath string `json:"document_path"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	UploadedAt   string `json:"uploaded_at"`
}

// AnnouncementResponse represents an announcement with attachments
type AnnouncementResponse struct {
	ID          uint                     `json:"id"`
	Title       string                   `json:"title"`
	Content     string                   `json:"content"`
	Category    string                   `json:"category"`
	Priority    string                   `json:"priority"`
	Status      string                   `json:"status"`
	PublishedAt *time.Time               `json:"published_at"`
	CreatedBy   uint                     `json:"created_by"`
	AuthorName  string                   `json:"author_name"`
	Attachments []AnnouncementAttachment `json:"attachments"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

// Create creates a new announcement (supports both JSON and multipart form)
func (h *AnnouncementHandler) Create(c *gin.Context) {
	// Get property database
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	// Get user ID, role and property ID
	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)
	propertyID := middleware.GetPropertyID(c)
	
	// Only property_admin can create announcements
	if userRole != "property_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property admin can create announcements"})
		return
	}

	var title, content, category, priority string
	var sendEmail bool

	// Try to parse as multipart form first
	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		title = c.PostForm("title")
		content = c.PostForm("content")
		category = c.PostForm("category")
		priority = c.PostForm("priority")
		sendEmail = c.PostForm("send_email") == "true"
	} else {
		// Parse as JSON
		var req struct {
			Title     string `json:"title"`
			Content   string `json:"content"`
			Category  string `json:"category"`
			Priority  string `json:"priority"`
			SendEmail bool   `json:"send_email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		title = req.Title
		content = req.Content
		category = req.Category
		priority = req.Priority
		sendEmail = req.SendEmail
	}

	// Validate required fields
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	// Set defaults
	if category == "" {
		category = property.AnnouncementCategoryGeneral
	}
	if priority == "" {
		priority = property.AnnouncementPriorityNormal
	}

	now := time.Now()
	announcement := property.Announcement{
		Title:       title,
		Content:     content,
		Category:    category,
		Priority:    priority,
		Status:      property.AnnouncementStatusPublished,
		PublishedAt: &now,
		CreatedBy:   userID,
	}

	if err := propertyDB.Create(&announcement).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create announcement"})
		return
	}

	// Handle file uploads
	h.saveAnnouncementAttachments(c, propertyDB, announcement.ID, userID)

	// Send email notifications if requested
	if sendEmail && h.smtpSvc != nil && h.masterDB != nil {
		go h.sendAnnouncementEmails(propertyID, title, content, priority)
	}

	// Create in-app notifications for all property users
	go h.createAnnouncementNotifications(propertyID, announcement.ID, title, priority)

	// Build response with attachments
	resp := h.buildAnnouncementResponse(announcement, propertyDB)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Announcement created successfully",
		"data":    resp,
	})
}

// sendAnnouncementEmails sends email notifications to all property users
func (h *AnnouncementHandler) sendAnnouncementEmails(propertyID uint, title, content, priority string) {
	// Get property info
	var prop master.Property
	propertyName := "HomeX Property"
	subdomain := ""
	if err := h.masterDB.First(&prop, propertyID).Error; err == nil {
		propertyName = prop.Name
		subdomain = prop.Subdomain
	}

	// Get all active users for this property
	var userRoles []master.UserRole
	if err := h.masterDB.Where("property_id = ? AND status = ?", propertyID, "active").Find(&userRoles).Error; err != nil {
		fmt.Printf("Failed to get users for property %d: %v\n", propertyID, err)
		return
	}

	// Collect unique user IDs
	userIDSet := make(map[uint]bool)
	for _, ur := range userRoles {
		userIDSet[ur.UserID] = true
	}

	// Get user emails
	userIDs := make([]uint, 0, len(userIDSet))
	for uid := range userIDSet {
		userIDs = append(userIDs, uid)
	}

	var users []master.User
	if err := h.masterDB.Where("id IN ? AND status = ?", userIDs, "active").Find(&users).Error; err != nil {
		fmt.Printf("Failed to get user emails: %v\n", err)
		return
	}

	// Send emails
	successCount := 0
	failCount := 0
	for _, user := range users {
		if user.Email != nil && *user.Email != "" {
			if err := h.smtpSvc.SendAnnouncementEmail(*user.Email, title, content, priority, propertyName, subdomain); err != nil {
				fmt.Printf("Failed to send announcement email to %s: %v\n", *user.Email, err)
				failCount++
			} else {
				successCount++
			}
		}
	}

	fmt.Printf("📢 Announcement emails sent: %d success, %d failed (property: %s)\n", successCount, failCount, propertyName)
}

// createAnnouncementNotifications creates in-app notifications for all property users
func (h *AnnouncementHandler) createAnnouncementNotifications(propertyID uint, announcementID uint, title string, priority string) {
	// Get property info to get database name
	var prop master.Property
	if err := h.masterDB.First(&prop, propertyID).Error; err != nil {
		fmt.Printf("Failed to get property info for notifications: %v\n", err)
		return
	}

	// Get property database connection
	propertyDB, err := database.GetPropertyDBBySubdomain(prop.Subdomain)
	if err != nil {
		fmt.Printf("Failed to get property database for notifications: %v\n", err)
		return
	}

	// Get all active users for this property
	var userRoles []master.UserRole
	if err := h.masterDB.Where("property_id = ? AND status = ?", propertyID, "active").Find(&userRoles).Error; err != nil {
		fmt.Printf("Failed to get users for property %d: %v\n", propertyID, err)
		return
	}

	// Collect unique user IDs
	userIDSet := make(map[uint]bool)
	for _, ur := range userRoles {
		userIDSet[ur.UserID] = true
	}

	// Create notifications for all users
	notificationService := service.NewNotificationService(propertyDB)
	successCount := 0
	failCount := 0
	for userID := range userIDSet {
		if err := notificationService.CreateAnnouncementNotification(userID, propertyID, announcementID, title); err != nil {
			fmt.Printf("Failed to create notification for user %d: %v\n", userID, err)
			failCount++
		} else {
			successCount++
			// Send email notification if user has email notifications enabled
			go h.sendNotificationEmail(userID, propertyID, "New Announcement", 
				"A new announcement \""+title+"\" has been published.", priority)
		}
	}

	fmt.Printf("📢 Announcement notifications created: %d success, %d failed (property: %d)\n", successCount, failCount, propertyID)
}

// sendNotificationEmail sends email notification if user has email notifications enabled
func (h *AnnouncementHandler) sendNotificationEmail(userID uint, propertyID uint, subject, content, priority string) {
	if h.smtpSvc == nil {
		return
	}

	// Get user info
	var user master.User
	if err := h.masterDB.First(&user, userID).Error; err != nil {
		return
	}

	// Check if user has email notifications enabled
	if !user.EmailNotificationEnabled {
		return
	}

	// Check if user has email
	if user.Email == nil || *user.Email == "" {
		return
	}

	// Get property info
	var prop master.Property
	propertyName := "HomeX Property"
	subdomain := ""
	if err := h.masterDB.First(&prop, propertyID).Error; err == nil {
		propertyName = prop.Name
		subdomain = prop.Subdomain
	}

	// Send email notification
	if err := h.smtpSvc.SendNotificationEmail(*user.Email, subject, content, propertyName, subdomain); err != nil {
		fmt.Printf("Failed to send notification email to %s: %v\n", *user.Email, err)
	}
}

// List returns all announcements
func (h *AnnouncementHandler) List(c *gin.Context) {
	// Get property database
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	// Get query parameters
	status := c.Query("status")
	priority := c.Query("priority")

	query := propertyDB.Model(&property.Announcement{}).Order("created_at DESC")

	// Filter by status
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		// By default, only show published announcements for non-staff
		userRole := middleware.GetUserRole(c)
		if userRole != "property_admin" && userRole != "property_staff" {
			query = query.Where("status = ?", property.AnnouncementStatusPublished)
		}
	}

	// Filter by priority
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	var announcements []property.Announcement
	if err := query.Find(&announcements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcements"})
		return
	}

	// Build response with attachments
	responses := make([]AnnouncementResponse, len(announcements))
	for i, a := range announcements {
		responses[i] = h.buildAnnouncementResponse(a, propertyDB)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": responses,
	})
}

// Get returns a single announcement
func (h *AnnouncementHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
		return
	}

	// Get property database
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	var announcement property.Announcement
	if err := propertyDB.First(&announcement, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Announcement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcement"})
		return
	}

	resp := h.buildAnnouncementResponse(announcement, propertyDB)

	c.JSON(http.StatusOK, gin.H{
		"data": resp,
	})
}

// Update updates an announcement
func (h *AnnouncementHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
		return
	}

	var req UpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get property database
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}
	
	// Only property_admin can update announcements
	userRole := middleware.GetUserRole(c)
	if userRole != "property_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property admin can update announcements"})
		return
	}

	var announcement property.Announcement
	if err := propertyDB.First(&announcement, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Announcement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcement"})
		return
	}

	// Update fields
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == property.AnnouncementStatusPublished && announcement.PublishedAt == nil {
			now := time.Now()
			updates["published_at"] = &now
		}
	}

	if err := propertyDB.Model(&announcement).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update announcement"})
		return
	}

	// Reload
	propertyDB.First(&announcement, id)

	resp := h.buildAnnouncementResponse(announcement, propertyDB)

	c.JSON(http.StatusOK, gin.H{
		"message": "Announcement updated successfully",
		"data":    resp,
	})
}

// Delete deletes an announcement
func (h *AnnouncementHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid announcement ID"})
		return
	}

	// Get property database
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}
	
	// Only property_admin can delete announcements
	userRole := middleware.GetUserRole(c)
	if userRole != "property_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property admin can delete announcements"})
		return
	}

	var announcement property.Announcement
	if err := propertyDB.First(&announcement, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Announcement not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch announcement"})
		return
	}

	// Delete associated attachments
	propertyDB.Where("entity_type = ? AND entity_id = ?", property.DocEntityAnnouncement, id).Delete(&property.Document{})

	if err := propertyDB.Delete(&announcement).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete announcement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Announcement deleted successfully",
	})
}

// buildAnnouncementResponse builds a response with attachments
func (h *AnnouncementHandler) buildAnnouncementResponse(a property.Announcement, db *gorm.DB) AnnouncementResponse {
	resp := AnnouncementResponse{
		ID:          a.ID,
		Title:       a.Title,
		Content:     a.Content,
		Category:    a.Category,
		Priority:    a.Priority,
		Status:      a.Status,
		PublishedAt: a.PublishedAt,
		CreatedBy:   a.CreatedBy,
		AuthorName:  "Property Admin",
		Attachments: []AnnouncementAttachment{},
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}

	// Load attachments
	if db != nil {
		var docs []property.Document
		if err := db.Where("entity_type = ? AND entity_id = ? AND is_active = ?", property.DocEntityAnnouncement, a.ID, true).
			Order("uploaded_at DESC").Find(&docs).Error; err == nil {
			for _, doc := range docs {
				att := AnnouncementAttachment{
					ID:           doc.ID,
					DocumentName: doc.DocumentName,
					DocumentPath: doc.DocumentPath,
					UploadedAt:   doc.UploadedAt.Format("2006-01-02 15:04:05"),
				}
				if doc.FileSize != nil {
					att.FileSize = *doc.FileSize
				}
				if doc.MimeType != nil {
					att.MimeType = *doc.MimeType
				}
				resp.Attachments = append(resp.Attachments, att)
			}
		}
	}

	return resp
}

// saveAnnouncementAttachments saves uploaded files and creates document records
func (h *AnnouncementHandler) saveAnnouncementAttachments(c *gin.Context, db *gorm.DB, announcementID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	files := form.File["attachments"]
	if len(files) == 0 {
		return nil
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/announcements/%d", announcementID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	for _, file := range files {
		// Validate file size (max 10MB)
		if file.Size > 10*1024*1024 {
			continue
		}

		// Generate unique filename
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitizeAnnouncementFilename(file.Filename), ext)
		filePath := filepath.Join(uploadDir, filename)

		// Save file
		src, err := file.Open()
		if err != nil {
			continue
		}
		defer src.Close()

		dst, err := os.Create(filePath)
		if err != nil {
			continue
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			continue
		}

		// Determine MIME type
		mimeType := file.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		// Create document record
		doc := property.Document{
			EntityType:   property.DocEntityAnnouncement,
			EntityID:     announcementID,
			DocumentType: property.DocTypeOther,
			DocumentName: file.Filename,
			DocumentPath: filePath,
			FileSize:     &file.Size,
			MimeType:     &mimeType,
			UploadedBy:   userID,
			IsActive:     true,
		}

		if err := db.Create(&doc).Error; err != nil {
			continue
		}
	}

	return nil
}

// sanitizeAnnouncementFilename removes special characters from filename
func sanitizeAnnouncementFilename(name string) string {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]

	result := make([]byte, 0, len(base))
	for i := 0; i < len(base); i++ {
		c := base[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "file"
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result)
}
