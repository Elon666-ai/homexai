package property

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type BillRepository struct {
	db *gorm.DB
}

func NewBillRepository(db *gorm.DB) *BillRepository {
	return &BillRepository{db: db}
}

// Create creates a new bill
func (r *BillRepository) Create(bill *property.Bill) error {
	return r.db.Create(bill).Error
}

// FindByID finds a bill by ID
func (r *BillRepository) FindByID(id uint) (*property.Bill, error) {
	var bill property.Bill
	err := r.db.Preload("Unit").Preload("ParkingSlot").Preload("Payments").First(&bill, id).Error
	if err != nil {
		return nil, err
	}
	return &bill, nil
}

// FindByNumber finds a bill by bill number
func (r *BillRepository) FindByNumber(billNumber string) (*property.Bill, error) {
	var bill property.Bill
	err := r.db.Where("bill_number = ?", billNumber).First(&bill).Error
	if err != nil {
		return nil, err
	}
	return &bill, nil
}

// Update updates a bill
func (r *BillRepository) Update(bill *property.Bill) error {
	return r.db.Save(bill).Error
}

// Delete deletes a bill
func (r *BillRepository) Delete(id uint) error {
	return r.db.Delete(&property.Bill{}, id).Error
}

// List lists bills with pagination and filters
// For landlord/tenant users, includes bills where they are landlord_id or tenant_id
// searchParams contains search criteria: unit_number, parking_slot_number, tenant_name, landlord_name
func (r *BillRepository) List(userID uint, status string, page, perPage int, searchParams map[string]string) ([]property.Bill, int64, error) {
	var bills []property.Bill
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.Bill{})

	if userID != 0 {
		// Include bills where user is the payer, landlord, tenant, or SPA
		query = query.Where("user_id = ? OR landlord_id = ? OR tenant_id = ? OR spa_id = ?", userID, userID, userID, userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Apply search filters
	if unitNumber, ok := searchParams["unit_number"]; ok && unitNumber != "" {
		query = query.Joins("LEFT JOIN units ON units.id = bills.unit_id").
			Where("units.unit_number LIKE ?", "%"+unitNumber+"%")
	}

	if parkingSlotNumber, ok := searchParams["parking_slot_number"]; ok && parkingSlotNumber != "" {
		query = query.Joins("LEFT JOIN parking_slots ON parking_slots.id = bills.parking_slot_id").
			Where("parking_slots.slot_number LIKE ?", "%"+parkingSlotNumber+"%")
	}

	// For tenant_name and landlord_name, we need to filter by user IDs
	// These will be handled by the service layer which has access to master DB
	if tenantUserIDs, ok := searchParams["tenant_user_ids"]; ok && tenantUserIDs != "" {
		// tenant_user_ids is a comma-separated list of user IDs
		// Parse comma-separated string to slice
		var ids []uint
		parts := strings.Split(tenantUserIDs, ",")
		for _, part := range parts {
			if id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32); err == nil {
				ids = append(ids, uint(id))
			}
		}
		if len(ids) > 0 {
			query = query.Where("tenant_id IN (?)", ids)
		} else {
			// No valid IDs, return empty result
			query = query.Where("1 = 0")
		}
	}

	if landlordUserIDs, ok := searchParams["landlord_user_ids"]; ok && landlordUserIDs != "" {
		// landlord_user_ids is a comma-separated list of user IDs
		// Parse comma-separated string to slice
		var ids []uint
		parts := strings.Split(landlordUserIDs, ",")
		for _, part := range parts {
			if id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32); err == nil {
				ids = append(ids, uint(id))
			}
		}
		if len(ids) > 0 {
			query = query.Where("landlord_id IN (?)", ids)
		} else {
			// No valid IDs, return empty result
			query = query.Where("1 = 0")
		}
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Preload("Unit").Preload("ParkingSlot").Preload("Payments").Offset(offset).Limit(perPage).
		Order("due_date DESC").Find(&bills).Error
	return bills, total, err
}

// ListByUnit lists bills for a unit
func (r *BillRepository) ListByUnit(unitID uint) ([]property.Bill, error) {
	var bills []property.Bill
	err := r.db.Where("unit_id = ?", unitID).
		Order("due_date DESC").Find(&bills).Error
	return bills, err
}

// ListOverdue lists overdue bills
func (r *BillRepository) ListOverdue() ([]property.Bill, error) {
	var bills []property.Bill
	err := r.db.Where("status = ? AND due_date < ?",
		property.BillStatusPending, time.Now()).
		Preload("Unit").Find(&bills).Error
	return bills, err
}

// ListDueSoon lists bills due within specified days
func (r *BillRepository) ListDueSoon(days int) ([]property.Bill, error) {
	var bills []property.Bill
	dueDate := time.Now().AddDate(0, 0, days)
	err := r.db.Where("status = ? AND due_date BETWEEN ? AND ?",
		property.BillStatusPending, time.Now(), dueDate).
		Preload("Unit").Find(&bills).Error
	return bills, err
}

// MarkAsPaid marks a bill as paid
// For pending status: only marks as paid if there is at least one completed payment
// For confirming status: can be marked as paid directly
func (r *BillRepository) MarkAsPaid(id uint) error {
	// Check if bill exists
	var bill property.Bill
	if err := r.db.First(&bill, id).Error; err != nil {
		return err
	}

	// Only allow marking confirming status as paid directly
	if bill.Status != property.BillStatusConfirming {
		return fmt.Errorf("cannot mark bill as paid. Current status is '%s'. Only bills with status 'confirming' can be marked as paid", bill.Status)
	}

	// For confirming status, mark as paid directly without payment check
	now := time.Now()
	return r.db.Model(&property.Bill{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":  property.BillStatusPaid,
			"paid_at": now,
		}).Error
}

// UpdateStatus updates bill status
func (r *BillRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&property.Bill{}).Where("id = ?", id).
		Update("status", status).Error
}

// GetTotalAmount gets total amount for bills
// Note: amount is stored as string (decimal), so we need to cast it
func (r *BillRepository) GetTotalAmount(userID uint, status string) (float64, error) {
	var total float64
	query := r.db.Model(&property.Bill{})

	if userID != 0 {
		query = query.Where("user_id = ? OR landlord_id = ? OR tenant_id = ? OR spa_id = ?", userID, userID, userID, userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Cast string amount to decimal for sum
	err := query.Select("COALESCE(SUM(CAST(amount AS DECIMAL(10,2))), 0)").Scan(&total).Error
	return total, err
}

// CountByStatus counts bills by status
func (r *BillRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&property.Bill{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// CountPendingByUserID counts pending and overdue bills for a specific user
func (r *BillRepository) CountPendingByUserID(userID uint) (int64, int64, error) {
	var pendingCount, overdueCount int64
	
	// Count pending bills
	err := r.db.Model(&property.Bill{}).
		Where("(user_id = ? OR landlord_id = ? OR tenant_id = ? OR spa_id = ?) AND status = ?",
			userID, userID, userID, userID, property.BillStatusPending).
		Count(&pendingCount).Error
	if err != nil {
		return 0, 0, err
	}
	
	// Count overdue bills (status = overdue OR (status = pending AND due_date < now))
	now := time.Now()
	err = r.db.Model(&property.Bill{}).
		Where("(user_id = ? OR landlord_id = ? OR tenant_id = ? OR spa_id = ?) AND (status = ? OR (status = ? AND due_date < ?))",
			userID, userID, userID, userID, property.BillStatusOverdue, property.BillStatusPending, now).
		Count(&overdueCount).Error
	if err != nil {
		return 0, 0, err
	}
	
	return pendingCount, overdueCount, nil
}

// CountOverdueAll counts all overdue bills (for admin/staff/accountant)
func (r *BillRepository) CountOverdueAll() (int64, error) {
	var count int64
	now := time.Now()
	err := r.db.Model(&property.Bill{}).
		Where("status = ? OR (status = ? AND due_date < ?)",
			property.BillStatusOverdue, property.BillStatusPending, now).
		Count(&count).Error
	return count, err
}

// GetOverdueBillIDs gets list of overdue bill IDs only
func (r *BillRepository) GetOverdueBillIDs(limit int) ([]uint, error) {
	var billIDs []uint
	now := time.Now()
	
	err := r.db.Model(&property.Bill{}).
		Where("status = ? OR (status = ? AND due_date < ?)",
			property.BillStatusOverdue, property.BillStatusPending, now).
		Order("due_date ASC, created_at DESC").
		Pluck("id", &billIDs).Error
	if err != nil {
		return nil, err
	}
	
	// Apply limit
	if limit > 0 && len(billIDs) > limit {
		billIDs = billIDs[:limit]
	}
	
	return billIDs, nil
}

// GetOverdueBillIDsByUserID gets list of overdue bill IDs for a specific user
func (r *BillRepository) GetOverdueBillIDsByUserID(userID uint, limit int) ([]uint, error) {
	var billIDs []uint
	now := time.Now()
	
	err := r.db.Model(&property.Bill{}).
		Where("(user_id = ? OR landlord_id = ? OR tenant_id = ? OR spa_id = ?) AND (status = ? OR (status = ? AND due_date < ?))",
			userID, userID, userID, userID,
			property.BillStatusOverdue, property.BillStatusPending, now).
		Order("due_date ASC, created_at DESC").
		Pluck("id", &billIDs).Error
	if err != nil {
		return nil, err
	}
	
	// Apply limit
	if limit > 0 && len(billIDs) > limit {
		billIDs = billIDs[:limit]
	}
	
	return billIDs, nil
}

// ListPendingByUserID lists pending and overdue bills for a specific user
func (r *BillRepository) ListPendingByUserID(userID uint, limit int) ([]property.Bill, error) {
	var bills []property.Bill
	now := time.Now()
	
	query := r.db.Model(&property.Bill{}).
		Where("(user_id = ? OR landlord_id = ? OR tenant_id = ? OR spa_id = ?) AND (status = ? OR status = ? OR (status = ? AND due_date < ?))",
			userID, userID, userID, userID,
			property.BillStatusPending, property.BillStatusOverdue, property.BillStatusPending, now).
		Preload("Unit").
		Preload("ParkingSlot").
		Order("due_date ASC, created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	err := query.Find(&bills).Error
	return bills, err
}

// ListOverdueAll lists all overdue bills (for admin/staff/accountant)
func (r *BillRepository) ListOverdueAll(limit int) ([]property.Bill, error) {
	var billIDs []uint
	now := time.Now()
	
	// First, get distinct bill IDs
	err := r.db.Model(&property.Bill{}).
		Where("status = ? OR (status = ? AND due_date < ?)",
			property.BillStatusOverdue, property.BillStatusPending, now).
		Order("due_date ASC, created_at DESC").
		Pluck("id", &billIDs).Error
	if err != nil {
		return nil, err
	}
	
	// Apply limit to IDs
	if limit > 0 && len(billIDs) > limit {
		billIDs = billIDs[:limit]
	}
	
	if len(billIDs) == 0 {
		return []property.Bill{}, nil
	}
	
	// Then fetch bills with preloads
	var bills []property.Bill
	err = r.db.Where("id IN ?", billIDs).
		Preload("Unit").
		Preload("ParkingSlot").
		Order("due_date ASC, created_at DESC").
		Find(&bills).Error
	return bills, err
}
