package service

import (
	"errors"
	"homexai/internal/models/master"
	"homexai/internal/utils"

	"gorm.io/gorm"
)

// LoginResponseEnhanced contains login result with token pair
type LoginResponseEnhanced struct {
	User               *master.User `json:"user"`
	Role               string       `json:"role"`
	PropertyID         uint         `json:"property_id"`
	Subdomain          string       `json:"subdomain"`
	MustChangePassword bool         `json:"must_change_password"`
	TokenPair          *TokenPair   `json:"token_pair"` // Access and Refresh tokens
}

// LoginWithEmailPasswordEnhanced authenticates user with email and password
// Returns token pair (access + refresh tokens)
func (s *AuthService) LoginWithEmailPasswordEnhanced(email, password string) (*LoginResponseEnhanced, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid email or password#3")
		}
		return nil, err
	}

	// Check password
	if !utils.CheckPasswordHash(password, user.PasswordHash, user.Salt) {
		return nil, errors.New("invalid email or password#4")
	}

	// Check if user is active
	if !user.IsActive() {
		return nil, errors.New("account is not active")
	}

	// Check if email is verified (optional - can be enabled later)
	// if !user.EmailVerified {
	//     return nil, errors.New("email not verified")
	// }

	// Get user role
	var userRole master.UserRole
	err = s.masterDB.Preload("Role").Where("user_id = ?", user.ID).First(&userRole).Error
	if err != nil {
		return nil, errors.New("user role not found")
	}

	// Get property ID if exists
	var propertyID uint
	var subdomain string

	if userRole.PropertyID > 0 {
		propertyID = userRole.PropertyID

		// Get property subdomain
		type PropertyInfo struct {
			Subdomain string `gorm:"column:subdomain"`
		}
		var property PropertyInfo
		if err := s.masterDB.Table("properties").
			Select("subdomain").
			Where("id = ?", propertyID).
			First(&property).Error; err == nil {
			subdomain = property.Subdomain
		}
	}

	// Generate token pair
	tokenPair, err := GenerateTokenPair(user.ID, *user.Email, userRole.Role, propertyID)
	if err != nil {
		return nil, errors.New("failed to generate authentication tokens")
	}

	response := &LoginResponseEnhanced{
		User:               user,
		Role:               userRole.Role,
		PropertyID:         propertyID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
		TokenPair:          tokenPair,
	}

	return response, nil
}

// LoginWithEmailPasswordPropertyEnhanced authenticates user with email, password and property subdomain
// This is for PROPERTY-SPECIFIC login (with subdomain)
func (s *AuthService) LoginWithEmailPasswordPropertyEnhanced(email, password, subdomain string) (*LoginResponseEnhanced, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid email or password#5")
		}
		return nil, err
	}

	// Check password
	if !utils.CheckPasswordHash(password, user.PasswordHash, user.Salt) {
		return nil, errors.New("invalid email or password#6")
	}

	// Check if user is active
	if !user.IsActive() {
		return nil, errors.New("account is not active")
	}

	// Get property by subdomain
	type PropertyInfo struct {
		ID        uint   `gorm:"column:id"`
		Subdomain string `gorm:"column:subdomain"`
		Status    string `gorm:"column:status"`
	}
	var property PropertyInfo
	err = s.masterDB.Table("properties").
		Where("subdomain = ?", subdomain).
		First(&property).Error

	if err != nil {
		return nil, errors.New("property not found")
	}

	if property.Status != "active" {
		return nil, errors.New("property is not active")
	}

	// Get user role for this property
	var userRole master.UserRole
	err = s.masterDB.Preload("Role").
		Where("user_id = ? AND property_id = ?", user.ID, property.ID).
		First(&userRole).Error

	if err != nil {
		return nil, errors.New("user does not have access to this property")
	}

	// Generate token pair
	tokenPair, err := GenerateTokenPair(user.ID, *user.Email, userRole.Role, property.ID)
	if err != nil {
		return nil, errors.New("failed to generate authentication tokens")
	}

	response := &LoginResponseEnhanced{
		User:               user,
		Role:               userRole.Role,
		PropertyID:         property.ID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
		TokenPair:          tokenPair,
	}

	return response, nil
}

// LoginWithCodeEnhanced authenticates user with verification code
// Returns token pair (access + refresh tokens)
func (s *AuthService) LoginWithCodeEnhanced(identifier, code, subdomain string) (*LoginResponseEnhanced, error) {
	// Verify the code
	if err := s.verificationSvc.VerifyCode(s.ctx, identifier, CodeTypeLogin, code); err != nil {
		return nil, err
	}

	// Find user by identifier (email or phone)
	user, err := s.userRepo.FindByEmailOrPhone(identifier)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	// Check if user is active
	if !user.IsActive() {
		return nil, errors.New("account is not active")
	}

	// If subdomain is provided, verify user has access to this property
	var propertyID uint
	var roleName string

	if subdomain != "" {
		// Get property by subdomain
		type PropertyInfo struct {
			ID        uint   `gorm:"column:id"`
			Subdomain string `gorm:"column:subdomain"`
			Status    string `gorm:"column:status"`
		}
		var property PropertyInfo
		err = s.masterDB.Table("properties").
			Where("subdomain = ?", subdomain).
			First(&property).Error

		if err != nil {
			return nil, errors.New("property not found")
		}

		if property.Status != "active" {
			return nil, errors.New("property is not active")
		}

		// Get user role for this property
		var userRole master.UserRole
		err = s.masterDB.Preload("Role").
			Where("user_id = ? AND property_id = ?", user.ID, property.ID).
			First(&userRole).Error

		if err != nil {
			return nil, errors.New("user does not have access to this property")
		}

		propertyID = property.ID
		roleName = userRole.Role
	} else {
		// Main portal login (super_admin)
		var userRole master.UserRole
		err = s.masterDB.Preload("Role").Where("user_id = ?", user.ID).First(&userRole).Error
		if err != nil {
			return nil, errors.New("user role not found")
		}

		roleName = userRole.Role
		if userRole.PropertyID > 0 {
			propertyID = userRole.PropertyID
		}
	}

	// Generate token pair
	tokenPair, err := GenerateTokenPair(user.ID, *user.Email, roleName, propertyID)
	if err != nil {
		return nil, errors.New("failed to generate authentication tokens")
	}

	response := &LoginResponseEnhanced{
		User:               user,
		Role:               roleName,
		PropertyID:         propertyID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
		TokenPair:          tokenPair,
	}

	return response, nil
}
