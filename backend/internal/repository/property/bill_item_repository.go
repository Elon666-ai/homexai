package property

import (
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type BillItemRepository struct {
	db *gorm.DB
}

func NewBillItemRepository(db *gorm.DB) *BillItemRepository {
	return &BillItemRepository{db: db}
}

// Create creates a new bill item
func (r *BillItemRepository) Create(item *property.BillItem) error {
	return r.db.Create(item).Error
}

// CreateBatch creates multiple bill items
func (r *BillItemRepository) CreateBatch(items []property.BillItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

// FindByBillID finds all items for a bill
func (r *BillItemRepository) FindByBillID(billID uint) ([]property.BillItem, error) {
	var items []property.BillItem
	err := r.db.Where("bill_id = ?", billID).Order("created_at ASC").Find(&items).Error
	return items, err
}

// DeleteByBillID deletes all items for a bill
func (r *BillItemRepository) DeleteByBillID(billID uint) error {
	return r.db.Where("bill_id = ?", billID).Delete(&property.BillItem{}).Error
}

// Delete deletes a bill item
func (r *BillItemRepository) Delete(id uint) error {
	return r.db.Delete(&property.BillItem{}, id).Error
}

// Update updates a bill item
func (r *BillItemRepository) Update(item *property.BillItem) error {
	return r.db.Save(item).Error
}

