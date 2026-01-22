package master

import (
	"time"
)

// Permission 权限表
type Permission struct {
	ID          uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	Code        string    `gorm:"column:code;type:varchar(100);not null;uniqueIndex:uk_code" json:"code" default:"" comment:"权限代码"`
	NameEN      string    `gorm:"column:name_en;type:varchar(100);not null" json:"name_en" default:"" comment:"权限名称(英文)"`
	NameZhCN    *string   `gorm:"column:name_zh_cn;type:varchar(100)" json:"name_zh_cn" default:"null" comment:"权限名称(简体中文)"`
	NameZhTW    *string   `gorm:"column:name_zh_tw;type:varchar(100)" json:"name_zh_tw" default:"null" comment:"权限名称(繁体中文)"`
	NameTL      *string   `gorm:"column:name_tl;type:varchar(100)" json:"name_tl" default:"null" comment:"权限名称(Tagalog)"`
	Resource    string    `gorm:"column:resource;type:varchar(100);not null;index:idx_resource_action,priority:1" json:"resource" default:"" comment:"资源类型"`
	Action      string    `gorm:"column:action;type:varchar(50);not null;index:idx_resource_action,priority:2" json:"action" default:"" comment:"操作类型"`
	Description string    `gorm:"column:description;type:text" json:"description" default:"" comment:"权限描述"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`

	// Relationships
	RolePermissions []RolePermission `gorm:"foreignKey:PermissionID" json:"role_permissions,omitempty"`
}

func (Permission) TableName() string {
	return "permissions"
}

// GetName 根据语言获取权限名称
func (p *Permission) GetName(lang string) string {
	switch lang {
	case "zh-CN":
		if p.NameZhCN != nil && *p.NameZhCN != "" {
			return *p.NameZhCN
		}
	case "zh-TW":
		if p.NameZhTW != nil && *p.NameZhTW != "" {
			return *p.NameZhTW
		}
	case "tl":
		if p.NameTL != nil && *p.NameTL != "" {
			return *p.NameTL
		}
	}
	return p.NameEN
}

// Permission resource constants
const (
	ResourceUnit         = "unit"
	ResourceParking      = "parking"
	ResourceBill         = "bill"
	ResourcePayment      = "payment"
	ResourceRequest      = "request"
	ResourceVisitor      = "visitor"
	ResourceAnnouncement = "announcement"
	ResourceUser         = "user"
)

// Permission action constants
const (
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionAssign  = "assign"
	ActionApprove = "approve"
	ActionCheckin = "checkin"
)
