package handler

import (
	"fmt"
	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LandlordHandler handles landlord-related operations
type LandlordHandler struct {
	userService   *service.UserService
	tenantService *service.UserService // Reuse user service for tenant operations
	masterDB      *gorm.DB
}

// NewLandlordHandler creates a new landlord handler
func NewLandlordHandler(masterDB *gorm.DB) *LandlordHandler {
	return &LandlordHandler{
		userService:   service.NewUserService(),
		tenantService: service.NewUserService(),
		masterDB:      masterDB,
	}
}

// CreateLandlordRequest represents create landlord request
type CreateLandlordRequest struct {
	FullName   string `json:"full_name" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Phone      string `json:"phone"`
	UnitNumber string `json:"unit_number" binding:"required"`
	Password   string `json:"password" binding:"required,min=6"`
}

// CreateLandlord creates a new landlord user
// @Summary Create landlord
// @Description Create a new landlord user account
// @Tags Landlord
// @Accept json
// @Produce json
// @Param request body CreateLandlordRequest true "Create landlord request"
// @Success 201 {object} map[string]interface{}
// @Router /landlord/create [post]
func (h *LandlordHandler) CreateLandlord(c *gin.Context) {
	var req CreateLandlordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Create user role for this property
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.InternalServerErrorResponse(c, "Failed to get property ID", nil)
		return
	}

	// Get property database connection
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "Property context not found", nil)
		return
	}

	// Check if email already exists
	var user master.User
	if err := h.masterDB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		fmt.Println(req.Email, " not found, creating new user", err)
		user = master.User{
			Email:             &req.Email,
			FullName:          req.FullName,
			PreferredLanguage: "zh-CN",
			Status:            "active",
			EmailVerified:     true, // Admin-created landlords are automatically verified
		}
		// Set phone if provided
		if req.Phone != "" {
			user.Phone = &req.Phone
		}

		if err := h.userService.CreateUser(&user, req.Password); err != nil {
			utils.InternalServerErrorResponse(c, "Failed to create landlord user", err)
			return
		}
	}

	assignedByID := middleware.GetUserID(c)
	userRole := master.UserRole{
		UserID:     user.ID,
		PropertyID: propertyID,
		Role:       "landlord",
		Status:     "active",
		AssignedBy: &assignedByID, // Admin who created this landlord
	}

	if err := h.masterDB.Create(&userRole).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to assign landlord role", err)
		return
	}

	// Find unit by unit number and create landlord-unit association
	var unit property.Unit
	if err := propertyDB.Where("unit_number = ?", req.UnitNumber).First(&unit).Error; err != nil {
		utils.BadRequestResponse(c, "Unit not found", nil)
		return
	}

	defaultPercentage := "100.00"
	landlord := property.Landlord{
		UserID:              user.ID,
		UnitID:              unit.ID,
		OwnershipType:       "full",
		OwnershipPercentage: &defaultPercentage, // Default to 100%
	}

	if err := propertyDB.Create(&landlord).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create landlord-unit association", err)
		return
	}

	utils.SuccessResponse(c, "Landlord created successfully", map[string]interface{}{
		"user_id":     user.ID,
		"full_name":   user.FullName,
		"email":       user.Email,
		"unit_number": unit.UnitNumber,
	})
}

// ListLandlords lists all landlords for the property
// @Summary List landlords
// @Description Get all landlords for the current property
// @Tags Landlord
// @Accept json
// @Produce json
// @Param search query string false "Search by name, email, or phone"
// @Param unit_number query string false "Filter by unit number"
// @Success 200 {object} map[string]interface{}
// @Router /landlord/list [get]
func (h *LandlordHandler) ListLandlords(c *gin.Context) {
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

	// Get query parameters
	search := c.Query("search")
	unitNumber := c.Query("unit_number")

	// First, get all landlord user IDs for this property from master DB
	var userRoles []master.UserRole
	userRoleQuery := h.masterDB.Where("property_id = ? AND role = 'landlord' AND status = 'active'", propertyID)
	if err := userRoleQuery.Find(&userRoles).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch landlord roles", err)
		return
	}

	if len(userRoles) == 0 {
		utils.SuccessResponse(c, "Landlords retrieved successfully", map[string]interface{}{
			"landlords": []map[string]interface{}{},
			"total":     0,
		})
		return
	}

	// Extract user IDs
	userIDs := make([]uint, len(userRoles))
	for i, role := range userRoles {
		userIDs[i] = role.UserID
	}

	// Get users from master DB with search filter
	var users []master.User
	userQuery := h.masterDB.Where("id IN ?", userIDs)
	if search != "" {
		userQuery = userQuery.Where("full_name LIKE ? OR email LIKE ? OR phone LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if err := userQuery.Find(&users).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch users", err)
		return
	}

	// Get landlord-unit associations from property DB
	var landlords []property.Landlord
	landlordQuery := propertyDB.Where("user_id IN ?", userIDs)
	if unitNumber != "" {
		// Join with units table to filter by unit number
		landlordQuery = propertyDB.Model(&property.Landlord{}).
			Select("landlords.*").
			Joins("LEFT JOIN units ON landlords.unit_id = units.id").
			Where("landlords.user_id IN ? AND units.unit_number LIKE ?", userIDs, "%"+unitNumber+"%")
	} else {
		landlordQuery = propertyDB.Where("user_id IN ?", userIDs)
	}

	if err := landlordQuery.Find(&landlords).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch landlord associations", err)
		return
	}

	// Create a map of user_id to landlord association and unit info
	landlordMap := make(map[uint]map[string]interface{})
	for _, landlord := range landlords {
		var unit property.Unit
		if err := propertyDB.Where("id = ?", landlord.UnitID).First(&unit).Error; err == nil {
			landlordMap[landlord.UserID] = map[string]interface{}{
				"unit_id":              landlord.UnitID,
				"unit_number":          unit.UnitNumber,
				"ownership_type":       landlord.OwnershipType,
				"ownership_percentage": landlord.OwnershipPercentage,
			}
		}
	}

	// Build final response
	var result []map[string]interface{}
	for _, user := range users {
		landlordInfo := landlordMap[user.ID]
		if landlordInfo == nil {
			landlordInfo = map[string]interface{}{
				"unit_id":              nil,
				"unit_number":          nil,
				"ownership_type":       nil,
				"ownership_percentage": nil,
			}
		}

		result = append(result, map[string]interface{}{
			"id":                   user.ID,
			"full_name":            user.FullName,
			"email":                user.Email,
			"phone":                user.Phone,
			"status":               user.Status,
			"unit_id":              landlordInfo["unit_id"],
			"unit_number":          landlordInfo["unit_number"],
			"ownership_type":       landlordInfo["ownership_type"],
			"ownership_percentage": landlordInfo["ownership_percentage"],
		})
	}

	utils.SuccessResponse(c, "Landlords retrieved successfully", map[string]interface{}{
		"landlords": result,
		"total":     len(result),
	})
}

// GetLandlord gets a single landlord by ID
// @Summary Get landlord by ID
// @Description Get detailed information about a specific landlord
// @Tags Landlord
// @Accept json
// @Produce json
// @Param id path int true "Landlord ID"
// @Success 200 {object} map[string]interface{}
// @Router /landlord/{id} [get]
func (h *LandlordHandler) GetLandlord(c *gin.Context) {
	landlordID := c.Param("id")

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

	// First check if user has landlord role in master database
	var userRole master.UserRole
	if err := h.masterDB.Where("user_id = ? AND property_id = ? AND role = 'landlord' AND status = 'active'", landlordID, propertyID).First(&userRole).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "Landlord not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to verify landlord role", err)
		return
	}

	// Get user information from master database
	var user master.User
	if err := h.masterDB.Where("id = ?", landlordID).First(&user).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "User not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to fetch user information", err)
		return
	}

	// Get landlord records with unit information from property database
	var landlords []property.Landlord
	if err := propertyDB.Preload("Unit").Where("user_id = ?", landlordID).Find(&landlords).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch landlord units", err)
		return
	}

	// Format units information
	var units []map[string]interface{}
	for _, landlord := range landlords {
		unitInfo := map[string]interface{}{
			"id":                   landlord.ID,
			"unit_id":              landlord.UnitID,
			"unit_number":          landlord.Unit.UnitNumber,
			"ownership_type":       landlord.OwnershipType,
			"ownership_percentage": landlord.OwnershipPercentage,
			"ownership_start_date": landlord.OwnershipStartDate,
			"ownership_end_date":   landlord.OwnershipEndDate,
			"contract_number":      landlord.ContractNumber,
			"notes":                landlord.Notes,
		}
		units = append(units, unitInfo)
	}

	// Get landlord parking slots from property database
	var landlordParkingSlots []property.LandlordParkingSlot
	if err := propertyDB.Preload("ParkingSlot").Where("user_id = ?", landlordID).Find(&landlordParkingSlots).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch landlord parking slots", err)
		return
	}

	// Format parking slots information
	var parkingSlots []map[string]interface{}
	for _, lp := range landlordParkingSlots {
		parkingInfo := map[string]interface{}{
			"id":                   lp.ID,
			"parking_slot_id":      lp.ParkingSlotID,
			"slot_number":          lp.ParkingSlot.SlotNumber,
			"ownership_type":       lp.OwnershipType,
			"ownership_percentage": lp.OwnershipPercentage,
			"ownership_start_date": lp.OwnershipStartDate,
			"ownership_end_date":   lp.OwnershipEndDate,
			"contract_number":      lp.ContractNumber,
			"notes":                lp.Notes,
		}
		parkingSlots = append(parkingSlots, parkingInfo)
	}

	// Format response
	landlordDetail := map[string]interface{}{
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

	utils.SuccessResponse(c, "Landlord retrieved successfully", landlordDetail)
}

// UpdateLandlord updates landlord information
// @Summary Update landlord
// @Description Update landlord information
// @Tags Landlord
// @Accept json
// @Produce json
// @Param id path int true "Landlord ID"
// @Param request body UpdateLandlordRequest true "Update landlord request"
// @Success 200 {object} map[string]interface{}
// @Router /landlord/{id} [put]
func (h *LandlordHandler) UpdateLandlord(c *gin.Context) {
	var req UpdateLandlordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	landlordID := c.Param("id")

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

	// Update user information
	userUpdate := map[string]interface{}{
		"full_name": req.FullName,
		"email":     req.Email,
		"phone":     req.Phone,
	}

	if err := h.masterDB.Model(&master.User{}).Where("id = ?", landlordID).Updates(userUpdate).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update landlord information", err)
		return
	}

	utils.SuccessResponse(c, "Landlord updated successfully", nil)
}

// DeleteLandlord deletes a landlord by ID
// @Summary Delete landlord
// @Description Delete a landlord and remove their role assignments
// @Tags Landlord
// @Accept json
// @Produce json
// @Param id path int true "Landlord ID"
// @Success 200 {object} map[string]interface{}
// @Router /landlord/{id} [delete]
func (h *LandlordHandler) DeleteLandlord(c *gin.Context) {
	landlordID := c.Param("id")

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

	// Start transaction
	tx := propertyDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete landlord-unit associations
	if err := tx.Where("user_id = ?", landlordID).Delete(&property.Landlord{}).Error; err != nil {
		tx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to delete landlord associations", err)
		return
	}

	// Delete user role assignments for this property
	if err := tx.Where("user_id = ? AND property_id = ? AND role = 'landlord'", landlordID, propertyID).Delete(&master.UserRole{}).Error; err != nil {
		tx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to remove landlord role", err)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to commit transaction", err)
		return
	}

	utils.SuccessResponse(c, "Landlord deleted successfully", nil)
}

// UpdateLandlordRequest represents update landlord request
type UpdateLandlordRequest struct {
	FullName   string  `json:"full_name" binding:"required"`
	Email      string  `json:"email" binding:"required,email"`
	Phone      *string `json:"phone"`
	UnitNumber string  `json:"unit_number" binding:"required"`
}
