package models

import (
	"time"
)

// User 用户模型
type User struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Email             string    `gorm:"uniqueIndex;not null" json:"email"`
	Password          string    `gorm:"not null" json:"-"` // 不在JSON中返回密码
	Name              string    `json:"name"`
	IsEmailVerified   bool      `gorm:"default:false" json:"is_email_verified"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	LastLoginAt       *time.Time `json:"last_login_at"`
}

// VerificationCode 邮箱验证码模型
type VerificationCode struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"index;not null" json:"email"`
	Code      string    `gorm:"not null" json:"code"`
	Type      string    `gorm:"not null" json:"type"` // register, reset_password
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	Used      bool      `gorm:"default:false" json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// PasswordResetToken 密码重置令牌模型
type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	Used      bool      `gorm:"default:false" json:"used"`
	CreatedAt time.Time `json:"created_at"`
}
