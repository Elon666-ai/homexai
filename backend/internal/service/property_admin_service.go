package service

import (
	"errors"
	"fmt"

	"homexai/internal/database"
	"homexai/internal/models/master"
	masterRepo "homexai/internal/repository/master"
	"homexai/internal/utils"
)

// Helper function to convert string to *string
func strPtrAdmin(s string) *string {
	return &s
}

type PropertyAdminService struct {
	userRepo     *masterRepo.UserRepository
	propertyRepo *masterRepo.PropertyRepository
	staffRepo    *masterRepo.PropertyStaffRepository
}

func NewPropertyAdminService() *PropertyAdminService {
	masterDB := database.GetMasterGormDB()
	return &PropertyAdminService{
		userRepo:     masterRepo.NewUserRepository(masterDB),
		propertyRepo: masterRepo.NewPropertyRepository(masterDB),
		staffRepo:    masterRepo.NewPropertyStaffRepository(masterDB),
	}
}

// AssignAdminInput represents input for assigning property admin
type AssignAdminInput struct {
	Email    string
	FullName string
	Phone    string
}

// PropertyAdminResponse represents a property admin
type PropertyAdminResponse struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	FullName    string `json:"full_name"`
	Phone       string `json:"phone"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

// PropertyWithAdminResponse represents a property with its admin
type PropertyWithAdminResponse struct {
	ID        uint                   `json:"id"`
	Name      string                 `json:"name"`
	Subdomain string                 `json:"subdomain"`
	Status    string                 `json:"status"`
	Admin     *PropertyAdminResponse `json:"admin,omitempty"`
}

// GetPropertyAdmin gets the current property admin for a property
func (s *PropertyAdminService) GetPropertyAdmin(propertyID uint) (*PropertyAdminResponse, error) {
	// Verify property exists
	property, err := s.propertyRepo.FindByID(propertyID)
	if err != nil {
		return nil, errors.New("property not found")
	}
	_ = property

	// Get current property admin
	admin, err := s.staffRepo.GetPropertyAdmin(propertyID)
	if err != nil {
		return nil, errors.New("no property admin assigned")
	}

	response := &PropertyAdminResponse{
		ID:        admin.UserID,
		Email:     admin.Email,
		FullName:  admin.FullName,
		Phone:     admin.Phone,
		Status:    admin.Status,
		CreatedAt: admin.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if admin.LastLoginAt != nil {
		response.LastLoginAt = admin.LastLoginAt.Format("2006-01-02 15:04:05")
	}

	return response, nil
}

// AssignPropertyAdmin assigns a user as property admin (only one per property)
func (s *PropertyAdminService) AssignPropertyAdmin(propertyID, assignedBy uint, input AssignAdminInput) (*PropertyAdminResponse, error) {
	// Verify property exists
	property, err := s.propertyRepo.FindByID(propertyID)
	if err != nil {
		return nil, errors.New("property not found")
	}
	_ = property

	// Check if property already has a property admin
	count, err := s.staffRepo.GetPropertyAdminCount(propertyID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing admin: %w", err)
	}
	if count > 0 {
		return nil, errors.New("property already has a property admin. Use replace to change the admin")
	}

	// Check if email already exists
	existingUser, _ := s.userRepo.FindByEmail(input.Email)

	var user *master.User

	if existingUser != nil {
		// User exists, check if they are already a property_admin for another property
		existingAdminRole, _ := s.staffRepo.GetByPropertyAndUser(propertyID, existingUser.ID)
		if existingAdminRole != nil && existingAdminRole.Role == "property_admin" {
			return nil, errors.New("user is already a property admin for this property")
		}
		user = existingUser
	} else {
		passwordHash, err := utils.HashPassword(utils.DEFAULT_ADMIN_PASSWORD)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}

		user = &master.User{
			Email:                    strPtrAdmin(input.Email),
			FullName:                 input.FullName,
			PasswordHash:             passwordHash,
			Status:                   "active",
			MustChangePassword:       true,
			PublicEmail:              true,
			PublicPhone:              true,
			PublicFullName:           true,
			PublicPropertyCert:       true,
			PublicVehicleCROR:        true,
			EmailNotificationEnabled: true,
		}

		// Set phone if provided
		if input.Phone != "" {
			user.Phone = strPtrAdmin(input.Phone)
		}

		if err = s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Assign property_admin role
	err = s.staffRepo.AssignRole(user.ID, propertyID, "property_admin", assignedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to assign role: %w", err)
	}

	// Build response - safely dereference pointers
	response := &PropertyAdminResponse{
		ID:        user.ID,
		FullName:  user.FullName,
		Status:    "active",
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if user.Email != nil {
		response.Email = *user.Email
	}
	if user.Phone != nil {
		response.Phone = *user.Phone
	}

	return response, nil
}

// ReplacePropertyAdmin replaces the current property admin with a new one
func (s *PropertyAdminService) ReplacePropertyAdmin(propertyID, assignedBy uint, input AssignAdminInput) (*PropertyAdminResponse, error) {
	// Verify property exists
	property, err := s.propertyRepo.FindByID(propertyID)
	if err != nil {
		return nil, errors.New("property not found")
	}
	_ = property

	// Remove existing property admin
	err = s.removePropertyAdminInternal(propertyID)
	if err != nil {
		return nil, fmt.Errorf("failed to remove existing admin: %w", err)
	}

	// Assign new admin
	return s.AssignPropertyAdmin(propertyID, assignedBy, input)
}

// RemovePropertyAdmin removes the property admin from a property
func (s *PropertyAdminService) RemovePropertyAdmin(propertyID uint) error {
	// Verify property exists
	property, err := s.propertyRepo.FindByID(propertyID)
	if err != nil {
		return errors.New("property not found")
	}
	_ = property

	return s.removePropertyAdminInternal(propertyID)
}

// removePropertyAdminInternal removes the property admin role from a property
func (s *PropertyAdminService) removePropertyAdminInternal(propertyID uint) error {
	admin, err := s.staffRepo.GetPropertyAdmin(propertyID)
	if err != nil {
		return nil // No admin to remove
	}

	return s.staffRepo.RemovePropertyAdminRole(admin.UserID, propertyID)
}

// ListPropertiesWithAdmins lists all properties with their admin info
func (s *PropertyAdminService) ListPropertiesWithAdmins(page, perPage int) ([]PropertyWithAdminResponse, int64, error) {
	properties, total, err := s.propertyRepo.List(page, perPage)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]PropertyWithAdminResponse, len(properties))
	for i, prop := range properties {
		responses[i] = PropertyWithAdminResponse{
			ID:        prop.ID,
			Name:      prop.Name,
			Subdomain: prop.Subdomain,
			Status:    prop.Status,
		}

		// Get admin for this property
		admin, err := s.staffRepo.GetPropertyAdmin(prop.ID)
		if err == nil && admin != nil {
			responses[i].Admin = &PropertyAdminResponse{
				ID:        admin.UserID,
				Email:     admin.Email,
				FullName:  admin.FullName,
				Phone:     admin.Phone,
				Status:    admin.Status,
				CreatedAt: admin.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			if admin.LastLoginAt != nil {
				responses[i].Admin.LastLoginAt = admin.LastLoginAt.Format("2006-01-02 15:04:05")
			}
		}
	}

	return responses, total, nil
}
