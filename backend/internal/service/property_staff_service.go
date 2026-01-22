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
func strPtrStaff(s string) *string {
	return &s
}

type PropertyStaffService struct {
	userRepo     *masterRepo.UserRepository
	roleRepo     *masterRepo.RoleRepository
	propertyRepo *masterRepo.PropertyRepository
	staffRepo    *masterRepo.PropertyStaffRepository
}

func NewPropertyStaffService() *PropertyStaffService {
	masterDB := database.GetMasterGormDB()
	return &PropertyStaffService{
		userRepo:     masterRepo.NewUserRepository(masterDB),
		roleRepo:     masterRepo.NewRoleRepository(masterDB),
		propertyRepo: masterRepo.NewPropertyRepository(masterDB),
		staffRepo:    masterRepo.NewPropertyStaffRepository(masterDB),
	}
}

// CreateStaffInput represents input for creating staff
type CreateStaffInput struct {
	Email    string
	FullName string
	Phone    string
	Role     string
}

// CreateStaffResult represents the result of creating staff
type CreateStaffResult struct {
	Staff         *StaffResponse
	IsNewUser     bool   // true if a new user was created (needs email)
	IsReactivated bool   // true if an inactive user was reactivated
	TempPassword  string // temporary password if new user or reactivated
}

// UpdateStaffInput represents input for updating staff
type UpdateStaffInput struct {
	FullName string
	Phone    string
	Status   string
}

// StaffResponse represents a staff member with their role
type StaffResponse struct {
	ID              uint   `json:"id"`
	Email           string `json:"email"`
	FullName        string `json:"full_name"`
	Phone           string `json:"phone"`
	Status          string `json:"status"`
	Role            string `json:"role"`
	RoleDisplayName string `json:"role_display_name"`
	CreatedAt       string `json:"created_at"`
	LastLoginAt     string `json:"last_login_at,omitempty"`
}

// ListPropertyStaff lists all property_account and property_staff for a property
func (s *PropertyStaffService) ListPropertyStaff(propertyID uint, roleFilter string, page, perPage int) ([]StaffResponse, int64, error) {
	// Validate role filter
	validRoles := []string{"property_account", "property_staff"}
	if roleFilter != "" {
		isValid := false
		for _, r := range validRoles {
			if roleFilter == r {
				isValid = true
				break
			}
		}
		if !isValid {
			return nil, 0, errors.New("invalid role filter")
		}
		validRoles = []string{roleFilter}
	}

	staff, total, err := s.staffRepo.ListByPropertyAndRoles(propertyID, validRoles, page, perPage)
	if err != nil {
		return nil, 0, err
	}

	// Convert to response format
	responses := make([]StaffResponse, len(staff))
	for i, st := range staff {
		responses[i] = s.toStaffResponse(&st)
	}

	return responses, total, nil
}

// GetPropertyStaff gets a specific staff member
func (s *PropertyStaffService) GetPropertyStaff(propertyID, userID uint) (*StaffResponse, error) {
	staff, err := s.staffRepo.GetByPropertyAndUser(propertyID, userID)
	if err != nil {
		return nil, err
	}

	// Verify role is property_account or property_staff
	if staff.Role != "property_account" && staff.Role != "property_staff" {
		return nil, errors.New("user is not a property staff member")
	}

	response := s.toStaffResponse(staff)
	return &response, nil
}

// CreatePropertyStaff creates a new staff account
func (s *PropertyStaffService) CreatePropertyStaff(propertyID, assignedBy uint, input CreateStaffInput) (*CreateStaffResult, error) {
	// Validate role
	if input.Role != "property_account" && input.Role != "property_staff" && input.Role != "property_guard" {
		return nil, errors.New("invalid role: only property_account, property_staff, and property_guard are allowed")
	}

	// Check if email already exists
	existingUser, _ := s.userRepo.FindByEmail(input.Email)

	var user *master.User
	var err error
	var isNewUser bool
	var tempPassword string
	var needsReactivation bool

	if existingUser != nil {
		// User exists, check if they already have an active role in this property
		existingRole, _ := s.staffRepo.GetByPropertyAndUser(propertyID, existingUser.ID)
		if existingRole != nil {
			// Role exists - check if it's active or inactive
			if existingRole.Status == "active" {
				return nil, errors.New("user already has an active role in this property")
			}
			// Role exists but is inactive - reactivate it
			err = s.staffRepo.UpdateRoleStatus(existingUser.ID, propertyID, "active")
			if err != nil {
				return nil, fmt.Errorf("failed to reactivate role: %w", err)
			}
			needsReactivation = true
		}

		user = existingUser

		// Check if user is inactive - need to reactivate
		if existingUser.Status == "inactive" {
			needsReactivation = true
			passwordHash, err := utils.HashPassword(utils.DEFAULT_ADMIN_PASSWORD)
			if err != nil {
				return nil, fmt.Errorf("failed to hash password: %w", err)
			}

			// Reactivate user with new password
			err = s.userRepo.UpdateFields(existingUser.ID, map[string]interface{}{
				"status":               "active",
				"password_hash":        passwordHash,
				"must_change_password": true,
				"full_name":            input.FullName,
				"phone":                input.Phone,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to reactivate user: %w", err)
			}
			isNewUser = true // Treat as new user for email notification purposes
		} else {
			isNewUser = false
		}

		// If role didn't exist, assign it now
		if existingRole == nil {
			err = s.staffRepo.AssignRole(user.ID, propertyID, input.Role, assignedBy)
			if err != nil {
				return nil, fmt.Errorf("failed to assign role: %w", err)
			}
		}
	} else {
		passwordHash, err := utils.HashPassword(utils.DEFAULT_ADMIN_PASSWORD)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}

		user = &master.User{
			Email:                    strPtrStaff(input.Email),
			FullName:                 input.FullName,
			PasswordHash:             passwordHash,
			Status:                   "active",
			MustChangePassword:       true, // Force password change on first login
			PublicEmail:              true,
			PublicPhone:              true,
			PublicFullName:           true,
			PublicPropertyCert:       true,
			PublicVehicleCROR:        true,
			EmailNotificationEnabled: true,
		}

		// Set phone if provided
		if input.Phone != "" {
			user.Phone = strPtrStaff(input.Phone)
		}

		if err = s.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		isNewUser = true

		// Assign role to new user
		err = s.staffRepo.AssignRole(user.ID, propertyID, input.Role, assignedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to assign role: %w", err)
		}
	}

	// Get the created staff with all details
	staff, err := s.staffRepo.GetByPropertyAndUser(propertyID, user.ID)
	if err != nil {
		return nil, err
	}

	response := s.toStaffResponse(staff)
	return &CreateStaffResult{
		Staff:         &response,
		IsNewUser:     isNewUser,
		IsReactivated: needsReactivation,
		TempPassword:  tempPassword,
	}, nil
}

// UpdatePropertyStaff updates a staff member
func (s *PropertyStaffService) UpdatePropertyStaff(propertyID, userID uint, input UpdateStaffInput) error {
	// Verify staff exists and has correct role
	staff, err := s.staffRepo.GetByPropertyAndUser(propertyID, userID)
	if err != nil {
		return errors.New("staff member not found")
	}

	if staff.Role != "property_account" && staff.Role != "property_staff" {
		return errors.New("cannot update this user's role")
	}

	// Update user fields
	updates := make(map[string]interface{})
	if input.FullName != "" {
		updates["full_name"] = input.FullName
	}
	if input.Phone != "" {
		updates["phone"] = input.Phone
	}

	if len(updates) > 0 {
		if err = s.userRepo.UpdateFields(userID, updates); err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}
	}

	// Update role status if provided
	if input.Status != "" {
		if err = s.staffRepo.UpdateRoleStatus(userID, propertyID, input.Status); err != nil {
			return fmt.Errorf("failed to update role status: %w", err)
		}
	}

	return nil
}

// RemovePropertyStaff removes a staff member from a property
func (s *PropertyStaffService) RemovePropertyStaff(propertyID, userID uint) error {
	// Verify staff exists and has correct role
	staff, err := s.staffRepo.GetByPropertyAndUser(propertyID, userID)
	if err != nil {
		return fmt.Errorf("staff member not found: property_id=%d, user_id=%d", propertyID, userID)
	}

	if staff.Role != "property_account" && staff.Role != "property_staff" {
		return fmt.Errorf("cannot remove user with role '%s' - only property_account and property_staff can be removed", staff.Role)
	}

	// Remove role assignment from user_roles
	err = s.staffRepo.RemoveRole(userID, propertyID)
	if err != nil {
		return fmt.Errorf("failed to remove staff role: %w", err)
	}

	// Check if user has any other active roles
	hasOtherRoles, err := s.staffRepo.HasOtherActiveRoles(userID, propertyID)
	if err != nil {
		// Log but don't fail - role was already removed
		println("Warning: failed to check other roles:", err.Error())
	}

	// If user has no other roles, deactivate the user account
	if !hasOtherRoles {
		err = s.userRepo.UpdateFields(userID, map[string]interface{}{
			"status": "inactive",
		})
		if err != nil {
			// Log but don't fail - role was already removed
			println("Warning: failed to deactivate user:", err.Error())
		} else {
			println("User deactivated: user_id=", userID)
		}
	} else {
		println("User has other roles, not deactivating: user_id=", userID)
	}

	return nil
}

// toStaffResponse converts a PropertyStaffInfo to StaffResponse
func (s *PropertyStaffService) toStaffResponse(staff *masterRepo.PropertyStaffInfo) StaffResponse {
	response := StaffResponse{
		ID:              staff.UserID,
		Email:           staff.Email,
		FullName:        staff.FullName,
		Phone:           staff.Phone,
		Status:          staff.Status,
		Role:            staff.Role,
		RoleDisplayName: s.getRoleDisplayName(staff.Role),
		CreatedAt:       staff.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	if staff.LastLoginAt != nil {
		response.LastLoginAt = staff.LastLoginAt.Format("2006-01-02 15:04:05")
	}

	return response
}

// getRoleDisplayName returns the display name for a role
func (s *PropertyStaffService) getRoleDisplayName(roleCode string) string {
	switch roleCode {
	case "property_account":
		return "Property Accountant"
	case "property_staff":
		return "Property Staff"
	default:
		return roleCode
	}
}
