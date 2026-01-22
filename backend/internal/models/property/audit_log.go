package property

import (
	"time"
)

// AuditLog 物业级审计日志表
type AuditLog struct {
	ID           uint64    `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID       *uint     `gorm:"column:user_id;type:int unsigned;index:idx_user_id" json:"user_id" default:"null" comment:"操作用户ID(主库users表)"`
	Action       string    `gorm:"column:action;type:varchar(100);not null;index:idx_action" json:"action" default:"" comment:"操作类型"`
	ResourceType *string   `gorm:"column:resource_type;type:varchar(100)" json:"resource_type" default:"null" comment:"资源类型"`
	ResourceID   *uint     `gorm:"column:resource_id;type:int unsigned" json:"resource_id" default:"null" comment:"资源ID"`
	Description  *string   `gorm:"column:description;type:text" json:"description" default:"null" comment:"操作描述"`
	OldValue     *string   `gorm:"column:old_value;type:json" json:"old_value" default:"null" comment:"修改前的值"`
	NewValue     *string   `gorm:"column:new_value;type:json" json:"new_value" default:"null" comment:"修改后的值"`
	IPAddress    *string   `gorm:"column:ip_address;type:varchar(45)" json:"ip_address" default:"null" comment:"IP地址"`
	UserAgent    *string   `gorm:"column:user_agent;type:text" json:"user_agent" default:"null" comment:"用户代理"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// Audit action constants
const (
	AuditActionCreate  = "create"
	AuditActionUpdate  = "update"
	AuditActionDelete  = "delete"
	AuditActionLogin   = "login"
	AuditActionLogout  = "logout"
	AuditActionApprove = "approve"
	AuditActionReject  = "reject"
	AuditActionAssign  = "assign"
	AuditActionCheckin = "checkin"
)

// Audit resource type constants
const (
	AuditResourceUnit         = "unit"
	AuditResourceLandlord     = "landlord"
	AuditResourceTenant       = "tenant"
	AuditResourceBill         = "bill"
	AuditResourcePayment      = "payment"
	AuditResourceRequest      = "request"
	AuditResourceVisitor      = "visitor"
	AuditResourceAnnouncement = "announcement"
	AuditResourceParking      = "parking"
)
