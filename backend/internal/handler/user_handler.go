package handler

import (
	"strconv"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(),
	}
}

// UpdateProfileRequest represents update profile request
type UpdateProfileRequest struct {
	FullName          string  `json:"full_name"`
	Nickname          *string `json:"nickname"`
	Phone             string  `json:"phone"`
	PreferredLanguage string  `json:"preferred_language"`
}

// UpdatePrivacySettingsRequest represents update privacy settings request
type UpdatePrivacySettingsRequest struct {
	PublicEmail             bool `json:"public_email"`
	PublicPhone             bool `json:"public_phone"`
	PublicFullName          bool `json:"public_full_name"`
	PublicPropertyCert      bool `json:"public_property_cert"`
	PublicVehicleCROR       bool `json:"public_vehicle_cr_or"`
	EmailNotificationEnabled bool `json:"email_notification_enabled"`
}

// GetProfile gets current user profile
// @Summary Get user profile
// @Description Get current authenticated user profile
// @Tags User
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /user/profile [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	user, err := h.userService.GetProfileWithRoles(userID)
	if err != nil {
		utils.NotFoundResponse(c, "User not found")
		return
	}

	// Get associated data based on user role
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.SuccessResponse(c, "Profile retrieved successfully", user)
		return
	}

	response := map[string]interface{}{"user": user}

	switch userRole {
	case "landlord":
		// Fetch landlord's units and parking slots
		var units []map[string]interface{}
		var parkingSlots []map[string]interface{}

		// Get landlord's units
		var landlordRecords []property.Landlord
		propertyDB.Where("user_id = ?", userID).Find(&landlordRecords)
		for _, landlord := range landlordRecords {
			var unit property.Unit
			if err := propertyDB.First(&unit, landlord.UnitID).Error; err == nil {
				unitData := map[string]interface{}{
					"id":           unit.ID,
					"unit_number":  unit.UnitNumber,
					"building":     unit.Building,
					"floor":        unit.Floor,
					"area":         unit.Area,
					"bedrooms":     unit.Bedrooms,
					"bathrooms":    unit.Bathrooms,
					"monthly_rent": unit.MonthlyRent,
					"ownership_percentage": landlord.OwnershipPercentage,
					"ownership_start_date": landlord.OwnershipStartDate,
					"ownership_end_date":   landlord.OwnershipEndDate,
				}
				units = append(units, unitData)
			}
		}

		// Get landlord's parking slots
		var landlordParkingRecords []property.LandlordParkingSlot
		propertyDB.Where("user_id = ?", userID).Find(&landlordParkingRecords)
		for _, landlordParking := range landlordParkingRecords {
			var parkingSlot property.ParkingSlot
			if err := propertyDB.First(&parkingSlot, landlordParking.ParkingSlotID).Error; err == nil {
				parkingData := map[string]interface{}{
					"id":                parkingSlot.ID,
					"slot_number":       parkingSlot.SlotNumber,
					"parking_type":      parkingSlot.ParkingType,
					"monthly_rent":      parkingSlot.MonthlyRent,
					"ownership_percentage": landlordParking.OwnershipPercentage,
					"ownership_start_date": landlordParking.OwnershipStartDate,
					"ownership_end_date":   landlordParking.OwnershipEndDate,
				}
				parkingSlots = append(parkingSlots, parkingData)
			}
		}

		response["units"] = units
		response["parking_slots"] = parkingSlots

	case "spa":
		// Fetch SPA's managed units and parking slots
		var units []map[string]interface{}
		var parkingSlots []map[string]interface{}

		// Get SPA's units
		var spaUnitRecords []property.SPAUnit
		propertyDB.Where("user_id = ? AND is_active = ?", userID, true).Find(&spaUnitRecords)
		for _, spaUnit := range spaUnitRecords {
			var unit property.Unit
			if err := propertyDB.First(&unit, spaUnit.UnitID).Error; err == nil {
				unitData := map[string]interface{}{
					"id":           unit.ID,
					"unit_number":  unit.UnitNumber,
					"building":     unit.Building,
					"floor":        unit.Floor,
					"area":         unit.Area,
					"bedrooms":     unit.Bedrooms,
					"bathrooms":    unit.Bathrooms,
					"monthly_rent": unit.MonthlyRent,
					"start_date":   spaUnit.AuthorizationStartDate,
					"end_date":     spaUnit.AuthorizationEndDate,
				}
				units = append(units, unitData)
			}
		}

		// Get SPA's parking slots
		var spaParkingRecords []property.SPAParkingSlot
		propertyDB.Where("user_id = ? AND is_active = ?", userID, true).Find(&spaParkingRecords)
		for _, spaParking := range spaParkingRecords {
			var parkingSlot property.ParkingSlot
			if err := propertyDB.First(&parkingSlot, spaParking.ParkingSlotID).Error; err == nil {
				parkingData := map[string]interface{}{
					"id":          parkingSlot.ID,
					"slot_number": parkingSlot.SlotNumber,
					"parking_type": parkingSlot.ParkingType,
					"monthly_rent": parkingSlot.MonthlyRent,
					"start_date":  spaParking.AuthorizationStartDate,
					"end_date":    spaParking.AuthorizationEndDate,
				}
				parkingSlots = append(parkingSlots, parkingData)
			}
		}

		response["units"] = units
		response["parking_slots"] = parkingSlots

	case "tenant":
		// Fetch tenant's rented units and assigned parking slots
		var units []map[string]interface{}
		var parkingSlots []map[string]interface{}

		// Get tenant's units
		var tenantRecords []property.Tenant
		propertyDB.Where("user_id = ? AND status = ?", userID, "active").Find(&tenantRecords)
		for _, tenant := range tenantRecords {
			var unit property.Unit
			if err := propertyDB.First(&unit, tenant.UnitID).Error; err == nil {
				unitData := map[string]interface{}{
					"id":               unit.ID,
					"unit_number":      unit.UnitNumber,
					"building":         unit.Building,
					"floor":            unit.Floor,
					"area":             unit.Area,
					"bedrooms":         unit.Bedrooms,
					"bathrooms":        unit.Bathrooms,
					"monthly_rent":     unit.MonthlyRent,
					"lease_start_date": tenant.LeaseStartDate,
					"lease_end_date":   tenant.LeaseEndDate,
				}
				units = append(units, unitData)
			}
		}

		// Get tenant's parking assignments
		var parkingAssignments []property.ParkingAssignment
		propertyDB.Where("user_id = ? AND status = ?", userID, "active").Find(&parkingAssignments)
		for _, assignment := range parkingAssignments {
			var parkingSlot property.ParkingSlot
			if err := propertyDB.First(&parkingSlot, assignment.ParkingSlotID).Error; err == nil {
				parkingData := map[string]interface{}{
					"id":                parkingSlot.ID,
					"slot_number":       parkingSlot.SlotNumber,
					"parking_type":      parkingSlot.ParkingType,
					"monthly_rent":      parkingSlot.MonthlyRent,
					"assignment_type":   assignment.AssignmentType,
					"vehicle_plate":     assignment.VehiclePlate,
					"start_date":        assignment.StartDate,
					"end_date":          assignment.EndDate,
				}
				parkingSlots = append(parkingSlots, parkingData)
			}
		}

		response["units"] = units
		response["parking_slots"] = parkingSlots

	default:
		response["units"] = []property.Unit{}
		response["parking_slots"] = []property.ParkingSlot{}
	}

	utils.SuccessResponse(c, "Profile retrieved successfully", response)
}

// UpdateProfile updates current user profile
// @Summary Update user profile
// @Description Update current authenticated user profile
// @Tags User
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateProfileRequest true "Update profile request"
// @Success 200 {object} map[string]interface{}
// @Router /user/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.PreferredLanguage != "" {
		updates["preferred_language"] = req.PreferredLanguage
	}

	err := h.userService.UpdateProfile(userID, updates)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Profile updated successfully", nil)
}

// UpdatePrivacySettings updates user privacy settings
// @Summary Update privacy settings
// @Description Update current user's privacy settings (only for tenant, landlord, spa)
// @Tags User
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdatePrivacySettingsRequest true "Update privacy settings request"
// @Success 200 {object} map[string]interface{}
// @Router /user/privacy [put]
func (h *UserHandler) UpdatePrivacySettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, and spa can update privacy settings
	if userRole != "tenant" && userRole != "landlord" && userRole != "spa" {
		utils.BadRequestResponse(c, "Privacy settings are only available for tenant, landlord, and spa roles", nil)
		return
	}

	var req UpdatePrivacySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	updates["public_email"] = req.PublicEmail
	updates["public_phone"] = req.PublicPhone
	updates["public_full_name"] = req.PublicFullName
	updates["public_property_cert"] = req.PublicPropertyCert
	updates["public_vehicle_cr_or"] = req.PublicVehicleCROR
	updates["email_notification_enabled"] = req.EmailNotificationEnabled

	err := h.userService.UpdateProfile(userID, updates)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Privacy settings updated successfully", nil)
}

// UpdateLanguage updates user language preference
// @Summary Update language preference
// @Description Update user's preferred language
// @Tags User
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param language query string true "Language code (en, zh-CN, zh-TW, tl)"
// @Success 200 {object} map[string]interface{}
// @Router /user/language [put]
func (h *UserHandler) UpdateLanguage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	language := c.Query("language")

	if language == "" {
		utils.BadRequestResponse(c, "Language parameter required", nil)
		return
	}

	err := h.userService.UpdateLanguagePreference(userID, language)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Language preference updated successfully", nil)
}

// ListUsers lists all users (admin only)
// @Summary List users
// @Description List all users with pagination (admin only)
// @Tags User
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /user/list [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	users, total, err := h.userService.ListUsers(page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve users", err)
		return
	}

	// Sanitize user information based on viewer
	viewerID := middleware.GetUserID(c)
	for i := range users {
		users[i].SanitizeForViewer(viewerID)
	}

	utils.SuccessResponseWithPagination(c, users, total, page, perPage, "Users retrieved successfully")
}

// SearchUsers searches users (admin only)
// @Summary Search users
// @Description Search users by name or email (admin only)
// @Tags User
// @Produce json
// @Security BearerAuth
// @Param q query string true "Search query"
// @Param role query string false "Filter by user role (optional)"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /user/search [get]
func (h *UserHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	role := c.Query("role")

	// Allow wildcard search for all users
	if query == "*" {
		query = ""
	}

	// For wildcard searches (query="*"), we allow empty query to get all users
	// For other cases, require a search query
	if query == "" && c.Query("q") != "*" {
		utils.BadRequestResponse(c, "Search query required", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	var users []master.User
	var total int64
	var err error

	if role != "" {
		// Search users by role
		users, total, err = h.userService.SearchUsersByRole(query, role, page, perPage)
	} else {
		// Search all users
		users, total, err = h.userService.SearchUsers(query, page, perPage)
	}

	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to search users", err)
		return
	}

	// Sanitize user information based on viewer
	viewerID := middleware.GetUserID(c)
	for i := range users {
		users[i].SanitizeForViewer(viewerID)
	}

	utils.SuccessResponseWithPagination(c, users, total, page, perPage, "Search results")
}

// GetUser gets user by ID (admin only, or landlord viewing their tenant)
// @Summary Get user
// @Description Get user by ID (admin only, or landlord viewing their tenant)
// @Tags User
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /user/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	viewerID := middleware.GetUserID(c)
	viewerRole := middleware.GetUserRole(c)

	// Check if viewer is admin
	isAdmin := viewerRole == "super_admin" || viewerRole == "property_admin" || viewerRole == "property_staff"
	
	// If admin, allow access without property context check
	if isAdmin {
		// Admin can view any user, proceed to fetch user
	} else if viewerRole == "landlord" {
		// Check if the target user is a tenant of any unit owned by this landlord
		propertyDB := middleware.GetPropertyDB(c)
		if propertyDB == nil {
			utils.ForbiddenResponse(c, "Property context not found")
			return
		}
		
		var landlordUnits []struct {
			UnitID uint
		}
		propertyDB.Table("landlords").
			Where("user_id = ?", viewerID).
			Select("unit_id").
			Find(&landlordUnits)
		
		var unitIDs []uint
		for _, lu := range landlordUnits {
			unitIDs = append(unitIDs, lu.UnitID)
		}
		
		if len(unitIDs) > 0 {
			var tenantCount int64
			propertyDB.Table("tenants").
				Where("user_id = ? AND unit_id IN ?", uint(id), unitIDs).
				Count(&tenantCount)
			
			if tenantCount == 0 {
				// Also check parking assignments
				var parkingSlots []struct {
					ParkingSlotID uint
				}
				propertyDB.Table("landlord_parking_slots").
					Where("user_id = ?", viewerID).
					Select("parking_slot_id").
					Find(&parkingSlots)
				
				var parkingSlotIDs []uint
				for _, ps := range parkingSlots {
					parkingSlotIDs = append(parkingSlotIDs, ps.ParkingSlotID)
				}
				
				if len(parkingSlotIDs) > 0 {
					var assignmentCount int64
					propertyDB.Table("parking_assignments").
						Where("user_id = ? AND parking_slot_id IN ? AND status = ?", uint(id), parkingSlotIDs, "active").
						Count(&assignmentCount)
					
					if assignmentCount == 0 {
						utils.ForbiddenResponse(c, "You can only view tenants of your units or parking slots")
						return
					}
				} else {
					utils.ForbiddenResponse(c, "You can only view tenants of your units or parking slots")
					return
				}
			}
		} else {
			// Check parking slots only
			var parkingSlots []struct {
				ParkingSlotID uint
			}
			propertyDB.Table("landlord_parking_slots").
				Where("user_id = ?", viewerID).
				Select("parking_slot_id").
				Find(&parkingSlots)
			
			var parkingSlotIDs []uint
			for _, ps := range parkingSlots {
				parkingSlotIDs = append(parkingSlotIDs, ps.ParkingSlotID)
			}
			
			if len(parkingSlotIDs) > 0 {
				var assignmentCount int64
				propertyDB.Table("parking_assignments").
					Where("user_id = ? AND parking_slot_id IN ? AND status = ?", uint(id), parkingSlotIDs, "active").
					Count(&assignmentCount)
				
				if assignmentCount == 0 {
					utils.ForbiddenResponse(c, "You can only view tenants of your units or parking slots")
					return
				}
			} else {
				utils.ForbiddenResponse(c, "You can only view tenants of your units or parking slots")
				return
			}
		}
	} else {
		utils.ForbiddenResponse(c, "Access denied")
		return
	}

	user, err := h.userService.GetProfileWithRoles(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "User not found")
		return
	}

	// Sanitize user information based on viewer
	user.SanitizeForViewer(viewerID)

	utils.SuccessResponse(c, "User retrieved successfully", user)
}

// ActivateUser activates a user (admin only)
// @Summary Activate user
// @Description Activate a user account (admin only)
// @Tags User
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /user/{id}/activate [post]
func (h *UserHandler) ActivateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	err = h.userService.ActivateUser(uint(id))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to activate user", err)
		return
	}

	utils.SuccessResponse(c, "User activated successfully", nil)
}

// SuspendUser suspends a user (admin only)
// @Summary Suspend user
// @Description Suspend a user account (admin only)
// @Tags User
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /user/{id}/suspend [post]
func (h *UserHandler) SuspendUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	err = h.userService.SuspendUser(uint(id))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to suspend user", err)
		return
	}

	utils.SuccessResponse(c, "User suspended successfully", nil)
}

// DeactivateUser deactivates a user (admin only)
// @Summary Deactivate user
// @Description Deactivate a user account (admin only)
// @Tags User
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /user/{id}/deactivate [post]
func (h *UserHandler) DeactivateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	err = h.userService.DeactivateUser(uint(id))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to deactivate user", err)
		return
	}

	utils.SuccessResponse(c, "User deactivated successfully", nil)
}

// GetOwnerNames gets owner user names list
// @Summary Get owner names list
// @Description Get list of owner (landlord) user names for current property
// @Tags User
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /user/owners/names [get]
func (h *UserHandler) GetOwnerNames(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.InternalServerErrorResponse(c, "Failed to get property ID", nil)
		return
	}

	users, err := h.userService.GetUserNamesByRole(propertyID, "landlord")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve owner names", err)
		return
	}

	utils.SuccessResponse(c, "Owner names retrieved successfully", map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

// GetTenantNames gets tenant user names list
// @Summary Get tenant names list
// @Description Get list of tenant user names for current property
// @Tags User
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /user/tenants/names [get]
func (h *UserHandler) GetTenantNames(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.InternalServerErrorResponse(c, "Failed to get property ID", nil)
		return
	}

	users, err := h.userService.GetUserNamesByRole(propertyID, "tenant")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve tenant names", err)
		return
	}

	utils.SuccessResponse(c, "Tenant names retrieved successfully", map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

// GetSPANames gets SPA user names list
// @Summary Get SPA names list
// @Description Get list of SPA user names for current property
// @Tags User
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /user/spas/names [get]
func (h *UserHandler) GetSPANames(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.InternalServerErrorResponse(c, "Failed to get property ID", nil)
		return
	}

	users, err := h.userService.GetUserNamesByRole(propertyID, "spa")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve SPA names", err)
		return
	}

	utils.SuccessResponse(c, "SPA names retrieved successfully", map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}