package service

import (
	"context"
	"errors"
	"fmt"

	"homexai/internal/database"
	"homexai/internal/models/master"
	masterRepo "homexai/internal/repository/master"
	"homexai/internal/utils"

	"gorm.io/gorm"
)

type AuthService struct {
	userRepo        *masterRepo.UserRepository
	verificationSvc *VerificationService
	smtpSvc         *SmtpService
	smsSvc          *SMSService
	ctx             context.Context
	masterDB        *gorm.DB
}

// LoginResponse contains login result with user, role and property info
type LoginResponse struct {
	User               *master.User `json:"user"`
	Role               string       `json:"role"`
	PropertyID         uint         `json:"property_id"`
	Subdomain          string       `json:"subdomain"`
	MustChangePassword bool         `json:"must_change_password"`
}

func NewAuthService(smtpSvc *SmtpService, smsSvc *SMSService) *AuthService {
	masterDB := database.GetMasterGormDB()
	redisClient := database.GetRedisClient()
	return &AuthService{
		userRepo:        masterRepo.NewUserRepository(masterDB),
		verificationSvc: NewVerificationService(redisClient),
		smtpSvc:         smtpSvc,
		smsSvc:          smsSvc,
		ctx:             context.Background(),
		masterDB:        masterDB,
	}
}

// Helper function to safely get string from *string pointer
func getEmailString(email *string) string {
	if email == nil {
		return ""
	}
	return *email
}

// Helper function to convert string to *string pointer
func strPtr(s string) *string {
	return &s
}

// LoginWithEmailPassword authenticates user with email and password
// This is for MAIN PORTAL login only (no subdomain)
// Only super_admin users are allowed to login here
func (s *AuthService) LoginWithEmailPassword(email, password string) (string, *LoginResponse, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		fmt.Printf("LoginWithEmailPassword failure! %+v\n", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid email or password#1")
		}
		return "", nil, err
	}

	// Check password
	if !utils.CheckPasswordHash(password, user.PasswordHash, user.Salt) {
		return "", nil, errors.New("invalid email or password#2")
	}

	// Check if user is active
	if !user.IsActive() {
		return "", nil, errors.New("account is not active")
	}

	// CRITICAL: Main portal is only for super_admin users
	// Check if user has super_admin role (PropertyID = 0 means cross-property admin)
	var superAdminRole master.UserRole
	err = s.masterDB.Where("user_id = ? AND role = ? AND status = ?", user.ID, "super_admin", "active").
		First(&superAdminRole).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User is not a super_admin, must use property portal
			return "", nil, errors.New("please login via your property portal")
		}
		return "", nil, err
	}

	// Generate JWT token with super_admin role
	token, err := GenerateToken(user.ID, getEmailString(user.Email), "super_admin", 0)
	if err != nil {
		return "", nil, err
	}

	return token, &LoginResponse{
		User:               user,
		Role:               "super_admin",
		PropertyID:         0,
		Subdomain:          "",
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// LoginWithPhonePassword authenticates user with phone and password
// This is for MAIN PORTAL login only (no subdomain)
// Only super_admin users are allowed to login here
func (s *AuthService) LoginWithPhonePassword(phone, password string) (string, *LoginResponse, error) {
	user, err := s.userRepo.FindByPhone(phone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid phone or password")
		}
		return "", nil, err
	}

	if !utils.CheckPasswordHash(password, user.PasswordHash, user.Salt) {
		return "", nil, errors.New("invalid phone or password")
	}

	if !user.IsActive() {
		return "", nil, errors.New("account is not active")
	}

	// CRITICAL: Main portal is only for super_admin users
	// Check if user has super_admin role
	var superAdminRole master.UserRole
	err = s.masterDB.Where("user_id = ? AND role = ? AND status = ?", user.ID, "super_admin", "active").
		First(&superAdminRole).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User is not a super_admin, must use property portal
			return "", nil, errors.New("please login via your property portal")
		}
		return "", nil, err
	}

	// Generate JWT token with super_admin role
	token, err := GenerateToken(user.ID, getEmailString(user.Email), "super_admin", 0)
	if err != nil {
		return "", nil, err
	}

	return token, &LoginResponse{
		User:               user,
		Role:               "super_admin",
		PropertyID:         0,
		Subdomain:          "",
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// LoginWithEmailCode authenticates user with email and verification code
func (s *AuthService) LoginWithEmailCode(email, code, subdomain string) (string, *LoginResponse, error) {
	// 1. Verify subdomain and get property
	var property master.Property
	err := s.masterDB.Where("subdomain = ? AND status = ?", subdomain, "active").First(&property).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid property subdomain")
		}
		return "", nil, err
	}

	// 2. Verify code using Redis
	if err := s.verificationSvc.VerifyCode(s.ctx, email, CodeTypeLogin, code); err != nil {
		return "", nil, err
	}

	// 3. Find or create user
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// New user - create and bind to current property
			user = &master.User{
				Email:                    strPtr(email),
				EmailVerified:            true,
				Status:                   "active",
				PublicEmail:              true,
				PublicPhone:              true,
				PublicFullName:           true,
				PublicPropertyCert:       true,
				PublicVehicleCROR:        true,
				EmailNotificationEnabled: true,
			}
			if err := s.userRepo.Create(user); err != nil {
				return "", nil, err
			}

			// Create user-property relationship for new user
			upr := &master.UserRole{
				UserID:     user.ID,
				PropertyID: property.ID,
				Role:       "tenant",
				Status:     "active",
			}
			if err := s.masterDB.Create(upr).Error; err != nil {
				return "", nil, err
			}
		} else {
			return "", nil, err
		}
	} else {
		// Existing user - verify email
		s.userRepo.VerifyEmail(user.ID)

		// CRITICAL: Check if user is bound to this property
		var upr master.UserRole
		err = s.masterDB.Where("user_id = ? AND property_id = ?", user.ID, property.ID).First(&upr).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// User exists but not bound to this property
				// ISOLATION POLICY: Deny access (do NOT auto-bind)
				return "", nil, errors.New("you are not authorized to access this property")
			} else {
				return "", nil, err
			}
		}

		// User is bound to this property - check if binding is active
		if upr.Status != "active" {
			return "", nil, errors.New("your access to this property has been disabled")
		}
	}

	// 4. Get user's role for this property
	role, propertyID, subdomain := s.getUserRole(user.ID)

	// 5. Generate token
	token, err := GenerateToken(user.ID, getEmailString(user.Email), role, propertyID)
	if err != nil {
		return "", nil, err
	}

	return token, &LoginResponse{
		User:               user,
		Role:               role,
		PropertyID:         propertyID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// LoginWithPhoneCode authenticates user with phone and verification code
func (s *AuthService) LoginWithPhoneCode(phone, code, subdomain string) (string, *LoginResponse, error) {
	// 1. Verify subdomain and get property
	var property master.Property
	err := s.masterDB.Where("subdomain = ? AND status = ?", subdomain, "active").First(&property).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid property subdomain")
		}
		return "", nil, err
	}

	// 2. Verify code using Redis
	if err := s.verificationSvc.VerifyCode(s.ctx, phone, CodeTypeLogin, code); err != nil {
		return "", nil, err
	}

	// 3. Find or create user
	user, err := s.userRepo.FindByPhone(phone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// New user - create and bind to current property
			user = &master.User{
				Phone:                    strPtr(phone),
				PhoneVerified:            true,
				Status:                   "active",
				PublicEmail:              true,
				PublicPhone:              true,
				PublicFullName:           true,
				PublicPropertyCert:       true,
				PublicVehicleCROR:        true,
				EmailNotificationEnabled: true,
			}
			if err := s.userRepo.Create(user); err != nil {
				return "", nil, err
			}

			// Create user-property relationship for new user
			upr := &master.UserRole{
				UserID:     user.ID,
				PropertyID: property.ID,
				Role:       "tenant",
				Status:     "active",
			}
			if err := s.masterDB.Create(upr).Error; err != nil {
				return "", nil, err
			}
		} else {
			return "", nil, err
		}
	} else {
		// Existing user - verify phone
		s.userRepo.VerifyPhone(user.ID)

		// CRITICAL: Check if user is bound to this property
		var upr master.UserRole
		err = s.masterDB.Where("user_id = ? AND property_id = ?", user.ID, property.ID).First(&upr).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// User exists but not bound to this property
				// ISOLATION POLICY: Deny access (do NOT auto-bind)
				return "", nil, errors.New("you are not authorized to access this property")
			} else {
				return "", nil, err
			}
		}

		// User is bound to this property - check if binding is active
		if upr.Status != "active" {
			return "", nil, errors.New("your access to this property has been disabled")
		}
	}

	// 4. Get user's role for this property
	role, propertyID, subdomain := s.getUserRole(user.ID)

	// 5. Generate token
	token, err := GenerateToken(user.ID, getEmailString(user.Email), role, propertyID)
	if err != nil {
		return "", nil, err
	}

	return token, &LoginResponse{
		User:               user,
		Role:               role,
		PropertyID:         propertyID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// SendVerificationCode sends a verification code to email or phone
func (s *AuthService) SendVerificationCode(identifier, codeType string) (string, error) {
	// Check if can resend (respects cooldown)
	code, err := s.verificationSvc.ResendCode(s.ctx, identifier, codeType)
	if err != nil {
		// If error is about cooldown, try generating normally
		code, err = s.verificationSvc.GenerateCode(s.ctx, identifier, codeType)
		if err != nil {
			return "", err
		}
	}

	// Determine if identifier is email or phone
	if utils.IsValidEmail(identifier) {
		// Send email using SmtpService
		if s.smtpSvc != nil {
			err = s.smtpSvc.SendVerificationCode(identifier, code, codeType)
			if err != nil {
				fmt.Printf("Failed to send email to %s: %v\n", identifier, err)
				// Don't return error - code is still valid in Redis
			}
		} else {
			// Fallback: log to console
			fmt.Printf("📧 Email verification code for %s (%s): %s\n", identifier, codeType, code)
		}
	} else {
		// Send SMS using SMSService
		if s.smsSvc != nil {
			err = s.smsSvc.SendVerificationCode(identifier, code, codeType)
			if err != nil {
				fmt.Printf("Failed to send SMS to %s: %v\n", identifier, err)
				// Don't return error - code is still valid in Redis
			}
		} else {
			// Fallback: log to console
			fmt.Printf("📱 SMS verification code for %s (%s): %s\n", identifier, codeType, code)
		}
	}

	return code, nil
}

// Register registers a new user with email and password
// If subdomain is provided, binds the user to that property
// Same email can register in different properties (creates new binding)
func (s *AuthService) Register(email, password, fullName, subdomain string) (*master.User, error) {
	// If subdomain is provided, verify the property exists first
	var property *master.Property
	if subdomain != "" {
		property = &master.Property{}
		err := s.masterDB.Where("subdomain = ? AND status = ?", subdomain, "active").First(property).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("invalid property subdomain")
			}
			return nil, err
		}
	}

	// Check if user already exists
	existingUser, err := s.userRepo.FindByEmail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var user *master.User

	if existingUser != nil {
		// User exists - check if already bound to this property
		if property != nil {
			var existingBinding master.UserRole
			err := s.masterDB.Where("user_id = ? AND property_id = ?", existingUser.ID, property.ID).
				First(&existingBinding).Error
			if err == nil {
				// Already registered in this property
				return nil, errors.New("you are already registered in this property")
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			// User exists but not bound to this property - create new binding
			upr := &master.UserRole{
				UserID:     existingUser.ID,
				PropertyID: property.ID,
				Role:       "tenant",
				Status:     "active",
			}
			if err := s.masterDB.Create(upr).Error; err != nil {
				return nil, fmt.Errorf("failed to register in property: %v", err)
			}

			// Update password if provided (user might want to use different password)
			if password != "" {
				passwordResult, err := utils.HashPasswordWithSalt(password, "")
				if err != nil {
					return nil, err
				}
				s.masterDB.Model(existingUser).Updates(map[string]interface{}{
					"password_hash": passwordResult.Hash,
					"salt":          passwordResult.Salt,
				})
			}

			return existingUser, nil
		} else {
			// No subdomain - main portal registration not allowed for existing users
			return nil, errors.New("email already registered")
		}
	}

	// New user - create account
	passwordResult, err := utils.HashPasswordWithSalt(password, "")
	if err != nil {
		return nil, err
	}

	user = &master.User{
		Email:                    strPtr(email),
		PasswordHash:             passwordResult.Hash,
		Salt:                     passwordResult.Salt,
		FullName:                 fullName,
		Status:                   "active",
		EmailVerified:            true,
		PublicEmail:              true,
		PublicPhone:              true,
		PublicFullName:           true,
		PublicPropertyCert:       true,
		PublicVehicleCROR:        true,
		EmailNotificationEnabled: true,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// If property was specified, create user-property binding
	if property != nil {
		upr := &master.UserRole{
			UserID:     user.ID,
			PropertyID: property.ID,
			Role:       "tenant",
			Status:     "active",
		}
		if err := s.masterDB.Create(upr).Error; err != nil {
			fmt.Printf("Warning: failed to create user-property binding: %v\n", err)
		}
	}

	// Send verification email
	s.SendVerificationCode(email, CodeTypeRegister)

	// Send welcome email
	if s.smtpSvc != nil {
		go s.smtpSvc.SendWelcomeEmail(email, fullName)
	}

	return user, nil
}

// ChangePassword changes user password
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	// Verify old password
	if !utils.CheckPasswordHash(oldPassword, user.PasswordHash, user.Salt) {
		return errors.New("invalid old password")
	}

	// Hash new password
	newPasswordResult, err := utils.HashPasswordWithSalt(newPassword, "")
	if err != nil {
		return err
	}

	// Update password and salt
	err = s.userRepo.UpdatePasswordAndSalt(userID, newPasswordResult.Hash, newPasswordResult.Salt)
	if err != nil {
		return err
	}

	// Clear the must_change_password flag
	return s.userRepo.UpdateFields(userID, map[string]interface{}{
		"must_change_password": false,
	})
}

// ResetPassword resets user password with verification code
func (s *AuthService) ResetPassword(identifier, code, newPassword string) error {
	// Verify code using Redis
	if err := s.verificationSvc.VerifyCode(s.ctx, identifier, CodeTypeResetPassword, code); err != nil {
		return err
	}

	// Find user
	user, err := s.userRepo.FindByEmailOrPhone(identifier)
	if err != nil {
		return errors.New("user not found")
	}

	// Hash new password
	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password
	err = s.userRepo.UpdatePassword(user.ID, passwordHash)
	if err != nil {
		return err
	}

	// Send password reset notification email
	if s.smtpSvc != nil && utils.IsValidEmail(identifier) {
		go s.smtpSvc.SendPasswordResetEmail(identifier)
	}

	return nil
}

// VerifyEmail verifies user email with code
func (s *AuthService) VerifyEmail(email, code string) error {
	// Verify code using Redis
	if err := s.verificationSvc.VerifyCode(s.ctx, email, CodeTypeVerifyEmail, code); err != nil {
		return err
	}

	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return err
	}

	return s.userRepo.VerifyEmail(user.ID)
}

// VerifyPhone verifies user phone with code
func (s *AuthService) VerifyPhone(phone, code string) error {
	// Verify code using Redis
	if err := s.verificationSvc.VerifyCode(s.ctx, phone, CodeTypeVerifyPhone, code); err != nil {
		return err
	}

	user, err := s.userRepo.FindByPhone(phone)
	if err != nil {
		return err
	}

	return s.userRepo.VerifyPhone(user.ID)
}

// getUserRole returns user's role, property_id and subdomain
func (s *AuthService) getUserRole(userID uint) (string, uint, string) {
	var upr master.UserRole
	err := s.masterDB.Where("user_id = ? AND status = ?", userID, "active").
		Order("id ASC").
		First(&upr).Error
	if err != nil {
		return "", 0, ""
	}

	// Get subdomain
	var property master.Property
	err = s.masterDB.Where("id = ?", upr.PropertyID).First(&property).Error
	if err != nil {
		return upr.Role, upr.PropertyID, ""
	}

	return upr.Role, upr.PropertyID, property.Subdomain
}

// LoginWithEmailPasswordProperty authenticates user with email/password and verifies property access
func (s *AuthService) LoginWithEmailPasswordProperty(email, password, subdomain string) (string, *LoginResponse, error) {
	// 1. Verify subdomain and get property
	var property master.Property
	err := s.masterDB.Where("subdomain = ? AND status = ?", subdomain, "active").First(&property).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid property subdomain")
		}
		return "", nil, err
	}

	// 2. Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid email or password#7")
		}
		return "", nil, err
	}

	// 3. Check password
	if !utils.CheckPasswordHash(password, user.PasswordHash, user.Salt) {
		return "", nil, errors.New("invalid email or password#8")
	}

	// 4. Check if user is active
	if !user.IsActive() {
		return "", nil, errors.New("account is not active")
	}

	// 5. CRITICAL: Check if user is bound to this property
	var upr master.UserRole
	err = s.masterDB.Where("user_id = ? AND property_id = ?", user.ID, property.ID).First(&upr).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User exists but not bound to this property
			// ISOLATION POLICY: Deny access
			return "", nil, errors.New("you are not authorized to access this property")
		}
		return "", nil, err
	}

	// 6. Check if binding is active
	if upr.Status != "active" {
		return "", nil, errors.New("your access to this property has been disabled")
	}

	// 7. Generate JWT token with property info
	token, err := GenerateToken(user.ID, getEmailString(user.Email), upr.Role, property.ID)
	if err != nil {
		return "", nil, err
	}

	return token, &LoginResponse{
		User:               user,
		Role:               upr.Role,
		PropertyID:         property.ID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
	}, nil
}

// LoginWithPhonePasswordProperty authenticates user with phone/password and verifies property access
func (s *AuthService) LoginWithPhonePasswordProperty(phone, password, subdomain string) (string, *LoginResponse, error) {
	// 1. Verify subdomain and get property
	var property master.Property
	err := s.masterDB.Where("subdomain = ? AND status = ?", subdomain, "active").First(&property).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid property subdomain")
		}
		return "", nil, err
	}

	// 2. Find user by phone
	user, err := s.userRepo.FindByPhone(phone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, errors.New("invalid phone or password")
		}
		return "", nil, err
	}

	// 3. Check password
	if !utils.CheckPasswordHash(password, user.PasswordHash, user.Salt) {
		return "", nil, errors.New("invalid phone or password")
	}

	// 4. Check if user is active
	if !user.IsActive() {
		return "", nil, errors.New("account is not active")
	}

	// 5. CRITICAL: Check if user is bound to this property
	var upr master.UserRole
	err = s.masterDB.Where("user_id = ? AND property_id = ?", user.ID, property.ID).First(&upr).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// User exists but not bound to this property
			// ISOLATION POLICY: Deny access
			return "", nil, errors.New("you are not authorized to access this property")
		}
		return "", nil, err
	}

	// 6. Check if binding is active
	if upr.Status != "active" {
		return "", nil, errors.New("your access to this property has been disabled")
	}

	// 7. Generate JWT token with property info
	token, err := GenerateToken(user.ID, getEmailString(user.Email), upr.Role, property.ID)
	if err != nil {
		return "", nil, err
	}

	return token, &LoginResponse{
		User:               user,
		Role:               upr.Role,
		PropertyID:         property.ID,
		Subdomain:          subdomain,
		MustChangePassword: user.MustChangePassword,
	}, nil
}
