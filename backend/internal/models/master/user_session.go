package master

import (
	"time"
)

// UserSession 用户会话表
type UserSession struct {
	ID             uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID         uint      `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"用户ID"`
	SessionToken   string    `gorm:"column:session_token;type:varchar(200);not null;uniqueIndex:uk_session_token" json:"session_token" default:"" comment:"会话令牌"`
	RefreshToken   *string   `gorm:"column:refresh_token;type:varchar(200);uniqueIndex:uk_refresh_token" json:"refresh_token" default:"null" comment:"刷新令牌"`
	DeviceInfo     *string   `gorm:"column:device_info;type:text" json:"device_info" default:"null" comment:"设备信息"`
	IPAddress      *string   `gorm:"column:ip_address;type:varchar(45)" json:"ip_address" default:"null" comment:"IP地址"`
	Language       string    `gorm:"column:language;type:varchar(10);not null" json:"language" default:"en" comment:"语言"`
	ExpiresAt      time.Time `gorm:"column:expires_at;type:datetime;not null;index:idx_expires_at" json:"expires_at" default:"" comment:"过期时间"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	LastActivityAt time.Time `gorm:"column:last_activity_at;type:datetime;not null;autoUpdateTime" json:"last_activity_at" default:"CURRENT_TIMESTAMP" comment:"最后活动时间"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
}

func (UserSession) TableName() string {
	return "user_sessions"
}

// IsExpired 检查会话是否过期
func (s *UserSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
