package service

import (
	"fmt"
	"strings"
	"time"

	"homexai/internal/models/property"
	propertyRepo "homexai/internal/repository/property"

	"gorm.io/gorm"
)

type BillService struct {
	propertyDB *gorm.DB
}

func NewBillService(propertyDB *gorm.DB) *BillService {
	return &BillService{
		propertyDB: propertyDB,
	}
}

// CreateBill creates a new bill with items
func (s *BillService) CreateBill(bill *property.Bill, items []property.BillItem) error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	itemRepo := propertyRepo.NewBillItemRepository(s.propertyDB)

	// Generate bill number if not provided
	if bill.BillNumber == "" {
		var number string
		if bill.UnitID != nil && *bill.UnitID > 0 {
			// Get unit number
			var unit property.Unit
			if err := s.propertyDB.First(&unit, *bill.UnitID).Error; err == nil {
				number = unit.UnitNumber
			}
		} else if bill.ParkingSlotID != nil && *bill.ParkingSlotID > 0 {
			// Get parking slot number
			var parkingSlot property.ParkingSlot
			if err := s.propertyDB.First(&parkingSlot, *bill.ParkingSlotID).Error; err == nil {
				number = parkingSlot.SlotNumber
			}
		}
		bill.BillNumber = s.generateBillNumber(number, bill.BillingMonth)
	}

	// Set issue date if not provided
	if bill.IssueDate.IsZero() {
		bill.IssueDate = time.Now()
	}

	// Initialize amount and paid_amount if not set
	if bill.Amount == "" {
		bill.Amount = "0.00"
	}
	if bill.PaidAmount == "" {
		bill.PaidAmount = "0.00"
	}

	// Calculate total amount from items if provided
	if len(items) > 0 {
		var total float64
		for _, item := range items {
			var amount float64
			fmt.Sscanf(item.Amount, "%f", &amount)
			total += amount
		}
		bill.Amount = fmt.Sprintf("%.2f", total)
	}

	// Create bill
	if err := repo.Create(bill); err != nil {
		return err
	}

	// Create bill items
	if len(items) > 0 {
		for i := range items {
			items[i].BillID = bill.ID
			items[i].Currency = bill.Currency
		}
		if err := itemRepo.CreateBatch(items); err != nil {
			// Rollback bill creation if items fail
			repo.Delete(bill.ID)
			return fmt.Errorf("failed to create bill items: %v", err)
		}
	}

	return nil
}

// GetBill gets bill by ID with items
func (s *BillService) GetBill(id uint) (*property.Bill, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	itemRepo := propertyRepo.NewBillItemRepository(s.propertyDB)

	bill, err := repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// Load items
	items, err := itemRepo.FindByBillID(id)
	if err == nil {
		bill.Items = items
	}

	return bill, nil
}

// UpdateBill updates a bill
func (s *BillService) UpdateBill(bill *property.Bill) error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.Update(bill)
}

// UpdateBillWithItems updates a bill and its items
func (s *BillService) UpdateBillWithItems(bill *property.Bill, items []property.BillItem) error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	itemRepo := propertyRepo.NewBillItemRepository(s.propertyDB)

	// Calculate total amount from items if provided
	if len(items) > 0 {
		var total float64
		for _, item := range items {
			var amount float64
			fmt.Sscanf(item.Amount, "%f", &amount)
			total += amount
		}
		bill.Amount = fmt.Sprintf("%.2f", total)
	}

	// Update bill
	if err := repo.Update(bill); err != nil {
		return err
	}

	// Delete existing items
	if err := itemRepo.DeleteByBillID(bill.ID); err != nil {
		return fmt.Errorf("failed to delete existing bill items: %v", err)
	}

	// Create new items
	if len(items) > 0 {
		for i := range items {
			items[i].BillID = bill.ID
			items[i].Currency = bill.Currency
		}
		if err := itemRepo.CreateBatch(items); err != nil {
			return fmt.Errorf("failed to create bill items: %v", err)
		}
	}

	return nil
}

// DeleteBill deletes a bill
func (s *BillService) DeleteBill(id uint) error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.Delete(id)
}

// ListBills lists bills with filters and pagination
// searchParams can contain: unit_number, parking_slot_number, tenant_user_ids, landlord_user_ids
// tenant_user_ids and landlord_user_ids should be comma-separated user IDs from master DB
func (s *BillService) ListBills(userID uint, status string, page, perPage int, searchParams map[string]string) ([]property.Bill, int64, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.List(userID, status, page, perPage, searchParams)
}

// ListUserBills lists bills for a specific user (includes bills where user is landlord or tenant)
func (s *BillService) ListUserBills(userID uint, page, perPage int) ([]property.Bill, int64, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	itemRepo := propertyRepo.NewBillItemRepository(s.propertyDB)

	bills, total, err := repo.List(userID, "", page, perPage, make(map[string]string))
	if err != nil {
		return nil, 0, err
	}

	// Load items for each bill
	for i := range bills {
		items, err := itemRepo.FindByBillID(bills[i].ID)
		if err == nil {
			bills[i].Items = items
		}
	}

	return bills, total, nil
}

// ListUnitBills lists bills for a unit
func (s *BillService) ListUnitBills(unitID uint) ([]property.Bill, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.ListByUnit(unitID)
}

// ListOverdueBills lists overdue bills
func (s *BillService) ListOverdueBills() ([]property.Bill, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.ListOverdue()
}

// ListDueSoonBills lists bills due within specified days
func (s *BillService) ListDueSoonBills(days int) ([]property.Bill, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.ListDueSoon(days)
}

// MarkBillAsPaid marks a bill as paid
func (s *BillService) MarkBillAsPaid(id uint) error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.MarkAsPaid(id)
}

// CancelBill cancels a bill
func (s *BillService) CancelBill(id uint) error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.UpdateStatus(id, property.BillStatusCancelled)
}

// GetUserTotalDue gets total amount due for a user
func (s *BillService) GetUserTotalDue(userID uint) (float64, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	return repo.GetTotalAmount(userID, property.BillStatusPending)
}

// GetStatistics gets bill statistics
// Returns statistics grouped by unit and parking slot
func (s *BillService) GetStatistics() (map[string]interface{}, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)

	// Get total count
	var totalCount int64
	if err := s.propertyDB.Model(&property.Bill{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}

	// Get counts by status
	totalPending, err := repo.CountByStatus(property.BillStatusPending)
	if err != nil {
		return nil, err
	}

	totalPaid, err := repo.CountByStatus(property.BillStatusPaid)
	if err != nil {
		return nil, err
	}

	// Get overdue count (pending bills with due date passed)
	now := time.Now()
	var overdueCount int64
	if err := s.propertyDB.Model(&property.Bill{}).
		Where("status = ? AND due_date < ?", property.BillStatusPending, now).
		Count(&overdueCount).Error; err != nil {
		return nil, err
	}

	// Get total amount due (pending bills)
	totalDue, err := repo.GetTotalAmount(0, property.BillStatusPending)
	if err != nil {
		return nil, err
	}

	pendingAmount, err := repo.GetTotalAmount(0, property.BillStatusPending)
	if err != nil {
		return nil, err
	}

	paidAmount, err := repo.GetTotalAmount(0, property.BillStatusPaid)
	if err != nil {
		return nil, err
	}

	// Calculate collection rate
	var collectionRate float64
	if paidAmount+pendingAmount > 0 {
		collectionRate = paidAmount / (paidAmount + pendingAmount) * 100
	}

	// Get statistics by bill type (unit vs parking)
	unitStats, err := s.getStatisticsByBillType(property.BillTypeUnit)
	if err != nil {
		return nil, err
	}

	parkingStats, err := s.getStatisticsByBillType(property.BillTypeParking)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":           totalCount,
		"pending":         totalPending,
		"paid":            totalPaid,
		"overdue":         overdueCount, // Use actual overdue count (pending + past due date)
		"total_due":       totalDue,
		"pending_amount":  pendingAmount,
		"paid_amount":     paidAmount,
		"collection_rate": collectionRate,
		"by_type": map[string]interface{}{
			"unit":    unitStats,
			"parking": parkingStats,
		},
	}, nil
}

// getStatisticsByBillType gets statistics for a specific bill type (unit or parking)
func (s *BillService) getStatisticsByBillType(billType string) (map[string]interface{}, error) {
	now := time.Now()

	// Get total count for this bill type
	var totalCount int64
	if err := s.propertyDB.Model(&property.Bill{}).
		Where("bill_type = ?", billType).
		Count(&totalCount).Error; err != nil {
		return nil, err
	}

	// Get counts by status for this bill type
	var totalPending, totalPaid, overdueCount int64

	if err := s.propertyDB.Model(&property.Bill{}).
		Where("bill_type = ? AND status = ?", billType, property.BillStatusPending).
		Count(&totalPending).Error; err != nil {
		return nil, err
	}

	if err := s.propertyDB.Model(&property.Bill{}).
		Where("bill_type = ? AND status = ?", billType, property.BillStatusPaid).
		Count(&totalPaid).Error; err != nil {
		return nil, err
	}

	// Get overdue count for this bill type
	if err := s.propertyDB.Model(&property.Bill{}).
		Where("bill_type = ? AND status = ? AND due_date < ?", billType, property.BillStatusPending, now).
		Count(&overdueCount).Error; err != nil {
		return nil, err
	}

	// Get total amounts for this bill type
	var pendingAmount, paidAmount, totalDue float64

	if err := s.propertyDB.Model(&property.Bill{}).
		Where("bill_type = ? AND status = ?", billType, property.BillStatusPending).
		Select("COALESCE(SUM(CAST(amount AS DECIMAL(10,2))), 0)").
		Scan(&pendingAmount).Error; err != nil {
		return nil, err
	}

	if err := s.propertyDB.Model(&property.Bill{}).
		Where("bill_type = ? AND status = ?", billType, property.BillStatusPaid).
		Select("COALESCE(SUM(CAST(amount AS DECIMAL(10,2))), 0)").
		Scan(&paidAmount).Error; err != nil {
		return nil, err
	}

	totalDue = pendingAmount

	// Calculate collection rate for this bill type
	var collectionRate float64
	if paidAmount+pendingAmount > 0 {
		collectionRate = paidAmount / (paidAmount + pendingAmount) * 100
	}

	return map[string]interface{}{
		"total":           totalCount,
		"pending":         totalPending,
		"paid":            totalPaid,
		"overdue":         overdueCount,
		"total_due":       totalDue,
		"pending_amount":  pendingAmount,
		"paid_amount":     paidAmount,
		"collection_rate": collectionRate,
	}, nil
}

// generateBillNumber generates a bill number using unit_number/parking_slot_number + yyyy_mm
func (s *BillService) generateBillNumber(number string, billingMonth string) string {
	if number == "" {
		// Fallback to timestamp-based number if number is empty
		now := time.Now()
		return fmt.Sprintf("BILL-%s-%d", now.Format("200601"), now.Unix())
	}
	
	// Format billing month (YYYY-MM) to YYYY_MM
	monthStr := billingMonth
	if monthStr != "" {
		// Replace - with _
		monthStr = strings.ReplaceAll(monthStr, "-", "_")
	} else {
		// Fallback to current month if billingMonth is empty
		now := time.Now()
		monthStr = fmt.Sprintf("%d_%02d", now.Year(), now.Month())
	}
	
	return fmt.Sprintf("%s_%s", number, monthStr)
}

// UpdateOverdueBills updates status of overdue bills
func (s *BillService) UpdateOverdueBills() error {
	repo := propertyRepo.NewBillRepository(s.propertyDB)

	// Get all pending bills
	bills, _, err := repo.List(0, property.BillStatusPending, 1, 1000, make(map[string]string))
	if err != nil {
		return err
	}

	// Update overdue bills
	for _, bill := range bills {
		if bill.IsOverdue() {
			repo.UpdateStatus(bill.ID, property.BillStatusOverdue)
		}
	}

	return nil
}

// CalculateServiceFeeForUnit calculates total service fee from completed marketplace orders for a unit
// This should be called when creating a unit bill to auto-add service fee item
func (s *BillService) CalculateServiceFeeForUnit(unitID uint, startDate, endDate time.Time) (float64, error) {
	orderRepo := propertyRepo.NewServiceOrderRepository(s.propertyDB)

	// Get completed service orders for this unit within date range
	orders, err := orderRepo.ListByUnit(unitID)
	if err != nil {
		return 0, err
	}

	var totalFee float64
	for _, order := range orders {
		// Only count property-owned services that are completed
		// Check if ServiceListing is loaded (ID > 0) and is property-owned
		if order.ServiceListing.ID > 0 &&
			order.ServiceListing.IsPropertyOwnedService() &&
			order.Status == property.ServiceOrderStatusCompleted &&
			order.CompletedAt != nil &&
			!order.CompletedAt.Before(startDate) &&
			!order.CompletedAt.After(endDate) {
			var price float64
			fmt.Sscanf(order.ServiceListing.Price, "%f", &price)
			totalFee += price
		}
	}

	return totalFee, nil
}

// GetPendingBillsCount gets count of pending/overdue bills based on user role
// Returns: totalCount, overdueCount, error
func (s *BillService) GetPendingBillsCount(userID uint, userRole string) (int, int, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	
	// For admin/accountant: count all overdue bills
	if userRole == "property_admin" || userRole == "property_account" {
		count, err := repo.CountOverdueAll()
		if err != nil {
			return 0, 0, err
		}
		return int(count), int(count), nil
	}
	
	// For tenant/landlord/SPA: count their own pending and overdue bills
	pendingCount, overdueCount, err := repo.CountPendingByUserID(userID)
	if err != nil {
		return 0, 0, err
	}
	
	totalCount := pendingCount + overdueCount
	return int(totalCount), int(overdueCount), nil
}

// GetPendingBills gets list of pending/overdue bills based on user role
func (s *BillService) GetPendingBills(userID uint, userRole string, limit int) ([]property.Bill, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	
	// For admin/accountant: get all overdue bills
	if userRole == "property_admin" || userRole == "property_account" {
		return repo.ListOverdueAll(limit)
	}
	
	// For tenant/landlord/SPA: get their own pending and overdue bills
	return repo.ListPendingByUserID(userID, limit)
}

// GetOverdueBillIDs gets list of overdue bill IDs based on user role
func (s *BillService) GetOverdueBillIDs(userID uint, userRole string, limit int) ([]uint, error) {
	repo := propertyRepo.NewBillRepository(s.propertyDB)
	
	// For admin/accountant: get all overdue bill IDs
	if userRole == "property_admin" || userRole == "property_account" {
		return repo.GetOverdueBillIDs(limit)
	}
	
	// For tenant/landlord/SPA: get their own overdue bill IDs
	return repo.GetOverdueBillIDsByUserID(userID, limit)
}
