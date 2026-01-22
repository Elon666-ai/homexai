package master

import (
	"time"
)

// Role 角色表
type Role struct {
	ID              uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	Code            string    `gorm:"column:code;type:varchar(50);not null;uniqueIndex:uk_code" json:"code" default:"" comment:"角色代码"`
	NameEN          string    `gorm:"column:name_en;type:varchar(100);not null" json:"name_en" default:"" comment:"角色名称(英文)"`
	NameZhCN        *string   `gorm:"column:name_zh_cn;type:varchar(100)" json:"name_zh_cn" default:"null" comment:"角色名称(简体中文)"`
	NameZhTW        *string   `gorm:"column:name_zh_tw;type:varchar(100)" json:"name_zh_tw" default:"null" comment:"角色名称(繁体中文)"`
	NameTL          *string   `gorm:"column:name_tl;type:varchar(100)" json:"name_tl" default:"null" comment:"角色名称(Tagalog)"`
	Description     string    `gorm:"column:description;type:text" json:"description" default:"" comment:"角色描述"`
	IsSystem        bool      `gorm:"column:is_system;type:tinyint(1);not null;index:idx_is_system" json:"is_system" default:"0" comment:"是否系统预设角色"`
	IsCrossProperty bool      `gorm:"column:is_cross_property;type:tinyint(1);not null" json:"is_cross_property" default:"0" comment:"是否跨物业角色"`
	CreatedAt       time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	RolePermissions []RolePermission `gorm:"foreignKey:RoleID" json:"role_permissions,omitempty"`
}

func (Role) TableName() string {
	return "roles"
}

// GetName 根据语言获取角色名称
func (r *Role) GetName(lang string) string {
	switch lang {
	case "zh-CN":
		if r.NameZhCN != nil && *r.NameZhCN != "" {
			return *r.NameZhCN
		}
	case "zh-TW":
		if r.NameZhTW != nil && *r.NameZhTW != "" {
			return *r.NameZhTW
		}
	case "tl":
		if r.NameTL != nil && *r.NameTL != "" {
			return *r.NameTL
		}
	}
	return r.NameEN
}

// Role code constants
const (
	RoleSuperAdmin      = "super_admin"
	RolePropertyAdmin   = "property_admin"
	RolePropertyAccount = "property_account"
	RolePropertyStaff   = "property_staff"
	RolePropertyGuard   = "property_guard"
	RoleLandlord        = "landlord"
	RoleSPA             = "spa"
	RoleTenant          = "tenant"
	RoleVendor          = "vendor"
)
