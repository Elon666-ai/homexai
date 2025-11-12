package models

import (
	"time"
)

// OAuthProvider 定义OAuth提供商类型
type OAuthProvider string

const (
	ProviderGoogle   OAuthProvider = "google"
	ProviderFacebook OAuthProvider = "facebook"
)

type User struct {
	ID            uint          `gorm:"primaryKey" json:"id"`
	Provider      OAuthProvider `gorm:"type:varchar(20);not null" json:"provider"`
	ProviderID    string        `gorm:"uniqueIndex:idx_provider_id;not null" json:"provider_id"`
	Email         string        `gorm:"index" json:"email"`
	Name          string        `json:"name"`
	Picture       string        `json:"picture"`
	EmailVerified bool          `json:"email_verified"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	LastLoginAt   time.Time     `json:"last_login_at"`
}

// GoogleUserInfo Google用户信息
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

// FacebookUserInfo Facebook用户信息
type FacebookUserInfo struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Email   string               `json:"email"`
	Picture FacebookPictureData  `json:"picture"`
}

type FacebookPictureData struct {
	Data FacebookPictureURL `json:"data"`
}

type FacebookPictureURL struct {
	URL string `json:"url"`
}
