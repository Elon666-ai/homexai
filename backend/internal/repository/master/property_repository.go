package master

import (
	"homexai/internal/models/master"

	"gorm.io/gorm"
)

type PropertyRepository struct {
	db *gorm.DB
}

func NewPropertyRepository(db *gorm.DB) *PropertyRepository {
	return &PropertyRepository{db: db}
}

// Create creates a new property
func (r *PropertyRepository) Create(property *master.Property) error {
	return r.db.Create(property).Error
}

// FindByID finds a property by ID
func (r *PropertyRepository) FindByID(id uint) (*master.Property, error) {
	var property master.Property
	err := r.db.First(&property, id).Error
	if err != nil {
		return nil, err
	}
	return &property, nil
}

// FindBySubdomain finds a property by subdomain
func (r *PropertyRepository) FindBySubdomain(subdomain string) (*master.Property, error) {
	var property master.Property
	err := r.db.Where("subdomain = ?", subdomain).First(&property).Error
	if err != nil {
		return nil, err
	}
	return &property, nil
}

// Update updates a property
func (r *PropertyRepository) Update(property *master.Property) error {
	return r.db.Save(property).Error
}

// UpdateFields updates specific fields
func (r *PropertyRepository) UpdateFields(id uint, fields map[string]interface{}) error {
	return r.db.Model(&master.Property{}).Where("id = ?", id).Updates(fields).Error
}

// Delete deletes a property
func (r *PropertyRepository) Delete(id uint) error {
	return r.db.Delete(&master.Property{}, id).Error
}

// List lists all properties with pagination
func (r *PropertyRepository) List(page, perPage int) ([]master.Property, int64, error) {
	var properties []master.Property
	var total int64

	offset := (page - 1) * perPage

	err := r.db.Model(&master.Property{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.Offset(offset).Limit(perPage).Find(&properties).Error
	return properties, total, err
}

// ListActive lists active properties
func (r *PropertyRepository) ListActive() ([]master.Property, error) {
	var properties []master.Property
	err := r.db.Where("status = ?", "active").Find(&properties).Error
	return properties, err
}

// ExistsBySubdomain checks if subdomain exists
func (r *PropertyRepository) ExistsBySubdomain(subdomain string) (bool, error) {
	var count int64
	err := r.db.Model(&master.Property{}).Where("subdomain = ?", subdomain).Count(&count).Error
	return count > 0, err
}

// FindWithMapping finds property with database mapping
func (r *PropertyRepository) FindWithMapping(id uint) (*master.Property, error) {
	var property master.Property
	err := r.db.Preload("PropertyDBMapping").First(&property, id).Error
	if err != nil {
		return nil, err
	}
	return &property, nil
}

// Search searches properties by name
func (r *PropertyRepository) Search(query string) ([]master.Property, error) {
	var properties []master.Property
	searchPattern := "%" + query + "%"
	err := r.db.Where("name LIKE ?", searchPattern).Find(&properties).Error
	return properties, err
}
