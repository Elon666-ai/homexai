package handler

import (
	"net/http"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// VisitorHandler handles visitor-related HTTP requests
type VisitorHandler struct {
	masterDB *gorm.DB
}

// NewVisitorHandler creates a new VisitorHandler
func NewVisitorHandler(masterDB *gorm.DB) *VisitorHandler {
	return &VisitorHandler{
		masterDB: masterDB,
	}
}

// ListVisitorsRequest represents list visitors parameters
type ListVisitorsRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Status    string `form:"status" binding:"omitempty"`
	Purpose   string `form:"purpose" binding:"omitempty"`
	UnitID    uint   `form:"unit_id" binding:"omitempty"`
	DateFrom  string `form:"date_from" binding:"omitempty"`
	DateTo    string `form:"date_to" binding:"omitempty"`
	Search    string `form:"search" binding:"omitempty"`
	SortBy    string `form:"sort_by" binding:"omitempty"`
	SortOrder string `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

// CreateVisitorRequest represents create visitor payload
type CreateVisitorRequest struct {
	UnitID        uint   `json:"unit_id" binding:"required"`
	VisitorName   string `json:"visitor_name" binding:"required,min=2,max=200"`
	VisitorPhone  string `json:"visitor_phone" binding:"omitempty,max=20"`
	VisitorEmail  string `json:"visitor_email" binding:"omitempty,email,max=255"`
	VisitorIDType string `json:"visitor_id_type" binding:"omitempty,max=50"`
	VisitorIDNo   string `json:"visitor_id_no" binding:"omitempty,max=100"`
	Purpose       string `json:"purpose" binding:"required,oneof=visit delivery service other"`
	VehiclePlate  string `json:"vehicle_plate" binding:"omitempty,max=20"`
	ExpectedAt    string `json:"expected_at" binding:"required"`
	Notes         string `json:"notes" binding:"omitempty,max=1000"`
}

// UpdateVisitorRequest represents update visitor payload
type UpdateVisitorRequest struct {
	VisitorName   string `json:"visitor_name" binding:"omitempty,min=2,max=200"`
	VisitorPhone  string `json:"visitor_phone" binding:"omitempty,max=20"`
	VisitorEmail  string `json:"visitor_email" binding:"omitempty,email,max=255"`
	Purpose       string `json:"purpose" binding:"omitempty,oneof=visit delivery service other"`
	VehiclePlate  string `json:"vehicle_plate" binding:"omitempty,max=20"`
	ExpectedAt    string `json:"expected_at" binding:"omitempty"`
	Notes         string `json:"notes" binding:"omitempty,max=1000"`
}

// VisitorResponse represents a visitor in API response
type VisitorResponse struct {
	ID            uint       `json:"id"`
	UnitID        uint       `json:"unit_id"`
	UnitNumber    string     `json:"unit_number"`
	HostUserID    uint       `json:"host_user_id"`
	HostName      string     `json:"host_name"`
	VisitorName   string     `json:"visitor_name"`
	VisitorPhone  *string    `json:"visitor_phone"`
	VisitorEmail  *string    `json:"visitor_email"`
	VisitorIDType *string    `json:"visitor_id_type"`
	VisitorIDNo   *string    `json:"visitor_id_no"`
	Purpose       string     `json:"purpose"`
	VehiclePlate  *string    `json:"vehicle_plate"`
	ExpectedAt    time.Time  `json:"expected_at"`
	CheckedInAt   *time.Time `json:"checked_in_at"`
	CheckedOutAt  *time.Time `json:"checked_out_at"`
	CheckedInBy   *uint      `json:"checked_in_by"`
	CheckedOutBy  *uint      `json:"checked_out_by"`
	Status        string     `json:"status"`
	Notes         *string    `json:"notes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ListVisitors lists visitors with pagination and filters
// @Summary List visitors
// @Tags Visitors
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /visitors [get]
func (h *VisitorHandler) ListVisitors(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var req ListVisitorsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default pagination
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	db := propertyDB
	query := db.Model(&property.Visitor{})

	// Role-based filtering
	// property_admin and property_staff can see all visitors
	// landlord, spa, tenant can only see their own visitors (as host)
	if userRole == "landlord" || userRole == "spa" || userRole == "tenant" {
		query = query.Where("host_user_id = ?", userID)
	}

	// Apply filters
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.Purpose != "" {
		query = query.Where("purpose = ?", req.Purpose)
	}
	if req.UnitID > 0 {
		query = query.Where("unit_id = ?", req.UnitID)
	}
	if req.DateFrom != "" {
		query = query.Where("expected_at >= ?", req.DateFrom)
	}
	if req.DateTo != "" {
		query = query.Where("expected_at <= ?", req.DateTo+" 23:59:59")
	}
	if req.Search != "" {
		search := "%" + req.Search + "%"
		query = query.Where("visitor_name LIKE ? OR visitor_phone LIKE ? OR vehicle_plate LIKE ?", search, search, search)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count visitors"})
		return
	}

	// Sorting
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "expected_at"
	}
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	query = query.Order(sortBy + " " + sortOrder)

	// Pagination
	offset := (req.Page - 1) * req.PageSize
	var visitors []property.Visitor
	if err := query.Preload("Unit").Offset(offset).Limit(req.PageSize).Find(&visitors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch visitors"})
		return
	}

	// Build response with user info
	responses := make([]VisitorResponse, len(visitors))
	for i, v := range visitors {
		resp := h.buildVisitorResponse(v)
		responses[i] = resp
	}

	c.JSON(http.StatusOK, gin.H{
		"data": responses,
		"meta": gin.H{
			"page":        req.Page,
			"page_size":   req.PageSize,
			"total":       total,
			"total_pages": (total + int64(req.PageSize) - 1) / int64(req.PageSize),
		},
	})
}

// GetVisitor gets a single visitor by ID
// @Summary Get visitor details
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Success 200 {object} VisitorResponse
// @Router /visitors/{id} [get]
func (h *VisitorHandler) GetVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB
	var visitor property.Visitor
	if err := db.Preload("Unit").First(&visitor, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch visitor"})
		return
	}

	// Check permission
	if userRole == "tenant" || userRole == "landlord" || userRole == "spa" {
		if visitor.HostUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	resp := h.buildVisitorResponse(visitor)
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// CreateVisitor creates a new visitor registration
// @Summary Create visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param body body CreateVisitorRequest true "Visitor data"
// @Success 201 {object} VisitorResponse
// @Router /visitors [post]
func (h *VisitorHandler) CreateVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	var req CreateVisitorRequest
	var unitFound bool

	// Try to parse as multipart form first
	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Parse form data
		visitorName := c.PostForm("visitor_name")
		contactNumber := c.PostForm("contact_number")
		unitNumber := c.PostForm("unit_number")
		tower := c.PostForm("tower")
		purposeOfVisit := c.PostForm("purpose_of_visit")
		timeIn := c.PostForm("time_in")
		timeOut := c.PostForm("time_out")
		visitorPassNo := c.PostForm("visitor_pass_no")

		// Map purpose_of_visit to purpose
		purpose := "visit"
		if purposeOfVisit == "delivery" {
			purpose = "delivery"
		} else if purposeOfVisit == "repair_maintenance" {
			purpose = "service"
		} else if purposeOfVisit == "other" {
			purpose = "other"
		}

		// Build request
		req.VisitorName = visitorName
		req.VisitorPhone = contactNumber
		req.Purpose = purpose
		req.ExpectedAt = timeIn
		if timeOut != "" {
			req.Notes = "Time out: " + timeOut
		}
		if visitorPassNo != "" {
			if req.Notes != "" {
				req.Notes += "; Visitor Pass No: " + visitorPassNo
			} else {
				req.Notes = "Visitor Pass No: " + visitorPassNo
			}
		}

		// Find unit by unit_number
		if unitNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unit number is required"})
			return
		}

		var unit property.Unit
		query := db.Where("unit_number = ?", unitNumber)
		if tower != "" {
			query = query.Where("building = ?", tower)
		}
		if err := query.First(&unit).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unit not found"})
			return
		}
		req.UnitID = unit.ID
		unitFound = true
	} else {
		// Parse as JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Parse expected time
	expectedAt, err := time.Parse("2006-01-02 15:04:05", req.ExpectedAt)
	if err != nil {
		expectedAt, err = time.Parse("2006-01-02T15:04:05", req.ExpectedAt)
		if err != nil {
			expectedAt, err = time.Parse("2006-01-02T15:04", req.ExpectedAt)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expected_at format. Use YYYY-MM-DD HH:MM:SS or ISO format"})
				return
			}
		}
	}

	// Verify unit exists (if not already found)
	var unit property.Unit
	if !unitFound {
		if err := db.First(&unit, req.UnitID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unit not found"})
			return
		}
	}

	// Verify user has access to this unit (for non-admin roles)
	if userRole == "tenant" || userRole == "landlord" || userRole == "spa" {
		hasAccess := false

		if userRole == "tenant" {
			// Check if user is tenant of this unit
			var tenant property.Tenant
			if err := db.Where("unit_id = ? AND user_id = ? AND is_active = ?", req.UnitID, userID, true).First(&tenant).Error; err == nil {
				hasAccess = true
			}
		} else if userRole == "landlord" {
			// Check if user is landlord of this unit
			var landlord property.Landlord
			if err := db.Where("unit_id = ? AND user_id = ? AND is_active = ?", req.UnitID, userID, true).First(&landlord).Error; err == nil {
				hasAccess = true
			}
		} else if userRole == "spa" {
			// Check if user has SPA access to this unit
			var spaUnit property.SPAUnit
			if err := db.Where("unit_id = ? AND spa_user_id = ? AND is_active = ?", req.UnitID, userID, true).First(&spaUnit).Error; err == nil {
				hasAccess = true
			}
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to register visitors for this unit"})
			return
		}
	}

	visitor := property.Visitor{
		UnitID:       req.UnitID,
		HostUserID:   userID,
		VisitorName:  req.VisitorName,
		Purpose:      req.Purpose,
		ExpectedAt:   expectedAt,
		Status:       property.VisitorStatusPending,
	}

	if req.VisitorPhone != "" {
		visitor.VisitorPhone = &req.VisitorPhone
	}
	if req.VisitorEmail != "" {
		visitor.VisitorEmail = &req.VisitorEmail
	}
	if req.VisitorIDType != "" {
		visitor.VisitorIDType = &req.VisitorIDType
	}
	if req.VisitorIDNo != "" {
		visitor.VisitorIDNo = &req.VisitorIDNo
	}
	if req.VehiclePlate != "" {
		visitor.VehiclePlate = &req.VehiclePlate
	}
	if req.Notes != "" {
		visitor.Notes = &req.Notes
	}

	if err := db.Create(&visitor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create visitor"})
		return
	}

	// Reload with unit
	db.Preload("Unit").First(&visitor, visitor.ID)

	resp := h.buildVisitorResponse(visitor)
	c.JSON(http.StatusCreated, gin.H{"data": resp, "message": "Visitor registered successfully"})
}

// UpdateVisitor updates a visitor
// @Summary Update visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Param body body UpdateVisitorRequest true "Visitor data"
// @Success 200 {object} VisitorResponse
// @Router /visitors/{id} [put]
func (h *VisitorHandler) UpdateVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var req UpdateVisitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := propertyDB

	var visitor property.Visitor
	if err := db.First(&visitor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
		return
	}

	// Only host or admin can update
	if userRole != "property_admin" && userRole != "property_staff" && userRole != "super_admin" {
		if visitor.HostUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only update your own visitor registrations"})
			return
		}
	}

	// Can only update pending visitors
	if visitor.Status != property.VisitorStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only update pending visitor registrations"})
		return
	}

	updates := make(map[string]interface{})

	if req.VisitorName != "" {
		updates["visitor_name"] = req.VisitorName
	}
	if req.VisitorPhone != "" {
		updates["visitor_phone"] = req.VisitorPhone
	}
	if req.VisitorEmail != "" {
		updates["visitor_email"] = req.VisitorEmail
	}
	if req.Purpose != "" {
		updates["purpose"] = req.Purpose
	}
	if req.VehiclePlate != "" {
		updates["vehicle_plate"] = req.VehiclePlate
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.ExpectedAt != "" {
		expectedAt, err := time.Parse("2006-01-02 15:04:05", req.ExpectedAt)
		if err != nil {
			expectedAt, err = time.Parse("2006-01-02T15:04:05", req.ExpectedAt)
			if err != nil {
				expectedAt, err = time.Parse("2006-01-02T15:04", req.ExpectedAt)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expected_at format"})
					return
				}
			}
		}
		updates["expected_at"] = expectedAt
	}

	if len(updates) > 0 {
		if err := db.Model(&visitor).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update visitor"})
			return
		}
	}

	// Reload
	db.Preload("Unit").First(&visitor, id)

	resp := h.buildVisitorResponse(visitor)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Visitor updated successfully"})
}

// ApproveVisitor approves a visitor registration
// @Summary Approve visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Success 200 {object} map[string]interface{}
// @Router /visitors/{id}/approve [post]
func (h *VisitorHandler) ApproveVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	var visitor property.Visitor
	if err := db.First(&visitor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
		return
	}

	// Host, property_admin, or property_staff can approve
	if userRole != "property_admin" && userRole != "property_staff" && userRole != "super_admin" {
		if visitor.HostUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only approve your own visitor registrations"})
			return
		}
	}

	if visitor.Status != property.VisitorStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only approve pending visitors"})
		return
	}

	if err := db.Model(&visitor).Update("status", property.VisitorStatusApproved).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve visitor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Visitor approved successfully"})
}

// RejectVisitor rejects a visitor registration
// @Summary Reject visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Success 200 {object} map[string]interface{}
// @Router /visitors/{id}/reject [post]
func (h *VisitorHandler) RejectVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	var visitor property.Visitor
	if err := db.First(&visitor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
		return
	}

	// Host, property_admin, or property_staff can reject
	if userRole != "property_admin" && userRole != "property_staff" && userRole != "super_admin" {
		if visitor.HostUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only reject your own visitor registrations"})
			return
		}
	}

	if visitor.Status != property.VisitorStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only reject pending visitors"})
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)

	updates := map[string]interface{}{
		"status": property.VisitorStatusRejected,
	}
	if body.Reason != "" {
		updates["notes"] = body.Reason
	}

	if err := db.Model(&visitor).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject visitor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Visitor rejected"})
}

// CheckInVisitor checks in a visitor (staff only)
// @Summary Check in visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Success 200 {object} map[string]interface{}
// @Router /visitors/{id}/checkin [post]
func (h *VisitorHandler) CheckInVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only staff and guards can check in visitors
	if userRole != "property_admin" && userRole != "property_staff" && userRole != "property_guard" && userRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can check in visitors"})
		return
	}

	db := propertyDB

	var visitor property.Visitor
	if err := db.First(&visitor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
		return
	}

	if visitor.Status != property.VisitorStatusApproved {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only check in approved visitors"})
		return
	}

	now := time.Now()
	if err := db.Model(&visitor).Updates(map[string]interface{}{
		"status":        property.VisitorStatusCheckedIn,
		"checked_in_at": now,
		"checked_in_by": userID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check in visitor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Visitor checked in successfully", "checked_in_at": now})
}

// CheckOutVisitor checks out a visitor (staff only)
// @Summary Check out visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Success 200 {object} map[string]interface{}
// @Router /visitors/{id}/checkout [post]
func (h *VisitorHandler) CheckOutVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only staff can check out visitors
	if userRole != "property_admin" && userRole != "property_staff" && userRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only staff can check out visitors"})
		return
	}

	db := propertyDB

	var visitor property.Visitor
	if err := db.First(&visitor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
		return
	}

	if visitor.Status != property.VisitorStatusCheckedIn {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only check out checked-in visitors"})
		return
	}

	now := time.Now()
	if err := db.Model(&visitor).Updates(map[string]interface{}{
		"status":         property.VisitorStatusCheckedOut,
		"checked_out_at": now,
		"checked_out_by": userID,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check out visitor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Visitor checked out successfully", "checked_out_at": now})
}

// CancelVisitor cancels a visitor registration
// @Summary Cancel visitor
// @Tags Visitors
// @Accept json
// @Produce json
// @Param id path int true "Visitor ID"
// @Success 200 {object} map[string]interface{}
// @Router /visitors/{id}/cancel [post]
func (h *VisitorHandler) CancelVisitor(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid visitor ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	var visitor property.Visitor
	if err := db.First(&visitor, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Visitor not found"})
		return
	}

	// Host or admin can cancel
	if userRole != "property_admin" && userRole != "property_staff" && userRole != "super_admin" {
		if visitor.HostUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "You can only cancel your own visitor registrations"})
			return
		}
	}

	// Can only cancel pending or approved visitors
	if visitor.Status != property.VisitorStatusPending && visitor.Status != property.VisitorStatusApproved {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only cancel pending or approved visitors"})
		return
	}

	if err := db.Model(&visitor).Update("status", property.VisitorStatusCancelled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel visitor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Visitor registration cancelled"})
}

// GetVisitorStats gets visitor statistics
// @Summary Get visitor stats
// @Tags Visitors
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /visitors/stats [get]
func (h *VisitorHandler) GetVisitorStats(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	// Base query
	baseQuery := db.Model(&property.Visitor{})

	// Role-based filtering
	if userRole == "landlord" || userRole == "spa" || userRole == "tenant" {
		baseQuery = baseQuery.Where("host_user_id = ?", userID)
	}

	// Count by status
	var stats []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}

	if err := baseQuery.Select("status, count(*) as count").Group("status").Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get visitor stats"})
		return
	}

	// Build stats map
	result := map[string]int64{
		"pending":     0,
		"approved":    0,
		"checked_in":  0,
		"checked_out": 0,
		"cancelled":   0,
		"rejected":    0,
		"total":       0,
	}

	for _, s := range stats {
		result[s.Status] = s.Count
		result["total"] += s.Count
	}

	// Today's visitors
	today := time.Now().Format("2006-01-02")
	var todayCount int64
	todayQuery := db.Model(&property.Visitor{}).Where("DATE(expected_at) = ?", today)
	if userRole == "landlord" || userRole == "spa" || userRole == "tenant" {
		todayQuery = todayQuery.Where("host_user_id = ?", userID)
	}
	todayQuery.Count(&todayCount)
	result["today"] = todayCount

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// buildVisitorResponse builds a VisitorResponse from a Visitor model
func (h *VisitorHandler) buildVisitorResponse(v property.Visitor) VisitorResponse {
	resp := VisitorResponse{
		ID:            v.ID,
		UnitID:        v.UnitID,
		HostUserID:    v.HostUserID,
		VisitorName:   v.VisitorName,
		VisitorPhone:  v.VisitorPhone,
		VisitorEmail:  v.VisitorEmail,
		VisitorIDType: v.VisitorIDType,
		VisitorIDNo:   v.VisitorIDNo,
		Purpose:       v.Purpose,
		VehiclePlate:  v.VehiclePlate,
		ExpectedAt:    v.ExpectedAt,
		CheckedInAt:   v.CheckedInAt,
		CheckedOutAt:  v.CheckedOutAt,
		CheckedInBy:   v.CheckedInBy,
		CheckedOutBy:  v.CheckedOutBy,
		Status:        v.Status,
		Notes:         v.Notes,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
	}

	// Get unit number
	if v.Unit.ID > 0 {
		resp.UnitNumber = v.Unit.UnitNumber
	}

	// Get host name from master DB
	if h.masterDB != nil {
		var user master.User
		if err := h.masterDB.Select("id, full_name").First(&user, v.HostUserID).Error; err == nil {
			resp.HostName = user.FullName
		}
	}

	return resp
}
