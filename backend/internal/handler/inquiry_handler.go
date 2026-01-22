package handler

import (
	"net/http"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InquiryHandler struct{}

func NewInquiryHandler() *InquiryHandler {
	return &InquiryHandler{}
}

// InquiryResponse represents the API response for an inquiry
type InquiryResponse struct {
	ID            uint       `json:"id"`
	UserID        uint       `json:"user_id"`
	UnitID        *uint      `json:"unit_id"`
	ParkingID     *uint      `json:"parking_id"`
	Title         string     `json:"title"`
	Description   *string    `json:"description"`
	Status        string     `json:"status"`
	Response      *string    `json:"response"`
	RespondedBy   *uint      `json:"responded_by"`
	RespondedAt   *time.Time `json:"responded_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	UnitNumber    string     `json:"unit_number,omitempty"`
	ParkingNumber string     `json:"parking_number,omitempty"`
}

func (h *InquiryHandler) buildInquiryResponse(inquiry property.Inquiry) InquiryResponse {
	resp := InquiryResponse{
		ID:          inquiry.ID,
		UserID:      inquiry.UserID,
		UnitID:      inquiry.UnitID,
		ParkingID:   inquiry.ParkingID,
		Title:       inquiry.Title,
		Description: inquiry.Description,
		Status:      inquiry.Status,
		Response:    inquiry.Response,
		RespondedBy: inquiry.RespondedBy,
		RespondedAt: inquiry.RespondedAt,
		CreatedAt:   inquiry.CreatedAt,
		UpdatedAt:   inquiry.UpdatedAt,
	}

	if inquiry.Unit != nil {
		resp.UnitNumber = inquiry.Unit.UnitNumber
	}
	if inquiry.Parking != nil {
		resp.ParkingNumber = inquiry.Parking.SlotNumber
	}

	return resp
}

// ListMyInquiries returns inquiries for the current user
// @Summary List my inquiries
// @Tags Inquiries
// @Produce json
// @Success 200 {array} InquiryResponse
// @Router /inquiries/my [get]
func (h *InquiryHandler) ListMyInquiries(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)

	var inquiries []property.Inquiry
	if err := propertyDB.Where("user_id = ?", userID).
		Preload("Unit").Preload("Parking").
		Order("created_at DESC").
		Find(&inquiries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inquiries"})
		return
	}

	var responses []InquiryResponse
	for _, inquiry := range inquiries {
		responses = append(responses, h.buildInquiryResponse(inquiry))
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// CreateInquiry creates a new inquiry
// @Summary Create inquiry
// @Tags Inquiries
// @Accept json
// @Produce json
// @Success 201 {object} InquiryResponse
// @Router /inquiries [post]
func (h *InquiryHandler) CreateInquiry(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)

	var body struct {
		Title       string  `json:"title" binding:"required,max=255"`
		Description string  `json:"description"`
		UnitID      *uint   `json:"unit_id"`
		ParkingID   *uint   `json:"parking_id"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	desc := body.Description
	inquiry := property.Inquiry{
		UserID:      userID,
		UnitID:      body.UnitID,
		ParkingID:   body.ParkingID,
		Title:       body.Title,
		Description: &desc,
		Status:      property.InquiryStatusPending,
	}

	if err := propertyDB.Create(&inquiry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create inquiry"})
		return
	}

	propertyDB.Preload("Unit").Preload("Parking").First(&inquiry, inquiry.ID)
	c.JSON(http.StatusCreated, gin.H{"data": h.buildInquiryResponse(inquiry), "message": "Inquiry created successfully"})
}

// GetInquiry returns a single inquiry
// @Summary Get inquiry
// @Tags Inquiries
// @Produce json
// @Param id path int true "Inquiry ID"
// @Success 200 {object} InquiryResponse
// @Router /inquiries/{id} [get]
func (h *InquiryHandler) GetInquiry(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid inquiry ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var inquiry property.Inquiry
	if err := propertyDB.Preload("Unit").Preload("Parking").First(&inquiry, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Inquiry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inquiry"})
		return
	}

	// Check ownership (non-staff can only view their own)
	if userRole != "property_admin" && userRole != "property_staff" && inquiry.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": h.buildInquiryResponse(inquiry)})
}

// ListAllInquiries returns all inquiries (for staff)
// @Summary List all inquiries
// @Tags Inquiries
// @Produce json
// @Success 200 {array} InquiryResponse
// @Router /inquiries [get]
func (h *InquiryHandler) ListAllInquiries(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	status := c.Query("status")

	query := propertyDB.Preload("Unit").Preload("Parking").Order("created_at DESC")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var inquiries []property.Inquiry
	if err := query.Find(&inquiries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inquiries"})
		return
	}

	var responses []InquiryResponse
	for _, inquiry := range inquiries {
		responses = append(responses, h.buildInquiryResponse(inquiry))
	}

	c.JSON(http.StatusOK, gin.H{"data": responses})
}

// RespondToInquiry responds to an inquiry (staff only)
// @Summary Respond to inquiry
// @Tags Inquiries
// @Accept json
// @Produce json
// @Param id path int true "Inquiry ID"
// @Success 200 {object} InquiryResponse
// @Router /inquiries/{id}/respond [post]
func (h *InquiryHandler) RespondToInquiry(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid inquiry ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only staff can respond
	if userRole != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property staff can respond to inquiries"})
		return
	}

	var body struct {
		Response string `json:"response" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var inquiry property.Inquiry
	if err := propertyDB.First(&inquiry, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Inquiry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inquiry"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       property.InquiryStatusAnswered,
		"response":     body.Response,
		"responded_by": userID,
		"responded_at": &now,
	}

	if err := propertyDB.Model(&inquiry).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to respond to inquiry"})
		return
	}

	propertyDB.Preload("Unit").Preload("Parking").First(&inquiry, id)
	c.JSON(http.StatusOK, gin.H{"data": h.buildInquiryResponse(inquiry), "message": "Response submitted successfully"})
}

// CloseInquiry closes an inquiry
// @Summary Close inquiry
// @Tags Inquiries
// @Produce json
// @Param id path int true "Inquiry ID"
// @Success 200 {object} InquiryResponse
// @Router /inquiries/{id}/close [post]
func (h *InquiryHandler) CloseInquiry(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid inquiry ID"})
		return
	}

	userRole := middleware.GetUserRole(c)

	// Only staff can close inquiries
	if userRole != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property staff can close inquiries"})
		return
	}

	var inquiry property.Inquiry
	if err := propertyDB.First(&inquiry, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Inquiry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inquiry"})
		return
	}

	if err := propertyDB.Model(&inquiry).Update("status", property.InquiryStatusClosed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close inquiry"})
		return
	}

	propertyDB.Preload("Unit").Preload("Parking").First(&inquiry, id)
	c.JSON(http.StatusOK, gin.H{"data": h.buildInquiryResponse(inquiry), "message": "Inquiry closed successfully"})
}
