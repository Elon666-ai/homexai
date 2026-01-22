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

// CreateHouseholdStaffRegistrationRequest represents household staff registration creation payload
type CreateHouseholdStaffRegistrationRequest struct {
	TenantID             *uint   `json:"tenant_id" form:"tenant_id"`
	UnitID               uint    `json:"unit_id" form:"unit_id" binding:"required"`
	ResidentName         string  `json:"resident_name" form:"resident_name" binding:"required"`
	LastName             string  `json:"last_name" form:"last_name" binding:"required"`
	FirstName            string  `json:"first_name" form:"first_name" binding:"required"`
	MiddleName           *string `json:"middle_name" form:"middle_name"`
	Gender               string  `json:"gender" form:"gender" binding:"required,oneof=male female other"`
	Designation          string  `json:"designation" form:"designation" binding:"required,oneof=driver housekeeper other"`
	StayInStayOut        string  `json:"stay_in_stay_out" form:"stay_in_stay_out" binding:"required,oneof=stay_in stay_out"`
	EmployeeMobileNumber string  `json:"employee_mobile_number" form:"employee_mobile_number" binding:"required"`
	EmployeeAddress      string  `json:"employee_address" form:"employee_address" binding:"required"`
	IsDraft              bool    `json:"is_draft" form:"is_draft"`
}

// CreateHouseholdStaffRegistration creates a household staff registration
// @Summary Create household staff registration
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param request formData CreateHouseholdStaffRegistrationRequest true "Household staff registration data"
// @Param nbi_clearance formData file false "NBI / Police Clearance"
// @Param government_id formData file false "Government Issued Valid ID"
// @Param id_photo formData file false "2x2 ID Photo"
// @Success 201 {object} map[string]interface{}
// @Router /requests/household-staff-registration [post]
func (h *RequestHandler) CreateHouseholdStaffRegistration(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Property database not found"})
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Only tenant, landlord, spa can create household staff registrations
	if userRole != "landlord" && userRole != "spa" && userRole != "tenant" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only tenant, landlord, or spa can create household staff registrations"})
		return
	}

	var req CreateHouseholdStaffRegistrationRequest
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
	status := property.HouseholdStaffRegistrationStatusPending
	if req.IsDraft {
		status = property.HouseholdStaffRegistrationStatusDraft
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
		RequestType: property.RequestTypeHouseholdStaffRegistration,
		Title:       fmt.Sprintf("Household Staff Registration - %s", unit.UnitNumber),
		Description: nil,
		Priority:    property.RequestPriorityNormal,
		Status:      property.RequestStatusPending,
	}

	// Note: Request status remains "pending" even for drafts
	// The draft status is tracked in HouseholdStaffRegistration.Status field

	if err := tx.Create(&request).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}

	// Create household staff registration record
	hsr := property.HouseholdStaffRegistration{
		RequestID:            request.ID,
		TenantID:             tenantID,
		UnitID:               req.UnitID,
		ResidentName:         req.ResidentName,
		LastName:             req.LastName,
		FirstName:            req.FirstName,
		MiddleName:           req.MiddleName,
		Gender:               req.Gender,
		Designation:          req.Designation,
		StayInStayOut:        req.StayInStayOut,
		EmployeeMobileNumber: req.EmployeeMobileNumber,
		EmployeeAddress:      req.EmployeeAddress,
		Status:               status,
		IsDraft:              req.IsDraft,
	}

	if err := tx.Create(&hsr).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create household staff registration: " + err.Error()})
		return
	}

	// Save documents (NBI clearance, government ID, ID photo)
	if err := h.saveHouseholdStaffRegistrationDocuments(c, tx, request.ID, userID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save documents: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":                      "Household staff registration created successfully",
		"household_staff_registration": hsr,
		"request":                      request,
	})
}

// GetHouseholdStaffRegistration gets a household staff registration by request ID
// @Summary Get household staff registration
// @Tags Requests
// @Produce json
// @Param id path int true "Request ID"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/household-staff-registration [get]
func (h *RequestHandler) GetHouseholdStaffRegistration(c *gin.Context) {
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

	var hsr property.HouseholdStaffRegistration
	if err := propertyDB.Where("request_id = ?", requestID).Preload("Request").Preload("Unit").First(&hsr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Household staff registration not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"household_staff_registration": hsr,
	})
}

// UpdateHouseholdStaffRegistrationRequest represents household staff registration update payload
type UpdateHouseholdStaffRegistrationRequest struct {
	ResidentName         *string `json:"resident_name" form:"resident_name"`
	LastName             *string `json:"last_name" form:"last_name"`
	FirstName            *string `json:"first_name" form:"first_name"`
	MiddleName           *string `json:"middle_name" form:"middle_name"`
	Gender               *string `json:"gender" form:"gender"`
	Designation          *string `json:"designation" form:"designation"`
	StayInStayOut        *string `json:"stay_in_stay_out" form:"stay_in_stay_out"`
	EmployeeMobileNumber *string `json:"employee_mobile_number" form:"employee_mobile_number"`
	EmployeeAddress      *string `json:"employee_address" form:"employee_address"`
	IsDraft              *bool   `json:"is_draft" form:"is_draft"`
}

// UpdateHouseholdStaffRegistration updates a household staff registration
// @Summary Update household staff registration
// @Tags Requests
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Request ID"
// @Param request formData UpdateHouseholdStaffRegistrationRequest true "Household staff registration update data"
// @Param nbi_clearance formData file false "NBI / Police Clearance"
// @Param government_id formData file false "Government Issued Valid ID"
// @Param id_photo formData file false "2x2 ID Photo"
// @Success 200 {object} map[string]interface{}
// @Router /requests/:id/household-staff-registration [put]
func (h *RequestHandler) UpdateHouseholdStaffRegistration(c *gin.Context) {
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

	var hsr property.HouseholdStaffRegistration
	if err := propertyDB.Where("request_id = ?", requestID).First(&hsr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Household staff registration not found"})
		return
	}

	var req UpdateHouseholdStaffRegistrationRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.ResidentName != nil {
		updates["resident_name"] = *req.ResidentName
	}

	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}

	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}

	if req.MiddleName != nil {
		updates["middle_name"] = *req.MiddleName
	}

	if req.Gender != nil {
		if *req.Gender != property.GenderMale && *req.Gender != property.GenderFemale && *req.Gender != property.GenderOther {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gender, must be male, female, or other"})
			return
		}
		updates["gender"] = *req.Gender
	}

	if req.Designation != nil {
		if *req.Designation != property.DesignationDriver && *req.Designation != property.DesignationHousekeeper && *req.Designation != property.DesignationOther {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid designation, must be driver, housekeeper, or other"})
			return
		}
		updates["designation"] = *req.Designation
	}

	if req.StayInStayOut != nil {
		if *req.StayInStayOut != property.StayInStayOutIn && *req.StayInStayOut != property.StayInStayOutOut {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stay_in_stay_out, must be stay_in or stay_out"})
			return
		}
		updates["stay_in_stay_out"] = *req.StayInStayOut
	}

	if req.EmployeeMobileNumber != nil {
		updates["employee_mobile_number"] = *req.EmployeeMobileNumber
	}

	if req.EmployeeAddress != nil {
		updates["employee_address"] = *req.EmployeeAddress
	}

	if req.IsDraft != nil {
		updates["is_draft"] = *req.IsDraft
		if *req.IsDraft {
			updates["status"] = property.HouseholdStaffRegistrationStatusDraft
		} else {
			updates["status"] = property.HouseholdStaffRegistrationStatusPending
		}
	}

	if err := propertyDB.Model(&hsr).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update household staff registration: " + err.Error()})
		return
	}

	// Save documents if provided
	userID := middleware.GetUserID(c)
	if err := h.saveHouseholdStaffRegistrationDocuments(c, propertyDB, uint(requestID), userID); err != nil {
		// Log error but don't fail the update
	}

	// Reload household staff registration
	propertyDB.Preload("Request").Preload("Unit").First(&hsr, hsr.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":                      "Household staff registration updated successfully",
		"household_staff_registration": hsr,
	})
}

// saveHouseholdStaffRegistrationDocuments saves NBI clearance, government ID, and ID photo for a household staff registration
func (h *RequestHandler) saveHouseholdStaffRegistrationDocuments(c *gin.Context, db *gorm.DB, requestID uint, userID uint) error {
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

	// Save NBI / Police Clearance
	if nbiFiles, ok := form.File["nbi_clearance"]; ok && len(nbiFiles) > 0 {
		file := nbiFiles[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("nbi_clearance_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
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
								DocumentName: "NBI_Clearance_" + file.Filename,
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

	// Save Government Issued Valid ID
	if govIdFiles, ok := form.File["government_id"]; ok && len(govIdFiles) > 0 {
		file := govIdFiles[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("government_id_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
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
								DocumentType: property.DocTypeIDCopy,
								DocumentName: "Government_ID_" + file.Filename,
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

	// Save 2x2 ID Photo
	if idPhotoFiles, ok := form.File["id_photo"]; ok && len(idPhotoFiles) > 0 {
		file := idPhotoFiles[0]
		if file.Size <= maxSize {
			contentType := file.Header.Get("Content-Type")
			if allowedTypes[contentType] {
				ext := filepath.Ext(file.Filename)
				filename := fmt.Sprintf("id_photo_%d_%s%s", time.Now().UnixNano(), sanitizeFilename2(file.Filename), ext)
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
								DocumentName: "ID_Photo_" + file.Filename,
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
