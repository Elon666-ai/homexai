package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"homexai/internal/database"
	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UnitHandler struct{}

func NewUnitHandler() *UnitHandler {
	return &UnitHandler{}
}

// CreateUnitRequest represents create unit request
// Note: Parking is now managed separately via parking_slots table
type CreateUnitRequest struct {
	UnitNumber  string  `json:"unit_number" form:"unit_number" binding:"required"`
	UnitType    string  `json:"unit_type" form:"unit_type" binding:"required,oneof=apartment storage commercial"`
	Floor       int     `json:"floor" form:"floor"`
	Building    string  `json:"building" form:"building"`
	Size        float64 `json:"size" form:"size"`
	Bedrooms    int     `json:"bedrooms" form:"bedrooms"`
	Bathrooms   int     `json:"bathrooms" form:"bathrooms"`
	MonthlyRent float64 `json:"monthly_rent" form:"monthly_rent"`
	Description string  `json:"description" form:"description"`
}

// UpdateUnitRequest represents update unit request
type UpdateUnitRequest struct {
	Floor       *int     `json:"floor" form:"floor"`
	Building    *string  `json:"building" form:"building"`
	Size        *float64 `json:"size" form:"size"`
	Bedrooms    *int     `json:"bedrooms" form:"bedrooms"`
	Bathrooms   *int     `json:"bathrooms" form:"bathrooms"`
	MonthlyRent *float64 `json:"monthly_rent" form:"monthly_rent"`
	Status      *string  `json:"status" form:"status"`
	Description *string  `json:"description" form:"description"`
	// Owner-related fields (only editable by property_admin)
	OwnerID             *uint   `json:"owner_id" form:"owner_id"`
	OwnershipType       *string `json:"ownership_type" form:"ownership_type"`
	OwnershipPercentage *string `json:"ownership_percentage" form:"ownership_percentage"`
	OwnershipStartDate  *string `json:"ownership_start_date" form:"ownership_start_date"`
	OwnershipEndDate    *string `json:"ownership_end_date" form:"ownership_end_date"`
	ContractNumber      *string `json:"contract_number" form:"contract_number"`
	OwnerNotes          *string `json:"owner_notes" form:"owner_notes"`
	// SPA-related fields (only editable by property_admin)
	SpaUserID              *uint   `json:"spa_user_id" form:"spa_user_id"`
	AuthorizationStartDate *string `json:"authorization_start_date" form:"authorization_start_date"`
	AuthorizationEndDate   *string `json:"authorization_end_date" form:"authorization_end_date"`
	Scope                  *string `json:"scope" form:"scope"`
	SpaNotes               *string `json:"spa_notes" form:"spa_notes"`
	IsActive               *bool   `json:"is_active" form:"is_active"`
	// Tenant-related fields (only editable by property_staff and landlord/spa)
	TenantUserID         *uint   `json:"tenant_user_id" form:"tenant_user_id"`
	LeaseStartDate       *string `json:"lease_start_date" form:"lease_start_date"`
	LeaseEndDate         *string `json:"lease_end_date" form:"lease_end_date"`
	TenantMonthlyRent    *string `json:"tenant_monthly_rent" form:"tenant_monthly_rent"`
	DepositAmount        *string `json:"deposit_amount" form:"deposit_amount"`
	TenantContractNumber *string `json:"tenant_contract_number" form:"tenant_contract_number"`
	TenantStatus         *string `json:"tenant_status" form:"tenant_status"`
	TenantNotes          *string `json:"tenant_notes" form:"tenant_notes"`
	// Support for multiple tenants
	Tenants []TenantUpdateRequest `json:"tenants" form:"tenants"`
}

// TenantUpdateRequest represents a tenant update request for batch operations
type TenantUpdateRequest struct {
	ID             uint     `json:"id"`                         // 用户ID
	TenantRecordID *uint    `json:"tenant_record_id,omitempty"` // tenant关联记录ID，用于更新现有记录
	Names          []string `json:"names,omitempty"`
	LeaseStartDate string   `json:"lease_start_date"`
	LeaseEndDate   string   `json:"lease_end_date"`
	MonthlyRent    string   `json:"monthly_rent"`
	DepositAmount  string   `json:"deposit_amount"`
	ContractNumber string   `json:"contract_number"`
	Status         string   `json:"status"`
	Notes          string   `json:"notes"`
}

// getUnitService gets unit service from property context
func (h *UnitHandler) getUnitService(c *gin.Context) *service.UnitService {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		return nil
	}
	return service.NewUnitService(propertyDB)
}

// CreateUnit creates a new unit
// @Summary Create unit
// @Description Create a new unit (apartment, parking, etc.)
// @Tags Unit
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateUnitRequest true "Create unit request"
// @Param property_certificate formData file false "Property certificate file"
// @Param unit_photos formData file false "Unit photos (max 6)"
// @Success 201 {object} map[string]interface{}
// @Router /units [post]
func (h *UnitHandler) CreateUnit(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	userID := middleware.GetUserID(c)

	// Check if multipart form
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")

	var req CreateUnitRequest
	if isMultipart {
		// Parse form data
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	} else {
		// Parse JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	}

	unit := &property.Unit{
		UnitNumber: req.UnitNumber,
		UnitType:   req.UnitType,
		Status:     property.UnitStatusAvailable,
	}

	// Set description if provided
	if req.Description != "" {
		unit.Description = &req.Description
	}

	// Set monthly rent if provided
	if req.MonthlyRent != 0 {
		rentStr := strconv.FormatFloat(req.MonthlyRent, 'f', 2, 64)
		unit.MonthlyRent = &rentStr
	}

	// Set apartment-specific fields
	if req.UnitType == property.UnitTypeApartment {
		if req.Floor != 0 {
			unit.Floor = &req.Floor
		}
		if req.Building != "" {
			unit.Building = &req.Building
		}
		if req.Size != 0 {
			sizeStr := strconv.FormatFloat(req.Size, 'f', 2, 64)
			unit.Area = &sizeStr
		}
		if req.Bedrooms != 0 {
			unit.Bedrooms = &req.Bedrooms
		}
		if req.Bathrooms != 0 {
			unit.Bathrooms = &req.Bathrooms
		}
	}

	err := unitService.CreateUnit(unit)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	// If current user is landlord, automatically create landlord-unit association
	userRole := middleware.GetUserRole(c)
	if userRole == "landlord" {
		if propertyDB != nil {
			// Check if association already exists
			var existing property.Landlord
			err := propertyDB.Where("unit_id = ? AND user_id = ?", unit.ID, userID).First(&existing).Error
			if err != nil {
				// Association doesn't exist, create it
				now := time.Now()
				landlord := property.Landlord{
					UnitID:             unit.ID,
					UserID:             userID,
					OwnershipType:      "full",
					OwnershipStartDate: &now,
				}
				if err := propertyDB.Create(&landlord).Error; err != nil {
					// Log error but don't fail the request
					fmt.Printf("Failed to create landlord association: %v\n", err)
				}
			}
		}
	}

	// Handle file uploads if multipart form
	if isMultipart && propertyDB != nil {
		if err := h.saveUnitDocuments(c, propertyDB, unit.ID, userID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save unit documents: %v\n", err)
		}
	}

	utils.CreatedResponse(c, "Unit created successfully", unit)
}

// GetUnit gets unit by ID
// @Summary Get unit
// @Description Get unit details by ID
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param id path int true "Unit ID"
// @Success 200 {object} map[string]interface{}
// @Router /units/{id} [get]
func (h *UnitHandler) GetUnit(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	// Get current user role for privacy check
	userRole := middleware.GetUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid unit ID", nil)
		return
	}

	unit, err := unitService.GetUnit(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Unit not found")
		return
	}

	// Load documents (photos) for unit - always load for all roles
	var photos []map[string]interface{}
	var documents []property.Document
	propertyDB.Where("entity_type = ? AND entity_id = ? AND document_type = ? AND is_active = ?",
		property.DocEntityUnit, unit.ID, property.DocTypePhoto, true).
		Find(&documents)

	for _, doc := range documents {
		// Convert relative path to URL
		urlPath := strings.TrimPrefix(doc.DocumentPath, "./uploads")
		if !strings.HasPrefix(urlPath, "/") {
			urlPath = "/" + urlPath
		}
		urlPath = "/api/v1/static" + urlPath

		var fileSize int64
		if doc.FileSize != nil {
			fileSize = *doc.FileSize
		}
		var mimeType string
		if doc.MimeType != nil {
			mimeType = *doc.MimeType
		}

		photos = append(photos, map[string]interface{}{
			"id":            doc.ID,
			"document_name": doc.DocumentName,
			"document_path": urlPath,
			"url":           urlPath,
			"file_size":     fileSize,
			"mime_type":     mimeType,
		})
	}

	// Get all landlords for this unit - always load for all roles
	masterDB := database.GetMasterDB()
	var landlordList []map[string]interface{}
	// Check if this is for editing (no privacy restrictions)
	forEdit := c.Query("for_edit") == "true"

	var landlords []property.Landlord
	if err := propertyDB.Where("unit_id = ?", unit.ID).Find(&landlords).Error; err == nil {
		for _, landlord := range landlords {
			if masterDB != nil && landlord.UserID > 0 {
				var owner master.User
				if err := masterDB.DB.First(&owner, landlord.UserID).Error; err == nil {
					// Get user roles for user type
					var userRoles []master.UserRole
					masterDB.DB.Where("user_id = ? AND status = ?", owner.ID, "active").Find(&userRoles)

					userType := "unknown"
					if len(userRoles) > 0 {
						userType = userRoles[0].Role // Use the first active role
					}

					var fullName string
					var email *string
					var phone *string

					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						// For editing, return real data without privacy restrictions
						fullName = owner.FullName
						email = owner.Email
						phone = owner.Phone
					} else {
						// Check privacy settings - respect user privacy preferences
						shouldShowFullName := owner.PublicFullName
						shouldShowEmail := owner.PublicEmail
						shouldShowPhone := owner.PublicPhone

						fullName = owner.FullName
						if !shouldShowFullName {
							fullName = "***"
						}

						if shouldShowEmail {
							email = owner.Email
						} else {
							emailStr := "***"
							email = &emailStr
						}

						if shouldShowPhone {
							phone = owner.Phone
						} else {
							phoneStr := "***"
							phone = &phoneStr
						}
					}

					ownerInfo := map[string]interface{}{
						"id":                   owner.ID,
						"full_name":            fullName,
						"email":                email,
						"phone":                phone,
						"user_type":            userType,
						"ownership_type":       landlord.OwnershipType,
						"ownership_percentage": landlord.OwnershipPercentage,
						"ownership_start_date": landlord.OwnershipStartDate,
						"ownership_end_date":   landlord.OwnershipEndDate,
						"contract_number":      landlord.ContractNumber,
						"notes":                landlord.Notes,
					}
					landlordList = append(landlordList, ownerInfo)
				}
			}
		}
	}

	// Get tenant information - always load for all roles
	// For editing, load any tenant (not just active); for viewing, load active tenant
	var tenantList []map[string]interface{}
	var tenants []property.Tenant
	tenantQuery := propertyDB.Where("unit_id = ?", unit.ID)
	if !forEdit {
		tenantQuery = tenantQuery.Where("status = ?", property.TenantStatusActive)
	}
	if err := tenantQuery.Order("created_at DESC").Find(&tenants).Error; err == nil {
		for _, tenant := range tenants {
			if masterDB != nil && tenant.UserID > 0 {
				var tenantUser master.User
				if err := masterDB.DB.First(&tenantUser, tenant.UserID).Error; err == nil {
					// Get user roles for user type
					var tenantUserRoles []master.UserRole
					masterDB.DB.Where("user_id = ? AND status = ?", tenantUser.ID, "active").Find(&tenantUserRoles)

					tenantUserType := "unknown"
					if len(tenantUserRoles) > 0 {
						tenantUserType = tenantUserRoles[0].Role // Use the first active role
					}

					var fullName string
					var email *string
					var phone *string

					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						// For editing, return real data without privacy restrictions
						fullName = tenantUser.FullName
						email = tenantUser.Email
						phone = tenantUser.Phone
					} else {
						// Check privacy settings - respect user privacy preferences
						shouldShowFullName := tenantUser.PublicFullName
						shouldShowEmail := tenantUser.PublicEmail
						shouldShowPhone := tenantUser.PublicPhone

						fullName = tenantUser.FullName
						if !shouldShowFullName {
							fullName = "***"
						}

						if shouldShowEmail {
							email = tenantUser.Email
						} else {
							emailStr := "***"
							email = &emailStr
						}

						if shouldShowPhone {
							phone = tenantUser.Phone
						} else {
							phoneStr := "***"
							phone = &phoneStr
						}
					}

					tenantInfo := map[string]interface{}{
						"id":        tenantUser.ID,
						"full_name": fullName,
						"email":     email,
						"phone":     phone,
						"user_type": tenantUserType,
					}

					// If for editing, include tenant lease details
					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						tenantInfo["lease_start_date"] = tenant.LeaseStartDate
						tenantInfo["lease_end_date"] = tenant.LeaseEndDate
						tenantInfo["monthly_rent"] = tenant.MonthlyRent
						tenantInfo["deposit_amount"] = tenant.DepositAmount
						tenantInfo["contract_number"] = tenant.ContractNumber
						tenantInfo["status"] = tenant.Status
						tenantInfo["notes"] = tenant.Notes
						tenantInfo["tenant_id"] = tenant.ID // The tenant association ID
					}

					tenantList = append(tenantList, tenantInfo)
				}
			}
		}
	}

	// Get all SPAs for this unit - always load for all roles
	var spaList []map[string]interface{}
	var spaUnits []property.SPAUnit
	if err := propertyDB.Where("unit_id = ? AND is_active = ?", unit.ID, true).Find(&spaUnits).Error; err == nil {
		for _, spaUnit := range spaUnits {
			if masterDB != nil && spaUnit.SpaUserID > 0 {
				var spaUser master.User
				if err := masterDB.DB.First(&spaUser, spaUnit.SpaUserID).Error; err == nil {
					// Get user roles for user type
					var spaUserRoles []master.UserRole
					masterDB.DB.Where("user_id = ? AND status = ?", spaUser.ID, "active").Find(&spaUserRoles)

					spaUserType := "unknown"
					if len(spaUserRoles) > 0 {
						spaUserType = spaUserRoles[0].Role // Use the first active role
					}

					var fullName string
					var email *string
					var phone *string

					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						// For editing, return real data without privacy restrictions
						fullName = spaUser.FullName
						email = spaUser.Email
						phone = spaUser.Phone
					} else {
						// Check privacy settings - respect user privacy preferences
						shouldShowFullName := spaUser.PublicFullName
						shouldShowEmail := spaUser.PublicEmail
						shouldShowPhone := spaUser.PublicPhone

						fullName = spaUser.FullName
						if !shouldShowFullName {
							fullName = "***"
						}

						if shouldShowEmail {
							email = spaUser.Email
						} else {
							emailStr := "***"
							email = &emailStr
						}

						if shouldShowPhone {
							phone = spaUser.Phone
						} else {
							phoneStr := "***"
							phone = &phoneStr
						}
					}

					spaInfo := map[string]interface{}{
						"id":                       spaUser.ID,
						"full_name":                fullName,
						"email":                    email,
						"phone":                    phone,
						"user_type":                spaUserType,
						"authorization_start_date": spaUnit.AuthorizationStartDate,
						"authorization_end_date":   spaUnit.AuthorizationEndDate,
						"scope":                    spaUnit.Scope,
						"notes":                    spaUnit.Notes,
						"is_active":                spaUnit.IsActive,
					}
					spaList = append(spaList, spaInfo)
				}
			}
		}
	}

	// Build response with all data - frontend will handle role-based display
	response := map[string]interface{}{
		"id":           unit.ID,
		"unit_number":  unit.UnitNumber,
		"unit_type":    unit.UnitType,
		"building":     unit.Building,
		"floor":        unit.Floor,
		"area":         unit.Area,
		"bedrooms":     unit.Bedrooms,
		"bathrooms":    unit.Bathrooms,
		"monthly_rent": unit.MonthlyRent,
		"status":       unit.Status,
		"description":  unit.Description,
		"created_at":   unit.CreatedAt,
		"updated_at":   unit.UpdatedAt,
		"photos":       photos,
		"documents":    photos, // For backward compatibility
		"landlords":    landlordList,
		"tenants":      tenantList,
		"spas":         spaList,
	}

	utils.SuccessResponse(c, "Unit retrieved successfully", response)
}

// UpdateUnit updates unit by ID
// CreateUnitTenant creates a new tenant user and associates them with the unit
// @Summary Create unit tenant
// @Description Create a new tenant user and associate them with a specific unit
// @Tags Unit
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Unit ID"
// @Param request body map[string]interface{} true "Tenant creation request"
// @Success 200 {object} map[string]interface{}
// @Router /units/{id}/tenants [post]
func (h *UnitHandler) CreateUnitTenant(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	masterDB := database.GetMasterDB()
	if masterDB == nil {
		utils.InternalServerErrorResponse(c, "Master database not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid unit ID", nil)
		return
	}

	// Check if unit exists
	var unit property.Unit
	if err := propertyDB.First(&unit, uint(id)).Error; err != nil {
		utils.NotFoundResponse(c, "Unit not found")
		return
	}

	userRole := middleware.GetUserRole(c)

	// Check permissions - only staff and landlord/spa can create tenants
	isAdmin := userRole == master.RolePropertyAdmin
	isStaff := userRole == master.RolePropertyStaff
	isLandlordOrSPA := userRole == master.RoleLandlord || userRole == master.RoleSPA

	if !isAdmin && !isStaff && !isLandlordOrSPA {
		utils.ForbiddenResponse(c, "Insufficient permissions to create tenants")
		return
	}

	// Parse request body
	var tenantData struct {
		FullName       string  `json:"full_name" binding:"required"`
		Email          string  `json:"email" binding:"required,email"`
		Phone          *string `json:"phone"`
		LeaseStartDate string  `json:"lease_start_date"`
		LeaseEndDate   string  `json:"lease_end_date"`
		MonthlyRent    *string `json:"monthly_rent"`
		DepositAmount  *string `json:"deposit_amount"`
		ContractNumber *string `json:"contract_number"`
		Status         string  `json:"status"`
		Notes          *string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&tenantData); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Check if email already exists
	var existingUser master.User
	if err := masterDB.DB.Where("email = ?", tenantData.Email).First(&existingUser).Error; err == nil {
		utils.BadRequestResponse(c, "Email already exists", nil)
		return
	}

	// Start transaction for master database
	tx := masterDB.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerErrorResponse(c, "Failed to start transaction", tx.Error)
		return
	}

	// Start transaction for property database
	propertyTx := propertyDB.Begin()
	if propertyTx.Error != nil {
		tx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to start property transaction", propertyTx.Error)
		return
	}

	// Ensure rollback on error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			propertyTx.Rollback()
			panic(r)
		}
	}()

	// Create new user
	newUser := &master.User{
		FullName: tenantData.FullName,
		Email:    &tenantData.Email,
		Phone:    tenantData.Phone,
		Status:   "active",
	}

	// Generate a temporary password
	tempPassword := utils.GenerateTempPassword(12)

	// Hash password
	passwordHash, err := utils.HashPassword(tempPassword)
	if err != nil {
		tx.Rollback()
		propertyTx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to hash password", err)
		return
	}
	newUser.PasswordHash = passwordHash

	// Create user in transaction
	if err := tx.Create(newUser).Error; err != nil {
		tx.Rollback()
		propertyTx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to create user", err)
		return
	}

	// Debug log
	fmt.Printf("[DEBUG] Created user with ID: %d, Email: %s\n", newUser.ID, *newUser.Email)

	// Assign tenant role to the user
	tenantRole := &master.UserRole{
		UserID: newUser.ID,
		Role:   master.RoleTenant,
		Status: "active",
	}

	if err := tx.Create(tenantRole).Error; err != nil {
		tx.Rollback()
		propertyTx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to assign tenant role", err)
		return
	}

	// Create tenant association with the unit
	tenant := property.Tenant{
		UserID: newUser.ID,
		UnitID: uint(id),
		Status: tenantData.Status,
		Notes:  tenantData.Notes,
	}

	// Parse dates if provided
	if tenantData.LeaseStartDate != "" {
		if startDate, err := time.Parse("2006-01-02", tenantData.LeaseStartDate); err == nil {
			tenant.LeaseStartDate = startDate
		}
	}

	if tenantData.LeaseEndDate != "" {
		if endDate, err := time.Parse("2006-01-02", tenantData.LeaseEndDate); err == nil {
			tenant.LeaseEndDate = endDate
		}
	}

	if tenantData.MonthlyRent != nil {
		tenant.MonthlyRent = *tenantData.MonthlyRent
	}

	if tenantData.DepositAmount != nil {
		tenant.DepositAmount = tenantData.DepositAmount
	}

	if tenantData.ContractNumber != nil {
		tenant.ContractNumber = tenantData.ContractNumber
	}

	if err := propertyTx.Create(&tenant).Error; err != nil {
		tx.Rollback()
		propertyTx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to create tenant association", err)
		return
	}

	// Commit both transactions
	if err := tx.Commit().Error; err != nil {
		propertyTx.Rollback()
		utils.InternalServerErrorResponse(c, "Failed to commit master transaction", err)
		return
	}

	if err := propertyTx.Commit().Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to commit property transaction", err)
		return
	}

	utils.SuccessResponse(c, "Tenant created successfully", map[string]interface{}{
		"id":               newUser.ID,
		"tenant_id":        tenant.ID, // The tenant association ID
		"full_name":        newUser.FullName,
		"email":            newUser.Email,
		"phone":            newUser.Phone,
		"lease_start_date": tenant.LeaseStartDate,
		"lease_end_date":   tenant.LeaseEndDate,
		"monthly_rent":     tenant.MonthlyRent,
		"deposit_amount":   tenant.DepositAmount,
		"contract_number":  tenant.ContractNumber,
		"status":           tenant.Status,
		"notes":            tenant.Notes,
		"temp_password":    tempPassword, // Return temp password for staff to share with tenant
	})
}

// GetUnitTenants gets all tenants for a specific unit
// @Summary Get unit tenants
// @Description Get all tenants assigned to a specific unit
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param id path int true "Unit ID"
// @Success 200 {object} map[string]interface{}
// @Router /units/{id}/tenants [get]
func (h *UnitHandler) GetUnitTenants(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid unit ID", nil)
		return
	}

	// Check if unit exists
	var unit property.Unit
	if err := propertyDB.First(&unit, uint(id)).Error; err != nil {
		utils.NotFoundResponse(c, "Unit not found")
		return
	}

	// Get all tenants for this unit
	var tenants []property.Tenant
	if err := propertyDB.Where("unit_id = ?", uint(id)).Order("created_at DESC").Find(&tenants).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve tenants", err)
		return
	}

	// Get master DB for user information
	masterDB := database.GetMasterDB()
	if masterDB == nil {
		utils.InternalServerErrorResponse(c, "Master database not found", nil)
		return
	}

	// Build tenant information with user details
	var tenantList []map[string]interface{}
	for _, tenant := range tenants {
		if tenant.UserID > 0 {
			var tenantUser master.User
			if err := masterDB.DB.First(&tenantUser, tenant.UserID).Error; err == nil {
				// Get user roles for user type
				var tenantUserRoles []master.UserRole
				masterDB.DB.Where("user_id = ? AND status = ?", tenantUser.ID, "active").Find(&tenantUserRoles)

				tenantUserType := "unknown"
				if len(tenantUserRoles) > 0 {
					tenantUserType = tenantUserRoles[0].Role // Use the first active role
				}

				// For editing, return real data without privacy restrictions
				tenantInfo := map[string]interface{}{
					"id":               tenantUser.ID,
					"full_name":        tenantUser.FullName,
					"email":            tenantUser.Email,
					"phone":            tenantUser.Phone,
					"user_type":        tenantUserType,
					"lease_start_date": tenant.LeaseStartDate,
					"lease_end_date":   tenant.LeaseEndDate,
					"monthly_rent":     tenant.MonthlyRent,
					"deposit_amount":   tenant.DepositAmount,
					"contract_number":  tenant.ContractNumber,
					"status":           tenant.Status,
					"notes":            tenant.Notes,
					"tenant_id":        tenant.ID, // The tenant association ID
				}

				tenantList = append(tenantList, tenantInfo)
			}
		}
	}

	utils.SuccessResponse(c, "Unit tenants retrieved successfully", tenantList)
}

// @Summary Update unit
// @Description Update unit details
// @Tags Unit
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "Unit ID"
// @Param request body UpdateUnitRequest true "Update unit request"
// @Param unit_photos formData file false "Unit photos (max 6)"
// @Success 200 {object} map[string]interface{}
// @Router /units/{id} [put]
func (h *UnitHandler) UpdateUnit(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid unit ID", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)
	isMultipart := c.ContentType() == "multipart/form-data" || strings.Contains(c.ContentType(), "multipart/form-data")

	var req UpdateUnitRequest
	if isMultipart {
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	}

	// Get existing unit to check type
	unit, err := unitService.GetUnit(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Unit not found")
		return
	}

	// Check permissions based on role
	isAdmin := userRole == master.RolePropertyAdmin
	isStaff := userRole == master.RolePropertyStaff
	isLandlordOrSPA := userRole == master.RoleLandlord || userRole == master.RoleSPA

	// Admin: can only update owner and spa fields, reject basic info updates
	if isAdmin && (req.Floor != nil || req.Building != nil || req.Size != nil || req.Bedrooms != nil ||
		req.Bathrooms != nil || req.MonthlyRent != nil || req.Status != nil || req.Description != nil) {
		utils.ForbiddenResponse(c, "property_admin can only edit owner/SPA-related fields, not basic unit information")
		return
	}

	// Staff: can only update basic info and tenant fields, reject owner/spa updates
	if isStaff && (req.OwnerID != nil || req.OwnershipType != nil || req.OwnershipPercentage != nil ||
		req.OwnershipStartDate != nil || req.OwnershipEndDate != nil || req.ContractNumber != nil || req.OwnerNotes != nil ||
		req.SpaUserID != nil || req.AuthorizationStartDate != nil || req.AuthorizationEndDate != nil ||
		req.Scope != nil || req.SpaNotes != nil || req.IsActive != nil) {
		utils.ForbiddenResponse(c, "property_staff can only edit basic unit information and tenant information, not owner/SPA-related fields")
		return
	}

	// Admin: reject tenant updates
	if isAdmin && (req.TenantUserID != nil || req.LeaseStartDate != nil || req.LeaseEndDate != nil ||
		req.TenantMonthlyRent != nil || req.DepositAmount != nil || req.TenantContractNumber != nil ||
		req.TenantStatus != nil || req.TenantNotes != nil) {
		utils.ForbiddenResponse(c, "property_admin can only edit owner/SPA-related fields, not tenant information")
		return
	}

	// Landlord/SPA: can update all fields (no restrictions needed here, but will check in handler)

	updates := make(map[string]interface{})

	// Basic unit fields (editable by staff and landlord/spa, not admin)
	if !isAdmin || isLandlordOrSPA {
		if req.MonthlyRent != nil {
			rentStr := strconv.FormatFloat(*req.MonthlyRent, 'f', 2, 64)
			updates["monthly_rent"] = rentStr
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}

		// Apartment-specific fields
		if unit.UnitType == property.UnitTypeApartment {
			if req.Floor != nil {
				updates["floor"] = *req.Floor
			}
			if req.Building != nil {
				updates["building"] = *req.Building
			}
			if req.Size != nil {
				sizeStr := strconv.FormatFloat(*req.Size, 'f', 2, 64)
				updates["area"] = sizeStr
			}
			if req.Bedrooms != nil {
				updates["bedrooms"] = *req.Bedrooms
			}
			if req.Bathrooms != nil {
				updates["bathrooms"] = *req.Bathrooms
			}
		}
	}

	// Only update unit fields if there are any updates to apply
	if len(updates) > 0 {
		err = unitService.UpdateUnit(uint(id), updates)
		if err != nil {
			utils.BadRequestResponse(c, err.Error(), nil)
			return
		}
	}

	// Handle owner-related fields update (only for property_admin and landlord/spa)
	if (isAdmin || isLandlordOrSPA) && req.OwnerID != nil {
		// Get or create landlord association
		var landlord property.Landlord
		err := propertyDB.Where("unit_id = ? AND user_id = ?", uint(id), *req.OwnerID).First(&landlord).Error

		if err == gorm.ErrRecordNotFound {
			// Create new landlord association
			now := time.Now()
			landlord = property.Landlord{
				UserID:             *req.OwnerID,
				UnitID:             uint(id),
				OwnershipType:      "full",
				OwnershipStartDate: &now,
			}
			if req.OwnershipType != nil {
				landlord.OwnershipType = *req.OwnershipType
			}
			if req.OwnershipPercentage != nil {
				landlord.OwnershipPercentage = req.OwnershipPercentage
			}
			if req.OwnershipStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.OwnershipStartDate); err == nil {
					landlord.OwnershipStartDate = &startDate
				}
			}
			if req.OwnershipEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.OwnershipEndDate); err == nil {
					landlord.OwnershipEndDate = &endDate
				}
			}
			if req.ContractNumber != nil {
				landlord.ContractNumber = req.ContractNumber
			}
			if req.OwnerNotes != nil {
				landlord.Notes = req.OwnerNotes
			}
			if err := propertyDB.Create(&landlord).Error; err != nil {
				fmt.Printf("Failed to create landlord association: %v\n", err)
			}
		} else if err == nil {
			// Update existing landlord association
			landlordUpdates := make(map[string]interface{})
			if req.OwnershipType != nil {
				landlordUpdates["ownership_type"] = *req.OwnershipType
			}
			if req.OwnershipPercentage != nil {
				landlordUpdates["ownership_percentage"] = *req.OwnershipPercentage
			}
			if req.OwnershipStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.OwnershipStartDate); err == nil {
					landlordUpdates["ownership_start_date"] = startDate
				}
			}
			if req.OwnershipEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.OwnershipEndDate); err == nil {
					landlordUpdates["ownership_end_date"] = endDate
				}
			}
			if req.ContractNumber != nil {
				landlordUpdates["contract_number"] = *req.ContractNumber
			}
			if req.OwnerNotes != nil {
				landlordUpdates["notes"] = *req.OwnerNotes
			}
			if len(landlordUpdates) > 0 {
				if err := propertyDB.Model(&landlord).Updates(landlordUpdates).Error; err != nil {
					fmt.Printf("Failed to update landlord association: %v\n", err)
				}
			}
		}
	}

	// Handle SPA-related fields update (only for property_admin and landlord/spa)
	if (isAdmin || isLandlordOrSPA) && req.SpaUserID != nil {
		// Get or create SPA unit association
		var spaUnit property.SPAUnit
		err := propertyDB.Where("unit_id = ? AND spa_user_id = ?", uint(id), *req.SpaUserID).First(&spaUnit).Error

		if err == gorm.ErrRecordNotFound {
			// Create new SPA unit association
			now := time.Now()
			spaUnit = property.SPAUnit{
				SpaUserID:              *req.SpaUserID,
				UnitID:                 uint(id),
				Scope:                  "full",
				IsActive:               true,
				AuthorizationStartDate: &now,
			}
			if req.AuthorizationStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.AuthorizationStartDate); err == nil {
					spaUnit.AuthorizationStartDate = &startDate
				}
			}
			if req.AuthorizationEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.AuthorizationEndDate); err == nil {
					spaUnit.AuthorizationEndDate = &endDate
				}
			}
			if req.Scope != nil {
				spaUnit.Scope = *req.Scope
			}
			if req.SpaNotes != nil {
				spaUnit.Notes = req.SpaNotes
			}
			if req.IsActive != nil {
				spaUnit.IsActive = *req.IsActive
			}
			if err := propertyDB.Create(&spaUnit).Error; err != nil {
				fmt.Printf("Failed to create SPA unit association: %v\n", err)
			}
		} else if err == nil {
			// Update existing SPA unit association
			spaUnitUpdates := make(map[string]interface{})
			if req.AuthorizationStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.AuthorizationStartDate); err == nil {
					spaUnitUpdates["authorization_start_date"] = startDate
				}
			}
			if req.AuthorizationEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.AuthorizationEndDate); err == nil {
					spaUnitUpdates["authorization_end_date"] = endDate
				}
			}
			if req.Scope != nil {
				spaUnitUpdates["scope"] = *req.Scope
			}
			if req.SpaNotes != nil {
				spaUnitUpdates["notes"] = *req.SpaNotes
			}
			if req.IsActive != nil {
				spaUnitUpdates["is_active"] = *req.IsActive
			}
			if len(spaUnitUpdates) > 0 {
				if err := propertyDB.Model(&spaUnit).Updates(spaUnitUpdates).Error; err != nil {
					fmt.Printf("Failed to update SPA unit association: %v\n", err)
				}
			}
		}
	}

	// Handle tenant-related fields update (only for property_staff and landlord/spa)
	if (isStaff || isLandlordOrSPA) && len(req.Tenants) > 0 {
		fmt.Printf("DEBUG: Processing %d tenants\n", len(req.Tenants))
		// Handle batch tenant operations (create or update)
		for _, tenantReq := range req.Tenants {
			if tenantReq.TenantRecordID != nil && *tenantReq.TenantRecordID > 0 {
				// Update existing tenant association
				var tenant property.Tenant
				err := propertyDB.Where("id = ?", *tenantReq.TenantRecordID).First(&tenant).Error

				if err == nil {
					tenantUpdates := make(map[string]interface{})

					if tenantReq.LeaseStartDate != "" {
						if startDate, err := time.Parse("2006-01-02", tenantReq.LeaseStartDate); err == nil {
							tenantUpdates["lease_start_date"] = startDate
						}
					}
					if tenantReq.LeaseEndDate != "" {
						if endDate, err := time.Parse("2006-01-02", tenantReq.LeaseEndDate); err == nil {
							tenantUpdates["lease_end_date"] = endDate
						}
					}
					if tenantReq.MonthlyRent != "" {
						tenantUpdates["monthly_rent"] = tenantReq.MonthlyRent
					}
					if tenantReq.DepositAmount != "" {
						tenantUpdates["deposit_amount"] = &tenantReq.DepositAmount
					}
					if tenantReq.ContractNumber != "" {
						tenantUpdates["contract_number"] = &tenantReq.ContractNumber
					}
					if tenantReq.Status != "" {
						tenantUpdates["status"] = tenantReq.Status
					}
					if tenantReq.Notes != "" {
						tenantUpdates["notes"] = &tenantReq.Notes
					}

					if len(tenantUpdates) > 0 {
						if err := propertyDB.Model(&tenant).Updates(tenantUpdates).Error; err != nil {
							fmt.Printf("Failed to update tenant association ID %d: %v\n", *tenantReq.TenantRecordID, err)
						}
					}
				}
			} else {
				// Create new tenant association for selected user
				now := time.Now()
				leaseStartDate := now
				leaseEndDate := now.AddDate(0, 1, 0) // Default 1 month

				if tenantReq.LeaseStartDate != "" {
					if startDate, err := time.Parse("2006-01-02", tenantReq.LeaseStartDate); err == nil {
						leaseStartDate = startDate
					}
				}
				if tenantReq.LeaseEndDate != "" {
					if endDate, err := time.Parse("2006-01-02", tenantReq.LeaseEndDate); err == nil {
						leaseEndDate = endDate
					}
				}

				tenant := property.Tenant{
					UserID:         tenantReq.ID, // User ID
					UnitID:         uint(id),
					LeaseStartDate: leaseStartDate,
					LeaseEndDate:   leaseEndDate,
					MonthlyRent:    tenantReq.MonthlyRent,
					Status:         property.TenantStatusActive,
				}

				if tenantReq.DepositAmount != "" {
					tenant.DepositAmount = &tenantReq.DepositAmount
				}
				if tenantReq.ContractNumber != "" {
					tenant.ContractNumber = &tenantReq.ContractNumber
				}
				if tenantReq.Status != "" {
					tenant.Status = tenantReq.Status
				}
				if tenantReq.Notes != "" {
					tenant.Notes = &tenantReq.Notes
				}

				if err := propertyDB.Create(&tenant).Error; err != nil {
					fmt.Printf("Failed to create tenant association for user ID %d: %v\n", tenantReq.ID, err)
				}
			}
		}
	}

	// Handle legacy single tenant update (for backward compatibility)
	if (isStaff || isLandlordOrSPA) && req.TenantUserID != nil {
		// Get or create tenant association
		var tenant property.Tenant
		err := propertyDB.Where("unit_id = ? AND user_id = ?", uint(id), *req.TenantUserID).First(&tenant).Error

		if err == gorm.ErrRecordNotFound {
			// Create new tenant association
			now := time.Now()
			leaseStartDate := now
			leaseEndDate := now.AddDate(1, 0, 0) // Default 1 year lease

			if req.LeaseStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.LeaseStartDate); err == nil {
					leaseStartDate = startDate
				}
			}
			if req.LeaseEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.LeaseEndDate); err == nil {
					leaseEndDate = endDate
				}
			}

			tenant = property.Tenant{
				UserID:         *req.TenantUserID,
				UnitID:         uint(id),
				LeaseStartDate: leaseStartDate,
				LeaseEndDate:   leaseEndDate,
				MonthlyRent:    "0.00",
				Status:         property.TenantStatusPending,
			}
			if req.TenantMonthlyRent != nil {
				tenant.MonthlyRent = *req.TenantMonthlyRent
			}
			if req.DepositAmount != nil {
				tenant.DepositAmount = req.DepositAmount
			}
			if req.TenantContractNumber != nil {
				tenant.ContractNumber = req.TenantContractNumber
			}
			if req.TenantStatus != nil {
				tenant.Status = *req.TenantStatus
			}
			if req.TenantNotes != nil {
				tenant.Notes = req.TenantNotes
			}
			if err := propertyDB.Create(&tenant).Error; err != nil {
				fmt.Printf("Failed to create tenant association: %v\n", err)
			}
		} else if err == nil {
			// Update existing tenant association
			tenantUpdates := make(map[string]interface{})
			if req.LeaseStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.LeaseStartDate); err == nil {
					tenantUpdates["lease_start_date"] = startDate
				}
			}
			if req.LeaseEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.LeaseEndDate); err == nil {
					tenantUpdates["lease_end_date"] = endDate
				}
			}
			if req.TenantMonthlyRent != nil {
				tenantUpdates["monthly_rent"] = *req.TenantMonthlyRent
			}
			if req.DepositAmount != nil {
				tenantUpdates["deposit_amount"] = *req.DepositAmount
			}
			if req.TenantContractNumber != nil {
				tenantUpdates["contract_number"] = *req.TenantContractNumber
			}
			if req.TenantStatus != nil {
				tenantUpdates["status"] = *req.TenantStatus
			}
			if req.TenantNotes != nil {
				tenantUpdates["notes"] = *req.TenantNotes
			}
			if len(tenantUpdates) > 0 {
				if err := propertyDB.Model(&tenant).Updates(tenantUpdates).Error; err != nil {
					fmt.Printf("Failed to update tenant association: %v\n", err)
				}
			}
		}
	}

	// Handle file uploads if multipart form (only for landlord/spa, not admin/staff)
	if isMultipart && propertyDB != nil && (isLandlordOrSPA || !isAdmin) {
		if err := h.saveUnitDocuments(c, propertyDB, uint(id), userID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save unit documents: %v\n", err)
		}
	}

	// Get updated unit
	updatedUnit, _ := unitService.GetUnit(uint(id))
	utils.SuccessResponse(c, "Unit updated successfully", updatedUnit)
}

// DeleteUnit deletes unit by ID
// @Summary Delete unit
// @Description Delete a unit
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param id path int true "Unit ID"
// @Success 200 {object} map[string]interface{}
// @Router /units/{id} [delete]
func (h *UnitHandler) DeleteUnit(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid unit ID", nil)
		return
	}

	err = unitService.DeleteUnit(uint(id))
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Unit deleted successfully", nil)
}

// ListUnits lists all units with pagination
// @Summary List units
// @Description Get list of units with optional filters
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param unit_number query string false "Filter by unit number"
// @Param unit_type query string false "Filter by unit type"
// @Param exclude_type query string false "Exclude unit type (e.g., parking)"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Router /units [get]
func (h *UnitHandler) ListUnits(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	unitNumber := c.Query("unit_number")
	unitType := c.Query("unit_type")
	excludeType := c.Query("exclude_type")
	status := c.Query("status")

	units, total, err := unitService.ListUnitsWithFilters(unitNumber, unitType, excludeType, status, page, pageSize)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponseWithPagination(c, units, total, page, pageSize, "Units retrieved successfully")
}

// ListApartments lists all apartments with pagination
// @Summary List apartments
// @Description Get list of apartments only
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Router /units/apartments [get]
func (h *UnitHandler) ListApartments(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	units, total, err := unitService.ListUnits(property.UnitTypeApartment, status, page, pageSize)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponseWithPagination(c, units, total, page, pageSize, "Apartments retrieved successfully")
}

// ListParkingSpaces lists all parking spaces with pagination
// @Summary List parking spaces
// @Description Get list of parking spaces only
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param slot_number query string false "Filter by slot number"
// @Param parking_type query string false "Filter by parking type"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Router /units/parking [get]
// @Deprecated Use /parking-slots endpoint instead
func (h *UnitHandler) ListParkingSpaces(c *gin.Context) {
	// Parking spaces are now managed in a separate parking_slots table
	// This endpoint is kept for backward compatibility but redirects to parking slots
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	slotNumber := c.Query("slot_number")
	parkingType := c.Query("parking_type")
	status := c.Query("status")

	var parkingSlots []property.ParkingSlot
	var total int64

	query := propertyDB.Model(&property.ParkingSlot{})
	if slotNumber != "" {
		query = query.Where("slot_number LIKE ?", "%"+slotNumber+"%")
	}
	if parkingType != "" {
		query = query.Where("parking_type = ?", parkingType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&parkingSlots).Error; err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	// Build response with photos and SPA info
	type ParkingSlotResponse struct {
		property.ParkingSlot
		Photos     []map[string]interface{} `json:"photos"`
		SPANAME    *string                  `json:"spa_name,omitempty"`
		SPAContact *string                  `json:"spa_contact,omitempty"`
		SPAEmail   *string                  `json:"spa_email,omitempty"`
	}

	responses := make([]ParkingSlotResponse, len(parkingSlots))
	for i, slot := range parkingSlots {
		// Load documents (photos) for each parking slot
		var documents []property.Document
		propertyDB.Where("entity_type = ? AND entity_id = ? AND document_type = ? AND is_active = ?",
			property.DocEntityParking, slot.ID, property.DocTypePhoto, true).
			Find(&documents)

		var photos []map[string]interface{}
		for _, doc := range documents {
			// Convert relative path to URL
			// Path format: ./uploads/parking/{id}/filename
			// URL format: /api/v1/static/uploads/parking/{id}/filename
			urlPath := strings.TrimPrefix(doc.DocumentPath, "./uploads")
			if !strings.HasPrefix(urlPath, "/") {
				urlPath = "/" + urlPath
			}
			urlPath = "/api/v1/static" + urlPath

			var fileSize int64
			if doc.FileSize != nil {
				fileSize = *doc.FileSize
			}
			var mimeType string
			if doc.MimeType != nil {
				mimeType = *doc.MimeType
			}

			photos = append(photos, map[string]interface{}{
				"id":            doc.ID,
				"document_name": doc.DocumentName,
				"url":           urlPath,
				"file_size":     fileSize,
				"mime_type":     mimeType,
			})
		}

		// Find active SPA user for this parking slot
		var spaName, spaContact, spaEmail *string

		// Check if there's any active SPA parking slot association for this slot
		var spaParkingSlot property.SPAParkingSlot
		if err := propertyDB.Where("parking_slot_id = ? AND is_active = ?", slot.ID, true).First(&spaParkingSlot).Error; err == nil {
			// Get SPA user information from master database
			masterDB := database.GetMasterGormDB()
			var spaUser master.User
			if err := masterDB.Where("id = ?", spaParkingSlot.SpaUserID).First(&spaUser).Error; err == nil {
				spaName = &spaUser.FullName
				spaContact = spaUser.Phone
				spaEmail = spaUser.Email
			}
		}

		responses[i] = ParkingSlotResponse{
			ParkingSlot: slot,
			Photos:      photos,
			SPANAME:     spaName,
			SPAContact:  spaContact,
			SPAEmail:    spaEmail,
		}
	}

	utils.SuccessResponseWithPagination(c, responses, total, page, pageSize, "Parking slots retrieved successfully")
}

// GetUnitServiceFee calculates service fee for a unit from completed marketplace orders
// @Summary Get unit service fee
// @Description Calculate total service fee from completed marketplace orders for a unit
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param id path int true "Unit ID"
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Router /units/{id}/service-fee [get]
func (h *UnitHandler) GetUnitServiceFee(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid unit ID", nil)
		return
	}

	// Parse date range (default to current month)
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endDate := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, parsed.Location())
		}
	}

	// Calculate service fee
	billService := service.NewBillService(propertyDB)
	amount, err := billService.CalculateServiceFeeForUnit(uint(id), startDate, endDate)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to calculate service fee", err)
		return
	}

	utils.SuccessResponse(c, "Service fee calculated successfully", gin.H{
		"amount":     amount,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	})
}

// SearchUnits searches units by number or description
// @Summary Search units
// @Description Search units by unit number or description
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param q query string true "Search query"
// @Param unit_type query string false "Filter by unit type"
// @Param status query string false "Filter by status"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /units/search [get]
func (h *UnitHandler) SearchUnits(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	query := c.Query("q")
	if query == "" {
		utils.BadRequestResponse(c, "Search query is required", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	unitType := c.Query("unit_type")
	status := c.Query("status")

	units, total, err := unitService.SearchUnits(query, unitType, status, page, pageSize)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponseWithPagination(c, units, total, page, pageSize, "Search results")
}

// GetStatistics gets unit statistics
// @Summary Get unit statistics
// @Description Get statistics of units by type and status
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /units/statistics [get]
func (h *UnitHandler) GetStatistics(c *gin.Context) {
	unitService := h.getUnitService(c)
	if unitService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	stats, err := unitService.GetStatistics()
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Statistics retrieved successfully", stats)
}

// GetMyUnits returns units associated with the current user
// For tenant: returns units from tenants table
// For landlord: returns units from landlords table
// For spa: returns units from spa_units table
// @Summary Get my units
// @Tags Units
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /units/my [get]
func (h *UnitHandler) GetMyUnits(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var units []property.Unit

	switch userRole {
	case "tenant":
		// Get tenant's active units
		var tenants []property.Tenant
		propertyDB.Where("user_id = ? AND status = ?", userID, property.TenantStatusActive).Find(&tenants)
		var unitIDs []uint
		for _, t := range tenants {
			unitIDs = append(unitIDs, t.UnitID)
		}
		if len(unitIDs) > 0 {
			propertyDB.Where("id IN ?", unitIDs).Find(&units)
		}
	case "landlord":
		// Get landlord's units
		var landlords []property.Landlord
		propertyDB.Where("user_id = ?", userID).Find(&landlords)
		var unitIDs []uint
		for _, l := range landlords {
			unitIDs = append(unitIDs, l.UnitID)
		}
		if len(unitIDs) > 0 {
			propertyDB.Where("id IN ?", unitIDs).Find(&units)
		}
	case "spa":
		// Get SPA's managed units
		var spaUnits []property.SPAUnit
		propertyDB.Where("spa_user_id = ? AND is_active = ?", userID, true).Find(&spaUnits)
		var unitIDs []uint
		for _, s := range spaUnits {
			unitIDs = append(unitIDs, s.UnitID)
		}
		if len(unitIDs) > 0 {
			propertyDB.Where("id IN ?", unitIDs).Find(&units)
		}
	default:
		// For admin roles, return empty (they should use ListUnits)
		units = []property.Unit{}
	}

	// Build response with tenant information
	masterDB := database.GetMasterGormDB()
	responses := make([]map[string]interface{}, len(units))

	for i, unit := range units {
		response := map[string]interface{}{
			"id":           unit.ID,
			"unit_number":  unit.UnitNumber,
			"unit_type":    unit.UnitType,
			"building":     unit.Building,
			"floor":        unit.Floor,
			"area":         unit.Area,
			"bedrooms":     unit.Bedrooms,
			"bathrooms":    unit.Bathrooms,
			"monthly_rent": unit.MonthlyRent,
			"status":       unit.Status,
			"description":  unit.Description,
			"created_at":   unit.CreatedAt,
			"updated_at":   unit.UpdatedAt,
		}

		// Get current active tenant
		var tenant property.Tenant
		if err := propertyDB.Where("unit_id = ? AND status = ?", unit.ID, property.TenantStatusActive).First(&tenant).Error; err == nil {
			// Get tenant user info from master DB
			var tenantUser master.User
			if err := masterDB.First(&tenantUser, tenant.UserID).Error; err == nil {
				response["tenant_id"] = tenant.UserID
				response["tenant_name"] = tenantUser.FullName
				response["tenant_email"] = tenantUser.Email
			}
		}

		responses[i] = response
	}

	utils.SuccessResponse(c, "Units retrieved successfully", responses)
}

// GetParkingSlot gets parking slot by ID
// @Summary Get parking slot
// @Description Get parking slot details by ID
// @Tags Unit
// @Produce json
// @Security BearerAuth
// @Param id path int true "Parking Slot ID"
// @Success 200 {object} map[string]interface{}
// @Router /parking-slots/{id} [get]
func (h *UnitHandler) GetParkingSlot(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	// Get current user role for privacy check
	userRole := middleware.GetUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid parking slot ID", nil)
		return
	}

	var parkingSlot property.ParkingSlot
	if err := propertyDB.First(&parkingSlot, id).Error; err != nil {
		utils.NotFoundResponse(c, "Parking slot not found")
		return
	}


	// Load documents (photos) for parking slot - always load for all roles
	var photos []map[string]interface{}
	var documents []property.Document
	propertyDB.Where("entity_type = ? AND entity_id = ? AND document_type = ? AND is_active = ?",
		property.DocEntityParking, parkingSlot.ID, property.DocTypePhoto, true).
		Find(&documents)

	for _, doc := range documents {
		// Convert relative path to URL
		// DocumentPath format: ./uploads/parking/{id}/filename.png
		urlPath := strings.TrimPrefix(doc.DocumentPath, "./")
		urlPath = strings.ReplaceAll(urlPath, "\\", "/")
		// Remove "uploads/" prefix if present (since we'll add /api/v1/static/uploads/)
		urlPath = strings.TrimPrefix(urlPath, "uploads/")
		urlPath = strings.TrimPrefix(urlPath, "/uploads/")
		// Prepend API path
		urlPath = "/api/v1/static/uploads/" + urlPath

		var fileSize int64
		if doc.FileSize != nil {
			fileSize = *doc.FileSize
		}
		var mimeType string
		if doc.MimeType != nil {
			mimeType = *doc.MimeType
		}

		photos = append(photos, map[string]interface{}{
			"id":            doc.ID,
			"document_name": doc.DocumentName,
			"document_path": urlPath,
			"url":           urlPath,
			"file_size":     fileSize,
			"mime_type":     mimeType,
		})
	}

	// Get all landlords for this parking slot - always load for all roles
	masterDB := database.GetMasterDB()
	var landlordList []map[string]interface{}
	// Check if this is for editing (no privacy restrictions)
	forEdit := c.Query("for_edit") == "true"

	var landlordParkings []property.LandlordParkingSlot
	if err := propertyDB.Where("parking_slot_id = ?", parkingSlot.ID).Find(&landlordParkings).Error; err == nil {
		for _, landlordParking := range landlordParkings {
			if masterDB != nil && landlordParking.UserID > 0 {
				var owner master.User
				if err := masterDB.DB.First(&owner, landlordParking.UserID).Error; err == nil {
					// Get user roles for user type
					var userRoles []master.UserRole
					masterDB.DB.Where("user_id = ? AND status = ?", owner.ID, "active").Find(&userRoles)

					userType := "unknown"
					if len(userRoles) > 0 {
						userType = userRoles[0].Role // Use the first active role
					}

					var fullName string
					var email *string
					var phone *string

					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						// For editing, return real data without privacy restrictions
						fullName = owner.FullName
						email = owner.Email
						phone = owner.Phone
					} else {
						// Check privacy settings - respect user privacy preferences
						shouldShowFullName := owner.PublicFullName
						shouldShowEmail := owner.PublicEmail
						shouldShowPhone := owner.PublicPhone

						fullName = owner.FullName
						if !shouldShowFullName {
							fullName = "***"
						}

						if shouldShowEmail {
							email = owner.Email
						} else {
							emailStr := "***"
							email = &emailStr
						}

						if shouldShowPhone {
							phone = owner.Phone
						} else {
							phoneStr := "***"
							phone = &phoneStr
						}
					}

					ownerInfo := map[string]interface{}{
						"id":                   owner.ID,
						"full_name":            fullName,
						"email":                email,
						"phone":                phone,
						"user_type":            userType,
						"ownership_type":       landlordParking.OwnershipType,
						"ownership_percentage": landlordParking.OwnershipPercentage,
						"ownership_start_date": landlordParking.OwnershipStartDate,
						"ownership_end_date":   landlordParking.OwnershipEndDate,
						"contract_number":      landlordParking.ContractNumber,
						"notes":                landlordParking.Notes,
					}
					landlordList = append(landlordList, ownerInfo)
				}
			}
		}
	}

	// Get all SPAs for this parking slot - always load for all roles
	var spaList []map[string]interface{}
	var spaParkingSlots []property.SPAParkingSlot
	if err := propertyDB.Where("parking_slot_id = ? AND is_active = ?", parkingSlot.ID, true).Find(&spaParkingSlots).Error; err == nil {
		for _, spaParkingSlot := range spaParkingSlots {
			if masterDB != nil && spaParkingSlot.SpaUserID > 0 {
				var spaUser master.User
				if err := masterDB.DB.First(&spaUser, spaParkingSlot.SpaUserID).Error; err == nil {
					// Get user roles for user type
					var spaUserRoles []master.UserRole
					masterDB.DB.Where("user_id = ? AND status = ?", spaUser.ID, "active").Find(&spaUserRoles)

					spaUserType := "unknown"
					if len(spaUserRoles) > 0 {
						spaUserType = spaUserRoles[0].Role // Use the first active role
					}

					var fullName string
					var email *string
					var phone *string

					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						// For editing, return real data without privacy restrictions
						fullName = spaUser.FullName
						email = spaUser.Email
						phone = spaUser.Phone
					} else {
						// Check privacy settings - respect user privacy preferences
						shouldShowFullName := spaUser.PublicFullName
						shouldShowEmail := spaUser.PublicEmail
						shouldShowPhone := spaUser.PublicPhone

						fullName = spaUser.FullName
						if !shouldShowFullName {
							fullName = "***"
						}

						if shouldShowEmail {
							email = spaUser.Email
						} else {
							emailStr := "***"
							email = &emailStr
						}

						if shouldShowPhone {
							phone = spaUser.Phone
						} else {
							phoneStr := "***"
							phone = &phoneStr
						}
					}

					spaInfo := map[string]interface{}{
						"id":                       spaUser.ID,
						"full_name":                fullName,
						"email":                    email,
						"phone":                    phone,
						"user_type":                spaUserType,
						"authorization_start_date": spaParkingSlot.AuthorizationStartDate,
						"authorization_end_date":   spaParkingSlot.AuthorizationEndDate,
						"scope":                    spaParkingSlot.Scope,
						"notes":                    spaParkingSlot.Notes,
						"is_active":                spaParkingSlot.IsActive,
					}
					spaList = append(spaList, spaInfo)
				}
			}
		}
	}

	// Get current parking assignment (tenant) information - always load for all roles
	// For editing, load any assignment (not just active); for viewing, load only current active assignment
	var currentAssignment *map[string]interface{}
	var assignments []property.ParkingAssignment
	assignmentQuery := propertyDB.Where("parking_slot_id = ?", parkingSlot.ID)
	if !forEdit {
		assignmentQuery = assignmentQuery.Where("status = ?", property.AssignmentStatusActive)
	}
	if err := assignmentQuery.Order("created_at DESC").Find(&assignments).Error; err == nil {
		// Only process the first (most recent) assignment for viewing, or all for editing
		assignmentsToProcess := assignments
		if !forEdit && len(assignments) > 0 {
			assignmentsToProcess = assignments[:1] // Only the most recent active assignment
		}

		for _, assignment := range assignmentsToProcess {
			if masterDB != nil && assignment.UserID > 0 {
				var assignmentUser master.User
				if err := masterDB.DB.First(&assignmentUser, assignment.UserID).Error; err == nil {
					var fullName string
					var email *string
					var phone *string

					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						// For editing, return real data without privacy restrictions
						fullName = assignmentUser.FullName
						email = assignmentUser.Email
						phone = assignmentUser.Phone
					} else {
						// Check privacy settings - respect user privacy preferences
						shouldShowFullName := assignmentUser.PublicFullName
						shouldShowEmail := assignmentUser.PublicEmail
						shouldShowPhone := assignmentUser.PublicPhone

						fullName = assignmentUser.FullName
						if !shouldShowFullName {
							fullName = "***"
						}

						if shouldShowEmail {
							email = assignmentUser.Email
						} else {
							emailStr := "***"
							email = &emailStr
						}

						if shouldShowPhone {
							phone = assignmentUser.Phone
						} else {
							phoneStr := "***"
							phone = &phoneStr
						}
					}

					assignmentInfo := map[string]interface{}{
						"id":        assignmentUser.ID,
						"full_name": fullName,
						"email":     email,
						"phone":     phone,
					}

					// Always include assignment details for current assignment
					// Property admin and staff can see all information regardless of privacy settings
					if forEdit || userRole == "property_admin" || userRole == "property_staff" {
						assignmentInfo["vehicle_plate"] = assignment.VehiclePlate
						assignmentInfo["vehicle_brand"] = assignment.VehicleBrand
						assignmentInfo["vehicle_model"] = assignment.VehicleModel
						assignmentInfo["vehicle_color"] = assignment.VehicleColor
						assignmentInfo["vehicle_type"] = assignment.VehicleType
						assignmentInfo["assignment_type"] = assignment.AssignmentType
						assignmentInfo["start_date"] = assignment.StartDate
						assignmentInfo["end_date"] = assignment.EndDate
						assignmentInfo["monthly_fee"] = assignment.MonthlyFee
						assignmentInfo["status"] = assignment.Status
						assignmentInfo["notes"] = assignment.Notes
						assignmentInfo["assignment_id"] = assignment.ID // The assignment association ID
					}

					// Set current assignment (only one assignment returned for viewing)
					tempAssignment := assignmentInfo
					currentAssignment = &tempAssignment
				}
			}
		}
	}

	// Build response with all data - frontend will handle role-based display
	response := map[string]interface{}{
		"id":                   parkingSlot.ID,
		"slot_number":          parkingSlot.SlotNumber,
		"parking_type":         parkingSlot.ParkingType,
		"parking_level":        parkingSlot.ParkingLevel,
		"parking_zone":         parkingSlot.ParkingZone,
		"size":                 parkingSlot.Size,
		"monthly_rent":         parkingSlot.MonthlyRent,
		"status":               parkingSlot.Status,
		"description":          parkingSlot.Description,
		"vehicle_type_allowed": parkingSlot.VehicleTypeAllowed,
		"has_charger":          parkingSlot.HasCharger,
		"is_accessible":        parkingSlot.IsAccessible,
		"created_at":           parkingSlot.CreatedAt,
		"updated_at":           parkingSlot.UpdatedAt,
		"photos":               photos,
		"documents":            photos, // For backward compatibility
		"landlords":            landlordList,
		"spas":                 spaList,
		"current_assignment":   currentAssignment, // Current parking assignment (tenant) information
	}

	utils.SuccessResponse(c, "Parking slot retrieved successfully", response)
}

// GetMyParkingSlots gets parking slots owned by the current user (landlord)
// @Summary Get my parking slots
// @Description Get parking slots owned by the current landlord user
// @Tags Units
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /parking-slots/my [get]
func (h *UnitHandler) GetMyParkingSlots(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.BadRequestResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	var parkingSlots []property.ParkingSlot

	if userRole == "landlord" {
		// Get landlord's parking slots from landlord_parking_slots table
		var landlordParkings []property.LandlordParkingSlot
		propertyDB.Where("user_id = ?", userID).Find(&landlordParkings)

		var parkingSlotIDs []uint
		for _, lp := range landlordParkings {
			parkingSlotIDs = append(parkingSlotIDs, lp.ParkingSlotID)
		}

		if len(parkingSlotIDs) > 0 {
			propertyDB.Where("id IN ?", parkingSlotIDs).Find(&parkingSlots)
		}
	} else if userRole == "spa" {
		// Get SPA's managed parking slots from spa_parking_slots table
		var spaParkings []property.SPAParkingSlot
		propertyDB.Where("spa_user_id = ? AND is_active = ?", userID, true).Find(&spaParkings)

		var parkingSlotIDs []uint
		for _, sp := range spaParkings {
			parkingSlotIDs = append(parkingSlotIDs, sp.ParkingSlotID)
		}

		if len(parkingSlotIDs) > 0 {
			propertyDB.Where("id IN ?", parkingSlotIDs).Find(&parkingSlots)
		}
	} else {
		// For other roles, return empty
		parkingSlots = []property.ParkingSlot{}
	}

	// Build response with tenant information
	masterDB := database.GetMasterGormDB()
	responses := make([]map[string]interface{}, len(parkingSlots))

	for i, parking := range parkingSlots {
		response := map[string]interface{}{
			"id":                   parking.ID,
			"slot_number":          parking.SlotNumber,
			"parking_type":         parking.ParkingType,
			"parking_level":        parking.ParkingLevel,
			"parking_zone":         parking.ParkingZone,
			"vehicle_type_allowed": parking.VehicleTypeAllowed,
			"size":                 parking.Size,
			"has_charger":          parking.HasCharger,
			"is_accessible":        parking.IsAccessible,
			"monthly_rent":         parking.MonthlyRent,
			"status":               parking.Status,
			"description":          parking.Description,
			"created_at":           parking.CreatedAt,
			"updated_at":           parking.UpdatedAt,
		}

		// Get current active parking assignment (tenant)
		var assignment property.ParkingAssignment
		if err := propertyDB.Where("parking_slot_id = ? AND status = ?", parking.ID, property.AssignmentStatusActive).First(&assignment).Error; err == nil {
			// Get tenant user info from master DB
			var tenantUser master.User
			if err := masterDB.First(&tenantUser, assignment.UserID).Error; err == nil {
				response["tenant_id"] = assignment.UserID
				response["tenant_name"] = tenantUser.FullName
				response["tenant_email"] = tenantUser.Email
			}
		}

		responses[i] = response
	}

	utils.SuccessResponse(c, "Parking slots retrieved successfully", responses)
}

// CreateParkingSlotRequest represents create parking slot request
type CreateParkingSlotRequest struct {
	SlotNumber         string  `json:"slot_number" form:"slot_number" binding:"required"`
	ParkingType        string  `json:"parking_type" form:"parking_type" binding:"required,oneof=indoor outdoor covered uncovered"`
	ParkingLevel       string  `json:"parking_level" form:"parking_level"`
	ParkingZone        string  `json:"parking_zone" form:"parking_zone"`
	VehicleTypeAllowed string  `json:"vehicle_type_allowed" form:"vehicle_type_allowed" binding:"oneof=car motorcycle both"`
	Size               string  `json:"size" form:"size" binding:"oneof=standard compact large motorcycle"`
	MonthlyRent        float64 `json:"monthly_rent" form:"monthly_rent"`
	Description        string  `json:"description" form:"description"`
}

// CreateParkingSlot creates a new parking slot
// @Summary Create parking slot
// @Description Create a new parking slot
// @Tags Unit
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateParkingSlotRequest true "Create parking slot request"
// @Param ownership_certificate formData file false "Ownership certificate file"
// @Param parking_photos formData file false "Parking photos (max 3)"
// @Success 201 {object} map[string]interface{}
// @Router /parking-slots [post]
func (h *UnitHandler) CreateParkingSlot(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Check if multipart form
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")

	var req CreateParkingSlotRequest
	if isMultipart {
		// Parse form data
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	} else {
		// Parse JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	}

	// Set default values
	if req.VehicleTypeAllowed == "" {
		req.VehicleTypeAllowed = "car"
	}
	if req.Size == "" {
		req.Size = "standard"
	}

	// Create parking slot
	parkingSlot := &property.ParkingSlot{
		SlotNumber:         req.SlotNumber,
		OwnerID:            userID, // Set current user as owner if landlord
		ParkingType:        req.ParkingType,
		VehicleTypeAllowed: req.VehicleTypeAllowed,
		Status:             property.ParkingSlotStatusAvailable,
	}

	if req.ParkingLevel != "" {
		parkingSlot.ParkingLevel = &req.ParkingLevel
	}
	if req.ParkingZone != "" {
		parkingSlot.ParkingZone = &req.ParkingZone
	}
	if req.Size != "" {
		parkingSlot.Size = &req.Size
	}
	if req.MonthlyRent > 0 {
		rentStr := strconv.FormatFloat(req.MonthlyRent, 'f', 2, 64)
		parkingSlot.MonthlyRent = &rentStr
	}
	if req.Description != "" {
		parkingSlot.Description = &req.Description
	}

	// Create parking slot
	if err := propertyDB.Create(parkingSlot).Error; err != nil {
		utils.BadRequestResponse(c, "Failed to create parking slot: "+err.Error(), nil)
		return
	}

	// If current user is landlord, automatically create landlord-parking slot association
	if userRole == "landlord" {
		// Check if association already exists
		var existing property.LandlordParkingSlot
		err := propertyDB.Where("parking_slot_id = ? AND user_id = ?", parkingSlot.ID, userID).First(&existing).Error
		if err != nil {
			// Association doesn't exist, create it
			now := time.Now()
			ownershipPct := "100.00"
			landlordParking := property.LandlordParkingSlot{
				UserID:              userID,
				ParkingSlotID:       parkingSlot.ID,
				OwnershipType:       "full",
				OwnershipPercentage: &ownershipPct,
				OwnershipStartDate:  &now,
			}
			if err := propertyDB.Create(&landlordParking).Error; err != nil {
				// Log error but don't fail the request
				fmt.Printf("Failed to create landlord parking association: %v\n", err)
			}
		}
	}

	isAdmin := userRole == master.RolePropertyAdmin
	isLandlordOrSPA := userRole == master.RoleLandlord || userRole == master.RoleSPA
	// Handle file uploads if multipart form (only for landlord/spa, not admin/staff)
	if isMultipart && (isLandlordOrSPA || !isAdmin) {
		if err := h.saveParkingDocuments(c, propertyDB, parkingSlot.ID, userID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save parking documents: %v\n", err)
		}
	}

	utils.CreatedResponse(c, "Parking slot created successfully", parkingSlot)
}

// UpdateParkingSlotRequest represents update parking slot request
type UpdateParkingSlotRequest struct {
	ParkingType        *string  `json:"parking_type" form:"parking_type"`
	ParkingLevel       *string  `json:"parking_level" form:"parking_level"`
	ParkingZone        *string  `json:"parking_zone" form:"parking_zone"`
	VehicleTypeAllowed *string  `json:"vehicle_type_allowed" form:"vehicle_type_allowed"`
	MonthlyRent        *float64 `json:"monthly_rent" form:"monthly_rent"`
	Status             *string  `json:"status" form:"status"`
	Description        *string  `json:"description" form:"description"`
	// Owner-related fields (only editable by property_admin)
	OwnerID             *uint   `json:"owner_id" form:"owner_id"`
	OwnershipType       *string `json:"ownership_type" form:"ownership_type"`
	OwnershipPercentage *string `json:"ownership_percentage" form:"ownership_percentage"`
	OwnershipStartDate  *string `json:"ownership_start_date" form:"ownership_start_date"`
	OwnershipEndDate    *string `json:"ownership_end_date" form:"ownership_end_date"`
	PurchasePrice       *string `json:"purchase_price" form:"purchase_price"`
	ContractNumber      *string `json:"contract_number" form:"contract_number"`
	OwnerNotes          *string `json:"owner_notes" form:"owner_notes"`
	// Assignment-related fields (only editable by property_staff and landlord/spa)
	AssignmentUserID     *uint   `json:"assignment_user_id" form:"assignment_user_id"`
	VehiclePlate         *string `json:"vehicle_plate" form:"vehicle_plate"`
	VehicleBrand         *string `json:"vehicle_brand" form:"vehicle_brand"`
	VehicleModel         *string `json:"vehicle_model" form:"vehicle_model"`
	VehicleColor         *string `json:"vehicle_color" form:"vehicle_color"`
	VehicleType          *string `json:"vehicle_type" form:"vehicle_type"`
	AssignmentType       *string `json:"assignment_type" form:"assignment_type"`
	AssignmentStartDate  *string `json:"assignment_start_date" form:"assignment_start_date"`
	AssignmentEndDate    *string `json:"assignment_end_date" form:"assignment_end_date"`
	AssignmentMonthlyFee *string `json:"assignment_monthly_fee" form:"assignment_monthly_fee"`
	AssignmentStatus     *string `json:"assignment_status" form:"assignment_status"`
	AssignmentNotes      *string `json:"assignment_notes" form:"assignment_notes"`
}

// UpdateParkingSlot updates parking slot by ID
// @Summary Update parking slot
// @Description Update parking slot details
// @Tags Unit
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "Parking Slot ID"
// @Param request body UpdateParkingSlotRequest true "Update parking slot request"
// @Param parking_photos formData file false "Parking photos (max 3)"
// @Success 200 {object} map[string]interface{}
// @Router /parking-slots/{id} [put]
func (h *UnitHandler) UpdateParkingSlot(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid parking slot ID", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)
	isMultipart := c.ContentType() == "multipart/form-data" || strings.Contains(c.ContentType(), "multipart/form-data")

	var req UpdateParkingSlotRequest
	if isMultipart {
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
	}

	// Get existing parking slot
	var parkingSlot property.ParkingSlot
	if err := propertyDB.First(&parkingSlot, id).Error; err != nil {
		utils.NotFoundResponse(c, "Parking slot not found")
		return
	}

	// Check permissions based on role
	isAdmin := userRole == master.RolePropertyAdmin
	isStaff := userRole == master.RolePropertyStaff
	isLandlordOrSPA := userRole == master.RoleLandlord || userRole == master.RoleSPA
	// Admin: can only update owner and spa fields, reject basic info updates
	if isAdmin && (req.ParkingType != nil || req.ParkingLevel != nil || req.ParkingZone != nil ||
		req.VehicleTypeAllowed != nil || req.MonthlyRent != nil || req.Status != nil || req.Description != nil) {
		utils.ForbiddenResponse(c, "property_admin can only edit owner/SPA-related fields, not basic parking slot information")
		return
	}

	// Staff: can only update basic info and assignment fields, reject owner/spa updates
	if isStaff && (req.OwnerID != nil || req.OwnershipType != nil || req.OwnershipPercentage != nil ||
		req.OwnershipStartDate != nil || req.OwnershipEndDate != nil || req.PurchasePrice != nil ||
		req.ContractNumber != nil || req.OwnerNotes != nil) {
		utils.ForbiddenResponse(c, "property_staff can only edit basic parking slot information and assignment information, not owner-related fields")
		return
	}

	// Admin: reject assignment updates
	if isAdmin && (req.AssignmentUserID != nil || req.VehiclePlate != nil || req.VehicleBrand != nil ||
		req.VehicleModel != nil || req.VehicleColor != nil || req.VehicleType != nil ||
		req.AssignmentType != nil || req.AssignmentStartDate != nil || req.AssignmentEndDate != nil ||
		req.AssignmentMonthlyFee != nil || req.AssignmentStatus != nil || req.AssignmentNotes != nil) {
		utils.ForbiddenResponse(c, "property_admin can only edit owner/SPA-related fields, not assignment information")
		return
	}

	// Landlord/SPA: can update all fields (no restrictions needed here, but will check in handler)

	// Update fields (editable by staff and landlord/spa, not admin)
	updates := make(map[string]interface{})

	if !isAdmin || isLandlordOrSPA {
		if req.ParkingType != nil {
			updates["parking_type"] = *req.ParkingType
		}
		if req.ParkingLevel != nil {
			updates["parking_level"] = *req.ParkingLevel
		}
		if req.ParkingZone != nil {
			updates["parking_zone"] = *req.ParkingZone
		}
		if req.VehicleTypeAllowed != nil {
			updates["vehicle_type_allowed"] = *req.VehicleTypeAllowed
		}
		if req.MonthlyRent != nil {
			rentStr := strconv.FormatFloat(*req.MonthlyRent, 'f', 2, 64)
			updates["monthly_rent"] = rentStr
		}
		if req.Status != nil {
			updates["status"] = *req.Status
		}
		if req.Description != nil {
			updates["description"] = *req.Description
		}
	}

	// Update parking slot only if there are updates to apply
	if len(updates) > 0 {
		if err := propertyDB.Model(&parkingSlot).Updates(updates).Error; err != nil {
			utils.BadRequestResponse(c, "Failed to update parking slot: "+err.Error(), nil)
			return
		}
	}

	// Handle owner-related fields update (only for property_admin and landlord/spa)
	if (isAdmin || isLandlordOrSPA) && req.OwnerID != nil {
		// Get or create landlord parking slot association
		var landlordParking property.LandlordParkingSlot
		err := propertyDB.Where("parking_slot_id = ? AND user_id = ?", uint(id), *req.OwnerID).First(&landlordParking).Error

		if err == gorm.ErrRecordNotFound {
			// Create new landlord parking association
			now := time.Now()
			landlordParking = property.LandlordParkingSlot{
				UserID:             *req.OwnerID,
				ParkingSlotID:      uint(id),
				OwnershipType:      "full",
				OwnershipStartDate: &now,
			}
			if req.OwnershipType != nil {
				landlordParking.OwnershipType = *req.OwnershipType
			}
			if req.OwnershipPercentage != nil {
				landlordParking.OwnershipPercentage = req.OwnershipPercentage
			}
			if req.OwnershipStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.OwnershipStartDate); err == nil {
					landlordParking.OwnershipStartDate = &startDate
				}
			}
			if req.OwnershipEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.OwnershipEndDate); err == nil {
					landlordParking.OwnershipEndDate = &endDate
				}
			}
			if req.PurchasePrice != nil {
				landlordParking.PurchasePrice = req.PurchasePrice
			}
			if req.ContractNumber != nil {
				landlordParking.ContractNumber = req.ContractNumber
			}
			if req.OwnerNotes != nil {
				landlordParking.Notes = req.OwnerNotes
			}
			if err := propertyDB.Create(&landlordParking).Error; err != nil {
				fmt.Printf("Failed to create landlord parking association: %v\n", err)
			}
		} else if err == nil {
			// Update existing landlord parking association
			landlordParkingUpdates := make(map[string]interface{})
			if req.OwnershipType != nil {
				landlordParkingUpdates["ownership_type"] = *req.OwnershipType
			}
			if req.OwnershipPercentage != nil {
				landlordParkingUpdates["ownership_percentage"] = *req.OwnershipPercentage
			}
			if req.OwnershipStartDate != nil {
				if startDate, err := time.Parse("2006-01-02", *req.OwnershipStartDate); err == nil {
					landlordParkingUpdates["ownership_start_date"] = startDate
				}
			}
			if req.OwnershipEndDate != nil {
				if endDate, err := time.Parse("2006-01-02", *req.OwnershipEndDate); err == nil {
					landlordParkingUpdates["ownership_end_date"] = endDate
				}
			}
			if req.PurchasePrice != nil {
				landlordParkingUpdates["purchase_price"] = *req.PurchasePrice
			}
			if req.ContractNumber != nil {
				landlordParkingUpdates["contract_number"] = *req.ContractNumber
			}
			if req.OwnerNotes != nil {
				landlordParkingUpdates["notes"] = *req.OwnerNotes
			}
			if len(landlordParkingUpdates) > 0 {
				if err := propertyDB.Model(&landlordParking).Updates(landlordParkingUpdates).Error; err != nil {
					fmt.Printf("Failed to update landlord parking association: %v\n", err)
				}
			}
		}
	}

	// Handle assignment-related fields update (only for property_staff and landlord/spa)
	if (isStaff || isLandlordOrSPA) && req.AssignmentUserID != nil {
		// Get or create parking assignment
		var assignment property.ParkingAssignment
		err := propertyDB.Where("parking_slot_id = ? AND user_id = ?", uint(id), *req.AssignmentUserID).First(&assignment).Error

		if err == gorm.ErrRecordNotFound {
			// Create new parking assignment
			now := time.Now()
			startDate := now
			endDate := now.AddDate(1, 0, 0) // Default 1 year assignment

			if req.AssignmentStartDate != nil {
				if start, err := time.Parse("2006-01-02", *req.AssignmentStartDate); err == nil {
					startDate = start
				}
			}
			if req.AssignmentEndDate != nil {
				if end, err := time.Parse("2006-01-02", *req.AssignmentEndDate); err == nil {
					endDate = end
				}
			}

			vehiclePlate := ""
			if req.VehiclePlate != nil {
				vehiclePlate = *req.VehiclePlate
			}

			assignment = property.ParkingAssignment{
				ParkingSlotID:  uint(id),
				UserID:         *req.AssignmentUserID,
				VehiclePlate:   vehiclePlate,
				VehicleType:    property.VehicleTypeCar,
				AssignmentType: property.AssignmentTypeTenant,
				StartDate:      startDate,
				EndDate:        &endDate,
				Status:         property.AssignmentStatusActive,
			}
			if req.VehicleBrand != nil {
				assignment.VehicleBrand = req.VehicleBrand
			}
			if req.VehicleModel != nil {
				assignment.VehicleModel = req.VehicleModel
			}
			if req.VehicleColor != nil {
				assignment.VehicleColor = req.VehicleColor
			}
			if req.VehicleType != nil {
				assignment.VehicleType = *req.VehicleType
			}
			if req.AssignmentType != nil {
				assignment.AssignmentType = *req.AssignmentType
			}
			if req.AssignmentMonthlyFee != nil {
				assignment.MonthlyFee = req.AssignmentMonthlyFee
			}
			if req.AssignmentStatus != nil {
				assignment.Status = *req.AssignmentStatus
			}
			if req.AssignmentNotes != nil {
				assignment.Notes = req.AssignmentNotes
			}
			if err := propertyDB.Create(&assignment).Error; err != nil {
				fmt.Printf("Failed to create parking assignment: %v\n", err)
			}
		} else if err == nil {
			// Update existing parking assignment
			assignmentUpdates := make(map[string]interface{})
			if req.VehiclePlate != nil {
				assignmentUpdates["vehicle_plate"] = *req.VehiclePlate
			}
			if req.VehicleBrand != nil {
				assignmentUpdates["vehicle_brand"] = *req.VehicleBrand
			}
			if req.VehicleModel != nil {
				assignmentUpdates["vehicle_model"] = *req.VehicleModel
			}
			if req.VehicleColor != nil {
				assignmentUpdates["vehicle_color"] = *req.VehicleColor
			}
			if req.VehicleType != nil {
				assignmentUpdates["vehicle_type"] = *req.VehicleType
			}
			if req.AssignmentType != nil {
				assignmentUpdates["assignment_type"] = *req.AssignmentType
			}
			if req.AssignmentStartDate != nil {
				if start, err := time.Parse("2006-01-02", *req.AssignmentStartDate); err == nil {
					assignmentUpdates["start_date"] = start
				}
			}
			if req.AssignmentEndDate != nil {
				if end, err := time.Parse("2006-01-02", *req.AssignmentEndDate); err == nil {
					assignmentUpdates["end_date"] = end
				}
			}
			if req.AssignmentMonthlyFee != nil {
				assignmentUpdates["monthly_fee"] = *req.AssignmentMonthlyFee
			}
			if req.AssignmentStatus != nil {
				assignmentUpdates["status"] = *req.AssignmentStatus
			}
			if req.AssignmentNotes != nil {
				assignmentUpdates["notes"] = *req.AssignmentNotes
			}
			if len(assignmentUpdates) > 0 {
				if err := propertyDB.Model(&assignment).Updates(assignmentUpdates).Error; err != nil {
					fmt.Printf("Failed to update parking assignment: %v\n", err)
				}
			}
		}
	}

	// Handle file uploads if multipart form
	if isMultipart && propertyDB != nil {
		if err := h.saveParkingDocuments(c, propertyDB, uint(id), userID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save parking documents: %v\n", err)
		}
	}

	utils.SuccessResponse(c, "Parking slot updated successfully", parkingSlot)
}

// saveUnitDocuments saves uploaded documents for a unit
func (h *UnitHandler) saveUnitDocuments(c *gin.Context, db *gorm.DB, unitID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/units/%d", unitID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	// Save property certificate if provided
	if certFile := form.File["property_certificate"]; len(certFile) > 0 {
		file := certFile[0]
		if err := h.saveDocument(db, file, uploadDir, property.DocEntityUnit, unitID, property.DocTypePropertyCertificate, userID); err != nil {
			fmt.Printf("Failed to save property certificate: %v\n", err)
		}
	}

	// Save unit photos (max 6)
	photoFiles := form.File["unit_photos"]
	if len(photoFiles) > 6 {
		photoFiles = photoFiles[:6] // Limit to 6 photos
	}
	for _, file := range photoFiles {
		if err := h.saveDocument(db, file, uploadDir, property.DocEntityUnit, unitID, property.DocTypePhoto, userID); err != nil {
			fmt.Printf("Failed to save unit photo: %v\n", err)
			continue
		}
	}

	return nil
}

// saveParkingDocuments saves uploaded documents for a parking slot
func (h *UnitHandler) saveParkingDocuments(c *gin.Context, db *gorm.DB, parkingSlotID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/parking/%d", parkingSlotID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	// Save ownership certificate if provided
	if certFile := form.File["ownership_certificate"]; len(certFile) > 0 {
		file := certFile[0]
		if err := h.saveDocument(db, file, uploadDir, property.DocEntityParking, parkingSlotID, property.DocTypeOwnershipCertificate, userID); err != nil {
			fmt.Printf("Failed to save ownership certificate: %v\n", err)
		}
	}

	// Save parking photos (max 3)
	photoFiles := form.File["parking_photos"]
	if len(photoFiles) > 3 {
		photoFiles = photoFiles[:3] // Limit to 3 photos
	}
	for _, file := range photoFiles {
		if err := h.saveDocument(db, file, uploadDir, property.DocEntityParking, parkingSlotID, property.DocTypePhoto, userID); err != nil {
			fmt.Printf("Failed to save parking photo: %v\n", err)
			continue
		}
	}

	return nil
}

// saveDocument saves a single document file
func (h *UnitHandler) saveDocument(db *gorm.DB, fileHeader *multipart.FileHeader, uploadDir string, entityType string, entityID uint, docType string, userID uint) error {
	// Validate file size (max 10MB for certificates, 2MB for photos)
	maxSize := int64(10 * 1024 * 1024) // 10MB
	if docType == property.DocTypePhoto {
		maxSize = 2 * 1024 * 1024 // 2MB for photos
	}

	if fileHeader.Size > maxSize {
		return fmt.Errorf("file size exceeds limit")
	}

	// Validate file type
	contentType := fileHeader.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"application/pdf": true,
	}
	if !allowedTypes[contentType] {
		return fmt.Errorf("file type not allowed: %s", contentType)
	}

	// Generate unique filename
	ext := filepath.Ext(fileHeader.Filename)
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitizeFilename(fileHeader.Filename), ext)
	filePath := filepath.Join(uploadDir, filename)

	// Save file
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	// Create document record
	fileSize := fileHeader.Size
	doc := property.Document{
		EntityType:   entityType,
		EntityID:     entityID,
		DocumentType: docType,
		DocumentName: fileHeader.Filename,
		DocumentPath: filePath,
		FileSize:     &fileSize,
		MimeType:     &contentType,
		UploadedBy:   userID,
		IsActive:     true,
	}

	return db.Create(&doc).Error
}

// sanitizeFilename removes special characters from filename
func sanitizeFilename(name string) string {
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else if r == ' ' {
			result += "_"
		}
	}
	return result
}
