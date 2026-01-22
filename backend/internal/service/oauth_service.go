package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"homexai/internal/database"
	"homexai/internal/models/master"
	masterRepo "homexai/internal/repository/master"

	"gorm.io/gorm"
)

// Helper function to convert string to *string
func strPtrOAuth(s string) *string {
	return &s
}

type OAuthService struct {
	userRepo     *masterRepo.UserRepository
	oauthRepo    *masterRepo.OAuthProviderRepository
	googleConfig *GoogleOAuthConfig
	fbConfig     *FacebookOAuthConfig
}

// GoogleOAuthConfig holds Google OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// FacebookOAuthConfig holds Facebook OAuth configuration
type FacebookOAuthConfig struct {
	AppID       string
	AppSecret   string
	RedirectURL string
}

// GoogleUserInfo represents user info from Google
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// FacebookUserInfo represents user info from Facebook
type FacebookUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

// OAuthTokenResponse represents OAuth token response
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func NewOAuthService(googleConfig *GoogleOAuthConfig, fbConfig *FacebookOAuthConfig) *OAuthService {
	masterDB := database.GetMasterGormDB()
	return &OAuthService{
		userRepo:     masterRepo.NewUserRepository(masterDB),
		oauthRepo:    masterRepo.NewOAuthProviderRepository(masterDB),
		googleConfig: googleConfig,
		fbConfig:     fbConfig,
	}
}

// GetGoogleAuthURL generates Google OAuth authorization URL
func (s *OAuthService) GetGoogleAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", s.googleConfig.ClientID)
	params.Add("redirect_uri", s.googleConfig.RedirectURL)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile")
	params.Add("state", state)
	params.Add("access_type", "offline")
	params.Add("prompt", "consent")

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// GetFacebookAuthURL generates Facebook OAuth authorization URL
func (s *OAuthService) GetFacebookAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", s.fbConfig.AppID)
	params.Add("redirect_uri", s.fbConfig.RedirectURL)
	params.Add("response_type", "code")
	params.Add("scope", "email,public_profile")
	params.Add("state", state)

	return "https://www.facebook.com/v18.0/dialog/oauth?" + params.Encode()
}

// HandleGoogleCallback handles Google OAuth callback
func (s *OAuthService) HandleGoogleCallback(code string) (string, *master.User, error) {
	// Exchange code for token
	token, err := s.exchangeGoogleCode(code)
	if err != nil {
		return "", nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get user info
	userInfo, err := s.getGoogleUserInfo(token.AccessToken)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Find or create user
	user, err := s.findOrCreateOAuthUser(
		userInfo.Email,
		userInfo.Name,
		"google",
		userInfo.ID,
		token.AccessToken,
		token.RefreshToken,
		token.ExpiresIn,
	)
	if err != nil {
		return "", nil, err
	}

	// Generate JWT token
	jwtToken, err := GenerateToken(user.ID, getEmailString(user.Email), "", 0)
	if err != nil {
		return "", nil, err
	}

	return jwtToken, user, nil
}

// HandleFacebookCallback handles Facebook OAuth callback
func (s *OAuthService) HandleFacebookCallback(code string) (string, *master.User, error) {
	// Exchange code for token
	token, err := s.exchangeFacebookCode(code)
	if err != nil {
		return "", nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get user info
	userInfo, err := s.getFacebookUserInfo(token.AccessToken)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Find or create user
	user, err := s.findOrCreateOAuthUser(
		userInfo.Email,
		userInfo.Name,
		"facebook",
		userInfo.ID,
		token.AccessToken,
		token.RefreshToken,
		token.ExpiresIn,
	)
	if err != nil {
		return "", nil, err
	}

	// Generate JWT token
	jwtToken, err := GenerateToken(user.ID, getEmailString(user.Email), "", 0)
	if err != nil {
		return "", nil, err
	}

	return jwtToken, user, nil
}

// exchangeGoogleCode exchanges authorization code for access token
func (s *OAuthService) exchangeGoogleCode(code string) (*OAuthTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", s.googleConfig.ClientID)
	data.Set("client_secret", s.googleConfig.ClientSecret)
	data.Set("redirect_uri", s.googleConfig.RedirectURL)
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// exchangeFacebookCode exchanges authorization code for access token
func (s *OAuthService) exchangeFacebookCode(code string) (*OAuthTokenResponse, error) {
	params := url.Values{}
	params.Set("client_id", s.fbConfig.AppID)
	params.Set("client_secret", s.fbConfig.AppSecret)
	params.Set("redirect_uri", s.fbConfig.RedirectURL)
	params.Set("code", code)

	tokenURL := "https://graph.facebook.com/v18.0/oauth/access_token?" + params.Encode()

	resp, err := http.Get(tokenURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// getGoogleUserInfo fetches user info from Google
func (s *OAuthService) getGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// getFacebookUserInfo fetches user info from Facebook
func (s *OAuthService) getFacebookUserInfo(accessToken string) (*FacebookUserInfo, error) {
	url := fmt.Sprintf(
		"https://graph.facebook.com/v18.0/me?fields=id,name,email,picture&access_token=%s",
		accessToken,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	var userInfo FacebookUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// findOrCreateOAuthUser finds or creates user with OAuth provider
func (s *OAuthService) findOrCreateOAuthUser(
	email, fullName, provider, providerUserID, accessToken, refreshToken string,
	expiresIn int,
) (*master.User, error) {
	// Check if OAuth provider already exists
	oauthProvider, err := s.oauthRepo.FindByProviderAndUserID(provider, providerUserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var user *master.User

	if oauthProvider != nil {
		// Existing OAuth connection, get user
		user, err = s.userRepo.FindByID(oauthProvider.UserID)
		if err != nil {
			return nil, err
		}

		// Update OAuth tokens
		tokenExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		err = s.oauthRepo.UpdateTokens(oauthProvider.ID, accessToken, refreshToken, &tokenExpiresAt)
		if err != nil {
			return nil, err
		}
	} else {
		// Try to find user by email
		user, err = s.userRepo.FindByEmail(email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		if user == nil {
			// Create new user - use helper function for pointer
			user = &master.User{
				Email:                  strPtrOAuth(email),
				FullName:               fullName,
				EmailVerified:          true, // OAuth email is already verified
				Status:                 "active",
				PublicEmail:            true,
				PublicPhone:            true,
				PublicFullName:         true,
				PublicPropertyCert:     true,
				PublicVehicleCROR:      true,
				EmailNotificationEnabled: true,
			}
			if err := s.userRepo.Create(user); err != nil {
				return nil, err
			}
		}

		// Create OAuth provider record - use pointers for nullable fields
		tokenExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
		var accessTokenPtr, refreshTokenPtr *string
		if accessToken != "" {
			accessTokenPtr = &accessToken
		}
		if refreshToken != "" {
			refreshTokenPtr = &refreshToken
		}
		oauthProvider = &master.OAuthProvider{
			UserID:         user.ID,
			Provider:       provider,
			ProviderUserID: providerUserID,
			AccessToken:    accessTokenPtr,
			RefreshToken:   refreshTokenPtr,
			TokenExpiresAt: &tokenExpiresAt,
		}
		if err := s.oauthRepo.Create(oauthProvider); err != nil {
			return nil, err
		}
	}

	return user, nil
}

// UnlinkProvider removes OAuth provider from user
func (s *OAuthService) UnlinkProvider(userID uint, provider string) error {
	return s.oauthRepo.DeleteByUserIDAndProvider(userID, provider)
}

// GetUserProviders gets all OAuth providers linked to user
func (s *OAuthService) GetUserProviders(userID uint) ([]master.OAuthProvider, error) {
	return s.oauthRepo.FindByUserID(userID)
}
