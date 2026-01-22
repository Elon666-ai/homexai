package property

import (
	"time"

	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type ServiceOrderRepository struct {
	db *gorm.DB
}

func NewServiceOrderRepository(db *gorm.DB) *ServiceOrderRepository {
	return &ServiceOrderRepository{db: db}
}

// Create creates a new service order
func (r *ServiceOrderRepository) Create(order *property.ServiceOrder) error {
	return r.db.Create(order).Error
}

// FindByID finds a service order by ID
func (r *ServiceOrderRepository) FindByID(id uint) (*property.ServiceOrder, error) {
	var order property.ServiceOrder
	err := r.db.Preload("ServiceListing").Preload("Unit").
		First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// FindByOrderNumber finds a service order by order number
func (r *ServiceOrderRepository) FindByOrderNumber(orderNumber string) (*property.ServiceOrder, error) {
	var order property.ServiceOrder
	err := r.db.Where("order_number = ?", orderNumber).
		Preload("ServiceListing").Preload("Unit").
		First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// Update updates a service order
func (r *ServiceOrderRepository) Update(order *property.ServiceOrder) error {
	return r.db.Save(order).Error
}

// List lists service orders with pagination and filters
func (r *ServiceOrderRepository) List(userID uint, status string, page, perPage int) ([]property.ServiceOrder, int64, error) {
	var orders []property.ServiceOrder
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.ServiceOrder{})

	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Preload("ServiceListing").Preload("Unit").
		Offset(offset).Limit(perPage).
		Order("created_at DESC").Find(&orders).Error
	return orders, total, err
}

// ListByServiceListing lists orders for a service listing
func (r *ServiceOrderRepository) ListByServiceListing(serviceListingID uint) ([]property.ServiceOrder, error) {
	var orders []property.ServiceOrder
	err := r.db.Where("service_listing_id = ?", serviceListingID).
		Preload("Unit").
		Order("created_at DESC").Find(&orders).Error
	return orders, err
}

// ListByUnit lists orders for a unit
func (r *ServiceOrderRepository) ListByUnit(unitID uint) ([]property.ServiceOrder, error) {
	var orders []property.ServiceOrder
	err := r.db.Where("unit_id = ?", unitID).
		Preload("ServiceListing").
		Order("created_at DESC").Find(&orders).Error
	return orders, err
}

// UpdateStatus updates service order status
func (r *ServiceOrderRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&property.ServiceOrder{}).Where("id = ?", id).
		Update("status", status).Error
}

// AssignStaff assigns staff to an order
func (r *ServiceOrderRepository) AssignStaff(id, staffID uint) error {
	return r.db.Model(&property.ServiceOrder{}).Where("id = ?", id).
		Update("assigned_staff_id", staffID).Error
}

// CompleteOrder marks an order as completed
func (r *ServiceOrderRepository) CompleteOrder(id uint) error {
	now := time.Now()
	return r.db.Model(&property.ServiceOrder{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      property.ServiceOrderStatusCompleted,
			"completed_at": now,
		}).Error
}

// ConfirmOrder marks an order as confirmed by user and sets status to closed
func (r *ServiceOrderRepository) ConfirmOrder(id uint) error {
	now := time.Now()
	return r.db.Model(&property.ServiceOrder{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       property.ServiceOrderStatusClosed,
			"confirmed_at": now,
		}).Error
}

// CancelOrder cancels an order
func (r *ServiceOrderRepository) CancelOrder(id uint) error {
	now := time.Now()
	return r.db.Model(&property.ServiceOrder{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       property.ServiceOrderStatusCancelled,
			"cancelled_at": now,
		}).Error
}

