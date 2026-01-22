package handler

import (
	"strings"

	"homexai/internal/middleware"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

// RefreshTokenRequest represents refresh token request body
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken handles token refresh
// @Summary Refresh access token
// @Description Get a new access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token request"
// @Success 200 {object} map[string]interface{}
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	// Generate new token pair
	tokenPair, err := service.RefreshAccessTokenEnhanced(req.RefreshToken)
	if err != nil {
		utils.UnauthorizedResponse(c, "Invalid or expired refresh token")
		return
	}

	utils.SuccessResponse(c, "RefreshToken successful", tokenPair)
}

// LogoutEnhanced handles user logout with token revocation
// @Summary User logout
// @Description Logout current user and revoke token
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /auth/logout [post]
func (h *AuthHandler) LogoutEnhanced(c *gin.Context) {
	// Get token from header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Extract token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]
			// Revoke the token
			if err := service.RevokeToken(token); err != nil {
				utils.InternalServerErrorResponse(c, "Failed to revoke token", err)
				return
			}
		}
	}

	utils.SuccessResponse(c, "Logout successful", nil)
}

// LogoutAllDevices handles logout from all devices
// @Summary Logout from all devices
// @Description Revoke all tokens for current user
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /auth/logout-all [post]
func (h *AuthHandler) LogoutAllDevices(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	// Revoke all tokens for this user
	if err := service.RevokeAllUserTokens(userID); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to logout from all devices", err)
		return
	}

	utils.SuccessResponse(c, "Logged out from all devices successfully", nil)
}
