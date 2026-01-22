package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

type BillHandler struct{}

func NewBillHandler() *BillHandler {
	return &BillHandler{}
}

// BillItemRequest represents a bill item in create request
type BillItemRequest struct {
	ItemType    string  `json:"item_type" binding:"required"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount" binding:"required,gte=0"`
}

// CreateBillRequest represents create bill request
type CreateBillRequest struct {
	UnitID        *uint            `json:"unit_id"`         // Required for unit bills
	ParkingSlotID *uint            `json:"parking_slot_id"`  // Required for parking bills
	BillType      string           `json:"bill_type" binding:"required,oneof=unit parking"`
	BillingMonth  string           `json:"billing_month" binding:"required"` // YYYY-MM format
	Items         []BillItemRequest `json:"items" binding:"required,min=1"` // Bill items
	DueDate       string           `json:"due_date" binding:"required"`    // YYYY-MM-DD format
	Description   string           `json:"description"`
	Currency      string           `json:"currency"` // Default: PHP
}

// getBillService gets bill service from property context
func (h *BillHandler) getBillService(c *gin.Context) *service.BillService {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		return nil
	}

	return service.NewBillService(propertyDB)
}

type BillService struct {
	propertyDB *gorm.DB
}

func NewBillService(propertyDB *gorm.DB) *BillService {
	return &BillService{
		propertyDB: propertyDB,
	}
}

// CreateBill creates a new bill (only for accountant role)
// @Summary Create bill
// @Description Create a new bill with items. Only accountant can create bills.
// @Tags Bill
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateBillRequest true "Create bill request"
// @Success 201 {object} map[string]interface{}
// @Router /bills [post]
func (h *BillHandler) CreateBill(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	// Check if user is accountant
	userRole := middleware.GetUserRole(c)
	if userRole != "property_account" && userRole != "property_admin" && userRole != "super_admin" {
		utils.UnauthorizedResponse(c, "Only accountant can create bills")
		return
	}

	var req CreateBillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	propertyDB := middleware.GetPropertyDB(c)
	
	// Validate bill type and required fields
	if req.BillType == property.BillTypeUnit {
		if req.UnitID == nil || *req.UnitID == 0 {
			utils.BadRequestResponse(c, "unit_id is required for unit bills", nil)
			return
		}
		// Validate unit exists
		var unit property.Unit
		if err := propertyDB.First(&unit, *req.UnitID).Error; err != nil {
			utils.BadRequestResponse(c, "Invalid unit_id: unit not found", nil)
			return
		}
	} else if req.BillType == property.BillTypeParking {
		if req.ParkingSlotID == nil || *req.ParkingSlotID == 0 {
			utils.BadRequestResponse(c, "parking_slot_id is required for parking bills", nil)
			return
		}
		// Validate parking slot exists
		var parkingSlot property.ParkingSlot
		if err := propertyDB.First(&parkingSlot, *req.ParkingSlotID).Error; err != nil {
			utils.BadRequestResponse(c, "Invalid parking_slot_id: parking slot not found", nil)
			return
		}
	}

	// Validate and parse billing month (YYYY-MM format)
	if req.BillingMonth == "" {
		utils.BadRequestResponse(c, "billing_month is required", nil)
		return
	}
	_, err := time.Parse("2006-01", req.BillingMonth)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid billing month format. Use YYYY-MM", nil)
		return
	}

	// Parse due date
	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid due date format. Use YYYY-MM-DD", nil)
		return
	}

	createdBy := middleware.GetUserID(c)

	// Get landlord, tenant, and SPA IDs
	var landlordID, tenantID, spaID *uint
	var userID uint

	if req.BillType == property.BillTypeUnit {
		// Get unit landlords and tenants
		var landlords []property.Landlord
		propertyDB.Where("unit_id = ?", *req.UnitID).Find(&landlords)
		if len(landlords) > 0 {
			landlordID = &landlords[0].UserID
			userID = landlords[0].UserID // Default to first landlord
		}

		var tenants []property.Tenant
		propertyDB.Where("unit_id = ?", *req.UnitID).Find(&tenants)
		if len(tenants) > 0 {
			tenantID = &tenants[0].UserID
			if userID == 0 {
				userID = tenants[0].UserID // Use tenant if no landlord
			}
		}

		// Get unit SPAs (active ones)
		var spaUnits []property.SPAUnit
		propertyDB.Where("unit_id = ? AND is_active = ?", *req.UnitID, true).Find(&spaUnits)
		if len(spaUnits) > 0 {
			spaID = &spaUnits[0].SpaUserID
			if userID == 0 {
				userID = spaUnits[0].SpaUserID // Use SPA if no landlord or tenant
			}
		}

		if userID == 0 {
			utils.BadRequestResponse(c, "Unit has no landlord, tenant, or SPA", nil)
			return
		}
	} else if req.BillType == property.BillTypeParking {
		// Get parking slot landlords and tenants
		var landlords []property.LandlordParkingSlot
		propertyDB.Where("parking_slot_id = ?", *req.ParkingSlotID).Find(&landlords)
		if len(landlords) > 0 {
			landlordID = &landlords[0].UserID
			userID = landlords[0].UserID
		}

		// Get parking assignments (tenants)
		var assignments []property.ParkingAssignment
		propertyDB.Where("parking_slot_id = ?", *req.ParkingSlotID).Find(&assignments)
		if len(assignments) > 0 {
			tenantID = &assignments[0].UserID
			if userID == 0 {
				userID = assignments[0].UserID
			}
		}

		// Get parking slot SPAs (active ones)
		var spaParkingSlots []property.SPAParkingSlot
		propertyDB.Where("parking_slot_id = ? AND is_active = ?", *req.ParkingSlotID, true).Find(&spaParkingSlots)
		if len(spaParkingSlots) > 0 {
			spaID = &spaParkingSlots[0].SpaUserID
			if userID == 0 {
				userID = spaParkingSlots[0].SpaUserID // Use SPA if no landlord or tenant
			}
		}

		if userID == 0 {
			utils.BadRequestResponse(c, "Parking slot has no owner, tenant, or SPA", nil)
			return
		}
	}

	// Set currency
	currency := req.Currency
	if currency == "" {
		currency = "PHP"
	}

	// Convert items
	billItems := make([]property.BillItem, len(req.Items))
	for i, item := range req.Items {
		billItems[i] = property.BillItem{
			ItemType:    item.ItemType,
			Description: item.Description,
			Amount:      strconv.FormatFloat(item.Amount, 'f', 2, 64),
			Currency:    currency,
		}
	}

	// Create bill
	bill := &property.Bill{
		UnitID:        req.UnitID,
		ParkingSlotID: req.ParkingSlotID,
		UserID:        userID,
		TenantID:      tenantID,
		LandlordID:    landlordID,
		SpaID:         spaID,
		BillType:      req.BillType,
		BillingMonth:  req.BillingMonth,
		Currency:      currency,
		DueDate:       dueDate,
		Status:        property.BillStatusPending,
		PaidAmount:    "0.00", // Initialize paid amount to 0
		CreatedBy:     &createdBy,
	}

	if req.Description != "" {
		bill.Description = &req.Description
	}

	err = billService.CreateBill(bill, billItems)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	// Reload bill with items
	bill, _ = billService.GetBill(bill.ID)

	utils.CreatedResponse(c, "Bill created successfully", bill)
}

// GetBill gets bill by ID
// @Summary Get bill
// @Description Get bill details by ID
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bill ID"
// @Success 200 {object} map[string]interface{}
// @Router /bills/{id} [get]
func (h *BillHandler) GetBill(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid bill ID", nil)
		return
	}

	bill, err := billService.GetBill(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Bill not found")
		return
	}

	utils.SuccessResponse(c, "Bill retrieved successfully", bill)
}

// UpdateBillRequest represents update bill request
type UpdateBillRequest struct {
	BillingMonth string           `json:"billing_month" binding:"required"` // YYYY-MM format
	Items        []BillItemRequest `json:"items" binding:"required,min=1"`    // Bill items
	DueDate      string           `json:"due_date" binding:"required"`      // YYYY-MM-DD format
	Description  string           `json:"description"`
	Currency     string           `json:"currency"` // Default: PHP
}

// UpdateBill updates a bill (only for accountant role)
// @Summary Update bill
// @Description Update an existing bill with items. Only accountant can update bills.
// @Tags Bill
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bill ID"
// @Param request body UpdateBillRequest true "Update bill request"
// @Success 200 {object} map[string]interface{}
// @Router /bills/{id} [put]
func (h *BillHandler) UpdateBill(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	// Check if user is accountant
	userRole := middleware.GetUserRole(c)
	if userRole != "property_account" && userRole != "property_admin" && userRole != "super_admin" {
		utils.UnauthorizedResponse(c, "Only accountant can update bills")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid bill ID", nil)
		return
	}

	// Get existing bill
	existingBill, err := billService.GetBill(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Bill not found")
		return
	}

	// Check if bill can be edited (cannot edit if already paid, cancelled, or confirming)
	if existingBill.Status == property.BillStatusPaid || 
	   existingBill.Status == property.BillStatusCancelled || 
	   existingBill.Status == property.BillStatusConfirming {
		utils.BadRequestResponse(c, "Cannot edit bill that is already paid, cancelled, or confirming", nil)
		return
	}

	var req UpdateBillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Validate and parse billing month (YYYY-MM format)
	_, err = time.Parse("2006-01", req.BillingMonth)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid billing month format. Use YYYY-MM", nil)
		return
	}

	// Parse due date
	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid due date format. Use YYYY-MM-DD", nil)
		return
	}

	// Set currency
	currency := req.Currency
	if currency == "" {
		currency = existingBill.Currency
		if currency == "" {
			currency = "PHP"
		}
	}

	// Convert items
	billItems := make([]property.BillItem, len(req.Items))
	for i, item := range req.Items {
		billItems[i] = property.BillItem{
			ItemType:    item.ItemType,
			Description: item.Description,
			Amount:      strconv.FormatFloat(item.Amount, 'f', 2, 64),
			Currency:    currency,
		}
	}

	// Update bill
	existingBill.BillingMonth = req.BillingMonth
	existingBill.DueDate = dueDate
	existingBill.Currency = currency
	if req.Description != "" {
		existingBill.Description = &req.Description
	} else {
		existingBill.Description = nil
	}

	err = billService.UpdateBillWithItems(existingBill, billItems)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	// Reload bill with items
	bill, _ := billService.GetBill(uint(id))

	utils.SuccessResponse(c, "Bill updated successfully", bill)
}

// ListBills lists bills with filters
// @Summary List bills
// @Description List bills with optional filters
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "User ID filter"
// @Param status query string false "Status filter (pending, paid, overdue, cancelled)"
// @Param unit_number query string false "Unit number search (fuzzy)"
// @Param parking_slot_number query string false "Parking slot number search (fuzzy)"
// @Param tenant_name query string false "Tenant name search (fuzzy)"
// @Param landlord_name query string false "Landlord name search (fuzzy)"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /bills [get]
func (h *BillHandler) ListBills(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	var userID uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, _ := strconv.ParseUint(userIDStr, 10, 32)
		userID = uint(id)
	}

	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	// Build search parameters
	searchParams := make(map[string]string)
	
	// Unit number search
	if unitNumber := c.Query("unit_number"); unitNumber != "" {
		searchParams["unit_number"] = unitNumber
	}
	
	// Parking slot number search
	if parkingSlotNumber := c.Query("parking_slot_number"); parkingSlotNumber != "" {
		searchParams["parking_slot_number"] = parkingSlotNumber
	}
	
	// Tenant name search - need to query master DB for matching user IDs
	if tenantName := c.Query("tenant_name"); tenantName != "" {
		tenantUserIDs := h.searchUsersByName(tenantName, "tenant")
		if tenantUserIDs != "" {
			searchParams["tenant_user_ids"] = tenantUserIDs
		} else {
			// No matching tenants found, return empty result
			utils.SuccessResponseWithPagination(c, []BillResponse{}, 0, page, perPage, "Bills retrieved successfully")
			return
		}
	}
	
	// Landlord name search - need to query master DB for matching user IDs
	if landlordName := c.Query("landlord_name"); landlordName != "" {
		landlordUserIDs := h.searchUsersByName(landlordName, "landlord")
		if landlordUserIDs != "" {
			searchParams["landlord_user_ids"] = landlordUserIDs
		} else {
			// No matching landlords found, return empty result
			utils.SuccessResponseWithPagination(c, []BillResponse{}, 0, page, perPage, "Bills retrieved successfully")
			return
		}
	}

	bills, total, err := billService.ListBills(userID, status, page, perPage, searchParams)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve bills", err)
		return
	}

	// Build response with user names
	responses := h.buildBillResponses(bills)

	utils.SuccessResponseWithPagination(c, responses, total, page, perPage, "Bills retrieved successfully")
}

// searchUsersByName searches for users by name in master DB and returns comma-separated user IDs
func (h *BillHandler) searchUsersByName(name string, roleFilter string) string {
	masterDB := database.GetMasterGormDB()
	if masterDB == nil {
		return ""
	}

	searchPattern := "%" + name + "%"
	var users []struct {
		ID uint
	}

	// Query users table for matching names
	query := masterDB.Model(&master.User{}).
		Where("full_name LIKE ? OR email LIKE ?", searchPattern, searchPattern).
		Select("id")

	if err := query.Find(&users).Error; err != nil {
		return ""
	}

	if len(users) == 0 {
		return ""
	}

	// Convert to comma-separated string
	result := ""
	for i, user := range users {
		if i > 0 {
			result += ","
		}
		result += strconv.FormatUint(uint64(user.ID), 10)
	}
	return result
}

// GetMyBills gets current user's bills
// @Summary Get my bills
// @Description Get bills for current authenticated user
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(20)
// @Success 200 {object} map[string]interface{}
// @Router /bills/my [get]
func (h *BillHandler) GetMyBills(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	bills, total, err := billService.ListUserBills(userID, page, perPage)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve bills", err)
		return
	}

	utils.SuccessResponseWithPagination(c, bills, total, page, perPage, "Bills retrieved successfully")
}

// ListOverdueBills lists overdue bills
// @Summary List overdue bills
// @Description List all overdue bills
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /bills/overdue [get]
func (h *BillHandler) ListOverdueBills(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	bills, err := billService.ListOverdueBills()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve overdue bills", err)
		return
	}

	utils.SuccessResponse(c, "Overdue bills retrieved successfully", bills)
}

// ListDueSoonBills lists bills due soon
// @Summary List bills due soon
// @Description List bills due within specified days
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param days query int false "Number of days" default(7)
// @Success 200 {object} map[string]interface{}
// @Router /bills/due-soon [get]
func (h *BillHandler) ListDueSoonBills(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))

	bills, err := billService.ListDueSoonBills(days)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve bills", err)
		return
	}

	utils.SuccessResponse(c, "Bills due soon retrieved successfully", bills)
}

// MarkBillAsPaid marks a bill as paid
// @Summary Mark bill as paid
// @Description Mark a bill as paid. Only allows transition from 'confirming' to 'paid'. Pending bills must complete payment first.
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bill ID"
// @Success 200 {object} map[string]interface{}
// @Router /bills/{id}/pay [post]
func (h *BillHandler) MarkBillAsPaid(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid bill ID", nil)
		return
	}

	// Get the bill first to check its current status
	bill, err := billService.GetBill(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Bill not found")
		return
	}

	// Only allow marking as paid if status is confirming
	// Pending bills must complete payment first
	if bill.Status != property.BillStatusConfirming {
		utils.BadRequestResponse(c, fmt.Sprintf("Cannot mark bill as paid. Current status is '%s'. Only bills with status 'confirming' can be marked as paid. Pending bills must complete payment first.", bill.Status), nil)
		return
	}

	err = billService.MarkBillAsPaid(uint(id))
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Bill marked as paid successfully", nil)
}

// CancelBill cancels a bill
// @Summary Cancel bill
// @Description Cancel a bill
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bill ID"
// @Success 200 {object} map[string]interface{}
// @Router /bills/{id}/cancel [post]
func (h *BillHandler) CancelBill(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid bill ID", nil)
		return
	}

	err = billService.CancelBill(uint(id))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to cancel bill", err)
		return
	}

	utils.SuccessResponse(c, "Bill cancelled successfully", nil)
}

// GetMyTotalDue gets current user's total amount due
// @Summary Get my total due
// @Description Get total amount due for current user
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /bills/my/total-due [get]
func (h *BillHandler) GetMyTotalDue(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	userID := middleware.GetUserID(c)

	total, err := billService.GetUserTotalDue(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve total due", err)
		return
	}

	utils.SuccessResponse(c, "Total due retrieved successfully", gin.H{
		"total_due": total,
	})
}

// GetStatistics gets bill statistics
// @Summary Get bill statistics
// @Description Get statistics about bills
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /bills/statistics [get]
func (h *BillHandler) GetStatistics(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	stats, err := billService.GetStatistics()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to retrieve statistics", err)
		return
	}

	utils.SuccessResponse(c, "Statistics retrieved successfully", stats)
}

// SubmitPayment submits a payment for a bill (supports multipart form for receipt upload)
// @Summary Submit payment for bill
// @Description Submit a payment for a bill. Supports file upload for "other" payment method.
// @Tags Bill
// @Accept json,multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bill ID"
// @Param payment_method formData string true "Payment method (gcash, paymaya, credit_card, bank_transfer, other)"
// @Param amount formData string false "Payment amount (defaults to bill amount)"
// @Param reference_number formData string false "Reference number"
// @Param notes formData string false "Notes"
// @Param receipt formData file false "Receipt file (required for 'other' method)"
// @Success 201 {object} map[string]interface{}
// @Router /bills/{id}/payments [post]
func (h *BillHandler) SubmitPayment(c *gin.Context) {
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.InternalServerErrorResponse(c, "Property database not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	if userID == 0 {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid bill ID", nil)
		return
	}

	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	// Get bill
	bill, err := billService.GetBill(uint(id))
	if err != nil {
		utils.NotFoundResponse(c, "Bill not found")
		return
	}

	// Check if user owns this bill (user_id, landlord_id, tenant_id, or spa_id)
	if bill.UserID != userID && 
		(bill.LandlordID == nil || *bill.LandlordID != userID) && 
		(bill.TenantID == nil || *bill.TenantID != userID) && 
		(bill.SpaID == nil || *bill.SpaID != userID) {
		utils.ForbiddenResponse(c, "You can only pay your own bills")
		return
	}

	// Check if bill is already paid
	if bill.Status == property.BillStatusPaid {
		utils.BadRequestResponse(c, "Bill is already paid", nil)
		return
	}

	// Parse request based on content type
	var paymentMethod, amount, referenceNumber, notes string
	var receiptFile *string

	contentType := c.GetHeader("Content-Type")
	if len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Multipart form
		paymentMethod = c.PostForm("payment_method")
		amount = c.PostForm("amount")
		referenceNumber = c.PostForm("reference_number")
		notes = c.PostForm("notes")

		// Handle file upload for "other" payment method
		if paymentMethod == property.PaymentMethodOther {
			file, err := c.FormFile("receipt")
			if err != nil || file == nil {
				utils.BadRequestResponse(c, "Receipt file is required for 'other' payment method", nil)
				return
			}

			// Validate file size (max 10MB)
			if file.Size > 10*1024*1024 {
				utils.BadRequestResponse(c, "File size exceeds 10MB limit", nil)
				return
			}

			// Validate file type (images and PDF)
			allowedTypes := map[string]bool{
				"image/jpeg":      true,
				"image/png":       true,
				"image/gif":       true,
				"image/webp":      true,
				"application/pdf": true,
			}
			mimeType := file.Header.Get("Content-Type")
			if !allowedTypes[mimeType] {
				utils.BadRequestResponse(c, "Invalid file type. Only images and PDF are allowed", nil)
				return
			}

			// Create upload directory
			uploadDir := fmt.Sprintf("./uploads/payments/%d", bill.ID)
			if err := os.MkdirAll(uploadDir, 0755); err != nil {
				utils.InternalServerErrorResponse(c, "Failed to create upload directory", err)
				return
			}

			// Generate unique filename
			ext := filepath.Ext(file.Filename)
			filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitizePaymentFilename(file.Filename), ext)
			filePath := filepath.Join(uploadDir, filename)

			// Save file
			src, err := file.Open()
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to open uploaded file", err)
				return
			}
			defer src.Close()

			dst, err := os.Create(filePath)
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to save file", err)
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				utils.InternalServerErrorResponse(c, "Failed to save file", err)
				return
			}

			receiptFile = &filePath

			// Create document record
			fileSize := file.Size
			doc := property.Document{
				EntityType:   property.DocEntityPayment,
				EntityID:     0, // Will be set after payment is created
				DocumentType: property.DocTypePaymentReceipt,
				DocumentName: file.Filename,
				DocumentPath: filePath,
				FileSize:     &fileSize,
				MimeType:     &mimeType,
				UploadedBy:   userID,
				IsActive:     true,
			}
			propertyDB.Create(&doc)
		}
	} else {
		// JSON
		var req struct {
			PaymentMethod   string `json:"payment_method" binding:"required"`
			Amount          string `json:"amount"`
			ReferenceNumber string `json:"reference_number"`
			Notes           string `json:"notes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.ValidationErrorResponse(c, err)
			return
		}
		paymentMethod = req.PaymentMethod
		amount = req.Amount
		referenceNumber = req.ReferenceNumber
		notes = req.Notes

		// For JSON requests with "other" method, receipt file is required but must be uploaded separately
		if paymentMethod == property.PaymentMethodOther {
			utils.BadRequestResponse(c, "Receipt file upload is required for 'other' payment method. Please use multipart/form-data", nil)
			return
		}
	}

	// Validate payment method
	validMethods := map[string]bool{
		property.PaymentMethodGCash:        true,
		property.PaymentMethodPayMaya:      true,
		property.PaymentMethodCreditCard:   true,
		property.PaymentMethodDebitCard:    true,
		property.PaymentMethodBankTransfer: true,
		property.PaymentMethodOther:        true,
	}
	if !validMethods[paymentMethod] {
		utils.BadRequestResponse(c, "Invalid payment method", nil)
		return
	}

	// Use bill amount if not specified
	if amount == "" {
		amount = bill.Amount
	}

	// Generate payment number
	paymentNumber := fmt.Sprintf("PAY-%s-%d", time.Now().Format("20060102"), time.Now().Unix())

	// Create payment
	payment := &property.Payment{
		PaymentNumber:   paymentNumber,
		BillID:          bill.ID,
		UserID:          userID,
		Amount:          amount,
		Currency:        bill.Currency,
		PaymentMethod:   paymentMethod,
		PaymentDate:     time.Now(),
		Status:          property.PaymentStatusPending,
		ReceiptFile:     receiptFile,
	}

	if referenceNumber != "" {
		payment.ReferenceNumber = &referenceNumber
	}
	if notes != "" {
		payment.Notes = &notes
	}

	// Save payment first
	if err := propertyDB.Create(payment).Error; err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create payment", err)
		return
	}

	// If payment method is "other" (payment proof), update bill status to confirming
	fmt.Printf("[SubmitPayment] Payment method: '%s', Expected: '%s'\n", paymentMethod, property.PaymentMethodOther)
	if paymentMethod == property.PaymentMethodOther {
		fmt.Printf("[SubmitPayment] Updating bill %d status from '%s' to '%s'\n", bill.ID, bill.Status, property.BillStatusConfirming)
		
		// Use Updates with map to ensure the update happens
		updateData := map[string]interface{}{
			"status": property.BillStatusConfirming,
		}
		updateResult := propertyDB.Model(&property.Bill{}).
			Where("id = ?", bill.ID).
			Updates(updateData)
		
		if updateResult.Error != nil {
			fmt.Printf("[SubmitPayment] ERROR: Failed to update bill status: %v\n", updateResult.Error)
		} else {
			fmt.Printf("[SubmitPayment] Update result - Rows affected: %d, Error: %v\n", updateResult.RowsAffected, updateResult.Error)
			if updateResult.RowsAffected == 0 {
				fmt.Printf("[SubmitPayment] WARNING: No rows updated for bill ID %d. Current bill status: %s\n", bill.ID, bill.Status)
				// Try to reload bill to check current status
				var currentBill property.Bill
				if err := propertyDB.First(&currentBill, bill.ID).Error; err == nil {
					fmt.Printf("[SubmitPayment] Current bill status in DB: %s\n", currentBill.Status)
				}
			} else {
				fmt.Printf("[SubmitPayment] SUCCESS: Updated bill %d status to '%s'\n", bill.ID, property.BillStatusConfirming)
			}
		}
	} else {
		fmt.Printf("[SubmitPayment] Payment method '%s' is not 'other', skipping status update\n", paymentMethod)
	}

	// Update document entity_id if receipt was uploaded
	if receiptFile != nil {
		propertyDB.Model(&property.Document{}).
			Where("entity_type = ? AND entity_id = ? AND document_path = ?", property.DocEntityPayment, 0, *receiptFile).
			Update("entity_id", payment.ID)
	}

	utils.CreatedResponse(c, "Payment submitted successfully", payment)
}

// BillResponse represents a bill in API response with user info
type BillResponse struct {
	property.Bill
	UserName      string `json:"user_name,omitempty"`
	UnitNumber    string `json:"unit_number,omitempty"`
	PaymentMethod string `json:"payment_method,omitempty"`
}

// buildBillResponses builds bill responses with user names and payment methods
func (h *BillHandler) buildBillResponses(bills []property.Bill) []BillResponse {
	masterDB := database.GetMasterGormDB()
	responses := make([]BillResponse, len(bills))

	for i, bill := range bills {
		resp := BillResponse{
			Bill: bill,
		}

		// Get user name from master DB
		if masterDB != nil {
			var user master.User
			if masterDB.First(&user, bill.UserID).Error == nil {
				resp.UserName = user.FullName
			}
		}

		// Get unit number
		if bill.Unit.UnitNumber != "" {
			resp.UnitNumber = bill.Unit.UnitNumber
		}

		// Get latest payment method (from most recent payment)
		if len(bill.Payments) > 0 {
			// Find the most recent payment (by created_at)
			latestPayment := bill.Payments[0]
			for _, payment := range bill.Payments {
				if payment.CreatedAt.After(latestPayment.CreatedAt) {
					latestPayment = payment
				}
			}
			resp.PaymentMethod = latestPayment.PaymentMethod
		}

		responses[i] = resp
	}

	return responses
}

// sanitizePaymentFilename removes special characters from filename
func sanitizePaymentFilename(name string) string {
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]

	result := make([]byte, 0, len(base))
	for i := 0; i < len(base); i++ {
		c := base[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "receipt"
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result)
}

// PendingBillResponse represents a pending bill in API response
type PendingBillResponse struct {
	ID                uint   `json:"id"`
	BillNumber        string `json:"bill_number"`
	BillType          string `json:"bill_type"`
	Amount            string `json:"amount"`
	DueDate           string `json:"due_date"`
	Status            string `json:"status"`
	UnitNumber        string `json:"unit_number,omitempty"`
	ParkingSlotNumber string `json:"parking_slot_number,omitempty"`
	BillingMonth      string `json:"billing_month"`
	DaysUntilDue      int    `json:"days_until_due"` // Negative if overdue
	IsOverdue         bool   `json:"is_overdue"`
	UserName          string `json:"user_name,omitempty"` // Only for admin/accountant
}

// GetPendingBillsCount gets count of pending/overdue bills
// @Summary Get pending bills count
// @Description Get count of pending/overdue bills for current user (or all overdue for admin/accountant)
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /bills/pending-count [get]
func (h *BillHandler) GetPendingBillsCount(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	totalCount, overdueCount, err := billService.GetPendingBillsCount(userID, userRole)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get pending bills count", err)
		return
	}

	pendingCount := totalCount - overdueCount
	if pendingCount < 0 {
		pendingCount = 0
	}

	utils.SuccessResponse(c, "Pending bills count retrieved successfully", gin.H{
		"count":         totalCount,
		"overdue_count": overdueCount,
		"pending_count": pendingCount,
	})
}

// GetPendingBills gets list of pending/overdue bills
// @Summary Get pending bills
// @Description Get list of pending/overdue bills for current user (or all overdue for admin/accountant)
// @Tags Bill
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit number of results (default: 10, max: 20)"
// @Success 200 {object} map[string]interface{}
// @Router /bills/pending [get]
func (h *BillHandler) GetPendingBills(c *gin.Context) {
	billService := h.getBillService(c)
	if billService == nil {
		utils.InternalServerErrorResponse(c, "Property context not found", nil)
		return
	}

	userID := middleware.GetUserID(c)
	userRole := middleware.GetUserRole(c)

	// Parse limit parameter
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 20 {
			limit = parsedLimit
		}
	}

	// Get only overdue bill IDs
	billIDs, err := billService.GetOverdueBillIDs(userID, userRole, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get overdue bills", err)
		return
	}

	utils.SuccessResponse(c, "Overdue bills retrieved successfully", gin.H{
		"bill_ids": billIDs,
		"total":    len(billIDs),
	})
}

// buildPendingBillResponses builds pending bill responses with additional computed fields
func (h *BillHandler) buildPendingBillResponses(bills []property.Bill, userRole string) []PendingBillResponse {
	masterDB := database.GetMasterGormDB()
	responses := make([]PendingBillResponse, len(bills))
	now := time.Now()

	// Fetch user names in batch for admin/accountant
	userMap := make(map[uint]string)
	if (userRole == "property_admin" || userRole == "property_account") && masterDB != nil {
		userIDs := make([]uint, 0)
		for _, bill := range bills {
			userIDs = append(userIDs, bill.UserID)
		}
		var users []master.User
		if len(userIDs) > 0 {
			masterDB.Where("id IN ?", userIDs).Find(&users)
			for _, user := range users {
				userMap[user.ID] = user.GetDisplayName()
			}
		}
	}

	for i, bill := range bills {
		resp := PendingBillResponse{
			ID:           bill.ID,
			BillNumber:   bill.BillNumber,
			BillType:     bill.BillType,
			Amount:       bill.Amount,
			DueDate:      bill.DueDate.Format(time.RFC3339),
			Status:       bill.Status,
			BillingMonth: bill.BillingMonth,
		}

		// Add unit or parking slot number
		if bill.Unit.UnitNumber != "" {
			resp.UnitNumber = bill.Unit.UnitNumber
		}
		if bill.ParkingSlot != nil && bill.ParkingSlot.SlotNumber != "" {
			resp.ParkingSlotNumber = bill.ParkingSlot.SlotNumber
		}

		// Calculate days until due
		daysUntilDue := int(bill.DueDate.Sub(now).Hours() / 24)
		resp.DaysUntilDue = daysUntilDue
		resp.IsOverdue = bill.Status == property.BillStatusOverdue || (bill.Status == property.BillStatusPending && bill.DueDate.Before(now))

		// Add user name for admin/accountant
		if userRole == "property_admin" || userRole == "property_account" {
			if userName, ok := userMap[bill.UserID]; ok {
				resp.UserName = userName
			}
		}

		responses[i] = resp
	}

	return responses
}
