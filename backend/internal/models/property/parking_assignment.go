package property

import (
	"time"
)

// ParkingAssignment 停车位分配表（关联parking_slots和units）
// 一个unit可以分配多个parking slot
type ParkingAssignment struct {
	ID            uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	ParkingSlotID uint       `gorm:"column:parking_slot_id;type:int unsigned;not null;index:idx_parking_slot_id" json:"parking_slot_id" default:"" comment:"停车位ID(parking_slots表)"`
	UnitID        *uint      `gorm:"column:unit_id;type:int unsigned;index:idx_unit_id" json:"unit_id" default:"null" comment:"分配给的房源ID(units表)"`
	UserID        uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"使用者用户ID(主库users表)"`
	VehiclePlate  string     `gorm:"column:vehicle_plate;type:varchar(20);not null;index:idx_vehicle_plate" json:"vehicle_plate" default:"" comment:"车牌号"`
	VehicleBrand  *string    `gorm:"column:vehicle_brand;type:varchar(100)" json:"vehicle_brand" default:"null" comment:"车辆品牌"`
	VehicleModel  *string    `gorm:"column:vehicle_model;type:varchar(100)" json:"vehicle_model" default:"null" comment:"车辆型号"`
	VehicleColor  *string    `gorm:"column:vehicle_color;type:varchar(50)" json:"vehicle_color" default:"null" comment:"车辆颜色"`
	VehicleType   string     `gorm:"column:vehicle_type;type:varchar(20);not null" json:"vehicle_type" default:"car" comment:"车辆类型(car,motorcycle,other)"`
	AssignmentType string    `gorm:"column:assignment_type;type:varchar(20);not null" json:"assignment_type" default:"" comment:"分配类型(owner,tenant,visitor,rental)"`
	StartDate     time.Time  `gorm:"column:start_date;type:date;not null" json:"start_date" default:"" comment:"开始日期"`
	EndDate       *time.Time `gorm:"column:end_date;type:date" json:"end_date" default:"null" comment:"结束日期"`
	MonthlyFee    *string    `gorm:"column:monthly_fee;type:decimal(10,2)" json:"monthly_fee" default:"null" comment:"月租费用"`
	Status        string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"active" comment:"状态(active,expired,cancelled)"`
	Notes         *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	ParkingSlot ParkingSlot `gorm:"foreignKey:ParkingSlotID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parking_slot,omitempty"`
	Unit        *Unit       `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"unit,omitempty"`
}

func (ParkingAssignment) TableName() string {
	return "parking_assignments"
}

// Assignment type constants
const (
	AssignmentTypeOwner   = "owner"   // 业主自有车位
	AssignmentTypeTenant  = "tenant"  // 租客租用
	AssignmentTypeVisitor = "visitor" // 访客临时
	AssignmentTypeRental  = "rental"  // 对外出租
)

// Assignment status constants
const (
	AssignmentStatusActive    = "active"
	AssignmentStatusExpired   = "expired"
	AssignmentStatusCancelled = "cancelled"
)

// Vehicle type constants
const (
	VehicleTypeCar        = "car"
	VehicleTypeMotorcycle = "motorcycle"
	VehicleTypeOther      = "other"
)

// IsActive 检查分配是否有效
func (p *ParkingAssignment) IsActive() bool {
	if p.Status != AssignmentStatusActive {
		return false
	}
	if p.EndDate != nil && time.Now().After(*p.EndDate) {
		return false
	}
	return true
}

// IsExpired 检查分配是否过期
func (p *ParkingAssignment) IsExpired() bool {
	if p.EndDate == nil {
		return false
	}
	return time.Now().After(*p.EndDate)
}
