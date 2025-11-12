package controllers

import (
	"homexai_email_auth/services"
	"log"
	"net/http"
	"regexp"

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

// 验证邮箱格式
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// SendVerificationCode 发送验证码
func (ac *AuthController) SendVerificationCode(c *gin.Context) {
	var request struct {
		Email string `json:"email" binding:"required"`
		Type  string `json:"type" binding:"required"` // register, reset_password
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	// 验证邮箱格式
	if !isValidEmail(request.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "邮箱格式不正确",
		})
		return
	}

	// 验证type
	if request.Type != "register" && request.Type != "reset_password" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "验证码类型错误",
		})
		return
	}

	// 发送验证码
	if err := ac.authService.SendVerificationCode(request.Email, request.Type); err != nil {
		log.Printf("Failed to send verification code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "验证码已发送，请查收邮件",
	})
}

// Register 用户注册
func (ac *AuthController) Register(c *gin.Context) {
	var request struct {
		Email            string `json:"email" binding:"required"`
		Password         string `json:"password" binding:"required"`
		Name             string `json:"name" binding:"required"`
		VerificationCode string `json:"verification_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	// 验证邮箱格式
	if !isValidEmail(request.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "邮箱格式不正确",
		})
		return
	}

	// 验证密码长度
	if len(request.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "密码长度至少6位",
		})
		return
	}

	// 注册用户
	user, err := ac.authService.Register(
		request.Email,
		request.Password,
		request.Name,
		request.VerificationCode,
	)
	if err != nil {
		log.Printf("Failed to register user: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 生成Token
	token, err := ac.authService.GenerateJWT(user.ID, user.Email)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "注册成功，但生成令牌失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"user":    user,
		"token":   token,
	})
}

// Login 用户登录
func (ac *AuthController) Login(c *gin.Context) {
	var request struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	// 验证邮箱格式
	if !isValidEmail(request.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "邮箱格式不正确",
		})
		return
	}

	// 登录
	user, token, err := ac.authService.Login(request.Email, request.Password)
	if err != nil {
		log.Printf("Failed to login: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "邮箱或密码错误",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"user":    user,
		"token":   token,
	})
}

// RequestPasswordReset 请求密码重置
func (ac *AuthController) RequestPasswordReset(c *gin.Context) {
	var request struct {
		Email string `json:"email" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	// 验证邮箱格式
	if !isValidEmail(request.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "邮箱格式不正确",
		})
		return
	}

	// 请求密码重置
	if err := ac.authService.RequestPasswordReset(request.Email); err != nil {
		log.Printf("Failed to request password reset: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "发送重置邮件失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "如果该邮箱已注册，您将收到密码重置邮件",
	})
}

// ResetPassword 重置密码
func (ac *AuthController) ResetPassword(c *gin.Context) {
	var request struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	// 验证密码长度
	if len(request.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "密码长度至少6位",
		})
		return
	}

	// 重置密码
	if err := ac.authService.ResetPassword(request.Token, request.NewPassword); err != nil {
		log.Printf("Failed to reset password: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "密码重置成功",
	})
}

// GetProfile 获取用户信息（需要认证）
func (ac *AuthController) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未授权",
		})
		return
	}

	user, err := ac.authService.GetUserByID(userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// VerifyToken 验证Token
func (ac *AuthController) VerifyToken(c *gin.Context) {
	var request struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误",
		})
		return
	}

	userID, err := ac.authService.ValidateJWT(request.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"valid": false,
			"error": "Token无效或已过期",
		})
		return
	}

	user, err := ac.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"valid": false,
			"error": "用户不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":  true,
		"user_id": user.ID,
		"email":   user.Email,
	})
}
