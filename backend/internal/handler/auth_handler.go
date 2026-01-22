package handler

import (
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(smtpSvc *service.SmtpService, smsSvc *service.SMSService) *AuthHandler {
	return &AuthHandler{
		authService: service.NewAuthService(smtpSvc, smsSvc),
	}
}

// LoginRequest represents login request body
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // email or phone
	Password   string `json:"password"`
	Code       string `json:"code"`
	LoginType  string `json:"login_type" binding:"required,oneof=password code"` // password or code
	Subdomain  string `json:"subdomain"`                                         // required for code login
	RememberMe bool   `json:"rememberMe"`                                        // extend token expiration to 30 days
}

// RegisterRequest represents registration request body
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FullName  string `json:"full_name" binding:"required"`
	Subdomain string `json:"subdomain"` // Property subdomain - required for property user registration
}

// SendCodeRequest represents send code request body
type SendCodeRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	CodeType   string `json:"code_type" binding:"required,oneof=login register reset_password"`
}

// ChangePasswordRequest represents change password request body
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ResetPasswordRequest represents reset password request body
type ResetPasswordRequest struct {
	Identifier  string `json:"identifier" binding:"required"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// VerifyRequest represents verification request body
type VerifyRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Code       string `json:"code" binding:"required"`
}

// Login handles user login
// @Summary User login
// @Description Login with email/phone and password or verification code
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// CRITICAL: Subdomain is ALWAYS required
	// Users must access via a valid subdomain (e.g., admin.homex.ph or demo.homex.ph)
	if req.Subdomain == "" {
		utils.BadRequestResponse(c, "Please access via a valid subdomain (e.g., admin.homex.ph or demo.homex.ph)", nil)
		return
	}

	var token string
	var loginResp *service.LoginResponse
	var err error

	// Check if this is main portal (admin subdomain)
	isMainPortal := req.Subdomain == "admin"
	if req.LoginType == "password" {
		if isMainPortal {
			// Main portal password login - only super_admin allowed
			if utils.IsEmail(req.Identifier) {
				token, loginResp, err = h.authService.LoginWithEmailPassword(req.Identifier, req.Password)
			} else {
				token, loginResp, err = h.authService.LoginWithPhonePassword(req.Identifier, req.Password)
			}
		} else {
			// Property-specific password login - check user binding
			if utils.IsEmail(req.Identifier) {
				token, loginResp, err = h.authService.LoginWithEmailPasswordProperty(req.Identifier, req.Password, req.Subdomain)
			} else {
				token, loginResp, err = h.authService.LoginWithPhonePasswordProperty(req.Identifier, req.Password, req.Subdomain)
			}
		}
	} else {
		// Code login - only allowed on property portals, not main portal
		if isMainPortal {
			utils.BadRequestResponse(c, "Code login is not available on the admin portal", nil)
			return
		}

		if utils.IsEmail(req.Identifier) {
			token, loginResp, err = h.authService.LoginWithEmailCode(req.Identifier, req.Code, req.Subdomain)
		} else {
			token, loginResp, err = h.authService.LoginWithPhoneCode(req.Identifier, req.Code, req.Subdomain)
		}
	}

	if err != nil {
		utils.UnauthorizedResponse(c, err.Error())
		return
	}

	// If rememberMe is true, regenerate token with extended expiration (30 days)
	if req.RememberMe {
		// Safely get email string from pointer
		email := ""
		if loginResp.User.Email != nil {
			email = *loginResp.User.Email
		}
		token, err = service.GenerateTokenWithRemember(
			loginResp.User.ID,
			email,
			loginResp.Role,
			loginResp.PropertyID,
			true,
		)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to generate extended token", nil)
			return
		}
	}

	utils.SuccessResponse(c, "Login successful", gin.H{
		"token":                token,
		"user":                 loginResp.User,
		"role":                 loginResp.Role,
		"property_id":          loginResp.PropertyID,
		"subdomain":            loginResp.Subdomain,
		"must_change_password": loginResp.MustChangePassword,
	})
}

// Register handles user registration
// @Summary User registration
// @Description Register a new user account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration request"
// @Success 201 {object} map[string]interface{}
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	user, err := h.authService.Register(req.Email, req.Password, req.FullName, req.Subdomain)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.CreatedResponse(c, "Registration successful. Please check your email for verification.", user)
}

// SendCode handles sending verification code
// @Summary Send verification code
// @Description Send verification code to email or phone
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body SendCodeRequest true "Send code request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/send-code [post]
func (h *AuthHandler) SendCode(c *gin.Context) {
	var req SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	_, err := h.authService.SendVerificationCode(req.Identifier, req.CodeType)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to send verification code", err)
		return
	}

	utils.SuccessResponse(c, "Verification code sent successfully", nil)
}

// ChangePassword handles password change
// @Summary Change password
// @Description Change user password (requires authentication)
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "Change password request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err := h.authService.ChangePassword(userID.(uint), req.OldPassword, req.NewPassword)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Password changed successfully", nil)
}

// ResetPassword handles password reset
// @Summary Reset password
// @Description Reset password with verification code
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "Reset password request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err := h.authService.ResetPassword(req.Identifier, req.Code, req.NewPassword)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Password reset successfully", nil)
}

// VerifyEmail handles email verification
// @Summary Verify email
// @Description Verify user email with code
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body VerifyRequest true "Verify request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err := h.authService.VerifyEmail(req.Identifier, req.Code)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Email verified successfully", nil)
}

// VerifyPhone handles phone verification
// @Summary Verify phone
// @Description Verify user phone with code
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body VerifyRequest true "Verify request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/verify-phone [post]
func (h *AuthHandler) VerifyPhone(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	err := h.authService.VerifyPhone(req.Identifier, req.Code)
	if err != nil {
		utils.BadRequestResponse(c, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Phone verified successfully", nil)
}

// Logout handles user logout
// @Summary User logout
// @Description Logout current user
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// Token invalidation could be implemented here if needed
	utils.SuccessResponse(c, "Logout successful", nil)
}
