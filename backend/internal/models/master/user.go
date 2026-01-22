package master

import (
	"time"
)

// User 用户表（全局）
type User struct {
	ID                 uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	Email              *string    `gorm:"column:email;type:varchar(191);uniqueIndex:uk_email_phone" json:"email" default:"null" comment:"邮箱地址"`
	Phone              *string    `gorm:"column:phone;type:varchar(20);uniqueIndex:uk_email_phone" json:"phone" default:"null" comment:"手机号"`
	PasswordHash       string     `gorm:"column:password_hash;type:varchar(255);not null" json:"-" default:"" comment:"密码哈希"`
	Salt               string     `gorm:"column:salt;type:varchar(4);not null" json:"-" default:"" comment:"密码盐值"`
	FullName           string     `gorm:"column:full_name;type:varchar(200);not null" json:"full_name" default:"" comment:"用户全名（隐私字段）"`
	Nickname           *string    `gorm:"column:nickname;type:varchar(100)" json:"nickname" default:"null" comment:"昵称（公开显示）"`
	AvatarURL          *string    `gorm:"column:avatar_url;type:varchar(500)" json:"avatar_url" default:"null" comment:"头像URL"`
	PreferredLanguage  string     `gorm:"column:preferred_language;type:varchar(10);not null" json:"preferred_language" default:"en" comment:"首选语言(en,zh-CN,zh-TW,tl)"`
	Status             string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"active" comment:"状态(active,inactive,suspended)"`
	EmailVerified      bool       `gorm:"column:email_verified;type:tinyint(1);not null" json:"email_verified" default:"0" comment:"邮箱是否已验证"`
	EmailVerifiedAt    *time.Time `gorm:"column:email_verified_at;type:datetime" json:"email_verified_at" default:"null" comment:"邮箱验证时间"`
	PhoneVerified      bool       `gorm:"column:phone_verified;type:tinyint(1);not null" json:"phone_verified" default:"0" comment:"手机是否已验证"`
	PhoneVerifiedAt    *time.Time `gorm:"column:phone_verified_at;type:datetime" json:"phone_verified_at" default:"null" comment:"手机验证时间"`
	MustChangePassword bool       `gorm:"column:must_change_password;type:tinyint(1);not null" json:"must_change_password" default:"0" comment:"是否强制修改密码"`
	LastLoginAt        *time.Time `gorm:"column:last_login_at;type:datetime" json:"last_login_at" default:"null" comment:"最后登录时间"`
	LastLoginIP        *string    `gorm:"column:last_login_ip;type:varchar(45)" json:"last_login_ip" default:"null" comment:"最后登录IP"`
	// Privacy settings
	PublicEmail        bool `gorm:"column:public_email;type:tinyint(1);not null" json:"public_email" default:"1" comment:"是否公开邮箱"`
	PublicPhone        bool `gorm:"column:public_phone;type:tinyint(1);not null" json:"public_phone" default:"1" comment:"是否公开手机号"`
	PublicFullName     bool `gorm:"column:public_full_name;type:tinyint(1);not null" json:"public_full_name" default:"1" comment:"是否公开真实姓名"`
	PublicPropertyCert bool `gorm:"column:public_property_cert;type:tinyint(1);not null" json:"public_property_cert" default:"1" comment:"是否公开房产证"`
	PublicVehicleCROR  bool `gorm:"column:public_vehicle_cr_or;type:tinyint(1);not null" json:"public_vehicle_cr_or" default:"1" comment:"是否公开汽车CR/OR"`
	// Notification settings
	EmailNotificationEnabled bool `gorm:"column:email_notification_enabled;type:tinyint(1);not null" json:"email_notification_enabled" default:"1" comment:"是否启用邮件通知"`
	CreatedAt          time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships (not mapped to DB columns)
	OAuthProviders []OAuthProvider `gorm:"foreignKey:UserID" json:"oauth_providers,omitempty"`
	UserRoles      []UserRole      `gorm:"foreignKey:UserID" json:"user_roles,omitempty"`
}

func (User) TableName() string {
	return "users"
}

// IsActive 检查用户是否激活
func (u *User) IsActive() bool {
	return u.Status == "active"
}

// IsSuspended 检查用户是否被禁用
func (u *User) IsSuspended() bool {
	return u.Status == "suspended"
}

// HasVerifiedEmail 检查邮箱是否已验证
func (u *User) HasVerifiedEmail() bool {
	return u.EmailVerified
}

// HasVerifiedPhone 检查手机是否已验证
func (u *User) HasVerifiedPhone() bool {
	return u.PhoneVerified
}

// GetDisplayName 获取显示名称（优先使用nickname，如果为空则使用full_name）
func (u *User) GetDisplayName() string {
	if u.Nickname != nil && *u.Nickname != "" {
		return *u.Nickname
	}
	return u.FullName
}

// SanitizeForViewer 根据查看者过滤敏感信息
// viewerID: 查看者用户ID，如果为0或等于当前用户ID，则返回完整信息
// 对于系统角色，如果不是自己查看，则隐藏敏感信息
func (u *User) SanitizeForViewer(viewerID uint) {
	// 如果是自己查看，不进行过滤
	if viewerID == 0 || viewerID == u.ID {
		return
	}

	// 如果是系统角色，隐藏敏感信息
	if u.IsSystemRole() {
		u.Email = nil
		u.Phone = nil
		u.FullName = ""
		u.LastLoginAt = nil
		u.LastLoginIP = nil
		u.EmailVerified = false
		u.PhoneVerified = false
		u.EmailVerifiedAt = nil
		u.PhoneVerifiedAt = nil
		return
	}

	// 对于非系统角色，根据隐私设置过滤
	if !u.ShouldShowEmail(viewerID) {
		u.Email = nil
	}
	if !u.ShouldShowPhone(viewerID) {
		u.Phone = nil
	}
	if !u.ShouldShowFullName(viewerID) {
		u.FullName = ""
	}
}

// IsSystemRole 检查用户是否有系统角色（不公开资料的角色）
// 系统角色包括：super_admin, property_admin, property_account, property_staff, property_guard
func (u *User) IsSystemRole() bool {
	if u.UserRoles == nil || len(u.UserRoles) == 0 {
		return false
	}
	for _, role := range u.UserRoles {
		if role.Status == "active" {
			switch role.Role {
			case RoleSuperAdmin, RolePropertyAdmin, RolePropertyAccount, RolePropertyStaff, "property_guard":
				return true
			}
		}
	}
	return false
}

// ShouldShowFullName 根据隐私设置判断是否应该显示真实姓名
// viewerID: 查看者用户ID，如果为0或等于当前用户ID，则总是显示
func (u *User) ShouldShowFullName(viewerID uint) bool {
	// 如果是自己查看，总是显示
	if viewerID == 0 || viewerID == u.ID {
		return true
	}
	// 系统角色的资料不公开
	if u.IsSystemRole() {
		return false
	}
	// 否则根据隐私设置
	return u.PublicFullName
}

// ShouldShowEmail 根据隐私设置判断是否应该显示邮箱
func (u *User) ShouldShowEmail(viewerID uint) bool {
	if viewerID == 0 || viewerID == u.ID {
		return true
	}
	// 系统角色的资料不公开
	if u.IsSystemRole() {
		return false
	}
	return u.PublicEmail
}

// ShouldShowPhone 根据隐私设置判断是否应该显示手机号
func (u *User) ShouldShowPhone(viewerID uint) bool {
	if viewerID == 0 || viewerID == u.ID {
		return true
	}
	// 系统角色的资料不公开
	if u.IsSystemRole() {
		return false
	}
	return u.PublicPhone
}
