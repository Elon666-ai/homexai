package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
)

// CreateWorkPermitRequest represents work permit creation payload
type CreateWorkPermitRequest struct {
	TenantID            *uint                        `json:"tenant_id"`
	UnitID              uint                         `json:"unit_id" binding:"required"`
	DateOfApplication   string                       `json:"date_of_application" binding:"required"`
	WorkDescriptions    []string                     `json:"work_descriptions" binding:"required,min=1"` // noisy_work, dusty_work, hot_work
	WorkType            string                       `json:"work_type" binding:"required"`
	FromTime            string                       `json:"from_time" binding:"required"` // ISO 8601 format: 2006-01-02T15:04
	EndTime             string                       `json:"end_time" binding:"required"`  // ISO 8601 format: 2006-01-02T15:04
	DescriptionOfWork   string                       `json:"description_of_work" binding:"required"`
	Personnel           []property.Personnel         `json:"personnel" binding:"required,min=1"`
	PowerToolsMaterials []property.PowerToolMaterial `json:"power_tools_materials"`
	IsDraft             bool                         `json:"is_draft"`
}

// CreateWorkPermit creates a work permit
// @Summary Create work permit
// @Tags Requests
// @Accept json
// @Produce json
// @Param request body CreateWorkPermitRequest true "Work permit data"
// @Success 201 {object} map[string]interface{}
// @Router /requests/work-permit [post]
func (h *RequestHandler) CreateWorkPermit(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create work permits
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create work permits"})
		return
	}

	var req CreateWorkPermitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse date
	dateOfApplication, err := time.Parse("2006-01-02", req.DateOfApplication)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
		return
	}

	// Validate unit
	var unit property.Unit
	if err := propertyDB.First(&unit, req.UnitID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit ID"})
		return
	}

	// Auto-fill tenant_id if user is a tenant and has an active lease for this unit
	var tenantID *uint
	if userRole == "tenant" {
		var tenant property.Tenant
		if err := propertyDB.Where("user_id = ? AND unit_id = ? AND status = ?", userID, req.UnitID, property.TenantStatusActive).First(&tenant).Error; err == nil {
			tenantID = &tenant.ID
		}
	} else if req.TenantID != nil {
		// For landlord/spa, use provided tenant_id
		tenantID = req.TenantID
	}

	// Parse from_time and end_time (ISO 8601 format: 2006-01-02T15:04)
	fromTime, err := time.Parse("2006-01-02T15:04", req.FromTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid from_time format, use YYYY-MM-DDTHH:MM"})
		return
	}

	endTime, err := time.Parse("2006-01-02T15:04", req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format, use YYYY-MM-DDTHH:MM"})
		return
	}

	// Convert work descriptions to comma-separated string
	workDescsStr := ""
	for i, desc := range req.WorkDescriptions {
		if i > 0 {
			workDescsStr += ","
		}
		workDescsStr += desc
	}

	// Convert personnel to JSONB
	personnelJSON := property.JSONB{}
	personnelList := make([]interface{}, len(req.Personnel))
	for i, p := range req.Personnel {
		personnelList[i] = map[string]interface{}{
			"name":         p.Name,
			"company_name": p.CompanyName,
		}
	}
	personnelJSON["list"] = personnelList

	// Convert power tools/materials to JSONB
	toolsJSON := property.JSONB{}
	if len(req.PowerToolsMaterials) > 0 {
		toolsList := make([]interface{}, len(req.PowerToolsMaterials))
		for i, t := range req.PowerToolsMaterials {
			toolsList[i] = map[string]interface{}{
				"description": t.Description,
				"quantity":    t.Quantity,
			}
		}
		toolsJSON["list"] = toolsList
	} else {
		toolsJSON["list"] = []interface{}{}
	}

	// Determine status
	status := property.WorkPermitStatusPending
	if req.IsDraft {
		status = property.WorkPermitStatusDraft
	}

	// Start transaction
	tx := propertyDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create request record
	request := property.Request{
		UserID:      userID,
		UnitID:      &req.UnitID,
		Category:    property.RequestCategoryHouse,
		RequestType: property.RequestTypeWorkPermit,
		Title:       fmt.Sprintf("Work Permit - %s", unit.UnitNumber),
		Description: &req.DescriptionOfWork,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	if req.IsDraft {
		request.Status = property.RequestStatusPending
	}

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create work permit record
	workPermit := property.WorkPermit{
		RequestID:           request.ID,
		TenantID:            tenantID,
		UnitID:              req.UnitID,
		DateOfApplication:   dateOfApplication,
		WorkDescriptions:    workDescsStr,
		WorkType:            req.WorkType,
		FromTime:            fromTime,
		EndTime:             endTime,
		DescriptionOfWork:   req.DescriptionOfWork,
		Personnel:           personnelJSON,
		PowerToolsMaterials: toolsJSON,
		Status:              status,
		IsDraft:             req.IsDraft,
	}

	if err := tx.Create(&workPermit).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create work permit: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Work permit created successfully",
		"work_permit": workPermit,
		"request":     request,
	})
}

// GetWorkPermit gets a work permit by request ID
// @Summary Get work permit
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/work-permit [get]
func (h *RequestHandler) GetWorkPermit(c *gin.Context) {
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

	var workPermit property.WorkPermit
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&workPermit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work permit not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"work_permit": workPermit,
	})
}

// UpdateWorkPermitRequest represents work permit update payload
type UpdateWorkPermitRequest struct {
	DateOfApplication   *string                      `json:"date_of_application"`
	WorkDescriptions    []string                     `json:"work_descriptions"`
	WorkType            *string                      `json:"work_type"`
	FromTime            *string                      `json:"from_time"`
	EndTime             *string                      `json:"end_time"`
	DescriptionOfWork   *string                      `json:"description_of_work"`
	Personnel           []property.Personnel         `json:"personnel"`
	PowerToolsMaterials []property.PowerToolMaterial `json:"power_tools_materials"`
	IsDraft             *bool                        `json:"is_draft"`
}

// UpdateWorkPermit updates a work permit
// @Summary Update work permit
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param request body UpdateWorkPermitRequest true "Work permit update data"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/work-permit [put]
func (h *RequestHandler) UpdateWorkPermit(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	requestID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	// Find work permit with request
	var workPermit property.WorkPermit
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").First(&workPermit).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Work permit not found"})
		return
	}

	// Check permission: only creator can update
	if workPermit.Request.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only update your own work permits"})
		return
	}

	// Only allow update if draft or pending
	if workPermit.Status != property.WorkPermitStatusDraft && workPermit.Status != property.WorkPermitStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot update work permit in current status"})
		return
	}

	var req UpdateWorkPermitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields
	updates := make(map[string]interface{})

	if req.DateOfApplication != nil {
		dateOfApplication, err := time.Parse("2006-01-02", *req.DateOfApplication)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
			return
		}
		updates["date_of_application"] = dateOfApplication
	}

	if req.WorkDescriptions != nil && len(req.WorkDescriptions) > 0 {
		workDescsStr := ""
		for i, desc := range req.WorkDescriptions {
			if i > 0 {
				workDescsStr += ","
			}
			workDescsStr += desc
		}
		updates["work_descriptions"] = workDescsStr
	}

	if req.WorkType != nil {
		updates["work_type"] = *req.WorkType
	}

	if req.FromTime != nil {
		fromTime, err := time.Parse("2006-01-02T15:04", *req.FromTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid from_time format, use YYYY-MM-DDTHH:MM"})
			return
		}
		updates["from_time"] = fromTime
	}

	if req.EndTime != nil {
		endTime, err := time.Parse("2006-01-02T15:04", *req.EndTime)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format, use YYYY-MM-DDTHH:MM"})
			return
		}
		updates["end_time"] = endTime
	}

	if req.DescriptionOfWork != nil {
		updates["description_of_work"] = *req.DescriptionOfWork
	}

	if req.Personnel != nil {
		personnelJSON := property.JSONB{}
		personnelList := make([]interface{}, len(req.Personnel))
		for i, p := range req.Personnel {
			personnelList[i] = map[string]interface{}{
				"name":         p.Name,
				"company_name": p.CompanyName,
			}
		}
		personnelJSON["list"] = personnelList
		updates["personnel"] = personnelJSON
	}

	if req.PowerToolsMaterials != nil {
		toolsJSON := property.JSONB{}
		if len(req.PowerToolsMaterials) > 0 {
			toolsList := make([]interface{}, len(req.PowerToolsMaterials))
			for i, t := range req.PowerToolsMaterials {
				toolsList[i] = map[string]interface{}{
					"description": t.Description,
					"quantity":    t.Quantity,
				}
			}
			toolsJSON["list"] = toolsList
		} else {
			toolsJSON["list"] = []interface{}{}
		}
		updates["power_tools_materials"] = toolsJSON
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		if *req.IsDraft {
			updates["status"] = property.WorkPermitStatusDraft
		} else {
			updates["status"] = property.WorkPermitStatusPending
		}
	}

	if err := propertyDB.Model(&workPermit).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update work permit: " + err.Error()})
		return
	}

	// Reload work permit
	propertyDB.Preload("Request").Preload("Unit").First(&workPermit, workPermit.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Work permit updated successfully",
		"work_permit": workPermit,
	})
}

// CreateGatePassRequest represents gate pass creation payload
type CreateGatePassRequest struct {
	TenantID     *uint                   `json:"tenant_id"`
	UnitID       uint                    `json:"unit_id" binding:"required"`
	Type         string                  `json:"type" binding:"required,oneof=pull_in pull_out"`
	ContactNo    string                  `json:"contact_no" binding:"required"`
	DeliveryDate string                  `json:"delivery_date" binding:"required"`
	ItemList     []property.GatePassItem `json:"item_list" binding:"required,min=1"`
	Remarks      string                  `json:"remarks"`
	IsDraft      bool                    `json:"is_draft"`
}

// CreateGatePass creates a gate pass
// @Summary Create gate pass
// @Tags Requests
// @Accept json
// @Produce json
// @Param request body CreateGatePassRequest true "Gate pass data"
// @Success 201 {object} map[string]interface{}
// @Router /requests/gate-pass [post]
func (h *RequestHandler) CreateGatePass(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create gate passes
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create gate passes"})
		return
	}

	var req CreateGatePassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse delivery date
	deliveryDate, err := time.Parse("2006-01-02", req.DeliveryDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delivery_date format, use YYYY-MM-DD"})
		return
	}

	// Validate unit
	var unit property.Unit
	if err := propertyDB.First(&unit, req.UnitID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit ID"})
		return
	}

	// Auto-fill tenant_id if user is a tenant and has an active lease for this unit
	var tenantID *uint
	if userRole == "tenant" {
		var tenant property.Tenant
		if err := propertyDB.Where("user_id = ? AND unit_id = ? AND status = ?", userID, req.UnitID, property.TenantStatusActive).First(&tenant).Error; err == nil {
			tenantID = &tenant.ID
		}
	} else if req.TenantID != nil {
		// For landlord/spa, use provided tenant_id
		tenantID = req.TenantID
	}

	// Convert item list to JSONB
	itemListJSON := property.JSONB{}
	itemList := make([]interface{}, len(req.ItemList))
	for i, item := range req.ItemList {
		itemList[i] = map[string]interface{}{
			"quantity":            item.Quantity,
			"description":         item.Description,
			"unit_of_measurement": item.UnitOfMeasurement,
		}
	}
	itemListJSON["list"] = itemList

	// Determine status
	status := property.GatePassStatusPending
	if req.IsDraft {
		status = property.GatePassStatusDraft
	}

	// Start transaction
	tx := propertyDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create request record
	request := property.Request{
		UserID:      userID,
		UnitID:      &req.UnitID,
		Category:    property.RequestCategoryHouse,
		RequestType: property.RequestTypeGatePass,
		Title:       fmt.Sprintf("Gate Pass - %s", unit.UnitNumber),
		Description: &req.Remarks,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	if req.IsDraft {
		request.Status = property.RequestStatusPending
	}

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create gate pass record
	gatePass := property.GatePass{
		RequestID:    request.ID,
		TenantID:     tenantID,
		UnitID:       req.UnitID,
		Type:         req.Type,
		ContactNo:    req.ContactNo,
		DeliveryDate: deliveryDate,
		ItemList:     itemListJSON,
		Remarks:      req.Remarks,
		Status:       status,
		IsDraft:      req.IsDraft,
	}

	if err := tx.Create(&gatePass).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gate pass: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Gate pass created successfully",
		"gate_pass": gatePass,
		"request":   request,
	})
}

// GetGatePass gets a gate pass by request ID
// @Summary Get gate pass
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/gate-pass [get]
func (h *RequestHandler) GetGatePass(c *gin.Context) {
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

	var gatePass property.GatePass
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&gatePass).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate pass not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gate_pass": gatePass,
	})
}

// UpdateGatePassRequest represents gate pass update payload
type UpdateGatePassRequest struct {
	Type         *string                 `json:"type"`
	ContactNo    *string                 `json:"contact_no"`
	DeliveryDate *string                 `json:"delivery_date"`
	ItemList     []property.GatePassItem `json:"item_list"`
	Remarks      *string                 `json:"remarks"`
	IsDraft      *bool                   `json:"is_draft"`
}

// UpdateGatePass updates a gate pass
// @Summary Update gate pass
// @Tags Requests
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param request body UpdateGatePassRequest true "Gate pass update data"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/gate-pass [put]
func (h *RequestHandler) UpdateGatePass(c *gin.Context) {
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

	var gatePass property.GatePass
	if err := propertyDB.Where("request_id = ?", requestID).First(&gatePass).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate pass not found"})
		return
	}

	var req UpdateGatePassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.Type != nil {
		if *req.Type != property.GatePassTypePullIn && *req.Type != property.GatePassTypePullOut {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type, must be pull_in or pull_out"})
			return
		}
		updates["type"] = *req.Type
	}

	if req.ContactNo != nil {
		updates["contact_no"] = *req.ContactNo
	}

	if req.DeliveryDate != nil {
		deliveryDate, err := time.Parse("2006-01-02", *req.DeliveryDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delivery_date format, use YYYY-MM-DD"})
			return
		}
		updates["delivery_date"] = deliveryDate
	}

	if req.ItemList != nil {
		itemListJSON := property.JSONB{}
		if len(req.ItemList) > 0 {
			itemList := make([]interface{}, len(req.ItemList))
			for i, item := range req.ItemList {
				itemList[i] = map[string]interface{}{
					"quantity":            item.Quantity,
					"description":         item.Description,
					"unit_of_measurement": item.UnitOfMeasurement,
				}
			}
			itemListJSON["list"] = itemList
		} else {
			itemListJSON["list"] = []interface{}{}
		}
		updates["item_list"] = itemListJSON
	}

	if req.Remarks != nil {
		updates["remarks"] = *req.Remarks
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		if *req.IsDraft {
			updates["status"] = property.GatePassStatusDraft
		} else {
			updates["status"] = property.GatePassStatusPending
		}
	}

	if err := propertyDB.Model(&gatePass).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update gate pass: " + err.Error()})
		return
	}

	// Reload gate pass
	propertyDB.Preload("Request").Preload("Unit").First(&gatePass, gatePass.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Gate pass updated successfully",
		"gate_pass": gatePass,
	})
}

// sendNotificationEmail sends email notification if user has email notifications enabled
func (h *RequestHandler) sendNotificationEmail(userID uint, propertyID uint, subject, content string) {
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
