package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequestHandler handles request-related HTTP requests
type RequestHandler struct {
	masterDB       *gorm.DB
	requestService *service.RequestService
	smtpSvc        *service.SmtpService
}

// NewRequestHandler creates a new RequestHandler
func NewRequestHandler(masterDB *gorm.DB, requestService *service.RequestService, smtpSvc *service.SmtpService) *RequestHandler {
	return &RequestHandler{
		masterDB:       masterDB,
		requestService: requestService,
		smtpSvc:        smtpSvc,
	}
}

// validateStatusTransition validates if a status transition is allowed
func (h *RequestHandler) validateStatusTransition(currentStatus, newStatus string) error {
	validTransitions := map[string][]string{
		property.RequestStatusPending:    {property.RequestStatusInProgress, property.RequestStatusRejected},
		property.RequestStatusInProgress: {property.RequestStatusCompleted, property.RequestStatusRejected},
		property.RequestStatusRejected:   {property.RequestStatusPending}, // Allow resubmission
		property.RequestStatusCompleted:  {},                              // No transitions from completed
		property.RequestStatusCancelled:  {},                              // No transitions from cancelled
	}

	allowedStatuses, exists := validTransitions[currentStatus]
	if !exists {
		return fmt.Errorf("invalid current status: %s", currentStatus)
	}

	for _, allowed := range allowedStatuses {
		if allowed == newStatus {
			return nil
		}
	}

	return fmt.Errorf("invalid status transition from %s to %s", currentStatus, newStatus)
}

// ListRequestsRequest represents list requests parameters
type ListRequestsRequest struct {
	Page        int    `form:"page" binding:"omitempty,min=1"`
	PageSize    int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Category    string `form:"category" binding:"omitempty"`
	Status      string `form:"status" binding:"omitempty"`
	RequestType string `form:"request_type" binding:"omitempty"`
	Priority    string `form:"priority" binding:"omitempty"`
	AssignedTo  uint   `form:"assigned_to" binding:"omitempty"`
	UnitID      uint   `form:"unit_id" binding:"omitempty"`
	ParkingID   uint   `form:"parking_id" binding:"omitempty"`
	Search      string `form:"search" binding:"omitempty"`
	SortBy      string `form:"sort_by" binding:"omitempty"`
	SortOrder   string `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

// CreateRequestRequest represents create request payload
type CreateRequestRequest struct {
	Category    string `json:"category" binding:"required,oneof=house parking"`
	RequestType string `json:"request_type" binding:"required"`
	UnitID      *uint  `json:"unit_id"`
	ParkingID   *uint  `json:"parking_id"`
	Title       string `json:"title" binding:"required,min=2,max=200"`
	Description string `json:"description" binding:"omitempty,max=3000"`
	Priority    string `json:"priority" binding:"omitempty,oneof=low normal high urgent"`
}

// UpdateRequestRequest represents update request payload
type UpdateRequestRequest struct {
	Title       string `json:"title" binding:"omitempty,min=3,max=200"`
	Description string `json:"description"`
	Priority    string `json:"priority" binding:"omitempty,oneof=low normal high urgent"`
	Status      string `json:"status" binding:"omitempty,oneof=pending in_progress completed cancelled rejected"`
	AssignedTo  *uint  `json:"assigned_to"`
	Resolution  string `json:"resolution"`
}

// AttachmentResponse represents an attachment in API response
type AttachmentResponse struct {
	ID           uint   `json:"id"`
	DocumentName string `json:"document_name"`
	DocumentPath string `json:"document_path"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	UploadedAt   string `json:"uploaded_at"`
}

// RequestResponse represents a request in API response
type RequestResponse struct {
	ID                         uint                            `json:"id"`
	UserID                     uint                            `json:"user_id"`
	UserName                   string                          `json:"user_name"`
	UserEmail                  string                          `json:"user_email"`
	Category                   string                          `json:"category"`
	UnitID                     *uint                           `json:"unit_id"`
	UnitNumber                 string                          `json:"unit_number,omitempty"`
	ParkingID                  *uint                           `json:"parking_id"`
	ParkingNumber              string                          `json:"parking_number,omitempty"`
	RequestType                string                          `json:"request_type"`
	Title                      string                          `json:"title"`
	Description                *string                         `json:"description"`
	Priority                   string                          `json:"priority"`
	Status                     string                          `json:"status"`
	AssignedTo                 *uint                           `json:"assigned_to"`
	AssignedName               string                          `json:"assigned_name,omitempty"`
	ResolvedAt                 *time.Time                      `json:"resolved_at"`
	ResolvedBy                 *uint                           `json:"resolved_by"`
	ResolverName               string                          `json:"resolver_name,omitempty"`
	Resolution                 *string                         `json:"resolution"`
	Attachments                []AttachmentResponse            `json:"attachments"`
	VehicleSticker             *VehicleStickerInfo             `json:"vehicle_sticker,omitempty"`
	PetRegistration            *PetRegistrationInfo            `json:"pet_registration,omitempty"`
	MoveIn                     *MoveInInfo                     `json:"move_in,omitempty"`
	MoveOut                    *MoveOutInfo                    `json:"move_out,omitempty"`
	HouseholdStaffRegistration *HouseholdStaffRegistrationInfo `json:"household_staff_registration,omitempty"`
	CreatedAt                  time.Time                       `json:"created_at"`
	UpdatedAt                  time.Time                       `json:"updated_at"`
}

// VehicleStickerInfo represents vehicle sticker info in request response
type VehicleStickerInfo struct {
	Status  string `json:"status"`
	IsDraft bool   `json:"is_draft"`
}

// PetRegistrationInfo represents pet registration info in request response
type PetRegistrationInfo struct {
	Status  string `json:"status"`
	IsDraft bool   `json:"is_draft"`
}

// MoveInInfo represents move-in info in request response
type MoveInInfo struct {
	Status  string `json:"status"`
	IsDraft bool   `json:"is_draft"`
}

// MoveOutInfo represents move-out info in request response
type MoveOutInfo struct {
	Status  string `json:"status"`
	IsDraft bool   `json:"is_draft"`
}

// HouseholdStaffRegistrationInfo represents household staff registration info in request response
type HouseholdStaffRegistrationInfo struct {
	Status  string `json:"status"`
	IsDraft bool   `json:"is_draft"`
}

// ListRequests lists requests with pagination and filters
// @Summary List requests
// @Tags Requests
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /requests [get]
func (h *RequestHandler) ListRequests(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var req ListRequestsRequest
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
	query := db.Model(&property.Request{})

	// Role-based filtering
	// property_admin and property_staff can see all requests
	// landlord, spa, tenant can only see their own requests
	role := userRole
	if role == "landlord" || role == "spa" || role == "tenant" {
		query = query.Where("user_id = ?", userID)
	}

	// Apply filters
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.RequestType != "" {
		query = query.Where("request_type = ?", req.RequestType)
	}
	if req.Priority != "" {
		query = query.Where("priority = ?", req.Priority)
	}
	if req.AssignedTo > 0 {
		query = query.Where("assigned_to = ?", req.AssignedTo)
	}
	if req.UnitID > 0 {
		query = query.Where("unit_id = ?", req.UnitID)
	}
	if req.ParkingID > 0 {
		query = query.Where("parking_id = ?", req.ParkingID)
	}
	if req.Search != "" {
		search := "%" + req.Search + "%"
		query = query.Where("title LIKE ? OR description LIKE ?", search, search)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count requests"})
		return
	}

	// Sorting
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	query = query.Order(sortBy + " " + sortOrder)

	// Pagination
	offset := (req.Page - 1) * req.PageSize
	var requests []property.Request
	if err := query.Preload("Unit").Preload("Parking").Offset(offset).Limit(req.PageSize).Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch requests"})
		return
	}

	// Build response with user info
	responses := make([]RequestResponse, len(requests))
	for i, r := range requests {
		resp := h.buildRequestResponse(r, db)
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

// GetRequest gets a single request by ID
// @Summary Get request details
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} RequestResponse
// @Router /requests/{id} [get]
func (h *RequestHandler) GetRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB
	var request property.Request
	if err := db.Preload("Unit").Preload("Parking").First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Check permission
	role := userRole
	if role == "tenant" {
		// Tenant can only view their own requests
		if request.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	} else if role == "landlord" || role == "spa" {
		// Landlord/SPA can view their own requests OR requests from their units
		hasAccess := request.UserID == userID

		if !hasAccess && request.UnitID != nil && *request.UnitID > 0 {
			// Check if the unit belongs to this landlord/spa
			if role == "landlord" {
				var count int64
				db.Model(&property.Landlord{}).Where("user_id = ? AND unit_id = ?", userID, *request.UnitID).Count(&count)
				hasAccess = count > 0
			} else if role == "spa" {
				var count int64
				db.Model(&property.SPAUnit{}).Where("spa_user_id = ? AND unit_id = ? AND is_active = ?", userID, *request.UnitID, true).Count(&count)
				hasAccess = count > 0
			}
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// CreateRequest creates a new request
// @Summary Create request
// @Tags Requests
// @Accept json
// @Produce json
// @Param request body CreateRequestRequest true "Request data"
// @Success 201 {object} RequestResponse
// @Router /requests [post]
func (h *RequestHandler) CreateRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found", "message": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var req CreateRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "message": err.Error()})
		return
	}

	// Only landlord, spa, tenant can create requests
	role := userRole
	if role != "landlord" && role != "spa" && role != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only landlord, spa, or tenant can create requests", "message": "Only landlord, spa, or tenant can create requests"})
		return
	}

	// Default priority
	if req.Priority == "" {
		req.Priority = "normal"
	}

	db := propertyDB

	// For tenant: auto-link to their unit if not provided
	if role == "tenant" && req.Category == "house" {
		if req.UnitID == nil || *req.UnitID == 0 {
			// Find tenant's active unit from tenants table
			var tenant property.Tenant
			if err := db.Where("user_id = ? AND status = ?", userID, property.TenantStatusActive).First(&tenant).Error; err == nil {
				req.UnitID = &tenant.UnitID
			}
		}
	}

	// Validate unit if provided
	if req.UnitID != nil && *req.UnitID > 0 {
		var unit property.Unit
		if err := db.First(&unit, *req.UnitID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit ID", "message": "Invalid unit ID"})
			return
		}
	}

	// Validate parking if provided
	if req.ParkingID != nil && *req.ParkingID > 0 {
		var parking property.ParkingSlot
		if err := db.First(&parking, *req.ParkingID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parking ID", "message": "Invalid parking ID"})
			return
		}
	}

	// All requests start with pending status
	initialStatus := property.RequestStatusPending

	request := property.Request{
		UserID:      userID,
		Category:    req.Category,
		RequestType: req.RequestType,
		UnitID:      req.UnitID,
		ParkingID:   req.ParkingID,
		Title:       req.Title,
		Description: strPtr(req.Description),
		Priority:    req.Priority,
		Status:      initialStatus,
	}

	if err := db.Create(&request).Error; err != nil {
		errMsg := "Failed to create request: " + err.Error()
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg, "message": errMsg})
		return
	}

	// Create initial trace for request creation
	operatorName, operatorRole := h.getOperatorInfo(userID, role)
	h.createRequestTrace(
		db,
		request.ID,
		property.TraceActionCreated,
		nil,
		&initialStatus,
		userID,
		operatorName,
		operatorRole,
		nil,
		c,
	)

	// Reload with relationships
	db.Preload("Unit").Preload("Parking").First(&request, request.ID)

	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusCreated, gin.H{"data": resp, "message": "Request created successfully"})
}

// UpdateRequest updates a request
// @Summary Update request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param request body UpdateRequestRequest true "Request data"
// @Success 200 {object} RequestResponse
// @Router /requests/{id} [put]
func (h *RequestHandler) UpdateRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var req UpdateRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := propertyDB
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Check permission
	role := userRole
	if role == "landlord" || role == "spa" || role == "tenant" {
		// Regular users can only update their own requests and only certain fields
		if request.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		// Allow editing requests in pending, in_progress, or rejected status
		// But restrict status changes: can only cancel pending requests, or resubmit rejected/in_progress
		allowedStatusesForEdit := []string{property.RequestStatusPending, property.RequestStatusInProgress, property.RequestStatusRejected}
		canEdit := false
		for _, s := range allowedStatusesForEdit {
			if request.Status == s {
				canEdit = true
				break
			}
		}

		if !canEdit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Can only edit requests in pending, in_progress, or rejected status"})
			return
		}

		// Can only cancel pending requests
		if req.Status != "" && req.Status != "cancelled" && req.Status != "pending" {
			// For in_progress or rejected, allow resubmitting by setting status to pending
			if request.Status == property.RequestStatusInProgress || request.Status == property.RequestStatusRejected {
				if req.Status != "pending" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Can only resubmit in_progress or rejected requests by setting status to pending"})
					return
				}
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "You can only cancel or resubmit your requests"})
				return
			}
		}

		if request.Status != "pending" && req.Status == "cancelled" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Can only cancel pending requests"})
			return
		}
	}

	// Store old values for trace
	oldStatus := request.Status
	oldPriority := request.Priority
	oldAssignedTo := request.AssignedTo

	// Build updates map
	updates := make(map[string]interface{})
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Status != "" {
		updates["status"] = req.Status
		// Set resolved info if completing
		if req.Status == "completed" || req.Status == "rejected" {
			now := time.Now()
			uid := userID
			updates["resolved_at"] = &now
			updates["resolved_by"] = &uid
		}
	}
	if req.AssignedTo != nil {
		updates["assigned_to"] = req.AssignedTo
	}
	if req.Resolution != "" {
		updates["resolution"] = req.Resolution
	}

	if len(updates) > 0 {
		if err := db.Model(&request).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
			return
		}

		// Create traces for changes
		operatorName, operatorRole := h.getOperatorInfo(userID, role)

		// Status change trace
		if req.Status != "" && req.Status != oldStatus {
			action := property.TraceActionStatusChanged
			if req.Status == "completed" {
				action = property.TraceActionResolved
			} else if req.Status == "cancelled" {
				action = property.TraceActionCancelled
			} else if req.Status == "rejected" {
				action = property.TraceActionRejected
			}
			var remark *string
			if req.Resolution != "" {
				remark = &req.Resolution
			}
			h.createRequestTrace(db, uint(id), action, &oldStatus, &req.Status, userID, operatorName, operatorRole, remark, c)

			// Create notification for request owner (if status changed by staff/admin)
			if request.UserID != userID {
				propertyID := middleware.GetPropertyID(c)
				notificationService := service.NewNotificationService(db)
				if err := notificationService.CreateRequestStatusChangeNotification(
					request.UserID,
					propertyID,
					request.ID,
					request.Title,
					oldStatus,
					req.Status,
				); err == nil {
					// Send email notification if user has email notifications enabled
					go h.sendNotificationEmail(request.UserID, propertyID, "Request Status Updated",
						"Your request \""+request.Title+"\" status changed from "+oldStatus+" to "+req.Status+".")
				}
			}
		}

		// Priority change trace
		if req.Priority != "" && req.Priority != oldPriority {
			remark := "Priority changed from " + oldPriority + " to " + req.Priority
			h.createRequestTrace(db, uint(id), property.TraceActionPriorityChanged, nil, nil, userID, operatorName, operatorRole, &remark, c)
		}

		// Assignment change trace
		if req.AssignedTo != nil {
			isNewAssignment := oldAssignedTo == nil || *oldAssignedTo == 0
			isReassignment := oldAssignedTo != nil && *oldAssignedTo > 0 && *req.AssignedTo != *oldAssignedTo
			if isNewAssignment || isReassignment {
				action := property.TraceActionAssigned
				if isReassignment {
					action = property.TraceActionReassigned
				}
				// Get assignee name for remark
				var assignee master.User
				var remark string
				if h.masterDB.First(&assignee, *req.AssignedTo).Error == nil {
					remark = "Assigned to " + assignee.FullName
				}
				h.createRequestTrace(db, uint(id), action, nil, nil, userID, operatorName, operatorRole, &remark, c)
			}
		}
	}

	// Reload
	db.Preload("Unit").Preload("Parking").First(&request, id)

	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Request updated successfully"})
}

// AssignRequest assigns a request to a staff member
// @Summary Assign request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} RequestResponse
// @Router /requests/{id}/assign [post]
func (h *RequestHandler) AssignRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var body struct {
		AssignedTo uint `json:"assigned_to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := propertyDB
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Store old values for trace
	oldStatus := request.Status
	oldAssignedTo := request.AssignedTo

	// Update assignment and status
	newStatus := property.RequestStatusInProgress

	// Validate status transition
	if err := h.validateStatusTransition(request.Status, newStatus); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"assigned_to": body.AssignedTo,
		"status":      newStatus,
	}
	if err := db.Model(&request).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign request"})
		return
	}

	// Create trace for assignment
	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)
	operatorName, operatorRole := h.getOperatorInfo(userID, userRole)

	// Get assignee name
	var assignee master.User
	var remark string
	if h.masterDB.First(&assignee, body.AssignedTo).Error == nil {
		remark = "Assigned to " + assignee.FullName
	}

	// Determine if this is new assignment or reassignment
	action := property.TraceActionAssigned
	if oldAssignedTo != nil && *oldAssignedTo > 0 {
		action = property.TraceActionReassigned
	}
	h.createRequestTrace(db, uint(id), action, nil, nil, userID, operatorName, operatorRole, &remark, c)

	// Create status change trace if status changed
	if oldStatus != newStatus {
		h.createRequestTrace(db, uint(id), property.TraceActionStatusChanged, &oldStatus, &newStatus, userID, operatorName, operatorRole, nil, c)
	}

	db.Preload("Unit").Preload("Parking").First(&request, id)
	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Request assigned successfully"})
}

// ResolveRequest marks a request as resolved (property_staff only)
// @Summary Resolve request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} RequestResponse
// @Router /requests/{id}/resolve [post]
func (h *RequestHandler) ResolveRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only property_staff can resolve requests
	if userRole != "property_staff" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only property staff can resolve requests"})
		return
	}

	var body struct {
		Resolution string `json:"resolution" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := propertyDB
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Store old status for trace
	oldStatus := request.Status
	newStatus := property.RequestStatusCompleted

	// Validate status transition
	if err := h.validateStatusTransition(request.Status, newStatus); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	uid := userID
	updates := map[string]interface{}{
		"status":      newStatus,
		"resolution":  body.Resolution,
		"resolved_at": &now,
		"resolved_by": &uid,
	}
	if err := db.Model(&request).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve request"})
		return
	}

	// Create trace for resolution
	operatorName, operatorRole := h.getOperatorInfo(userID, userRole)
	h.createRequestTrace(db, uint(id), property.TraceActionResolved, &oldStatus, &newStatus, userID, operatorName, operatorRole, &body.Resolution, c)

	db.Preload("Unit").Preload("Parking").First(&request, id)
	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Request resolved successfully"})
}

// ApproveRequest approves a request (property_staff, or landlord/spa for their tenant's requests)
// @Summary Approve request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} RequestResponse
// @Router /requests/{id}/approve [post]
func (h *RequestHandler) ApproveRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Parse request body for comment
	var body struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&body)

	db := propertyDB
	var request property.Request
	if err := db.Preload("Unit").Preload("Parking").First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Check permission: property_staff can approve any request
	// landlord/spa can approve requests from tenants in their units
	canApprove := false
	if userRole == "property_staff" || userRole == "property_admin" {
		canApprove = true
	} else if userRole == "landlord" || userRole == "spa" {
		// Check if this request is from a tenant in their unit
		if request.UnitID != nil && *request.UnitID > 0 && request.UserID != userID {
			if userRole == "landlord" {
				var count int64
				db.Model(&property.Landlord{}).Where("user_id = ? AND unit_id = ?", userID, *request.UnitID).Count(&count)
				canApprove = count > 0
			} else if userRole == "spa" {
				var count int64
				db.Model(&property.SPAUnit{}).Where("spa_user_id = ? AND unit_id = ? AND is_active = ?", userID, *request.UnitID, true).Count(&count)
				canApprove = count > 0
			}
		}
	}

	if !canApprove {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to approve this request"})
		return
	}

	// Can only approve requests that are pending
	if request.Status != property.RequestStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only approve pending requests"})
		return
	}

	// Store old status for trace
	oldStatus := request.Status
	newStatus := property.RequestStatusInProgress

	// Validate status transition
	if err := h.validateStatusTransition(request.Status, newStatus); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update status to in_progress
	if err := db.Model(&request).Update("status", newStatus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve request"})
		return
	}

	// For MoveIn/MoveOut requests, also update the specific record status to approved
	if request.RequestType == property.RequestTypeMoveIn {
		if err := db.Model(&property.MoveIn{}).Where("request_id = ?", id).Update("status", property.MoveInStatusApproved).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update move-in status"})
			return
		}
	} else if request.RequestType == property.RequestTypeMoveOut {
		if err := db.Model(&property.MoveOut{}).Where("request_id = ?", id).Update("status", property.MoveOutStatusApproved).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update move-out status"})
			return
		}
	}

	// Create trace for approval
	operatorName, operatorRole := h.getOperatorInfo(userID, userRole)
	remark := "Request approved"
	if body.Comment != "" {
		remark = "Request approved: " + body.Comment
	}
	h.createRequestTrace(db, uint(id), property.TraceActionStatusChanged, &oldStatus, &newStatus, userID, operatorName, operatorRole, &remark, c)

	db.Preload("Unit").Preload("Parking").First(&request, id)
	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Request approved successfully"})
}

// RejectRequest rejects a request (property_staff, or landlord/spa for their tenant's requests)
// @Summary Reject request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} RequestResponse
// @Router /requests/{id}/reject [post]
func (h *RequestHandler) RejectRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)

	db := propertyDB
	var request property.Request
	if err := db.Preload("Unit").Preload("Parking").First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Check permission: property_staff can reject any request
	// landlord/spa can reject requests from tenants in their units
	canReject := false
	if userRole == "property_staff" || userRole == "property_admin" {
		canReject = true
	} else if userRole == "landlord" || userRole == "spa" {
		// Check if this request is from a tenant in their unit
		if request.UnitID != nil && *request.UnitID > 0 && request.UserID != userID {
			if userRole == "landlord" {
				var count int64
				db.Model(&property.Landlord{}).Where("user_id = ? AND unit_id = ?", userID, *request.UnitID).Count(&count)
				canReject = count > 0
			} else if userRole == "spa" {
				var count int64
				db.Model(&property.SPAUnit{}).Where("spa_user_id = ? AND unit_id = ? AND is_active = ?", userID, *request.UnitID, true).Count(&count)
				canReject = count > 0
			}
		}
	}

	if !canReject {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to reject this request"})
		return
	}

	// Can only reject requests that are pending
	if request.Status != property.RequestStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only reject pending requests"})
		return
	}

	// Store old status for trace
	oldStatus := request.Status
	newStatus := property.RequestStatusRejected

	// Validate status transition
	if err := h.validateStatusTransition(request.Status, newStatus); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update status to rejected and store reason in resolution field
	updates := map[string]interface{}{
		"status":     newStatus,
		"resolution": body.Reason,
	}
	if err := db.Model(&request).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject request"})
		return
	}

	// For MoveIn/MoveOut requests, also update the specific record status to rejected
	if request.RequestType == property.RequestTypeMoveIn {
		if err := db.Model(&property.MoveIn{}).Where("request_id = ?", id).Update("status", property.MoveInStatusRejected).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update move-in status"})
			return
		}
	} else if request.RequestType == property.RequestTypeMoveOut {
		if err := db.Model(&property.MoveOut{}).Where("request_id = ?", id).Update("status", property.MoveOutStatusRejected).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update move-out status"})
			return
		}
	}

	// Create trace for rejection
	operatorName, operatorRole := h.getOperatorInfo(userID, userRole)
	remark := "Request rejected"
	if body.Reason != "" {
		remark = "Request rejected: " + body.Reason
	}
	h.createRequestTrace(db, uint(id), property.TraceActionRejected, &oldStatus, &newStatus, userID, operatorName, operatorRole, &remark, c)

	db.Preload("Unit").Preload("Parking").First(&request, id)
	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Request rejected successfully"})
}

// ResubmitRequest allows the original requester to edit and resubmit a rejected request
// @Summary Resubmit rejected request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param body body UpdateRequestRequest true "Updated request data"
// @Success 200 {object} RequestResponse
// @Router /requests/{id}/resubmit [post]
func (h *RequestHandler) ResubmitRequest(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Try to get form data first (multipart), fall back to JSON
	title := c.PostForm("title")
	description := c.PostForm("description")
	priority := c.PostForm("priority")
	comment := c.PostForm("comment")

	// If form data is empty, try JSON
	if title == "" && description == "" && priority == "" {
		var body struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    string `json:"priority"`
			Comment     string `json:"comment"`
		}
		c.ShouldBindJSON(&body)
		title = body.Title
		description = body.Description
		priority = body.Priority
		comment = body.Comment
	}

	db := propertyDB
	var request property.Request
	if err := db.Preload("Unit").Preload("Parking").First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Only the original requester can resubmit
	if request.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the original requester can resubmit this request"})
		return
	}

	// Can resubmit rejected or in_progress requests
	if request.Status != property.RequestStatusRejected && request.Status != property.RequestStatusInProgress {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only resubmit rejected or in_progress requests"})
		return
	}

	// Store old status for trace
	oldStatus := request.Status
	newStatus := property.RequestStatusPending

	// Prepare updates
	updates := map[string]interface{}{
		"status":      newStatus,
		"resolution":  nil, // Clear the rejection reason
		"assigned_to": nil, // Clear assignment so it goes back to pending queue
	}

	if title != "" {
		updates["title"] = title
	}
	if description != "" {
		updates["description"] = description
	}
	if priority != "" && (priority == "low" || priority == "normal" || priority == "high" || priority == "urgent") {
		updates["priority"] = priority
	}

	if err := db.Model(&request).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resubmit request"})
		return
	}

	// Handle file uploads (attachments)
	h.saveRequestAttachments(c, db, uint(id), userID)

	// Create trace for resubmission
	operatorName, operatorRole := h.getOperatorInfo(userID, userRole)
	remark := "Request resubmitted"
	if oldStatus == property.RequestStatusRejected {
		remark = "Request resubmitted after rejection"
	} else if oldStatus == property.RequestStatusInProgress {
		remark = "Request resubmitted after being in progress"
	}
	if comment != "" {
		remark = "Request resubmitted: " + comment
	}
	h.createRequestTrace(db, uint(id), property.TraceActionResubmitted, &oldStatus, &newStatus, userID, operatorName, operatorRole, &remark, c)

	db.Preload("Unit").Preload("Parking").First(&request, id)
	resp := h.buildRequestResponse(request, db)
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "Request resubmitted successfully"})
}

// GetMyRequests gets current user's requests
// For landlord/spa: also includes requests from tenants in their units
// @Summary Get my requests
// @Tags Requests
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /requests/my [get]
func (h *RequestHandler) GetMyRequests(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var req ListRequestsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	db := propertyDB
	var query *gorm.DB

	// For landlord/spa: get requests from themselves AND from tenants in their units
	if userRole == "landlord" || userRole == "spa" {
		// Get unit IDs associated with this landlord/spa
		var unitIDs []uint

		if userRole == "landlord" {
			// Get units owned by this landlord
			var landlordUnits []property.Landlord
			db.Where("user_id = ?", userID).Find(&landlordUnits)
			for _, lu := range landlordUnits {
				unitIDs = append(unitIDs, lu.UnitID)
			}
		} else if userRole == "spa" {
			// Get units managed by this SPA
			var spaUnits []property.SPAUnit
			db.Where("spa_user_id = ? AND is_active = ?", userID, true).Find(&spaUnits)
			for _, su := range spaUnits {
				unitIDs = append(unitIDs, su.UnitID)
			}
		}

		// Query: user's own requests OR requests from their units
		if len(unitIDs) > 0 {
			query = db.Model(&property.Request{}).Where(
				"user_id = ? OR unit_id IN ?",
				userID, unitIDs,
			)
		} else {
			// No associated units, only show own requests
			query = db.Model(&property.Request{}).Where("user_id = ?", userID)
		}
	} else {
		// For tenant and other roles: only their own requests
		query = db.Model(&property.Request{}).Where("user_id = ?", userID)
	}

	// Apply filters
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.RequestType != "" {
		query = query.Where("request_type = ?", req.RequestType)
	}

	var total int64
	query.Count(&total)

	offset := (req.Page - 1) * req.PageSize
	var requests []property.Request
	query.Preload("Unit").Preload("Parking").Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&requests)

	responses := make([]RequestResponse, len(requests))
	for i, r := range requests {
		responses[i] = h.buildRequestResponse(r, db)
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

// GetRequestStats gets request statistics
// @Summary Get request statistics
// @Tags Requests
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /requests/stats [get]
func (h *RequestHandler) GetRequestStats(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	db := propertyDB

	var stats struct {
		Total       int64 `json:"total"`
		Pending     int64 `json:"pending"`
		InProgress  int64 `json:"in_progress"`
		Completed   int64 `json:"completed"`
		Cancelled   int64 `json:"cancelled"`
		Rejected    int64 `json:"rejected"`
		Urgent      int64 `json:"urgent"`
		House       int64 `json:"house"`
		Parking     int64 `json:"parking"`
		Maintenance int64 `json:"maintenance"`
		Complaint   int64 `json:"complaint"`
	}

	db.Model(&property.Request{}).Count(&stats.Total)
	db.Model(&property.Request{}).Where("status = ?", property.RequestStatusPending).Count(&stats.Pending)
	db.Model(&property.Request{}).Where("status = ?", property.RequestStatusInProgress).Count(&stats.InProgress)
	db.Model(&property.Request{}).Where("status = ?", property.RequestStatusCompleted).Count(&stats.Completed)
	db.Model(&property.Request{}).Where("status = ?", property.RequestStatusCancelled).Count(&stats.Cancelled)
	db.Model(&property.Request{}).Where("status = ?", property.RequestStatusRejected).Count(&stats.Rejected)
	db.Model(&property.Request{}).Where("priority = ?", "urgent").Count(&stats.Urgent)
	db.Model(&property.Request{}).Where("category = ?", "house").Count(&stats.House)
	db.Model(&property.Request{}).Where("category = ?", "parking").Count(&stats.Parking)
	db.Model(&property.Request{}).Where("request_type = ?", "maintenance").Count(&stats.Maintenance)
	db.Model(&property.Request{}).Where("request_type = ?", "complaint").Count(&stats.Complaint)

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetRequestTypes returns available request types by category
// @Summary Get request types
// @Tags Requests
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /requests/types [get]
func (h *RequestHandler) GetRequestTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"house":   property.HouseRequestTypes,
			"parking": property.ParkingRequestTypes,
		},
	})
}

// buildRequestResponse builds a request response with user info
func (h *RequestHandler) buildRequestResponse(r property.Request, db *gorm.DB) RequestResponse {
	resp := RequestResponse{
		ID:          r.ID,
		UserID:      r.UserID,
		Category:    r.Category,
		UnitID:      r.UnitID,
		ParkingID:   r.ParkingID,
		RequestType: r.RequestType,
		Title:       r.Title,
		Description: r.Description,
		Priority:    r.Priority,
		Status:      r.Status,
		AssignedTo:  r.AssignedTo,
		ResolvedAt:  r.ResolvedAt,
		ResolvedBy:  r.ResolvedBy,
		Resolution:  r.Resolution,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		Attachments: []AttachmentResponse{},
	}

	// Get unit number
	if r.Unit != nil {
		resp.UnitNumber = r.Unit.UnitNumber
	}

	// Get parking number
	if r.Parking != nil {
		resp.ParkingNumber = r.Parking.SlotNumber
	}

	// Get user info from master DB
	var user master.User
	if h.masterDB.First(&user, r.UserID).Error == nil {
		resp.UserName = user.FullName
		if user.Email != nil {
			resp.UserEmail = *user.Email
		}
	}

	// Get assigned user info
	if r.AssignedTo != nil && *r.AssignedTo > 0 {
		var assignee master.User
		if h.masterDB.First(&assignee, *r.AssignedTo).Error == nil {
			resp.AssignedName = assignee.FullName
		}
	}

	// Get resolver info
	if r.ResolvedBy != nil && *r.ResolvedBy > 0 {
		var resolver master.User
		if h.masterDB.First(&resolver, *r.ResolvedBy).Error == nil {
			resp.ResolverName = resolver.FullName
		}
	}

	// Load vehicle sticker info if request type is parking_sticker_apply
	if r.RequestType == property.RequestTypeParkingStickerApply && db != nil {
		var vehicleSticker property.VehicleSticker
		if err := db.Where("request_id = ?", r.ID).First(&vehicleSticker).Error; err == nil {
			resp.VehicleSticker = &VehicleStickerInfo{
				Status:  vehicleSticker.Status,
				IsDraft: vehicleSticker.IsDraft,
			}
		}
	}

	// Load pet registration info if request type is pet_registration
	if r.RequestType == property.RequestTypePetRegistration && db != nil {
		var petRegistration property.PetRegistration
		if err := db.Where("request_id = ?", r.ID).First(&petRegistration).Error; err == nil {
			resp.PetRegistration = &PetRegistrationInfo{
				Status:  petRegistration.Status,
				IsDraft: petRegistration.IsDraft,
			}
		}
	}

	// Load move-in info if request type is move_in
	if r.RequestType == property.RequestTypeMoveIn && db != nil {
		var moveIn property.MoveIn
		if err := db.Where("request_id = ?", r.ID).First(&moveIn).Error; err == nil {
			resp.MoveIn = &MoveInInfo{
				Status:  moveIn.Status,
				IsDraft: moveIn.IsDraft,
			}
		}
	}

	// Load move-out info if request type is move_out
	if r.RequestType == property.RequestTypeMoveOut && db != nil {
		var moveOut property.MoveOut
		if err := db.Where("request_id = ?", r.ID).First(&moveOut).Error; err == nil {
			resp.MoveOut = &MoveOutInfo{
				Status:  moveOut.Status,
				IsDraft: moveOut.IsDraft,
			}
		}
	}

	// Load household staff registration info if request type is household_staff_registration
	if r.RequestType == property.RequestTypeHouseholdStaffRegistration && db != nil {
		var hsr property.HouseholdStaffRegistration
		if err := db.Where("request_id = ?", r.ID).First(&hsr).Error; err == nil {
			resp.HouseholdStaffRegistration = &HouseholdStaffRegistrationInfo{
				Status:  hsr.Status,
				IsDraft: hsr.IsDraft,
			}
		}
	}

	// Load attachments from documents table
	if db != nil {
		var docs []property.Document
		if err := db.Where("entity_type = ? AND entity_id = ? AND is_active = ?", property.DocEntityRequest, r.ID, true).
			Order("uploaded_at DESC").Find(&docs).Error; err == nil {
			for _, doc := range docs {
				att := AttachmentResponse{
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

// Helper function
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// saveRequestAttachments saves uploaded files and creates document records
func (h *RequestHandler) saveRequestAttachments(c *gin.Context, db *gorm.DB, requestID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	files := form.File["attachments"]
	if len(files) == 0 {
		return nil
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/requests/%d", requestID)
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
		filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
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
			EntityType:   property.DocEntityRequest,
			EntityID:     requestID,
			DocumentType: property.DocTypeOther,
			DocumentName: file.Filename,
			DocumentPath: filePath,
			FileSize:     &file.Size,
			MimeType:     &mimeType,
			UploadedBy:   userID,
			IsActive:     true,
		}

		if err := db.Create(&doc).Error; err != nil {
			// Log error but continue with other files
			continue
		}
	}

	return nil
}

// sanitizeFilename removes special characters from filename
func sanitizeFilename2(name string) string {
	// Remove extension first
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]

	// Keep only alphanumeric and some safe characters
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

// RequestTraceResponse represents a request trace in API response
type RequestTraceResponse struct {
	ID           uint      `json:"id"`
	RequestID    uint      `json:"request_id"`
	Action       string    `json:"action"`
	ActionLabel  string    `json:"action_label"`
	FromStatus   *string   `json:"from_status"`
	ToStatus     *string   `json:"to_status"`
	OperatorID   uint      `json:"operator_id"`
	OperatorName string    `json:"operator_name"`
	OperatorRole string    `json:"operator_role"`
	Remark       *string   `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

// createRequestTrace creates a new trace record for a request
func (h *RequestHandler) createRequestTrace(db *gorm.DB, requestID uint, action string, fromStatus, toStatus *string, operatorID uint, operatorName, operatorRole string, remark *string, c *gin.Context) error {
	var ipAddress, userAgent *string
	if c != nil {
		ip := c.ClientIP()
		ua := c.GetHeader("User-Agent")
		ipAddress = &ip
		if ua != "" {
			// Truncate user agent if too long
			if len(ua) > 255 {
				ua = ua[:255]
			}
			userAgent = &ua
		}
	}

	trace := property.RequestTrace{
		RequestID:    requestID,
		Action:       action,
		FromStatus:   fromStatus,
		ToStatus:     toStatus,
		OperatorID:   operatorID,
		OperatorName: operatorName,
		OperatorRole: operatorRole,
		Remark:       remark,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	return db.Create(&trace).Error
}

// getOperatorInfo gets operator name and role from master DB
func (h *RequestHandler) getOperatorInfo(userID uint, role string) (string, string) {
	var user master.User
	operatorName := "Unknown"
	if h.masterDB.First(&user, userID).Error == nil {
		operatorName = user.FullName
	}
	return operatorName, role
}

// GetRequestTraces returns all traces for a request
// @Summary Get request traces
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/{id}/traces [get]
func (h *RequestHandler) GetRequestTraces(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	// Check if request exists
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Check permission - same as GetRequest
	role := userRole
	if role == "tenant" {
		if request.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	} else if role == "landlord" || role == "spa" {
		hasAccess := request.UserID == userID

		if !hasAccess && request.UnitID != nil && *request.UnitID > 0 {
			if role == "landlord" {
				var count int64
				db.Model(&property.Landlord{}).Where("user_id = ? AND unit_id = ?", userID, *request.UnitID).Count(&count)
				hasAccess = count > 0
			} else if role == "spa" {
				var count int64
				db.Model(&property.SPAUnit{}).Where("spa_user_id = ? AND unit_id = ? AND is_active = ?", userID, *request.UnitID, true).Count(&count)
				hasAccess = count > 0
			}
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	// Get traces ordered by created_at ascending (oldest first)
	var traces []property.RequestTrace
	if err := db.Where("request_id = ?", id).Order("created_at ASC").Find(&traces).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch traces"})
		return
	}

	// Build response
	responses := make([]RequestTraceResponse, len(traces))
	for i, t := range traces {
		responses[i] = RequestTraceResponse{
			ID:           t.ID,
			RequestID:    t.RequestID,
			Action:       t.Action,
			ActionLabel:  property.GetActionLabel(t.Action),
			FromStatus:   t.FromStatus,
			ToStatus:     t.ToStatus,
			OperatorID:   t.OperatorID,
			OperatorName: t.OperatorName,
			OperatorRole: t.OperatorRole,
			Remark:       t.Remark,
			CreatedAt:    t.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": responses, "count": len(responses)})
}

// AddRequestComment adds a comment/remark to a request
// @Summary Add comment to request
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param request body object true "Comment data"
// @Success 200 {object} map[string]interface{}
// @Router /requests/{id}/comments [post]
func (h *RequestHandler) AddRequestComment(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var body struct {
		Comment string `json:"comment" binding:"required,min=1,max=1000"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := propertyDB

	// Check if request exists
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Get operator info
	operatorName, operatorRole := h.getOperatorInfo(userID, userRole)

	// Create trace for comment
	if err := h.createRequestTrace(
		db,
		uint(id),
		property.TraceActionCommented,
		nil,
		nil,
		userID,
		operatorName,
		operatorRole,
		&body.Comment,
		c,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add comment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment added successfully"})
}

// UploadRequestAttachment uploads attachments for a request
// @Summary Upload request attachment
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Request ID"
// @Param files formData file true "Files to upload (max 3)"
// @Success 200 {object} map[string]interface{}
// @Router /requests/{id}/attachments [post]
func (h *RequestHandler) UploadRequestAttachment(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	userID := middleware.GetUserID(c)

	db := propertyDB

	// Check if request exists
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	// Check if user owns the request or is admin/staff
	userRole := middleware.GetUserRole(c)
	if request.UserID != userID && userRole != "property_admin" && userRole != "staff" && userRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only upload attachments to your own requests"})
		return
	}

	// Count existing attachments
	var existingCount int64
	db.Model(&property.Document{}).Where("entity_type = ? AND entity_id = ?", property.DocEntityRequest, id).Count(&existingCount)

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form data"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	// Check total attachments won't exceed 3
	if int(existingCount)+len(files) > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 3 attachments allowed per request", "current": existingCount, "uploading": len(files)})
		return
	}

	// Allowed file types
	allowedTypes := map[string]bool{
		"image/jpeg":         true,
		"image/png":          true,
		"image/gif":          true,
		"image/webp":         true,
		"application/pdf":    true,
		"application/msword": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	}

	// Max file size: 10MB
	maxSize := int64(10 * 1024 * 1024)

	var uploadedDocs []property.Document

	for _, fileHeader := range files {
		// Check file size
		if fileHeader.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File too large: " + fileHeader.Filename + " (max 10MB)"})
			return
		}

		// Check file type
		contentType := fileHeader.Header.Get("Content-Type")
		if !allowedTypes[contentType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed: " + fileHeader.Filename})
			return
		}

		// Generate unique filename
		filename := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + fileHeader.Filename

		// Save file
		uploadPath := "uploads/requests/" + strconv.FormatUint(id, 10)
		fullPath := uploadPath + "/" + filename

		// Create directory if not exists
		if err := c.SaveUploadedFile(fileHeader, fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + fileHeader.Filename})
			return
		}

		// Create document record
		fileSize := fileHeader.Size
		doc := property.Document{
			EntityType:   property.DocEntityRequest,
			EntityID:     uint(id),
			DocumentType: property.DocTypeOther,
			DocumentName: fileHeader.Filename,
			DocumentPath: "/" + fullPath,
			FileSize:     &fileSize,
			MimeType:     &contentType,
			UploadedBy:   userID,
			IsActive:     true,
		}

		if err := db.Create(&doc).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save document record"})
			return
		}

		uploadedDocs = append(uploadedDocs, doc)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Files uploaded successfully",
		"data":    uploadedDocs,
		"count":   len(uploadedDocs),
	})
}

// GetRequestAttachments gets all attachments for a request
// @Summary Get request attachments
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/{id}/attachments [get]
func (h *RequestHandler) GetRequestAttachments(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	db := propertyDB

	// Check if request exists
	var request property.Request
	if err := db.First(&request, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	// Get attachments
	var documents []property.Document
	db.Where("entity_type = ? AND entity_id = ? AND is_active = ?", property.DocEntityRequest, id, true).
		Order("created_at DESC").
		Find(&documents)

	c.JSON(http.StatusOK, gin.H{"data": documents, "count": len(documents)})
}

// DeleteRequestAttachment deletes an attachment from a request
// @Summary Delete request attachment
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param attachmentId path int true "Attachment ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/{id}/attachments/{attachmentId} [delete]
func (h *RequestHandler) DeleteRequestAttachment(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	requestID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	db := propertyDB

	// Check if request exists
	var request property.Request
	if err := db.First(&request, requestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	// Check if attachment exists and belongs to this request
	var doc property.Document
	if err := db.Where("id = ? AND entity_type = ? AND entity_id = ?", attachmentID, property.DocEntityRequest, requestID).First(&doc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
		return
	}

	// Check permission: only uploader or admin can delete
	if doc.UploadedBy != userID && userRole != "property_admin" && userRole != "staff" && userRole != "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own attachments"})
		return
	}

	// Soft delete (set is_active to false)
	if err := db.Model(&doc).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete attachment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
}
