package master

import (
	"time"
)

// OAuthProvider OAuth提供商关联表
type OAuthProvider struct {
	ID             uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID         uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"用户ID"`
	Provider       string     `gorm:"column:provider;type:varchar(20);not null" json:"provider" default:"" comment:"OAuth提供商(google,facebook)"`
	ProviderUserID string     `gorm:"column:provider_user_id;type:varchar(200);not null;uniqueIndex:uk_provider_user,priority:1" json:"provider_user_id" default:"" comment:"提供商用户ID"`
	ProviderEmail  *string    `gorm:"column:provider_email;type:varchar(200)" json:"provider_email" default:"null" comment:"提供商邮箱"`
	ProviderName   *string    `gorm:"column:provider_name;type:varchar(200)" json:"provider_name" default:"null" comment:"提供商用户名"`
	ProviderAvatar *string    `gorm:"column:provider_avatar;type:varchar(200)" json:"provider_avatar" default:"null" comment:"提供商头像"`
	AccessToken    *string    `gorm:"column:access_token;type:text" json:"-" default:"null" comment:"访问令牌"`
	RefreshToken   *string    `gorm:"column:refresh_token;type:text" json:"-" default:"null" comment:"刷新令牌"`
	TokenExpiresAt *time.Time `gorm:"column:token_expires_at;type:datetime" json:"token_expires_at" default:"null" comment:"令牌过期时间"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
}

func (OAuthProvider) TableName() string {
	return "oauth_providers"
}

// IsGoogle 检查是否是Google提供商
func (o *OAuthProvider) IsGoogle() bool {
	return o.Provider == "google"
}

// IsFacebook 检查是否是Facebook提供商
func (o *OAuthProvider) IsFacebook() bool {
	return o.Provider == "facebook"
}

// IsTokenExpired 检查令牌是否过期
func (o *OAuthProvider) IsTokenExpired() bool {
	if o.TokenExpiresAt == nil {
		return false
	}
	return time.Now().After(*o.TokenExpiresAt)
}
