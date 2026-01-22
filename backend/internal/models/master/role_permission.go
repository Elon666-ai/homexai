package master

import (
	"time"
)

// RolePermission 角色权限关联表
type RolePermission struct {
	ID           uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	RoleID       uint      `gorm:"column:role_id;type:int unsigned;not null;uniqueIndex:uk_role_permission,priority:1" json:"role_id" default:"" comment:"角色ID"`
	PermissionID uint      `gorm:"column:permission_id;type:int unsigned;not null;uniqueIndex:uk_role_permission,priority:2" json:"permission_id" default:"" comment:"权限ID"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`

	// Relationships
	Role       Role       `gorm:"foreignKey:RoleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"role,omitempty"`
	Permission Permission `gorm:"foreignKey:PermissionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"permission,omitempty"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}
