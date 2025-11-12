package services

import (
	"context"
	"encoding/json"
	"fmt"
	"homexai_oauth/config"
	"homexai_oauth/database"
	"homexai_oauth/models"
	"homexai_oauth/utils"
	"io"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

type AuthService struct {
	config             *config.Config
	googleOAuthConfig  *oauth2.Config
	facebookOAuthConfig *oauth2.Config
}

func NewAuthService(cfg *config.Config) *AuthService {
	// Google OAuth配置
	googleConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	// Facebook OAuth配置
	facebookConfig := &oauth2.Config{
		ClientID:     cfg.FacebookAppID,
		ClientSecret: cfg.FacebookAppSecret,
		RedirectURL:  cfg.FacebookRedirectURL,
		Scopes: []string{
			"email",
			"public_profile",
		},
		Endpoint: facebook.Endpoint,
	}

	return &AuthService{
		config:              cfg,
		googleOAuthConfig:   googleConfig,
		facebookOAuthConfig: facebookConfig,
	}
}

// Google OAuth方法
func (s *AuthService) GetGoogleAuthURL(state string) string {
	return s.googleOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *AuthService) ExchangeGoogleCode(code string) (*oauth2.Token, error) {
	return s.googleOAuthConfig.Exchange(context.Background(), code)
}

func (s *AuthService) GetGoogleUserInfo(token *oauth2.Token) (*models.GoogleUserInfo, error) {
	client := s.googleOAuthConfig.Client(context.Background(), token)
	
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo models.GoogleUserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &userInfo, nil
}

// Facebook OAuth方法
func (s *AuthService) GetFacebookAuthURL(state string) string {
	return s.facebookOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *AuthService) ExchangeFacebookCode(code string) (*oauth2.Token, error) {
	return s.facebookOAuthConfig.Exchange(context.Background(), code)
}

func (s *AuthService) GetFacebookUserInfo(token *oauth2.Token) (*models.FacebookUserInfo, error) {
	client := s.facebookOAuthConfig.Client(context.Background(), token)
	
	// Facebook Graph API - 获取用户信息
	resp, err := client.Get("https://graph.facebook.com/me?fields=id,name,email,picture")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo models.FacebookUserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &userInfo, nil
}

// 创建或更新用户（通用方法）
func (s *AuthService) CreateOrUpdateUser(providerID string, provider models.OAuthProvider, email, name, picture string, emailVerified bool) (*models.User, error) {
	db := database.GetDB()
	
	var user models.User
	result := db.Where("provider = ? AND provider_id = ?", provider, providerID).First(&user)
	
	now := time.Now()
	
	if result.RowsAffected == 0 {
		// 创建新用户
		user = models.User{
			Provider:      provider,
			ProviderID:    providerID,
			Email:         email,
			Name:          name,
			Picture:       picture,
			EmailVerified: emailVerified,
			LastLoginAt:   now,
		}
		
		if err := db.Create(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		// 更新现有用户
		user.Email = email
		user.Name = name
		user.Picture = picture
		user.EmailVerified = emailVerified
		user.LastLoginAt = now
		
		if err := db.Save(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	return &user, nil
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	return utils.GenerateJWT(user.ID, user.Email, user.Provider, s.config.JWTSecret)
}

func (s *AuthService) ValidateToken(tokenString string) (*utils.Claims, error) {
	return utils.ValidateJWT(tokenString, s.config.JWTSecret)
}

func (s *AuthService) GetUserByID(userID uint) (*models.User, error) {
	db := database.GetDB()
	
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}
