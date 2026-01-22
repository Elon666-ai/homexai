package property

import (
	"time"
)

// Request 请求/工单表
type Request struct {
	ID          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID      uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"请求者用户ID(主库users表)"`
	UnitID      *uint      `gorm:"column:unit_id;type:int unsigned;index:idx_unit_id" json:"unit_id" default:"null" comment:"关联房源ID"`
	ParkingID   *uint      `gorm:"column:parking_id;type:int unsigned;index:idx_parking_id" json:"parking_id" default:"null" comment:"关联车位ID"`
	Category    string     `gorm:"column:category;type:varchar(20);not null;index:idx_category" json:"category" default:"house" comment:"类别(house=办公/居住,parking=停车)"`
	RequestType string     `gorm:"column:request_type;type:varchar(50);not null;index:idx_request_type" json:"request_type" default:"" comment:"请求类型"`
	Title       string     `gorm:"column:title;type:varchar(200);not null" json:"title" default:"" comment:"标题"`
	Description *string    `gorm:"column:description;type:text" json:"description" default:"null" comment:"描述"`
	Priority    string     `gorm:"column:priority;type:varchar(20);not null;index:idx_priority" json:"priority" default:"normal" comment:"优先级(low,normal,high,urgent)"`
	Status      string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"pending" comment:"状态(pending,in_progress,completed,cancelled,rejected)"`
	AssignedTo  *uint      `gorm:"column:assigned_to;type:int unsigned;index:idx_assigned_to" json:"assigned_to" default:"null" comment:"分配给用户ID"`
	ResolvedAt  *time.Time `gorm:"column:resolved_at;type:datetime" json:"resolved_at" default:"null" comment:"解决时间"`
	ResolvedBy  *uint      `gorm:"column:resolved_by;type:int unsigned" json:"resolved_by" default:"null" comment:"解决者用户ID"`
	Resolution  *string    `gorm:"column:resolution;type:text" json:"resolution" default:"null" comment:"解决方案"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Unit    *Unit        `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"unit,omitempty"`
	Parking *ParkingSlot `gorm:"foreignKey:ParkingID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"parking,omitempty"`
}

func (Request) TableName() string {
	return "requests"
}

// Request category constants
const (
	RequestCategoryHouse   = "house"
	RequestCategoryParking = "parking"
)

// House-related request type constants
const (
	RequestTypeMoveIn                     = "move_in"
	RequestTypeMoveOut                    = "move_out"
	RequestTypeWorkPermit                 = "work_permit"
	RequestTypeGatePass                   = "gate_pass"
	RequestTypePetRegistration            = "pet_registration"
	RequestTypeHouseholdStaffRegistration = "household_staff_registration"
)

// Parking-related request type constants
const (
	RequestTypeParkingRentApply       = "parking_rent_apply"
	RequestTypeParkingStickerApply    = "parking_sticker_apply"
	RequestTypeParkingRentTermination = "parking_rent_termination"
)

// HouseRequestTypes returns all house-related request types
var HouseRequestTypes = []string{
	RequestTypeMoveIn,
	RequestTypeMoveOut,
	RequestTypeWorkPermit,
	RequestTypeGatePass,
	RequestTypePetRegistration,
	RequestTypeHouseholdStaffRegistration,
}

// ParkingRequestTypes returns all parking-related request types
var ParkingRequestTypes = []string{
	RequestTypeParkingRentApply,
	RequestTypeParkingStickerApply,
	RequestTypeParkingRentTermination,
}

// Request priority constants
const (
	RequestPriorityLow    = "low"
	RequestPriorityNormal = "normal"
	RequestPriorityHigh   = "high"
	RequestPriorityUrgent = "urgent"
)

// Request status constants
const (
	RequestStatusPending    = "pending"
	RequestStatusInProgress = "in_progress"
	RequestStatusCompleted  = "completed"
	RequestStatusCancelled  = "cancelled"
	RequestStatusRejected   = "rejected"
)

// IsPending 检查请求是否待处理
func (r *Request) IsPending() bool {
	return r.Status == RequestStatusPending
}

// IsInProgress 检查请求是否进行中
func (r *Request) IsInProgress() bool {
	return r.Status == RequestStatusInProgress
}

// IsCompleted 检查请求是否已完成
func (r *Request) IsCompleted() bool {
	return r.Status == RequestStatusCompleted
}

// IsUrgent 检查请求是否紧急
func (r *Request) IsUrgent() bool {
	return r.Priority == RequestPriorityUrgent
}

// IsHouseRequest 检查是否为房屋相关请求
func (r *Request) IsHouseRequest() bool {
	return r.Category == RequestCategoryHouse
}

// IsParkingRequest 检查是否为车位相关请求
func (r *Request) IsParkingRequest() bool {
	return r.Category == RequestCategoryParking
}
