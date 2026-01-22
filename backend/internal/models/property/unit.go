package property

import (
	"time"
)

// Unit 房源表(公寓、仓储、商业 - 停车位已独立到parking_slots表)
type Unit struct {
	ID         uint    `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UnitNumber string  `gorm:"column:unit_number;type:varchar(50);not null;uniqueIndex:uk_unit_number" json:"unit_number" default:"" comment:"单元编号"`
	UnitType   string  `gorm:"column:unit_type;type:varchar(20);not null;index:idx_unit_type" json:"unit_type" default:"apartment" comment:"房源类型(apartment,storage,commercial)"`
	Floor      *int    `gorm:"column:floor;type:int;index:idx_floor" json:"floor" default:"null" comment:"楼层"`
	Building   *string `gorm:"column:building;type:varchar(50)" json:"building" default:"null" comment:"楼栋"`
	Area       *string `gorm:"column:area;type:decimal(10,2)" json:"area" default:"null" comment:"面积(平方米)"`
	Bedrooms   *int    `gorm:"column:bedrooms;type:int" json:"bedrooms" default:"null" comment:"卧室数"`
	Bathrooms  *int    `gorm:"column:bathrooms;type:int" json:"bathrooms" default:"null" comment:"浴室数"`

	// 通用字段
	Status      string    `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"available" comment:"状态(available,occupied,maintenance,reserved)"`
	MonthlyRent *string   `gorm:"column:monthly_rent;type:decimal(10,2)" json:"monthly_rent" default:"null" comment:"月租金"`
	Description *string   `gorm:"column:description;type:text" json:"description" default:"null" comment:"描述"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Landlords          []Landlord          `gorm:"foreignKey:UnitID" json:"landlords,omitempty"`
	Tenants            []Tenant            `gorm:"foreignKey:UnitID" json:"tenants,omitempty"`
	ParkingAssignments []ParkingAssignment `gorm:"foreignKey:UnitID" json:"parking_assignments,omitempty"` // 该unit分配的停车位
}

func (Unit) TableName() string {
	return "units"
}

// Unit type constants (不再包含parking)
const (
	UnitTypeApartment  = "apartment"
	UnitTypeStorage    = "storage"
	UnitTypeCommercial = "commercial"
)

// Unit status constants
const (
	UnitStatusAvailable   = "available"
	UnitStatusOccupied    = "occupied"
	UnitStatusMaintenance = "maintenance"
	UnitStatusReserved    = "reserved"
)

// IsAvailable 检查房源是否可用
func (u *Unit) IsAvailable() bool {
	return u.Status == UnitStatusAvailable
}

// IsOccupied 检查房源是否已被占用
func (u *Unit) IsOccupied() bool {
	return u.Status == UnitStatusOccupied
}

// IsApartment 检查是否是公寓
func (u *Unit) IsApartment() bool {
	return u.UnitType == UnitTypeApartment
}

// IsStorage 检查是否是仓储
func (u *Unit) IsStorage() bool {
	return u.UnitType == UnitTypeStorage
}

// IsCommercial 检查是否是商业
func (u *Unit) IsCommercial() bool {
	return u.UnitType == UnitTypeCommercial
}
