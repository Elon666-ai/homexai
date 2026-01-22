package handler

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"homexai/internal/config"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

type OAuthHandler struct {
	oauthService *service.OAuthService
	frontendURL  string
}

func NewOAuthHandler() *OAuthHandler {
	googleConfig := &service.GoogleOAuthConfig{
		ClientID:     config.Yaml.OAuth.Google.ClientID,
		ClientSecret: config.Yaml.OAuth.Google.ClientSecret,
		RedirectURL:  config.Yaml.OAuth.Google.RedirectURL,
	}

	fbConfig := &service.FacebookOAuthConfig{
		AppID:       config.Yaml.OAuth.Facebook.ClientID,
		AppSecret:   config.Yaml.OAuth.Facebook.ClientSecret,
		RedirectURL: config.Yaml.OAuth.Facebook.RedirectURL,
	}

	return &OAuthHandler{
		oauthService: service.NewOAuthService(googleConfig, fbConfig),
		frontendURL:  config.Yaml.Server.FrontendURL,
	}
}

// GoogleLogin initiates Google OAuth flow
// @Summary Google OAuth login
// @Description Redirects user to Google OAuth authorization page
// @Tags auth
// @Produce json
// @Success 302 {string} string "Redirect to Google"
// @Router /api/v1/auth/google [get]
func (h *OAuthHandler) GoogleLogin(c *gin.Context) {
	state := generateStateToken()

	// Store state in session/cookie for verification
	c.SetCookie(
		"oauth_state",
		state,
		300, // 5 minutes
		"/",
		"",
		false,
		true, // HttpOnly
	)

	authURL := h.oauthService.GetGoogleAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// GoogleCallback handles Google OAuth callback
// @Summary Google OAuth callback
// @Description Handles callback from Google OAuth
// @Tags auth
// @Produce json
// @Param code query string true "Authorization code"
// @Param state query string true "State token"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/google/callback [get]
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	// Verify state
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil {
		h.redirectToFrontendWithError(c, "Invalid state token")
		return
	}

	state := c.Query("state")
	if state != stateCookie {
		h.redirectToFrontendWithError(c, "State token mismatch")
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	code := c.Query("code")
	if code == "" {
		h.redirectToFrontendWithError(c, "Authorization code not provided")
		return
	}

	// Handle OAuth callback
	token, user, err := h.oauthService.HandleGoogleCallback(code)
	if err != nil {
		h.redirectToFrontendWithError(c, fmt.Sprintf("OAuth failed: %v", err))
		return
	}

	// Set token in cookie for web apps
	c.SetCookie(
		"auth_token",
		token,
		86400*7, // 7 days
		"/",
		"",
		false,
		true, // HttpOnly
	)

	// Redirect to frontend callback page with token
	h.redirectToFrontendWithToken(c, token, user)
}

// FacebookLogin initiates Facebook OAuth flow
// @Summary Facebook OAuth login
// @Description Redirects user to Facebook OAuth authorization page
// @Tags auth
// @Produce json
// @Success 302 {string} string "Redirect to Facebook"
// @Router /api/v1/auth/facebook [get]
func (h *OAuthHandler) FacebookLogin(c *gin.Context) {
	state := generateStateToken()

	// Store state in session/cookie for verification
	c.SetCookie(
		"oauth_state",
		state,
		300, // 5 minutes
		"/",
		"",
		false,
		true, // HttpOnly
	)

	authURL := h.oauthService.GetFacebookAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// FacebookCallback handles Facebook OAuth callback
// @Summary Facebook OAuth callback
// @Description Handles callback from Facebook OAuth
// @Tags auth
// @Produce json
// @Param code query string true "Authorization code"
// @Param state query string true "State token"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/facebook/callback [get]
func (h *OAuthHandler) FacebookCallback(c *gin.Context) {
	// Verify state
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil {
		h.redirectToFrontendWithError(c, "Invalid state token")
		return
	}

	state := c.Query("state")
	if state != stateCookie {
		h.redirectToFrontendWithError(c, "State token mismatch")
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	code := c.Query("code")
	if code == "" {
		h.redirectToFrontendWithError(c, "Authorization code not provided")
		return
	}

	// Handle OAuth callback
	token, user, err := h.oauthService.HandleFacebookCallback(code)
	if err != nil {
		h.redirectToFrontendWithError(c, fmt.Sprintf("OAuth failed: %v", err))
		return
	}

	// Set token in cookie for web apps
	c.SetCookie(
		"auth_token",
		token,
		86400*7, // 7 days
		"/",
		"",
		false,
		true, // HttpOnly
	)

	// Redirect to frontend callback page with token
	h.redirectToFrontendWithToken(c, token, user)
}

// UnlinkProvider unlinks OAuth provider from user
// @Summary Unlink OAuth provider
// @Description Removes OAuth provider connection from user account
// @Tags auth
// @Accept json
// @Produce json
// @Param provider path string true "Provider name (google/facebook)"
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/oauth/unlink/:provider [delete]
func (h *OAuthHandler) UnlinkProvider(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	provider := c.Param("provider")

	if provider != "google" && provider != "facebook" {
		utils.BadRequestResponse(c, "Invalid provider", nil)
		return
	}

	err := h.oauthService.UnlinkProvider(userID.(uint), provider)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to unlink provider", err)
		return
	}

	utils.SuccessResponse(c, "Provider unlinked successfully", nil)
}

// GetLinkedProviders gets user's linked OAuth providers
// @Summary Get linked OAuth providers
// @Description Returns list of OAuth providers linked to user account
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/oauth/providers [get]
func (h *OAuthHandler) GetLinkedProviders(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	providers, err := h.oauthService.GetUserProviders(userID.(uint))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get providers", err)
		return
	}

	// Format response
	result := make([]gin.H, len(providers))
	for i, p := range providers {
		result[i] = gin.H{
			"provider":  p.Provider,
			"linkedAt":  p.CreatedAt,
			"isExpired": p.IsTokenExpired(),
		}
	}

	utils.SuccessResponse(c, "Providers retrieved successfully", gin.H{
		"providers": result,
	})
}

// Helper functions

// redirectToFrontendWithToken redirects to frontend with success token
func (h *OAuthHandler) redirectToFrontendWithToken(c *gin.Context, token string, user interface{}) {
	// Build frontend callback URL with token
	callbackURL := fmt.Sprintf("%s/auth/callback?token=%s", h.frontendURL, url.QueryEscape(token))
	c.Redirect(http.StatusTemporaryRedirect, callbackURL)
}

// redirectToFrontendWithError redirects to frontend with error message
func (h *OAuthHandler) redirectToFrontendWithError(c *gin.Context, errorMsg string) {
	// Build frontend login URL with error
	loginURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape(errorMsg))
	c.Redirect(http.StatusTemporaryRedirect, loginURL)
}

// generateStateToken generates a random state token for OAuth
func generateStateToken() string {
	// Generate 32 random bytes
	b := make([]byte, 32)
	rand.Read(b)

	// Encode to base64
	state := base64.URLEncoding.EncodeToString(b)

	// Add timestamp to make it unique
	state = fmt.Sprintf("%s_%d", state, time.Now().Unix())

	return state
}
