package handler

import (
	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SPAHandler handles SPA-related operations
type SPAHandler struct {
	userService *service.UserService
	masterDB    *gorm.DB
}

// NewSPAHandler creates a new SPA handler
func NewSPAHandler(masterDB *gorm.DB) *SPAHandler {
	return &SPAHandler{
		userService: service.NewUserService(),
		masterDB:    masterDB,
	}
}

// GetSPA gets a single SPA by ID
// @Summary Get SPA by ID
// @Description Get detailed information about a specific SPA including all associated units and parking slots
// @Tags SPA
// @Accept json
// @Produce json
// @Param id path int true "SPA User ID"
// @Success 200 {object} map[string]interface{}
// @Router /spa/{id} [get]
func (h *SPAHandler) GetSPA(c *gin.Context) {
	spaID := c.Param("id")

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "Property context not found", nil)
		return
	}

	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.InternalServerErrorResponse(c, "Failed to get property ID", nil)
		return
	}

	// First check if user has SPA role in master database
	var userRole master.UserRole
	if err := h.masterDB.Where("user_id = ? AND property_id = ? AND role = 'spa' AND status = 'active'", spaID, propertyID).First(&userRole).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "SPA not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to verify SPA role", err)
		return
	}

	// Get user information from master database
	var user master.User
	if err := h.masterDB.Where("id = ?", spaID).First(&user).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "User not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to fetch user information", err)
		return
	}

	// Get SPA unit records with unit information from property database
	var spaUnits []property.SPAUnit
	if err := propertyDB.Preload("Unit").Where("spa_user_id = ? AND is_active = ?", spaID, true).Find(&spaUnits).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch SPA units", err)
		return
	}

	// Format units information
	var units []map[string]interface{}
	for _, spa := range spaUnits {
		unitInfo := map[string]interface{}{
			"id":                      spa.ID,
			"unit_id":                 spa.UnitID,
			"unit_number":             spa.Unit.UnitNumber,
			"authorization_start_date": spa.AuthorizationStartDate,
			"authorization_end_date":   spa.AuthorizationEndDate,
			"authorization_document":   spa.AuthorizationDocument,
			"scope":                   spa.Scope,
			"notes":                   spa.Notes,
			"is_active":               spa.IsActive,
		}
		units = append(units, unitInfo)
	}

	// Get SPA parking slot records with parking slot information from property database
	var spaParkingSlots []property.SPAParkingSlot
	if err := propertyDB.Preload("ParkingSlot").Where("spa_user_id = ? AND is_active = ?", spaID, true).Find(&spaParkingSlots).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch SPA parking slots", err)
		return
	}

	// Format parking slots information
	var parkingSlots []map[string]interface{}
	for _, spa := range spaParkingSlots {
		parkingInfo := map[string]interface{}{
			"id":                      spa.ID,
			"parking_slot_id":         spa.ParkingSlotID,
			"slot_number":             spa.ParkingSlot.SlotNumber,
			"authorization_start_date": spa.AuthorizationStartDate,
			"authorization_end_date":   spa.AuthorizationEndDate,
			"authorization_document":   spa.AuthorizationDocument,
			"scope":                   spa.Scope,
			"notes":                   spa.Notes,
			"is_active":               spa.IsActive,
		}
		parkingSlots = append(parkingSlots, parkingInfo)
	}

	// Format response
	spaDetail := map[string]interface{}{
		"id":            user.ID,
		"full_name":     user.FullName,
		"nickname":      user.Nickname,
		"email":         user.Email,
		"phone":         user.Phone,
		"status":        user.Status,
		"role":          userRole.Role,
		"role_status":   userRole.Status,
		"units":         units,
		"parking_slots": parkingSlots,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	}

	utils.SuccessResponse(c, "SPA retrieved successfully", spaDetail)
}