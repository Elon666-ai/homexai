package service

import (
	"homexai/internal/models/property"

	"gorm.io/gorm"
)

// RequestService handles request-related business logic
type RequestService struct {
	masterDB *gorm.DB
}

// NewRequestService creates a new RequestService
func NewRequestService(masterDB *gorm.DB) *RequestService {
	return &RequestService{
		masterDB: masterDB,
	}
}

// GetRequestsByUser gets all requests for a specific user
func (s *RequestService) GetRequestsByUser(propertyDB *gorm.DB, userID uint) ([]property.Request, error) {
	var requests []property.Request
	err := propertyDB.Where("user_id = ?", userID).
		Preload("Unit").
		Order("created_at DESC").
		Find(&requests).Error
	return requests, err
}

// GetRequestsByUnit gets all requests for a specific unit
func (s *RequestService) GetRequestsByUnit(propertyDB *gorm.DB, unitID uint) ([]property.Request, error) {
	var requests []property.Request
	err := propertyDB.Where("unit_id = ?", unitID).
		Preload("Unit").
		Order("created_at DESC").
		Find(&requests).Error
	return requests, err
}

// GetPendingRequests gets all pending requests
func (s *RequestService) GetPendingRequests(propertyDB *gorm.DB) ([]property.Request, error) {
	var requests []property.Request
	err := propertyDB.Where("status = ?", "pending").
		Preload("Unit").
		Order("priority DESC, created_at ASC").
		Find(&requests).Error
	return requests, err
}

// GetUrgentRequests gets all urgent requests
func (s *RequestService) GetUrgentRequests(propertyDB *gorm.DB) ([]property.Request, error) {
	var requests []property.Request
	err := propertyDB.Where("priority = ?", "urgent").
		Where("status NOT IN ?", []string{"completed", "cancelled", "rejected"}).
		Preload("Unit").
		Order("created_at ASC").
		Find(&requests).Error
	return requests, err
}

// GetRequestsAssignedTo gets requests assigned to a specific user
func (s *RequestService) GetRequestsAssignedTo(propertyDB *gorm.DB, userID uint) ([]property.Request, error) {
	var requests []property.Request
	err := propertyDB.Where("assigned_to = ?", userID).
		Preload("Unit").
		Order("priority DESC, created_at ASC").
		Find(&requests).Error
	return requests, err
}
