package handler

import (
	"fmt"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TenantHandler struct {
	userService   *service.UserService
	tenantService *service.UserService // Reuse user service for tenant operations
	masterDB      *gorm.DB
}

type CreateTenantRequest struct {
	FullName   string `json:"full_name" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Phone      string `json:"phone"`
	UnitNumber string `json:"unit_number" binding:"required"`
	Password   string `json:"password" binding:"required,min=6"`
}

type UpdateTenantRequest struct {
	FullName        string  `json:"full_name" binding:"required"`
	Email           string  `json:"email" binding:"required"`
	Phone           *string `json:"phone"`
	LeaseStartDate  string  `json:"lease_start_date" binding:"required"`
	LeaseEndDate    string  `json:"lease_end_date" binding:"required"`
	MonthlyRent     *string `json:"monthly_rent"`
	DepositAmount   *string `json:"deposit_amount"`
	ContractNumber  *string `json:"contract_number"`
	Status          string  `json:"status" binding:"required"`
	Notes           *string `json:"notes"`
}

func NewTenantHandler(masterDB *gorm.DB) *TenantHandler {
	return &TenantHandler{
		userService:   service.NewUserService(),
		tenantService: service.NewUserService(),
		masterDB:      masterDB,
	}
}

// CreateTenant creates a new tenant user in master DB and property DB
// @Summary Create tenant
// @Description Create a new tenant user account in master DB and tenant record in property DB
// @Tags Tenant
// @Accept json
// @Produce json
// @Param request body CreateTenantRequest true "Create tenant request"
// @Success 201 {object} map[string]interface{}
// @Router /tenant/create [post]
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "property_db not found in user context!", nil)
		return
	}

	// Create user in master DB
	// subdomainStr := subdomain.(string)
	user := &master.User{
		Email:             &req.Email,
		FullName:          req.FullName,
		Phone:             &req.Phone,
		PreferredLanguage: "zh-CN",
		Status:            "active",
		EmailVerified:     true, // Staff-created tenants are automatically verified
	}

	if err := h.userService.CreateUser(user, req.Password); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create user", err)
		return
	}

	// Find unit by unit number
	var unit property.Unit
	if err := propertyDB.Where("unit_number = ?", req.UnitNumber).First(&unit).Error; err != nil {
		utils.BadRequestResponse(c, "Unit not found", nil)
		return
	}

	// Now create tenant record in property DB
	propertyTx := propertyDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			propertyTx.Rollback()
		}
	}()

	// Set default lease dates (current date to one year later)
	now := time.Now()
	oneYearLater := now.AddDate(1, 0, 0)

	tenant := property.Tenant{
		UserID:         user.ID,
		UnitID:         unit.ID,
		LeaseStartDate: now,
		LeaseEndDate:   oneYearLater,
		MonthlyRent:    "0.00", // Default rent, can be updated later
		Status:         "active",
	}

	if err := propertyTx.Create(&tenant).Error; err != nil {
		propertyTx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to create tenant record", err)
		return
	}

	// Commit property DB transaction
	if err := propertyTx.Commit().Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to commit tenant creation", err)
		return
	}

	utils.CreatedResponse(c, "Tenant created successfully", map[string]interface{}{
		"user_id":     user.ID,
		"tenant_id":   tenant.ID,
		"unit_id":     unit.ID,
		"full_name":   user.FullName,
		"email":       *user.Email,
		"unit_number": unit.UnitNumber,
	})
}

// ListTenants gets a paginated list of tenants
// @Summary List tenants
// @Description Get a paginated list of tenants for the current property
// @Tags Tenant
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Param search query string false "Search query"
// @Param status query string false "Filter by status"
// @Param unit_number query string false "Filter by unit number"
// @Success 200 {object} map[string]interface{}
// @Router /tenant/list [get]
func (h *TenantHandler) ListTenants(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "property_db not found in user context!", nil)
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	search := c.Query("search")
	status := c.Query("status")
	unitNumber := c.Query("unit_number")

	// Ensure minimum values
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	// offset := (page - 1) * perPage

	// Build query
	query := propertyDB.Model(&property.Tenant{}).Preload("Unit")

	// Add filters
	if search != "" {
		query = query.Joins("JOIN users u ON tenants.user_id = u.id").
			Where("u.full_name LIKE ? OR u.email LIKE ? OR u.phone LIKE ?",
				"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if status != "" {
		query = query.Where("tenants.status = ?", status)
	}

	if unitNumber != "" {
		query = query.Joins("JOIN units ON tenants.unit_id = units.id").
			Where("units.unit_number LIKE ?", "%"+unitNumber+"%")
	}

	// Get total count
	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to count tenants", err)
		return
	}

	// Get all tenants (simplified implementation)
	var tenants []property.Tenant
	if err := query.Find(&tenants).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch tenants", err)
		return
	}

	// Format response
	tenantList := make([]map[string]interface{}, 0, len(tenants))
	for _, tenant := range tenants {
		// Get user info
		var user master.User
		if err := h.masterDB.Where("id = ?", tenant.UserID).First(&user).Error; err != nil {
			continue // Skip if user not found
		}

		// Get unit info if not preloaded
		unitNumber := ""
		if tenant.Unit.ID != 0 {
			unitNumber = tenant.Unit.UnitNumber
		}

		tenantList = append(tenantList, map[string]interface{}{
			"id":               tenant.ID,
			"user_id":          tenant.UserID,
			"unit_id":          tenant.UnitID,
			"full_name":        user.FullName,
			"email":            user.Email,
			"phone":            user.Phone,
			"unit_number":      unitNumber,
			"lease_start_date": tenant.LeaseStartDate.Format("2006-01-02"),
			"lease_end_date":   tenant.LeaseEndDate.Format("2006-01-02"),
			"monthly_rent":     tenant.MonthlyRent,
			"status":           tenant.Status,
			"created_at":       tenant.CreatedAt,
		})
	}

	totalPages := (int(totalCount) + perPage - 1) / perPage

	utils.SuccessResponse(c, "Tenants retrieved successfully", map[string]interface{}{
		"tenants":     tenantList,
		"total_count": len(tenantList),
		"total_pages": totalPages,
		"page":        page,
		"per_page":    perPage,
	})
}

// GetTenant gets a single tenant by ID
// @Summary Get tenant by ID
// @Description Get detailed information about a specific tenant including all associated units and parking slots
// @Tags Tenant
// @Accept json
// @Produce json
// @Param id path int true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Router /tenant/{id} [get]
func (h *TenantHandler) GetTenant(c *gin.Context) {
	tenantID := c.Param("id")

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "property_db not found in user context!", nil)
		return
	}

	// Get tenant record first to find user_id
	var tenant property.Tenant
	if err := propertyDB.Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "Tenant not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to fetch tenant", err)
		return
	}

	// Get user information
	var user master.User
	if err := h.masterDB.Where("id = ?", tenant.UserID).First(&user).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch user information", err)
		return
	}

	// Get all tenant records for this user (multiple units)
	var tenants []property.Tenant
	if err := propertyDB.Preload("Unit").Where("user_id = ?", tenant.UserID).Find(&tenants).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch tenant units", err)
		return
	}

	// Format units information
	var units []map[string]interface{}
	for _, t := range tenants {
		unitInfo := map[string]interface{}{
			"id":               t.ID,
			"unit_id":          t.UnitID,
			"unit_number":      t.Unit.UnitNumber,
			"lease_start_date": t.LeaseStartDate.Format("2006-01-02"),
			"lease_end_date":   t.LeaseEndDate.Format("2006-01-02"),
			"monthly_rent":     t.MonthlyRent,
			"deposit_amount":   t.DepositAmount,
			"contract_number":  t.ContractNumber,
			"status":           t.Status,
			"notes":            t.Notes,
		}
		units = append(units, unitInfo)
	}

	// Get all parking assignments for this user
	var parkingAssignments []property.ParkingAssignment
	if err := propertyDB.Preload("ParkingSlot").Where("user_id = ?", tenant.UserID).Find(&parkingAssignments).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch parking assignments", err)
		return
	}

	// Format parking slots information
	var parkingSlots []map[string]interface{}
	for _, assignment := range parkingAssignments {
		parkingInfo := map[string]interface{}{
			"id":             assignment.ID,
			"parking_slot_id": assignment.ParkingSlotID,
			"slot_number":    assignment.ParkingSlot.SlotNumber,
			"vehicle_plate":  assignment.VehiclePlate,
			"vehicle_brand":  assignment.VehicleBrand,
			"vehicle_model":  assignment.VehicleModel,
			"vehicle_color":  assignment.VehicleColor,
			"vehicle_type":   assignment.VehicleType,
			"assignment_type": assignment.AssignmentType,
			"start_date":     assignment.StartDate.Format("2006-01-02"),
			"end_date":       assignment.EndDate,
			"monthly_fee":    assignment.MonthlyFee,
			"status":         assignment.Status,
			"notes":          assignment.Notes,
		}
		parkingSlots = append(parkingSlots, parkingInfo)
	}

	// Format response
	tenantDetail := map[string]interface{}{
		"id":            user.ID,
		"full_name":     user.FullName,
		"nickname":      user.Nickname,
		"email":         user.Email,
		"phone":         user.Phone,
		"status":        user.Status,
		"units":         units,
		"parking_slots": parkingSlots,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	}

	utils.SuccessResponse(c, "Tenant retrieved successfully", tenantDetail)
}

// UpdateTenant updates tenant information
// @Summary Update tenant
// @Description Update tenant information
// @Tags Tenant
// @Accept json
// @Produce json
// @Param id path int true "Tenant ID"
// @Param request body UpdateTenantRequest true "Update tenant request"
// @Success 200 {object} map[string]interface{}
// @Router /tenant/{id} [put]
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	var req UpdateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Log the detailed error for debugging
		fmt.Printf("UpdateTenant binding error: %v\n", err)
		utils.ValidationErrorResponse(c, err)
		return
	}

	tenantID := c.Param("id")

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "property_db not found in user context!", nil)
		return
	}

	// Get tenant
	var tenant property.Tenant
	if err := propertyDB.Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "Tenant not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to fetch tenant", err)
		return
	}

	// Update user information
	userUpdate := map[string]interface{}{
		"full_name": req.FullName,
		"email":     req.Email,
		"phone":     req.Phone,
	}

	if err := h.masterDB.Model(&master.User{}).Where("id = ?", tenant.UserID).Updates(userUpdate).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update user information", err)
		return
	}

	// Update tenant information
	tenantUpdate := make(map[string]interface{})

	// Parse lease start date
	if req.LeaseStartDate != "" {
		if startDate, err := time.Parse("2006-01-02", req.LeaseStartDate); err == nil {
			tenantUpdate["lease_start_date"] = startDate
		}
	}

	// Parse lease end date
	if req.LeaseEndDate != "" {
		if endDate, err := time.Parse("2006-01-02", req.LeaseEndDate); err == nil {
			tenantUpdate["lease_end_date"] = endDate
		}
	}

	// Update other fields
	tenantUpdate["monthly_rent"] = req.MonthlyRent
	tenantUpdate["deposit_amount"] = req.DepositAmount
	tenantUpdate["contract_number"] = req.ContractNumber
	tenantUpdate["status"] = req.Status
	tenantUpdate["notes"] = req.Notes

	if err := propertyDB.Model(&tenant).Updates(tenantUpdate).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update tenant", err)
		return
	}

	utils.SuccessResponse(c, "Tenant updated successfully", nil)
}

// DeleteTenant deletes a tenant by ID
// @Summary Delete tenant
// @Description Delete a tenant and associated user account
// @Tags Tenant
// @Accept json
// @Produce json
// @Param id path int true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Router /tenant/{id} [delete]
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	tenantID := c.Param("id")

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "property_db not found in user context!", nil)
		return
	}

	// Get tenant to find user ID
	var tenant property.Tenant
	if err := propertyDB.Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		if err.Error() == "record not found" {
			utils.NotFoundResponse(c, "Tenant not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Failed to fetch tenant", err)
		return
	}

	// Start transaction
	tx := propertyDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete tenant record from property DB
	if err := tx.Delete(&tenant).Error; err != nil {
		tx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to delete tenant", err)
		return
	}

	// Delete user record from master DB (optional - depends on business logic)
	// Uncomment if you want to delete the user account as well
	// if err := h.masterDB.Delete(&master.User{}, tenant.UserID).Error; err != nil {
	//     tx.Rollback()
	//     utils.InternalServerErrorResponse(c, "Failed to delete user", err)
	//     return
	// }

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to commit transaction", err)
		return
	}

	utils.SuccessResponse(c, "Tenant deleted successfully", nil)
}
