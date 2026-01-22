package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ComplaintHandler struct {
	masterDB *gorm.DB
}

func NewComplaintHandler(masterDB *gorm.DB) *ComplaintHandler {
	return &ComplaintHandler{
		masterDB: masterDB,
	}
}

// ComplaintAttachment represents an attachment in API response
type ComplaintAttachment struct {
	ID           uint   `json:"id"`
	DocumentName string `json:"document_name"`
	DocumentPath string `json:"document_path"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
}

// ComplaintMessageResponse represents a message in complaint conversation
type ComplaintMessageResponse struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"user_id"`
	Message     string    `json:"message"`
	IsFromStaff bool      `json:"is_from_staff"`
	CreatedAt   time.Time `json:"created_at"`
	UserName    string    `json:"user_name,omitempty"`
}

// ComplaintResponse represents the API response for a complaint
type ComplaintResponse struct {
	ID            uint                       `json:"id"`
	UserID        uint                       `json:"user_id"`
	UnitID        *uint                      `json:"unit_id"`
	ParkingID     *uint                      `json:"parking_id"`
	Title         string                     `json:"title"`
	Description   *string                    `json:"description"`
	Priority      string                     `json:"priority"`
	Status        string                     `json:"status"`
	CreatedAt     time.Time                  `json:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	UnitNumber    string                     `json:"unit_number,omitempty"`
	ParkingNumber string                     `json:"parking_number,omitempty"`
	Attachments   []ComplaintAttachment      `json:"attachments"`
	Messages      []ComplaintMessageResponse `json:"messages,omitempty"`
}

func (h *ComplaintHandler) buildComplaintResponse(complaint property.Complaint, db *gorm.DB) ComplaintResponse {
	resp := ComplaintResponse{
		ID:          complaint.ID,
		UserID:      complaint.UserID,
		UnitID:      complaint.UnitID,
		ParkingID:   complaint.ParkingID,
		Title:       complaint.Title,
		Description: complaint.Description,
		Priority:    complaint.Priority,
		Status:      complaint.Status,
		CreatedAt:   complaint.CreatedAt,
		UpdatedAt:   complaint.UpdatedAt,
		Attachments: []ComplaintAttachment{},
		Messages:    []ComplaintMessageResponse{},
	}

	if complaint.Unit != nil {
		resp.UnitNumber = complaint.Unit.UnitNumber
	}
	if complaint.Parking != nil {
		resp.ParkingNumber = complaint.Parking.SlotNumber
	}

	// Get attachments
	if db != nil {
		var docs []property.Document
		if err := db.Where("entity_type = ? AND entity_id = ? AND is_active = ?",
			property.DocEntityComplaint, complaint.ID, true).Find(&docs).Error; err == nil {
			for _, doc := range docs {
				att := ComplaintAttachment{
					ID:           doc.ID,
					DocumentName: doc.DocumentName,
					DocumentPath: doc.DocumentPath,
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

		// Get messages
		var messages []property.ComplaintMessage
		if err := db.Where("complaint_id = ?", complaint.ID).
			Order("created_at ASC").
			Find(&messages).Error; err == nil {
			// Collect user IDs
			userIDs := make([]uint, 0, len(messages))
			for _, msg := range messages {
				userIDs = append(userIDs, msg.UserID)
			}
			
			// Load users from master DB
			usersMap := make(map[uint]*master.User)
			if len(userIDs) > 0 && h.masterDB != nil {
				var users []master.User
				if err := h.masterDB.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
					for i := range users {
						usersMap[users[i].ID] = &users[i]
					}
				}
			}
			
			// Build message responses with user names
			for _, msg := range messages {
				msgResp := ComplaintMessageResponse{
					ID:          msg.ID,
					UserID:      msg.UserID,
					Message:     msg.Message,
					IsFromStaff: msg.IsFromStaff,
					CreatedAt:   msg.CreatedAt,
				}
				if user, ok := usersMap[msg.UserID]; ok {
					msgResp.UserName = user.GetDisplayName()
				}
				resp.Messages = append(resp.Messages, msgResp)
			}
		}
	}

	return resp
}

// ListMyComplaints returns complaints for the current user
// @Summary List my complaints
// @Tags Complaints
// @Produce json
// @Success 200 {array} ComplaintResponse
// @Router /complaints/my [get]
func (h *ComplaintHandler) ListMyComplaints(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)

	var complaints []property.Complaint
	if err := propertyDB.Where("user_id = ?", userID).
		Preload("Unit").Preload("Parking").
		Order("created_at DESC").
		Find(&complaints).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch complaints"})
		return
	}

	var responses []ComplaintResponse
	for _, complaint := range complaints {
		responses = append(responses, h.buildComplaintResponse(complaint, propertyDB))
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// CreateComplaint creates a new complaint (supports both JSON and multipart form)
// @Summary Create complaint
// @Tags Complaints
// @Accept json
// @Produce json
// @Success 201 {object} ComplaintResponse
// @Router /complaints [post]
func (h *ComplaintHandler) CreateComplaint(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)

	var title, description, priority string
	var unitID, parkingID *uint

	// Try to parse as multipart form first
	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		title = c.PostForm("title")
		description = c.PostForm("description")
		priority = c.PostForm("priority")

		// Parse unit_id
		if unitIDStr := c.PostForm("unit_id"); unitIDStr != "" {
			if id, err := strconv.ParseUint(unitIDStr, 10, 32); err == nil {
				uid := uint(id)
				unitID = &uid
			}
		}

		// Parse parking_id
		if parkingIDStr := c.PostForm("parking_id"); parkingIDStr != "" {
			if id, err := strconv.ParseUint(parkingIDStr, 10, 32); err == nil {
				pid := uint(id)
				parkingID = &pid
			}
		}
	} else {
		// Parse as JSON
		var body struct {
			Title       string `json:"title" binding:"required,max=255"`
			Description string `json:"description"`
			Priority    string `json:"priority"`
			UnitID      *uint  `json:"unit_id"`
			ParkingID   *uint  `json:"parking_id"`
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		title = body.Title
		description = body.Description
		priority = body.Priority
		unitID = body.UnitID
		parkingID = body.ParkingID
	}

	// Validate required fields
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}

	if priority == "" {
		priority = property.ComplaintPriorityNormal
	}

	complaint := property.Complaint{
		UserID:      userID,
		UnitID:      unitID,
		ParkingID:   parkingID,
		Title:       title,
		Description: &description,
		Priority:    priority,
		Status:      property.ComplaintStatusOpen,
	}

	if err := propertyDB.Create(&complaint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create complaint"})
		return
	}

	// Handle file uploads
	h.saveComplaintAttachments(c, propertyDB, complaint.ID, userID)

	propertyDB.Preload("Unit").Preload("Parking").First(&complaint, complaint.ID)
	c.JSON(http.StatusCreated, gin.H{"data": h.buildComplaintResponse(complaint, propertyDB), "message": "Complaint created successfully"})
}

// GetComplaint returns a single complaint
// @Summary Get complaint
// @Tags Complaints
// @Produce json
// @Param id path int true "Complaint ID"
// @Success 200 {object} ComplaintResponse
// @Router /complaints/{id} [get]
func (h *ComplaintHandler) GetComplaint(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid complaint ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var complaint property.Complaint
	if err := propertyDB.Preload("Unit").Preload("Parking").First(&complaint, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Complaint not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch complaint"})
		return
	}

	// Check ownership (non-staff can only view their own)
	if userRole != "property_admin" && userRole != "property_staff" && complaint.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.buildComplaintResponse(complaint, propertyDB)})
}

// ListAllComplaints returns all complaints (for staff)
// @Summary List all complaints
// @Tags Complaints
// @Produce json
// @Success 200 {array} ComplaintResponse
// @Router /complaints [get]
func (h *ComplaintHandler) ListAllComplaints(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	status := c.Query("status")
	priority := c.Query("priority")

	query := propertyDB.Preload("Unit").Preload("Parking").Order("created_at DESC")

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}

	var complaints []property.Complaint
	if err := query.Find(&complaints).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch complaints"})
		return
	}

	var responses []ComplaintResponse
	for _, complaint := range complaints {
		responses = append(responses, h.buildComplaintResponse(complaint, propertyDB))
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// GetComplaintMessages returns messages for a complaint
// @Summary Get complaint messages
// @Tags Complaints
// @Produce json
// @Param id path int true "Complaint ID"
// @Success 200 {array} ComplaintMessageResponse
// @Router /complaints/{id}/messages [get]
func (h *ComplaintHandler) GetComplaintMessages(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid complaint ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Check ownership (non-staff can only view their own complaints)
	if userRole != "property_admin" && userRole != "property_staff" {
		var complaint property.Complaint
		if err := propertyDB.Select("user_id").First(&complaint, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Complaint not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch complaint"})
			return
		}
		if complaint.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	var messages []property.ComplaintMessage
	if err := propertyDB.Where("complaint_id = ?", id).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	// Collect user IDs
	userIDs := make([]uint, 0, len(messages))
	for _, msg := range messages {
		userIDs = append(userIDs, msg.UserID)
	}
	
	// Load users from master DB
	usersMap := make(map[uint]*master.User)
	if len(userIDs) > 0 && h.masterDB != nil {
		var users []master.User
		if err := h.masterDB.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for i := range users {
				usersMap[users[i].ID] = &users[i]
			}
		}
	}

	var responses []ComplaintMessageResponse
	for _, msg := range messages {
		resp := ComplaintMessageResponse{
			ID:          msg.ID,
			UserID:      msg.UserID,
			Message:     msg.Message,
			IsFromStaff: msg.IsFromStaff,
			CreatedAt:   msg.CreatedAt,
		}
		if user, ok := usersMap[msg.UserID]; ok {
			resp.UserName = user.GetDisplayName()
		}
		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// SendComplaintMessage sends a message to a complaint
// @Summary Send message to complaint
// @Tags Complaints
// @Accept json
// @Produce json
// @Param id path int true "Complaint ID"
// @Success 201 {object} ComplaintMessageResponse
// @Router /complaints/{id}/messages [post]
func (h *ComplaintHandler) SendComplaintMessage(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid complaint ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Check complaint exists and status
	var complaint property.Complaint
	if err := propertyDB.Select("user_id", "status").First(&complaint, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Complaint not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch complaint"})
		return
	}

	// Check ownership (non-staff can only send to their own complaints)
	if userRole != "property_admin" && userRole != "property_staff" {
		if complaint.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	// No one can send messages to closed complaints
	if complaint.Status == property.ComplaintStatusClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send messages to closed complaints"})
		return
	}

	var body struct {
		Message string `json:"message" binding:"required,min=1,max=1000"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Determine if message is from staff
	isFromStaff := userRole == "property_staff" || userRole == "property_admin"

	message := property.ComplaintMessage{
		ComplaintID: uint(id),
		UserID:      userID,
		Message:     body.Message,
		IsFromStaff: isFromStaff,
	}

	if err := propertyDB.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	// Reload message
	propertyDB.First(&message, message.ID)

	response := ComplaintMessageResponse{
		ID:          message.ID,
		UserID:      message.UserID,
		Message:     message.Message,
		IsFromStaff: message.IsFromStaff,
		CreatedAt:   message.CreatedAt,
	}
	
	// Load user from master DB
	if h.masterDB != nil {
		var user master.User
		if err := h.masterDB.First(&user, message.UserID).Error; err == nil {
			response.UserName = user.GetDisplayName()
		}
	}

	// Update complaint's updated_at timestamp
	propertyDB.Model(&property.Complaint{}).Where("id = ?", id).Update("updated_at", time.Now())

	c.JSON(http.StatusCreated, gin.H{"data": response, "message": "Message sent successfully"})
}

// CloseComplaint closes a complaint (only by original complainant)
// @Summary Close complaint
// @Tags Complaints
// @Produce json
// @Param id path int true "Complaint ID"
// @Success 200 {object} ComplaintResponse
// @Router /complaints/{id}/close [post]
func (h *ComplaintHandler) CloseComplaint(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid complaint ID"})
		return
	}

	userID := middleware.GetUserID(c)

	var complaint property.Complaint
	if err := propertyDB.First(&complaint, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Complaint not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch complaint"})
		return
	}

	// Only the original complainant can close their own complaint
	if complaint.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the original complainant can close their complaint"})
		return
	}

	if err := propertyDB.Model(&complaint).Update("status", property.ComplaintStatusClosed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close complaint"})
		return
	}

	propertyDB.Preload("Unit").Preload("Parking").First(&complaint, id)
	c.JSON(http.StatusOK, gin.H{"data": h.buildComplaintResponse(complaint, propertyDB), "message": "Complaint closed successfully"})
}

// saveComplaintAttachments saves uploaded files and creates document records
func (h *ComplaintHandler) saveComplaintAttachments(c *gin.Context, db *gorm.DB, complaintID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	files := form.File["attachments"]
	if len(files) == 0 {
		return nil
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/complaints/%d", complaintID)
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
		filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitizeComplaintFilename(file.Filename), ext)
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
			EntityType:   property.DocEntityComplaint,
			EntityID:     complaintID,
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

// sanitizeComplaintFilename removes problematic characters from filename
func sanitizeComplaintFilename(name string) string {
	// Remove extension first
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)

	// Replace problematic characters
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)

	return replacer.Replace(base)
}
