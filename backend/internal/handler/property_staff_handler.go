package handler

import (
	"strconv"

	"homexai/internal/middleware"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

type PropertyStaffHandler struct {
	staffService *service.PropertyStaffService
	smtpService  *service.SmtpService
}

func NewPropertyStaffHandler(smtpSvc *service.SmtpService) *PropertyStaffHandler {
	return &PropertyStaffHandler{
		staffService: service.NewPropertyStaffService(),
		smtpService:  smtpSvc,
	}
}

// CreateStaffRequest represents the request to create a staff account
type CreateStaffRequest struct {
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"full_name" binding:"required,min=2,max=100"`
	Phone    string `json:"phone"`
	Role     string `json:"role" binding:"required,oneof=property_account property_staff"`
}

// UpdateStaffRequest represents the request to update a staff account
type UpdateStaffRequest struct {
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Status   string `json:"status" binding:"omitempty,oneof=active inactive suspended"`
}

// ListStaff lists all staff accounts for the current property
// @Summary List property staff
// @Description List all property_account and property_staff for the current property
// @Tags Property Staff
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Param role query string false "Filter by role (property_account, property_staff)"
// @Success 200 {object} map[string]interface{}
// @Router /property/staff [get]
func (h *PropertyStaffHandler) ListStaff(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.BadRequestResponse(c, "Property ID required", nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	roleFilter := c.Query("role")

	staff, total, err := h.staffService.ListPropertyStaff(propertyID, roleFilter, page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve staff", err)
		return
	}

	utils.SuccessResponseWithPagination(c, staff, total, page, perPage, "Staff retrieved successfully")
}

// GetStaff gets a specific staff member
// @Summary Get staff member
// @Description Get a specific staff member by ID
// @Tags Property Staff
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /property/staff/{id} [get]
func (h *PropertyStaffHandler) GetStaff(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.BadRequestResponse(c, "Property ID required", nil)
		return
	}

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	staff, err := h.staffService.GetPropertyStaff(propertyID, uint(userID))
	if err != nil {
		utils.NotFoundResponse(c, "Staff member not found")
		return
	}

	utils.SuccessResponse(c, "Staff retrieved successfully", staff)
}

// CreateStaff creates a new staff account
// @Summary Create staff account
// @Description Create a new property_account or property_staff account
// @Tags Property Staff
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateStaffRequest true "Staff creation request"
// @Success 201 {object} map[string]interface{}
// @Router /property/staff [post]
func (h *PropertyStaffHandler) CreateStaff(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.BadRequestResponse(c, "Property ID required", nil)
		return
	}

	currentUserID := middleware.GetUserID(c)

	var req CreateStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Validate role - only property_account, property_staff, and property_guard allowed
	if req.Role != "property_account" && req.Role != "property_staff" && req.Role != "property_guard" {
		utils.BadRequestResponse(c, "Invalid role. Only property_account, property_staff, and property_guard are allowed", nil)
		return
	}

	result, err := h.staffService.CreatePropertyStaff(propertyID, currentUserID, service.CreateStaffInput{
		Email:    req.Email,
		FullName: req.FullName,
		Phone:    req.Phone,
		Role:     req.Role,
	})
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	// Send email with credentials if new user was created OR existing user was reactivated
	shouldSendEmail := (result.IsNewUser || result.IsReactivated) && result.TempPassword != ""
	if shouldSendEmail && h.smtpService != nil {
		// Get property name for the email
		propertyName := middleware.GetPropertyName(c)
		if propertyName == "" {
			propertyName = "HomeX Property" // Fallback name
		}

		// Send credentials email asynchronously (don't block the response)
		go func() {
			if err := h.smtpService.SendStaffCredentialsEmail(
				req.Email,
				req.FullName,
				result.TempPassword,
				req.Role,
				propertyName,
			); err != nil {
				// Log error but don't fail the request
				// The staff account was created successfully, email is just a notification
				println("Failed to send staff credentials email:", err.Error())
			}
		}()
	}

	// Set appropriate response message
	var message string
	if result.IsReactivated {
		message = "Staff account reactivated successfully. New login credentials have been sent to the email address."
	} else if result.IsNewUser {
		message = "Staff account created successfully. Login credentials have been sent to the email address."
	} else {
		message = "Staff account created successfully. User already has an account, no email was sent."
	}

	utils.CreatedResponse(c, message, result.Staff)
}

// UpdateStaff updates a staff account
// @Summary Update staff account
// @Description Update a property_account or property_staff account
// @Tags Property Staff
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body UpdateStaffRequest true "Staff update request"
// @Success 200 {object} map[string]interface{}
// @Router /property/staff/{id} [put]
func (h *PropertyStaffHandler) UpdateStaff(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.BadRequestResponse(c, "Property ID required", nil)
		return
	}

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	var req UpdateStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err = h.staffService.UpdatePropertyStaff(propertyID, uint(userID), service.UpdateStaffInput{
		FullName: req.FullName,
		Phone:    req.Phone,
		Status:   req.Status,
	})
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Staff updated successfully", nil)
}

// DeleteStaff removes a staff account from the property
// @Summary Remove staff account
// @Description Remove a property_account or property_staff from the property
// @Tags Property Staff
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /property/staff/{id} [delete]
func (h *PropertyStaffHandler) DeleteStaff(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.BadRequestResponse(c, "Property ID required", nil)
		return
	}

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID", nil)
		return
	}

	// Log the delete attempt for debugging
	println("DeleteStaff called: property_id=", propertyID, ", user_id=", userID)

	err = h.staffService.RemovePropertyStaff(propertyID, uint(userID))
	if err != nil {
		println("DeleteStaff error:", err.Error())
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	println("DeleteStaff success: staff removed")
	utils.SuccessResponse(c, "Staff removed successfully", nil)
}
