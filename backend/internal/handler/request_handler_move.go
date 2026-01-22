package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
)

// CreateMoveInRequest represents move-in creation payload
type CreateMoveInRequest struct {
	TenantID                    *uint  `json:"tenant_id" form:"tenant_id"`
	UnitID                      uint   `json:"unit_id" form:"unit_id" binding:"required"`
	ResidentName                string `json:"resident_name" form:"resident_name" binding:"required"`
	BuildingTower               string `json:"building_tower" form:"building_tower"`
	MobileNumber                string `json:"mobile_number" form:"mobile_number" binding:"required"`
	EmailAddress                string `json:"email_address" form:"email_address"`
	MoveInDate                  string `json:"move_in_date" form:"move_in_date" binding:"required"`
	TypeOfOccupancy             string `json:"type_of_occupancy" form:"type_of_occupancy" binding:"required,oneof=owner tenant"`
	NumberOfOccupants           int    `json:"number_of_occupants" form:"number_of_occupants" binding:"required,min=1"`
	OccupantNames               string `json:"occupant_names" form:"occupant_names"`
	WillBringFurniture          bool   `json:"will_bring_furniture" form:"will_bring_furniture"`
	EstimatedMoveInTime         string `json:"estimated_move_in_time" form:"estimated_move_in_time"`
	MovingCompanyName           string `json:"moving_company_name" form:"moving_company_name"`
	VehiclePlateNo              string `json:"vehicle_plate_no" form:"vehicle_plate_no"`
	AgreeToHouseRules           bool   `json:"agree_to_house_rules" form:"agree_to_house_rules"`
	ResponsibleForDamage        bool   `json:"responsible_for_damage" form:"responsible_for_damage"`
	UnderstandSubjectToApproval bool   `json:"understand_subject_to_approval" form:"understand_subject_to_approval"`
	IsDraft                     bool   `json:"is_draft" form:"is_draft"`
}

// CreateMoveIn creates a move-in request
// @Summary Create move-in request
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param request formData CreateMoveInRequest true "Move-in data"
// @Success 201 {object} map[string]interface{}
// @Router /requests/move-in [post]
func (h *RequestHandler) CreateMoveIn(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create move-in requests
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create move-in requests"})
		return
	}

	var req CreateMoveInRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate unit
	var unit property.Unit
	if err := propertyDB.First(&unit, req.UnitID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit ID"})
		return
	}

	// Parse move-in date
	moveInDate, err := time.Parse("2006-01-02", req.MoveInDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid move-in date format. Expected YYYY-MM-DD"})
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

	// Determine status
	status := property.MoveInStatusPending
	if req.IsDraft {
		status = property.MoveInStatusDraft
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
		RequestType: property.RequestTypeMoveIn,
		Title:       fmt.Sprintf("Move In - %s", unit.UnitNumber),
		Description: nil,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create move-in record
	buildingTower := &req.BuildingTower
	if req.BuildingTower == "" {
		buildingTower = nil
	}

	emailAddress := &req.EmailAddress
	if req.EmailAddress == "" {
		emailAddress = nil
	}

	estimatedMoveInTime := &req.EstimatedMoveInTime
	if req.EstimatedMoveInTime == "" {
		estimatedMoveInTime = nil
	}

	movingCompanyName := &req.MovingCompanyName
	if req.MovingCompanyName == "" {
		movingCompanyName = nil
	}

	vehiclePlateNo := &req.VehiclePlateNo
	if req.VehiclePlateNo == "" {
		vehiclePlateNo = nil
	}

	occupantNames := &req.OccupantNames
	if req.OccupantNames == "" {
		occupantNames = nil
	}

	moveIn := property.MoveIn{
		RequestID:                   request.ID,
		TenantID:                    tenantID,
		UnitID:                      req.UnitID,
		ResidentName:                req.ResidentName,
		BuildingTower:               buildingTower,
		MobileNumber:                req.MobileNumber,
		EmailAddress:                emailAddress,
		MoveInDate:                  moveInDate,
		TypeOfOccupancy:             req.TypeOfOccupancy,
		NumberOfOccupants:           req.NumberOfOccupants,
		OccupantNames:               occupantNames,
		WillBringFurniture:          req.WillBringFurniture,
		EstimatedMoveInTime:         estimatedMoveInTime,
		MovingCompanyName:           movingCompanyName,
		VehiclePlateNo:              vehiclePlateNo,
		AgreeToHouseRules:           req.AgreeToHouseRules,
		ResponsibleForDamage:        req.ResponsibleForDamage,
		UnderstandSubjectToApproval: req.UnderstandSubjectToApproval,
		Status:                      status,
		IsDraft:                     req.IsDraft,
	}

	if err := tx.Create(&moveIn).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create move-in: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Move-in request created successfully",
		"move_in": moveIn,
		"request": request,
	})
}

// GetMoveIn gets a move-in request by request ID
// @Summary Get move-in request
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/move-in [get]
func (h *RequestHandler) GetMoveIn(c *gin.Context) {
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

	var moveIn property.MoveIn
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&moveIn).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Move-in request not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"move_in": moveIn,
	})
}

// UpdateMoveInRequest represents move-in update payload
type UpdateMoveInRequest struct {
	ResidentName                *string `json:"resident_name" form:"resident_name"`
	BuildingTower               *string `json:"building_tower" form:"building_tower"`
	MobileNumber                *string `json:"mobile_number" form:"mobile_number"`
	EmailAddress                *string `json:"email_address" form:"email_address"`
	MoveInDate                  *string `json:"move_in_date" form:"move_in_date"`
	TypeOfOccupancy             *string `json:"type_of_occupancy" form:"type_of_occupancy"`
	NumberOfOccupants           *int    `json:"number_of_occupants" form:"number_of_occupants"`
	OccupantNames               *string `json:"occupant_names" form:"occupant_names"`
	WillBringFurniture          *bool   `json:"will_bring_furniture" form:"will_bring_furniture"`
	EstimatedMoveInTime         *string `json:"estimated_move_in_time" form:"estimated_move_in_time"`
	MovingCompanyName           *string `json:"moving_company_name" form:"moving_company_name"`
	VehiclePlateNo              *string `json:"vehicle_plate_no" form:"vehicle_plate_no"`
	AgreeToHouseRules           *bool   `json:"agree_to_house_rules" form:"agree_to_house_rules"`
	ResponsibleForDamage        *bool   `json:"responsible_for_damage" form:"responsible_for_damage"`
	UnderstandSubjectToApproval *bool   `json:"understand_subject_to_approval" form:"understand_subject_to_approval"`
	IsDraft                     *bool   `json:"is_draft" form:"is_draft"`
}

// UpdateMoveIn updates a move-in request
// @Summary Update move-in request
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Request ID"
// @Param request formData UpdateMoveInRequest true "Move-in update data"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/move-in [put]
func (h *RequestHandler) UpdateMoveIn(c *gin.Context) {
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

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var moveIn property.MoveIn
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&moveIn).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Move-in request not found"})
		return
	}

	// Check permission: allow landlord/spa/tenant of the same unit to update
	hasPermission := false

	// Creator can always update
	if moveIn.Request.UserID == userID {
		hasPermission = true
	} else if userRole == "tenant" || userRole == "landlord" || userRole == "spa" {
		// Check if user belongs to this unit
		if moveIn.Request.UnitID != nil && *moveIn.Request.UnitID > 0 {
			if userRole == "tenant" {
				var count int64
				propertyDB.Model(&property.Tenant{}).Where("unit_id = ? AND user_id = ? AND is_active = ?", *moveIn.Request.UnitID, userID, true).Count(&count)
				hasPermission = count > 0
			} else if userRole == "landlord" {
				var count int64
				propertyDB.Model(&property.Landlord{}).Where("unit_id = ? AND user_id = ?", *moveIn.Request.UnitID, userID).Count(&count)
				hasPermission = count > 0
			} else if userRole == "spa" {
				var count int64
				propertyDB.Model(&property.SPAUnit{}).Where("unit_id = ? AND spa_user_id = ? AND is_active = ?", *moveIn.Request.UnitID, userID, true).Count(&count)
				hasPermission = count > 0
			}
		}
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this move-in request"})
		return
	}

	// Can only update draft, pending, or rejected move-ins
	if moveIn.Status != property.MoveInStatusDraft && moveIn.Status != property.MoveInStatusPending && moveIn.Status != property.MoveInStatusRejected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only update draft, pending, or rejected move-in requests"})
		return
	}

	var req UpdateMoveInRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.ResidentName != nil {
		updates["resident_name"] = *req.ResidentName
	}

	if req.BuildingTower != nil {
		if *req.BuildingTower == "" {
			updates["building_tower"] = nil
		} else {
			updates["building_tower"] = *req.BuildingTower
		}
	}

	if req.MobileNumber != nil {
		updates["mobile_number"] = *req.MobileNumber
	}

	if req.EmailAddress != nil {
		if *req.EmailAddress == "" {
			updates["email_address"] = nil
		} else {
			updates["email_address"] = *req.EmailAddress
		}
	}

	if req.MoveInDate != nil {
		moveInDate, err := time.Parse("2006-01-02", *req.MoveInDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid move-in date format. Expected YYYY-MM-DD"})
			return
		}
		updates["move_in_date"] = moveInDate
	}

	if req.TypeOfOccupancy != nil {
		updates["type_of_occupancy"] = *req.TypeOfOccupancy
	}

	if req.NumberOfOccupants != nil {
		updates["number_of_occupants"] = *req.NumberOfOccupants
	}

	if req.OccupantNames != nil {
		if *req.OccupantNames == "" {
			updates["occupant_names"] = nil
		} else {
			updates["occupant_names"] = *req.OccupantNames
		}
	}

	if req.WillBringFurniture != nil {
		updates["will_bring_furniture"] = *req.WillBringFurniture
	}

	if req.EstimatedMoveInTime != nil {
		if *req.EstimatedMoveInTime == "" {
			updates["estimated_move_in_time"] = nil
		} else {
			updates["estimated_move_in_time"] = *req.EstimatedMoveInTime
		}
	}

	if req.MovingCompanyName != nil {
		if *req.MovingCompanyName == "" {
			updates["moving_company_name"] = nil
		} else {
			updates["moving_company_name"] = *req.MovingCompanyName
		}
	}

	if req.VehiclePlateNo != nil {
		if *req.VehiclePlateNo == "" {
			updates["vehicle_plate_no"] = nil
		} else {
			updates["vehicle_plate_no"] = *req.VehiclePlateNo
		}
	}

	if req.AgreeToHouseRules != nil {
		updates["agree_to_house_rules"] = *req.AgreeToHouseRules
	}

	if req.ResponsibleForDamage != nil {
		updates["responsible_for_damage"] = *req.ResponsibleForDamage
	}

	if req.UnderstandSubjectToApproval != nil {
		updates["understand_subject_to_approval"] = *req.UnderstandSubjectToApproval
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		// Update status based on is_draft
		if *req.IsDraft {
			updates["status"] = property.MoveInStatusDraft
		} else {
			updates["status"] = property.MoveInStatusPending
		}
	}

	if err := propertyDB.Model(&moveIn).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update move-in: " + err.Error()})
		return
	}

	// Reload move-in with relationships
	propertyDB.Preload("Request").Preload("Unit").First(&moveIn, moveIn.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Move-in request updated successfully",
		"move_in": moveIn,
	})
}

// CreateMoveOutRequest represents move-out creation payload
type CreateMoveOutRequest struct {
	TenantID                   *uint  `json:"tenant_id" form:"tenant_id"`
	UnitID                     uint   `json:"unit_id" form:"unit_id" binding:"required"`
	ResidentName               string `json:"resident_name" form:"resident_name" binding:"required"`
	BuildingTower              string `json:"building_tower" form:"building_tower"`
	MobileNumber               string `json:"mobile_number" form:"mobile_number" binding:"required"`
	EmailAddress               string `json:"email_address" form:"email_address"`
	MoveOutDate                string `json:"move_out_date" form:"move_out_date" binding:"required"`
	OccupancyType              string `json:"occupancy_type" form:"occupancy_type" binding:"required,oneof=owner tenant"`
	PrimaryOccupant            string `json:"primary_occupant" form:"primary_occupant"`
	OccupantNames              string `json:"occupant_names" form:"occupant_names"`
	ReasonForMoveOut           string `json:"reason_for_move_out" form:"reason_for_move_out"`
	AllKeysReturned            string `json:"all_keys_returned" form:"all_keys_returned" binding:"required,oneof=yes no"`
	UnitConditionUponMoveOut   string `json:"unit_condition_upon_move_out" form:"unit_condition_upon_move_out" binding:"required,oneof=good minor_damage major_damage"`
	AllUtilityBillsSettled     bool   `json:"all_utility_bills_settled" form:"all_utility_bills_settled"`
	AuthorizeInspection        bool   `json:"authorize_inspection" form:"authorize_inspection"`
	UnderstandDepositDeduction bool   `json:"understand_deposit_deduction" form:"understand_deposit_deduction"`
	IsDraft                    bool   `json:"is_draft" form:"is_draft"`
}

// CreateMoveOut creates a move-out request
// @Summary Create move-out request
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param request formData CreateMoveOutRequest true "Move-out data"
// @Success 201 {object} map[string]interface{}
// @Router /requests/move-out [post]
func (h *RequestHandler) CreateMoveOut(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create move-out requests
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create move-out requests"})
		return
	}

	var req CreateMoveOutRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate unit
	var unit property.Unit
	if err := propertyDB.First(&unit, req.UnitID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit ID"})
		return
	}

	// Parse move-out date
	moveOutDate, err := time.Parse("2006-01-02", req.MoveOutDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid move-out date format. Expected YYYY-MM-DD"})
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

	// Determine status
	status := property.MoveOutStatusPending
	if req.IsDraft {
		status = property.MoveOutStatusDraft
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
		RequestType: property.RequestTypeMoveOut,
		Title:       fmt.Sprintf("Move Out - %s", unit.UnitNumber),
		Description: nil,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create move-out record
	buildingTower := &req.BuildingTower
	if req.BuildingTower == "" {
		buildingTower = nil
	}

	emailAddress := &req.EmailAddress
	if req.EmailAddress == "" {
		emailAddress = nil
	}

	reasonForMoveOut := &req.ReasonForMoveOut
	if req.ReasonForMoveOut == "" {
		reasonForMoveOut = nil
	}

	occupantNames := &req.OccupantNames
	if req.OccupantNames == "" {
		occupantNames = nil
	}

	primaryOccupant := &req.PrimaryOccupant
	if req.PrimaryOccupant == "" {
		primaryOccupant = nil
	}

	moveOut := property.MoveOut{
		RequestID:                  request.ID,
		TenantID:                   tenantID,
		UnitID:                     req.UnitID,
		ResidentName:               req.ResidentName,
		BuildingTower:              buildingTower,
		MobileNumber:               req.MobileNumber,
		EmailAddress:               emailAddress,
		MoveOutDate:                moveOutDate,
		OccupancyType:              req.OccupancyType,
		PrimaryOccupant:            primaryOccupant,
		OccupantNames:              occupantNames,
		ReasonForMoveOut:           reasonForMoveOut,
		AllKeysReturned:            req.AllKeysReturned,
		UnitConditionUponMoveOut:   req.UnitConditionUponMoveOut,
		AllUtilityBillsSettled:     req.AllUtilityBillsSettled,
		AuthorizeInspection:        req.AuthorizeInspection,
		UnderstandDepositDeduction: req.UnderstandDepositDeduction,
		Status:                     status,
		IsDraft:                    req.IsDraft,
	}

	if err := tx.Create(&moveOut).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create move-out: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Move-out request created successfully",
		"move_out": moveOut,
		"request":  request,
	})
}

// GetMoveOut gets a move-out request by request ID
// @Summary Get move-out request
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/move-out [get]
func (h *RequestHandler) GetMoveOut(c *gin.Context) {
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

	var moveOut property.MoveOut
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&moveOut).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Move-out request not found"})
		return
	}

	// Find the most recent approved Move-in record for this unit to get occupant names
	var moveIn property.MoveIn
	var availableOccupantList []string
	if err := propertyDB.Where("unit_id = ? AND status = ?", moveOut.UnitID, property.MoveInStatusApproved).
		Order("approved_at DESC, move_in_date DESC").
		First(&moveIn).Error; err == nil && moveIn.OccupantNames != nil {
		// If move-out doesn't have occupant_names yet, use move-in's occupant_names
		if moveOut.OccupantNames == nil {
			moveOut.OccupantNames = moveIn.OccupantNames
		}
		// Parse into array for easier frontend consumption
		names := strings.Split(*moveIn.OccupantNames, ",")
		for _, name := range names {
			trimmed := strings.TrimSpace(name)
			if trimmed != "" {
				availableOccupantList = append(availableOccupantList, trimmed)
			}
		}
	}

	response := gin.H{
		"move_out": moveOut,
	}

	// Also include available occupant names from Move-in for frontend to use
	if len(availableOccupantList) > 0 {
		response["available_occupant_list"] = availableOccupantList
		if moveIn.OccupantNames != nil {
			response["available_occupant_names"] = *moveIn.OccupantNames
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetUnitMoveInOccupants gets occupants from approved Move-in records for a unit
// @Summary Get unit move-in occupants
// @Tags Requests
// @Produce json
// @Param unit_id query int true "Unit ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/unit-move-in-occupants [get]
func (h *RequestHandler) GetUnitMoveInOccupants(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	unitIDStr := c.Query("unit_id")
	if unitIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unit_id is required"})
		return
	}

	unitID, err := strconv.ParseUint(unitIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid unit_id"})
		return
	}

	// Find the most recent approved Move-in record for this unit to get occupant names
	var moveIn property.MoveIn
	var availableOccupantList []string
	if err := propertyDB.Where("unit_id = ? AND status = ?", unitID, property.MoveInStatusApproved).
		Order("approved_at DESC, move_in_date DESC").
		First(&moveIn).Error; err == nil && moveIn.OccupantNames != nil {
		// Parse into array for easier frontend consumption
		names := strings.Split(*moveIn.OccupantNames, ",")
		for _, name := range names {
			trimmed := strings.TrimSpace(name)
			if trimmed != "" {
				availableOccupantList = append(availableOccupantList, trimmed)
			}
		}
	}

	response := gin.H{
		"available_occupant_list": availableOccupantList,
	}

	if moveIn.OccupantNames != nil {
		response["available_occupant_names"] = *moveIn.OccupantNames
	}

	c.JSON(http.StatusOK, response)
}

// UpdateMoveOutRequest represents move-out update payload
type UpdateMoveOutRequest struct {
	ResidentName               *string `json:"resident_name" form:"resident_name"`
	BuildingTower              *string `json:"building_tower" form:"building_tower"`
	MobileNumber               *string `json:"mobile_number" form:"mobile_number"`
	EmailAddress               *string `json:"email_address" form:"email_address"`
	MoveOutDate                *string `json:"move_out_date" form:"move_out_date"`
	OccupancyType              *string `json:"occupancy_type" form:"occupancy_type"`
	PrimaryOccupant            *string `json:"primary_occupant" form:"primary_occupant"`
	OccupantNames              *string `json:"occupant_names" form:"occupant_names"`
	ReasonForMoveOut           *string `json:"reason_for_move_out" form:"reason_for_move_out"`
	AllKeysReturned            *string `json:"all_keys_returned" form:"all_keys_returned"`
	UnitConditionUponMoveOut   *string `json:"unit_condition_upon_move_out" form:"unit_condition_upon_move_out"`
	AllUtilityBillsSettled     *bool   `json:"all_utility_bills_settled" form:"all_utility_bills_settled"`
	AuthorizeInspection        *bool   `json:"authorize_inspection" form:"authorize_inspection"`
	UnderstandDepositDeduction *bool   `json:"understand_deposit_deduction" form:"understand_deposit_deduction"`
	IsDraft                    *bool   `json:"is_draft" form:"is_draft"`
}

// UpdateMoveOut updates a move-out request
// @Summary Update move-out request
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Request ID"
// @Param request formData UpdateMoveOutRequest true "Move-out update data"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/move-out [put]
func (h *RequestHandler) UpdateMoveOut(c *gin.Context) {
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

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var moveOut property.MoveOut
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&moveOut).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Move-out request not found"})
		return
	}

	// Check permission: allow landlord/spa/tenant of the same unit to update
	hasPermission := false

	// Creator can always update
	if moveOut.Request.UserID == userID {
		hasPermission = true
	} else if userRole == "tenant" || userRole == "landlord" || userRole == "spa" {
		// Check if user belongs to this unit
		if moveOut.Request.UnitID != nil && *moveOut.Request.UnitID > 0 {
			if userRole == "tenant" {
				var count int64
				propertyDB.Model(&property.Tenant{}).Where("unit_id = ? AND user_id = ? AND is_active = ?", *moveOut.Request.UnitID, userID, true).Count(&count)
				hasPermission = count > 0
			} else if userRole == "landlord" {
				var count int64
				propertyDB.Model(&property.Landlord{}).Where("unit_id = ? AND user_id = ?", *moveOut.Request.UnitID, userID).Count(&count)
				hasPermission = count > 0
			} else if userRole == "spa" {
				var count int64
				propertyDB.Model(&property.SPAUnit{}).Where("unit_id = ? AND spa_user_id = ? AND is_active = ?", *moveOut.Request.UnitID, userID, true).Count(&count)
				hasPermission = count > 0
			}
		}
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to update this move-out request"})
		return
	}

	// Can only update draft, pending, or rejected move-outs
	if moveOut.Status != property.MoveOutStatusDraft && moveOut.Status != property.MoveOutStatusPending && moveOut.Status != property.MoveOutStatusRejected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Can only update draft, pending, or rejected move-out requests"})
		return
	}

	var req UpdateMoveOutRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.ResidentName != nil {
		updates["resident_name"] = *req.ResidentName
	}

	if req.BuildingTower != nil {
		if *req.BuildingTower == "" {
			updates["building_tower"] = nil
		} else {
			updates["building_tower"] = *req.BuildingTower
		}
	}

	if req.MobileNumber != nil {
		updates["mobile_number"] = *req.MobileNumber
	}

	if req.EmailAddress != nil {
		if *req.EmailAddress == "" {
			updates["email_address"] = nil
		} else {
			updates["email_address"] = *req.EmailAddress
		}
	}

	if req.MoveOutDate != nil {
		moveOutDate, err := time.Parse("2006-01-02", *req.MoveOutDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid move-out date format. Expected YYYY-MM-DD"})
			return
		}
		updates["move_out_date"] = moveOutDate
	}

	if req.OccupancyType != nil {
		updates["occupancy_type"] = *req.OccupancyType
	}

	if req.PrimaryOccupant != nil {
		if *req.PrimaryOccupant == "" {
			updates["primary_occupant"] = nil
		} else {
			updates["primary_occupant"] = *req.PrimaryOccupant
		}
	}

	if req.OccupantNames != nil {
		if *req.OccupantNames == "" {
			updates["occupant_names"] = nil
		} else {
			updates["occupant_names"] = *req.OccupantNames
		}
	}

	if req.ReasonForMoveOut != nil {
		if *req.ReasonForMoveOut == "" {
			updates["reason_for_move_out"] = nil
		} else {
			updates["reason_for_move_out"] = *req.ReasonForMoveOut
		}
	}

	if req.AllKeysReturned != nil {
		updates["all_keys_returned"] = *req.AllKeysReturned
	}

	if req.UnitConditionUponMoveOut != nil {
		updates["unit_condition_upon_move_out"] = *req.UnitConditionUponMoveOut
	}

	if req.AllUtilityBillsSettled != nil {
		updates["all_utility_bills_settled"] = *req.AllUtilityBillsSettled
	}

	if req.AuthorizeInspection != nil {
		updates["authorize_inspection"] = *req.AuthorizeInspection
	}

	if req.UnderstandDepositDeduction != nil {
		updates["understand_deposit_deduction"] = *req.UnderstandDepositDeduction
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		// Update status based on is_draft
		if *req.IsDraft {
			updates["status"] = property.MoveOutStatusDraft
		} else {
			updates["status"] = property.MoveOutStatusPending
		}
	}

	if err := propertyDB.Model(&moveOut).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update move-out: " + err.Error()})
		return
	}

	// Reload move-out with relationships
	propertyDB.Preload("Request").Preload("Unit").First(&moveOut, moveOut.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":  "Move-out request updated successfully",
		"move_out": moveOut,
	})
}
