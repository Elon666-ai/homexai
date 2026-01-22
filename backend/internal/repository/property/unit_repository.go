package property

import (
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type UnitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) *UnitRepository {
	return &UnitRepository{db: db}
}

// Create creates a new unit
func (r *UnitRepository) Create(unit *property.Unit) error {
	return r.db.Create(unit).Error
}

// FindByID finds a unit by ID
func (r *UnitRepository) FindByID(id uint) (*property.Unit, error) {
	var unit property.Unit
	err := r.db.First(&unit, id).Error
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

// FindByNumber finds a unit by number and type
func (r *UnitRepository) FindByNumber(unitNumber, unitType string) (*property.Unit, error) {
	var unit property.Unit
	err := r.db.Where("unit_number = ? AND unit_type = ?", unitNumber, unitType).
		First(&unit).Error
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

// Update updates a unit
func (r *UnitRepository) Update(unit *property.Unit) error {
	return r.db.Save(unit).Error
}

// UpdateFields updates specific fields
func (r *UnitRepository) UpdateFields(id uint, fields map[string]interface{}) error {
	return r.db.Model(&property.Unit{}).Where("id = ?", id).Updates(fields).Error
}

// Delete deletes a unit
func (r *UnitRepository) Delete(id uint) error {
	return r.db.Delete(&property.Unit{}, id).Error
}

// List lists units with pagination and filters
func (r *UnitRepository) List(unitType, status string, page, perPage int) ([]property.Unit, int64, error) {
	var units []property.Unit
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.Unit{})

	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset(offset).Limit(perPage).Find(&units).Error
	return units, total, err
}

// ListWithExclude lists units with pagination, filters, and type exclusion
func (r *UnitRepository) ListWithExclude(unitType, excludeType, status string, page, perPage int) ([]property.Unit, int64, error) {
	var units []property.Unit
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.Unit{})

	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	if excludeType != "" {
		query = query.Where("unit_type != ?", excludeType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset(offset).Limit(perPage).Find(&units).Error
	return units, total, err
}

// ListWithFilters lists units with comprehensive filters including unit number
func (r *UnitRepository) ListWithFilters(unitNumber, unitType, excludeType, status string, page, perPage int) ([]property.Unit, int64, error) {
	var units []property.Unit
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.Unit{})

	if unitNumber != "" {
		query = query.Where("unit_number LIKE ?", "%"+unitNumber+"%")
	}
	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}
	if excludeType != "" {
		query = query.Where("unit_type != ?", excludeType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset(offset).Limit(perPage).Find(&units).Error
	return units, total, err
}

// ListByType lists units by type
func (r *UnitRepository) ListByType(unitType string) ([]property.Unit, error) {
	var units []property.Unit
	err := r.db.Where("unit_type = ?", unitType).Find(&units).Error
	return units, err
}

// ListAvailable lists available units
func (r *UnitRepository) ListAvailable(unitType string) ([]property.Unit, error) {
	var units []property.Unit
	query := r.db.Where("status = ?", property.UnitStatusAvailable)

	if unitType != "" {
		query = query.Where("unit_type = ?", unitType)
	}

	err := query.Find(&units).Error
	return units, err
}

// UpdateStatus updates unit status
func (r *UnitRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&property.Unit{}).Where("id = ?", id).
		Update("status", status).Error
}

// ExistsByNumber checks if unit number exists
func (r *UnitRepository) ExistsByNumber(unitNumber, unitType string) (bool, error) {
	var count int64
	err := r.db.Model(&property.Unit{}).
		Where("unit_number = ? AND unit_type = ?", unitNumber, unitType).
		Count(&count).Error
	return count > 0, err
}

// Search searches units with pagination and filters
func (r *UnitRepository) Search(query, unitType, status string, page, perPage int) ([]property.Unit, int64, error) {
	var units []property.Unit
	var total int64
	searchPattern := "%" + query + "%"

	db := r.db.Model(&property.Unit{}).Where("unit_number LIKE ? OR description LIKE ?", searchPattern, searchPattern)

	if unitType != "" {
		db = db.Where("unit_type = ?", unitType)
	}

	if status != "" {
		db = db.Where("status = ?", status)
	}

	// Count total
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * perPage
	err := db.Offset(offset).Limit(perPage).Find(&units).Error
	return units, total, err
}

// CountByStatus counts units by status
func (r *UnitRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&property.Unit{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// CountByType counts units by type
func (r *UnitRepository) CountByType(unitType string) (int64, error) {
	var count int64
	err := r.db.Model(&property.Unit{}).Where("unit_type = ?", unitType).Count(&count).Error
	return count, err
}
