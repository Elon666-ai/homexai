package master

import (
	"time"
)

// UserRole 用户物业角色关联表
type UserRole struct {
	ID         uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID     uint       `gorm:"column:user_id;type:int unsigned;not null;uniqueIndex:uk_user_property_role,priority:1;index:idx_user_id" json:"user_id" default:"" comment:"用户ID"`
	PropertyID uint       `gorm:"column:property_id;type:int unsigned;not null;uniqueIndex:uk_user_property_role,priority:2;index:idx_property_id" json:"property_id" default:"0" comment:"物业ID(0表示跨物业/系统级)"`
	Role       string     `gorm:"column:role;type:varchar(50);not null;uniqueIndex:uk_user_property_role,priority:3;index:idx_role" json:"role" default:"" comment:"角色(super_admin,property_admin,property_account,property_staff,landlord,spa,tenant)"`
	Status     string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"active" comment:"状态(active,inactive,suspended)"`
	AssignedBy *uint      `gorm:"column:assigned_by;type:int unsigned" json:"assigned_by" default:"null" comment:"分配者用户ID"`
	AssignedAt *time.Time `gorm:"column:assigned_at;type:datetime" json:"assigned_at" default:"CURRENT_TIMESTAMP" comment:"分配时间"`
	Notes      *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt  time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	User     User     `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
	Property Property `gorm:"foreignKey:PropertyID;references:ID;constraint:false" json:"property,omitempty"` // No FK constraint - allows PropertyID=0 for system-level roles (super_admin)
}

func (UserRole) TableName() string {
	return "user_roles"
}

// IsActive 检查角色分配是否激活
func (upr *UserRole) IsActive() bool {
	return upr.Status == "active"
}

// IsSuperAdmin 检查是否是超级管理员
func (upr *UserRole) IsSuperAdmin() bool {
	return upr.Role == RoleSuperAdmin
}

// IsPropertyAdmin 检查是否是物业管理员
func (upr *UserRole) IsPropertyAdmin() bool {
	return upr.Role == RolePropertyAdmin
}

// IsLandlord 检查是否是业主
func (upr *UserRole) IsLandlord() bool {
	return upr.Role == RoleLandlord
}

// IsTenant 检查是否是租客
func (upr *UserRole) IsTenant() bool {
	return upr.Role == RoleTenant
}

// IsSPA 检查是否是SPA代理人
func (upr *UserRole) IsSPA() bool {
	return upr.Role == RoleSPA
}
