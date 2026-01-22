package service

import (
	"errors"

	"homexai/internal/database"
	"homexai/internal/models/master"
	masterRepo "homexai/internal/repository/master"
	"homexai/internal/utils"
)

type UserService struct {
	userRepo *masterRepo.UserRepository
}

func NewUserService() *UserService {
	masterDB := database.GetMasterGormDB()
	return &UserService{
		userRepo: masterRepo.NewUserRepository(masterDB),
	}
}

// GetProfile gets user profile by ID
func (s *UserService) GetProfile(userID uint) (*master.User, error) {
	return s.userRepo.FindByID(userID)
}

// GetProfileWithRoles gets user profile with roles
func (s *UserService) GetProfileWithRoles(userID uint) (*master.User, error) {
	return s.userRepo.FindWithRoles(userID)
}

// UpdateProfile updates user profile
func (s *UserService) UpdateProfile(userID uint, updates map[string]interface{}) error {
	// Validate updates - don't allow updating sensitive fields directly
	delete(updates, "password_hash")
	delete(updates, "email_verified")
	delete(updates, "phone_verified")
	delete(updates, "status")

	return s.userRepo.UpdateFields(userID, updates)
}

// UpdateLanguagePreference updates user language preference
func (s *UserService) UpdateLanguagePreference(userID uint, language string) error {
	if !master.IsValidLanguage(language) {
		return errors.New("invalid language")
	}

	return s.userRepo.UpdateFields(userID, map[string]interface{}{
		"preferred_language": language,
	})
}

// ListUsers lists users with pagination
func (s *UserService) ListUsers(page, perPage int) ([]master.User, int64, error) {
	return s.userRepo.List(page, perPage)
}

// SearchUsers searches users by query
func (s *UserService) SearchUsers(query string, page, perPage int) ([]master.User, int64, error) {
	return s.userRepo.Search(query, page, perPage)
}

func (s *UserService) SearchUsersByRole(query string, role string, page, perPage int) ([]master.User, int64, error) {
	return s.userRepo.SearchByRole(query, role, page, perPage)
}

// CreateUser creates a new user (admin function)
func (s *UserService) CreateUser(user *master.User, password string) error {
	// Check if email or phone already exists
	if user.Email != nil && *user.Email != "" {
		exists, err := s.userRepo.ExistsByEmail(*user.Email)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("email already exists")
		}
	}

	if user.Phone != nil && *user.Phone != "" {
		exists, err := s.userRepo.ExistsByPhone(*user.Phone)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("phone already exists")
		}
	}

	// Hash password if provided
	if password != "" {
		passwordResult, err := utils.HashPasswordWithSalt(password, "")
		if err != nil {
			return err
		}
		user.PasswordHash = passwordResult.Hash
		user.Salt = passwordResult.Salt
	}

	return s.userRepo.Create(user)
}

// UpdateUser updates a user (admin function)
func (s *UserService) UpdateUser(userID uint, updates map[string]interface{}) error {
	// Check if user exists
	_, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	return s.userRepo.UpdateFields(userID, updates)
}

// DeleteUser deletes a user (admin function)
func (s *UserService) DeleteUser(userID uint) error {
	return s.userRepo.Delete(userID)
}

// ActivateUser activates a user account
func (s *UserService) ActivateUser(userID uint) error {
	return s.userRepo.UpdateFields(userID, map[string]interface{}{
		"status": "active",
	})
}

// SuspendUser suspends a user account
func (s *UserService) SuspendUser(userID uint) error {
	return s.userRepo.UpdateFields(userID, map[string]interface{}{
		"status": "suspended",
	})
}

// DeactivateUser deactivates a user account
func (s *UserService) DeactivateUser(userID uint) error {
	return s.userRepo.UpdateFields(userID, map[string]interface{}{
		"status": "inactive",
	})
}

// GetUserByEmail gets user by email
func (s *UserService) GetUserByEmail(email string) (*master.User, error) {
	return s.userRepo.FindByEmail(email)
}

// GetUserByPhone gets user by phone
func (s *UserService) GetUserByPhone(phone string) (*master.User, error) {
	return s.userRepo.FindByPhone(phone)
}

// CheckEmailExists checks if email exists
func (s *UserService) CheckEmailExists(email string) (bool, error) {
	return s.userRepo.ExistsByEmail(email)
}

// CheckPhoneExists checks if phone exists
func (s *UserService) CheckPhoneExists(phone string) (bool, error) {
	return s.userRepo.ExistsByPhone(phone)
}

// GetUserNamesByRole gets user names list by role for the current property
func (s *UserService) GetUserNamesByRole(propertyID uint, role string) ([]map[string]interface{}, error) {
	return s.userRepo.GetUserNamesByRole(propertyID, role)
}
