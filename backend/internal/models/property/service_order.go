package property

import (
	"time"
)

// ServiceOrder 服务订单表
type ServiceOrder struct {
	ID              uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	OrderNumber     string     `gorm:"column:order_number;type:varchar(50);not null;uniqueIndex:uk_order_number" json:"order_number" default:"" comment:"订单编号"`
	ServiceListingID uint       `gorm:"column:service_listing_id;type:int unsigned;not null;index:idx_service_listing_id" json:"service_listing_id" default:"" comment:"服务ID"`
	UnitID          uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" default:"" comment:"房源ID"`
	UserID          uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"下单用户ID(主库users表)"`
	Nickname        string     `gorm:"column:nickname;type:varchar(100);not null" json:"nickname" default:"" comment:"昵称"`
	Phone           string     `gorm:"column:phone;type:varchar(20);not null" json:"phone" default:"" comment:"联系电话"`
	ServiceTime     time.Time  `gorm:"column:service_time;type:datetime;not null;index:idx_service_time" json:"service_time" default:"" comment:"服务时间"`
	Status          string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"in_service" comment:"订单状态(in_service,completed,cancelled)"`
	AssignedStaffID *uint      `gorm:"column:assigned_staff_id;type:int unsigned;index:idx_assigned_staff_id" json:"assigned_staff_id" default:"null" comment:"指派员工ID(主库users表)"`
	CompletedAt     *time.Time `gorm:"column:completed_at;type:datetime" json:"completed_at" default:"null" comment:"服务完成时间"`
	ConfirmedAt     *time.Time `gorm:"column:confirmed_at;type:datetime" json:"confirmed_at" default:"null" comment:"用户确认时间"`
	CancelledAt     *time.Time `gorm:"column:cancelled_at;type:datetime" json:"cancelled_at" default:"null" comment:"取消时间"`
	Notes           *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	ServiceListing ServiceListing `gorm:"foreignKey:ServiceListingID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"service_listing,omitempty"`
	Unit           Unit           `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (ServiceOrder) TableName() string {
	return "service_orders"
}

// Service order status constants
const (
	ServiceOrderStatusInService = "in_service" // 服务中
	ServiceOrderStatusCompleted = "completed"  // 服务完成
	ServiceOrderStatusCancelled = "cancelled"  // 服务取消
	ServiceOrderStatusClosed    = "closed"     // 已关闭（用户确认后）
)

// IsInService 检查订单是否服务中
func (o *ServiceOrder) IsInService() bool {
	return o.Status == ServiceOrderStatusInService
}

// IsCompleted 检查订单是否已完成
func (o *ServiceOrder) IsCompleted() bool {
	return o.Status == ServiceOrderStatusCompleted
}

// IsCancelled 检查订单是否已取消
func (o *ServiceOrder) IsCancelled() bool {
	return o.Status == ServiceOrderStatusCancelled
}

// IsClosed 检查订单是否已关闭
func (o *ServiceOrder) IsClosed() bool {
	return o.Status == ServiceOrderStatusClosed
}

