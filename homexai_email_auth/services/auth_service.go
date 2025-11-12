package services

import (
	"crypto/rand"
	"fmt"
	"homexai_email_auth/config"
	"homexai_email_auth/database"
	"homexai_email_auth/models"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	config       *config.Config
	emailService *EmailService
}

func NewAuthService(cfg *config.Config, emailService *EmailService) *AuthService {
	return &AuthService{
		config:       cfg,
		emailService: emailService,
	}
}

// GenerateVerificationCode 生成6位数字验证码
func (s *AuthService) GenerateVerificationCode() string {
	code := ""
	for i := 0; i < 6; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		code += fmt.Sprintf("%d", n)
	}
	return code
}

// GenerateResetToken 生成密码重置令牌
func (s *AuthService) GenerateResetToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// SendVerificationCode 发送验证码
func (s *AuthService) SendVerificationCode(email, codeType string) error {
	db := database.GetDB()
	
	// 检查是否有未过期的验证码
	var existingCode models.VerificationCode
	result := db.Where("email = ? AND type = ? AND used = ? AND expires_at > ?",
		email, codeType, false, time.Now()).First(&existingCode)
	
	if result.RowsAffected > 0 {
		// 如果1分钟内已发送，不重复发送
		if time.Since(existingCode.CreatedAt) < time.Minute {
			return fmt.Errorf("验证码已发送，请1分钟后再试")
		}
	}
	
	// 生成新验证码
	code := s.GenerateVerificationCode()
	expiresAt := time.Now().Add(time.Duration(s.config.VerificationCodeExpireMinutes) * time.Minute)
	
	verificationCode := models.VerificationCode{
		Email:     email,
		Code:      code,
		Type:      codeType,
		ExpiresAt: expiresAt,
		Used:      false,
	}
	
	if err := db.Create(&verificationCode).Error; err != nil {
		return fmt.Errorf("failed to create verification code: %w", err)
	}
	
	// 发送邮件
	if err := s.emailService.SendVerificationCode(email, code); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	return nil
}

// VerifyCode 验证验证码
func (s *AuthService) VerifyCode(email, code, codeType string) error {
	db := database.GetDB()
	
	var verificationCode models.VerificationCode
	result := db.Where("email = ? AND code = ? AND type = ? AND used = ? AND expires_at > ?",
		email, code, codeType, false, time.Now()).First(&verificationCode)
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("验证码无效或已过期")
	}
	
	// 标记为已使用
	verificationCode.Used = true
	db.Save(&verificationCode)
	
	return nil
}

// HashPassword 哈希密码
func (s *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 检查密码
func (s *AuthService) CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// Register 注册用户
func (s *AuthService) Register(email, password, name, verificationCode string) (*models.User, error) {
	db := database.GetDB()
	
	// 验证验证码
	if err := s.VerifyCode(email, verificationCode, "register"); err != nil {
		return nil, err
	}
	
	// 检查用户是否已存在
	var existingUser models.User
	if result := db.Where("email = ?", email).First(&existingUser); result.RowsAffected > 0 {
		return nil, fmt.Errorf("邮箱已被注册")
	}
	
	// 哈希密码
	hashedPassword, err := s.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	
	// 创建用户
	user := models.User{
		Email:           email,
		Password:        hashedPassword,
		Name:            name,
		IsEmailVerified: true, // 通过验证码注册，邮箱已验证
	}
	
	if err := db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	// 发送欢迎邮件
	go s.emailService.SendWelcomeEmail(email, name)
	
	return &user, nil
}

// Login 用户登录
func (s *AuthService) Login(email, password string) (*models.User, string, error) {
	db := database.GetDB()
	
	var user models.User
	if result := db.Where("email = ?", email).First(&user); result.RowsAffected == 0 {
		return nil, "", fmt.Errorf("邮箱或密码错误")
	}
	
	// 检查密码
	if err := s.CheckPassword(user.Password, password); err != nil {
		return nil, "", fmt.Errorf("邮箱或密码错误")
	}
	
	// 更新最后登录时间
	now := time.Now()
	user.LastLoginAt = &now
	db.Save(&user)
	
	// 生成JWT Token
	token, err := s.GenerateJWT(user.ID, user.Email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}
	
	return &user, token, nil
}

// RequestPasswordReset 请求密码重置
func (s *AuthService) RequestPasswordReset(email string) error {
	db := database.GetDB()
	
	// 检查用户是否存在
	var user models.User
	if result := db.Where("email = ?", email).First(&user); result.RowsAffected == 0 {
		// 为了安全，不要告诉用户邮箱不存在
		return nil
	}
	
	// 生成重置令牌
	token, err := s.GenerateResetToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	
	expiresAt := time.Now().Add(time.Duration(s.config.PasswordResetTokenExpireMinutes) * time.Minute)
	
	resetToken := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiresAt,
		Used:      false,
	}
	
	if err := db.Create(&resetToken).Error; err != nil {
		return fmt.Errorf("failed to create reset token: %w", err)
	}
	
	// 发送重置密码邮件
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.config.FrontendURL, token)
	if err := s.emailService.SendPasswordResetEmail(email, resetLink); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	return nil
}

// ResetPassword 重置密码
func (s *AuthService) ResetPassword(token, newPassword string) error {
	db := database.GetDB()
	
	// 验证令牌
	var resetToken models.PasswordResetToken
	result := db.Where("token = ? AND used = ? AND expires_at > ?",
		token, false, time.Now()).First(&resetToken)
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("重置令牌无效或已过期")
	}
	
	// 获取用户
	var user models.User
	if err := db.First(&user, resetToken.UserID).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	
	// 哈希新密码
	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	
	// 更新密码
	user.Password = hashedPassword
	if err := db.Save(&user).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	
	// 标记令牌为已使用
	resetToken.Used = true
	db.Save(&resetToken)
	
	return nil
}

// GenerateJWT 生成JWT Token
func (s *AuthService) GenerateJWT(userID uint, email string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour * 7) // 7天有效期
	
	claims := &jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", userID),
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "homexai",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateJWT 验证JWT Token
func (s *AuthService) ValidateJWT(tokenString string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		var userID uint
		fmt.Sscanf(claims.Subject, "%d", &userID)
		return userID, nil
	}

	return 0, fmt.Errorf("invalid token")
}

// GetUserByID 通过ID获取用户
func (s *AuthService) GetUserByID(userID uint) (*models.User, error) {
	db := database.GetDB()
	
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}
