package handler

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"homexai/internal/config"
	"homexai/internal/database"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// SuperAdminHandler handles super admin operations
type SuperAdminHandler struct {
	masterDB *gorm.DB
}

// NewSuperAdminHandler creates a new super admin handler
func NewSuperAdminHandler(masterDB *gorm.DB) *SuperAdminHandler {
	return &SuperAdminHandler{
		masterDB: masterDB,
	}
}

// CreatePropertyRequest represents the request body for creating a property
type CreatePropertyRequest struct {
	Name                      string `json:"name" form:"name" binding:"required,min=2,max=200"`
	Subdomain                 string `json:"subdomain" form:"subdomain" binding:"required,min=2,max=50"`
	Category                  string `json:"category" form:"category" binding:"required,oneof=commercial_office warehouse residential"`
	Province                  string `json:"province" form:"province" binding:"required"`
	City                      string `json:"city" form:"city"`
	District                  string `json:"district" form:"district"`
	AreaName                  string `json:"area_name" form:"area_name"`
	Address                   string `json:"address" form:"address" binding:"required"`
	Developer                 string `json:"developer" form:"developer" binding:"required,min=1,max=200"`
	HandoverDate              string `json:"handover_date" form:"handover_date" binding:"required"`
	PropertyManagementCompany string `json:"property_management_company" form:"property_management_company"`
	PostalCode                string `json:"postal_code" form:"postal_code"`
	ContactEmail              string `json:"contact_email" form:"contact_email" binding:"omitempty,email"`
	ContactPhone              string `json:"contact_phone" form:"contact_phone"`
	Website                   string `json:"website" form:"website"`
	Description               string `json:"description" form:"description"`
}

// UpdatePropertyRequest represents the request body for updating a property
type UpdatePropertyRequest struct {
	Name                      string `json:"name" form:"name" binding:"omitempty,min=2,max=200"`
	Category                  string `json:"category" form:"category" binding:"omitempty,oneof=commercial_office warehouse residential"`
	Province                  string `json:"province" form:"province"`
	City                      string `json:"city" form:"city"`
	District                  string `json:"district" form:"district"`
	AreaName                  string `json:"area_name" form:"area_name"`
	Address                   string `json:"address" form:"address"`
	Developer                 string `json:"developer" form:"developer" binding:"omitempty,min=1,max=200"`
	HandoverDate              string `json:"handover_date" form:"handover_date"`
	PropertyManagementCompany string `json:"property_management_company" form:"property_management_company"`
	PostalCode                string `json:"postal_code" form:"postal_code"`
	ContactEmail              string `json:"contact_email" form:"contact_email" binding:"omitempty,email"`
	ContactPhone              string `json:"contact_phone" form:"contact_phone"`
	Website                   string `json:"website" form:"website"`
	Description               string `json:"description" form:"description"`
}

// CreateProperty creates a new property
// @Summary Create a new property
// @Description Super Admin creates a new property with database
// @Tags SuperAdmin
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Param request body CreatePropertyRequest true "Property creation request"
// @Param logo formData file false "Property logo image"
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/super-admin/properties [post]
func (h *SuperAdminHandler) CreateProperty(c *gin.Context) {
	var req CreatePropertyRequest

	// Check content type - support both JSON and multipart/form-data
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle multipart form data
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	} else {
		// Handle JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	}

	// Validate subdomain format (alphanumeric and hyphens only, lowercase)
	subdomainRegex := regexp.MustCompile("^[a-z0-9-]+$")
	if !subdomainRegex.MatchString(req.Subdomain) {
		utils.Error(c, http.StatusBadRequest, "Subdomain can only contain lowercase letters, numbers, and hyphens")
		return
	}

	// Check if subdomain already exists
	var existingProperty master.Property
	if err := h.masterDB.Where("subdomain = ?", req.Subdomain).First(&existingProperty).Error; err == nil {
		utils.Error(c, http.StatusConflict, "Subdomain already exists")
		return
	}

	// Generate database name
	dbName := "homexai_property_" + req.Subdomain

	// Parse handover date
	handoverDate, err := time.Parse("2006-01-02", req.HandoverDate)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid handover date format. Use YYYY-MM-DD")
		return
	}

	// Create property record
	property := master.Property{
		Name:            req.Name,
		Subdomain:       req.Subdomain,
		DBName:          dbName,
		Status:          "active",
		Category:        req.Category,
		Province:        req.Province,
		Developer:       req.Developer,
		HandoverDate:    handoverDate,
		DefaultLanguage: "en",
	}

	// Set required pointer fields
	property.Address = &req.Address

	// Set optional pointer fields
	if req.City != "" {
		property.City = &req.City
	}
	if req.District != "" {
		property.District = &req.District
	}
	if req.AreaName != "" {
		property.AreaName = &req.AreaName
	}
	if req.PropertyManagementCompany != "" {
		property.PropertyManagementCompany = &req.PropertyManagementCompany
	}
	if req.PostalCode != "" {
		property.PostalCode = &req.PostalCode
	}
	if req.ContactEmail != "" {
		property.ContactEmail = &req.ContactEmail
	}
	if req.ContactPhone != "" {
		property.ContactPhone = &req.ContactPhone
	}
	if req.Website != "" {
		property.Website = &req.Website
	}
	if req.Description != "" {
		property.Description = &req.Description
	}

	// Start transaction
	tx := h.masterDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create property record
	if err := tx.Create(&property).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to create property: "+err.Error())
		return
	}

	// Handle logo upload if present (only for multipart/form-data)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := h.savePropertyLogo(c, tx, property.ID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save logo: %v\n", err)
		}
	}

	// Create database mapping
	dbMapping := master.PropertyDBMapping{
		PropertyID:          property.ID,
		Subdomain:           req.Subdomain,
		DBHost:              "localhost",
		DBPort:              3306,
		DBName:              dbName,
		DBUser:              "homexai",
		DBPasswordEncrypted: "", // Will be set by admin
		IsActive:            true,
		MaxConnections:      50,
	}

	if err := tx.Create(&dbMapping).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to create database mapping: "+err.Error())
		return
	}

	// Create Property Admin user
	adminEmail := req.Subdomain + ".admin@homex.ph"
	adminPassword := req.Subdomain + "Adm123!"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 10)
	if err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to generate password hash: "+err.Error())
		return
	}

	now := time.Now()
	adminUser := master.User{
		Email:         &adminEmail,
		FullName:      req.Name + " Admin",
		PasswordHash:  string(passwordHash),
		Status:        "active",
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := tx.Create(&adminUser).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to create admin user: "+err.Error())
		return
	}

	// Get current user ID (Super Admin who is creating this property)
	currentUserID, exists := c.Get("user_id")
	var assignedBy *uint
	if exists {
		if uid, ok := currentUserID.(uint); ok {
			assignedBy = &uid
		}
	}

	// Assign property_admin role to the new user
	userRole := master.UserRole{
		UserID:     adminUser.ID,
		PropertyID: property.ID,
		Role:       master.RolePropertyAdmin,
		Status:     "active",
		AssignedBy: assignedBy,
	}

	if err := tx.Create(&userRole).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to assign admin role: "+err.Error())
		return
	}

	// Commit transaction first (before creating database)
	if err := tx.Commit().Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to commit transaction: "+err.Error())
		return
	}

	// Create property database and run migrations
	if err := h.createPropertyDatabase(dbName); err != nil {
		// Log error but don't fail the request (database can be created manually later)
		fmt.Printf("⚠️  Warning: Failed to create property database %s: %v\n", dbName, err)
		utils.SuccessWithCode(c, http.StatusCreated, "Property created successfully, but database creation failed", gin.H{
			"property": property,
			"admin": gin.H{
				"email":    adminEmail,
				"password": adminPassword,
				"note":     "Please save this password, it will only be shown once",
			},
			"warning": fmt.Sprintf("Database creation failed: %v. Please create the database manually: %s", err, dbName),
		})
		return
	}

	utils.SuccessWithCode(c, http.StatusCreated, "Property created successfully", gin.H{
		"property": property,
		"admin": gin.H{
			"email":    adminEmail,
			"password": adminPassword,
			"note":     "Please save this password, it will only be shown once",
		},
		"database": gin.H{
			"name":    dbName,
			"status":  "created",
			"message": "Property database created and migrated successfully",
		},
	})
}

// createPropertyDatabase creates a new property database and runs migrations
func (h *SuperAdminHandler) createPropertyDatabase(dbName string) error {
	// Step 1: Create database using raw SQL connection (without specifying database name)
	dsnWithoutDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
		config.Yaml.Database.User,
		config.Yaml.GetDatabasePassword(),
		config.Yaml.Database.Host,
		config.Yaml.Database.Port,
	)

	// Connect to MySQL server (without database)
	db, err := sql.Open("mysql", dsnWithoutDB)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL server: %w", err)
	}
	defer db.Close()

	// Create database (dbName is safe as it's generated from validated subdomain)
	// Escape database name to prevent SQL injection (though subdomain is already validated)
	escapedDBName := "`" + strings.ReplaceAll(dbName, "`", "``") + "`"
	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", escapedDBName)
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	fmt.Printf("✓ Database '%s' created successfully\n", dbName)

	// Step 2: Connect to the new database and run migrations
	dsn := config.Yaml.GetPropertyDSN(dbName)
	propDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to property database: %w", err)
	}

	// Run AutoMigrate for all property models
	if err := propDB.AutoMigrate(
		&property.Unit{},
		&property.ParkingSlot{},
		&property.Facility{},
		&property.FacilityReservation{},
		&property.Landlord{},
		&property.LandlordParkingSlot{},
		&property.SPAUnit{},
		&property.SPAParkingSlot{},
		&property.Document{},
		&property.Tenant{},
		&property.ParkingAssignment{},
		&property.Bill{},
		&property.Payment{},
		&property.Request{},
		&property.RequestTrace{},
		&property.WorkPermit{},
		&property.GatePass{},
		&property.VehicleSticker{},
		&property.PetRegistration{},
		&property.MoveIn{},
		&property.MoveOut{},
		&property.HouseholdStaffRegistration{},
		&property.Complaint{},
		&property.Visitor{},
		&property.Announcement{},
		&property.AuditLog{},
		&property.ForumPost{},
		&property.ForumReply{},
		&property.ForumViewLog{},
		&property.ForumVote{},
		&property.PropertySettings{}, // 物业设置表
		&property.Notification{},     // 用户通知表
	); err != nil {
		return fmt.Errorf("failed to migrate property database: %w", err)
	}

	fmt.Printf("✓ Database '%s' migrated successfully\n", dbName)

	// Initialize property database pool to register the new database
	database.InitPropertyDBPool()
	if _, err := database.GetPropertyDB(dbName); err != nil {
		return fmt.Errorf("failed to register database in pool: %w", err)
	}

	return nil
}

// ListProperties returns all properties
// @Summary List all properties
// @Description Super Admin gets list of all properties
// @Tags SuperAdmin
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/super-admin/properties [get]
func (h *SuperAdminHandler) ListProperties(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	var properties []master.Property
	var total int64

	// Get total count
	h.masterDB.Model(&master.Property{}).Count(&total)

	// Get properties with pagination
	offset := (page - 1) * perPage
	if err := h.masterDB.Offset(offset).Limit(perPage).Order("id DESC").Find(&properties).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to get properties: "+err.Error())
		return
	}

	utils.SuccessWithPagination(c, "Properties retrieved successfully", properties, total, page, perPage)
}

// GetProperty returns a specific property by ID
// @Summary Get property by ID
// @Description Super Admin gets property details by ID
// @Tags SuperAdmin
// @Produce json
// @Param id path int true "Property ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /api/v1/super-admin/properties/{id} [get]
func (h *SuperAdminHandler) GetProperty(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid property ID")
		return
	}

	var property master.Property
	if err := h.masterDB.First(&property, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "Property not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to get property: "+err.Error())
		return
	}

	// Get database mapping
	var dbMapping master.PropertyDBMapping
	if err := h.masterDB.Where("property_id = ?", property.ID).First(&dbMapping).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Database mapping not found, return nil
			utils.Success(c, "Property retrieved successfully", gin.H{
				"property":   property,
				"db_mapping": nil,
			})
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to get database mapping: "+err.Error())
		return
	}

	utils.Success(c, "Property retrieved successfully", gin.H{
		"property":   property,
		"db_mapping": dbMapping,
	})
}

// UpdateProperty updates a property
// @Summary Update property
// @Description Super Admin updates property details
// @Tags SuperAdmin
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Property ID"
// @Param request body UpdatePropertyRequest true "Property update request"
// @Param logo formData file false "Property logo image"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/super-admin/properties/{id} [put]
func (h *SuperAdminHandler) UpdateProperty(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid property ID")
		return
	}

	var property master.Property
	if err := h.masterDB.First(&property, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "Property not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to get property: "+err.Error())
		return
	}

	var req UpdatePropertyRequest
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle multipart form data
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	} else {
		// Handle JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	}

	// Build updates map - only include fields that are provided
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Province != "" {
		updates["province"] = req.Province
	}
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.District != "" {
		updates["district"] = req.District
	}
	if req.AreaName != "" {
		updates["area_name"] = req.AreaName
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.Developer != "" {
		updates["developer"] = req.Developer
	}
	if req.HandoverDate != "" {
		// Parse handover date
		handoverDate, err := time.Parse("2006-01-02", req.HandoverDate)
		if err != nil {
			utils.Error(c, http.StatusBadRequest, "Invalid handover date format. Use YYYY-MM-DD")
			return
		}
		updates["handover_date"] = handoverDate
	}
	if req.PropertyManagementCompany != "" {
		updates["property_management_company"] = req.PropertyManagementCompany
	}
	if req.PostalCode != "" {
		updates["postal_code"] = req.PostalCode
	}
	if req.ContactEmail != "" {
		updates["contact_email"] = req.ContactEmail
	}
	if req.ContactPhone != "" {
		updates["contact_phone"] = req.ContactPhone
	}
	if req.Website != "" {
		updates["website"] = req.Website
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	// Only update if there are fields to update
	if len(updates) > 0 {
		if err := h.masterDB.Model(&property).Updates(updates).Error; err != nil {
			utils.Error(c, http.StatusInternalServerError, "Failed to update property: "+err.Error())
			return
		}
	}

	// Handle logo upload if present (for multipart/form-data)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := h.savePropertyLogo(c, h.masterDB, property.ID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save logo: %v\n", err)
		}
	}

	// Reload property
	h.masterDB.First(&property, id)

	utils.Success(c, "Property updated successfully", property)
}

// savePropertyLogo saves uploaded logo for a property (max 1 logo)
func (h *SuperAdminHandler) savePropertyLogo(c *gin.Context, db *gorm.DB, propertyID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	files := form.File["logo"]
	if len(files) == 0 {
		return nil
	}

	// Only allow 1 logo
	if len(files) > 1 {
		return fmt.Errorf("only 1 logo is allowed")
	}

	file := files[0]

	// Validate file size (max 5MB)
	if file.Size > 5*1024*1024 {
		return fmt.Errorf("file too large (max 5MB)")
	}

	// Validate file type (only images)
	contentType := file.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg":    true,
		"image/png":     true,
		"image/gif":     true,
		"image/webp":    true,
		"image/svg+xml": true,
	}
	if !allowedTypes[contentType] {
		return fmt.Errorf("only image files are allowed")
	}

	// Delete existing logo if exists
	var existingProperty master.Property
	if err := db.First(&existingProperty, propertyID).Error; err == nil && existingProperty.LogoURL != nil {
		// Delete old logo file if exists
		oldPath := *existingProperty.LogoURL
		if strings.HasPrefix(oldPath, "./uploads/") {
			if _, err := os.Stat(oldPath); err == nil {
				os.Remove(oldPath)
			}
		}
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/properties/%d", propertyID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("logo_%d%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(uploadDir, filename)

	// Save file
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	// Update property with logo URL
	logoURL := filePath
	if err := db.Model(&master.Property{}).Where("id = ?", propertyID).Update("logo_url", logoURL).Error; err != nil {
		// Try to delete the file if database update fails
		os.Remove(filePath)
		return fmt.Errorf("failed to update property logo URL: %v", err)
	}

	return nil
}

// CreatePropertyByLandlord creates a new property by landlord
// @Summary Create a new property (Landlord)
// @Description Landlord creates a new property with database
// @Tags Landlord
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Param request body CreatePropertyRequest true "Property creation request"
// @Param logo formData file false "Property logo image"
// @Success 201 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 409 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/landlord/properties [post]
func (h *SuperAdminHandler) CreatePropertyByLandlord(c *gin.Context) {
	// Get current user ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	landlordUserID := currentUserID.(uint)

	// Get current user to verify it's a landlord
	var user master.User
	if err := h.masterDB.First(&user, landlordUserID).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "User not found")
		return
	}

	var req CreatePropertyRequest

	// Check content type - support both JSON and multipart/form-data
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle multipart form data
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	} else {
		// Handle JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	}

	// Validate subdomain format (alphanumeric and hyphens only, lowercase)
	subdomainRegex := regexp.MustCompile("^[a-z0-9-]+$")
	if !subdomainRegex.MatchString(req.Subdomain) {
		utils.Error(c, http.StatusBadRequest, "Subdomain can only contain lowercase letters, numbers, and hyphens")
		return
	}

	// Check if subdomain already exists
	var existingProperty master.Property
	if err := h.masterDB.Where("subdomain = ?", req.Subdomain).First(&existingProperty).Error; err == nil {
		utils.Error(c, http.StatusConflict, "Subdomain already exists")
		return
	}

	// Generate database name
	dbName := "homexai_property_" + req.Subdomain

	// Create property record
	property := master.Property{
		Name:            req.Name,
		Subdomain:       req.Subdomain,
		DBName:          dbName,
		Status:          "active",
		DefaultLanguage: "en",
	}

	// Set optional pointer fields
	if req.Address != "" {
		property.Address = &req.Address
	}
	if req.PostalCode != "" {
		property.PostalCode = &req.PostalCode
	}
	if req.ContactEmail != "" {
		property.ContactEmail = &req.ContactEmail
	}
	if req.ContactPhone != "" {
		property.ContactPhone = &req.ContactPhone
	}
	if req.Website != "" {
		property.Website = &req.Website
	}
	if req.Description != "" {
		property.Description = &req.Description
	}

	// Start transaction
	tx := h.masterDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create property record
	if err := tx.Create(&property).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to create property: "+err.Error())
		return
	}

	// Handle logo upload if present (only for multipart/form-data)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := h.savePropertyLogo(c, tx, property.ID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save logo: %v\n", err)
		}
	}

	// Create database mapping
	dbMapping := master.PropertyDBMapping{
		PropertyID:          property.ID,
		Subdomain:           req.Subdomain,
		DBHost:              "localhost",
		DBPort:              3306,
		DBName:              dbName,
		DBUser:              "homexai",
		DBPasswordEncrypted: "", // Will be set by admin
		IsActive:            true,
		MaxConnections:      50,
	}

	if err := tx.Create(&dbMapping).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to create database mapping: "+err.Error())
		return
	}

	// Assign property_admin role to the landlord user (instead of creating a new admin)
	assignedBy := &landlordUserID
	userRole := master.UserRole{
		UserID:     landlordUserID,
		PropertyID: property.ID,
		Role:       "property_admin",
		Status:     "active",
		AssignedBy: assignedBy,
	}

	if err := tx.Create(&userRole).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to assign admin role: "+err.Error())
		return
	}

	// Also keep the landlord role for this property
	landlordRole := master.UserRole{
		UserID:     landlordUserID,
		PropertyID: property.ID,
		Role:       "landlord",
		Status:     "active",
		AssignedBy: assignedBy,
	}

	if err := tx.Create(&landlordRole).Error; err != nil {
		tx.Rollback()
		utils.Error(c, http.StatusInternalServerError, "Failed to assign landlord role: "+err.Error())
		return
	}

	// Commit transaction first (before creating database)
	if err := tx.Commit().Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to commit transaction: "+err.Error())
		return
	}

	// Create property database and run migrations
	if err := h.createPropertyDatabase(dbName); err != nil {
		// Log error but don't fail the request (database can be created manually later)
		fmt.Printf("⚠️  Warning: Failed to create property database %s: %v\n", dbName, err)
		utils.SuccessWithCode(c, http.StatusCreated, "Property created successfully, but database creation failed", gin.H{
			"property": property,
			"warning":  fmt.Sprintf("Database creation failed: %v. Please create the database manually: %s", err, dbName),
		})
		return
	}

	utils.SuccessWithCode(c, http.StatusCreated, "Property created successfully", gin.H{
		"property": property,
		"database": gin.H{
			"name":    dbName,
			"status":  "created",
			"message": "Property database created and migrated successfully",
		},
	})
}

// ListPropertiesByLandlord lists all properties owned by the landlord
// @Summary List properties (Landlord)
// @Description Get all properties where the current user is a landlord
// @Tags Landlord
// @Produce json
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/landlord/properties [get]
func (h *SuperAdminHandler) ListPropertiesByLandlord(c *gin.Context) {
	// Get current user ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	userID := currentUserID.(uint)

	// Get all properties where user has landlord or property_admin role
	var userRoles []master.UserRole
	if err := h.masterDB.Where("user_id = ? AND role IN (?, ?)", userID, "landlord", "property_admin").Find(&userRoles).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to get user properties: "+err.Error())
		return
	}

	if len(userRoles) == 0 {
		utils.Success(c, "No properties found", []interface{}{})
		return
	}

	// Get property IDs
	propertyIDs := make([]uint, 0, len(userRoles))
	for _, role := range userRoles {
		propertyIDs = append(propertyIDs, role.PropertyID)
	}

	// Get properties
	var properties []master.Property
	if err := h.masterDB.Where("id IN ?", propertyIDs).Find(&properties).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to get properties: "+err.Error())
		return
	}

	utils.Success(c, "Properties retrieved successfully", properties)
}

// GetPropertyByLandlord gets a property by ID (only if landlord owns it)
// @Summary Get property (Landlord)
// @Description Get property details by ID (only if user is landlord/admin of this property)
// @Tags Landlord
// @Produce json
// @Param id path int true "Property ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /api/v1/landlord/properties/{id} [get]
func (h *SuperAdminHandler) GetPropertyByLandlord(c *gin.Context) {
	// Get current user ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	userID := currentUserID.(uint)

	// Get property ID
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid property ID")
		return
	}

	// Check if user has access to this property
	var userRole master.UserRole
	if err := h.masterDB.Where("user_id = ? AND property_id = ? AND role IN (?, ?)", userID, propertyID, "landlord", "property_admin").First(&userRole).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusForbidden, "You don't have access to this property")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to check property access: "+err.Error())
		return
	}

	// Get property
	var property master.Property
	if err := h.masterDB.First(&property, propertyID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "Property not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to get property: "+err.Error())
		return
	}

	utils.Success(c, "Property retrieved successfully", property)
}

// UpdatePropertyByLandlord updates a property (only if landlord owns it)
// @Summary Update property (Landlord)
// @Description Update property details (only if user is landlord/admin of this property)
// @Tags Landlord
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Property ID"
// @Param request body UpdatePropertyRequest true "Property update request"
// @Param logo formData file false "Property logo image"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 403 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/landlord/properties/{id} [put]
func (h *SuperAdminHandler) UpdatePropertyByLandlord(c *gin.Context) {
	// Get current user ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	userID := currentUserID.(uint)

	// Get property ID
	propertyID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid property ID")
		return
	}

	// Check if user has access to this property
	var userRole master.UserRole
	if err := h.masterDB.Where("user_id = ? AND property_id = ? AND role IN (?, ?)", userID, propertyID, "landlord", "property_admin").First(&userRole).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusForbidden, "You don't have access to this property")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to check property access: "+err.Error())
		return
	}

	// Get property
	var property master.Property
	if err := h.masterDB.First(&property, propertyID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "Property not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to get property: "+err.Error())
		return
	}

	var req UpdatePropertyRequest
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle multipart form data
		if err := c.ShouldBind(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	} else {
		// Handle JSON
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationError(c, err)
			return
		}
	}

	// Build updates map - only include fields that are provided
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.PostalCode != "" {
		updates["postal_code"] = req.PostalCode
	}
	if req.ContactEmail != "" {
		updates["contact_email"] = req.ContactEmail
	}
	if req.ContactPhone != "" {
		updates["contact_phone"] = req.ContactPhone
	}
	if req.Website != "" {
		updates["website"] = req.Website
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	// Only update if there are fields to update
	if len(updates) > 0 {
		if err := h.masterDB.Model(&property).Updates(updates).Error; err != nil {
			utils.Error(c, http.StatusInternalServerError, "Failed to update property: "+err.Error())
			return
		}
	}

	// Handle logo upload if present (for multipart/form-data)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := h.savePropertyLogo(c, h.masterDB, property.ID); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to save logo: %v\n", err)
		}
	}

	// Reload property
	h.masterDB.First(&property, propertyID)

	utils.Success(c, "Property updated successfully", property)
}
