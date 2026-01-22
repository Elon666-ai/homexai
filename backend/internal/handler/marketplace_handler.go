package handler

import (
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/property"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

type MarketplaceHandler struct{}

func NewMarketplaceHandler() *MarketplaceHandler {
	return &MarketplaceHandler{}
}

// getMarketplaceService gets marketplace service from property context
func (h *MarketplaceHandler) getMarketplaceService(c *gin.Context) *service.MarketplaceService {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		return nil
	}
	return service.NewMarketplaceService(propertyDB)
}

// ========== Service Listing Management (Property Admin) ==========

// CreateServiceListingRequest represents create service listing request
type CreateServiceListingRequest struct {
	ServiceType       string  `json:"service_type" binding:"required,oneof=renovation cleaning repair moving other"`
	Title             string  `json:"title" binding:"required,min=1,max=200"`
	Description       string  `json:"description"`
	Price             float64 `json:"price" binding:"required,gte=0"` // Allow 0 for free services
	Currency          string  `json:"currency" binding:"required"`
	IsPropertyService bool    `json:"is_property_service" binding:"required"`
	ThirdPartyContact string  `json:"third_party_contact"` // Required if IsPropertyService is false
	Status            string  `json:"status"`              // Optional, defaults to active
}

// CreateServiceListing creates a new service listing
// @Summary Create service listing
// @Description Create a new service listing (property admin only)
// @Tags Marketplace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateServiceListingRequest true "Create service listing request"
// @Success 201 {object} map[string]interface{}
// @Router /property-admin/marketplace/services [post]
func (h *MarketplaceHandler) CreateServiceListing(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	var req CreateServiceListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Validate third party contact
	if !req.IsPropertyService && req.ThirdPartyContact == "" {
		utils.BadRequestResponse(c, "Third party contact is required for third party services", nil)
		return
	}

	listing := &property.ServiceListing{
		ServiceType:       req.ServiceType,
		Title:             req.Title,
		Price:             strconv.FormatFloat(req.Price, 'f', 2, 64),
		Currency:          req.Currency,
		IsPropertyService: req.IsPropertyService,
		Status:            req.Status,
	}

	if req.Description != "" {
		listing.Description = &req.Description
	}

	if req.ThirdPartyContact != "" {
		listing.ThirdPartyContact = &req.ThirdPartyContact
	}

	if listing.Status == "" {
		listing.Status = property.ServiceListingStatusActive
	}

	createdBy := middleware.GetUserID(c)
	listing.CreatedBy = &createdBy

	err := marketplaceService.CreateServiceListing(listing)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.CreatedResponse(c, "Service listing created successfully", listing)
}

// UpdateServiceListingRequest represents update service listing request
type UpdateServiceListingRequest struct {
	ServiceType       *string  `json:"service_type"`
	Title             *string  `json:"title"`
	Description       *string  `json:"description"`
	Price             *float64 `json:"price"`
	Currency          *string  `json:"currency"`
	IsPropertyService *bool    `json:"is_property_service"`
	ThirdPartyContact *string  `json:"third_party_contact"`
	Status            *string  `json:"status"`
}

// UpdateServiceListing updates a service listing
// @Summary Update service listing
// @Description Update a service listing (property admin only)
// @Tags Marketplace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Listing ID"
// @Param request body UpdateServiceListingRequest true "Update service listing request"
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/services/{id} [put]
func (h *MarketplaceHandler) UpdateServiceListing(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service listing ID", nil)
		return
	}

	var req UpdateServiceListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Get existing listing
	listing, err := marketplaceService.GetServiceListing(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Service listing not found")
		return
	}

	// Update fields
	if req.ServiceType != nil {
		listing.ServiceType = *req.ServiceType
	}
	if req.Title != nil {
		listing.Title = *req.Title
	}
	if req.Description != nil {
		listing.Description = req.Description
	}
	if req.Price != nil {
		// Validate price (allow 0 for free services)
		if *req.Price < 0 {
			utils.BadRequestResponse(c, "Price cannot be negative", nil)
			return
		}
		listing.Price = strconv.FormatFloat(*req.Price, 'f', 2, 64)
	}
	if req.Currency != nil {
		listing.Currency = *req.Currency
	}
	if req.IsPropertyService != nil {
		listing.IsPropertyService = *req.IsPropertyService
	}
	if req.ThirdPartyContact != nil {
		listing.ThirdPartyContact = req.ThirdPartyContact
	}
	if req.Status != nil {
		listing.Status = *req.Status
	}

	// Validate third party contact
	if !listing.IsPropertyService && (listing.ThirdPartyContact == nil || *listing.ThirdPartyContact == "") {
		utils.BadRequestResponse(c, "Third party contact is required for third party services", nil)
		return
	}

	err = marketplaceService.UpdateServiceListing(listing)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Service listing updated successfully", listing)
}

// GetServiceListing gets a service listing by ID
// @Summary Get service listing
// @Description Get service listing details by ID
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Listing ID"
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/services/{id} [get]
func (h *MarketplaceHandler) GetServiceListing(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service listing ID", nil)
		return
	}

	listing, err := marketplaceService.GetServiceListing(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Service listing not found")
		return
	}

	utils.SuccessResponse(c, "Service listing retrieved successfully", listing)
}

// ListServiceListings lists service listings
// @Summary List service listings
// @Description List service listings with filters (property admin only)
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param service_type query string false "Service type filter"
// @Param status query string false "Status filter (active, inactive)"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/services [get]
func (h *MarketplaceHandler) ListServiceListings(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	serviceType := c.Query("service_type")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	listings, total, err := marketplaceService.ListServiceListings(serviceType, status, page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve service listings", err)
		return
	}

	utils.SuccessResponseWithPagination(c, listings, total, page, perPage, "Service listings retrieved successfully")
}

// UpdateServiceListingStatus updates service listing status (上架/下架)
// @Summary Update service listing status
// @Description Update service listing status (property admin only)
// @Tags Marketplace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Listing ID"
// @Param status body map[string]string true "Status (active/inactive)"
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/services/{id}/status [put]
func (h *MarketplaceHandler) UpdateServiceListingStatus(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service listing ID", nil)
		return
	}

	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	status, ok := req["status"]
	if !ok {
		utils.BadRequestResponse(c, "Status is required", nil)
		return
	}

	err = marketplaceService.UpdateServiceListingStatus(uint(id), status)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Service listing status updated successfully", nil)
}

// DeleteServiceListing deletes a service listing
// @Summary Delete service listing
// @Description Delete a service listing (property admin only)
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Listing ID"
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/services/{id} [delete]
func (h *MarketplaceHandler) DeleteServiceListing(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service listing ID", nil)
		return
	}

	err = marketplaceService.DeleteServiceListing(uint(id))
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Service listing deleted successfully", nil)
}

// ========== Service Order Management ==========

// CreateServiceOrderRequest represents create service order request
type CreateServiceOrderRequest struct {
	ServiceListingID uint   `json:"service_listing_id" binding:"required"`
	UnitID           uint   `json:"unit_id" binding:"required"`
	Nickname         string `json:"nickname" binding:"required,min=1,max=100"`
	Phone            string `json:"phone" binding:"required"`
	ServiceTime      string `json:"service_time" binding:"required"` // YYYY-MM-DD HH:mm format
	Notes            string `json:"notes"`
}

// CreateServiceOrder creates a new service order
// @Summary Create service order
// @Description Create a new service order
// @Tags Marketplace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateServiceOrderRequest true "Create service order request"
// @Success 201 {object} map[string]interface{}
// @Router /marketplace/orders [post]
func (h *MarketplaceHandler) CreateServiceOrder(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	var req CreateServiceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Parse service time
	serviceTime, err := time.Parse("2006-01-02 15:04", req.ServiceTime)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service time format. Use YYYY-MM-DD HH:mm", nil)
		return
	}

	order := &property.ServiceOrder{
		ServiceListingID: req.ServiceListingID,
		UnitID:           req.UnitID,
		UserID:           middleware.GetUserID(c),
		Nickname:         req.Nickname,
		Phone:            req.Phone,
		ServiceTime:      serviceTime,
		Status:           property.ServiceOrderStatusInService,
	}

	if req.Notes != "" {
		order.Notes = &req.Notes
	}

	err = marketplaceService.CreateServiceOrder(order)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.CreatedResponse(c, "Service order created successfully", order)
}

// GetServiceOrder gets a service order by ID
// @Summary Get service order
// @Description Get service order details by ID
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Order ID"
// @Success 200 {object} map[string]interface{}
// @Router /marketplace/orders/{id} [get]
func (h *MarketplaceHandler) GetServiceOrder(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service order ID", nil)
		return
	}

	order, err := marketplaceService.GetServiceOrder(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Service order not found")
		return
	}

	utils.SuccessResponse(c, "Service order retrieved successfully", order)
}

// ListServiceOrders lists service orders
// @Summary List service orders
// @Description List service orders with filters
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param status query string false "Status filter (in_service, completed, cancelled)"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /marketplace/orders [get]
func (h *MarketplaceHandler) ListServiceOrders(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	var userID uint
	// For property admin/staff, list all orders; for regular users, list their own
	userRole := middleware.GetUserRole(c)
	if userRole == "property_admin" || userRole == "property_staff" || userRole == "super_admin" {
		userID = 0 // 0 means all orders
	} else {
		userID = middleware.GetUserID(c)
	}

	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	orders, total, err := marketplaceService.ListServiceOrders(userID, status, page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve service orders", err)
		return
	}

	utils.SuccessResponseWithPagination(c, orders, total, page, perPage, "Service orders retrieved successfully")
}

// GetMyServiceOrders gets current user's service orders
// @Summary Get my service orders
// @Description Get service orders for current authenticated user
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /marketplace/orders/my [get]
func (h *MarketplaceHandler) GetMyServiceOrders(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	orders, total, err := marketplaceService.ListUserServiceOrders(userID, page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve service orders", err)
		return
	}

	utils.SuccessResponseWithPagination(c, orders, total, page, perPage, "Service orders retrieved successfully")
}

// AssignStaffRequest represents assign staff request
type AssignStaffRequest struct {
	StaffID uint `json:"staff_id" binding:"required"`
}

// AssignStaff assigns staff to a service order
// @Summary Assign staff to service order
// @Description Assign staff to a service order (property admin only)
// @Tags Marketplace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Order ID"
// @Param request body AssignStaffRequest true "Assign staff request"
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/orders/{id}/assign-staff [post]
func (h *MarketplaceHandler) AssignStaff(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service order ID", nil)
		return
	}

	var req AssignStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err = marketplaceService.AssignStaff(uint(id), req.StaffID)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Staff assigned successfully", nil)
}

// CompleteServiceOrder marks a service order as completed
// @Summary Complete service order
// @Description Mark a service order as completed (property admin/staff only)
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Order ID"
// @Success 200 {object} map[string]interface{}
// @Router /property-admin/marketplace/orders/{id}/complete [post]
func (h *MarketplaceHandler) CompleteServiceOrder(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service order ID", nil)
		return
	}

	err = marketplaceService.CompleteServiceOrder(uint(id))
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Service order completed successfully", nil)
}

// ConfirmServiceOrder confirms a service order
// @Summary Confirm service order
// @Description Confirm a service order by user
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Order ID"
// @Success 200 {object} map[string]interface{}
// @Router /marketplace/orders/{id}/confirm [post]
func (h *MarketplaceHandler) ConfirmServiceOrder(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service order ID", nil)
		return
	}

	userID := middleware.GetUserID(c)
	err = marketplaceService.ConfirmServiceOrder(uint(id), userID)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Service order confirmed successfully", nil)
}

// CancelServiceOrder cancels a service order
// @Summary Cancel service order
// @Description Cancel a service order
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param id path int true "Service Order ID"
// @Success 200 {object} map[string]interface{}
// @Router /marketplace/orders/{id}/cancel [post]
func (h *MarketplaceHandler) CancelServiceOrder(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid service order ID", nil)
		return
	}

	userID := middleware.GetUserID(c)
	err = marketplaceService.CancelServiceOrder(uint(id), userID)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Service order cancelled successfully", nil)
}

// ========== Public Service Listings (for users to browse) ==========

// ListActiveServiceListings lists active service listings
// @Summary List active service listings
// @Description List active service listings for users to browse
// @Tags Marketplace
// @Produce json
// @Security BearerAuth
// @Param service_type query string false "Service type filter"
// @Success 200 {object} map[string]interface{}
// @Router /marketplace/services [get]
func (h *MarketplaceHandler) ListActiveServiceListings(c *gin.Context) {
	marketplaceService := h.getMarketplaceService(c)
	if marketplaceService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	serviceType := c.Query("service_type")

	listings, err := marketplaceService.ListActiveServiceListings(serviceType)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve service listings", err)
		return
	}

	utils.SuccessResponse(c, "Service listings retrieved successfully", listings)
}
