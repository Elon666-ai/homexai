package handler

import (
	"strconv"

	"homexai/internal/middleware"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

type PropertyAdminHandler struct {
	adminService *service.PropertyAdminService
}

func NewPropertyAdminHandler() *PropertyAdminHandler {
	return &PropertyAdminHandler{
		adminService: service.NewPropertyAdminService(),
	}
}

// AssignPropertyAdminRequest represents the request to assign a property admin
type AssignPropertyAdminRequest struct {
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"full_name" binding:"required,min=2,max=100"`
	Phone    string `json:"phone"`
}

// GetPropertyAdmin gets the current property admin for a property
// @Summary Get property admin
// @Description Get the current property admin for a property (super_admin only)
// @Tags Property Admin Assignment
// @Produce json
// @Security BearerAuth
// @Param property_id path int true "Property ID"
// @Success 200 {object} map[string]interface{}
// @Router /super/properties/{property_id}/admin [get]
func (h *PropertyAdminHandler) GetPropertyAdmin(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("property_id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid property ID", nil)
		return
	}

	admin, err := h.adminService.GetPropertyAdmin(uint(propertyID))
	if err != nil {
		utils.NotFoundResponse(c, "No property admin assigned")
		return
	}

	utils.SuccessResponse(c, "Property admin retrieved successfully", admin)
}

// AssignPropertyAdmin assigns a property admin to a property
// @Summary Assign property admin
// @Description Assign a user as property admin (super_admin only, only one per property)
// @Tags Property Admin Assignment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param property_id path int true "Property ID"
// @Param request body AssignPropertyAdminRequest true "Admin assignment request"
// @Success 201 {object} map[string]interface{}
// @Router /super/properties/{property_id}/admin [post]
func (h *PropertyAdminHandler) AssignPropertyAdmin(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("property_id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid property ID", nil)
		return
	}

	currentUserID := middleware.GetUserID(c)

	var req AssignPropertyAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	result, err := h.adminService.AssignPropertyAdmin(uint(propertyID), currentUserID, service.AssignAdminInput{
		Email:    req.Email,
		FullName: req.FullName,
		Phone:    req.Phone,
	})
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.CreatedResponse(c, "Property admin assigned successfully", result)
}

// ReplacePropertyAdmin replaces the current property admin with a new one
// @Summary Replace property admin
// @Description Replace the current property admin with a new one (super_admin only)
// @Tags Property Admin Assignment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param property_id path int true "Property ID"
// @Param request body AssignPropertyAdminRequest true "Admin replacement request"
// @Success 200 {object} map[string]interface{}
// @Router /super/properties/{property_id}/admin [put]
func (h *PropertyAdminHandler) ReplacePropertyAdmin(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("property_id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid property ID", nil)
		return
	}

	currentUserID := middleware.GetUserID(c)

	var req AssignPropertyAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	result, err := h.adminService.ReplacePropertyAdmin(uint(propertyID), currentUserID, service.AssignAdminInput{
		Email:    req.Email,
		FullName: req.FullName,
		Phone:    req.Phone,
	})
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Property admin replaced successfully", result)
}

// RemovePropertyAdmin removes the property admin from a property
// @Summary Remove property admin
// @Description Remove the current property admin from a property (super_admin only)
// @Tags Property Admin Assignment
// @Produce json
// @Security BearerAuth
// @Param property_id path int true "Property ID"
// @Success 200 {object} map[string]interface{}
// @Router /super/properties/{property_id}/admin [delete]
func (h *PropertyAdminHandler) RemovePropertyAdmin(c *gin.Context) {
	propertyID, err := strconv.ParseUint(c.Param("property_id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid property ID", nil)
		return
	}

	err = h.adminService.RemovePropertyAdmin(uint(propertyID))
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Property admin removed successfully", nil)
}

// ListPropertiesWithAdmins lists all properties with their admin info
// @Summary List properties with admins
// @Description List all properties with their assigned property admins (super_admin only)
// @Tags Property Admin Assignment
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /super/properties [get]
func (h *PropertyAdminHandler) ListPropertiesWithAdmins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	properties, total, err := h.adminService.ListPropertiesWithAdmins(page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve properties", err)
		return
	}

	utils.SuccessResponseWithPagination(c, properties, total, page, perPage, "Properties retrieved successfully")
}
