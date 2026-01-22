package property

import (
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

type RequestRepository struct {
	db *gorm.DB
}

func NewRequestRepository(db *gorm.DB) *RequestRepository {
	return &RequestRepository{db: db}
}

// Create creates a new request
func (r *RequestRepository) Create(request *property.Request) error {
	return r.db.Create(request).Error
}

// FindByID finds a request by ID
func (r *RequestRepository) FindByID(id uint) (*property.Request, error) {
	var request property.Request
	err := r.db.Preload("Unit").First(&request, id).Error
	if err != nil {
		return nil, err
	}
	return &request, nil
}

// Update updates a request
func (r *RequestRepository) Update(request *property.Request) error {
	return r.db.Save(request).Error
}

// Delete deletes a request
func (r *RequestRepository) Delete(id uint) error {
	return r.db.Delete(&property.Request{}, id).Error
}

// List lists requests with pagination and filters
func (r *RequestRepository) List(userID uint, status, requestType string, page, perPage int) ([]property.Request, int64, error) {
	var requests []property.Request
	var total int64

	offset := (page - 1) * perPage
	query := r.db.Model(&property.Request{})

	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if requestType != "" {
		query = query.Where("request_type = ?", requestType)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Preload("Unit").Offset(offset).Limit(perPage).
		Order("created_at DESC").Find(&requests).Error
	return requests, total, err
}

// ListByUnit lists requests for a unit
func (r *RequestRepository) ListByUnit(unitID uint) ([]property.Request, error) {
	var requests []property.Request
	err := r.db.Where("unit_id = ?", unitID).
		Order("created_at DESC").Find(&requests).Error
	return requests, err
}

// ListPending lists pending requests
func (r *RequestRepository) ListPending() ([]property.Request, error) {
	var requests []property.Request
	err := r.db.Where("status = ?", property.RequestStatusPending).
		Preload("Unit").Order("created_at ASC").Find(&requests).Error
	return requests, err
}

// ListUrgent lists urgent requests
func (r *RequestRepository) ListUrgent() ([]property.Request, error) {
	var requests []property.Request
	err := r.db.Where("priority = ? AND status != ?",
		property.RequestPriorityUrgent, property.RequestStatusCompleted).
		Preload("Unit").Order("created_at ASC").Find(&requests).Error
	return requests, err
}

// Assign assigns a request to staff
func (r *RequestRepository) Assign(id, staffID uint) error {
	return r.db.Model(&property.Request{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"assigned_to": staffID,
			"status":      property.RequestStatusInProgress,
		}).Error
}

// UpdateStatus updates request status
func (r *RequestRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&property.Request{}).Where("id = ?", id).
		Update("status", status).Error
}

// Complete completes a request
func (r *RequestRepository) Complete(id uint, resolution string) error {
	return r.db.Model(&property.Request{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      property.RequestStatusCompleted,
			"resolution":  resolution,
			"resolved_at": gorm.Expr("NOW()"),
		}).Error
}

// CountByStatus counts requests by status
func (r *RequestRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&property.Request{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// CountByType counts requests by type
func (r *RequestRepository) CountByType(requestType string) (int64, error) {
	var count int64
	err := r.db.Model(&property.Request{}).Where("request_type = ?", requestType).Count(&count).Error
	return count, err
}
