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
	"homexai/internal/models/property"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateVehicleStickerRequest represents vehicle sticker creation payload
type CreateVehicleStickerRequest struct {
	TenantID      *uint  `json:"tenant_id" form:"tenant_id"`
	UnitID        uint   `json:"unit_id" form:"unit_id" binding:"required"`
	ParkingSlotID *uint  `json:"parking_slot_id" form:"parking_slot_id"`
	VehicleBrand  string `json:"vehicle_brand" form:"vehicle_brand" binding:"required"`
	VehicleModel  string `json:"vehicle_model" form:"vehicle_model" binding:"required"`
	VehicleYear   string `json:"vehicle_year" form:"vehicle_year" binding:"required"`
	VehicleColor  string `json:"vehicle_color" form:"vehicle_color" binding:"required"`
	PlateNumber   string `json:"plate_number" form:"plate_number" binding:"required"`
	IsDraft       bool   `json:"is_draft" form:"is_draft"`
}

// CreateVehicleSticker creates a vehicle sticker application
// @Summary Create vehicle sticker
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param request formData CreateVehicleStickerRequest true "Vehicle sticker data"
// @Param or_image formData file false "OR (Official Receipt) image"
// @Param cr_image formData file false "CR (Certificate of Registration) image"
// @Success 201 {object} map[string]interface{}
// @Router /requests/vehicle-sticker [post]
func (h *RequestHandler) CreateVehicleSticker(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create vehicle sticker applications
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create vehicle sticker applications"})
		return
	}

	var req CreateVehicleStickerRequest
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

	// Validate parking slot if provided
	if req.ParkingSlotID != nil {
		var parkingSlot property.ParkingSlot
		if err := propertyDB.First(&parkingSlot, *req.ParkingSlotID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parking slot ID"})
			return
		}
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
	status := property.VehicleStickerStatusPending
	if req.IsDraft {
		status = property.VehicleStickerStatusDraft
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
		ParkingID:   req.ParkingSlotID,
		Category:    property.RequestCategoryParking,
		RequestType: property.RequestTypeParkingStickerApply,
		Title:       fmt.Sprintf("Vehicle Sticker Application - %s", unit.UnitNumber),
		Description: nil,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	// Note: Request status remains "pending" even for drafts
	// The draft status is tracked in VehicleSticker.Status field

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create vehicle sticker record
	vehicleSticker := property.VehicleSticker{
		RequestID:     request.ID,
		TenantID:      tenantID,
		UnitID:        req.UnitID,
		ParkingSlotID: req.ParkingSlotID,
		VehicleBrand:  req.VehicleBrand,
		VehicleModel:  req.VehicleModel,
		VehicleYear:   req.VehicleYear,
		VehicleColor:  req.VehicleColor,
		PlateNumber:   req.PlateNumber,
		Status:        status,
		IsDraft:       req.IsDraft,
	}

	if err := tx.Create(&vehicleSticker).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create vehicle sticker: " + err.Error()})
		return
	}

	// Save OR and CR images
	if err := h.saveVehicleStickerImages(c, tx, request.ID, userID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save images: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":         "Vehicle sticker application created successfully",
		"vehicle_sticker": vehicleSticker,
		"request":         request,
	})
}

// GetVehicleSticker gets a vehicle sticker by request ID
// @Summary Get vehicle sticker
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/vehicle-sticker [get]
func (h *RequestHandler) GetVehicleSticker(c *gin.Context) {
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

	var vehicleSticker property.VehicleSticker
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").Preload("ParkingSlot").First(&vehicleSticker).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vehicle sticker not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"vehicle_sticker": vehicleSticker,
	})
}

// UpdateVehicleStickerRequest represents vehicle sticker update payload
type UpdateVehicleStickerRequest struct {
	ParkingSlotID *uint   `json:"parking_slot_id"`
	VehicleBrand  *string `json:"vehicle_brand"`
	VehicleModel  *string `json:"vehicle_model"`
	VehicleYear   *string `json:"vehicle_year"`
	VehicleColor  *string `json:"vehicle_color"`
	PlateNumber   *string `json:"plate_number"`
	IsDraft       *bool   `json:"is_draft"`
}

// UpdateVehicleSticker updates a vehicle sticker
// @Summary Update vehicle sticker
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Request ID"
// @Param request formData UpdateVehicleStickerRequest true "Vehicle sticker update data"
// @Param or_image formData file false "OR (Official Receipt) image"
// @Param cr_image formData file false "CR (Certificate of Registration) image"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/vehicle-sticker [put]
func (h *RequestHandler) UpdateVehicleSticker(c *gin.Context) {
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

	var vehicleSticker property.VehicleSticker
	if err := propertyDB.Where("request_id = ?", requestID).First(&vehicleSticker).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vehicle sticker not found"})
		return
	}

	var req UpdateVehicleStickerRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.ParkingSlotID != nil {
		// Validate parking slot if provided
		var parkingSlot property.ParkingSlot
		if err := propertyDB.First(&parkingSlot, *req.ParkingSlotID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parking slot ID"})
			return
		}
		updates["parking_slot_id"] = *req.ParkingSlotID
	}

	if req.VehicleBrand != nil {
		updates["vehicle_brand"] = *req.VehicleBrand
	}

	if req.VehicleModel != nil {
		updates["vehicle_model"] = *req.VehicleModel
	}

	if req.VehicleYear != nil {
		updates["vehicle_year"] = *req.VehicleYear
	}

	if req.VehicleColor != nil {
		updates["vehicle_color"] = *req.VehicleColor
	}

	if req.PlateNumber != nil {
		updates["plate_number"] = *req.PlateNumber
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		if *req.IsDraft {
			updates["status"] = property.VehicleStickerStatusDraft
		} else {
			updates["status"] = property.VehicleStickerStatusPending
		}
	}

	if err := propertyDB.Model(&vehicleSticker).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update vehicle sticker: " + err.Error()})
		return
	}

	// Save OR and CR images if provided
	userID := middleware.GetUserID(c)
	if err := h.saveVehicleStickerImages(c, propertyDB, uint(requestID), userID); err != nil {
		// Log error but don't fail the update
	}

	// Reload vehicle sticker
	propertyDB.Preload("Request").Preload("Unit").Preload("ParkingSlot").First(&vehicleSticker, vehicleSticker.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":         "Vehicle sticker updated successfully",
		"vehicle_sticker": vehicleSticker,
	})
}

// saveVehicleStickerImages saves OR and CR images for a vehicle sticker
func (h *RequestHandler) saveVehicleStickerImages(c *gin.Context, db *gorm.DB, requestID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/requests/%d", requestID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"application/pdf": true,
	}

	maxSize := int64(10 * 1024 * 1024) // 10MB

	// Save OR image
	if orFile, ok := form.File["or_image"]; ok && len(orFile) > 0 {
		file := orFile[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("or_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
				filePath := filepath.Join(uploadDir, filename)

				src, err := file.Open()
				if err == nil {
					defer src.Close()
					dst, err := os.Create(filePath)
					if err == nil {
						defer dst.Close()
						if _, err := io.Copy(dst, src); err == nil {
							fileSize := file.Size
							doc := property.Document{
								EntityType:   property.DocEntityRequest,
								EntityID:     requestID,
								DocumentType: property.DocTypeOther,
								DocumentName: "OR_" + file.Filename,
								DocumentPath: filePath,
								FileSize:     &fileSize,
								MimeType:     &contentType,
								UploadedBy:   userID,
								IsActive:     true,
							}
							db.Create(&doc)
						}
					}
				}
			}
		}
	}

	// Save CR image
	if crFile, ok := form.File["cr_image"]; ok && len(crFile) > 0 {
		file := crFile[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("cr_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
				filePath := filepath.Join(uploadDir, filename)

				src, err := file.Open()
				if err == nil {
					defer src.Close()
					dst, err := os.Create(filePath)
					if err == nil {
						defer dst.Close()
						if _, err := io.Copy(dst, src); err == nil {
							fileSize := file.Size
							doc := property.Document{
								EntityType:   property.DocEntityRequest,
								EntityID:     requestID,
								DocumentType: property.DocTypeOther,
								DocumentName: "CR_" + file.Filename,
								DocumentPath: filePath,
								FileSize:     &fileSize,
								MimeType:     &contentType,
								UploadedBy:   userID,
								IsActive:     true,
							}
							db.Create(&doc)
						}
					}
				}
			}
		}
	}

	return nil
}

// CreatePetRegistrationRequest represents pet registration creation payload
type CreatePetRegistrationRequest struct {
	TenantID         *uint   `json:"tenant_id" form:"tenant_id"`
	UnitID           uint    `json:"unit_id" form:"unit_id" binding:"required"`
	ResidentName     string  `json:"resident_name" form:"resident_name" binding:"required"`
	MobileNumber     string  `json:"mobile_number" form:"mobile_number" binding:"required"`
	EmailAddress     string  `json:"email_address" form:"email_address" binding:"required,email"`
	PetName          string  `json:"pet_name" form:"pet_name" binding:"required"`
	PetType          string  `json:"pet_type" form:"pet_type" binding:"required,oneof=dog cat"`
	Breed            *string `json:"breed" form:"breed"`
	Weight           string  `json:"weight" form:"weight" binding:"required"`
	ColorMarkings    *string `json:"color_markings" form:"color_markings"`
	RabiesVaccinated string  `json:"rabies_vaccinated" form:"rabies_vaccinated" binding:"required,oneof=yes no"`
	IsNonAggressive  bool    `json:"is_non_aggressive" form:"is_non_aggressive"`
	WillKeepLeashed  bool    `json:"will_keep_leashed" form:"will_keep_leashed"`
	AgreeToRules     bool    `json:"agree_to_rules" form:"agree_to_rules"`
	IsDraft          bool    `json:"is_draft" form:"is_draft"`
}

// CreatePetRegistration creates a pet registration
// @Summary Create pet registration
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param request formData CreatePetRegistrationRequest true "Pet registration data"
// @Param pet_photo formData file false "Pet photo"
// @Param certificate formData file false "Rabies vaccination certificate"
// @Success 201 {object} map[string]interface{}
// @Router /requests/pet-registration [post]
func (h *RequestHandler) CreatePetRegistration(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create pet registrations
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create pet registrations"})
		return
	}

	var req CreatePetRegistrationRequest
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
	status := property.PetRegistrationStatusPending
	if req.IsDraft {
		status = property.PetRegistrationStatusDraft
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
		RequestType: property.RequestTypePetRegistration,
		Title:       fmt.Sprintf("Pet Registration - %s", unit.UnitNumber),
		Description: nil,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create pet registration record
	petRegistration := property.PetRegistration{
		RequestID:        request.ID,
		TenantID:         tenantID,
		UnitID:           req.UnitID,
		ResidentName:     req.ResidentName,
		MobileNumber:     req.MobileNumber,
		EmailAddress:     req.EmailAddress,
		PetName:          req.PetName,
		PetType:          req.PetType,
		Breed:            req.Breed,
		Weight:           req.Weight,
		ColorMarkings:    req.ColorMarkings,
		RabiesVaccinated: req.RabiesVaccinated,
		IsNonAggressive:  req.IsNonAggressive,
		WillKeepLeashed:  req.WillKeepLeashed,
		AgreeToRules:     req.AgreeToRules,
		Status:           status,
		IsDraft:          req.IsDraft,
	}

	if err := tx.Create(&petRegistration).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pet registration: " + err.Error()})
		return
	}

	// Save pet photo and certificate
	if err := h.savePetRegistrationImages(c, tx, request.ID, userID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save images: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":          "Pet registration created successfully",
		"pet_registration": petRegistration,
		"request":          request,
	})
}

// GetPetRegistration gets a pet registration by request ID
// @Summary Get pet registration
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/pet-registration [get]
func (h *RequestHandler) GetPetRegistration(c *gin.Context) {
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

	var petRegistration property.PetRegistration
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&petRegistration).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pet registration not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pet_registration": petRegistration,
	})
}

// UpdatePetRegistrationRequest represents pet registration update payload
type UpdatePetRegistrationRequest struct {
	ResidentName     *string `json:"resident_name" form:"resident_name"`
	MobileNumber     *string `json:"mobile_number" form:"mobile_number"`
	EmailAddress     *string `json:"email_address" form:"email_address"`
	PetName          *string `json:"pet_name" form:"pet_name"`
	PetType          *string `json:"pet_type" form:"pet_type"`
	Breed            *string `json:"breed" form:"breed"`
	Weight           *string `json:"weight" form:"weight"`
	ColorMarkings    *string `json:"color_markings" form:"color_markings"`
	RabiesVaccinated *string `json:"rabies_vaccinated" form:"rabies_vaccinated"`
	IsNonAggressive  *bool   `json:"is_non_aggressive" form:"is_non_aggressive"`
	WillKeepLeashed  *bool   `json:"will_keep_leashed" form:"will_keep_leashed"`
	AgreeToRules     *bool   `json:"agree_to_rules" form:"agree_to_rules"`
	IsDraft          *bool   `json:"is_draft" form:"is_draft"`
}

// UpdatePetRegistration updates a pet registration
// @Summary Update pet registration
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Request ID"
// @Param request formData UpdatePetRegistrationRequest true "Pet registration update data"
// @Param pet_photo formData file false "Pet photo"
// @Param certificate formData file false "Rabies vaccination certificate"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/pet-registration [put]
func (h *RequestHandler) UpdatePetRegistration(c *gin.Context) {
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

	var petRegistration property.PetRegistration
	if err := propertyDB.Where("request_id = ?", requestID).First(&petRegistration).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pet registration not found"})
		return
	}

	var req UpdatePetRegistrationRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.ResidentName != nil {
		updates["resident_name"] = *req.ResidentName
	}

	if req.MobileNumber != nil {
		updates["mobile_number"] = *req.MobileNumber
	}

	if req.EmailAddress != nil {
		updates["email_address"] = *req.EmailAddress
	}

	if req.PetName != nil {
		updates["pet_name"] = *req.PetName
	}

	if req.PetType != nil {
		if *req.PetType != property.PetTypeDog && *req.PetType != property.PetTypeCat {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pet type, must be dog or cat"})
			return
		}
		updates["pet_type"] = *req.PetType
	}

	if req.Breed != nil {
		updates["breed"] = *req.Breed
	}

	if req.Weight != nil {
		updates["weight"] = *req.Weight
	}

	if req.ColorMarkings != nil {
		updates["color_markings"] = *req.ColorMarkings
	}

	if req.RabiesVaccinated != nil {
		if *req.RabiesVaccinated != property.RabiesVaccinatedYes && *req.RabiesVaccinated != property.RabiesVaccinatedNo {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rabies_vaccinated value, must be yes or no"})
			return
		}
		updates["rabies_vaccinated"] = *req.RabiesVaccinated
	}

	if req.IsNonAggressive != nil {
		updates["is_non_aggressive"] = *req.IsNonAggressive
	}

	if req.WillKeepLeashed != nil {
		updates["will_keep_leashed"] = *req.WillKeepLeashed
	}

	if req.AgreeToRules != nil {
		updates["agree_to_rules"] = *req.AgreeToRules
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		if *req.IsDraft {
			updates["status"] = property.PetRegistrationStatusDraft
		} else {
			updates["status"] = property.PetRegistrationStatusPending
		}
	}

	if err := propertyDB.Model(&petRegistration).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pet registration: " + err.Error()})
		return
	}

	// Save pet photo and certificate if provided
	userID := middleware.GetUserID(c)
	if err := h.savePetRegistrationImages(c, propertyDB, uint(requestID), userID); err != nil {
		// Log error but don't fail the update
	}

	// Reload pet registration
	propertyDB.Preload("Request").Preload("Unit").First(&petRegistration, petRegistration.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":          "Pet registration updated successfully",
		"pet_registration": petRegistration,
	})
}

// savePetRegistrationImages saves pet photo and certificate for a pet registration
func (h *RequestHandler) savePetRegistrationImages(c *gin.Context, db *gorm.DB, requestID uint, userID uint) error {
	form, err := c.MultipartForm()
	if err != nil {
		return nil // No multipart form, skip
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/requests/%d", requestID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create upload directory: %v", err)
	}

	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"application/pdf": true,
	}

	maxSize := int64(10 * 1024 * 1024) // 10MB

	// Save pet photo
	if petPhotoFile, ok := form.File["pet_photo"]; ok && len(petPhotoFile) > 0 {
		file := petPhotoFile[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("pet_photo_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
				filePath := filepath.Join(uploadDir, filename)

				src, err := file.Open()
				if err == nil {
					defer src.Close()
					dst, err := os.Create(filePath)
					if err == nil {
						defer dst.Close()
						if _, err := io.Copy(dst, src); err == nil {
							fileSize := file.Size
							doc := property.Document{
								EntityType:   property.DocEntityRequest,
								EntityID:     requestID,
								DocumentType: property.DocTypePhoto,
								DocumentName: "Pet_Photo_" + file.Filename,
								DocumentPath: filePath,
								FileSize:     &fileSize,
								MimeType:     &contentType,
								UploadedBy:   userID,
								IsActive:     true,
							}
							db.Create(&doc)
						}
					}
				}
			}
		}
	}

	// Save certificate
	if certFile, ok := form.File["certificate"]; ok && len(certFile) > 0 {
		file := certFile[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("certificate_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
				filePath := filepath.Join(uploadDir, filename)

				src, err := file.Open()
				if err == nil {
					defer src.Close()
					dst, err := os.Create(filePath)
					if err == nil {
						defer dst.Close()
						if _, err := io.Copy(dst, src); err == nil {
							fileSize := file.Size
							doc := property.Document{
								EntityType:   property.DocEntityRequest,
								EntityID:     requestID,
								DocumentType: property.DocTypeOther,
								DocumentName: "Certificate_" + file.Filename,
								DocumentPath: filePath,
								FileSize:     &fileSize,
								MimeType:     &contentType,
								UploadedBy:   userID,
								IsActive:     true,
							}
							db.Create(&doc)
						}
					}
				}
			}
		}
	}

	return nil
}
