package property

import (
	"time"
)

// ParkingSlot 停车位表（独立表，与units分离）
type ParkingSlot struct {
	ID                 uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	SlotNumber         string    `gorm:"column:slot_number;type:varchar(50);not null;uniqueIndex:uk_slot_number" json:"slot_number" default:"" comment:"车位编号"`
	OwnerID            uint      `gorm:"column:owner_id;type:int unsigned;not null;index:idx_user_id" json:"owner_id" default:"" comment:"所有者用户ID(主库users表)"`
	ParkingType        string    `gorm:"column:parking_type;type:varchar(20);not null" json:"parking_type" default:"indoor" comment:"停车位类型(indoor,outdoor,covered,uncovered)"`
	ParkingLevel       *string   `gorm:"column:parking_level;type:varchar(20)" json:"parking_level" default:"null" comment:"停车层级(B1,B2,G,etc)"`
	ParkingZone        *string   `gorm:"column:parking_zone;type:varchar(20);index:idx_parking_zone" json:"parking_zone" default:"null" comment:"停车区域"`
	VehicleTypeAllowed string    `gorm:"column:vehicle_type_allowed;type:varchar(20);not null" json:"vehicle_type_allowed" default:"car" comment:"允许车辆类型(car,motorcycle,both)"`
	Size               *string   `gorm:"column:size;type:varchar(20)" json:"size" default:"null" comment:"车位尺寸(standard,compact,large,motorcycle)"`
	HasCharger         bool      `gorm:"column:has_charger;type:tinyint(1);not null" json:"has_charger" default:"0" comment:"是否有充电桩"`
	IsAccessible       bool      `gorm:"column:is_accessible;type:tinyint(1);not null" json:"is_accessible" default:"0" comment:"是否无障碍车位"`
	MonthlyRent        *string   `gorm:"column:monthly_rent;type:decimal(10,2)" json:"monthly_rent" default:"null" comment:"月租金"`
	Status             string    `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"available" comment:"状态(available,occupied,reserved,maintenance)"`
	Description        *string   `gorm:"column:description;type:text" json:"description" default:"null" comment:"描述"`
	CreatedAt          time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt          time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Assignments []ParkingAssignment   `gorm:"foreignKey:ParkingSlotID" json:"assignments,omitempty"`
	Owners      []LandlordParkingSlot `gorm:"foreignKey:ParkingSlotID" json:"owners,omitempty"` // 停车位所有者
}

func (ParkingSlot) TableName() string {
	return "parking_slots"
}

// Parking type constants
const (
	ParkingSlotTypeIndoor    = "indoor"
	ParkingSlotTypeOutdoor   = "outdoor"
	ParkingSlotTypeCovered   = "covered"
	ParkingSlotTypeUncovered = "uncovered"
)

// Vehicle type allowed constants
const (
	VehicleAllowedCar        = "car"
	VehicleAllowedMotorcycle = "motorcycle"
	VehicleAllowedBoth       = "both"
)

// Parking slot size constants
const (
	ParkingSlotSizeStandard   = "standard"
	ParkingSlotSizeCompact    = "compact"
	ParkingSlotSizeLarge      = "large"
	ParkingSlotSizeMotorcycle = "motorcycle"
)

// Parking slot status constants
const (
	ParkingSlotStatusAvailable   = "available"
	ParkingSlotStatusOccupied    = "occupied"
	ParkingSlotStatusReserved    = "reserved"
	ParkingSlotStatusMaintenance = "maintenance"
)

// IsAvailable 检查车位是否可用
func (p *ParkingSlot) IsAvailable() bool {
	return p.Status == ParkingSlotStatusAvailable
}

// IsOccupied 检查车位是否已占用
func (p *ParkingSlot) IsOccupied() bool {
	return p.Status == ParkingSlotStatusOccupied
}

// AllowsCar 检查是否允许汽车
func (p *ParkingSlot) AllowsCar() bool {
	return p.VehicleTypeAllowed == VehicleAllowedCar || p.VehicleTypeAllowed == VehicleAllowedBoth
}

// AllowsMotorcycle 检查是否允许摩托车
func (p *ParkingSlot) AllowsMotorcycle() bool {
	return p.VehicleTypeAllowed == VehicleAllowedMotorcycle || p.VehicleTypeAllowed == VehicleAllowedBoth
}
