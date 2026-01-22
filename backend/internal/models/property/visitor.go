package property

import (
	"time"
)

// Visitor 访客登记表
type Visitor struct {
	ID            uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UnitID        uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" default:"" comment:"被访房源ID"`
	HostUserID    uint       `gorm:"column:host_user_id;type:int unsigned;not null;index:idx_host_user_id" json:"host_user_id" default:"" comment:"接待人用户ID(主库users表)"`
	VisitorName   string     `gorm:"column:visitor_name;type:varchar(200);not null" json:"visitor_name" default:"" comment:"访客姓名"`
	VisitorPhone  *string    `gorm:"column:visitor_phone;type:varchar(20)" json:"visitor_phone" default:"null" comment:"访客电话"`
	VisitorEmail  *string    `gorm:"column:visitor_email;type:varchar(255)" json:"visitor_email" default:"null" comment:"访客邮箱"`
	VisitorIDType *string    `gorm:"column:visitor_id_type;type:varchar(50)" json:"visitor_id_type" default:"null" comment:"证件类型"`
	VisitorIDNo   *string    `gorm:"column:visitor_id_no;type:varchar(100)" json:"visitor_id_no" default:"null" comment:"证件号码"`
	Purpose       string     `gorm:"column:purpose;type:varchar(100);not null" json:"purpose" default:"" comment:"访问目的(visit,delivery,service,other)"`
	VehiclePlate  *string    `gorm:"column:vehicle_plate;type:varchar(20)" json:"vehicle_plate" default:"null" comment:"车牌号"`
	ExpectedAt    time.Time  `gorm:"column:expected_at;type:datetime;not null;index:idx_expected_at" json:"expected_at" default:"" comment:"预计到访时间"`
	CheckedInAt   *time.Time `gorm:"column:checked_in_at;type:datetime" json:"checked_in_at" default:"null" comment:"实际到访时间"`
	CheckedOutAt  *time.Time `gorm:"column:checked_out_at;type:datetime" json:"checked_out_at" default:"null" comment:"离开时间"`
	CheckedInBy   *uint      `gorm:"column:checked_in_by;type:int unsigned" json:"checked_in_by" default:"null" comment:"登记人用户ID"`
	CheckedOutBy  *uint      `gorm:"column:checked_out_by;type:int unsigned" json:"checked_out_by" default:"null" comment:"登出人用户ID"`
	Status        string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"pending" comment:"状态(pending,approved,checked_in,checked_out,cancelled,rejected)"`
	Notes         *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Unit Unit `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (Visitor) TableName() string {
	return "visitors"
}

// Visitor purpose constants
const (
	VisitorPurposeVisit    = "visit"
	VisitorPurposeDelivery = "delivery"
	VisitorPurposeService  = "service"
	VisitorPurposeOther    = "other"
)

// Visitor status constants
const (
	VisitorStatusPending    = "pending"
	VisitorStatusApproved   = "approved"
	VisitorStatusCheckedIn  = "checked_in"
	VisitorStatusCheckedOut = "checked_out"
	VisitorStatusCancelled  = "cancelled"
	VisitorStatusRejected   = "rejected"
)

// IsPending 检查访客是否待审批
func (v *Visitor) IsPending() bool {
	return v.Status == VisitorStatusPending
}

// IsApproved 检查访客是否已审批
func (v *Visitor) IsApproved() bool {
	return v.Status == VisitorStatusApproved
}

// IsCheckedIn 检查访客是否已登记
func (v *Visitor) IsCheckedIn() bool {
	return v.Status == VisitorStatusCheckedIn
}

// IsCheckedOut 检查访客是否已离开
func (v *Visitor) IsCheckedOut() bool {
	return v.Status == VisitorStatusCheckedOut
}
