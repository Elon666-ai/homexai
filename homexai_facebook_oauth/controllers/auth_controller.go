package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"homexai_oauth/models"
	"homexai_oauth/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService *services.AuthService
}

func NewAuthController(authService *services.AuthService) *AuthController {
	return &AuthController{
		authService: authService,
	}
}

// 生成随机state用于防止CSRF攻击
func generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ==================== Google OAuth ====================

// GoogleLogin 发起Google OAuth登录
func (ac *AuthController) GoogleLogin(c *gin.Context) {
	state, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate state",
		})
		return
	}

	authURL := ac.authService.GetGoogleAuthURL(state)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

// GoogleCallback 处理Google OAuth回调
func (ac *AuthController) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Authorization code is required",
		})
		return
	}

	log.Printf("Received Google callback with state: %s", state)

	// 交换授权码获取访问令牌
	token, err := ac.authService.ExchangeGoogleCode(code)
	if err != nil {
		log.Printf("Failed to exchange code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to exchange authorization code",
		})
		return
	}

	// 获取Google用户信息
	googleUser, err := ac.authService.GetGoogleUserInfo(token)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user information",
		})
		return
	}

	// 创建或更新用户
	user, err := ac.authService.CreateOrUpdateUser(
		googleUser.ID,
		models.ProviderGoogle,
		googleUser.Email,
		googleUser.Name,
		googleUser.Picture,
		googleUser.VerifiedEmail,
	)
	if err != nil {
		log.Printf("Failed to create/update user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process user",
		})
		return
	}

	// 生成JWT token
	jwtToken, err := ac.authService.GenerateToken(user)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate authentication token",
		})
		return
	}

	// 重定向到前端，携带token
	redirectURL := fmt.Sprintf("http://localhost:8080/auth-success.html?token=%s&provider=google", jwtToken)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// ==================== Facebook OAuth ====================

// FacebookLogin 发起Facebook OAuth登录
func (ac *AuthController) FacebookLogin(c *gin.Context) {
	state, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate state",
		})
		return
	}

	authURL := ac.authService.GetFacebookAuthURL(state)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

// FacebookCallback 处理Facebook OAuth回调
func (ac *AuthController) FacebookCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Authorization code is required",
		})
		return
	}

	log.Printf("Received Facebook callback with state: %s", state)

	// 交换授权码获取访问令牌
	token, err := ac.authService.ExchangeFacebookCode(code)
	if err != nil {
		log.Printf("Failed to exchange code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to exchange authorization code",
		})
		return
	}

	// 获取Facebook用户信息
	fbUser, err := ac.authService.GetFacebookUserInfo(token)
	if err != nil {
		log.Printf("Failed to get user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user information",
		})
		return
	}

	// Facebook的email可能为空（用户可以选择不分享）
	email := fbUser.Email
	if email == "" {
		email = fmt.Sprintf("%s@facebook.user", fbUser.ID)
	}

	// 创建或更新用户
	user, err := ac.authService.CreateOrUpdateUser(
		fbUser.ID,
		models.ProviderFacebook,
		email,
		fbUser.Name,
		fbUser.Picture.Data.URL,
		fbUser.Email != "", // 如果有email说明已验证
	)
	if err != nil {
		log.Printf("Failed to create/update user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process user",
		})
		return
	}

	// 生成JWT token
	jwtToken, err := ac.authService.GenerateToken(user)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate authentication token",
		})
		return
	}

	// 重定向到前端，携带token
	redirectURL := fmt.Sprintf("http://localhost:8080/auth-success.html?token=%s&provider=facebook", jwtToken)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// ==================== 通用方法 ====================

// GetProfile 获取当前用户信息（需要认证）
func (ac *AuthController) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	user, err := ac.authService.GetUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// VerifyToken 验证token有效性
func (ac *AuthController) VerifyToken(c *gin.Context) {
	var request struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Token is required",
		})
		return
	}

	claims, err := ac.authService.ValidateToken(request.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"valid": false,
			"error": "Invalid or expired token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":    true,
		"user_id":  claims.UserID,
		"email":    claims.Email,
		"provider": claims.Provider,
	})
}
