package property

import (
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type ServiceListingRepository struct {
	db *gorm.DB
}

func NewServiceListingRepository(db *gorm.DB) *ServiceListingRepository {
	return &ServiceListingRepository{db: db}
}

// Create creates a new service listing
func (r *ServiceListingRepository) Create(listing *property.ServiceListing) error {
	return r.db.Create(listing).Error
}

// FindByID finds a service listing by ID
func (r *ServiceListingRepository) FindByID(id uint) (*property.ServiceListing, error) {
	var listing property.ServiceListing
	err := r.db.First(&listing, id).Error
	if err != nil {
		return nil, err
	}
	return &listing, nil
}

// Update updates a service listing
func (r *ServiceListingRepository) Update(listing *property.ServiceListing) error {
	return r.db.Save(listing).Error
}

// Delete deletes a service listing
func (r *ServiceListingRepository) Delete(id uint) error {
	return r.db.Delete(&property.ServiceListing{}, id).Error
}

// List lists service listings with pagination and filters
func (r *ServiceListingRepository) List(serviceType, status string, page, perPage int) ([]property.ServiceListing, int64, error) {
	var listings []property.ServiceListing
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.ServiceListing{})

	if serviceType != "" {
		query = query.Where("service_type = ?", serviceType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset(offset).Limit(perPage).
		Order("created_at DESC").Find(&listings).Error
	return listings, total, err
}

// ListActive lists active service listings
func (r *ServiceListingRepository) ListActive(serviceType string) ([]property.ServiceListing, error) {
	var listings []property.ServiceListing
	query := r.db.Where("status = ?", property.ServiceListingStatusActive)

	if serviceType != "" {
		query = query.Where("service_type = ?", serviceType)
	}

	err := query.Order("created_at DESC").Find(&listings).Error
	return listings, err
}

// UpdateStatus updates service listing status
func (r *ServiceListingRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&property.ServiceListing{}).Where("id = ?", id).
		Update("status", status).Error
}

